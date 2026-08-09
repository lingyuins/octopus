package model

import (
	"encoding/json"
	"strings"
	"time"
)

// AccountPool 号池：集中管理上游账号凭据，渠道通过 PoolID 关联。
type AccountPool struct {
	ID          int    `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"size:128;uniqueIndex;not null"`
	Description string `json:"description" gorm:"size:512"`
	// Strategy 调度策略：ewma（默认，错误率+TTFT 加权）/ round_robin / random / least_loaded（最小荷载比）。
	Strategy           string    `json:"strategy" gorm:"type:varchar(32);not null;default:'ewma'"`
	DefaultConcurrency int       `json:"default_concurrency" gorm:"default:1"`
	CooldownBaseSec    int       `json:"cooldown_base_sec" gorm:"default:300"`
	Enabled            bool      `json:"enabled" gorm:"default:true"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (AccountPool) TableName() string { return "account_pools" }

// PoolAccount 号池内的单个上游账号。
type PoolAccount struct {
	ID               int        `json:"id" gorm:"primaryKey"`
	PoolID           int        `json:"pool_id" gorm:"index;not null"`
	Name             string     `json:"name" gorm:"size:128"`
	Platform         string     `json:"platform" gorm:"type:varchar(32);not null;default:'custom'"`
	Type             string     `json:"type" gorm:"type:varchar(32);not null;default:'apikey'"`
	Models           string     `json:"models" gorm:"type:text"`      // 逗号分隔模型列表，空=不限
	Credentials      string     `json:"credentials" gorm:"type:text"` // 加密存储（crypto.Encrypt）
	BaseURL          string     `json:"base_url" gorm:"size:512"`
	Quota            string     `json:"quota" gorm:"type:text"` // JSON 额度快照缓存（加密存储）
	Status           string     `json:"status" gorm:"type:varchar(32);not null;default:'active'"`
	Schedulable      bool       `json:"schedulable" gorm:"default:true"`
	Priority         int        `json:"priority" gorm:"default:0"`
	Concurrency      int        `json:"concurrency" gorm:"default:0"`
	ProxyConfigID    *int       `json:"proxy_config_id"`
	RateLimitResetAt int64      `json:"rate_limit_reset_at" gorm:"default:0"`
	OverloadUntil    int64      `json:"overload_until" gorm:"default:0"`
	TokenExpiresAt   int64      `json:"token_expires_at" gorm:"default:0"` // OAuth access_token 过期 unix 秒
	TotalRequests    int64      `json:"total_requests" gorm:"default:0"`
	TotalErrors      int64      `json:"total_errors" gorm:"default:0"`
	TotalTokens      int64      `json:"total_tokens" gorm:"default:0"`
	LastUsedAt       *time.Time `json:"last_used_at"`
	ErrorMessage     string     `json:"error_message" gorm:"type:text"`
	Notes            string     `json:"notes" gorm:"size:512"`

	// P0 调度健壮性：临时不可调度（频控/鉴权失败，窗口截止前不参与调度）
	TempUnschedUntil     int64  `json:"temp_unsched_until" gorm:"default:0"`
	TempUnschedReason    string `json:"temp_unsched_reason" gorm:"type:text"`
	AuthErrorCount       int    `json:"auth_error_count" gorm:"default:0"`
	AuthErrorWindowStart int64  `json:"auth_error_window_start" gorm:"default:0"`

	// P2 生命周期：账号整体过期（订阅到期）
	ExpiresAt          int64 `json:"expires_at" gorm:"default:0"`
	AutoPauseOnExpired bool  `json:"auto_pause_on_expired" gorm:"default:false"`

	// P1 负载与路由
	Weight     int `json:"weight" gorm:"default:0"`
	LoadFactor int `json:"load_factor" gorm:"default:0"`

	// P3 平台附加字段（加密存储）
	Extra string `json:"extra" gorm:"type:text"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (PoolAccount) TableName() string { return "pool_accounts" }

// 号池账号平台常量
const (
	PoolPlatformAnthropic  = "anthropic"
	PoolPlatformOpenAI     = "openai"
	PoolPlatformGemini     = "gemini"
	PoolPlatformGrok       = "grok"
	PoolPlatformVolcengine = "volcengine"
	PoolPlatformCustom     = "custom"
)

// 号池账号凭据类型常量
const (
	PoolTypeOAuth      = "oauth"
	PoolTypeAPIKey     = "apikey"
	PoolTypeCookie     = "cookie"
	PoolTypeUpstream   = "upstream"
	PoolTypeSetupToken = "setup-token"
)

// OAuthTypeCodeAssist 是 PoolAccountExtra.OAuthType 的取值，标记 gemini OAuth
// 账号走 Cloud Code Assist（cloudcode-pa）而非官方 Generative Language API。
const OAuthTypeCodeAssist = "code_assist"

// PoolAccountExtra 平台附加字段（Extra JSON 反序列化后的结构）
// 敏感键（含 key/token/secret/cookie 字样的 header value）在写库前单独脱敏存储，
// 但本 struct 只承载平台标识与路由相关字段，不直接放凭据。
type PoolAccountExtra struct {
	// P3 平台字段
	ProjectID   string `json:"project_id,omitempty"`   // gemini/antigravity
	TierID      string `json:"tier_id,omitempty"`      // gemini
	OAuthType   string `json:"oauth_type,omitempty"`   // gemini code_assist
	AuthMode    string `json:"auth_mode,omitempty"`    // openai personalAccessToken
	PrivacyMode string `json:"privacy_mode,omitempty"` // openai/antigravity
	// P3 自定义额外请求头（仅 apikey 类型，平台 anthropic/openai；grok 允许 oauth）
	HeaderOverridesEnabled bool              `json:"header_overrides_enabled,omitempty"`
	HeaderOverrides        map[string]string `json:"header_overrides,omitempty"`
	// P3 TLS 指纹预留（本计划不实现 dialer）
	TLSFingerprintProfile string `json:"tls_fingerprint_profile,omitempty"`
	// P2 刷新失败退避（写入 Extra JSON，不加列）
	RefreshFailureCount  int   `json:"refresh_failure_count,omitempty"`
	NextRefreshAllowedAt int64 `json:"next_refresh_allowed_at,omitempty"`
}

// IsSchedulable 判断账号当前是否可参与调度。
func (a *PoolAccount) IsSchedulable() bool {
	if a.Status != "active" || !a.Schedulable {
		return false
	}
	now := time.Now().Unix()
	if a.RateLimitResetAt > now || a.OverloadUntil > now {
		return false
	}
	if a.IsTempUnsched() {
		return false
	}
	if a.IsExpired() {
		return false
	}
	if a.IsTokenExpired() {
		return false
	}
	return true
}

// IsTempUnsched 判断账号是否被临时调度禁用。
func (a *PoolAccount) IsTempUnsched() bool {
	return a.TempUnschedUntil > 0 && time.Now().Unix() < a.TempUnschedUntil
}

// IsExpired 判断账号是否整体过期（订阅到期）
func (a *PoolAccount) IsExpired() bool {
	return a.AutoPauseOnExpired && a.ExpiresAt > 0 && time.Now().Unix() >= a.ExpiresAt
}

// GetExtra 解析 Extra JSON；解析失败返回空结构。
func (a *PoolAccount) GetExtra() PoolAccountExtra {
	var e PoolAccountExtra
	if a.Extra == "" {
		return e
	}
	_ = json.Unmarshal([]byte(a.Extra), &e)
	return e
}

// SetExtra 序列化 Extra 结构体并写入 Extra 字段。
func (a *PoolAccount) SetExtra(e PoolAccountExtra) {
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	a.Extra = string(b)
}

// EffectiveLoadFactor 返回调度荷载因子：load_factor>0 则返回它，否则回退 EffectiveConcurrency(0)。
func (a *PoolAccount) EffectiveLoadFactor() int {
	if a.LoadFactor > 0 {
		return a.LoadFactor
	}
	return a.EffectiveConcurrency(0)
}

// IsRefreshAllowed 判断当前是否在刷新退避窗口内，true 表示允许刷新。
func (a *PoolAccount) IsRefreshAllowed(now time.Time) bool {
	extra := a.GetExtra()
	return extra.NextRefreshAllowedAt <= now.Unix()
}

// IsTokenExpired 判断 OAuth 账号的 access_token 是否即将过期（提前 60 秒视为过期）。
// 非 OAuth 类型或未记录过期时间的账号返回 false。
func (a *PoolAccount) IsTokenExpired() bool {
	if a.Type != PoolTypeOAuth {
		return false
	}
	if a.TokenExpiresAt <= 0 {
		return false
	}
	return a.TokenExpiresAt < time.Now().Unix()+60
}

// EffectiveConcurrency 返回生效的并发上限：账号级优先，0 则继承池默认。
func (a *PoolAccount) EffectiveConcurrency(poolDefault int) int {
	if a.Concurrency > 0 {
		return a.Concurrency
	}
	if poolDefault > 0 {
		return poolDefault
	}
	return 1
}

// PoolCredential 凭据 JSON 结构。按 Type 解析不同字段：
//   - apikey:   {"type":"apikey","api_key":"sk-..."}
//   - cookie:   {"type":"cookie","cookie":"sessionKey=..."}
//   - oauth:    {"type":"oauth","access_token":"...","refresh_token":"...","account_id":"...","id_token":"..."}
//   - upstream: {"type":"upstream","api_key":"...","base_url":"https://..."}
//
// 旧格式 {"type":"bearer","token":"..."} 与 {"type":"cookie","token":"..."} 向后兼容：
// Token 字段保留，bearer 归入 apikey 语义（出站按 Bearer 头发送），cookie.token 归入 cookie。
type PoolCredential struct {
	Type         string `json:"type"`
	Token        string `json:"token,omitempty"`         // 向后兼容：bearer token 或 cookie value
	APIKey       string `json:"api_key,omitempty"`       // apikey / upstream
	Cookie       string `json:"cookie,omitempty"`        // cookie
	AccessToken  string `json:"access_token,omitempty"`  // oauth
	RefreshToken string `json:"refresh_token,omitempty"` // oauth
	AccountID    string `json:"account_id,omitempty"`    // oauth (openai/codex)
	IDToken      string `json:"id_token,omitempty"`      // oauth (openai/grok)
	BaseURL      string `json:"base_url,omitempty"`      // upstream
}

// ParsePoolCredential 解析凭据 JSON。解析失败时返回空 PoolCredential。
func ParsePoolCredential(raw string) PoolCredential {
	var cred PoolCredential
	if raw == "" {
		return cred
	}
	_ = json.Unmarshal([]byte(raw), &cred)
	// 旧格式兼容：type=bearer -> 当作 apikey，token 映射到 APIKey。
	if cred.Type == "bearer" {
		if cred.APIKey == "" {
			cred.APIKey = cred.Token
		}
		cred.Type = PoolTypeAPIKey
	}
	// 旧格式 cookie：token 字段映射到 Cookie。
	if cred.Type == "cookie" && cred.Cookie == "" && cred.Token != "" {
		cred.Cookie = cred.Token
	}
	return cred
}

// EffectiveKey 返回用于出站鉴权的 ChannelKey 字符串（空 extra）。
// 调用方知账号 extra 时应使用 EffectiveKeyWithExtra，以支持 auth_mode=personalAccessToken 等分支。
func (c PoolCredential) EffectiveKey(platform string) string {
	return c.EffectiveKeyWithExtra(platform, PoolAccountExtra{})
}

// EffectiveKeyWithExtra 返回用于出站鉴权的 ChannelKey 字符串。
// 按 Type 选取最合适的字段；oauth 的 openai/codex 返回完整 OAuth JSON（供 codex 适配器解析）。
// extra.AuthMode==personalAccessToken 时返回 AccessToken 作为 Bearer，不走 JSON 透传。
func (c PoolCredential) EffectiveKeyWithExtra(platform string, extra PoolAccountExtra) string {
	switch c.Type {
	case PoolTypeAPIKey:
		if c.APIKey != "" {
			return c.APIKey
		}
		return c.Token
	case PoolTypeCookie:
		if c.Cookie != "" {
			return c.Cookie
		}
		return c.Token
	case PoolTypeUpstream:
		return c.APIKey
	case PoolTypeOAuth:
		// personalAccessToken 模式：不走 OAuth JSON，直接用 access_token 作 Bearer。
		if platform == PoolPlatformOpenAI && extra.AuthMode == "personalAccessToken" {
			return c.AccessToken
		}
		// openai/codex 平台需要完整 OAuth JSON（含 account_id），交给 codex 适配器解析。
		if platform == PoolPlatformOpenAI {
			b, _ := json.Marshal(map[string]string{
				"access_token":  c.AccessToken,
				"account_id":    c.AccountID,
				"refresh_token": c.RefreshToken,
				"id_token":      c.IDToken,
			})
			return string(b)
		}
		// gemini 平台的 OAuth 账号走 Cloud Code Assist：出站需要 project_id，
		// 且必须与官方 API key 区分开（后者用 ?key=，前者用 Bearer）。
		// 裸 access_token 承载不了这两点，故与 openai 一样透传 JSON。
		if platform == PoolPlatformGemini {
			b, _ := json.Marshal(map[string]string{
				"access_token": c.AccessToken,
				"project_id":   extra.ProjectID,
				"oauth_type":   OAuthTypeCodeAssist,
			})
			return string(b)
		}
		// 其他平台用 access_token 作为 Bearer。
		return c.AccessToken
	}
	return c.Token
}

// ModelMatches 判断账号绑定的模型列表是否包含目标模型。
// models 为空表示不限（返回 true）；匹配采用 trim 后精确比较，不做模糊。
func ModelMatches(modelsCSV, model string) bool {
	if modelsCSV == "" {
		return true
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return true
	}
	for _, m := range strings.Split(modelsCSV, ",") {
		if strings.TrimSpace(m) == model {
			return true
		}
	}
	return false
}
