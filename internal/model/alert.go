package model

// AlertRuleConditionType defines the trigger condition for an alert rule.
type AlertRuleConditionType string

const (
	AlertConditionCostThreshold AlertRuleConditionType = "cost_threshold"
	AlertConditionErrorRate     AlertRuleConditionType = "error_rate"
	AlertConditionQuotaExceeded AlertRuleConditionType = "quota_exceeded"
	AlertConditionChannelDown   AlertRuleConditionType = "channel_down"
)

// AlertRule defines an alert rule with condition, threshold, and notification channel.
type AlertRule struct {
	ID             int                    `json:"id" gorm:"primaryKey"`
	Name           string                 `json:"name" gorm:"not null"`
	Enabled        bool                   `json:"enabled" gorm:"default:true"`
	ConditionType  AlertRuleConditionType `json:"condition_type" gorm:"not null"`
	Threshold      float64                `json:"threshold"`
	ConditionJSON  string                 `json:"condition_json,omitempty"`
	NotifChannelID int                    `json:"notif_channel_id"`
	CooldownSec    int                    `json:"cooldown_sec" gorm:"default:300"`
	ScopeChannelID int                    `json:"scope_channel_id,omitempty"`
	ScopeAPIKeyID  int                    `json:"scope_api_key_id,omitempty"`
}

// AlertNotifChannelType defines the type of a notification channel.
type AlertNotifChannelType string

const (
	AlertNotifWebhook  AlertNotifChannelType = "webhook"
	AlertNotifGotify   AlertNotifChannelType = "gotify"
	AlertNotifEmail    AlertNotifChannelType = "email"
	AlertNotifTelegram AlertNotifChannelType = "telegram"
	AlertNotifFeishu   AlertNotifChannelType = "feishu"
	AlertNotifDingTalk AlertNotifChannelType = "dingtalk"
	AlertNotifWeCom    AlertNotifChannelType = "wecom"
	AlertNotifNtfy     AlertNotifChannelType = "ntfy"
)

// AlertNotifChannel defines a notification channel (webhook, gotify, email, etc.).
type AlertNotifChannel struct {
	ID      int    `json:"id" gorm:"primaryKey"`
	Name    string `json:"name" gorm:"not null"`
	Type    string `json:"type" gorm:"not null;default:'webhook'"`
	URL     string `json:"url"`
	Secret  string `json:"secret,omitempty"`
	Headers string `json:"headers,omitempty"`
	Config  string `json:"config,omitempty"` // JSON blob for type-specific config (gotify token, email SMTP, etc.)
}

// GotifyConfig holds the configuration for a Gotify notification channel.
type GotifyConfig struct {
	ServerURL string `json:"server_url"`         // e.g. https://gotify.example.com
	Token     string `json:"token"`              // application token
	Priority  int    `json:"priority,omitempty"` // message priority (1-10, default 5)
}

// EmailConfig holds the configuration for an Email notification channel.
type EmailConfig struct {
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"` // default 587
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`    // sender address
	To       string `json:"to"`      // comma-separated recipient addresses
	UseTLS   bool   `json:"use_tls"` // default true
}

// TelegramConfig holds the configuration for a Telegram notification channel.
type TelegramConfig struct {
	BotToken string `json:"bot_token"` // Telegram Bot API token
	ChatID   string `json:"chat_id"`   // target chat ID
}

// FeishuConfig holds the configuration for a Feishu (Lark) notification channel.
type FeishuConfig struct {
	WebhookKey string `json:"webhook_key"` // bot webhook key
}

// DingTalkConfig holds the configuration for a DingTalk notification channel.
type DingTalkConfig struct {
	WebhookKey string `json:"webhook_key"`      // robot access_token
	Secret     string `json:"secret,omitempty"` // optional HMAC-SHA256 signing secret
}

// WeComConfig holds the configuration for a WeCom (Enterprise WeChat) notification channel.
type WeComConfig struct {
	WebhookKey string `json:"webhook_key"` // group robot key
}

// NtfyConfig holds the configuration for an ntfy push notification channel.
type NtfyConfig struct {
	TopicURL    string `json:"topic_url"`              // e.g. "https://ntfy.sh/mytopic" or just "mytopic"
	AccessToken string `json:"access_token,omitempty"` // optional Bearer token
}

// AlertState represents the current firing state of an alert rule.
type AlertState int

const (
	AlertStateOK       AlertState = 0
	AlertStateFiring   AlertState = 1
	AlertStateResolved AlertState = 2
)

// AlertStateRecord tracks the current state of an alert rule.
type AlertStateRecord struct {
	RuleID         int        `json:"rule_id" gorm:"primaryKey"`
	State          AlertState `json:"state"`
	LastFiredAt    int64      `json:"last_fired_at"`
	LastResolvedAt int64      `json:"last_resolved_at"`
	LastCheckedAt  int64      `json:"last_checked_at"`
	FiredCount     int64      `json:"fired_count"`
}

// AlertHistory records a single alert event.
type AlertHistory struct {
	ID         int64      `json:"id" gorm:"primaryKey"`
	RuleID     int        `json:"rule_id"`
	RuleName   string     `json:"rule_name"`
	State      AlertState `json:"state"`
	Message    string     `json:"message"`
	DetailJSON string     `json:"detail_json,omitempty"`
	Time       int64      `json:"time" gorm:"autoCreateTime:milli"`
}
