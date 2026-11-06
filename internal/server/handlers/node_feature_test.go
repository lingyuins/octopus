package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/helper"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/nodetransport"
	"github.com/lingyuins/octopus/internal/nodetransport/testpeer"
	channelop "github.com/lingyuins/octopus/internal/op/channel"
	nodeop "github.com/lingyuins/octopus/internal/op/node"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
)

func TestNodesFeatureManagementPermissionsAndTunnelLifecycle(t *testing.T) {
	previous := setting.GetCache().GetAll()
	t.Cleanup(func() {
		setting.GetCache().Clear()
		for key, value := range previous {
			setting.GetCache().Set(key, value)
		}
	})
	setupSettingTestDB(t)
	oldSecret := conf.AppConfig.Auth.JWTSecret
	conf.AppConfig.Auth.JWTSecret = "node-feature-test-jwt"
	t.Cleanup(func() { conf.AppConfig.Auth.JWTSecret = oldSecret })
	manager := nodetransport.NewManager()
	oldManager := nodetransport.DefaultManager
	nodetransport.DefaultManager = manager
	t.Cleanup(func() { _ = manager.Close(); nodetransport.DefaultManager = oldManager })

	engine := gin.New()
	engine.GET("/api/v1/plugins", middleware.Auth(), middleware.RequirePermission(auth.PermPluginsRead), listPlugins)
	engine.POST("/api/v1/plugins/:id/enable", middleware.Auth(), requirePluginAdmin, enablePlugin)
	engine.POST("/api/v1/plugins/:id/disable", middleware.Auth(), requirePluginAdmin, disablePlugin)
	engine.POST("/api/v1/setting/set", middleware.Auth(), middleware.RequirePermission(auth.PermSettingsWrite), middleware.RequireJSON(), setSetting)
	engine.POST("/api/v1/channel/enable", middleware.Auth(), middleware.RequirePermission(auth.PermChannelsWrite), enableChannel)
	engine.POST("/api/v1/channel/update", middleware.Auth(), middleware.RequirePermission(auth.PermChannelsWrite), updateChannel)
	engine.POST("/api/v1/channel/test-model", middleware.Auth(), middleware.RequirePermission(auth.PermChannelsWrite), testChannelModel)
	engine.GET("/api/v1/nodes", middleware.Auth(), requireNodesFeature, listNodes)
	engine.GET("/node/v1/connect", connectNode)
	tokens := make(map[string]string)
	for _, role := range []string{model.UserRoleAdmin, model.UserRoleEditor, model.UserRoleViewer} {
		u := model.User{Username: "feature-" + role, Role: role}
		if err := db.GetDB().Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		token, _, err := auth.GenerateJWTToken(60, u.ID, role)
		if err != nil {
			t.Fatal(err)
		}
		tokens[role] = token
	}
	request := func(method, role, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokens[role])
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w
	}
	ctx := context.Background()
	created, err := nodeop.Create(ctx, "edge", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	items := []model.Channel{
		{Name: "feature-node-on", NodeID: created.Node.ID, Enabled: true},
		{Name: "feature-node-off", NodeID: created.Node.ID},
		{Name: "feature-http", Enabled: true},
	}
	for i := range items {
		if err := channelop.Create(&items[i], ctx); err != nil {
			t.Fatal(err)
		}
	}
	for _, role := range []string{model.UserRoleEditor, model.UserRoleViewer} {
		for _, value := range []string{"true", "false"} {
			for _, operation := range []struct{ path, body string }{
				{"/api/v1/plugins/octopus.magpie-node/disable", ""},
				{"/api/v1/plugins/octopus.magpie-node/enable", ""},
				{"/api/v1/setting/set", fmt.Sprintf(`{"key":"nodes_feature_enabled","value":%q}`, value)},
			} {
				got := request(http.MethodPost, role, operation.path, operation.body)
				if got.Code != http.StatusForbidden || !nodeop.FeatureEnabled() {
					t.Fatalf("%s changed node feature: %d %s", role, got.Code, got.Body.String())
				}
			}
		}
	}

	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	var dispatched atomic.Int32
	var streaming atomic.Bool
	canceled := make(chan struct{}, 1)
	startPeer := func() {
		t.Helper()
		testpeer.Start(t, "ws"+strings.TrimPrefix(server.URL, "http")+"/node/v1/connect", http.Header{"Authorization": {"Bearer " + created.Token}}, 4, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dispatched.Add(1)
			if streaming.Load() {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data: partial\n\n")
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				canceled <- struct{}{}
				return
			}
			_, _ = io.WriteString(w, `{"id":"response","choices":[]}`)
		}))
		until := time.Now().Add(time.Second)
		for !manager.Snapshot(created.Node.ID).Online && time.Now().Before(until) {
			time.Sleep(time.Millisecond)
		}
		if !manager.Snapshot(created.Node.ID).Online {
			t.Fatal("node did not register")
		}
	}
	startPeer()
	oldClient := manager.Client(created.Node.ID, time.Second)
	newRequest := func() *http.Request {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, nodetransport.BaseURL+"/chat/completions", strings.NewReader(`{"model":"allowed","messages":[]}`))
		if err != nil {
			t.Fatal(err)
		}
		return req
	}
	streaming.Store(true)
	response, err := oldClient.Do(newRequest())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	for _, disable := range []struct{ path, body string }{
		{"/api/v1/plugins/octopus.magpie-node/disable", ""},
		{"/api/v1/setting/set", `{"key":"nodes_feature_enabled","value":"false"}`},
	} {
		got := request(http.MethodPost, model.UserRoleAdmin, disable.path, disable.body)
		if got.Code != http.StatusOK || nodeop.FeatureEnabled() || manager.Snapshot(created.Node.ID).Online {
			t.Fatalf("feature disable: status=%d enabled=%v online=%v body=%s", got.Code, nodeop.FeatureEnabled(), manager.Snapshot(created.Node.ID).Online, got.Body.String())
		}
		if streaming.Swap(false) {
			if _, err := io.ReadAll(response.Body); !nodetransport.IsExecutionUncertain(err) {
				t.Fatalf("in-flight stream not canceled safely: %v", err)
			}
			select {
			case <-canceled:
			case <-time.After(time.Second):
				t.Fatal("peer inference was not canceled")
			}
		}
		if nodeop.ValidateCredential(ctx, created.Node.ID, created.Token) {
			t.Fatal("disabled feature accepts node credentials")
		}
		if _, err := helper.ChannelHttpClient(&items[0]); !errors.Is(err, nodeop.ErrFeatureDisabled) {
			t.Fatalf("helper accepted disabled feature: %v", err)
		}
		before := dispatched.Load()
		if _, err := oldClient.Do(newRequest()); !errors.Is(err, nodetransport.ErrFeatureDisabled) {
			t.Fatalf("old client bypassed feature disable: %v", err)
		}
		if dispatched.Load() != before {
			t.Fatal("disabled feature dispatched a new inference")
		}
		for i, item := range items {
			ch, err := channelop.Get(item.ID, ctx)
			if err != nil || ch.Enabled != (i == 2) || ch.NodeFeaturePaused != (i == 0) {
				t.Fatalf("paused channel %d: %+v %v", i, ch, err)
			}
		}
		for _, path := range []string{"/api/v1/nodes", "/node/v1/connect"} {
			if got := request(http.MethodGet, model.UserRoleAdmin, path, ""); got.Code != http.StatusForbidden {
				t.Fatalf("disabled %s: %d", path, got.Code)
			}
		}
		for _, patch := range []struct{ path, body string }{
			{"/api/v1/channel/enable", fmt.Sprintf(`{"id":%d,"enabled":true}`, items[0].ID)},
			{"/api/v1/channel/update", fmt.Sprintf(`{"id":%d,"enabled":true}`, items[0].ID)},
			{"/api/v1/channel/test-model", fmt.Sprintf(`{"channel_id":%d,"model_name":"allowed","endpoint_type":"*"}`, items[0].ID)},
		} {
			got := request(http.MethodPost, model.UserRoleAdmin, patch.path, patch.body)
			if got.Code != http.StatusForbidden || !strings.Contains(got.Body.String(), `"message_key":"node.errors.featureDisabled"`) {
				t.Fatalf("channel re-enabled during disable: %d %s", got.Code, got.Body.String())
			}
		}
		listed := request(http.MethodGet, model.UserRoleAdmin, "/api/v1/plugins", "")
		var envelope struct {
			Data struct {
				Plugins []model.Plugin `json:"plugins"`
			} `json:"data"`
		}
		if err := json.Unmarshal(listed.Body.Bytes(), &envelope); err != nil || len(envelope.Data.Plugins) == 0 || envelope.Data.Plugins[0].Enabled {
			t.Fatalf("built-in status: %s %v", listed.Body.String(), err)
		}
		got = request(http.MethodPost, model.UserRoleAdmin, "/api/v1/setting/set", `{"key":"nodes_feature_enabled","value":"true"}`)
		if got.Code != http.StatusOK || !nodeop.FeatureEnabled() {
			t.Fatalf("re-enable: %d %s", got.Code, got.Body.String())
		}
		for i, item := range items {
			ch, err := channelop.Get(item.ID, ctx)
			if err != nil || ch.Enabled != (i != 1) || ch.NodeFeaturePaused {
				t.Fatalf("restored channel %d: %+v %v", i, ch, err)
			}
		}
		startPeer()
		resp, err := oldClient.Do(newRequest())
		if err != nil {
			t.Fatalf("manager unusable after re-enable: %v", err)
		}
		_, err = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}
