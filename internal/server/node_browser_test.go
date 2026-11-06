package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/nodetransport"
	"github.com/lingyuins/octopus/internal/nodetransport/testpeer"
	"github.com/lingyuins/octopus/internal/op"
	ch "github.com/lingyuins/octopus/internal/op/channel"
	nodeop "github.com/lingyuins/octopus/internal/op/node"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/router"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

// Explicit browser fixture: a disposable database, synthetic credentials and
// in-memory peers. No real Magpie configuration, vendor or background tasks.
func TestNodeBrowserAcceptance(t *testing.T) {
	if os.Getenv("OCTOPUS_NODE_BROWSER_ACCEPTANCE") != "1" {
		t.Skip("set OCTOPUS_NODE_BROWSER_ACCEPTANCE=1 for the temporary browser fixture")
	}
	gin.SetMode(gin.ReleaseMode)
	crypto.Init("octopus-test-encryption-key")
	if err := db.InitDB("sqlite", filepath.Join(t.TempDir(), "browser.db"), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	conf.AppConfig.Auth.JWTSecret = "node-browser-fixture-jwt-secret"
	if err := op.UserBootstrapCreate("fixture-admin", "Fixture-Password-Only-2026"); err != nil {
		t.Fatal(err)
	}
	if err := op.InitCache(); err != nil {
		t.Fatal(err)
	}
	manager := nodetransport.NewManager()
	old := nodetransport.DefaultManager
	nodetransport.DefaultManager = manager
	t.Cleanup(func() { _ = manager.Close(); nodetransport.DefaultManager = old })
	engine := gin.New()
	engine.Use(gin.Recovery(), middleware.SecurityHeaders(), middleware.AuditManagementWrite(), middleware.StaticLocal("/", "../../web/out"))
	if err := router.RegisterAll(engine); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:49670")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: engine, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	for _, mode := range []string{"online", "bad-key", "empty-models", "offline"} {
		created, err := nodeop.Create(context.Background(), "fixture-"+mode, true, 16)
		if err != nil {
			t.Fatal(err)
		}
		item := model.Channel{Name: "fixture-" + mode, Enabled: true, NodeID: created.Node.ID, Type: outbound.OutboundTypeOpenAIChat, Model: "allowed-model"}
		if err := ch.Create(&item, context.Background()); err != nil {
			t.Fatal(err)
		}
		if mode == "offline" {
			continue
		}
		if id, err := nodeop.Authenticate(context.Background(), created.Token); err != nil || id != created.Node.ID {
			t.Fatalf("fixture credential failed: %d %v", id, err)
		}
		testpeer.Start(t, "ws://127.0.0.1:49670/node/v1/connect", http.Header{"Authorization": {"Bearer " + created.Token}}, 4, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
				t.Errorf("unexpected model invocation: %s %s", r.Method, r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			switch mode {
			case "bad-key":
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprint(w, `{"error":"synthetic secret not reflected"}`)
			case "empty-models":
				_, _ = fmt.Fprint(w, `{"object":"list","data":[]}`)
			default:
				_, _ = fmt.Fprint(w, `{"object":"list","data":[{"id":"allowed-model"}]}`)
			}
		}))
		until := time.Now().Add(5 * time.Second)
		for !manager.Snapshot(created.Node.ID).Online && time.Now().Before(until) {
			time.Sleep(time.Millisecond)
		}
		if !manager.Snapshot(created.Node.ID).Online {
			t.Fatalf("fixture node %s did not register", mode)
		}
	}
	t.Log("Browser fixture: http://127.0.0.1:49670 — fixture-admin / Fixture-Password-Only-2026")
	stopFile := os.Getenv("OCTOPUS_NODE_BROWSER_STOP_FILE")
	if stopFile == "" || !filepath.IsAbs(stopFile) || strings.Contains(stopFile, "..") {
		t.Fatal("set an absolute temporary OCTOPUS_NODE_BROWSER_STOP_FILE for fixture shutdown")
	}
	deadline := time.NewTimer(15 * time.Minute)
	defer deadline.Stop()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if _, err := os.Stat(stopFile); err == nil {
				return
			}
		case <-deadline.C:
			t.Fatal("browser acceptance fixture reached its time limit")
		}
	}
}
