package channel

import (
	"testing"

	"github.com/lingyuins/octopus/internal/model"
)

func TestNormalizeChannelProxyFieldsFromLegacySystem(t *testing.T) {
	ch := &model.Channel{Proxy: true}
	if err := normalizeChannelProxyFields(ch); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if ch.ProxyMode != model.ProxyUsageModeSystem {
		t.Fatalf("ProxyMode = %q, want system", ch.ProxyMode)
	}
	if !ch.Proxy {
		t.Fatal("Proxy should remain true for system mode")
	}
	if ch.ProxyConfigID != nil {
		t.Fatalf("ProxyConfigID = %#v, want nil", ch.ProxyConfigID)
	}
}

func TestNormalizeChannelProxyFieldsPoolRequiresConfigOrURL(t *testing.T) {
	ch := &model.Channel{ProxyMode: model.ProxyUsageModePool}
	if err := normalizeChannelProxyFields(ch); err == nil {
		t.Fatal("expected error when pool mode has neither config id nor channel proxy")
	}
}

func TestNormalizeChannelProxyFieldsPoolWithConfig(t *testing.T) {
	configID := 9
	ch := &model.Channel{
		ProxyMode:     model.ProxyUsageModePool,
		ProxyConfigID: &configID,
	}
	if err := normalizeChannelProxyFields(ch); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if !ch.Proxy {
		t.Fatal("Proxy should be true for pool mode")
	}
	if ch.ProxyConfigID == nil || *ch.ProxyConfigID != configID {
		t.Fatalf("ProxyConfigID = %#v, want %d", ch.ProxyConfigID, configID)
	}
}

func TestNormalizeChannelProxyFieldsDirectClearsConfig(t *testing.T) {
	configID := 3
	ch := &model.Channel{
		ProxyMode:     model.ProxyUsageModeDirect,
		ProxyConfigID: &configID,
		Proxy:         true,
	}
	if err := normalizeChannelProxyFields(ch); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if ch.Proxy {
		t.Fatal("Proxy should be false for direct mode")
	}
	if ch.ProxyConfigID != nil {
		t.Fatalf("ProxyConfigID = %#v, want nil", ch.ProxyConfigID)
	}
}

func TestDeriveProxyModeFromLegacy(t *testing.T) {
	mode, id := deriveProxyModeFromLegacy(false, nil)
	if mode != model.ProxyUsageModeDirect || id != nil {
		t.Fatalf("direct: mode=%q id=%#v", mode, id)
	}

	mode, id = deriveProxyModeFromLegacy(true, nil)
	if mode != model.ProxyUsageModeSystem || id != nil {
		t.Fatalf("system: mode=%q id=%#v", mode, id)
	}

	proxyURL := "http://127.0.0.1:1"
	mode, id = deriveProxyModeFromLegacy(true, &proxyURL)
	if mode != model.ProxyUsageModePool || id != nil {
		t.Fatalf("custom: mode=%q id=%#v", mode, id)
	}
}
