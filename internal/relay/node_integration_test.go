package relay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/helper"
	dm "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/nodetransport"
	"github.com/lingyuins/octopus/internal/nodetransport/testpeer"
	"github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/op/group"
	nodeop "github.com/lingyuins/octopus/internal/op/node"
	"github.com/lingyuins/octopus/internal/transformer/inbound"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

func TestNodeChannelRelayModelsAndSSE(t *testing.T) {
	setupDiagnosticsRelay(t, false)
	created, err := nodeop.Create(context.Background(), "local-test-node", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	nodeID := created.Node.ID
	old := nodetransport.DefaultManager
	manager := nodetransport.NewManager()
	nodetransport.DefaultManager = manager
	t.Cleanup(func() { manager.Close(); nodetransport.DefaultManager = old })
	var calls atomic.Int32
	peerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("cloud credentials reached protocol peer")
		}
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"object":"list","data":[{"id":"upstream"}]}`)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), `"stream":true`) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"id\":\"ok\",\"object\":\"chat.completion.chunk\",\"model\":\"upstream\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello\"}}]}\n\n")
			w.(http.Flusher).Flush()
			fmt.Fprint(w, "data: {\"id\":\"ok\",\"object\":\"chat.completion.chunk\",\"model\":\"upstream\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"ok","object":"chat.completion","model":"upstream","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`)
	})
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-node-token" {
			w.WriteHeader(401)
			return
		}
		manager.Accept(r.Context(), nodeID, w, r, nodetransport.AcceptOptions{MaxConcurrency: 4, Validate: func() bool { return true }})
	}))
	t.Cleanup(cloud.Close)
	ctx := context.Background()
	testpeer.Start(t, "ws"+strings.TrimPrefix(cloud.URL, "http")+"/node/v1/connect", http.Header{"Authorization": []string{"Bearer test-node-token"}}, 4, peerHandler)
	deadline := time.Now().Add(3 * time.Second)
	for !manager.Status(nodeID).Online {
		if time.Now().After(deadline) {
			t.Fatal("connector offline")
		}
		time.Sleep(time.Millisecond)
	}
	id := 939010
	item := dm.Channel{ID: id, Name: "local-magpie", Enabled: true, NodeID: nodeID, Type: outbound.OutboundTypeOpenAIChat, BaseUrls: []dm.BaseUrl{{URL: nodetransport.BaseURL}}, Keys: []dm.ChannelKey{{ID: id * 10, Enabled: true, ChannelKey: "cloud-placeholder"}}}
	channel.GetCache().Set(id, item)
	defer channel.GetCache().Del(id)
	models, err := helper.FetchModels(ctx, item)
	if err != nil || len(models) != 1 || models[0] != "upstream" {
		t.Fatalf("models through node: %v %v", models, err)
	}
	summary, err := helper.TestChannel(ctx, item)
	if err != nil || !summary.Passed {
		t.Fatalf("probe through node: %+v %v", summary, err)
	}
	g := dm.Group{Name: "node-relay-model", EndpointType: "chat", Mode: dm.GroupModeFailover, OutboundFormat: "chat_only", Items: []dm.GroupItem{{ChannelID: id, ModelName: "upstream", Weight: 1, Priority: 1}}}
	if err := group.GroupCreate(&g, ctx); err != nil {
		t.Fatal(err)
	}
	for _, stream := range []bool{false, true} {
		c, w := diagnosticsRequest(g.Name, stream)
		Handler("chat", inbound.InboundTypeOpenAIChat, c)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "hello") {
			t.Fatalf("stream=%v relay failed: %d %s", stream, w.Code, w.Body.String())
		}
		if stream && !strings.Contains(w.Body.String(), "[DONE]") {
			t.Fatal("SSE did not finish")
		}
	}
	if calls.Load() != 4 {
		t.Fatalf("unexpected duplicate model calls: %d", calls.Load())
	}
}

func TestNodeMediaNeverFallsBackToNetwork(t *testing.T) {
	c, _ := diagnosticsRequest("node-model", false)
	channel := &dm.Channel{NodeID: 1, BaseUrls: []dm.BaseUrl{{URL: "http://127.0.0.1:1"}}}
	status, err := forwardMediaRequestJSON(c, mediaEndpointConfig{}, dm.Group{}, channel, "", []byte(`{}`), "node-model", "node-model", false, context.Background())
	if status != http.StatusBadRequest || err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("node media was not rejected before request creation: %d %v", status, err)
	}
}

func TestNodeUncertainExecutionStopsRetries(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", &nodetransport.ExecutionUncertainError{Err: context.DeadlineExceeded})
	decision := ClassifyRelayError(0, err, false)
	if decision.Scope != ScopeAbortAll || !decision.SkipFailureAccounting || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unsafe retry: %+v", decision)
	}
}
