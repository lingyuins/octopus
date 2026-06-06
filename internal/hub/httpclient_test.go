package hub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/model"
)

// newTestSite creates a RemoteSite pointing at the given test server URL.
// AccessToken is left without the "enc:" prefix so crypto.Decrypt passes it
// through as plaintext (no need to call crypto.Init).
func newTestSite(baseURL string) *model.RemoteSite {
	return &model.RemoteSite{
		BaseURL:     baseURL,
		AuthType:    model.AuthTypeAccessToken,
		AccessToken: "test-token-123",
	}
}

func TestFetchJSONSuccess(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header reaches the upstream.
		if got := r.Header.Get("Authorization"); got != "Bearer test-token-123" {
			t.Errorf("expected Authorization header %q, got %q", "Bearer test-token-123", got)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/info" {
			t.Errorf("expected path /api/v1/info, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"code":    200,
			"message": "",
			"data":    map[string]string{"name": "test"},
		})
	}))
	defer srv.Close()

	site := newTestSite(srv.URL)
	result, err := FetchJSON[payload](context.Background(), site, http.MethodGet, "/api/v1/info", nil)
	if err != nil {
		t.Fatalf("FetchJSON returned unexpected error: %v", err)
	}
	if result.Name != "test" {
		t.Errorf("expected Name=%q, got %q", "test", result.Name)
	}
}

func TestFetchJSONSuccessWithBody(t *testing.T) {
	type reqBody struct {
		Query string `json:"query"`
	}
	type respData struct {
		Count int `json:"count"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var body reqBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body.Query != "hello" {
			t.Errorf("expected query=%q, got %q", "hello", body.Query)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]int{"count": 42},
		})
	}))
	defer srv.Close()

	site := newTestSite(srv.URL)
	result, err := FetchJSON[respData](context.Background(), site, http.MethodPost, "/search", reqBody{Query: "hello"})
	if err != nil {
		t.Fatalf("FetchJSON returned unexpected error: %v", err)
	}
	if result.Count != 42 {
		t.Errorf("expected Count=42, got %d", result.Count)
	}
}

func TestFetchJSONAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "bad token",
		})
	}))
	defer srv.Close()

	site := newTestSite(srv.URL)
	_, err := FetchJSON[map[string]any](context.Background(), site, http.MethodGet, "/api/v1/info", nil)
	if err == nil {
		t.Fatal("expected error for API failure response, got nil")
	}
	if !strings.Contains(err.Error(), "bad token") {
		t.Errorf("expected error to contain %q, got %q", "bad token", err.Error())
	}
}

func TestFetchJSONAPICodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"code":    403,
			"message": "forbidden action",
		})
	}))
	defer srv.Close()

	site := newTestSite(srv.URL)
	_, err := FetchJSON[map[string]any](context.Background(), site, http.MethodGet, "/api/v1/info", nil)
	if err == nil {
		t.Fatal("expected error for non-200 code, got nil")
	}
	if !strings.Contains(err.Error(), "forbidden action") {
		t.Errorf("expected error to contain %q, got %q", "forbidden action", err.Error())
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected error to contain status code 403, got %q", err.Error())
	}
}

func TestFetchJSONHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	site := newTestSite(srv.URL)
	_, err := FetchJSON[map[string]any](context.Background(), site, http.MethodGet, "/api/v1/info", nil)
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("expected error to contain %q, got %q", "HTTP 500", err.Error())
	}
}

func TestFetchJSONAuthNone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("expected no Authorization header for AuthTypeNone, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]string{"status": "ok"},
		})
	}))
	defer srv.Close()

	site := &model.RemoteSite{
		BaseURL:  srv.URL,
		AuthType: model.AuthTypeNone,
	}

	type resp struct {
		Status string `json:"status"`
	}
	result, err := FetchJSON[resp](context.Background(), site, http.MethodGet, "/ping", nil)
	if err != nil {
		t.Fatalf("FetchJSON returned unexpected error: %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("expected Status=%q, got %q", "ok", result.Status)
	}
}

func TestFetchJSONNilData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    nil,
		})
	}))
	defer srv.Close()

	site := newTestSite(srv.URL)
	result, err := FetchJSON[map[string]any](context.Background(), site, http.MethodGet, "/empty", nil)
	if err != nil {
		t.Fatalf("FetchJSON returned unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for nil data, got %v", result)
	}
}

func TestBaseURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no trailing slash", "https://api.example.com", "https://api.example.com"},
		{"single trailing slash", "https://api.example.com/", "https://api.example.com"},
		{"multiple trailing slashes", "https://api.example.com///", "https://api.example.com"},
		{"empty string", "", ""},
		{"just a slash", "/", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site := &model.RemoteSite{BaseURL: tt.input}
			got := baseURL(site)
			if got != tt.want {
				t.Errorf("baseURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 5, "hello..."},
		{"empty string", "", 5, ""},
		{"zero limit", "hello", 0, "..."},
		{"single char", "a", 0, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}
