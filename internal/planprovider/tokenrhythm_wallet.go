package planprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// 基元律动（tokenrhythm.studio）资金明细。
//
// 口径与官网「用户中心 → 账户」页（/account/account）的「资金明细」表一致：
//   - GET /api/wallet/summary            账户余额汇总（可用/赠送/充值/欠费/冻结）
//   - GET /api/wallet/expiring-credits   赠送额度账本：逐笔赠金（含本金行与汇总），
//     page/pageSize/includeInactive=true，保留已用尽/已到期历史
//   - GET /api/usage-summary             累计消费 costCny（与卡片「已用额度」同源）
//
// 历史教训：早期实现取「钱包明细」页的全量流水 /api/wallet/transactions 首屏 20 条，
// 再过滤掉 DEBIT（模型调用扣费）只留到账 —— 活跃账号首屏 20 条几乎全是扣费，真实
// 到账被埋在数百条之后（实测 635 / 220 条），弹窗长期显示「暂无资金明细」。账户页的
// 赠送额度账本本身就是「逐笔到账」列表，不存在这个遮蔽问题。
//
// 鉴权与 usage-summary 一致：Cookie（tr_session + tr_csrf）。账号密码模式下复用
// ensureTokenRhythmSession 的会话缓存，失效（HTTP 401）时丢弃会话重登一次再试。

var (
	tokenRhythmWalletSummaryURL = "https://tokenrhythm.studio/api/wallet/summary"
	tokenRhythmWalletCreditsURL = "https://tokenrhythm.studio/api/wallet/expiring-credits"
)

// tokenRhythmWalletCreditsPageSize 明细条数：官网账户页可选 10/20/50，这里取 50 取满一页。
// 响应里带 Total，前端据此提示「共 N 笔」（超过 50 笔时只展示最新 50 笔）。
const tokenRhythmWalletCreditsPageSize = 50

// TokenRhythmWalletCredit 一笔到账额度（赠送额度账本条目，或充值本金行）。
type TokenRhythmWalletCredit struct {
	ID           string  `json:"id"`
	Source       string  `json:"source"`       // RECHARGE / IDENTITY_VERIFICATION_REWARD / INVITE_REWARD / INITIAL_BALANCE ...
	SourceLabel  string  `json:"source_label"` // 上游中文标签：充值本金 / 实名认证奖励 / 邀请奖励 ...
	GrantedCNY   float64 `json:"granted_cny"`
	ConsumedCNY  float64 `json:"consumed_cny"`
	RemainingCNY float64 `json:"remaining_cny"`
	Status       string  `json:"status"`     // ACTIVE 生效中 / PAUSED 暂停中 / USED_UP 已用尽 / EXPIRED 已到期
	GrantedAt    string  `json:"granted_at"` // 本金行上游不返回 → 空串，前端显示「长期有效」
	ExpiresAt    *string `json:"expires_at"`
	// IsPrincipal 充值本金行（上游单独放在 rechargePrincipal，不参与 list 分页）。
	IsPrincipal bool `json:"is_principal"`
}

// TokenRhythmWallet 资金明细查询结果（字段与官网账户页展示一一对应）。
type TokenRhythmWallet struct {
	Currency            string  `json:"currency"`
	AvailableBalanceCNY float64 `json:"available_balance_cny"` // 剩余可用
	GiftBalanceCNY      float64 `json:"gift_balance_cny"`
	RechargeBalanceCNY  float64 `json:"recharge_balance_cny"`
	DebtBalanceCNY      float64 `json:"debt_balance_cny"`
	FrozenBalanceCNY    float64 `json:"frozen_balance_cny"`
	// TotalReceivedCNY 累计获赠 = 账本 summary.cumulativeGiftGrantedCny。
	// ⚠️ 不是 wallet/summary.giftTotalCny —— 后者实测恒等于 giftAvailableCny（当前赠送余额），
	// 早期把它标成「累计到账」属错标（实测 3.20 / 218.46 vs 真实累计 766.00 / 290.00）。
	TotalReceivedCNY float64 `json:"total_received_cny"`
	// TotalConsumedCNY 累计消费 = usage-summary.costCny（不含到期失效金额，与官网口径一致）。
	TotalConsumedCNY float64 `json:"total_consumed_cny"`
	// ExpiringBalanceCNY 即将到期余额 = 账本 summary.expiringBalanceCny；NextExpiryAt 最近到期时间。
	ExpiringBalanceCNY float64 `json:"expiring_balance_cny"`
	NextExpiryAt       string  `json:"next_expiry_at"`
	AsOf               string  `json:"as_of"`
	// Total 账本条目总数（不含本金行）；Credits 为本次返回的一页（最多 50 笔，最新在前）。
	Total    int                       `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
	Credits  []TokenRhythmWalletCredit `json:"credits"`
	// RechargePrincipal 充值本金行（grantedCny > 0 时官网才把它插在表格第一行）。
	RechargePrincipal *TokenRhythmWalletCredit `json:"recharge_principal"`
}

// QueryTokenRhythmWalletByID 按 provider ID 查询资金明细（供 HTTP handler 调用）。
func QueryTokenRhythmWalletByID(ctx context.Context, id int) (*TokenRhythmWallet, error) {
	var provider model.PlanProvider
	if err := db.GetDB().WithContext(ctx).First(&provider, id).Error; err != nil {
		return nil, fmt.Errorf("find plan provider: %w", err)
	}
	// 凭据密文解密回明文供查询使用（只读，不落库）。
	decryptProviderSecrets(&provider)

	if provider.Category != model.PlanProviderTokenRhythm {
		return nil, fmt.Errorf("tokenrhythm: wallet transactions is only supported for tokenrhythm provider")
	}
	if provider.ProviderType != model.PlanProviderTypeBalance {
		return nil, fmt.Errorf("tokenrhythm: wallet transactions requires a balance provider")
	}

	cookie, err := tokenRhythmCookieForQuery(ctx, &provider)
	if err != nil {
		return nil, err
	}
	result, err := fetchTokenRhythmWallet(ctx, cookie)
	if err == nil {
		return result, nil
	}
	// 会话失效：账号密码模式下丢弃会话重登一次再试（与 refreshBalanceWithLogin 同构）。
	if !errors.Is(err, errTokenRhythmSessionInvalid) || provider.LoginUsername == "" {
		return nil, err
	}
	log.Warnf("planprovider: tokenrhythm provider %d 资金明细会话失效，重新登录后重试", provider.ID)
	invalidateTokenRhythmSession(provider.ID)
	provider.APIKey = ""
	cookie, err = tokenRhythmCookieForQuery(ctx, &provider)
	if err != nil {
		return nil, err
	}
	return fetchTokenRhythmWallet(ctx, cookie)
}

// tokenRhythmCookieForQuery 取查询用 Cookie：账号密码模式走会话缓存/自动登录，
// 手动模式直接用用户粘贴的 Cookie。
func tokenRhythmCookieForQuery(ctx context.Context, provider *model.PlanProvider) (string, error) {
	if provider.LoginUsername != "" && provider.LoginPasswordEnc != "" {
		return ensureTokenRhythmSession(ctx, provider)
	}
	if provider.APIKey == "" {
		return "", fmt.Errorf("tokenrhythm: 未配置凭据（控制台 Cookie 或账号密码）")
	}
	return provider.APIKey, nil
}

// fetchTokenRhythmWallet 拉取账户余额 + 赠送额度账本 + 累计消费。
func fetchTokenRhythmWallet(ctx context.Context, cookie string) (*TokenRhythmWallet, error) {
	summaryBody, err := doTokenRhythmGet(ctx, tokenRhythmWalletSummaryURL, cookie)
	if err != nil {
		return nil, fmt.Errorf("tokenrhythm: query wallet summary: %w", err)
	}
	var summaryResp struct {
		Code int `json:"code"`
		Data struct {
			Currency            string          `json:"currency"`
			AvailableBalanceCNY flexibleFloat64 `json:"availableBalanceCny"`
			GiftAvailableCNY    flexibleFloat64 `json:"giftAvailableCny"`
			RechargeBalanceCNY  flexibleFloat64 `json:"rechargeBalanceCny"`
			DebtBalanceCNY      flexibleFloat64 `json:"debtBalanceCny"`
			FrozenBalanceCNY    flexibleFloat64 `json:"frozenBalanceCny"`
			AsOf                string          `json:"asOf"`
		} `json:"data"`
	}
	if err := json.Unmarshal(summaryBody, &summaryResp); err != nil {
		return nil, fmt.Errorf("tokenrhythm: parse wallet summary: %w", err)
	}
	if summaryResp.Code != 0 {
		return nil, fmt.Errorf("tokenrhythm: wallet summary API error code=%d", summaryResp.Code)
	}

	// rawCredit 与上游账本条目字段一一对应（金额可能是字符串或数字，故用 flexibleFloat64）。
	type rawCredit struct {
		ID           string          `json:"id"`
		Source       string          `json:"source"`
		SourceLabel  string          `json:"sourceLabel"`
		GrantedCNY   flexibleFloat64 `json:"grantedCny"`
		ConsumedCNY  flexibleFloat64 `json:"consumedCny"`
		RemainingCNY flexibleFloat64 `json:"remainingCny"`
		Status       string          `json:"status"`
		GrantedAt    string          `json:"grantedAt"`
		ExpiresAt    *string         `json:"expiresAt"`
	}

	creditsURL := tokenRhythmWalletCreditsURL + "?page=1&pageSize=" +
		strconv.Itoa(tokenRhythmWalletCreditsPageSize) + "&includeInactive=true"
	creditsBody, err := doTokenRhythmGet(ctx, creditsURL, cookie)
	if err != nil {
		return nil, fmt.Errorf("tokenrhythm: query wallet credits: %w", err)
	}
	var creditsResp struct {
		Code int `json:"code"`
		Data struct {
			AsOf     string      `json:"asOf"`
			Total    int         `json:"total"`
			Page     int         `json:"page"`
			PageSize int         `json:"pageSize"`
			List     []rawCredit `json:"list"`
			Summary  struct {
				ExpiringBalanceCNY       flexibleFloat64 `json:"expiringBalanceCny"`
				NextExpiryAt             string          `json:"nextExpiryAt"`
				CumulativeGiftGrantedCNY flexibleFloat64 `json:"cumulativeGiftGrantedCny"`
			} `json:"summary"`
			RechargePrincipal *rawCredit `json:"rechargePrincipal"`
		} `json:"data"`
	}
	if err := json.Unmarshal(creditsBody, &creditsResp); err != nil {
		return nil, fmt.Errorf("tokenrhythm: parse wallet credits: %w", err)
	}
	if creditsResp.Code != 0 {
		return nil, fmt.Errorf("tokenrhythm: wallet credits API error code=%d", creditsResp.Code)
	}

	// 累计消费：复用 usage-summary 的 costCny（与卡片「已用额度」同源）。
	usageBody, err := doTokenRhythmGet(ctx, tokenRhythmUsageSummaryURL, cookie)
	if err != nil {
		return nil, fmt.Errorf("tokenrhythm: query usage-summary for total consumed: %w", err)
	}
	var usageResp struct {
		Code int `json:"code"`
		Data struct {
			CostCny flexibleFloat64 `json:"costCny"`
		} `json:"data"`
	}
	if err := json.Unmarshal(usageBody, &usageResp); err != nil {
		return nil, fmt.Errorf("tokenrhythm: parse usage-summary: %w", err)
	}
	if usageResp.Code != 0 {
		return nil, fmt.Errorf("tokenrhythm: usage-summary API error code=%d", usageResp.Code)
	}

	currency := summaryResp.Data.Currency
	if currency == "" {
		currency = "CNY"
	}

	toCredit := func(raw rawCredit, principal bool) TokenRhythmWalletCredit {
		return TokenRhythmWalletCredit{
			ID:           raw.ID,
			Source:       raw.Source,
			SourceLabel:  raw.SourceLabel,
			GrantedCNY:   float64(raw.GrantedCNY),
			ConsumedCNY:  float64(raw.ConsumedCNY),
			RemainingCNY: float64(raw.RemainingCNY),
			Status:       raw.Status,
			GrantedAt:    raw.GrantedAt,
			ExpiresAt:    raw.ExpiresAt,
			IsPrincipal:  principal,
		}
	}

	result := &TokenRhythmWallet{
		Currency:            currency,
		AvailableBalanceCNY: float64(summaryResp.Data.AvailableBalanceCNY),
		GiftBalanceCNY:      float64(summaryResp.Data.GiftAvailableCNY),
		RechargeBalanceCNY:  float64(summaryResp.Data.RechargeBalanceCNY),
		DebtBalanceCNY:      float64(summaryResp.Data.DebtBalanceCNY),
		FrozenBalanceCNY:    float64(summaryResp.Data.FrozenBalanceCNY),
		TotalReceivedCNY:    float64(creditsResp.Data.Summary.CumulativeGiftGrantedCNY),
		TotalConsumedCNY:    float64(usageResp.Data.CostCny),
		ExpiringBalanceCNY:  float64(creditsResp.Data.Summary.ExpiringBalanceCNY),
		NextExpiryAt:        creditsResp.Data.Summary.NextExpiryAt,
		AsOf:                creditsResp.Data.AsOf,
		Total:               creditsResp.Data.Total,
		Page:                creditsResp.Data.Page,
		PageSize:            creditsResp.Data.PageSize,
		Credits:             make([]TokenRhythmWalletCredit, 0, len(creditsResp.Data.List)),
	}
	if result.AsOf == "" {
		result.AsOf = summaryResp.Data.AsOf
	}
	for _, item := range creditsResp.Data.List {
		result.Credits = append(result.Credits, toCredit(item, false))
	}
	if principal := creditsResp.Data.RechargePrincipal; principal != nil {
		converted := toCredit(*principal, true)
		result.RechargePrincipal = &converted
	}
	return result, nil
}
