package handlers

import (
	"testing"

	"github.com/lingyuins/octopus/internal/model"
)

func TestMaskURLDomainForViewer(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "https with path", raw: "https://api.example.com/v1", want: "https://***/v1"},
		{name: "host with port", raw: "http://127.0.0.1:3000/api", want: "http://***/api"},
		{name: "raw domain", raw: "api.example.com", want: "***"},
		{name: "empty", raw: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskURLDomainForViewer(tt.raw); got != tt.want {
				t.Fatalf("maskURLDomainForViewer(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestRedactChannelBaseURLsForViewer(t *testing.T) {
	channels := []model.Channel{{BaseUrls: []model.BaseUrl{{URL: "https://api.example.com/v1", Delay: 12}}}}

	redactChannelBaseURLsForViewer(channels)

	if channels[0].BaseUrls[0].URL != "https://***/v1" {
		t.Fatalf("base url = %q, want masked", channels[0].BaseUrls[0].URL)
	}
	if channels[0].BaseUrls[0].Delay != 12 {
		t.Fatalf("delay = %d, want preserved", channels[0].BaseUrls[0].Delay)
	}
}

func TestRedactSettingsURLsForViewer(t *testing.T) {
	settings := []model.Setting{
		{Key: model.SettingKeyPublicAPIBaseURL, Value: "https://octopus.example.com"},
		{Key: model.SettingKeySemanticCacheEmbeddingModel, Value: "text-embedding-3-small"},
	}

	redactSettingsURLsForViewer(settings)

	if settings[0].Value != "https://***" {
		t.Fatalf("public api base url = %q, want masked", settings[0].Value)
	}
	if settings[1].Value != "text-embedding-3-small" {
		t.Fatalf("non-url setting = %q, want unchanged", settings[1].Value)
	}
}
