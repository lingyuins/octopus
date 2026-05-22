package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
)

func setupModelListTest(t *testing.T) (int, func()) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name()))
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}

	apiKey := &model.APIKey{
		Name:    "test-key",
		APIKey:  "sk-octopus-test-" + strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name()),
		Enabled: true,
	}
	if err := db.GetDB().WithContext(context.Background()).Create(apiKey).Error; err != nil {
		t.Fatalf("create api key: %v", err)
	}

	// InitCache after DB is seeded so caches pick up the API key
	if err := op.InitCache(); err != nil {
		t.Fatalf("init cache: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
	}

	return apiKey.ID, cleanup
}

func TestGetModelListOpenAIResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKeyID, cleanup := setupModelListTest(t)
	defer cleanup()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Request = c.Request.WithContext(context.Background())
	c.Set("api_key_id", apiKeyID)

	getModelList(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if _, ok := body["success"]; ok {
		t.Fatal("OpenAI /v1/models response must NOT contain 'success' field")
	}
	if obj, ok := body["object"].(string); !ok || obj != "list" {
		t.Fatalf("object = %v, want 'list'", body["object"])
	}
	if _, ok := body["data"]; !ok {
		t.Fatal("response missing 'data' field")
	}
}

func seedChannelAndGroup(t *testing.T) (int64, int64) {
	t.Helper()

	ch := model.Channel{
		Name:    "test-channel",
		Type:    1,
		BaseUrls: []model.BaseUrl{{URL: "https://example.com"}},
		Enabled: true,
	}
	if err := db.GetDB().WithContext(context.Background()).Create(&ch).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	ck := model.ChannelKey{
		ChannelID:  ch.ID,
		ChannelKey: "sk-test-key",
		Enabled:    true,
	}
	if err := db.GetDB().WithContext(context.Background()).Create(&ck).Error; err != nil {
		t.Fatalf("create channel key: %v", err)
	}
	return int64(ch.ID), int64(ck.ID)
}

func TestGetModelListWithEndpointParam_MusicOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKeyID, cleanup := setupModelListTest(t)
	defer cleanup()

	chID, _ := seedChannelAndGroup(t)

	db.GetDB().WithContext(context.Background()).Create(&model.Group{
		Name:         "music-2.6",
		EndpointType: model.EndpointTypeMusicGeneration,
		Mode:         1,
		Items: []model.GroupItem{
			{ChannelID: int(chID), ModelName: "music-2.6", Priority: 1, Weight: 1},
		},
	})
	db.GetDB().WithContext(context.Background()).Create(&model.Group{
		Name:         "gpt-4o",
		EndpointType: model.EndpointTypeChat,
		Mode:         1,
		Items: []model.GroupItem{
			{ChannelID: int(chID), ModelName: "gpt-4o", Priority: 1, Weight: 1},
		},
	})

	if err := op.InitCache(); err != nil {
		t.Fatalf("refresh cache: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models?endpoint=music_generation", nil)
	c.Request = c.Request.WithContext(context.Background())
	c.Set("api_key_id", apiKeyID)

	getModelList(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	data, ok := body["data"].([]any)
	if !ok {
		t.Fatal("response missing 'data' array")
	}

	hasMusic := false
	hasChat := false
	for _, item := range data {
		m, _ := item.(map[string]any)
		if id, _ := m["id"].(string); id == "music-2.6" {
			hasMusic = true
		}
		if id, _ := m["id"].(string); id == "gpt-4o" {
			hasChat = true
		}
	}
	if !hasMusic {
		t.Error("music_generation endpoint filter should include music-2.6")
	}
	if hasChat {
		t.Error("music_generation endpoint filter should NOT include gpt-4o")
	}
}

func TestGetModelListWithEndpointParam_InvalidEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKeyID, cleanup := setupModelListTest(t)
	defer cleanup()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models?endpoint=foobar", nil)
	c.Request = c.Request.WithContext(context.Background())
	c.Set("api_key_id", apiKeyID)

	getModelList(c)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestGetModelListWithEndpointParam_ConversationFamily(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKeyID, cleanup := setupModelListTest(t)
	defer cleanup()

	chID, _ := seedChannelAndGroup(t)

	db.GetDB().WithContext(context.Background()).Create(&model.Group{
		Name:         "gpt-4.1",
		EndpointType: model.EndpointTypeResponses,
		Mode:         1,
		Items: []model.GroupItem{
			{ChannelID: int(chID), ModelName: "gpt-4.1", Priority: 1, Weight: 1},
		},
	})

	if err := op.InitCache(); err != nil {
		t.Fatalf("refresh cache: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models?endpoint=chat", nil)
	c.Request = c.Request.WithContext(context.Background())
	c.Set("api_key_id", apiKeyID)

	getModelList(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	data, _ := body["data"].([]any)
	found := false
	for _, item := range data {
		m, _ := item.(map[string]any)
		if id, _ := m["id"].(string); id == "gpt-4.1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("?endpoint=chat should include responses model gpt-4.1 (conversation family)")
	}
}

func TestGetModelListWithEndpointParam_DefaultNoParamStillWorks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKeyID, cleanup := setupModelListTest(t)
	defer cleanup()

	chID, _ := seedChannelAndGroup(t)

	db.GetDB().WithContext(context.Background()).Create(&model.Group{
		Name:         "music-2.6",
		EndpointType: model.EndpointTypeMusicGeneration,
		Mode:         1,
		Items: []model.GroupItem{
			{ChannelID: int(chID), ModelName: "music-2.6", Priority: 1, Weight: 1},
		},
	})
	db.GetDB().WithContext(context.Background()).Create(&model.Group{
		Name:         "gpt-4o",
		EndpointType: model.EndpointTypeChat,
		Mode:         1,
		Items: []model.GroupItem{
			{ChannelID: int(chID), ModelName: "gpt-4o", Priority: 1, Weight: 1},
		},
	})

	if err := op.InitCache(); err != nil {
		t.Fatalf("refresh cache: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Request = c.Request.WithContext(context.Background())
	c.Set("api_key_id", apiKeyID)

	getModelList(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if obj, ok := body["object"].(string); !ok || obj != "list" {
		t.Fatalf("object = %v, want 'list'", body["object"])
	}

	data, _ := body["data"].([]any)
	hasMusic := false
	hasChat := false
	for _, item := range data {
		m, _ := item.(map[string]any)
		if id, _ := m["id"].(string); id == "music-2.6" {
			hasMusic = true
		}
		if id, _ := m["id"].(string); id == "gpt-4o" {
			hasChat = true
		}
	}
	if !hasMusic || !hasChat {
		t.Error("default /v1/models (no endpoint param) should include both music and chat models")
	}
}

func TestGetModelListAnthropicResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKeyID, cleanup := setupModelListTest(t)
	defer cleanup()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Request = c.Request.WithContext(context.Background())
	c.Set("request_type", "anthropic")
	c.Set("api_key_id", apiKeyID)

	getModelList(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if _, ok := body["data"]; !ok {
		t.Fatal("Anthropic response missing 'data' field")
	}
	if _, ok := body["has_more"]; !ok {
		t.Fatal("Anthropic response missing 'has_more' field")
	}
}
