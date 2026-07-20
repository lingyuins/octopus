package helper

import (
	"context"
	"net/http"
	"testing"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

func TestResolveChannelProxyUsesProxyModePool(t *testing.T) {
	configID := 42
	channel := &model.Channel{
		ProxyMode:     model.ProxyUsageModePool,
		ProxyConfigID: &configID,
		Proxy:         true,
	}

	mode, gotID, customURL := resolveChannelProxy(channel)
	if mode != model.ProxyUsageModePool {
		t.Fatalf("mode = %q, want pool", mode)
	}
	if gotID == nil || *gotID != configID {
		t.Fatalf("proxy config id = %#v, want %d", gotID, configID)
	}
	if customURL != "" {
		t.Fatalf("custom url = %q, want empty", customURL)
	}
}

func TestResolveChannelProxyUsesSystemMode(t *testing.T) {
	channel := &model.Channel{
		ProxyMode: model.ProxyUsageModeSystem,
		Proxy:     true,
	}

	mode, gotID, customURL := resolveChannelProxy(channel)
	if mode != model.ProxyUsageModeSystem {
		t.Fatalf("mode = %q, want system", mode)
	}
	if gotID != nil {
		t.Fatalf("proxy config id = %#v, want nil", gotID)
	}
	if customURL != "" {
		t.Fatalf("custom url = %q, want empty", customURL)
	}
}

func TestResolveChannelProxyLegacyCustomURL(t *testing.T) {
	proxyURL := "socks5://127.0.0.1:1080"
	channel := &model.Channel{
		Proxy:        true,
		ChannelProxy: &proxyURL,
	}

	mode, gotID, customURL := resolveChannelProxy(channel)
	if mode != model.ProxyUsageModePool {
		t.Fatalf("mode = %q, want pool", mode)
	}
	if gotID != nil {
		t.Fatalf("proxy config id = %#v, want nil", gotID)
	}
	if customURL != proxyURL {
		t.Fatalf("custom url = %q, want %q", customURL, proxyURL)
	}
}

func TestResolveChannelProxyLegacySystemToggle(t *testing.T) {
	channel := &model.Channel{Proxy: true}

	mode, gotID, customURL := resolveChannelProxy(channel)
	if mode != model.ProxyUsageModeSystem {
		t.Fatalf("mode = %q, want system", mode)
	}
	if gotID != nil || customURL != "" {
		t.Fatalf("unexpected pool fields id=%#v url=%q", gotID, customURL)
	}
}

func TestResolveChannelProxyDirect(t *testing.T) {
	// 显式 direct：ProxyMode=direct 且 Proxy=false（或未开旧开关）
	channel := &model.Channel{ProxyMode: model.ProxyUsageModeDirect, Proxy: false}

	mode, gotID, customURL := resolveChannelProxy(channel)
	if mode != model.ProxyUsageModeDirect {
		t.Fatalf("mode = %q, want direct", mode)
	}
	if gotID != nil || customURL != "" {
		t.Fatalf("unexpected proxy fields id=%#v url=%q", gotID, customURL)
	}
}

func TestResolveChannelProxyLegacyProxyTrueWithDefaultDirectMode(t *testing.T) {
	// 迁移后常见状态：proxy_mode 默认 direct，但旧 proxy 开关仍为 true
	channel := &model.Channel{
		ProxyMode: model.ProxyUsageModeDirect,
		Proxy:     true,
	}
	mode, gotID, customURL := resolveChannelProxy(channel)
	if mode != model.ProxyUsageModeSystem {
		t.Fatalf("mode = %q, want system (legacy proxy flag)", mode)
	}
	if gotID != nil || customURL != "" {
		t.Fatalf("unexpected pool fields id=%#v url=%q", gotID, customURL)
	}
}

func TestResolveChannelProxyLegacyCustomURLWithDefaultDirectMode(t *testing.T) {
	proxyURL := "http://legacy-proxy:8080"
	channel := &model.Channel{
		ProxyMode:    model.ProxyUsageModeDirect,
		Proxy:        true,
		ChannelProxy: &proxyURL,
	}
	mode, gotID, customURL := resolveChannelProxy(channel)
	if mode != model.ProxyUsageModePool {
		t.Fatalf("mode = %q, want pool", mode)
	}
	if gotID != nil {
		t.Fatalf("proxy config id = %#v, want nil", gotID)
	}
	if customURL != proxyURL {
		t.Fatalf("custom url = %q, want %q", customURL, proxyURL)
	}
}

func TestChannelHttpClientPoolUsesCustomProxyURL(t *testing.T) {
	proxyURL := "http://127.0.0.1:18080"
	channel := &model.Channel{
		ProxyMode:    model.ProxyUsageModePool,
		Proxy:        true,
		ChannelProxy: &proxyURL,
	}

	httpClient, err := ChannelHttpClient(channel)
	if err != nil {
		t.Fatalf("ChannelHttpClient: %v", err)
	}
	transport, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", httpClient.Transport)
	}
	if transport.Proxy == nil {
		t.Fatal("expected transport.Proxy to be set")
	}
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	got, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy(req): %v", err)
	}
	if got == nil || got.String() != proxyURL {
		t.Fatalf("proxy url = %v, want %s", got, proxyURL)
	}
}

func TestChannelHttpClientPoolUsesConfigResolver(t *testing.T) {
	configID := 7
	resolvedURL := "http://proxy-pool.example:8080"
	oldResolver := ProxyURLByConfigFunc
	ProxyURLByConfigFunc = func(id int, _ context.Context) (string, error) {
		if id != configID {
			t.Fatalf("resolver id = %d, want %d", id, configID)
		}
		return resolvedURL, nil
	}
	t.Cleanup(func() { ProxyURLByConfigFunc = oldResolver })

	channel := &model.Channel{
		ProxyMode:     model.ProxyUsageModePool,
		ProxyConfigID: &configID,
		Proxy:         true,
	}

	httpClient, err := ChannelHttpClient(channel)
	if err != nil {
		t.Fatalf("ChannelHttpClient: %v", err)
	}
	transport, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", httpClient.Transport)
	}
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	got, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy(req): %v", err)
	}
	if got == nil || got.String() != resolvedURL {
		t.Fatalf("proxy url = %v, want %s", got, resolvedURL)
	}
}

func TestChannelHttpClientSystemUsesSettingProxy(t *testing.T) {
	proxyURL := "http://system-proxy.example:9090"
	setting.GetCache().Set(model.SettingKeyProxyURL, proxyURL)

	channel := &model.Channel{
		ProxyMode: model.ProxyUsageModeSystem,
		Proxy:     true,
	}
	httpClient, err := ChannelHttpClient(channel)
	if err != nil {
		t.Fatalf("ChannelHttpClient: %v", err)
	}
	transport, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", httpClient.Transport)
	}
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	got, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy(req): %v", err)
	}
	if got == nil || got.String() != proxyURL {
		t.Fatalf("proxy url = %v, want %s", got, proxyURL)
	}
}
