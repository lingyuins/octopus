package planprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/crypto"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// 基元律动（tokenrhythm.studio）账号密码自动登录。
//
// 背景：基元律动的额度监控用浏览器会话 Cookie 鉴权（tr_session + tr_csrf，
// 见 query.go 的 queryTokenRhythmBalance），手工粘贴的 Cookie 虽然每次请求会被
// 服务端滑动续期（Max-Age 2592000 = 30 天），但会话一旦被服务端判定失效就只能
// 重新粘贴。本文件实现账号密码登录：POST /api/auth/login {account, password}
// 换取新的会话 Cookie，供查询侧自动续期。
//
// 凭据存储复用 sensenova / deepseek 先例：LoginUsername / LoginPasswordEnc
// （AES 加密，不回传前端）。与 deepseek 的区别：登录拿到的 Cookie 就是查询
// 所需的主凭据，因此写入 APIKey 字段落库（对应"账号密码模式"）；
// 用户切回手动模式时清空账号密码字段、继续使用手工粘贴的 Cookie。
//
// 实测（2026-09-12，HAR 抓包 + 服务端直连验证）：
//   - 登录请求体只有 account/password，不需要阿里云人机验证参数；
//   - 登录成功 Set-Cookie: tr_session / tr_csrf（HttpOnly, Secure, 30 天）；
//   - 凭据错误或会话失效均为 HTTP 401，响应体 {"code":"UNAUTHORIZED"}。

// tokenRhythmLoginURL 控制台登录端点（var 便于 mock 测试）。
var tokenRhythmLoginURL = "https://tokenrhythm.studio/api/auth/login"

// tokenRhythmChromeUA 模拟浏览器 UA（控制台接口 / 登录接口共用）。
const tokenRhythmChromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"

// tokenRhythmSessionTTL 进程内会话缓存时长。服务端 Cookie Max-Age 为 30 天
// （每次请求滑动续期），保守取 29 天；到期后下次查询重新登录。
const tokenRhythmSessionTTL = 29 * 24 * time.Hour

// tokenRhythmLoginCooldown 登录失败后的冷却时长：密码错误/风控期间避免每次
// 轮询都真实登录控制台（自动刷新间隔可能只有几十分钟），防触发风控。
const tokenRhythmLoginCooldown = 5 * time.Minute

// errTokenRhythmSessionInvalid 表示控制台会话失效（HTTP 401），
// 调用方据此丢弃会话并重新登录一次。
var errTokenRhythmSessionInvalid = errors.New("tokenrhythm session invalid")

// tokenRhythmSession 一次控制台登录会话。
type tokenRhythmSession struct {
	cookie    string
	expiresAt time.Time
}

// tokenRhythmSessionEntry 带锁的会话缓存条目（按 provider ID 维度缓存）。
type tokenRhythmSessionEntry struct {
	mu      sync.Mutex
	session *tokenRhythmSession
	// nextRetry 登录失败后的冷却截止时间；在此时间前不重试真实登录，
	// 直接返回错误，避免轮询触发风控。
	nextRetry time.Time
}

var tokenRhythmSessionCache sync.Map // providerID(int) → *tokenRhythmSessionEntry

// tokenRhythmLogin 账号+密码登录基元律动控制台，返回 Cookie 请求头值
// （形如 "tr_session=...; tr_csrf=..."）。
func tokenRhythmLogin(ctx context.Context, account, password string) (string, error) {
	body, err := json.Marshal(map[string]string{"account": account, "password": password})
	if err != nil {
		return "", fmt.Errorf("tokenrhythm_login: marshal login body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenRhythmLoginURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("tokenrhythm_login: create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", tokenRhythmChromeUA)
	req.Header.Set("Origin", "https://tokenrhythm.studio")
	req.Header.Set("Referer", "https://tokenrhythm.studio/login")

	resp, err := (&http.Client{Timeout: requestTimeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("tokenrhythm_login: login request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("tokenrhythm_login: read login response: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("tokenrhythm_login: 登录失败（账号或密码错误）: %s", truncateBody(respBody))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("tokenrhythm_login: 登录 http %d: %s", resp.StatusCode, truncateBody(respBody))
	}

	// 响应结构：{"code":0,"message":"ok","data":{"user":{...}}}
	var parsed struct {
		Code    json.RawMessage `json:"code"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("tokenrhythm_login: parse login response: %w", err)
	}
	if !bytes.Equal(bytes.TrimSpace(parsed.Code), []byte("0")) {
		msg := parsed.Message
		if msg == "" {
			msg = truncateBody(respBody)
		}
		return "", fmt.Errorf("tokenrhythm_login: 登录失败: %s", msg)
	}

	cookie := collectTokenRhythmCookie(resp)
	if !strings.Contains(cookie, "tr_session=") {
		return "", fmt.Errorf("tokenrhythm_login: 登录响应未返回 tr_session Cookie")
	}
	return cookie, nil
}

// ensureTokenRhythmSession 返回可用于查询的会话 Cookie：
//  1. 进程内会话缓存有效 → 直接复用；
//  2. provider.APIKey 中已有 Cookie（上次登录写入或手工粘贴）→ 复用，失效时由查询侧触发重登；
//  3. 都没有 → 用账号密码登录换取新 Cookie，并写入 provider.APIKey（由调用方落库）。
//
// 登录失败后进入冷却期，冷却期内不再真实登录（避免轮询触发风控）。
func ensureTokenRhythmSession(ctx context.Context, provider *model.PlanProvider) (string, error) {
	if provider.LoginUsername == "" || provider.LoginPasswordEnc == "" {
		return "", fmt.Errorf("tokenrhythm: 未配置控制台账号密码，无法自动登录")
	}
	entryI, _ := tokenRhythmSessionCache.LoadOrStore(provider.ID, &tokenRhythmSessionEntry{})
	entry := entryI.(*tokenRhythmSessionEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()
	if entry.session != nil && now.Before(entry.session.expiresAt) {
		return entry.session.cookie, nil
	}
	// 已有的 Cookie 先直接用：有效性由查询侧判定（失效时 invalidate + 重登），
	// 避免每次刷新都真实登录控制台。
	if cookie := provider.APIKey; strings.Contains(cookie, "tr_session=") {
		entry.session = &tokenRhythmSession{cookie: cookie, expiresAt: now.Add(tokenRhythmSessionTTL)}
		return cookie, nil
	}
	if now.Before(entry.nextRetry) {
		return "", fmt.Errorf("tokenrhythm: 上次登录失败，冷却中（稍后自动重试）")
	}

	pw, err := crypto.Decrypt(provider.LoginPasswordEnc)
	if err != nil || pw == "" {
		return "", fmt.Errorf("tokenrhythm: 登录密码解密失败")
	}
	cookie, err := tokenRhythmLogin(ctx, provider.LoginUsername, pw)
	if err != nil {
		// ctx 取消（用户刷新/离开页面中断请求）不是登录失败，不触发冷却。
		if !errors.Is(err, context.Canceled) {
			entry.nextRetry = now.Add(tokenRhythmLoginCooldown)
		}
		return "", err
	}
	entry.session = &tokenRhythmSession{cookie: cookie, expiresAt: now.Add(tokenRhythmSessionTTL)}
	entry.nextRetry = time.Time{}
	// 登录结果写入主凭据字段，由调用方落库（重启后无需重新登录）。
	provider.APIKey = cookie
	log.Infof("planprovider: tokenrhythm provider %d 账号密码登录成功，会话 Cookie 已更新", provider.ID)
	return cookie, nil
}

// invalidateTokenRhythmSession 丢弃进程内缓存的会话（会话失效后重登前调用），
// 保留登录失败的冷却计时，避免绕过风控保护。
func invalidateTokenRhythmSession(providerID int) {
	v, ok := tokenRhythmSessionCache.Load(providerID)
	if !ok {
		return
	}
	entry := v.(*tokenRhythmSessionEntry)
	entry.mu.Lock()
	entry.session = nil
	entry.mu.Unlock()
}

// clearTokenRhythmSession 清除指定 provider 的会话缓存（凭据变更/删除时调用）。
func clearTokenRhythmSession(providerID int) {
	tokenRhythmSessionCache.Delete(providerID)
}

// refreshBalanceWithLogin 查询 balance 类厂商余额。
// 基元律动配置了账号密码时：先确保会话有效（必要时自动登录并更新 APIKey 中的
// Cookie），查询遇会话失效（HTTP 401）则丢弃会话重登一次再试。
// 其他厂商（以及未配置账号密码的基元律动）等价于直接 QueryBalance。
func refreshBalanceWithLogin(ctx context.Context, provider *model.PlanProvider) (*BalanceResult, error) {
	if provider.Category != model.PlanProviderTokenRhythm || provider.LoginUsername == "" {
		return QueryBalance(ctx, provider.Category, provider.APIKey, provider.BaseURL)
	}
	if _, err := ensureTokenRhythmSession(ctx, provider); err != nil {
		return nil, err
	}
	result, err := QueryBalance(ctx, provider.Category, provider.APIKey, provider.BaseURL)
	if !errors.Is(err, errTokenRhythmSessionInvalid) {
		return result, err
	}
	log.Warnf("planprovider: tokenrhythm provider %d 会话已失效，重新登录后重试", provider.ID)
	invalidateTokenRhythmSession(provider.ID)
	provider.APIKey = ""
	if _, err := ensureTokenRhythmSession(ctx, provider); err != nil {
		return nil, err
	}
	return QueryBalance(ctx, provider.Category, provider.APIKey, provider.BaseURL)
}

// collectTokenRhythmCookie 从登录响应的 Set-Cookie 中拼出 Cookie 请求头值。
func collectTokenRhythmCookie(resp *http.Response) string {
	parts := make([]string, 0, 3)
	for _, ck := range resp.Cookies() {
		switch ck.Name {
		case "tr_session", "tr_csrf", "tr_ref_device":
			if ck.Value != "" {
				parts = append(parts, ck.Name+"="+ck.Value)
			}
		}
	}
	return strings.Join(parts, "; ")
}

// truncateBody 截断响应体用于错误信息（避免超长/敏感内容刷屏）。
func truncateBody(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
