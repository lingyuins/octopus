package planprovider

import (
	"bytes"
	"context"
	"crypto/rand"
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

// DeepSeek 平台（platform.deepseek.com）控制台 usage API 接入。
//
// 背景：DeepSeek 官方 API（api.deepseek.com）只有余额接口（/user/balance，
// 用 API key 认证），查不到 token 使用量。控制台（platform.deepseek.com）
// 有 usage 接口（/api/v0/usage/by_api_key/amount），按天分桶返回每个 API key
// × 模型的 token 用量，但需要控制台登录态（手机号+密码 → token）。
//
// 本文件实现：
//  1. deepseekPlatformLogin —— 手机号+密码登录控制台换取 token（HTTP 会话）
//  2. ensureDeepSeekSession —— 进程内会话缓存（按 provider ID），过期自动重登
//  3. queryDeepSeekUsage —— 查询官方 usage 并聚合出累计/今日 token 与请求数
//
// 凭据存储复用 sensenova 先例：LoginUsername / LoginPasswordEnc（AES 加密）。
// 与 sensenova 不同：DeepSeek 的 APIKey 字段仍保留 API key（查余额用），
// 登录 token 只放内存缓存，不覆盖 APIKey，两者并存。

// deepseekPlatformLoginURL 登录端点（var 便于 mock 测试）。
var deepseekPlatformLoginURL = "https://platform.deepseek.com/auth-api/v0/users/login"

// deepseekPlatformUsageURL usage 端点（var 便于 mock 测试）。
var deepseekPlatformUsageURL = "https://platform.deepseek.com/api/v0/usage/by_api_key/amount"

// deepseekSessionTTL 控制台 token 在进程内的缓存时长。官方未公开有效期，
// 保守取 6 小时；到期后下次查询自动重新登录（账号密码自动续期）。
const deepseekSessionTTL = 6 * time.Hour

// deepseekLoginCooldown 登录失败后的冷却时长：密码错误/风控期间避免每次
// 轮询都真实登录控制台（ListProviders 轮询间隔可能只有几十秒），防触发风控封号。
const deepseekLoginCooldown = 5 * time.Minute

// deepseekChromeUA 模拟浏览器 UA（控制台接口可能有 UA 校验）。
const deepseekChromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"

// deepseekSession 一次控制台登录会话。
type deepseekSession struct {
	token     string
	expiresAt time.Time
}

// deepseekSessionEntry 带锁的会话缓存条目（按 provider ID 维度缓存）。
type deepseekSessionEntry struct {
	mu sync.Mutex
	s  *deepseekSession
	// nextRetry 登录失败后的冷却截止时间；在此时间前不重试真实登录，
	// 直接返回上次错误，避免轮询触发风控。
	nextRetry time.Time
}

var deepseekSessionCache sync.Map // providerID(int) → *deepseekSessionEntry

// deepseekPlatformLogin 账号+密码登录 DeepSeek 控制台。
//
// 请求体对齐 HAR 抓包：email/mobile 二选一（含 @ 视为邮箱，否则手机号）、
// password、area_code +86、os web。服务端强制要求 device_id 字段（Pydantic
// 校验存在性），但**不校验真实性**——实测随机 UUID 即可通过（不需要浏览器
// 生成的加密风控指纹），因此纯后端登录可行。
func deepseekPlatformLogin(ctx context.Context, username, password string) (string, error) {
	email, mobile := "", username
	if strings.Contains(username, "@") {
		email, mobile = username, ""
	}
	body, err := json.Marshal(map[string]string{
		"email":     email,
		"mobile":    mobile,
		"password":  password,
		"area_code": "+86",
		"os":        "web",
		"device_id": randomUUID(),
	})
	if err != nil {
		return "", fmt.Errorf("deepseek_login: marshal login body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deepseekPlatformLoginURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("deepseek_login: create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", deepseekChromeUA)
	req.Header.Set("Referer", "https://platform.deepseek.com/")
	req.Header.Set("Origin", "https://platform.deepseek.com")

	resp, err := (&http.Client{Timeout: requestTimeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("deepseek_login: login request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("deepseek_login: read login response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := strings.TrimSpace(string(respBody))
		if len(detail) > 200 {
			detail = detail[:200]
		}
		return "", fmt.Errorf("deepseek_login: login http %d（手机号或密码错误，或账号被风控）: %s", resp.StatusCode, detail)
	}

	// 响应结构（与 oauth/get_token 一致）：{"code":0,"msg":"","data":{"biz_code":0,"biz_msg":"","biz_data":{"token":"..."}}}
	var parsed struct {
		Code int `json:"code"`
		Data struct {
			BizData struct {
				Token string `json:"token"`
			} `json:"biz_data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("deepseek_login: parse login response: %w", err)
	}
	if parsed.Code != 0 || parsed.Data.BizData.Token == "" {
		return "", fmt.Errorf("deepseek_login: 登录失败（code=%d）：%s", parsed.Code, strings.TrimSpace(string(respBody)))
	}
	return parsed.Data.BizData.Token, nil
}

// ensureDeepSeekSession 返回有效的控制台 token：优先用进程内缓存，
// 缓存过期或缺失时用账号密码重新登录。凭据解密失败或登录失败返回错误。
func ensureDeepSeekSession(ctx context.Context, provider *model.PlanProvider) (string, error) {
	if provider.LoginUsername == "" || provider.LoginPasswordEnc == "" {
		return "", fmt.Errorf("deepseek: 未配置控制台账号密码，无法查询官方用量")
	}
	entryI, _ := deepseekSessionCache.LoadOrStore(provider.ID, &deepseekSessionEntry{})
	entry := entryI.(*deepseekSessionEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()
	if entry.s != nil && now.Before(entry.s.expiresAt) {
		return entry.s.token, nil
	}
	// 登录失败冷却期内不重试，直接返回上次错误（避免轮询触发风控）。
	if now.Before(entry.nextRetry) {
		return "", fmt.Errorf("deepseek: 控制台登录失败，冷却中（稍后自动重试）")
	}

	pw, err := crypto.Decrypt(provider.LoginPasswordEnc)
	if err != nil || pw == "" {
		return "", fmt.Errorf("deepseek: 登录密码解密失败")
	}
	token, err := deepseekPlatformLogin(ctx, provider.LoginUsername, pw)
	if err != nil {
		// ctx 取消（用户刷新/离开页面中断 ListProviders）不是登录失败，
		// 不触发冷却，否则一次页面刷新就让官方用量静默降级 5 分钟。
		if !errors.Is(err, context.Canceled) {
			entry.nextRetry = now.Add(deepseekLoginCooldown)
		}
		return "", err
	}
	entry.s = &deepseekSession{token: token, expiresAt: now.Add(deepseekSessionTTL)}
	return token, nil
}

// clearDeepSeekSession 清除指定 provider 的会话缓存（凭据变更/删除时调用）。
func clearDeepSeekSession(providerID int) {
	deepseekSessionCache.Delete(providerID)
}

// deepseekUsageResult 聚合后的官方用量。
type deepseekUsageResult struct {
	totalRequests int64
	totalTokens   int64
	todayRequests int64
	todayTokens   int64
}

// queryDeepSeekUsage 查询官方 usage 并聚合累计/今日 token 与请求数。
//
// 累计窗口取 30 天（与平台控制台默认一致；HAR 抓包亦为 30 天窗口）。
// 注意：start/end 必须对齐 UTC 零点（HH:00 对齐会返回 INVALID_PARAM），
// 今日 = 本地时区今天 0 点之后的 bucket。
func queryDeepSeekUsage(ctx context.Context, token string, now time.Time) (*deepseekUsageResult, error) {
	// 对齐 UTC 零点（实测服务端要求，HAR 抓包亦为 UTC 零点）。
	utcNow := now.UTC()
	end := time.Date(utcNow.Year(), utcNow.Month(), utcNow.Day(), 0, 0, 0, 0, time.UTC).Unix()
	start := end - 30*24*3600
	tz := int64(0)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, deepseekPlatformUsageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("deepseek_usage: create request: %w", err)
	}
	q := req.URL.Query()
	q.Set("start", fmt.Sprintf("%d", start))
	q.Set("end", fmt.Sprintf("%d", end))
	q.Set("tz", fmt.Sprintf("%d", tz))
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", deepseekChromeUA)
	req.Header.Set("Referer", "https://platform.deepseek.com/usage")
	// x-client-* 头对齐 HAR 抓包（部分接口可能校验 bundle 标识）。
	req.Header.Set("x-client-bundle-id", "com.deepseek.chat")
	req.Header.Set("x-client-locale", "zh_CN")
	req.Header.Set("x-client-platform", "web")
	req.Header.Set("x-client-timezone-offset", "28800")
	req.Header.Set("x-client-version", "1.0.0")

	resp, err := (&http.Client{Timeout: requestTimeout}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("deepseek_usage: request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("deepseek_usage: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 401 表示会话失效，调用方可据此清理缓存触发重登。
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, errDeepSeekTokenInvalid
		}
		return nil, fmt.Errorf("deepseek_usage: http %d", resp.StatusCode)
	}

	var parsed struct {
		Code int `json:"code"`
		Data struct {
			BizData struct {
				Start  int64 `json:"start"`
				End    int64 `json:"end"`
				Bucket int64 `json:"bucket"`
				Series []struct {
					Model   string `json:"model"`
					Buckets []struct {
						Time  int64 `json:"time"`
						Usage struct {
							ResponseToken   int64 `json:"RESPONSE_TOKEN"`
							Request         int64 `json:"REQUEST"`
							PromptCacheHit  int64 `json:"PROMPT_CACHE_HIT_TOKEN"`
							PromptCacheMiss int64 `json:"PROMPT_CACHE_MISS_TOKEN"`
						} `json:"usage"`
					} `json:"buckets"`
				} `json:"series"`
			} `json:"biz_data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("deepseek_usage: parse response: %w", err)
	}
	if parsed.Code != 0 {
		return nil, fmt.Errorf("deepseek_usage: API code=%d", parsed.Code)
	}

	// 今日起点（本地时区 0 点）。
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	result := &deepseekUsageResult{}
	for _, series := range parsed.Data.BizData.Series {
		for _, bucket := range series.Buckets {
			tokens := bucket.Usage.ResponseToken + bucket.Usage.PromptCacheHit + bucket.Usage.PromptCacheMiss
			result.totalTokens += tokens
			result.totalRequests += bucket.Usage.Request
			if bucket.Time >= todayStart {
				result.todayTokens += tokens
				result.todayRequests += bucket.Usage.Request
			}
		}
	}
	return result, nil
}

// errDeepSeekTokenInvalid 表示控制台会话失效（401），需重新登录。
var errDeepSeekTokenInvalid = errors.New("deepseek usage token invalid")

// queryDeepSeekOfficialUsage 供 ListProviders 使用：确保会话 → 查询官方用量。
// 登录/查询失败返回 (nil, err)，调用方回退本地 stats。
func queryDeepSeekOfficialUsage(ctx context.Context, provider *model.PlanProvider) (*deepseekUsageResult, error) {
	if provider.LoginUsername == "" || provider.LoginPasswordEnc == "" {
		return nil, fmt.Errorf("deepseek: 未配置控制台账号密码")
	}
	token, err := ensureDeepSeekSession(ctx, provider)
	if err != nil {
		return nil, err
	}
	result, err := queryDeepSeekUsage(ctx, token, time.Now())
	if err != nil {
		// 会话失效：清缓存重登一次，仍失败则报错。
		if errors.Is(err, errDeepSeekTokenInvalid) {
			clearDeepSeekSession(provider.ID)
			if token2, err2 := ensureDeepSeekSession(ctx, provider); err2 == nil {
				return queryDeepSeekUsage(ctx, token2, time.Now())
			}
		}
		return nil, err
	}
	return result, nil
}

// logDeepSeekUsageErr 记录官方 usage 查询失败（供 ListProviders 调用，避免日志刷屏）。
func logDeepSeekUsageErr(providerID int, err error) {
	log.Warnf("deepseek provider %d: official usage query failed, fallback to local stats: %v", providerID, err)
}

// randomUUID 生成随机 UUID v4（用作 DeepSeek 登录的 device_id）。
// 服务端只校验字段存在，不校验真实性，随机值即可。
func randomUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// 熵源失败时回退时间戳伪 UUID（登录仍可继续）。
		return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().UnixMilli())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
