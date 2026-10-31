package planprovider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

// withTokenRhythmServers 起 mock 的基元律动登录 + usage-summary 服务，
// 返回登录/用量两个端点的请求计数。
func withTokenRhythmServers(t *testing.T, loginHandler, usageHandler http.HandlerFunc) (*int, *int) {
	t.Helper()
	loginRequests, usageRequests := 0, 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			loginRequests++
			loginHandler(w, r)
		case "/api/usage-summary":
			usageRequests++
			usageHandler(w, r)
		default:
			http.NotFound(w, r)
		}
	}))

	oldLogin, oldUsage := tokenRhythmLoginURL, tokenRhythmUsageSummaryURL
	tokenRhythmLoginURL = ts.URL + "/api/auth/login"
	tokenRhythmUsageSummaryURL = ts.URL + "/api/usage-summary"
	t.Cleanup(func() {
		tokenRhythmLoginURL = oldLogin
		tokenRhythmUsageSummaryURL = oldUsage
		ts.Close()
	})
	return &loginRequests, &usageRequests
}

// tokenRhythmLoginOK 登录成功：下发 tr_session / tr_csrf Cookie + 成功响应体。
func tokenRhythmLoginOK(session string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Account  string `json:"account"`
			Password string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Account == "" || body.Password == "" {
			http.Error(w, "missing credentials", http.StatusBadRequest)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "tr_session", Value: session, Path: "/", MaxAge: 2592000})
		http.SetCookie(w, &http.Cookie{Name: "tr_csrf", Value: "csrf-" + session, Path: "/", MaxAge: 2592000})
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"message":"ok","data":{"user":{"id":"u-1","name":"tester"}}}`))
	}
}

// tokenRhythmUsageOK 用量查询成功。
func tokenRhythmUsageOK(balance, cost string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"message":"ok","data":{"calls":10,"successCalls":10,` +
			`"inputTokens":1000,"outputTokens":200,` +
			`"costCny":"` + cost + `","balanceCny":"` + balance + `","currency":"CNY"}}`))
	}
}

// tokenRhythmUsageUnauthorized 会话失效：HTTP 401 + UNAUTHORIZED（真实响应）。
func tokenRhythmUsageUnauthorized() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"code":"UNAUTHORIZED","message":"未认证或登录已过期","traceId":"trace_x"}`))
	}
}

// newTokenRhythmProvider 构造带账号密码的 provider（密码走真实加密路径）。
func newTokenRhythmProvider(t *testing.T, id int, password, cookie string) *model.PlanProvider {
	t.Helper()
	enc, err := crypto.Encrypt(password)
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	return &model.PlanProvider{
		ID:               id,
		Category:         model.PlanProviderTokenRhythm,
		ProviderType:     model.PlanProviderTypeBalance,
		LoginUsername:    "19217618255",
		LoginPasswordEnc: enc,
		APIKey:           cookie,
		BaseURL:          "https://tokenrhythm.studio",
	}
}

// 账号密码模式：无 Cookie 时应登录换取会话，并用该 Cookie 查询成功。
func TestTokenRhythmLoginAndQuery(t *testing.T) {
	loginReqs, usageReqs := withTokenRhythmServers(t, tokenRhythmLoginOK("sess_new"), tokenRhythmUsageOK("159.03", "522.53"))
	clearTokenRhythmSession(1000)
	defer clearTokenRhythmSession(1000)

	provider := newTokenRhythmProvider(t, 1000, "pw-ok", "")
	result, err := refreshBalanceWithLogin(context.Background(), provider)
	if err != nil {
		t.Fatalf("refreshBalanceWithLogin: %v", err)
	}
	if result.Balance != 159.03 {
		t.Errorf("Balance = %v, want 159.03", result.Balance)
	}
	if result.BalanceUsed != 522.53 {
		t.Errorf("BalanceUsed = %v, want 522.53", result.BalanceUsed)
	}
	if *loginReqs != 1 {
		t.Errorf("login requests = %d, want 1", *loginReqs)
	}
	if *usageReqs != 1 {
		t.Errorf("usage requests = %d, want 1", *usageReqs)
	}
	// 登录结果应写回主凭据字段（由 RefreshProvider 落库）。
	if !strings.Contains(provider.APIKey, "tr_session=sess_new") {
		t.Errorf("APIKey = %q, want 包含 tr_session=sess_new", provider.APIKey)
	}
}

// 已有 Cookie 且有效：不应触发登录（避免不必要的控制台登录）。
func TestTokenRhythmReusesExistingCookie(t *testing.T) {
	loginReqs, usageReqs := withTokenRhythmServers(t, tokenRhythmLoginOK("sess_unused"), tokenRhythmUsageOK("10.00", "1.00"))
	clearTokenRhythmSession(1001)
	defer clearTokenRhythmSession(1001)

	provider := newTokenRhythmProvider(t, 1001, "pw-ok", "tr_session=sess_old; tr_csrf=csrf-old")
	if _, err := refreshBalanceWithLogin(context.Background(), provider); err != nil {
		t.Fatalf("refreshBalanceWithLogin: %v", err)
	}
	if *loginReqs != 0 {
		t.Errorf("login requests = %d, want 0（已有有效 Cookie 时不应登录）", *loginReqs)
	}
	if *usageReqs != 1 {
		t.Errorf("usage requests = %d, want 1", *usageReqs)
	}
	if provider.APIKey != "tr_session=sess_old; tr_csrf=csrf-old" {
		t.Errorf("APIKey 被意外改写: %q", provider.APIKey)
	}
}

// 会话失效（HTTP 401）：应丢弃旧 Cookie、重新登录一次并重试成功。
func TestTokenRhythmReloginOnSessionInvalid(t *testing.T) {
	first := true
	usageHandler := func(w http.ResponseWriter, r *http.Request) {
		if first {
			first = false
			tokenRhythmUsageUnauthorized()(w, r)
			return
		}
		tokenRhythmUsageOK("88.88", "12.34")(w, r)
	}
	loginReqs, usageReqs := withTokenRhythmServers(t, tokenRhythmLoginOK("sess_rotated"), usageHandler)
	clearTokenRhythmSession(1002)
	defer clearTokenRhythmSession(1002)

	provider := newTokenRhythmProvider(t, 1002, "pw-ok", "tr_session=sess_stale; tr_csrf=csrf-stale")
	result, err := refreshBalanceWithLogin(context.Background(), provider)
	if err != nil {
		t.Fatalf("refreshBalanceWithLogin: %v", err)
	}
	if result.Balance != 88.88 {
		t.Errorf("Balance = %v, want 88.88", result.Balance)
	}
	if *loginReqs != 1 {
		t.Errorf("login requests = %d, want 1（会话失效后重登一次）", *loginReqs)
	}
	if *usageReqs != 2 {
		t.Errorf("usage requests = %d, want 2（首次 401 + 重登后重试）", *usageReqs)
	}
	if !strings.Contains(provider.APIKey, "tr_session=sess_rotated") {
		t.Errorf("APIKey = %q, want 已更新为新会话", provider.APIKey)
	}
}

// 重登后仍失效：第二次 401 直接返回会话失效错误，不再无限重试。
func TestTokenRhythmReloginGivesUpAfterSecondFailure(t *testing.T) {
	loginReqs, usageReqs := withTokenRhythmServers(t, tokenRhythmLoginOK("sess_again"), tokenRhythmUsageUnauthorized())
	clearTokenRhythmSession(1003)
	defer clearTokenRhythmSession(1003)

	provider := newTokenRhythmProvider(t, 1003, "pw-ok", "tr_session=sess_stale")
	_, err := refreshBalanceWithLogin(context.Background(), provider)
	if err == nil {
		t.Fatal("expected error when session keeps being invalid")
	}
	if !errors.Is(err, errTokenRhythmSessionInvalid) {
		t.Errorf("err = %v, want errTokenRhythmSessionInvalid", err)
	}
	if *loginReqs != 1 {
		t.Errorf("login requests = %d, want 1（只重登一次）", *loginReqs)
	}
	if *usageReqs != 2 {
		t.Errorf("usage requests = %d, want 2", *usageReqs)
	}
}

// 密码错误（HTTP 401）：返回登录失败错误，且冷却期内不再重复登录（防风控）。
func TestTokenRhythmLoginFailureCoolDown(t *testing.T) {
	badLogin := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"code":"UNAUTHORIZED","message":"未认证或登录已过期"}`))
	}
	loginReqs, _ := withTokenRhythmServers(t, badLogin, tokenRhythmUsageOK("1", "1"))
	clearTokenRhythmSession(1004)
	defer clearTokenRhythmSession(1004)

	provider := newTokenRhythmProvider(t, 1004, "pw-wrong", "")
	_, err := refreshBalanceWithLogin(context.Background(), provider)
	if err == nil {
		t.Fatal("expected login error for wrong password")
	}
	if !strings.Contains(err.Error(), "账号或密码错误") {
		t.Errorf("err = %v, want 包含账号或密码错误", err)
	}

	// 冷却期内再次刷新：不应再次真实登录。
	if _, err := refreshBalanceWithLogin(context.Background(), provider); err == nil {
		t.Fatal("expected error during cool-down")
	}
	if *loginReqs != 1 {
		t.Errorf("login requests = %d, want 1（冷却期内不重复登录）", *loginReqs)
	}
}

// 未配置账号密码：refreshBalanceWithLogin 等价于直接查询（行为不变）。
func TestTokenRhythmManualCookieUnchanged(t *testing.T) {
	loginReqs, usageReqs := withTokenRhythmServers(t, tokenRhythmLoginOK("sess_x"), tokenRhythmUsageOK("5.00", "2.00"))
	clearTokenRhythmSession(1005)
	defer clearTokenRhythmSession(1005)

	provider := &model.PlanProvider{
		ID:       1005,
		Category: model.PlanProviderTokenRhythm,
		APIKey:   "tr_session=manual; tr_csrf=manual-csrf",
	}
	result, err := refreshBalanceWithLogin(context.Background(), provider)
	if err != nil {
		t.Fatalf("refreshBalanceWithLogin: %v", err)
	}
	if result.Balance != 5 {
		t.Errorf("Balance = %v, want 5", result.Balance)
	}
	if *loginReqs != 0 || *usageReqs != 1 {
		t.Errorf("login=%d usage=%d, want login=0 usage=1", *loginReqs, *usageReqs)
	}
}

// tokenRhythmLogin 直接单测：成功解析 Cookie、失败返回可读错误。
func TestTokenRhythmLogin(t *testing.T) {
	withTokenRhythmServers(t, tokenRhythmLoginOK("sess_direct"), tokenRhythmUsageOK("1", "1"))
	cookie, err := tokenRhythmLogin(context.Background(), "19217618255", "pw")
	if err != nil {
		t.Fatalf("tokenRhythmLogin: %v", err)
	}
	if !strings.Contains(cookie, "tr_session=sess_direct") || !strings.Contains(cookie, "tr_csrf=csrf-sess_direct") {
		t.Errorf("cookie = %q, want 含 tr_session 与 tr_csrf", cookie)
	}
}

// 登录成功但响应未带 tr_session Cookie 时报错（避免存下无效凭据）。
func TestTokenRhythmLoginMissingCookie(t *testing.T) {
	noCookie := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"message":"ok","data":{"user":{"id":"u"}}}`))
	}
	withTokenRhythmServers(t, noCookie, tokenRhythmUsageOK("1", "1"))
	if _, err := tokenRhythmLogin(context.Background(), "acct", "pw"); err == nil {
		t.Fatal("expected error when tr_session Cookie missing")
	}
}

// 业务码非 0（HTTP 200 + code 为字符串）时返回登录失败错误。
func TestTokenRhythmLoginBusinessError(t *testing.T) {
	bizErr := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":"captcha_required","message":"请完成安全验证"}`))
	}
	withTokenRhythmServers(t, bizErr, tokenRhythmUsageOK("1", "1"))
	_, err := tokenRhythmLogin(context.Background(), "acct", "pw")
	if err == nil {
		t.Fatal("expected error for non-zero business code")
	}
	if !strings.Contains(err.Error(), "请完成安全验证") {
		t.Errorf("err = %v, want 原样透出服务端提示", err)
	}
}

// 用量的 code 为字符串 UNAUTHORIZED（HTTP 200 变体）时按会话失处理。
func TestQueryTokenRhythmBalanceUnauthorizedBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":"UNAUTHORIZED","message":"未认证或登录已过期"}`))
	}))
	defer ts.Close()

	old := tokenRhythmUsageSummaryURL
	tokenRhythmUsageSummaryURL = ts.URL + "/api/usage-summary"
	defer func() { tokenRhythmUsageSummaryURL = old }()

	_, err := queryTokenRhythmBalance(context.Background(), "tr_session=whatever")
	if !errors.Is(err, errTokenRhythmSessionInvalid) {
		t.Errorf("err = %v, want errTokenRhythmSessionInvalid", err)
	}
}
