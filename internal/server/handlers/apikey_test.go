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

func setupAPIKeyHandlerTest(t *testing.T) context.Context {
	t.Helper()

	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	testName := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", testName)

	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	if err := op.InitCache(); err != nil {
		t.Fatalf("init cache: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	return ctx
}

func TestListAPIKeyMasksKeysForViewer(t *testing.T) {
	ctx := setupAPIKeyHandlerTest(t)

	apiKey := &model.APIKey{
		Name:    "viewer-key",
		APIKey:  "sk-octopus-secret-12345678",
		Enabled: true,
	}
	if err := op.APIKeyCreate(apiKey, ctx); err != nil {
		t.Fatalf("create api key: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/apikey/list", nil)
	c.Set("user_role", model.UserRoleViewer)

	listAPIKey(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Code int            `json:"code"`
		Data []model.APIKey `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 {
		t.Fatalf("unexpected response payload: %+v", response.Data)
	}
	got := response.Data[0].APIKey
	if got == "sk-octopus-secret-12345678" {
		t.Fatalf("expected masked API key, got raw value %q", got)
	}
	if !strings.HasPrefix(got, "sk-o") || !strings.HasSuffix(got, "5678") {
		t.Fatalf("expected masked API key to retain edges, got %q", got)
	}
}

func TestCreateAPIKeyIgnoresReadonlyFields(t *testing.T) {
	setupAPIKeyHandlerTest(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/apikey/create", strings.NewReader(`{
		"id":1234,
		"name":"created-key",
		"api_key":"sk-octopus-client-supplied",
		"enabled":true
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_role", model.UserRoleEditor)

	createAPIKey(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Code int          `json:"code"`
		Data model.APIKey `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ID == 1234 {
		t.Fatalf("create response preserved client supplied id")
	}
	if response.Data.APIKey == "sk-octopus-client-supplied" {
		t.Fatalf("create response preserved client supplied api_key")
	}
	if !strings.HasPrefix(response.Data.APIKey, "sk-octopus-") {
		t.Fatalf("expected generated octopus key, got %q", response.Data.APIKey)
	}
}

func TestUpdateAPIKeyDoesNotEchoRawKey(t *testing.T) {
	ctx := setupAPIKeyHandlerTest(t)

	apiKey := &model.APIKey{
		Name:    "updatable-key",
		APIKey:  "sk-octopus-secret-abcdef12",
		Enabled: true,
	}
	if err := op.APIKeyCreate(apiKey, ctx); err != nil {
		t.Fatalf("create api key: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/apikey/update", strings.NewReader(fmt.Sprintf(`{"id":%d,"name":"renamed","enabled":true}`, apiKey.ID)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_role", model.UserRoleEditor)

	updateAPIKey(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Code int          `json:"code"`
		Data model.APIKey `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.APIKey == "sk-octopus-secret-abcdef12" {
		t.Fatalf("update response leaked raw API key")
	}
}

func TestUpdateAPIKeyNotFoundReturns404(t *testing.T) {
	setupAPIKeyHandlerTest(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/apikey/update", strings.NewReader(`{"id":404,"name":"missing","enabled":true}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_role", model.UserRoleEditor)

	updateAPIKey(c)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
}
