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
