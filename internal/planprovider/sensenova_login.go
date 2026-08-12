package planprovider

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/crypto"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// --- SenseNova (商汤日日新) 账号密码自动登录 ---
//
// 平台登录是标准 OIDC + PKCE 授权码流程（无验证码、无 Cookie 状态）：
//
//  1. GET platform.sensenova.cn/oauth2/auth  （携带 PKCE code_challenge）
//     → 302 → iam.sensecoreapi.cn/iam/authn/v1/auth/login?login_challenge=...
//     → 302 → platform.sensenova.cn/login?login_challenge=...（登录页，跟随后解析 login_challenge）
//  2. GET signin.sensecore.cn/.well-known/jwks.json → RSA 公钥（kid=public:hydra.openid.id-token）
//  3. POST iam.sensecoreapi.cn/iam/authn/v1/auth/nova/login
//     body: {"username":"...","password":"<JWE:RSA-OAEP+A256GCM>","challenge":"<login_challenge>","is_encrypt":true}
//     → {"redirect":"https://platform.sensenova.cn/oauth2/auth?...&login_verifier=..."}
//  4. 跟随 redirect（login_verifier → consent 自动通过 → 302 带回 code）
//  5. POST platform.sensenova.cn/oauth2/token（PKCE code_verifier 换 access_token + refresh_token）
//
// access_token 有效期约 3 小时；refresh_token（offline_access scope）可长期续期；
// 两者都失效时用账号密码重新登录。

var (
	senseNovaOAuthAuthURL  = "https://platform.sensenova.cn/oauth2/auth"
	senseNovaOAuthTokenURL = "https://platform.sensenova.cn/oauth2/token"
	senseNovaIAMBaseURL    = "https://iam.sensecoreapi.cn/iam/authn/v1/auth"
	senseNovaJWKSURL       = "https://signin.sensecore.cn/.well-known/jwks.json"
)

const (
	senseNovaClientID    = "nova"
	senseNovaRedirectURI = "https://platform.sensenova.cn"
	senseNovaScope       = "openid offline offline_access"
	// senseNovaTokenTTL 默认 access_token 有效期（oauth2/token 响应无 expires_in 时兜底）
	senseNovaTokenTTL = 3 * time.Hour
	// senseNovaExpirySkew 提前刷新余量
	senseNovaExpirySkew = 5 * time.Minute
)

// errSenseNovaTokenInvalid 表示凭据本身失效（refresh_token 过期/被吊销/密码错误），
// 区别于网络抖动等临时故障：仅凭据失效时才降级走账号密码重新登录。
var errSenseNovaTokenInvalid = errors.New("sensenova token invalid")

var senseNovaChromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"

// senseNovaTokenInvalid 包装：凭据失效（HTTP 400/401）时由 token 请求返回。
func senseNovaTokenInvalid(err error) error {
	return fmt.Errorf("%w: %v", errSenseNovaTokenInvalid, err)
}

// senseNovaSession 一次 OIDC 会话结果
type senseNovaSession struct {
	accessToken  string
	refreshToken string
	expiresAt    time.Time
}

// senseNovaSessionEntry 带锁的会话缓存条目（按 provider ID 维度缓存，
// 避免并发刷新时重复登录/重复续期）。
type senseNovaSessionEntry struct {
	mu sync.Mutex
	s  *senseNovaSession
}

var senseNovaSessionCache sync.Map // providerID(int) → *senseNovaSessionEntry

// jwks 公钥缓存（公钥基本不变，1 小时刷新一次）
var (
	senseNovaJWKSMu      sync.Mutex
	senseNovaJWKSKey     *rsa.PublicKey
	senseNovaJWKSFetched time.Time
)

// senseNovaOIDCLogin 用账号密码完成完整 OIDC 登录流程，返回 access_token / refresh_token。
func senseNovaOIDCLogin(ctx context.Context, username, password string) (*senseNovaSession, error) {
	// 1. PKCE 参数
	codeVerifier, err := randomURLSafeString(64)
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: generate code_verifier: %w", err)
	}
	state, err := randomURLSafeString(32)
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: generate state: %w", err)
	}
	sum := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(sum[:])

	// 登录页会下发含 CSRF 值的 session cookie，授权码回跳（login_verifier →
	// consent → code）必须携带它，否则报 "No CSRF value available in the
	// session cookie"。用 cookie jar 自动保存/按域发送。
	client := &http.Client{
		Timeout: requestTimeout,
		Jar:     mustNewCookieJar(),
	}
	// 2. 打开授权页（跟随重定向到登录页，解析 login_challenge）
	authURL := fmt.Sprintf("%s?response_type=code&client_id=%s&code_challenge_method=S256&code_challenge=%s&redirect_uri=%s&scope=%s&state=%s&lang=zh-CN",
		senseNovaOAuthAuthURL, senseNovaClientID, url.QueryEscape(codeChallenge),
		url.QueryEscape(senseNovaRedirectURI), url.QueryEscape(senseNovaScope), url.QueryEscape(state))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: create auth request: %w", err)
	}
	req.Header.Set("User-Agent", senseNovaChromeUA)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: open oauth2/auth: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	finalURL := resp.Request.URL.String()
	loginChallenge := urlQueryParam(finalURL, "login_challenge")
	if loginChallenge == "" {
		return nil, fmt.Errorf("sensenova_login: 无法从重定向解析 login_challenge (final=%s)", finalURL)
	}

	// 3. 校验挑战（可选但更稳）
	if !senseNovaCheckChallenge(ctx, client, loginChallenge) {
		return nil, fmt.Errorf("sensenova_login: login_challenge 校验失败")
	}

	// 4. 获取 RSA 公钥并加密密码
	pubKey, err := senseNovaFetchJWKS(ctx, client)
	if err != nil {
		return nil, err
	}
	encPassword, err := senseNovaJWEEncrypt(pubKey, password)
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: encrypt password: %w", err)
	}

	// 5. 提交密码登录
	loginBody, err := json.Marshal(map[string]any{
		"username":   username,
		"password":   encPassword,
		"challenge":  loginChallenge,
		"is_encrypt": true,
	})
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: marshal login body: %w", err)
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, senseNovaIAMBaseURL+"/nova/login", strings.NewReader(string(loginBody)))
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", senseNovaChromeUA)
	req.Header.Set("Referer", "https://platform.sensenova.cn/login")
	loginResp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: nova/login: %w", err)
	}
	loginRespBody, err := io.ReadAll(loginResp.Body)
	loginResp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: read login response: %w", err)
	}
	if loginResp.StatusCode < 200 || loginResp.StatusCode >= 300 {
		// 透出服务端错误详情（如 "Wrong password"），仅取 message 字段、限长，
		// 不包含 token 等敏感信息。
		detail := ""
		var errResp struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(loginRespBody, &errResp) == nil && errResp.Message != "" {
			msg := strings.TrimSpace(errResp.Message)
			if len(msg) > 200 {
				msg = msg[:200]
			}
			detail = ": " + msg
		}
		return nil, fmt.Errorf("sensenova_login: nova/login http %d%s（账号或密码错误，或账号被风控）", loginResp.StatusCode, detail)
	}
	var loginResult struct {
		Redirect string `json:"redirect"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(loginRespBody, &loginResult); err != nil {
		return nil, fmt.Errorf("sensenova_login: parse login response: %w", err)
	}
	if loginResult.Error != "" || loginResult.Redirect == "" {
		return nil, fmt.Errorf("sensenova_login: 登录失败: %s", loginResult.Error)
	}

	// 6. 跟随 redirect（login_verifier → consent → code），最终落在 platform.sensenova.cn/?code=...
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, loginResult.Redirect, nil)
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: create redirect request: %w", err)
	}
	req.Header.Set("User-Agent", senseNovaChromeUA)
	callbackResp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: follow login redirect: %w", err)
	}
	defer callbackResp.Body.Close()
	io.Copy(io.Discard, callbackResp.Body)

	callbackURL := callbackResp.Request.URL.String()
	code := urlQueryParam(callbackURL, "code")
	if code == "" {
		return nil, fmt.Errorf("sensenova_login: 未从回调解析到 code (final=%s)", callbackURL)
	}
	if got := urlQueryParam(callbackURL, "state"); got != "" && got != state {
		return nil, fmt.Errorf("sensenova_login: state 不匹配")
	}

	// 7. 用授权码换 token
	return senseNovaExchangeCode(ctx, client, code, codeVerifier, state)
}

// mustNewCookieJar 创建 cookie jar；失败时 panic（正常环境不会失败）。
func mustNewCookieJar() http.CookieJar {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(fmt.Sprintf("sensenova_login: create cookie jar: %v", err))
	}
	return jar
}

// senseNovaExchangeCode 用授权码换取 access_token / refresh_token。
func senseNovaExchangeCode(ctx context.Context, client *http.Client, code, codeVerifier, state string) (*senseNovaSession, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("redirect_uri", senseNovaRedirectURI)
	form.Set("code_verifier", codeVerifier)
	form.Set("state", state)
	form.Set("client_id", senseNovaClientID)
	form.Set("grant_type", "authorization_code")
	return senseNovaTokenRequest(ctx, client, form)
}

// senseNovaRefreshAccessToken 用 refresh_token 续期 access_token。
func senseNovaRefreshAccessToken(ctx context.Context, refreshToken string) (*senseNovaSession, error) {
	client := &http.Client{Timeout: requestTimeout}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", senseNovaClientID)
	return senseNovaTokenRequest(ctx, client, form)
}

// senseNovaTokenRequest 提交 oauth2/token 表单并解析会话。
func senseNovaTokenRequest(ctx context.Context, client *http.Client, form url.Values) (*senseNovaSession, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, senseNovaOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", senseNovaChromeUA)
	req.Header.Set("Referer", senseNovaRedirectURI)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: oauth2/token: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: read token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 400/401 表示凭据失效（refresh_token 过期/被吊销），区别于网络等临时故障。
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
			return nil, senseNovaTokenInvalid(fmt.Errorf("oauth2/token http %d", resp.StatusCode))
		}
		return nil, fmt.Errorf("sensenova_login: oauth2/token http %d", resp.StatusCode)
	}
	var data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("sensenova_login: parse token response: %w", err)
	}
	if data.AccessToken == "" {
		return nil, fmt.Errorf("sensenova_login: token 响应缺少 access_token")
	}
	ttl := senseNovaTokenTTL
	if data.ExpiresIn > 0 {
		ttl = time.Duration(data.ExpiresIn) * time.Second
	}
	return &senseNovaSession{
		accessToken:  data.AccessToken,
		refreshToken: data.RefreshToken,
		expiresAt:    time.Now().Add(ttl),
	}, nil
}

// senseNovaCheckChallenge 校验 login_challenge 是否有效。
func senseNovaCheckChallenge(ctx context.Context, client *http.Client, challenge string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		senseNovaIAMBaseURL+"/checkChallenge?challenge="+url.QueryEscape(challenge), nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", senseNovaChromeUA)
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}
	var data struct {
		IsValid bool `json:"is_valid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return false
	}
	return data.IsValid
}

// senseNovaFetchJWKS 获取密码加密用的 RSA 公钥（kid=public:hydra.openid.id-token）。
// 公钥缓存 1 小时；网络请求在锁外执行（并发重复拉取无害，幂等）。
func senseNovaFetchJWKS(ctx context.Context, client *http.Client) (*rsa.PublicKey, error) {
	senseNovaJWKSMu.Lock()
	cached, fetched := senseNovaJWKSKey, senseNovaJWKSFetched
	senseNovaJWKSMu.Unlock()
	if cached != nil && time.Since(fetched) < time.Hour {
		return cached, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, senseNovaJWKSURL, nil)
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: create jwks request: %w", err)
	}
	req.Header.Set("User-Agent", senseNovaChromeUA)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("sensenova_login: read jwks: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sensenova_login: jwks http %d", resp.StatusCode)
	}
	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("sensenova_login: parse jwks: %w", err)
	}
	for _, k := range jwks.Keys {
		if k.Kty == "RSA" && k.Kid == "public:hydra.openid.id-token" && k.N != "" {
			nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
			if err != nil {
				return nil, fmt.Errorf("sensenova_login: decode jwks n: %w", err)
			}
			eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
			if err != nil {
				return nil, fmt.Errorf("sensenova_login: decode jwks e: %w", err)
			}
			e := 0
			for _, b := range eBytes {
				e = e<<8 | int(b)
			}
			if e <= 0 {
				return nil, fmt.Errorf("sensenova_login: invalid jwks exponent")
			}
			pub := &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}
			senseNovaJWKSMu.Lock()
			senseNovaJWKSKey = pub
			senseNovaJWKSFetched = time.Now()
			senseNovaJWKSMu.Unlock()
			return pub, nil
		}
	}
	return nil, fmt.Errorf("sensenova_login: jwks 中未找到加密公钥")
}

// senseNovaJWEEncrypt 构造 JWE compact 序列化（alg=RSA-OAEP, enc=A256GCM）。
// 与前端 jose 库行为一致：CEK 为 32 字节随机值，用 RSA-OAEP(SHA-1) 加密
// （RFC 7518 规定 alg "RSA-OAEP" 使用 SHA-1，"RSA-OAEP-256" 才是 SHA-256）；
// 明文用 AES-256-GCM 加密，AAD 为 base64url(protected header)。
// 输出格式：b64url(header).b64url(encryptedCEK).b64url(iv).b64url(ciphertext).b64url(tag)
func senseNovaJWEEncrypt(pub *rsa.PublicKey, plaintext string) (string, error) {
	header := `{"alg":"RSA-OAEP","enc":"A256GCM"}`
	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(header))

	// CEK：AES-256 密钥
	cek := make([]byte, 32)
	if _, err := rand.Read(cek); err != nil {
		return "", err
	}
	// 用 RSA-OAEP(SHA-1) 加密 CEK（RFC 7518 alg=RSA-OAEP → SHA-1）
	encCEK, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, pub, cek, nil)
	if err != nil {
		return "", err
	}
	// AES-256-GCM 加密明文，AAD = base64url(protected header)
	block, err := aes.NewCipher(cek)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, iv, []byte(plaintext), []byte(headerB64))
	ct, tag := sealed[:len(sealed)-gcm.Overhead()], sealed[len(sealed)-gcm.Overhead():]

	return strings.Join([]string{
		headerB64,
		base64.RawURLEncoding.EncodeToString(encCEK),
		base64.RawURLEncoding.EncodeToString(iv),
		base64.RawURLEncoding.EncodeToString(ct),
		base64.RawURLEncoding.EncodeToString(tag),
	}, "."), nil
}

// decodeSenseNovaTokenExpiry 解析 access_token（JWT）的 exp 字段。
func decodeSenseNovaTokenExpiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return time.Time{}, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		if decoded, err = base64.URLEncoding.DecodeString(parts[1]); err != nil {
			return time.Time{}, false
		}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}

// ensureSenseNovaSession 确保商汤套餐有可用的 access_token：
//  1. 进程内缓存未过期 → 直接用；
//  2. 缓存 miss 但 DB 中 APIKey（access_token）未过期 → 用之；
//  3. 有 refresh_token → 续期；
//  4. 否则用账号密码重新登录。
//
// 成功后会把新 token 写回 provider 并持久化到 DB。
func ensureSenseNovaSession(ctx context.Context, provider *model.PlanProvider) (string, error) {
	entryI, _ := senseNovaSessionCache.LoadOrStore(provider.ID, &senseNovaSessionEntry{})
	entry := entryI.(*senseNovaSessionEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()
	// 1. 缓存有效
	if entry.s != nil && now.Before(entry.s.expiresAt.Add(-senseNovaExpirySkew)) {
		return entry.s.accessToken, nil
	}
	// 2. DB 里的 token 未过期
	if provider.APIKey != "" {
		if exp, ok := decodeSenseNovaTokenExpiry(provider.APIKey); ok && now.Before(exp.Add(-senseNovaExpirySkew)) {
			entry.s = &senseNovaSession{accessToken: provider.APIKey, expiresAt: exp}
			return provider.APIKey, nil
		}
	}
	// 3. refresh_token 续期；仅凭据失效（4xx）才降级账号密码重新登录，
	// 网络抖动等临时故障直接返回错误，避免反复触发完整 OIDC 登录被风控。
	if provider.RefreshTokenEnc != "" {
		if rt, err := crypto.Decrypt(provider.RefreshTokenEnc); err == nil && rt != "" {
			if s, err := senseNovaRefreshAccessToken(ctx, rt); err == nil {
				senseNovaPersistSession(ctx, provider, s)
				entry.s = s
				return s.accessToken, nil
			} else if !errors.Is(err, errSenseNovaTokenInvalid) {
				return "", fmt.Errorf("sensenova_plan: refresh token 续期失败（临时故障，稍后自动重试）: %w", err)
			}
			// refresh_token 失效：继续走账号密码重新登录
		}
	}
	// 4. 账号密码登录
	if provider.LoginUsername == "" || provider.LoginPasswordEnc == "" {
		return "", fmt.Errorf("sensenova_plan: 控制台凭据已失效且未配置账号密码，请在套餐管理里更新凭据")
	}
	pw, err := crypto.Decrypt(provider.LoginPasswordEnc)
	if err != nil || pw == "" {
		return "", fmt.Errorf("sensenova_plan: 登录密码解密失败")
	}
	s, err := senseNovaOIDCLogin(ctx, provider.LoginUsername, pw)
	if err != nil {
		return "", err
	}
	senseNovaPersistSession(ctx, provider, s)
	entry.s = s
	return s.accessToken, nil
}

// senseNovaPersistSession 把新会话写回 provider 内存对象与 DB（含加密 refresh_token）。
// 写库前校验账号密码凭据未被并发替换（换凭据会清空缓存并更新 login 字段），
// 避免旧会话覆盖新凭据写入的 refresh_token。
func senseNovaPersistSession(ctx context.Context, provider *model.PlanProvider, s *senseNovaSession) {
	provider.APIKey = s.accessToken
	updates := map[string]any{
		"updated_at": time.Now(),
	}
	// access_token 加密落库（api_key 字段）。
	if enc, err := crypto.Encrypt(s.accessToken); err == nil {
		updates["api_key"] = enc
	} else {
		log.Warnf("planprovider: sensenova persist session: encrypt access token failed: %v", err)
		return
	}
	if s.refreshToken != "" {
		if enc, err := crypto.Encrypt(s.refreshToken); err == nil {
			provider.RefreshTokenEnc = enc
			updates["refresh_token_enc"] = enc
		}
	}
	if provider.ID <= 0 {
		return // AddProvider 阶段尚未落库，由调用方持久化
	}
	var cur model.PlanProvider
	if err := db.GetDB().WithContext(ctx).Select("login_username", "login_password_enc").First(&cur, provider.ID).Error; err != nil {
		log.Warnf("planprovider: sensenova persist session: load provider %d: %v", provider.ID, err)
		return
	}
	if cur.LoginUsername != provider.LoginUsername || cur.LoginPasswordEnc != provider.LoginPasswordEnc {
		log.Warnf("planprovider: sensenova persist session: provider %d 登录凭据已被并发替换，丢弃本次续期的 token", provider.ID)
		return
	}
	if err := db.GetDB().WithContext(ctx).Model(&model.PlanProvider{}).
		Where("id = ?", provider.ID).Updates(updates).Error; err != nil {
		log.Warnf("planprovider: sensenova persist session: update provider %d: %v", provider.ID, err)
	}
}

// clearSenseNovaSession 清除指定 provider 的会话缓存（删除/换凭据时调用）。
func clearSenseNovaSession(providerID int) {
	senseNovaSessionCache.Delete(providerID)
}

// randomURLSafeString 生成 URL 安全随机字符串（PKCE verifier / state）。
func randomURLSafeString(n int) (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf), nil
}

// urlQueryParam 从 URL 中解析查询参数。
func urlQueryParam(rawURL, key string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Query().Get(key)
}

// pemToRSAPublicKey 解析 PEM 公钥（测试辅助）。
func pemToRSAPublicKey(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not RSA public key")
	}
	return pub, nil
}
