package model

import "testing"

func TestSettingValidateAlertNotifyLanguage(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "simplified chinese", value: "zh-Hans"},
		{name: "traditional chinese", value: "zh-Hant"},
		{name: "english", value: "en"},
		{name: "invalid locale", value: "ja", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting := Setting{
				Key:   SettingKeyAlertNotifyLanguage,
				Value: tt.value,
			}

			err := setting.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestSettingValidateRelayRetry(t *testing.T) {
	tests := []struct {
		name    string
		key     SettingKey
		value   string
		wantErr bool
	}{
		// 单个渠道最大重试次数允许设为 0（0=不重试，只试一次），仅负数非法（issue #95 改动1）。
		{name: "retry count zero allowed", key: SettingKeyRelayRetryCount, value: "0"},
		{name: "retry count positive allowed", key: SettingKeyRelayRetryCount, value: "3"},
		{name: "retry count negative rejected", key: SettingKeyRelayRetryCount, value: "-1", wantErr: true},
		{name: "retry count non-integer rejected", key: SettingKeyRelayRetryCount, value: "abc", wantErr: true},
		// 路由级重试次数最小为 1（至少遍历一轮）。
		{name: "route retries one allowed", key: SettingKeyRelayRouteRetries, value: "1"},
		{name: "route retries two allowed", key: SettingKeyRelayRouteRetries, value: "2"},
		{name: "route retries zero rejected", key: SettingKeyRelayRouteRetries, value: "0", wantErr: true},
		// 最大总尝试次数 0=不限制，允许 0 与正数，负数非法。
		{name: "max total attempts zero allowed", key: SettingKeyRelayMaxTotalAttempts, value: "0"},
		{name: "max total attempts positive allowed", key: SettingKeyRelayMaxTotalAttempts, value: "5"},
		{name: "max total attempts negative rejected", key: SettingKeyRelayMaxTotalAttempts, value: "-1", wantErr: true},
		// 429 渠道内延时重试：开关默认关闭；间隔/总等待必须 >=1。
		{name: "rate limit hold interval one allowed", key: SettingKeyRateLimitHoldInterval, value: "10"},
		{name: "rate limit hold interval zero rejected", key: SettingKeyRateLimitHoldInterval, value: "0", wantErr: true},
		{name: "rate limit hold max wait one allowed", key: SettingKeyRateLimitHoldMaxWait, value: "60"},
		{name: "rate limit hold max wait zero rejected", key: SettingKeyRateLimitHoldMaxWait, value: "0", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting := Setting{
				Key:   tt.key,
				Value: tt.value,
			}

			err := setting.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestSettingValidateRateLimitHoldEnabled(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "true allowed", value: "true"},
		{name: "false allowed", value: "false"},
		{name: "invalid rejected", value: "yes", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting := Setting{Key: SettingKeyRateLimitHoldEnabled, Value: tt.value}
			err := setting.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestSettingValidateNavOrder(t *testing.T) {
	tests := []struct {
		name    string
		key     SettingKey
		value   string
		wantErr bool
	}{
		{name: "valid nav order array", key: SettingKeyNavOrder, value: `["home","setting"]`},
		{name: "valid nav visible array", key: SettingKeyNavVisible, value: `["home","setting"]`},
		{name: "malformed json", key: SettingKeyNavOrder, value: `["home"`, wantErr: true},
		{name: "non array value", key: SettingKeyNavVisible, value: `{"home":1}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting := Setting{
				Key:   tt.key,
				Value: tt.value,
			}

			err := setting.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}
