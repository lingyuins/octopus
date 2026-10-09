package relay

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	relayStreamSessions = &relayStreamSessionStore{
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
	relayStreamSessions = &relayStreamSessionStore{
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

func TestResolveRelayStreamSessionIdentityRequiresExplicitConversationID(t *testing.T) {
	stream := true
	req := &transmodel.InternalLLMRequest{
		Model:      "gpt-4o",
		Stream:     &stream,
		RawRequest: []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hello"}]}`),
	}

	conversationID, requestHash, ok := resolveRelayStreamSessionIdentity("chat", 0, 7, req)
	if ok || conversationID != "" || requestHash != 0 {
		t.Fatalf("resolveRelayStreamSessionIdentity() = (%q, %d, %t), want disabled", conversationID, requestHash, ok)
	}
}

func TestResolveRelayStreamSessionIdentityUsesExplicitConversationID(t *testing.T) {
	stream := true
	req := &transmodel.InternalLLMRequest{
		Model:          "gpt-4o",
		Stream:         &stream,
		ConversationID: "conv-1",
		RawRequest:     []byte(`{"conversation_id":"conv-1","model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hello"}]}`),
	}

	conversationID, requestHash, ok := resolveRelayStreamSessionIdentity("chat", 0, 7, req)
	if !ok {
		t.Fatal("resolveRelayStreamSessionIdentity() should enable explicit stream session")
	}
	if conversationID != "conv-1" {
		t.Fatalf("conversationID = %q, want conv-1", conversationID)
	}
	if requestHash == 0 {
		t.Fatal("requestHash should be non-zero")
	}

	retry := &transmodel.InternalLLMRequest{
		Model:          "gpt-4o",
		Stream:         &stream,
		ConversationID: "conv-1",
		RawRequest:     []byte(`{"conversation_id":"conv-1","model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hello"}],"last_event_id":2}`),
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

	relayStreamSessions = &relayStreamSessionStore{
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

	relayStreamSessions = &relayStreamSessionStore{
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

	relayStreamSessions = &relayStreamSessionStore{
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

	relayStreamSessions = &relayStreamSessionStore{
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

	relayStreamSessions = &relayStreamSessionStore{
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

func TestRelayStreamSessionFinishKeepsBufferForReconnect(t *testing.T) {
	// Finish 不应立即清空 replay 缓冲：断线重连需要重放已生成内容。
	relayStreamSessions = &relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}

	session, created, err := acquireRelayStreamSession("conv-keep", 1, 1)
	if err != nil || !created || session == nil {
		t.Fatalf("acquireRelayStreamSession() = (%v, %t, %v)", session, created, err)
	}
	session.AddPayload([]byte("data: hello\n\n"))
	session.Finish(nil)

	events, done, _ := session.Snapshot(0)
	if !done {
		t.Fatal("session should be done after Finish")
	}
	if len(events) == 0 {
		t.Fatal("buffered events should remain available for reconnect after Finish")
	}
}

func TestEnforceSessionLimitEvictsOldestDoneSessions(t *testing.T) {
	relayStreamSessions = &relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}
	store := relayStreamSessions

	// 填充超过上限的已完成会话，最旧的应被驱逐。
	total := relayStreamMaxSessions + 10
	base := time.Now().Add(-time.Hour)
	store.mu.Lock()
	for i := 0; i < total; i++ {
		key := "k-" + strconv.Itoa(i)
		store.byKey[key] = &relayStreamSession{
			store:             store,
			key:               key,
			conversationScope: key,
			done:              true,
			updatedAt:         base.Add(time.Duration(i) * time.Millisecond),
		}
	}
	store.enforceSessionLimitLocked()
	remaining := len(store.byKey)
	store.mu.Unlock()

	if remaining > relayStreamMaxSessions {
		t.Fatalf("session count = %d, want <= %d", remaining, relayStreamMaxSessions)
	}
}

// 活跃会话在超过宽限阈值（limit × factor）后也必须被驱逐。
// 旧实现「若全是活跃会话则不驱逐」，客户端中途断连时会话会一直保持活跃，
// 导致硬上限完全失效、map 无界增长直到进程被 OOM killer 杀掉（issue #196）。
func TestEnforceSessionLimitEvictsOldestActiveSessionsBeyondGrace(t *testing.T) {
	relayStreamSessions = &relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}
	store := relayStreamSessions

	limit := 8
	overrideStreamSessionMaxSessions = &limit
	t.Cleanup(func() { overrideStreamSessionMaxSessions = nil })

	activeLimit := limit * relayStreamActiveEvictFactor
	total := activeLimit + 5
	base := time.Now().Add(-time.Hour)
	store.mu.Lock()
	for i := 0; i < total; i++ {
		key := "active-" + strconv.Itoa(i)
		store.byKey[key] = &relayStreamSession{
			store:             store,
			key:               key,
			conversationScope: key,
			done:              false, // 全部为活跃会话
			updatedAt:         base.Add(time.Duration(i) * time.Millisecond),
			subscribers:       make(map[chan struct{}]struct{}),
		}
		store.activeByConversation[key] = key
	}
	store.enforceSessionLimitLocked()
	remaining := len(store.byKey)
	_, oldestStillPresent := store.byKey["active-0"]
	_, newestStillPresent := store.byKey["active-"+strconv.Itoa(total-1)]
	store.mu.Unlock()

	if remaining > activeLimit {
		t.Fatalf("session count = %d, want <= %d (active sessions must be evicted past grace)", remaining, activeLimit)
	}
	if oldestStillPresent {
		t.Fatal("oldest active session should have been evicted first")
	}
	if !newestStillPresent {
		t.Fatal("newest active session should be kept")
	}
}

// 未超过宽限阈值时不应驱逐活跃会话，避免正常波动打断正在进行的生成。
func TestEnforceSessionLimitKeepsActiveSessionsWithinGrace(t *testing.T) {
	relayStreamSessions = &relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}
	store := relayStreamSessions

	limit := 8
	overrideStreamSessionMaxSessions = &limit
	t.Cleanup(func() { overrideStreamSessionMaxSessions = nil })

	// 超过 limit 但未超过 limit × factor：活跃会话应全部保留。
	total := limit + 2
	base := time.Now().Add(-time.Hour)
	store.mu.Lock()
	for i := 0; i < total; i++ {
		key := "active-" + strconv.Itoa(i)
		store.byKey[key] = &relayStreamSession{
			store:             store,
			key:               key,
			conversationScope: key,
			done:              false,
			updatedAt:         base.Add(time.Duration(i) * time.Millisecond),
			subscribers:       make(map[chan struct{}]struct{}),
		}
	}
	store.enforceSessionLimitLocked()
	remaining := len(store.byKey)
	store.mu.Unlock()

	if remaining != total {
		t.Fatalf("session count = %d, want %d (active sessions within grace must be kept)", remaining, total)
	}
}

func TestClientGoneGraceExceeded(t *testing.T) {
	relayStreamSessions = &relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}

	session, created, err := acquireRelayStreamSession("conv-gone", 1, 1)
	if err != nil || !created || session == nil {
		t.Fatalf("acquireRelayStreamSession() = (%v, %t, %v)", session, created, err)
	}

	// 客户端未断连时不应触发。
	if session.ClientGoneGraceExceeded() {
		t.Fatal("ClientGoneGraceExceeded() = true before client disconnect")
	}

	session.MarkClientGone()
	if session.ClientGoneGraceExceeded() {
		t.Fatal("ClientGoneGraceExceeded() = true immediately after disconnect, want grace period")
	}

	// 把断连时刻回拨到宽限期之外，模拟上游长时间未结束。
	session.mu.Lock()
	session.clientGoneAt = time.Now().Add(-relayStreamClientGoneGrace - time.Second)
	session.mu.Unlock()

	if !session.ClientGoneGraceExceeded() {
		t.Fatal("ClientGoneGraceExceeded() = false after grace period elapsed")
	}

	// 会话结束后不再触发，避免重复收尾。
	session.Finish(nil)
	if session.ClientGoneGraceExceeded() {
		t.Fatal("ClientGoneGraceExceeded() = true after session finished")
	}
}

// 客户端重连（Subscribe）必须撤销断连宽限计时，否则原转发协程会在宽限期到点时
// 把一个「正有人在读」的会话强行结束——原请求的 context 早已 Done，无法自行感知重连。
func TestSubscribeCancelsClientGoneGrace(t *testing.T) {
	relayStreamSessions = &relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}

	session, _, err := acquireRelayStreamSession("conv-reconnect", 1, 1)
	if err != nil || session == nil {
		t.Fatalf("acquireRelayStreamSession() error: %v", err)
	}

	session.MarkClientGone()
	// 回拨到宽限期之外：若不撤销计时，此时已满足强制结束条件。
	session.mu.Lock()
	session.clientGoneAt = time.Now().Add(-relayStreamClientGoneGrace - time.Second)
	session.mu.Unlock()
	if !session.ClientGoneGraceExceeded() {
		t.Fatal("precondition failed: grace should be exceeded before reconnect")
	}

	_, unsubscribe := session.Subscribe()
	if session.ClientGoneGraceExceeded() {
		t.Fatal("ClientGoneGraceExceeded() = true after client reconnected via Subscribe")
	}
	if !session.HasSubscribers() {
		t.Fatal("HasSubscribers() = false while a subscriber is attached")
	}

	// 重连的读取者离开后，宽限计时应能重新起表。
	unsubscribe()
	if session.HasSubscribers() {
		t.Fatal("HasSubscribers() = true after unsubscribe")
	}
	session.MarkClientGone()
	if session.ClientGoneGraceExceeded() {
		t.Fatal("grace should restart from the new disconnect time, not the original one")
	}
}

// MarkClientGone 重复调用时宽限期应从第一次断连算起。
func TestMarkClientGoneIsIdempotent(t *testing.T) {
	relayStreamSessions = &relayStreamSessionStore{
		byKey:                make(map[string]*relayStreamSession),
		activeByConversation: make(map[string]string),
	}

	session, _, err := acquireRelayStreamSession("conv-gone-twice", 1, 1)
	if err != nil || session == nil {
		t.Fatalf("acquireRelayStreamSession() error: %v", err)
	}

	session.MarkClientGone()
	session.mu.RLock()
	first := session.clientGoneAt
	session.mu.RUnlock()

	time.Sleep(5 * time.Millisecond)
	session.MarkClientGone()

	session.mu.RLock()
	second := session.clientGoneAt
	session.mu.RUnlock()

	if !first.Equal(second) {
		t.Fatalf("clientGoneAt changed on repeated MarkClientGone: %v -> %v", first, second)
	}
}
