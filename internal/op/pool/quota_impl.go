package pool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/planprovider"
)

func init() {
	// 注入额度查询实现 + 全量同步实现。
	FetchAccountQuotaFunc = fetchAccountQuotaImpl
	SyncAllQuotasFunc = syncAllQuotasImpl
}

// fetchAccountQuotaImpl 按 platform/type 路由到 planprovider 查询额度。
func fetchAccountQuotaImpl(ctx context.Context, acct *model.PoolAccount) (*QuotaResult, error) {
	// 解密凭据。
	_ = DecryptAccountCredentials(acct)
	cred := model.ParsePoolCredential(acct.Credentials)

	switch {
	case acct.Platform == model.PoolPlatformOpenAI && acct.Type == model.PoolTypeOAuth:
		// codex OAuth：凭据是 OAuth JSON，传给 QueryTokenPlan。
		oauthJSON := acct.Credentials
		// 若凭据非 JSON（旧格式），用 EffectiveKey 构造。
		if oauthJSON == "" || oauthJSON[0] != '{' {
			oauthJSON = cred.EffectiveKey(acct.Platform)
		}
		tp, err := planprovider.QueryTokenPlan(ctx, model.PlanProviderCodex, oauthJSON, "",
			model.ProxyUsageModeDirect, acct.ProxyConfigID, "", "")
		if err != nil {
			return nil, err
		}
		return quotaResultFromTokenPlan(tp), nil

	case acct.Platform == model.PoolPlatformVolcengine && acct.Type == model.PoolTypeCookie:
		// volcengine cookie 凭据：格式 cookie|||csrf。
		credential := cred.Cookie
		if credential == "" {
			credential = cred.Token
		}
		tp, err := planprovider.QueryTokenPlan(ctx, model.PlanProviderVolcenginePlan, credential, "",
			model.ProxyUsageModeDirect, nil, "", "")
		if err != nil {
			return nil, err
		}
		return quotaResultFromTokenPlan(tp), nil

	default:
		// anthropic/gemini/grok/custom 无公开额度 API。
		return nil, fmt.Errorf("quota query not supported for platform %s type %s", acct.Platform, acct.Type)
	}
}

// quotaResultFromTokenPlan 将 planprovider.TokenPlanResult 转为 QuotaResult。
func quotaResultFromTokenPlan(tp *planprovider.TokenPlanResult) *QuotaResult {
	if tp == nil {
		return nil
	}
	result := &QuotaResult{
		Used:  tp.QuotaUsed,
		Total: tp.QuotaTotal,
	}
	if tp.QuotaResetAt != nil {
		result.ResetAt = tp.QuotaResetAt.Unix()
	}
	// 序列化完整快照到 Raw。
	if raw, err := json.Marshal(tp); err == nil {
		result.Raw = string(raw)
	}
	return result
}

// syncAllQuotasImpl 遍历所有账号查询额度（后台任务）。
func syncAllQuotasImpl(ctx context.Context) {
	accounts, err := ListAllAccounts()
	if err != nil {
		return
	}
	for i := range accounts {
		acct := &accounts[i]
		// 仅查询有额度 API 的平台/类型组合，避免无效请求。
		if !quotaSupportedForAccount(acct) {
			continue
		}
		_, _ = FetchAccountQuota(ctx, acct.PoolID, acct.ID)
	}
}

// quotaSupported 判断平台/类型组合是否支持额度查询（不考虑 extra）。
func quotaSupported(platform, ctype string) bool {
	switch {
	case platform == model.PoolPlatformOpenAI && ctype == model.PoolTypeOAuth:
		return true
	case platform == model.PoolPlatformVolcengine && ctype == model.PoolTypeCookie:
		return true
	}
	return false
}

// quotaSupportedForAccount 考虑 extra.OAuthType==code_assist 的 Gemini 帐号不走额度查询
// （Gemini Code Assist 未公开 quota API）。
func quotaSupportedForAccount(acct *model.PoolAccount) bool {
	if acct.Platform == model.PoolPlatformGemini {
		extra := acct.GetExtra()
		if extra.OAuthType == model.OAuthTypeCodeAssist {
			return false
		}
	}
	return quotaSupported(acct.Platform, acct.Type)
}
