package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
	ch "github.com/lingyuins/octopus/internal/op/channel"
	nodeop "github.com/lingyuins/octopus/internal/op/node"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

func TestNodeChannelPayloadRetainsPoolConflictsOnlyForNodes(t *testing.T) {
	for _, id := range []int{-1, 0, 1} {
		payload := channelRequestPayload{NodeID: id, PoolID: 7}
		want := 0
		if id != 0 {
			want = 7
		}
		for _, converted := range []model.Channel{payload.toChannel(), payload.toChannelWithKeyIDs()} {
			if converted.NodeID != id || converted.PoolID != want {
				t.Fatalf("payload conversion id=%d: node=%d pool=%d", id, converted.NodeID, converted.PoolID)
			}
		}
	}
}

func TestNodeChannelBindingRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	crypto.Init("octopus-test-encryption-key")
	if err := db.InitDB("sqlite", filepath.Join(t.TempDir(), "channels.db"), false); err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := op.InitCache(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	node, err := nodeop.Create(ctx, "local", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	item := model.Channel{Name: "node", Enabled: true, NodeID: node.Node.ID}
	if err := ch.Create(&item, ctx); err != nil {
		t.Fatal(err)
	}
	invoke := func(role, body string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/update", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("user_role", role)
		handler(c)
		return w
	}
	response := invoke(model.UserRoleEditor, fmt.Sprintf(`{"id":%d,"node_id":0}`, item.ID), updateChannel)
	if response.Code != http.StatusForbidden {
		t.Fatalf("editor detached node: %d %s", response.Code, response.Body.String())
	}
	response = invoke(model.UserRoleEditor, fmt.Sprintf(`{"id":%d,"name":"renamed","pool_id":0,"proxy_mode":"direct","channel_proxy":""}`, item.ID), updateChannel)
	if response.Code != http.StatusOK {
		t.Fatalf("editor ordinary patch: %d %s", response.Code, response.Body.String())
	}
	response = invoke(model.UserRoleEditor, fmt.Sprintf(`{"name":"other","node_id":%d}`, node.Node.ID), createChannel)
	if response.Code != http.StatusForbidden {
		t.Fatalf("editor created node channel: %d", response.Code)
	}
	for _, diagnostic := range []gin.HandlerFunc{fetchModel, fetchModelsPerKey, testChannel} {
		response = invoke(model.UserRoleEditor, fmt.Sprintf(`{"node_id":%d}`, node.Node.ID), diagnostic)
		if response.Code != http.StatusForbidden {
			t.Fatalf("editor used arbitrary node probe: %d", response.Code)
		}
		response = invoke(model.UserRoleAdmin, fmt.Sprintf(`{"node_id":%d,"pool_id":1}`, node.Node.ID), diagnostic)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("diagnostic dropped pool conflict: %d %s", response.Code, response.Body.String())
		}
		if !bytes.Contains(response.Body.Bytes(), []byte(`"message_key":"node.errors.operationFailed"`)) {
			t.Fatalf("node diagnostic lacks translated error: %s", response.Body.String())
		}
	}
	response = invoke(model.UserRoleAdmin, fmt.Sprintf(`{"name":"conflict","node_id":%d,"pool_id":1}`, node.Node.ID), createChannel)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("node create dropped pool conflict: %d", response.Code)
	}
	response = invoke(model.UserRoleEditor, fmt.Sprintf(`{"id":%d,"node_id":%d,"name":"echo"}`, item.ID, node.Node.ID), updateChannel)
	if response.Code != http.StatusOK {
		t.Fatalf("editor binding echo rejected: %d %s", response.Code, response.Body.String())
	}
	updated, err := ch.Get(item.ID, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if updated.NodeID != node.Node.ID || len(updated.Keys) != 1 || updated.Keys[0].ChannelKey != ch.NodeChannelKey {
		t.Fatal("ordinary editor patch lost node binding or placeholder")
	}
	other, err := nodeop.Create(ctx, "other", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	response = invoke(model.UserRoleEditor, fmt.Sprintf(`{"id":%d,"node_id":%d}`, item.ID, other.Node.ID), updateChannel)
	if response.Code != http.StatusForbidden {
		t.Fatalf("editor rebound node: %d", response.Code)
	}
}
