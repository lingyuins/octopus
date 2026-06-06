package relay

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
)

func TestSplitRelaySSEPayload_SplitsMultipleEvents(t *testing.T) {
	payload := []byte("data: first\n\n\nevent: update\ndata: second\n\n")

	got := splitRelaySSEPayload(payload)
	if len(got) != 2 {
		t.Fatalf("splitRelaySSEPayload() len = %d, want 2", len(got))
	}
	if string(got[0]) != "data: first\n\n" {
		t.Fatalf("splitRelaySSEPayload()[0] = %q", string(got[0]))
	}
	if string(got[1]) != "event: update\ndata: second\n\n" {
		t.Fatalf("splitRelaySSEPayload()[1] = %q", string(got[1]))
	}
}

func TestFormatRelaySSEEvent_PrefixesSequenceID(t *testing.T) {
	got := string(formatRelaySSEEvent(7, []byte("data: hello\n\n")))
	if !strings.HasPrefix(got, "id: 7\n") {
		t.Fatalf("formatRelaySSEEvent() = %q, want id prefix", got)
	}
	if !strings.Contains(got, "data: hello\n\n") {
		t.Fatalf("formatRelaySSEEvent() = %q, want payload", got)
	}
}

func TestPopulateRelayRequestSessionFields(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions?last_event_id=9", nil)
	c.Request.Header.Set("X-Conversation-ID", "conv-header")

	req := &transmodel.InternalLLMRequest{}
	populateRelayRequestSessionFields(c, req, []byte(`{"conversation_id":"conv-body","resume_from_sequence":5}`))

	if req.ConversationID != "conv-header" {
		t.Fatalf("ConversationID = %q, want %q", req.ConversationID, "conv-header")
	}
	if req.ResumeFromEventID != 9 {
		t.Fatalf("ResumeFromEventID = %d, want 9", req.ResumeFromEventID)
	}
}

func TestAcquireRelayStreamSession_AllowsReconnectAndBlocksConcurrentDifferentRequest(t *testing.T) {
	relayStreamSessions = relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}

	session, created, err := acquireRelayStreamSession("conv-1", 1, 1)
	if err != nil {
		t.Fatalf("acquireRelayStreamSession() unexpected error: %v", err)
	}
	if !created || session == nil {
		t.Fatal("acquireRelayStreamSession() did not create session")
	}

	reconnected, created, err := acquireRelayStreamSession("conv-1", 1, 1)
	if err != nil {
		t.Fatalf("acquireRelayStreamSession() reconnect error: %v", err)
	}
	if created || reconnected != session {
		t.Fatal("acquireRelayStreamSession() should reuse existing session")
	}

	if _, _, err := acquireRelayStreamSession("conv-1", 1, 2); err == nil {
		t.Fatal("acquireRelayStreamSession() expected busy error for concurrent different request")
	}

	session.Finish(context.Canceled)

	next, created, err := acquireRelayStreamSession("conv-1", 1, 2)
	if err != nil {
		t.Fatalf("acquireRelayStreamSession() second request error: %v", err)
	}
	if !created || next == nil {
		t.Fatal("acquireRelayStreamSession() should create next request session after finish")
	}
}

func TestAcquireRelayStreamSession_AllowsSameConversationAcrossAPIKeys(t *testing.T) {
	relayStreamSessions = relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}

	first, created, err := acquireRelayStreamSession("conv-1", 1, 1)
	if err != nil || !created || first == nil {
		t.Fatalf("first acquireRelayStreamSession() = (%v, %t, %v)", first, created, err)
	}

	second, created, err := acquireRelayStreamSession("conv-1", 2, 1)
	if err != nil {
		t.Fatalf("second acquireRelayStreamSession() unexpected error: %v", err)
	}
	if !created || second == nil {
		t.Fatal("acquireRelayStreamSession() should allow same conversation_id across API keys")
	}
}

func TestBuildRelayStreamSessionHash_IgnoresResumeControlFields(t *testing.T) {
	first := buildRelayStreamSessionHash(
		"chat",
		0,
		1,
		[]byte(`{"conversation_id":"conv-1","model":"gpt-4o","stream":true,"last_event_id":3}`),
	)
	second := buildRelayStreamSessionHash(
		"chat",
		0,
		1,
		[]byte(`{"conversation_id":"conv-1","model":"gpt-4o","stream":true,"last_event_id":9}`),
	)

	if first != second {
		t.Fatalf("buildRelayStreamSessionHash() mismatch: %d != %d", first, second)
	}
}

func TestResolveRelayStreamSessionIdentity_GeneratesImplicitConversationID(t *testing.T) {
	stream := true
	req := &transmodel.InternalLLMRequest{
		Model:      "gpt-4o",
		Stream:     &stream,
		RawRequest: []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hello"}]}`),
	}

	conversationID, requestHash, ok := resolveRelayStreamSessionIdentity("chat", 0, 7, req)
	if !ok {
		t.Fatal("resolveRelayStreamSessionIdentity() should enable stream session for stream requests without conversation_id")
	}
	if !strings.HasPrefix(conversationID, "implicit:") {
		t.Fatalf("conversationID = %q, want implicit prefix", conversationID)
	}
	if requestHash == 0 {
		t.Fatal("requestHash should be non-zero")
	}
	if req.ConversationID != conversationID {
		t.Fatalf("req.ConversationID = %q, want %q", req.ConversationID, conversationID)
	}

	retry := &transmodel.InternalLLMRequest{
		Model:      "gpt-4o",
		Stream:     &stream,
		RawRequest: []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hello"}],"last_event_id":2}`),
	}
	retryConversationID, retryHash, ok := resolveRelayStreamSessionIdentity("chat", 0, 7, retry)
	if !ok {
		t.Fatal("resolveRelayStreamSessionIdentity() should enable retry stream session")
	}
	if retryConversationID != conversationID {
		t.Fatalf("retry conversationID = %q, want %q", retryConversationID, conversationID)
	}
	if retryHash != requestHash {
		t.Fatalf("retry requestHash = %d, want %d", retryHash, requestHash)
	}
}

func TestRelayStreamSessionSnapshotReportsReplayWindowExpiredWhenTrimmed(t *testing.T) {
	maxEvents := 2
	maxBytes := 0
	overrideStreamSessionMaxEvents = &maxEvents
	overrideStreamSessionMaxBytes = &maxBytes
	defer func() {
		overrideStreamSessionMaxEvents = nil
		overrideStreamSessionMaxBytes = nil
	}()

	relayStreamSessions = relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}

	session, created, err := acquireRelayStreamSession("conv-1", 1, 1)
	if err != nil || !created || session == nil {
		t.Fatalf("acquireRelayStreamSession() = (%v, %t, %v)", session, created, err)
	}

	session.AddPayload([]byte("data: one\n\n"))
	session.AddPayload([]byte("data: two\n\n"))
	session.AddPayload([]byte("data: three\n\n"))

	if _, _, err := session.Snapshot(0); !errors.Is(err, errRelayReplayWindowExpired) {
		t.Fatalf("Snapshot(0) err = %v, want %v", err, errRelayReplayWindowExpired)
	}

	events, _, err := session.Snapshot(1)
	if err != nil {
		t.Fatalf("Snapshot(1) unexpected err: %v", err)
	}
	if len(events) != 2 || events[0].Sequence != 2 || events[1].Sequence != 3 {
		t.Fatalf("Snapshot(1) = %#v", events)
	}
}

func TestRelayStreamSessionFinishRemovesExpiredSession(t *testing.T) {
	ttl := 20 * time.Millisecond
	overrideStreamSessionTTL = &ttl
	defer func() {
		overrideStreamSessionTTL = nil
	}()

	relayStreamSessions = relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}

	session, created, err := acquireRelayStreamSession("conv-1", 1, 1)
	if err != nil || !created || session == nil {
		t.Fatalf("acquireRelayStreamSession() = (%v, %t, %v)", session, created, err)
	}

	session.Finish(nil)

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		relayStreamSessions.mu.RLock()
		_, ok := relayStreamSessions.byKey[session.key]
		relayStreamSessions.mu.RUnlock()
		if !ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("session was not cleaned up after TTL")
}

func TestHandleStreamResponseStopsImmediatelyWhenClientDisconnectsWithoutSession(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	clientCtx, cancelClient := context.WithCancel(context.Background())
	defer cancelClient()

	c.Request = httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil).WithContext(clientCtx)

	stream := true
	reader, writer := io.Pipe()
	defer writer.Close()

	ra := &relayAttempt{
		relayRequest: &relayRequest{
			c:               c,
			clientCtx:       clientCtx,
			operationCtx:    context.Background(),
			internalRequest: &transmodel.InternalLLMRequest{Stream: &stream},
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- ra.handleStreamResponse(context.Background(), &http.Response{
			StatusCode: http.StatusOK,
			Body:       reader,
		})
	}()

	cancelClient()

	select {
	case err := <-done:
		if !errors.Is(err, errClientDisconnected) {
			t.Fatalf("handleStreamResponse() err = %v, want errClientDisconnected", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("handleStreamResponse() did not stop after client disconnect")
	}
}

func TestServeRelayStreamSessionReplayExpiredBeforeHeadersReturns409(t *testing.T) {
	gin.SetMode(gin.TestMode)

	relayStreamSessions = relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}

	session, created, err := acquireRelayStreamSession("conv-1", 1, 1)
	if err != nil || !created || session == nil {
		t.Fatalf("acquireRelayStreamSession() = (%v, %t, %v)", session, created, err)
	}

	// Simulate replay window expired
	session.AddPayload([]byte("data: first\n\n"))
	session.droppedBeforeSeq = 10
	session.Finish(nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/chat/completions?last_event_id=1", nil)

	req := &relayRequest{
		streamSession: session,
		internalRequest: &transmodel.InternalLLMRequest{
			ResumeFromEventID: 1, // before droppedBeforeSeq
		},
	}

	serveRelayStreamSession(c, req)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusConflict, recorder.Body.String())
	}
}

func TestServeRelayStreamSessionDoneWithErrorBeforeHeadersReturnsBadGateway(t *testing.T) {
	gin.SetMode(gin.TestMode)

	relayStreamSessions = relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}

	session, created, err := acquireRelayStreamSession("conv-1", 1, 1)
	if err != nil || !created || session == nil {
		t.Fatalf("acquireRelayStreamSession() = (%v, %t, %v)", session, created, err)
	}

	session.Finish(errors.New("upstream internal error"))

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)

	req := &relayRequest{
		streamSession: session,
		internalRequest: &transmodel.InternalLLMRequest{
			ResumeFromEventID: 0,
		},
	}

	serveRelayStreamSession(c, req)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadGateway, recorder.Body.String())
	}
}

func TestServeRelayStreamSessionDoneWithErrorAfterHeadersWritesSSEError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	relayStreamSessions = relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}

	session, created, err := acquireRelayStreamSession("conv-1", 1, 1)
	if err != nil || !created || session == nil {
		t.Fatalf("acquireRelayStreamSession() = (%v, %t, %v)", session, created, err)
	}

	session.AddPayload([]byte("data: hello\n\n"))
	session.Finish(errors.New("upstream internal error"))

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/chat/completions?last_event_id=0", nil)

	req := &relayRequest{
		streamSession: session,
		internalRequest: &transmodel.InternalLLMRequest{
			ResumeFromEventID: 0,
		},
	}

	serveRelayStreamSession(c, req)

	// First event is the SSE data, then the error event — both should be in the body
	body := recorder.Body.String()
	if !strings.Contains(body, "data: hello") {
		t.Fatalf("body should contain data event, got: %s", body)
	}
	if !strings.Contains(body, "event: error") {
		t.Fatalf("body should contain SSE error event, got: %s", body)
	}
	if !strings.Contains(body, "upstream internal error") {
		t.Fatalf("body should contain error message, got: %s", body)
	}
}
