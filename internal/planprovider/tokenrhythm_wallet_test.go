package planprovider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFetchTokenRhythmWallet 校验资金明细三个端点的字段映射与口径：
// 赠送额度账本（逐笔到账 + 本金行 + 汇总）、金额字符串/数字两种格式、
// expiresAt 可空、累计获赠取账本 summary（不是 wallet/summary.giftTotalCny）。
func TestFetchTokenRhythmWallet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/wallet/summary":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"currency":"CNY",` +
				`"availableBalanceCny":"220.82454716","giftAvailableCny":"220.82454716",` +
				`"rechargeBalanceCny":"0.00000000","debtBalanceCny":"0.00000000",` +
				`"frozenBalanceCny":"0.00000000","giftTotalCny":"220.82454716",` +
				`"asOf":"2026-09-24T05:03:35.577Z"}}`))
		case "/api/wallet/expiring-credits":
			if got := r.URL.Query().Get("page"); got != "1" {
				t.Errorf("page = %q, want 1", got)
			}
			if got := r.URL.Query().Get("pageSize"); got != "50" {
				t.Errorf("pageSize = %q, want 50", got)
			}
			if got := r.URL.Query().Get("includeInactive"); got != "true" {
				t.Errorf("includeInactive = %q, want true", got)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{` +
				`"asOf":"2026-09-24T05:03:35.577Z","total":2,"page":1,"pageSize":50,` +
				`"summary":{"expiringBalanceCny":"3.20142724",` +
				`"nextExpiryAt":"2026-10-25T12:03:46.813Z","cumulativeGiftGrantedCny":"766.00000000"},` +
				`"rechargePrincipal":{"id":"recharge_principal","source":"RECHARGE","sourceLabel":"充值本金",` +
				`"grantedCny":"128.00000000","consumedCny":"0.00000000","remainingCny":"128.00000000",` +
				`"status":"ACTIVE","expiresAt":null},` +
				`"list":[` +
				`{"id":"c1","source":"IDENTITY_VERIFICATION_REWARD","sourceLabel":"实名认证奖励",` +
				`"grantedCny":"18.00000000","consumedCny":"14.79857276","remainingCny":"3.20142724",` +
				`"status":"ACTIVE","grantedAt":"2026-09-24T12:03:46.813Z","expiresAt":"2026-10-25T12:03:46.813Z"},` +
				`{"id":"c2","source":"INVITE_REWARD","sourceLabel":"邀请奖励",` +
				`"grantedCny":68,"consumedCny":68,"remainingCny":0,` +
				`"status":"USED_UP","grantedAt":"2026-08-21T06:32:22.072Z","expiresAt":"2026-09-21T06:32:22.072Z"}` +
				`]}}`))
		case "/api/usage-summary":
			_, _ = w.Write([]byte(`{"code":0,"data":{"costCny":"1.19545284","balanceCny":"220.82454716"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	restore := swapTokenRhythmWalletURLs(ts.URL)
	defer restore()

	got, err := fetchTokenRhythmWallet(context.Background(), "tr_session=sess_test; tr_csrf=csrf_test")
	if err != nil {
		t.Fatalf("fetchTokenRhythmWallet: %v", err)
	}
	if got.Currency != "CNY" {
		t.Errorf("currency = %q, want CNY", got.Currency)
	}
	if got.AvailableBalanceCNY != 220.82454716 {
		t.Errorf("available = %v, want 220.82454716", got.AvailableBalanceCNY)
	}
	// 累计获赠取账本 summary.cumulativeGiftGrantedCny（不是 wallet/summary.giftTotalCny）。
	if got.TotalReceivedCNY != 766 {
		t.Errorf("total received = %v, want 766 (cumulativeGiftGrantedCny)", got.TotalReceivedCNY)
	}
	if got.TotalConsumedCNY != 1.19545284 {
		t.Errorf("total consumed = %v, want 1.19545284 (usage-summary costCny)", got.TotalConsumedCNY)
	}
	if got.ExpiringBalanceCNY != 3.20142724 {
		t.Errorf("expiring balance = %v, want 3.20142724", got.ExpiringBalanceCNY)
	}
	if got.NextExpiryAt != "2026-10-25T12:03:46.813Z" {
		t.Errorf("next expiry = %q, want 2026-10-25T12:03:46.813Z", got.NextExpiryAt)
	}
	if got.Total != 2 || got.Page != 1 || got.PageSize != 50 {
		t.Errorf("pagination = total %d page %d pageSize %d, want 2/1/50", got.Total, got.Page, got.PageSize)
	}
	if len(got.Credits) != 2 {
		t.Fatalf("credits len = %d, want 2", len(got.Credits))
	}
	first := got.Credits[0]
	if first.SourceLabel != "实名认证奖励" || first.IsPrincipal {
		t.Errorf("credit0 = %+v, want 实名认证奖励 非本金", first)
	}
	if first.GrantedCNY != 18 || first.ConsumedCNY != 14.79857276 || first.RemainingCNY != 3.20142724 {
		t.Errorf("credit0 amounts = %v/%v/%v, want 18/14.79857276/3.20142724",
			first.GrantedCNY, first.ConsumedCNY, first.RemainingCNY)
	}
	if first.Status != "ACTIVE" || first.GrantedAt != "2026-09-24T12:03:46.813Z" {
		t.Errorf("credit0 status/grantedAt = %q/%q", first.Status, first.GrantedAt)
	}
	if first.ExpiresAt == nil || *first.ExpiresAt != "2026-10-25T12:03:46.813Z" {
		t.Errorf("credit0 expiresAt = %v, want 2026-10-25T12:03:46.813Z", first.ExpiresAt)
	}
	// 数字格式（非字符串）金额同样要能解析。
	if got.Credits[1].GrantedCNY != 68 || got.Credits[1].Status != "USED_UP" {
		t.Errorf("credit1 = %+v, want granted 68 / USED_UP", got.Credits[1])
	}
	// 本金行：单独返回、无 grantedAt（官网显示「长期有效」）。
	if got.RechargePrincipal == nil {
		t.Fatal("recharge principal = nil, want a row")
	}
	if !got.RechargePrincipal.IsPrincipal || got.RechargePrincipal.GrantedAt != "" {
		t.Errorf("principal = %+v, want IsPrincipal 且无 grantedAt", got.RechargePrincipal)
	}
	if got.RechargePrincipal.GrantedCNY != 128 || got.RechargePrincipal.Status != "ACTIVE" {
		t.Errorf("principal amounts = %v/%s, want 128/ACTIVE",
			got.RechargePrincipal.GrantedCNY, got.RechargePrincipal.Status)
	}
}

// TestFetchTokenRhythmWalletSessionInvalid 校验会话失效（HTTP 401）返回哨兵错误，
// 供调用方丢弃会话重登一次。
func TestFetchTokenRhythmWalletSessionInvalid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"UNAUTHORIZED","message":"未认证或登录已过期"}`))
	}))
	defer ts.Close()

	restore := swapTokenRhythmWalletURLs(ts.URL)
	defer restore()

	_, err := fetchTokenRhythmWallet(context.Background(), "tr_session=sess_dead")
	if !errors.Is(err, errTokenRhythmSessionInvalid) {
		t.Fatalf("err = %v, want errTokenRhythmSessionInvalid", err)
	}
}

// TestFetchTokenRhythmWalletAPIError 校验上游业务错误码（code != 0）被识别为错误。
func TestFetchTokenRhythmWalletAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":40001,"message":"boom"}`))
	}))
	defer ts.Close()

	restore := swapTokenRhythmWalletURLs(ts.URL)
	defer restore()

	if _, err := fetchTokenRhythmWallet(context.Background(), "tr_session=sess_test"); err == nil {
		t.Fatal("expected error for API code != 0, got nil")
	}
}

// swapTokenRhythmWalletURLs 把三个上游 URL 指向 mock server，返回恢复函数。
func swapTokenRhythmWalletURLs(base string) func() {
	oldSummary, oldCredits, oldUsage := tokenRhythmWalletSummaryURL, tokenRhythmWalletCreditsURL, tokenRhythmUsageSummaryURL
	tokenRhythmWalletSummaryURL = base + "/api/wallet/summary"
	tokenRhythmWalletCreditsURL = base + "/api/wallet/expiring-credits"
	tokenRhythmUsageSummaryURL = base + "/api/usage-summary"
	return func() {
		tokenRhythmWalletSummaryURL = oldSummary
		tokenRhythmWalletCreditsURL = oldCredits
		tokenRhythmUsageSummaryURL = oldUsage
	}
}
