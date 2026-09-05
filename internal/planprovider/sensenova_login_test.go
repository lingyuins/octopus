package planprovider

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

// makeTestRSAKey 生成测试用 RSA 密钥对。
func makeTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return key
}

// jweDecryptRaw 解密 JWE（返回错误而非 Fatalf，供 mock handler 使用）。
func jweDecryptRaw(priv *rsa.PrivateKey, compact string) (string, error) {
	parts := strings.Split(compact, ".")
	if len(parts) != 5 {
		return "", fmt.Errorf("JWE 分段数 = %d, want 5", len(parts))
	}
	headerB64, encCEKB64, ivB64, ctB64, tagB64 := parts[0], parts[1], parts[2], parts[3], parts[4]
	if headerB64 != base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RSA-OAEP","enc":"A256GCM"}`)) {
		return "", fmt.Errorf("protected header = %q", headerB64)
	}
	encCEK, err := base64.RawURLEncoding.DecodeString(encCEKB64)
	if err != nil {
		return "", err
	}
	cek, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, priv, encCEK, nil)
	if err != nil {
		return "", err
	}
	iv, _ := base64.RawURLEncoding.DecodeString(ivB64)
	ct, _ := base64.RawURLEncoding.DecodeString(ctB64)
	tag, _ := base64.RawURLEncoding.DecodeString(tagB64)
	block, err := aes.NewCipher(cek)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plain, err := gcm.Open(nil, iv, append(ct, tag...), []byte(headerB64))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// jweDecrypt 解密 JWE（测试断言辅助）。
func jweDecrypt(t *testing.T, priv *rsa.PrivateKey, compact string) string {
	t.Helper()
	plain, err := jweDecryptRaw(priv, compact)
	if err != nil {
		t.Fatalf("jweDecrypt: %v", err)
	}
	return plain
}

func TestSenseNovaJWEEncrypt(t *testing.T) {
	key := makeTestRSAKey(t)
	compact, err := senseNovaJWEEncrypt(&key.PublicKey, "my-secret-password")
	if err != nil {
		t.Fatalf("senseNovaJWEEncrypt() error = %v", err)
	}
	if got := jweDecrypt(t, key, compact); got != "my-secret-password" {
		t.Errorf("解密结果 = %q, want %q", got, "my-secret-password")
	}
}

// jwksJSON 把 RSA 公钥序列化为 JWKS 响应体。
func jwksJSON(t *testing.T, pub *rsa.PublicKey) string {
	t.Helper()
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	return `{"keys":[{"use":"sig","kty":"RSA","kid":"public:hydra.openid.id-token","alg":"RS256","n":"` + n + `","e":"` + e + `"}]}`
}

// makeTestAccessToken 构造带 exp 和 ext.tenant_id 的假 access_token。
func makeTestAccessToken(t *testing.T, exp time.Time, tenantID string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"exp": exp.Unix(),
		"ext": map[string]string{"tenant_id": tenantID},
	})
	if err != nil {
		t.Fatalf("marshal token: %v", err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

// withSenseNovaLoginServer 起一个完整 OIDC 登录流程的 mock 服务。
// password 为期望的登录密码（服务端解密 JWE 校验）；accessExp 为下发的 access_token 过期时间。
func withSenseNovaLoginServer(t *testing.T, rsaKey *rsa.PrivateKey, password string, accessExp time.Time, refreshToken string) *httptest.Server {
	t.Helper()
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth2/auth":
			q := r.URL.Query()
			// 分三步：login_verifier → consent_verifier → code
			if q.Get("consent_verifier") != "" {
				http.Redirect(w, r, ts.URL+"/?code=test-auth-code&state="+q.Get("state"), http.StatusSeeOther)
				return
			}
			if q.Get("login_verifier") != "" {
				http.Redirect(w, r, ts.URL+"/iam/authn/v1/auth/consent?consent_challenge=test-consent", http.StatusFound)
				return
			}
			http.Redirect(w, r, ts.URL+"/iam/authn/v1/auth/login?login_challenge=test-challenge", http.StatusFound)
		case r.URL.Path == "/iam/authn/v1/auth/login":
			http.Redirect(w, r, ts.URL+"/login?login_challenge=test-challenge&lang=zh-CN", http.StatusFound)
		case r.URL.Path == "/login":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/iam/authn/v1/auth/checkChallenge":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"is_valid":true,"redirect":"","platform":"uconsole"}`))
		case r.URL.Path == "/.well-known/jwks.json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(jwksJSON(t, &rsaKey.PublicKey)))
		case r.URL.Path == "/iam/authn/v1/auth/nova/login":
			var body struct {
				Username  string `json:"username"`
				Password  string `json:"password"`
				Challenge string `json:"challenge"`
				IsEncrypt bool   `json:"is_encrypt"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Challenge != "test-challenge" || !body.IsEncrypt {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"bad challenge"}`))
				return
			}
			gotPW, decryptErr := jweDecryptRaw(rsaKey, body.Password)
			if decryptErr != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"decrypt failed"}`))
				return
			}
			if gotPW != password || body.Username == "" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"incorrect password"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"redirect":"` + ts.URL + `/oauth2/auth?client_id=nova&login_verifier=test-login-verifier&state=any"}`))
		case r.URL.Path == "/iam/authn/v1/auth/consent":
			http.Redirect(w, r, ts.URL+"/oauth2/auth?client_id=nova&consent_verifier=test-consent-verifier", http.StatusFound)
		case r.URL.Path == "/oauth2/token":
			_ = r.ParseForm()
			switch r.Form.Get("grant_type") {
			case "authorization_code":
				if r.Form.Get("code") != "test-auth-code" || r.Form.Get("code_verifier") == "" {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte(`{"error":"bad code"}`))
					return
				}
			case "refresh_token":
				if r.Form.Get("refresh_token") != refreshToken {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte(`{"error":"bad refresh_token"}`))
					return
				}
			default:
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"bad grant_type"}`))
				return
			}
			access := makeTestAccessToken(t, accessExp, "tenant-1")
			body, _ := json.Marshal(map[string]any{
				"access_token":  access,
				"refresh_token": refreshToken,
				"expires_in":    10800,
				"token_type":    "Bearer",
			})
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return ts
}

// withSenseNovaURLs 把全局 URL 指向 mock 服务并恢复，同时清空 JWKS 公钥缓存
// （每个测试用独立的 RSA 密钥对，避免跨测试污染）。
func withSenseNovaURLs(t *testing.T, ts *httptest.Server) {
	t.Helper()
	old := []string{senseNovaOAuthAuthURL, senseNovaOAuthTokenURL, senseNovaIAMBaseURL, senseNovaJWKSURL}
	senseNovaOAuthAuthURL = ts.URL + "/oauth2/auth"
	senseNovaOAuthTokenURL = ts.URL + "/oauth2/token"
	senseNovaIAMBaseURL = ts.URL + "/iam/authn/v1/auth"
	senseNovaJWKSURL = ts.URL + "/.well-known/jwks.json"
	senseNovaJWKSMu.Lock()
	senseNovaJWKSKey = nil
	senseNovaJWKSFetched = time.Time{}
	senseNovaJWKSMu.Unlock()
	t.Cleanup(func() {
		senseNovaOAuthAuthURL, senseNovaOAuthTokenURL, senseNovaIAMBaseURL, senseNovaJWKSURL = old[0], old[1], old[2], old[3]
	})
}

func TestSenseNovaOIDCLogin(t *testing.T) {
	key := makeTestRSAKey(t)
	ts := withSenseNovaLoginServer(t, key, "secret-pw", time.Now().Add(3*time.Hour), "refresh-token-1")
	defer ts.Close()
	withSenseNovaURLs(t, ts)

	sess, err := senseNovaOIDCLogin(context.Background(), "winsks", "secret-pw")
	if err != nil {
		t.Fatalf("senseNovaOIDCLogin() error = %v", err)
	}
	if sess.accessToken == "" || sess.refreshToken != "refresh-token-1" {
		t.Errorf("会话 = %+v, want access_token 非空 + refresh_token=refresh-token-1", sess)
	}
	if !sess.expiresAt.After(time.Now().Add(2 * time.Hour)) {
		t.Errorf("expiresAt = %v, 应约为 3 小时后", sess.expiresAt)
	}
}

func TestSenseNovaOIDCLogin_WrongPassword(t *testing.T) {
	key := makeTestRSAKey(t)
	ts := withSenseNovaLoginServer(t, key, "secret-pw", time.Now().Add(3*time.Hour), "refresh-token-1")
	defer ts.Close()
	withSenseNovaURLs(t, ts)

	_, err := senseNovaOIDCLogin(context.Background(), "winsks", "wrong-pw")
	if err == nil {
		t.Fatal("senseNovaOIDCLogin() 应报错，却成功")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("错误信息 = %v, 应包含 401", err)
	}
}

func TestSenseNovaRefreshAccessToken(t *testing.T) {
	key := makeTestRSAKey(t)
	ts := withSenseNovaLoginServer(t, key, "secret-pw", time.Now().Add(3*time.Hour), "refresh-token-1")
	defer ts.Close()
	withSenseNovaURLs(t, ts)

	sess, err := senseNovaRefreshAccessToken(context.Background(), "refresh-token-1")
	if err != nil {
		t.Fatalf("senseNovaRefreshAccessToken() error = %v", err)
	}
	if sess.accessToken == "" || sess.refreshToken != "refresh-token-1" {
		t.Errorf("会话 = %+v", sess)
	}
}

func TestSenseNovaRefreshAccessToken_Invalid(t *testing.T) {
	key := makeTestRSAKey(t)
	ts := withSenseNovaLoginServer(t, key, "secret-pw", time.Now().Add(3*time.Hour), "refresh-token-1")
	defer ts.Close()
	withSenseNovaURLs(t, ts)

	if _, err := senseNovaRefreshAccessToken(context.Background(), "stale-refresh"); err == nil {
		t.Fatal("过期 refresh_token 应报错")
	}
}

func TestEnsureSenseNovaSession_RefreshTemporaryFailureNoRelogin(t *testing.T) {
	setupPlanProviderDB(t)
	crypto.Init("test-encryption-key")

	// 仅实现 oauth2/token（返回 500 模拟临时故障）；若降级触发 nova/login 则直接失败。
	loginCalled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "refresh_token" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/iam/authn/v1/auth/nova/login" {
			loginCalled = true
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()
	withSenseNovaURLs(t, ts)

	rtEnc, _ := crypto.Encrypt("rt-valid")
	pwEnc, _ := crypto.Encrypt("secret-pw")
	provider := &model.PlanProvider{
		ID:               6,
		APIKey:           makeTestAccessToken(t, time.Now().Add(-time.Hour), "t1"), // 已过期
		RefreshTokenEnc:  rtEnc,
		LoginUsername:    "winsks",
		LoginPasswordEnc: pwEnc,
	}
	_, err := ensureSenseNovaSession(context.Background(), provider)
	if err == nil {
		t.Fatal("临时故障应报错，却成功")
	}
	if !strings.Contains(err.Error(), "临时故障") {
		t.Errorf("错误信息 = %v, 应提示临时故障而非降级登录", err)
	}
	if loginCalled {
		t.Error("refresh 临时故障（5xx）不应降级触发账号密码登录（避免被风控）")
	}
}

func TestDecodeSenseNovaTokenExpiry(t *testing.T) {
	exp := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	tok := makeTestAccessToken(t, exp, "t1")
	got, ok := decodeSenseNovaTokenExpiry(tok)
	if !ok {
		t.Fatal("decodeSenseNovaTokenExpiry() 应解析成功")
	}
	if !got.Equal(exp) {
		t.Errorf("exp = %v, want %v", got, exp)
	}
	if _, ok := decodeSenseNovaTokenExpiry("not-a-jwt"); ok {
		t.Error("非法 token 不应解析成功")
	}
	if _, ok := decodeSenseNovaTokenExpiry("a.b"); ok {
		t.Error("无 exp 的 token 不应解析成功")
	}
}

// --- ensureSenseNovaSession 会话管理 ---

func TestEnsureSenseNovaSession_NoCredentials(t *testing.T) {
	provider := &model.PlanProvider{ID: 1}
	if _, err := ensureSenseNovaSession(context.Background(), provider); err == nil {
		t.Fatal("无任何凭据时应报错")
	}
}

func TestEnsureSenseNovaSession_ValidAPIKeyNoRequests(t *testing.T) {
	// APIKey 未过期：直接用，不应发起任何网络请求。
	exp := time.Now().Add(time.Hour)
	provider := &model.PlanProvider{
		ID:     2,
		APIKey: makeTestAccessToken(t, exp, "t1"),
	}
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer ts.Close()
	withSenseNovaURLs(t, ts)

	token, err := ensureSenseNovaSession(context.Background(), provider)
	if err != nil {
		t.Fatalf("ensureSenseNovaSession() error = %v", err)
	}
	if token != provider.APIKey {
		t.Errorf("token = %q", token)
	}
	if requests != 0 {
		t.Errorf("不应发起请求，实际 %d 次", requests)
	}
}

func TestEnsureSenseNovaSession_RefreshThenRelogin(t *testing.T) {
	setupPlanProviderDB(t)
	crypto.Init("test-encryption-key")

	key := makeTestRSAKey(t)
	// refresh_token 有效 → 走续期
	ts := withSenseNovaLoginServer(t, key, "secret-pw", time.Now().Add(3*time.Hour), "rt-valid")
	defer ts.Close()
	withSenseNovaURLs(t, ts)

	rtEnc, err := crypto.Encrypt("rt-valid")
	if err != nil {
		t.Fatalf("encrypt refresh token: %v", err)
	}
	pwEnc, err := crypto.Encrypt("secret-pw")
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	provider := createProviderRow(t, &model.PlanProvider{
		Category:         model.PlanProviderSenseNovaPlan,
		ProviderType:     model.PlanProviderTypeTokenPlan,
		APIKey:           makeTestAccessToken(t, time.Now().Add(-time.Hour), "t1"), // 已过期
		RefreshTokenEnc:  rtEnc,
		LoginUsername:    "winsks",
		LoginPasswordEnc: pwEnc,
	})
	oldKey := provider.APIKey
	token, err := ensureSenseNovaSession(context.Background(), provider)
	if err != nil {
		t.Fatalf("ensureSenseNovaSession() error = %v", err)
	}
	if token == "" || token == oldKey {
		t.Errorf("应换到新 token，实际 = %q", token)
	}

	// refresh_token 失效 → 自动降级账号密码重新登录
	rtEnc2, _ := crypto.Encrypt("stale-refresh")
	p2 := createProviderRow(t, &model.PlanProvider{
		Category:         model.PlanProviderSenseNovaPlan,
		ProviderType:     model.PlanProviderTypeTokenPlan,
		APIKey:           makeTestAccessToken(t, time.Now().Add(-time.Hour), "t1"),
		RefreshTokenEnc:  rtEnc2,
		LoginUsername:    "winsks",
		LoginPasswordEnc: pwEnc,
	})
	token2, err := ensureSenseNovaSession(context.Background(), p2)
	if err != nil {
		t.Fatalf("ensureSenseNovaSession() 降级登录 error = %v", err)
	}
	if token2 == "" {
		t.Error("降级登录应拿到 token")
	}

	// 会话缓存：同一 provider 再次调用不发请求（用已过期 token 的 provider 验证缓存命中）
	clearSenseNovaSession(3)
	// 缓存命中验证：替换 URL 指向不可达地址，缓存应避免任何请求
	senseNovaOAuthAuthURL = "http://127.0.0.1:1/oauth2/auth"
	provider2 := &model.PlanProvider{ID: provider.ID, APIKey: token}
	got, err := ensureSenseNovaSession(context.Background(), provider2)
	if err != nil {
		t.Fatalf("缓存命中场景 error = %v", err)
	}
	if got != token {
		t.Errorf("缓存命中应返回原 token")
	}
}

func TestEnsureSenseNovaSession_NoLoginCredentials(t *testing.T) {
	setupPlanProviderDB(t)
	crypto.Init("test-encryption-key")
	provider := &model.PlanProvider{
		ID:     5,
		APIKey: makeTestAccessToken(t, time.Now().Add(-time.Hour), "t1"), // 过期
	}
	_, err2 := ensureSenseNovaSession(context.Background(), provider)
	if err2 == nil {
		t.Fatal("token 过期且无账号密码时应报错")
	}
	if !strings.Contains(err2.Error(), "账号密码") {
		t.Errorf("错误信息 = %v", err2)
	}
}
