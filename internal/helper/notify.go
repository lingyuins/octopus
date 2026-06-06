package helper

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

func notifyHTTPClient() *http.Client {
	timeout := 10 * time.Second
	if v, err := setting.GetInt(model.SettingKeyNotifyHTTPTimeoutSeconds); err == nil && v > 0 {
		timeout = time.Duration(v) * time.Second
	}
	return &http.Client{Timeout: timeout}
}

// AlertWebhookPayload is the JSON body sent to webhook endpoints on alert state changes.
type AlertWebhookPayload struct {
	RuleID        int                          `json:"rule_id"`
	RuleName      string                       `json:"rule_name"`
	ConditionType model.AlertRuleConditionType `json:"condition_type"`
	State         string                       `json:"state"`
	Message       string                       `json:"message"`
	Threshold     float64                      `json:"threshold"`
	CurrentValue  float64                      `json:"current_value"`
	Time          string                       `json:"time"`
}

// SendWebhook sends an alert notification to the configured webhook URL.
func SendWebhook(channel *model.AlertNotifChannel, payload AlertWebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequest("POST", channel.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if channel.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+channel.Secret)
	}
	if channel.Headers != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(channel.Headers), &headers); err == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	}

	client := notifyHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook responded %d", resp.StatusCode)
	}
	return nil
}

// SendGotify sends an alert notification to a Gotify server.
func SendGotify(channel *model.AlertNotifChannel, payload AlertWebhookPayload) error {
	var cfg model.GotifyConfig
	if channel.Config != "" {
		if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
			return fmt.Errorf("parse gotify config: %w", err)
		}
	}

	// Fallback: if server_url is empty, use channel.URL; if token is empty, use channel.Secret
	serverURL := cfg.ServerURL
	if serverURL == "" {
		serverURL = strings.TrimRight(channel.URL, "/")
	}
	token := cfg.Token
	if token == "" {
		token = channel.Secret
	}

	if serverURL == "" || token == "" {
		return fmt.Errorf("gotify: server_url and token are required")
	}

	priority := cfg.Priority
	if priority <= 0 {
		priority = 5
	}

	gotifyMsg := map[string]interface{}{
		"title":    fmt.Sprintf("Octopus Alert: %s", payload.RuleName),
		"message":  fmt.Sprintf("%s\n\nCondition: %s\nState: %s\nThreshold: %.2f\nTime: %s", payload.Message, payload.ConditionType, payload.State, payload.Threshold, payload.Time),
		"priority": priority,
	}

	msgBody, err := json.Marshal(gotifyMsg)
	if err != nil {
		return fmt.Errorf("marshal gotify message: %w", err)
	}

	endpoint := fmt.Sprintf("%s/message?token=%s", strings.TrimRight(serverURL, "/"), token)
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(msgBody))
	if err != nil {
		return fmt.Errorf("create gotify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := notifyHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send gotify: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("gotify responded %d", resp.StatusCode)
	}
	return nil
}

// SendEmail sends an alert notification via SMTP email.
func SendEmail(channel *model.AlertNotifChannel, payload AlertWebhookPayload) error {
	var cfg model.EmailConfig
	if channel.Config != "" {
		if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
			return fmt.Errorf("parse email config: %w", err)
		}
	}

	if cfg.SMTPHost == "" || cfg.From == "" || cfg.To == "" {
		return fmt.Errorf("email: smtp_host, from, and to are required")
	}

	port := cfg.SMTPPort
	if port == 0 {
		port = 587
	}

	subject := fmt.Sprintf("Octopus Alert: %s - %s", payload.RuleName, payload.State)
	body := fmt.Sprintf(
		"Rule: %s\nCondition: %s\nState: %s\nMessage: %s\nThreshold: %.2f\nTime: %s\n",
		payload.RuleName, payload.ConditionType, payload.State, payload.Message, payload.Threshold, payload.Time,
	)

	// Build MIME message
	fromHeader := cfg.From
	toAddrs := strings.Split(cfg.To, ",")
	for i, a := range toAddrs {
		toAddrs[i] = strings.TrimSpace(a)
	}

	var msg strings.Builder
	msg.WriteString("From: " + fromHeader + "\r\n")
	msg.WriteString("To: " + cfg.To + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, port)
	useTLS := cfg.UseTLS
	if !useTLS && port == 465 {
		useTLS = true
	}

	if cfg.Username == "" && cfg.Password == "" {
		// No auth - try plain send
		err := smtp.SendMail(addr, nil, fromHeader, toAddrs, []byte(msg.String()))
		if err != nil {
			return fmt.Errorf("send email (no auth): %w", err)
		}
		return nil
	}

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	err := smtp.SendMail(addr, auth, fromHeader, toAddrs, []byte(msg.String()))
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

// buildAlertText builds a plain-text alert message from the payload.
func buildAlertText(payload AlertWebhookPayload) string {
	return fmt.Sprintf("%s\n\nRule: %s\nCondition: %s\nState: %s\nThreshold: %.2f\nCurrent: %.2f\nTime: %s",
		payload.Message, payload.RuleName, payload.ConditionType, payload.State, payload.Threshold, payload.CurrentValue, payload.Time)
}

// SendTelegram sends an alert notification via Telegram Bot API.
func SendTelegram(channel *model.AlertNotifChannel, payload AlertWebhookPayload) error {
	var cfg model.TelegramConfig
	if channel.Config != "" {
		if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
			return fmt.Errorf("parse telegram config: %w", err)
		}
	}
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return fmt.Errorf("telegram: bot_token and chat_id are required")
	}

	text := fmt.Sprintf("🐙 %s\n%s", payload.RuleName, buildAlertText(payload))
	if len(text) > 4096 {
		text = text[:4096]
	}

	body, _ := json.Marshal(map[string]interface{}{
		"chat_id":                  cfg.ChatID,
		"text":                     text,
		"disable_web_page_preview": true,
	})

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := notifyHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("telegram responded %d: %s", resp.StatusCode, string(respBody))
	}

	// Check Telegram API response for errors
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && !result.OK {
		return fmt.Errorf("telegram API error: %s", result.Description)
	}
	return nil
}

// SendFeishu sends an alert notification via Feishu (Lark) webhook.
func SendFeishu(channel *model.AlertNotifChannel, payload AlertWebhookPayload) error {
	var cfg model.FeishuConfig
	if channel.Config != "" {
		if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
			return fmt.Errorf("parse feishu config: %w", err)
		}
	}
	if cfg.WebhookKey == "" {
		return fmt.Errorf("feishu: webhook_key is required")
	}

	text := fmt.Sprintf("🐙 %s\n%s", payload.RuleName, buildAlertText(payload))
	body, _ := json.Marshal(map[string]interface{}{
		"msg_type": "text",
		"content":  map[string]string{"text": text},
	})

	endpoint := fmt.Sprintf("https://open.feishu.cn/open-apis/bot/v2/hook/%s", cfg.WebhookKey)
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create feishu request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := notifyHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send feishu: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code       int `json:"code"`
		StatusCode int `json:"StatusCode"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		if result.Code != 0 && result.StatusCode != 0 {
			return fmt.Errorf("feishu API error: code=%d statusCode=%d", result.Code, result.StatusCode)
		}
	}
	return nil
}

// SendDingTalk sends an alert notification via DingTalk robot webhook.
func SendDingTalk(channel *model.AlertNotifChannel, payload AlertWebhookPayload) error {
	var cfg model.DingTalkConfig
	if channel.Config != "" {
		if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
			return fmt.Errorf("parse dingtalk config: %w", err)
		}
	}
	if cfg.WebhookKey == "" {
		return fmt.Errorf("dingtalk: webhook_key is required")
	}

	endpoint := fmt.Sprintf("https://oapi.dingtalk.com/robot/send?access_token=%s", cfg.WebhookKey)

	// Optional HMAC-SHA256 signature
	if cfg.Secret != "" {
		timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
		stringToSign := timestamp + "\n" + cfg.Secret
		mac := hmac.New(sha256.New, []byte(cfg.Secret))
		mac.Write([]byte(stringToSign))
		sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		endpoint += "&timestamp=" + timestamp + "&sign=" + sign
	}

	text := fmt.Sprintf("🐙 %s\n%s", payload.RuleName, buildAlertText(payload))
	body, _ := json.Marshal(map[string]interface{}{
		"msgtype": "text",
		"text":    map[string]string{"content": text},
		"at":      map[string]interface{}{"isAtAll": false},
	})

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create dingtalk request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")

	client := notifyHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send dingtalk: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode interface{} `json:"errcode"` // can be number or string
		ErrMsg  string      `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		switch v := result.ErrCode.(type) {
		case float64:
			if v != 0 {
				return fmt.Errorf("dingtalk API error: errcode=%v errmsg=%s", v, result.ErrMsg)
			}
		case string:
			if v != "0" && v != "" {
				return fmt.Errorf("dingtalk API error: errcode=%s errmsg=%s", v, result.ErrMsg)
			}
		}
	}
	return nil
}

// SendWeCom sends an alert notification via WeCom (Enterprise WeChat) group robot.
func SendWeCom(channel *model.AlertNotifChannel, payload AlertWebhookPayload) error {
	var cfg model.WeComConfig
	if channel.Config != "" {
		if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
			return fmt.Errorf("parse wecom config: %w", err)
		}
	}
	if cfg.WebhookKey == "" {
		return fmt.Errorf("wecom: webhook_key is required")
	}

	text := fmt.Sprintf("🐙 %s\n%s", payload.RuleName, buildAlertText(payload))
	body, _ := json.Marshal(map[string]interface{}{
		"msgtype": "text",
		"text":    map[string]string{"content": text},
	})

	endpoint := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=%s", cfg.WebhookKey)
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create wecom request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := notifyHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send wecom: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode interface{} `json:"errcode"` // can be number or string
		ErrMsg  string      `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		switch v := result.ErrCode.(type) {
		case float64:
			if v != 0 {
				return fmt.Errorf("wecom API error: errcode=%v errmsg=%s", v, result.ErrMsg)
			}
		case string:
			if v != "0" && v != "" {
				return fmt.Errorf("wecom API error: errcode=%s errmsg=%s", v, result.ErrMsg)
			}
		}
	}
	return nil
}

// SendNtfy sends an alert notification via ntfy push notification service.
func SendNtfy(channel *model.AlertNotifChannel, payload AlertWebhookPayload) error {
	var cfg model.NtfyConfig
	if channel.Config != "" {
		if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
			return fmt.Errorf("parse ntfy config: %w", err)
		}
	}
	if cfg.TopicURL == "" {
		return fmt.Errorf("ntfy: topic_url is required")
	}

	// Resolve topic URL: support full URL, domain+path, or plain topic name
	topicURL := cfg.TopicURL
	if !strings.HasPrefix(topicURL, "http://") && !strings.HasPrefix(topicURL, "https://") {
		if strings.Contains(topicURL, "/") || strings.Contains(topicURL, ".") {
			topicURL = "https://" + topicURL
		} else {
			topicURL = "https://ntfy.sh/" + topicURL
		}
	}

	text := buildAlertText(payload)
	req, err := http.NewRequest("POST", topicURL, strings.NewReader(text))
	if err != nil {
		return fmt.Errorf("create ntfy request: %w", err)
	}

	// Title header: RFC 2047 encode if non-ASCII
	title := fmt.Sprintf("🐙 Octopus Alert: %s", payload.RuleName)
	if needsMimeEncoding(title) {
		title = mime.QEncoding.Encode("utf-8", title)
	}
	req.Header.Set("Title", title)
	req.Header.Set("Priority", "default")
	req.Header.Set("Tags", "bell")

	if cfg.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.AccessToken)
	}

	client := notifyHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send ntfy: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("ntfy responded %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// needsMimeEncoding returns true if s contains non-ASCII characters.
func needsMimeEncoding(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return true
		}
	}
	return false
}

// SendNotification dispatches an alert notification to the appropriate channel based on type.
func SendNotification(channel *model.AlertNotifChannel, payload AlertWebhookPayload) error {
	switch model.AlertNotifChannelType(channel.Type) {
	case model.AlertNotifGotify:
		return SendGotify(channel, payload)
	case model.AlertNotifEmail:
		return SendEmail(channel, payload)
	case model.AlertNotifTelegram:
		return SendTelegram(channel, payload)
	case model.AlertNotifFeishu:
		return SendFeishu(channel, payload)
	case model.AlertNotifDingTalk:
		return SendDingTalk(channel, payload)
	case model.AlertNotifWeCom:
		return SendWeCom(channel, payload)
	case model.AlertNotifNtfy:
		return SendNtfy(channel, payload)
	case model.AlertNotifWebhook:
		fallthrough
	default:
		return SendWebhook(channel, payload)
	}
}

// FormatEmailAddress validates and formats an email address.
func FormatEmailAddress(addr string) (string, error) {
	a, err := mail.ParseAddress(addr)
	if err != nil {
		return "", fmt.Errorf("invalid email address %q: %w", addr, err)
	}
	return a.String(), nil
}
