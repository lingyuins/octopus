package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDevMockPublicSuccessInterceptsChatCompletions(t *testing.T) {
	t.Setenv("OCTOPUS_DEV_MOCK_SUCCESS", "true")
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(DevMockPublicSuccess())
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		t.Fatal("request should have been intercepted by dev mock middleware")
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload["model"] != "gpt-4o" {
		t.Fatalf("model = %v, want %q", payload["model"], "gpt-4o")
	}
}

func TestDevMockPublicSuccessFallsThroughWhenDisabled(t *testing.T) {
	t.Setenv("OCTOPUS_DEV_MOCK_SUCCESS", "false")
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(DevMockPublicSuccess())
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusTeapot, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusTeapot, recorder.Body.String())
	}
}

func TestDevMockPublicSuccessHandlesOversizeBody(t *testing.T) {
	t.Setenv("OCTOPUS_DEV_MOCK_SUCCESS", "true")
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(DevMockPublicSuccess())
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		t.Fatal("request should have been intercepted by dev mock middleware")
	})

	oversizedBody := strings.Repeat("x", 128<<10) // 128 KiB > 64 KiB limit
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(oversizedBody))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	// Should still return mock success (200), not crash
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}
