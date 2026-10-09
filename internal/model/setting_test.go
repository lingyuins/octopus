package model

import (
	"testing"
)

func TestDefaultReasoningBufferStrategy(t *testing.T) {
	value, ok := DefaultSettingValue(SettingKeyReasoningBufferStrategy)
	if !ok || value != "immediate" {
		t.Fatalf("default reasoning buffer strategy = %q, registered = %v, want immediate", value, ok)
	}
}

func TestSettingValidateRelayRetry(t *testing.T) {
	tests := []struct {
		name    string
		key     SettingKey
		value   string
		wantErr bool
	}{
		{name: "retry count zero allowed", key: SettingKeyRelayRetryCount, value: "0"},
		{name: "retry count positive allowed", key: SettingKeyRelayRetryCount, value: "3"},
		{name: "retry count negative rejected", key: SettingKeyRelayRetryCount, value: "-1", wantErr: true},
		{name: "route retries one allowed", key: SettingKeyRelayRouteRetries, value: "1"},
		{name: "route retries zero rejected", key: SettingKeyRelayRouteRetries, value: "0", wantErr: true},
		{name: "ratelimit cooldown zero allowed", key: SettingKeyRatelimitCooldown, value: "0"},
		{name: "ratelimit cooldown positive allowed", key: SettingKeyRatelimitCooldown, value: "300"},
		{name: "ratelimit cooldown negative rejected", key: SettingKeyRatelimitCooldown, value: "-1", wantErr: true},
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
			setting := Setting{Key: tt.key, Value: tt.value}
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

// TestSettingValidateTaskIntervals 覆盖周期型设置键的校验：必须是整数且 >0，
// 并在换算 time.Duration 会溢出时拒绝（上界 = MaxInt64 纳秒 / 单位纳秒）。
// keyhealth 三键（interval/fail_threshold/notify_cooldown）曾遗漏在整数 case
// 外层列表中导致内层 >0 检查不可达，这里用非数字用例锁住可达性。
func TestSettingValidateTaskIntervals(t *testing.T) {
	tests := []struct {
		name    string
		key     SettingKey
		value   string
		wantErr bool
	}{
		// 合法值
		{name: "stats interval valid", key: SettingKeyStatsSaveInterval, value: "10"},
		{name: "site sync valid", key: SettingKeySiteSyncInterval, value: "12"},
		{name: "site checkin valid", key: SettingKeySiteCheckinInterval, value: "24"},
		{name: "model info valid", key: SettingKeyModelInfoUpdateInterval, value: "24"},
		{name: "sync llm valid", key: SettingKeySyncLLMInterval, value: "24"},
		{name: "key health valid", key: SettingKeyKeyHealthCheckInterval, value: "30"},
		{name: "key health threshold valid", key: SettingKeyKeyHealthCheckFailThreshold, value: "3"},
		{name: "key health notify cooldown valid", key: SettingKeyKeyHealthCheckNotifyCooldown, value: "300"},
		{name: "pool token refresh valid", key: SettingKeyPoolTokenRefreshInterval, value: "10"},
		{name: "pool quota sync valid", key: SettingKeyPoolQuotaSyncInterval, value: "360"},
		{name: "pool health valid", key: SettingKeyPoolHealthCheckInterval, value: "30"},
		// 非法：0 / 负数 / 非整数
		{name: "stats interval zero rejected", key: SettingKeyStatsSaveInterval, value: "0", wantErr: true},
		{name: "site sync zero rejected", key: SettingKeySiteSyncInterval, value: "0", wantErr: true},
		{name: "site checkin zero rejected", key: SettingKeySiteCheckinInterval, value: "0", wantErr: true},
		{name: "model info zero rejected", key: SettingKeyModelInfoUpdateInterval, value: "0", wantErr: true},
		{name: "sync llm negative rejected", key: SettingKeySyncLLMInterval, value: "-1", wantErr: true},
		{name: "key health zero rejected", key: SettingKeyKeyHealthCheckInterval, value: "0", wantErr: true},
		{name: "key health not a number rejected", key: SettingKeyKeyHealthCheckInterval, value: "abc", wantErr: true},
		{name: "key health threshold zero rejected", key: SettingKeyKeyHealthCheckFailThreshold, value: "0", wantErr: true},
		{name: "key health threshold not a number rejected", key: SettingKeyKeyHealthCheckFailThreshold, value: "abc", wantErr: true},
		{name: "key health notify cooldown zero rejected", key: SettingKeyKeyHealthCheckNotifyCooldown, value: "0", wantErr: true},
		{name: "key health notify cooldown not a number rejected", key: SettingKeyKeyHealthCheckNotifyCooldown, value: "abc", wantErr: true},
		{name: "pool quota zero rejected", key: SettingKeyPoolQuotaSyncInterval, value: "0", wantErr: true},
		{name: "stats interval not a number", key: SettingKeyStatsSaveInterval, value: "abc", wantErr: true},
		{name: "site sync float rejected", key: SettingKeySiteSyncInterval, value: "1.5", wantErr: true},
		// 溢出上界：小时键上界 2562047，分钟键上界 153722867（= MaxInt64 / 单位纳秒）
		{name: "hour key at overflow bound ok", key: SettingKeySiteSyncInterval, value: "2562047"},
		{name: "hour key over overflow bound rejected", key: SettingKeySiteSyncInterval, value: "2562048", wantErr: true},
		{name: "minute key at overflow bound ok", key: SettingKeyStatsSaveInterval, value: "153722867"},
		{name: "minute key over overflow bound rejected", key: SettingKeyStatsSaveInterval, value: "153722868", wantErr: true},
		{name: "hour key huge rejected", key: SettingKeyModelInfoUpdateInterval, value: "99999999999", wantErr: true},
		{name: "minute key huge rejected", key: SettingKeyKeyHealthCheckInterval, value: "999999999999999", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting := Setting{Key: tt.key, Value: tt.value}
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

// TestSettingValidateWebDAVConfig 覆盖通用保存路径（/api/v1/setting/set）下
// webdav_config 的完整校验：interval_hours 必须是 1..168 的数字，其余字段做
// 类型校验（与专用保存端点 /api/v1/backup/webdav/config 的行为对齐）。
func TestSettingValidateWebDAVConfig(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "default config ok", value: `{"enabled":false,"base_url":"","username":"","password":"","remote_path":"/octopus-backup/","interval_hours":6,"include_stats":true,"include_logs":false,"max_backups":10}`},
		{name: "interval 1 ok", value: `{"interval_hours":1}`},
		{name: "interval 168 ok", value: `{"interval_hours":168}`},
		{name: "interval 0 rejected", value: `{"interval_hours":0}`, wantErr: true},
		{name: "interval 169 rejected", value: `{"interval_hours":169}`, wantErr: true},
		{name: "interval missing rejected", value: `{"enabled":true}`, wantErr: true},
		{name: "interval string rejected", value: `{"interval_hours":"6"}`, wantErr: true},
		{name: "interval bool rejected", value: `{"interval_hours":true}`, wantErr: true},
		{name: "interval float rejected", value: `{"interval_hours":1.5}`, wantErr: true},
		{name: "max_backups zero rejected", value: `{"interval_hours":6,"max_backups":0}`, wantErr: true},
		{name: "max_backups negative rejected", value: `{"interval_hours":6,"max_backups":-1}`, wantErr: true},
		{name: "max_backups string rejected", value: `{"interval_hours":6,"max_backups":"10"}`, wantErr: true},
		{name: "max_backups float rejected", value: `{"interval_hours":6,"max_backups":1.5}`, wantErr: true},
		{name: "enabled wrong type rejected", value: `{"interval_hours":6,"enabled":"yes"}`, wantErr: true},
		{name: "remote_path wrong type rejected", value: `{"interval_hours":6,"remote_path":5}`, wantErr: true},
		{name: "array rejected", value: `[1,2]`, wantErr: true},
		{name: "invalid json rejected", value: `{`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting := Setting{Key: SettingKeyWebDAVConfig, Value: tt.value}
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

func TestDefaultSettingValue(t *testing.T) {
	if v, ok := DefaultSettingValue(SettingKeyStatsSaveInterval); !ok || v != "10" {
		t.Fatalf("DefaultSettingValue(stats_save_interval) = %q, %v; want \"10\", true", v, ok)
	}
	if v, ok := DefaultSettingValue(SettingKeySiteSyncInterval); !ok || v != "12" {
		t.Fatalf("DefaultSettingValue(site_sync_interval) = %q, %v; want \"12\", true", v, ok)
	}
	if _, ok := DefaultSettingValue(SettingKey("not_registered")); ok {
		t.Fatal("DefaultSettingValue for unknown key must return false")
	}
}
