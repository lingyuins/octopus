package sapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lingyuins/octopus/internal/hub"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

func TestFetchUserInfoLogsInAndMapsSAPIUser(t *testing.T) {
	loginCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			if r.Method != http.MethodPost {
				t.Fatalf("login method = %s", r.Method)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode login body: %v", err)
			}
			if body["username"] != "alice" || body["password"] != "secret" {
				t.Fatalf("login body = %#v", body)
			}
			loginCalls++
			json.NewEncoder(w).Encode(map[string]interface{}{
				"role":  "user",
				"token": "jwt-token",
				"user": map[string]interface{}{
					"id":       "usr_123",
					"username": "alice",
					"name":     "Alice Doe",
					"email":    "alice@example.com",
					"enabled":  true,
					"apiKey":   "sk-sapi-primary",
					"apiKeys": []map[string]interface{}{
						{"key": "sk-sapi-first", "enabled": true},
					},
				},
			})
		case "/api/user/me":
			if r.Method != http.MethodGet {
				t.Fatalf("me method = %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer jwt-token" {
				t.Fatalf("authorization = %q", got)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"user": map[string]interface{}{
					"id":       "usr_123",
					"username": "alice",
					"name":     "Alice Doe",
					"email":    "alice@example.com",
					"enabled":  true,
					"apiKey":   "sk-sapi-primary",
					"apiKeys": []map[string]interface{}{
						{"key": "sk-sapi-first", "enabled": true},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	password, err := crypto.Encrypt("secret")
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	site := &model.RemoteSite{
		BaseURL:  server.URL,
		SiteType: model.SiteTypeSAPI,
		Username: "alice",
		Password: password,
	}

	result, err := (&Adapter{}).FetchUserInfo(context.Background(), site)
	if err != nil {
		t.Fatalf("FetchUserInfo error: %v", err)
	}
	if loginCalls != 1 {
		t.Fatalf("login calls = %d", loginCalls)
	}
	if result.ID != 123 || result.Username != "alice" || result.DisplayName != "Alice Doe" || result.Email != "alice@example.com" {
		t.Fatalf("user info = %#v", result)
	}
	if result.Status != 1 || result.AccessToken != "sk-sapi-first" {
		t.Fatalf("status/token = %d/%q", result.Status, result.AccessToken)
	}
}

func TestFetchModelsReadsOpenAICompatibleList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]string{{"id": "gpt-4o"}, {"id": "claude-sonnet-4"}},
		})
	}))
	defer server.Close()

	models, err := (&Adapter{}).FetchModels(context.Background(), &model.RemoteSite{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("FetchModels error: %v", err)
	}
	if len(models) != 2 || models[0] != "gpt-4o" || models[1] != "claude-sonnet-4" {
		t.Fatalf("models = %#v", models)
	}
}

func TestSAPIAdapterIsRegistered(t *testing.T) {
	adapter, err := hub.Get(model.SiteTypeSAPI)
	if err != nil {
		t.Fatalf("Get sapi adapter: %v", err)
	}
	if _, ok := adapter.(*Adapter); !ok {
		t.Fatalf("adapter type = %T", adapter)
	}
}
