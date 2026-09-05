package planprovider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// makeTestOasisToken 构造一个合法的 Oasis-Token 格式（access...refresh），
// refresh payload 含指定 device_id。内容不参与签名校验（测试用）。
func makeTestOasisToken(t *testing.T, deviceID string) string {
	t.Helper()
	access := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhY3RpdmF0ZWQiOnRydWUsImV4cCI6OTk5OTk5OTk5OSwib2FzaXNfaWQiOjF9.sig"
	payload, _ := json.Marshal(map[string]any{
		"app_id":    10300,
		"device_id": deviceID,
		"exp":       9999999999,
		"platform":  "web",
	})
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	refresh := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." + encodedPayload + ".sig"
	return access + "..." + refresh
}

func TestDecodeStepFunWebID(t *testing.T) {
	token := makeTestOasisToken(t, "abc123device456")
	webid := decodeStepFunWebID(token)
	if webid != "abc123device456" {
		t.Errorf("decodeStepFunWebID() = %q, want %q", webid, "abc123device456")
	}
}

func TestDecodeStepFunWebID_NoSeparator(t *testing.T) {
	webid := decodeStepFunWebID("justanaccesstoken")
	if webid != "" {
		t.Errorf("decodeStepFunWebID() = %q, want empty", webid)
	}
}

func TestQueryStepFunPlanTokenPlan_Success(t *testing.T) {
	var gotCookie, gotAppID, gotMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCookie = r.Header.Get("Cookie")
		gotAppID = r.Header.Get("oasis-appid")

		if r.Header.Get("Connect-Protocol-Version") != "1" {
			t.Errorf("missing Connect-Protocol-Version header")
		}
		if r.Header.Get("oasis-platform") != "web" {
			t.Errorf("oasis-platform = %q, want web", r.Header.Get("oasis-platform"))
		}

		resp := map[string]any{
			"status":                     1,
			"desc":                       "",
			"five_hour_usage_left_rate":  0,
			"five_hour_usage_reset_time": "0",
			"weekly_usage_left_rate":     0,
			"weekly_usage_reset_time":    "0",
			"plan_family":                2,
			"plan_credit_rate_limit": map[string]any{
				"subscription_credit_left_rate":  0.9964648,
				"subscription_credit_reset_time": "1784379705",
				"topup_credit_left_rate":         0,
				"credit_buckets": []map[string]any{
					{
						"type":            1,
						"credit_total":    "400000000",
						"credit_residual": "398585926",
						"expire_at":       "1785675705",
						"next_reset_at":   "1784379705",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	// 覆盖硬编码 URL 指向 mock server
	origURL := stepFunPlanURL
	stepFunPlanURL = ts.URL
	defer func() { stepFunPlanURL = origURL }()

	token := makeTestOasisToken(t, "test-device-id")
	result, err := queryStepFunPlanTokenPlan(context.Background(), token)
	if err != nil {
		t.Fatalf("queryStepFunPlanTokenPlan() error = %v", err)
	}

	// 校验请求头
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if !strings.Contains(gotCookie, "Oasis-Token=") {
		t.Errorf("cookie missing Oasis-Token: %s", gotCookie)
	}
	if !strings.Contains(gotCookie, "Oasis-Webid=test-device-id") {
		t.Errorf("cookie missing Oasis-Webid: %s", gotCookie)
	}
	if gotAppID != "10300" {
		t.Errorf("oasis-appid = %q, want 10300", gotAppID)
	}

	// 校验解析结果
	if result.QuotaTotal != 400000000 {
		t.Errorf("QuotaTotal = %f, want 400000000", result.QuotaTotal)
	}
	// used = total - residual = 400000000 - 398585926 = 1414074
	if result.QuotaUsed != 1414074 {
		t.Errorf("QuotaUsed = %f, want 1414074", result.QuotaUsed)
	}
	if result.QuotaResetAt == nil {
		t.Error("QuotaResetAt is nil")
	} else {
		// 1784379705 = 2026-07-18 21:01:45 UTC
		want := time.Unix(1784379705, 0)
		if !result.QuotaResetAt.Equal(want) {
			t.Errorf("QuotaResetAt = %v, want %v", result.QuotaResetAt, want)
		}
	}
}

func TestQueryStepFunPlanTokenPlan_AuthFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"code":    "unauthenticated",
			"message": "auth failed: oasis-token is embezzled",
		})
	}))
	defer ts.Close()

	origURL := stepFunPlanURL
	stepFunPlanURL = ts.URL
	defer func() { stepFunPlanURL = origURL }()

	token := makeTestOasisToken(t, "device")
	_, err := queryStepFunPlanTokenPlan(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "鉴权失败") {
		t.Errorf("error = %q, want '鉴权失败'", err.Error())
	}
}

func TestQueryStepFunPlanTokenPlan_EmptyToken(t *testing.T) {
	_, err := queryStepFunPlanTokenPlan(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
	if !strings.Contains(err.Error(), "oasis token is required") {
		t.Errorf("error = %q, want 'oasis token is required'", err.Error())
	}
}

// --- SenseNova Plan 测试（新版 pool-usage 接口）---

func TestQuerySenseNovaPlanTokenPlan_Success(t *testing.T) {
	var gotAuth, gotMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")

		// 新版接口无查询参数（不再需要 account_id / model_ids）
		if r.URL.RawQuery != "" {
			t.Errorf("unexpected query params: %s", r.URL.RawQuery)
		}

		resp := map[string]any{
			"plan": map[string]any{
				"id":   "free",
				"name": "Free Plan",
				"type": "TOKEN_PLAN_PLAN_TYPE_FREE",
			},
			"pools": []any{
				map[string]any{
					"id":        "pool_default",
					"name":      "通用积分池",
					"pool_type": "default",
					"model_ids": []string{"deepseek-v4-flash", "sensenova-6.7-flash-lite"},
					"window_5h": map[string]any{
						"limit": "60000", "used": "0", "remaining": "60000", "reset_at": "1788523830",
					},
					"window_7d": map[string]any{
						"limit": "600000", "used": "354.84768", "remaining": "599645.15232", "reset_at": "1788948630",
					},
					"grant_balance":                  "1908.9024",
					"nearest_grant_expiry":           "1791025200",
					"nearest_grant_expiring_balance": "0.602",
				},
				map[string]any{
					"id":        "pool_dedicated",
					"name":      "Flash-Lite积分池",
					"pool_type": "dedicated",
					"model_ids": []string{"sensenova-6.7-flash-lite", "sensenova-6.8-flash-lite"},
					"window_5h": map[string]any{
						"limit": "60000", "used": "5716.0492", "remaining": "54283.9508", "reset_at": "1788523830",
					},
					"window_7d": map[string]any{
						"limit": "600000", "used": "5716.6512", "remaining": "594283.3488", "reset_at": "1788948630",
					},
					"grant_balance":                  "0",
					"nearest_grant_expiry":           "0",
					"nearest_grant_expiring_balance": "0",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	origURL := senseNovaPlanURL
	senseNovaPlanURL = ts.URL
	defer func() { senseNovaPlanURL = origURL }()

	result, err := querySenseNovaPlanTokenPlan(context.Background(), "test-bearer-token")
	if err != nil {
		t.Fatalf("querySenseNovaPlanTokenPlan() error = %v", err)
	}

	// 校验请求
	if gotMethod != http.MethodGet {
		t.Errorf("method = %s, want GET", gotMethod)
	}
	if !strings.Contains(gotAuth, "Bearer ") {
		t.Errorf("Authorization = %q, want Bearer", gotAuth)
	}

	// 校验解析结果：双池汇总
	if result.FiveHourTotal != 120000 {
		t.Errorf("FiveHourTotal = %f, want 120000 (双池求和)", result.FiveHourTotal)
	}
	if result.FiveHourUsed != 5716.0492 {
		t.Errorf("FiveHourUsed = %f, want 5716.0492", result.FiveHourUsed)
	}
	if result.WeeklyTotal != 1200000 {
		t.Errorf("WeeklyTotal = %f, want 1200000 (双池求和)", result.WeeklyTotal)
	}
	if result.WeeklyUsed != 6071.49888 {
		t.Errorf("WeeklyUsed = %f, want 6071.49888", result.WeeklyUsed)
	}
	if result.QuotaTotal != 1908.9024 {
		t.Errorf("QuotaTotal = %f, want 1908.9024 (grant 求和)", result.QuotaTotal)
	}
	if result.QuotaUsed != 0 {
		t.Errorf("QuotaUsed = %f, want 0", result.QuotaUsed)
	}
	// 重置时间：Unix 秒
	if result.FiveHourResetAt == nil || result.FiveHourResetAt.Unix() != 1788523830 {
		t.Errorf("FiveHourResetAt = %v, want 1788523830", result.FiveHourResetAt)
	}
	if result.WeeklyResetAt == nil || result.WeeklyResetAt.Unix() != 1788948630 {
		t.Errorf("WeeklyResetAt = %v, want 1788948630", result.WeeklyResetAt)
	}
	// 授权到期：取最早的 grant 到期
	if result.QuotaResetAt == nil || result.QuotaResetAt.Unix() != 1791025200 {
		t.Errorf("QuotaResetAt = %v, want 1791025200", result.QuotaResetAt)
	}
	// 分池明细：2 池
	if len(result.Pools) != 2 {
		t.Fatalf("Pools len = %d, want 2", len(result.Pools))
	}
	p0 := result.Pools[0]
	if p0.ID != "pool_default" || p0.Name != "通用积分池" || p0.PoolType != "default" {
		t.Errorf("pool[0] = %+v, want default 通用积分池", p0)
	}
	if p0.SevenDayLimit != 600000 || p0.SevenDayUsed != 354.84768 || p0.SevenDayRemain != 599645.15232 {
		t.Errorf("pool[0] SevenDay = %+v", p0)
	}
	if p0.FiveHourLimit != 60000 || p0.FiveHourRemain != 60000 {
		t.Errorf("pool[0] FiveHour = %+v", p0)
	}
	if p0.GrantBalance != 1908.9024 || p0.NearestGrantExpiringBal != 0.602 {
		t.Errorf("pool[0] grant = %+v", p0)
	}
	if p0.NearestGrantExpiry == nil || p0.NearestGrantExpiry.Unix() != 1791025200 {
		t.Errorf("pool[0] NearestGrantExpiry = %v, want 1791025200", p0.NearestGrantExpiry)
	}
	if len(p0.ModelIDs) != 2 || p0.ModelIDs[0] != "deepseek-v4-flash" {
		t.Errorf("pool[0] ModelIDs = %v", p0.ModelIDs)
	}
	p1 := result.Pools[1]
	if p1.PoolType != "dedicated" || p1.Name != "Flash-Lite积分池" {
		t.Errorf("pool[1] = %+v, want dedicated Flash-Lite积分池", p1)
	}
	if p1.GrantBalance != 0 || p1.NearestGrantExpiry != nil {
		t.Errorf("pool[1] grant = %+v, want 0/nil", p1)
	}
}

func TestQuerySenseNovaPlanTokenPlan_EmptyToken(t *testing.T) {
	_, err := querySenseNovaPlanTokenPlan(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
	if !strings.Contains(err.Error(), "token is required") {
		t.Errorf("error = %q, want 'token is required'", err.Error())
	}
}

func TestQuerySenseNovaPlanTokenPlan_NoPools(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"plan":  map[string]any{"id": "free", "name": "Free Plan"},
			"pools": []any{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	origURL := senseNovaPlanURL
	senseNovaPlanURL = ts.URL
	defer func() { senseNovaPlanURL = origURL }()

	_, err := querySenseNovaPlanTokenPlan(context.Background(), "token")
	if err == nil {
		t.Fatal("expected error for empty pools, got nil")
	}
	if !strings.Contains(err.Error(), "pools") {
		t.Errorf("error = %q, want mention of pools", err.Error())
	}
}
