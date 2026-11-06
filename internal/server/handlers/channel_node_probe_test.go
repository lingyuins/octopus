package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/nodetransport"
	ch "github.com/lingyuins/octopus/internal/op/channel"
	nodeop "github.com/lingyuins/octopus/internal/op/node"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
)

func TestCheckChannelKeysRequiresAdminOnlyForNodeChannels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("OCTOPUS_DEV_MOCK_SUCCESS", "true")
	if err := db.InitDB("sqlite", filepath.Join(t.TempDir(), "node-key-check.db"), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	manager := nodetransport.NewManager()
	previous := nodetransport.DefaultManager
	nodetransport.DefaultManager = manager
	t.Cleanup(func() { _ = manager.Close(); nodetransport.DefaultManager = previous })
	created, err := nodeop.Create(context.Background(), "node-key-check", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	nodeChannel := model.Channel{Name: "node-key-check", NodeID: created.Node.ID, Enabled: true}
	if err := ch.Create(&nodeChannel, context.Background()); err != nil {
		t.Fatal(err)
	}
	// Dev mock verifies successful ordinary checks without HTTP credentials
	// or an upstream request. Node probes deliberately never use that bypass.
	httpChannel := model.Channel{Name: "http-key-check", Enabled: true}
	if err := ch.Create(&httpChannel, context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{model.UserRoleAdmin, model.UserRoleEditor, model.UserRoleViewer} {
		t.Run(role, func(t *testing.T) {
			engine := gin.New()
			engine.Use(func(c *gin.Context) {
				c.Set("user_role", role)
				c.Next()
			})
			engine.POST("/api/v1/channel/check-keys/:id", middleware.RequirePermission(auth.PermChannelsRead), middleware.RequirePermission(auth.PermChannelsWrite), middleware.RequireJSON(), checkChannelKeys)
			for _, item := range []model.Channel{nodeChannel, httpChannel} {
				request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/channel/check-keys/%d", item.ID), nil)
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, request)
				want := http.StatusForbidden
				if role == model.UserRoleAdmin || (role == model.UserRoleEditor && item.NodeID == 0) {
					want = http.StatusOK
				}
				if response.Code != want {
					t.Fatalf("node=%v status=%d want=%d response=%s", item.NodeID != 0, response.Code, want, response.Body.String())
				}
				if want == http.StatusOK {
					var envelope struct {
						Data struct {
							Passed  bool `json:"passed"`
							Results []struct {
								Code string `json:"diagnostic_code"`
							} `json:"results"`
						} `json:"data"`
					}
					if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || len(envelope.Data.Results) != 1 {
						t.Fatalf("check result: %s error=%v", response.Body.String(), err)
					}
					if item.NodeID != 0 {
						if envelope.Data.Passed || envelope.Data.Results[0].Code != "offline" {
							t.Fatalf("admin node diagnostic: %s", response.Body.String())
						}
					} else if !envelope.Data.Passed || envelope.Data.Results[0].Code != "" {
						t.Fatalf("ordinary channel check regressed: %s", response.Body.String())
					}
				}
			}
		})
	}
}
