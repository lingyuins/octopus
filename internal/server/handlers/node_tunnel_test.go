package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/helper"
	"github.com/lingyuins/octopus/internal/nodetransport"
	"github.com/lingyuins/octopus/internal/nodetransport/testpeer"
	nodeop "github.com/lingyuins/octopus/internal/op/node"
	"github.com/lingyuins/octopus/internal/server/middleware"
)

func TestNodeTunnelThroughGinAndStaticMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := db.InitDB("sqlite", filepath.Join(t.TempDir(), "node-tunnel.db"), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	created, err := nodeop.Create(context.Background(), "test", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	manager := nodetransport.NewManager()
	old := nodetransport.DefaultManager
	nodetransport.DefaultManager = manager
	t.Cleanup(func() { _ = manager.Close(); nodetransport.DefaultManager = old })
	engine := gin.New()
	engine.Use(middleware.SecurityHeaders(), middleware.StaticLocal("/", t.TempDir()))
	engine.GET("/node/v1/connect", connectNode)
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	testpeer.Start(t, "ws"+strings.TrimPrefix(server.URL, "http")+"/node/v1/connect", http.Header{"Authorization": {"Bearer " + created.Token}}, 4, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Errorf("unexpected probe: %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"allowed"}]}`))
	}))
	until := time.Now().Add(time.Second)
	for !manager.Snapshot(created.Node.ID).Online && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if !manager.Snapshot(created.Node.ID).Online {
		t.Fatal("Gin WebSocket upgraded but the node never registered")
	}
	result, err := helper.TestNode(context.Background(), created.Node.ID)
	if err != nil || !result.Passed || result.ModelCount != 1 {
		t.Fatalf("probe after real Gin handshake: %+v %v", result, err)
	}
}
