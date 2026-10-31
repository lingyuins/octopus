package planprovider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 基元律动 TokenRhythm balance 查询：httptest mock server 验证
// 响应结构与真实 /api/usage-summary 一致。
func TestQueryTokenRhythmBalance(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/usage-summary" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// 验证 Cookie 透传
		if got := r.Header.Get("Cookie"); got == "" {
			t.Errorf("expected Cookie header, got empty")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"code": 0,
			"message": "ok",
			"data": {
				"calls": 1457,
				"successCalls": 1444,
				"errorCalls": 11,
				"abortedCalls": 2,
				"inputTokens": 350732048,
				"outputTokens": 1107942,
				"costCny": 14.75376696,
				"balanceCny": 53.24623304,
				"frozenBalanceCny": 0,
				"availableBalanceCny": 53.24623304,
				"currency": "CNY"
			},
			"traceId": "trace_test"
		}`))
	}))
	defer ts.Close()

	// 替换包级 URL 指向 mock server
	old := tokenRhythmUsageSummaryURL
	tokenRhythmUsageSummaryURL = ts.URL + "/api/usage-summary"
	defer func() { tokenRhythmUsageSummaryURL = old }()

	cookie := "tr_ref_device=test; tr_session=sess_test; tr_csrf=csrf_test"
	got, err := queryTokenRhythmBalance(context.Background(), cookie)
	if err != nil {
		t.Fatalf("queryTokenRhythmBalance: %v", err)
	}

	if got.Balance != 53.24623304 {
		t.Errorf("Balance = %v, want 53.24623304", got.Balance)
	}
	if got.BalanceUsed != 14.75376696 {
		t.Errorf("BalanceUsed = %v, want 14.75376696", got.BalanceUsed)
	}
	if got.Currency != "CNY" {
		t.Errorf("Currency = %q, want CNY", got.Currency)
	}
	if got.TotalTokens != 350732048+1107942 {
		t.Errorf("TotalTokens = %d, want %d", got.TotalTokens, 350732048+1107942)
	}
}

// 缺少有效 Cookie 时应报错
func TestQueryTokenRhythmBalanceMissingCookie(t *testing.T) {
	_, err := queryTokenRhythmBalance(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty cookie, got nil")
	}

	// 无 tr_session/tr_csrf 字段
	_, err = queryTokenRhythmBalance(context.Background(), "foo=bar")
	if err == nil {
		t.Fatal("expected error for invalid cookie, got nil")
	}
}

// 金额字段字符串格式（2026-08-16 tokenrhythm.studio 变更后真实格式）：
// costCny/balanceCny 等金额全部变为字符串，整数统计字段保持数字。
func TestQueryTokenRhythmBalanceStringAmounts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"code": 0,
			"message": "ok",
			"data": {
				"calls": 2087,
				"successCalls": 2027,
				"errorCalls": 54,
				"abortedCalls": 6,
				"inputTokens": 526263332,
				"outputTokens": 1537879,
				"costCny": "44.36027476",
				"tokenCostCny": "44.36027476",
				"imageCostCny": "0.00000000",
				"balanceCny": "23.63972524",
				"frozenBalanceCny": "0.00000000",
				"availableBalanceCny": "23.63972524",
				"expiringBalanceCny": "23.63972524",
				"currency": "CNY"
			},
			"traceId": "trace_str"
		}`))
	}))
	defer ts.Close()

	old := tokenRhythmUsageSummaryURL
	tokenRhythmUsageSummaryURL = ts.URL + "/api/usage-summary"
	defer func() { tokenRhythmUsageSummaryURL = old }()

	cookie := "tr_session=sess_str"
	got, err := queryTokenRhythmBalance(context.Background(), cookie)
	if err != nil {
		t.Fatalf("queryTokenRhythmBalance (string amounts): %v", err)
	}
	if got.Balance != 23.63972524 {
		t.Errorf("Balance = %v, want 23.63972524", got.Balance)
	}
	if got.BalanceUsed != 44.36027476 {
		t.Errorf("BalanceUsed = %v, want 44.36027476", got.BalanceUsed)
	}
	if got.Currency != "CNY" {
		t.Errorf("Currency = %q, want CNY", got.Currency)
	}
	if got.TotalTokens != 526263332+1537879 {
		t.Errorf("TotalTokens = %d, want %d", got.TotalTokens, 526263332+1537879)
	}
}

// 金额字段为 null / 空串时按 0 处理，不报错
func TestQueryTokenRhythmBalanceNullAmounts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"code": 0,
			"message": "ok",
			"data": {
				"calls": 0,
				"inputTokens": 0,
				"outputTokens": 0,
				"costCny": null,
				"balanceCny": "",
				"currency": "CNY"
			},
			"traceId": "trace_null"
		}`))
	}))
	defer ts.Close()

	old := tokenRhythmUsageSummaryURL
	tokenRhythmUsageSummaryURL = ts.URL + "/api/usage-summary"
	defer func() { tokenRhythmUsageSummaryURL = old }()

	got, err := queryTokenRhythmBalance(context.Background(), "tr_session=sess_null")
	if err != nil {
		t.Fatalf("queryTokenRhythmBalance (null amounts): %v", err)
	}
	if got.Balance != 0 || got.BalanceUsed != 0 {
		t.Errorf("Balance=%v BalanceUsed=%v, want both 0", got.Balance, got.BalanceUsed)
	}
	if got.Currency != "CNY" {
		t.Errorf("Currency = %q, want CNY", got.Currency)
	}
}

// API 返回非 0 code 时应报错
func TestQueryTokenRhythmBalanceAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code": 401, "message": "unauthorized", "data": null}`))
	}))
	defer ts.Close()

	old := tokenRhythmUsageSummaryURL
	tokenRhythmUsageSummaryURL = ts.URL + "/api/usage-summary"
	defer func() { tokenRhythmUsageSummaryURL = old }()

	_, err := queryTokenRhythmBalance(context.Background(), "tr_session=bad")
	if err == nil {
		t.Fatal("expected error for API code != 0, got nil")
	}
}
