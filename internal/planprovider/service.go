package planprovider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/helper"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
	stats "github.com/lingyuins/octopus/internal/op/stats"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
	"github.com/lingyuins/octopus/internal/utils/crypto"
	"github.com/lingyuins/octopus/internal/utils/log"
)

const planChannelGroupName = "Plan"

// planFetchModels 拉取渠道最新模型列表的可注入函数（测试时替换）。
var planFetchModels = func(ctx context.Context, channelType outbound.OutboundType, baseURL, apiKey string) ([]string, error) {
	request := model.Channel{
		Type: channelType,
		BaseUrls: []model.BaseUrl{
			{URL: baseURL, Delay: 0},
		},
		Keys: []model.ChannelKey{
			{Enabled: true, ChannelKey: apiKey, Managed: true},
		},
	}
	return helper.FetchModelsShortTimeout(ctx, request)
}

// resolvePlanChannelModels 解析创建渠道用的模型列表：
// 优先从上游拉取最新模型（Codex 除外——其凭据是 OAuth JSON，无法作为 Bearer 请求 /models），
// 拉取失败或结果为空时回退到厂商默认列表。
func resolvePlanChannelModels(ctx context.Context, category model.PlanProviderCategory, channelType outbound.OutboundType, baseURL, apiKey, fallback string) string {
	if category == model.PlanProviderCodex {
		return fallback
	}
	fetched, err := planFetchModels(ctx, channelType, baseURL, apiKey)
	if err != nil {
		log.Warnf("plan provider %s: fetch models from %s failed, fallback to default models: %v", category, baseURL, err)
		return fallback
	}
	normalized := normalizeModelList(strings.Join(fetched, ","))
	if normalized == "" {
		log.Warnf("plan provider %s: fetch models from %s returned empty, fallback to default models", category, baseURL)
		return fallback
	}
	return normalized
}

// legacyDeepSeekDefaultModels 旧版 DeepSeek 额度监控自动创建渠道时的默认模型列表
// （官方已弃用）。仅当渠道模型列表规范化后与此完全一致时，才判定为旧版自动创建的渠道。
var legacyDeepSeekDefaultModels = []string{"deepseek-chat", "deepseek-reasoner"}

// MigrateLegacyDeepSeekChannels 幂等迁移：把旧版自动创建的 DeepSeek 额度渠道
// （模型列表恰为弃用的 deepseek-chat,deepseek-reasoner）更新为当前默认模型
// （deepseek-v4-flash,deepseek-v4-pro）。只匹配与旧默认完全一致的渠道，
// 不触碰用户手动配置过模型列表的渠道。返回迁移数量。
func MigrateLegacyDeepSeekChannels(ctx context.Context) (int, error) {
	var providers []model.PlanProvider
	if err := db.GetDB().WithContext(ctx).
		Where("category = ? AND channel_id > 0", model.PlanProviderDeepSeek).
		Find(&providers).Error; err != nil {
		return 0, fmt.Errorf("list deepseek plan providers: %w", err)
	}

	old := normalizeModelList(strings.Join(legacyDeepSeekDefaultModels, ","))
	info := getCategoryInfo(model.PlanProviderDeepSeek)
	if info == nil {
		return 0, fmt.Errorf("deepseek category info not found")
	}

	migrated := 0
	for _, p := range providers {
		ch, err := op.ChannelGet(p.ChannelID, ctx)
		if err != nil || ch == nil {
			continue
		}
		if normalizeModelList(ch.Model) != old {
			continue
		}
		updateReq := &model.ChannelUpdateRequest{
			ID:    ch.ID,
			Model: &info.Models,
		}
		if _, err := op.ChannelUpdate(updateReq, ctx); err != nil {
			log.Warnf("planprovider: migrate legacy deepseek channel %d models failed: %v", ch.ID, err)
			continue
		}
		migrated++
	}
	if migrated > 0 {
		log.Infof("planprovider: migrated %d legacy deepseek channel(s) to default models %q", migrated, info.Models)
	}
	return migrated, nil
}

// ListProviders 列出所有 Plan Provider
func ListProviders(ctx context.Context, providerType model.PlanProviderType) ([]model.PlanProviderListItem, error) {
	var providers []model.PlanProvider
	if err := db.GetDB().WithContext(ctx).
		Where("provider_type = ?", providerType).
		Order("id ASC").
		Find(&providers).Error; err != nil {
		return nil, fmt.Errorf("list plan providers: %w", err)
	}

	result := make([]model.PlanProviderListItem, 0, len(providers))
	for _, p := range providers {
		item := model.PlanProviderListItem{PlanProvider: p}
		item.APIKey = ""
		item.ForwardAPIKey = ""
		item.LoginConfigured = p.LoginUsername != "" && p.LoginPasswordEnc != ""
		// 本次与上次检测之间的消费增量：
		// balance 类 = 上次余额 − 本次余额（充值导致的负值按 0）；
		// tokenplan 类 = 本次已用 − 上次已用（周期重置导致的负值按 0）。
		if p.ProviderType == model.PlanProviderTypeBalance {
			item.BalanceDelta = max(0, p.LastBalance-p.Balance)
		} else {
			item.QuotaUsedDelta = max(0, p.QuotaUsed-p.LastQuotaUsed)
		}
		if p.ChannelID > 0 {
			channel, err := op.ChannelGet(p.ChannelID, ctx)
			if err == nil {
				item.Models = channel.Model
				item.ChannelName = channel.Name
				item.ChannelEnabled = channel.Enabled
			}
			// DeepSeek 渠道调用统计：优先官方控制台 usage（配置了账号密码时），
			// 官方查询失败或无账号密码时回退本地 relay stats。
			if p.Category == model.PlanProviderDeepSeek {
				item.ChannelStats = queryPlanChannelStats(ctx, &p, p.ChannelID)
			}
		}
		result = append(result, item)
	}
	return result, nil
}

// queryPlanChannelStats 查询 DeepSeek 关联渠道的调用统计（累计 + 今日）。
//
// 数据源优先级：
//  1. 官方控制台 usage（platform.deepseek.com，配置了账号密码时）——真实 token 用量；
//  2. 本地 relay stats 落库表（与 Analytics 页口径一致）——无账号密码或官方查询失败时兜底。
func queryPlanChannelStats(ctx context.Context, provider *model.PlanProvider, channelID int) *model.PlanChannelStats {
	if channelID <= 0 {
		return nil
	}
	// 官方 usage 优先：真实 token 用量（覆盖账号下所有 API key 的调用）。
	if provider != nil && provider.LoginUsername != "" && provider.LoginPasswordEnc != "" {
		if official, err := queryDeepSeekOfficialUsage(ctx, provider); err == nil && official != nil {
			return &model.PlanChannelStats{
				TotalRequests: official.totalRequests,
				TotalTokens:   official.totalTokens,
				TodayRequests: official.todayRequests,
				TodayTokens:   official.todayTokens,
				Source:        "official",
			}
		} else if err != nil {
			logDeepSeekUsageErr(provider.ID, err)
		}
	}
	chStats := &model.PlanChannelStats{Source: "local"}
	var total model.StatsChannel
	if err := db.GetDB().WithContext(ctx).Where("channel_id = ?", channelID).First(&total).Error; err == nil {
		chStats.TotalRequests = total.RequestSuccess + total.RequestFailed
		chStats.TotalTokens = total.InputToken + total.OutputToken
	}
	today := stats.Now().Format("20060102")
	var daily model.StatsDailyChannel
	if err := db.GetDB().WithContext(ctx).Where("date = ? AND channel_id = ?", today, channelID).First(&daily).Error; err == nil {
		chStats.TodayRequests = daily.RequestSuccess + daily.RequestFailed
		chStats.TodayTokens = daily.InputToken + daily.OutputToken
	}
	return chStats
}

// AddProvider 添加 Plan Provider：查询额度 → （可选）创建/复用 Channel 并归入渠道 Plan 分组
//
// apiKey 是主凭据（balance 类厂商的 sk- key，或 stepfun_plan 的 Oasis-Token）。
// forwardAPIKey 仅 stepfun_plan 使用：可选的 sk- API Key，用于转发。
//   - 填了：创建或复用接入点 api.stepfun.com/step_plan/v1 的渠道，key 追加到模型相同的已有渠道。
//   - 不填：仅监控套餐额度，不创建渠道。
//
// proxyMode / proxyConfigID 仅 Codex 类生效（chatgpt.com 国内不可直连）：
// 同时作用于用量查询链路与自动创建的转发渠道；其他厂商强制 direct。
// refreshIntervalMin 自动刷新间隔（分钟），0 表示跟随全局默认设置。
//
// loginUsername / loginPassword 仅 sensenova_plan 使用（可选）：
// 配置商汤控制台账号密码后，系统自动完成 OIDC 登录获取控制台 Bearer Token 并自动续期，
// 无需手动更换每 3 小时过期的 Token；此时 apiKey 可留空（登录成功后会写入 access_token）。
func AddProvider(ctx context.Context, category model.PlanProviderCategory, apiKey, forwardAPIKey, customName string, refreshIntervalMin int, proxyMode model.ProxyUsageMode, proxyConfigID *int, teamOrgID, teamProjectID, loginUsername, loginPassword string) (*model.PlanProvider, error) {
	info := getCategoryInfo(category)
	if info == nil {
		return nil, fmt.Errorf("unknown plan provider category: %s", category)
	}
	if refreshIntervalMin < 0 {
		return nil, fmt.Errorf("refresh interval must be greater than or equal to 0")
	}

	// 代理配置仅 Codex 类采纳；其他厂商防御性强制 direct。
	if category != model.PlanProviderCodex {
		proxyMode = model.ProxyUsageModeDirect
		proxyConfigID = nil
	}
	if proxyMode == "" {
		proxyMode = model.ProxyUsageModeDirect
	}
	if err := proxyMode.Validate(false); err != nil {
		return nil, err
	}
	if proxyMode != model.ProxyUsageModePool {
		proxyConfigID = nil
	}
	if proxyMode == model.ProxyUsageModePool && (proxyConfigID == nil || *proxyConfigID <= 0) {
		return nil, fmt.Errorf("proxy config id is required when proxy mode is pool")
	}

	apiKey = strings.TrimSpace(apiKey)
	loginUsername = strings.TrimSpace(loginUsername)
	loginPassword = strings.TrimSpace(loginPassword)
	// 账号密码自动登录目前支持 sensenova_plan 与 deepseek。
	// sensenova：登录 token 作为主凭据（覆盖 APIKey）；deepseek：账号密码
	// 用于查询控制台官方 usage，APIKey 仍保留用于余额查询，两者并存。
	if loginUsername != "" && category != model.PlanProviderSenseNovaPlan && category != model.PlanProviderDeepSeek {
		return nil, fmt.Errorf("account login is only supported for sensenova_plan and deepseek")
	}
	if loginUsername != "" && loginPassword == "" {
		return nil, fmt.Errorf("login password is required")
	}
	if apiKey == "" && loginUsername == "" {
		return nil, fmt.Errorf("API key or login credentials is required")
	}
	// DeepSeek 的账号密码是"附加"的官方 usage 数据源，余额查询仍需要 API key。
	if category == model.PlanProviderDeepSeek && apiKey == "" {
		return nil, fmt.Errorf("deepseek requires an API key for balance query (account login is optional)")
	}

	// 账号密码模式：sensenova 需先完成 OIDC 登录，用拿到的 access_token 作为
	// 主凭据，并保存加密的 refresh_token 供后续自动续期；deepseek 只保存
	// 加密的账号密码（登录发生在查询官方 usage 时，token 不落库、不覆盖 APIKey）。
	var loginPasswordEnc, refreshTokenEnc string
	if loginUsername != "" && category == model.PlanProviderSenseNovaPlan {
		enc, err := crypto.Encrypt(loginPassword)
		if err != nil {
			return nil, fmt.Errorf("encrypt login password: %w", err)
		}
		loginPasswordEnc = enc
		sess, err := senseNovaOIDCLogin(ctx, loginUsername, loginPassword)
		if err != nil {
			return nil, fmt.Errorf("sensenova login: %w", err)
		}
		apiKey = sess.accessToken
		if sess.refreshToken != "" {
			if enc, err := crypto.Encrypt(sess.refreshToken); err == nil {
				refreshTokenEnc = enc
			}
		}
	} else if loginUsername != "" && category == model.PlanProviderDeepSeek {
		enc, err := crypto.Encrypt(loginPassword)
		if err != nil {
			return nil, fmt.Errorf("encrypt login password: %w", err)
		}
		loginPasswordEnc = enc
	}

	teamOrgID = strings.TrimSpace(teamOrgID)
	teamProjectID = strings.TrimSpace(teamProjectID)
	// 智谱团队版需 API Key + 组织 ID + 项目 ID 三者齐全，缺一不可。
	if category == model.PlanProviderZhipuTeam {
		if teamOrgID == "" || teamProjectID == "" {
			return nil, fmt.Errorf("zhipu team plan needs the API key + organization ID + project ID")
		}
	}

	forwardAPIKey = normalizePlanForwardAPIKey(category, strings.TrimSpace(forwardAPIKey))

	name := customName
	if name == "" {
		name = info.Name
	}

	// 1. 查询余额 / TokenPlan
	var balance, balanceUsed float64
	var totalTokens int64
	var quotaTotal, quotaUsed, weeklyTotal, weeklyUsed float64
	var fiveHourTotal, fiveHourUsed float64
	var quotaResetAt, weeklyResetAt, fiveHourResetAt *string

	if info.Type == model.PlanProviderTypeBalance {
		result, err := QueryBalance(ctx, category, apiKey, info.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("query balance: %w", err)
		}
		balance = result.Balance
		balanceUsed = result.BalanceUsed
		totalTokens = result.TotalTokens
	} else {
		result, err := QueryTokenPlan(ctx, category, apiKey, info.BaseURL, proxyMode, proxyConfigID, teamOrgID, teamProjectID)
		if err != nil {
			return nil, fmt.Errorf("query tokenplan: %w", err)
		}
		quotaTotal = result.QuotaTotal
		quotaUsed = result.QuotaUsed
		if result.QuotaResetAt != nil {
			s := result.QuotaResetAt.Format("2006-01-02 15:04:05")
			quotaResetAt = &s
		}
		weeklyTotal = result.WeeklyTotal
		weeklyUsed = result.WeeklyUsed
		if result.WeeklyResetAt != nil {
			s := result.WeeklyResetAt.Format("2006-01-02 15:04:05")
			weeklyResetAt = &s
		}
		fiveHourTotal = result.FiveHourTotal
		fiveHourUsed = result.FiveHourUsed
		if result.FiveHourResetAt != nil {
			s := result.FiveHourResetAt.Format("2006-01-02 15:04:05")
			fiveHourResetAt = &s
		}
	}

	// 2. 渠道创建/复用
	var channelID int
	// balance 类默认自动创建转发渠道；tokenrhythm（基元律动）例外：
	// 纯监控（Cookie 鉴权，无独立 API Key），不创建转发渠道。
	needCreateChannel := (info.Type == model.PlanProviderTypeBalance && category != model.PlanProviderTokenRhythm) ||
		(isConsoleTokenPlanCategory(category) && forwardAPIKey != "") ||
		category == model.PlanProviderCodex
	if needCreateChannel {
		channelGroupID, err := ensurePlanChannelGroup(ctx)
		if err != nil {
			return nil, fmt.Errorf("ensure plan channel group: %w", err)
		}

		// 渠道接入点与凭据：balance 类用 info.BaseURL + apiKey；
		// 控制台 token plan 类用各自的转发 API 地址 + forwardAPIKey；
		// Codex 用 info.BaseURL + apiKey（OAuth JSON），channel type 为 Codex。
		channelBaseURL := info.BaseURL
		channelKey := apiKey
		channelName := fmt.Sprintf("[%s] %s", info.Name, name)
		if customName != "" {
			channelName = fmt.Sprintf("[%s] %s", info.Name, customName)
		}
		channelType := outbound.OutboundTypeOpenAIChat
		if isConsoleTokenPlanCategory(category) {
			channelBaseURL = planForwardAPIBaseURL(category)
			channelKey = forwardAPIKey
			channelName = fmt.Sprintf("[%s] %s", planForwardLabel(category), name)
		}
		if category == model.PlanProviderCodex {
			channelType = outbound.OutboundTypeCodex
			channelKey = apiKey // OAuth JSON, same as monitoring credential
		}

		// 默认使用厂商内置模型列表；优先尝试从上游拉取最新模型，失败时回退。
		channelModel := resolvePlanChannelModels(ctx, category, channelType, channelBaseURL, channelKey, info.Models)

		// 查找可复用的已有渠道（同 category + 接入点 + 模型相同的渠道）
		reuseChannelID := findReusablePlanChannel(ctx, category, channelBaseURL, channelModel)

		if reuseChannelID > 0 {
			// 复用：追加 key 到已有渠道
			addReq := &model.ChannelUpdateRequest{
				ID: reuseChannelID,
				KeysToAdd: []model.ChannelKeyAddRequest{
					{Enabled: true, ChannelKey: channelKey, Managed: true},
				},
			}
			if _, err := op.ChannelUpdate(addReq, ctx); err != nil {
				return nil, fmt.Errorf("reuse channel add key: %w", err)
			}
			channelID = reuseChannelID
		} else {
			// 新建渠道
			channel := &model.Channel{
				Name:    channelName,
				GroupID: channelGroupID,
				Type:    channelType,
				Enabled: true,
				BaseUrls: []model.BaseUrl{
					{URL: channelBaseURL, Delay: 0},
				},
				Keys: []model.ChannelKey{
					{Enabled: true, ChannelKey: channelKey, Managed: true},
				},
				Model:     channelModel,
				AutoSync:  false,
				AutoGroup: model.AutoGroupTypeNone,
			}
			if category == model.PlanProviderCodex {
				// Codex 渠道继承套餐的代理配置（chatgpt.com 国内不可直连）。
				channel.ProxyMode = proxyMode
				channel.ProxyConfigID = proxyConfigID
			}
			if err := op.ChannelCreate(channel, ctx); err != nil {
				return nil, fmt.Errorf("create channel: %w", err)
			}
			channelID = channel.ID
		}
	}

	// 3. 持久化 PlanProvider
	now := time.Now()
	provider := &model.PlanProvider{
		Name:               name,
		Category:           category,
		ProviderType:       info.Type,
		APIKey:             apiKey,
		ForwardAPIKey:      forwardAPIKey,
		TeamOrganizationID: teamOrgID,
		TeamProjectID:      teamProjectID,
		LoginUsername:      loginUsername,
		LoginPasswordEnc:   loginPasswordEnc,
		RefreshTokenEnc:    refreshTokenEnc,
		BaseURL:            info.BaseURL,
		ChannelID:          channelID,
		ProxyMode:          proxyMode,
		ProxyConfigID:      proxyConfigID,
		RefreshIntervalMin: refreshIntervalMin,
		// 首次添加即建立快照：本次与上次检测之间暂无增量（delta 为 0）
		LastBalance:   balance,
		LastQuotaUsed: quotaUsed,
		Balance:       balance,
		BalanceUsed:   balanceUsed,
		TotalTokens:   totalTokens,
		QuotaTotal:    quotaTotal,
		QuotaUsed:     quotaUsed,
		WeeklyTotal:   weeklyTotal,
		WeeklyUsed:    weeklyUsed,
		FiveHourTotal: fiveHourTotal,
		FiveHourUsed:  fiveHourUsed,
	}

	if quotaResetAt != nil {
		if t, err := time.Parse("2006-01-02 15:04:05", *quotaResetAt); err == nil {
			provider.QuotaResetAt = &t
		}
	}
	if weeklyResetAt != nil {
		if t, err := time.Parse("2006-01-02 15:04:05", *weeklyResetAt); err == nil {
			provider.WeeklyResetAt = &t
		}
	}
	if fiveHourResetAt != nil {
		if t, err := time.Parse("2006-01-02 15:04:05", *fiveHourResetAt); err == nil {
			provider.FiveHourResetAt = &t
		}
	}

	provider.LastRefresh = &now
	provider.CreatedAt = now
	provider.UpdatedAt = now

	// APIKey/ForwardAPIKey 加密落库；落库完成后恢复明文（返回给调用方）。
	if err := encryptProviderSecrets(provider); err != nil {
		return nil, err
	}
	defer decryptProviderSecrets(provider)
	if err := db.GetDB().WithContext(ctx).Create(provider).Error; err != nil {
		// 补偿：provider 落盘失败，回滚已创建的 channel（仅新建的，复用的不删）
		if channelID > 0 && needCreateChannel {
			// 只在新建渠道时补偿；复用渠道追加的 key 留在渠道上（无害，用户可手动清理）
			if !isReusedChannel(ctx, channelID) {
				if delErr := op.ChannelDel(channelID, ctx); delErr != nil {
					log.Warnf("planprovider: compensate delete channel %d after provider create failed: %v", channelID, delErr)
				}
			}
		}
		return nil, fmt.Errorf("create plan provider record: %w", err)
	}

	return provider, nil
}

// RefreshProvider 刷新 Plan Provider 的余额 / TokenPlan
func RefreshProvider(ctx context.Context, id int) (*model.PlanProvider, error) {
	var provider model.PlanProvider
	if err := db.GetDB().WithContext(ctx).First(&provider, id).Error; err != nil {
		return nil, fmt.Errorf("find plan provider: %w", err)
	}
	// 凭据密文解密回明文供查询使用；Save 前会重新加密。
	decryptProviderSecrets(&provider)

	if provider.ProviderType == model.PlanProviderTypeBalance {
		// 快照旧余额：本次刷新后 LastBalance 表示"上次检测时的余额"，
		// 与最新 Balance 的差值即两次检测之间的消费；差额累加进累计已用额度。
		lastBalance := provider.Balance
		result, err := QueryBalance(ctx, provider.Category, provider.APIKey, provider.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("refresh balance: %w", err)
		}
		provider.LastBalance = lastBalance
		provider.Balance = result.Balance
		provider.BalanceUsed = result.BalanceUsed
		provider.TotalTokens = result.TotalTokens
		provider.TotalUsed += max(0, lastBalance-provider.Balance)
	} else {
		// 商汤日日新：若配置了账号密码，先确保控制台 access_token 有效
		// （过期则自动 refresh_token 续期或重新登录），再查套餐用量。
		if provider.Category == model.PlanProviderSenseNovaPlan && provider.LoginUsername != "" {
			if _, err := ensureSenseNovaSession(ctx, &provider); err != nil {
				return nil, fmt.Errorf("refresh sensenova session: %w", err)
			}
		}
		lastQuotaUsed := provider.QuotaUsed
		result, err := QueryTokenPlan(ctx, provider.Category, provider.APIKey, provider.BaseURL, provider.ProxyMode, provider.ProxyConfigID, provider.TeamOrganizationID, provider.TeamProjectID)
		if err != nil {
			return nil, fmt.Errorf("refresh tokenplan: %w", err)
		}
		provider.LastQuotaUsed = lastQuotaUsed
		provider.QuotaTotal = result.QuotaTotal
		provider.QuotaUsed = result.QuotaUsed
		if result.QuotaResetAt != nil {
			provider.QuotaResetAt = result.QuotaResetAt
		}
		provider.WeeklyTotal = result.WeeklyTotal
		provider.WeeklyUsed = result.WeeklyUsed
		if result.WeeklyResetAt != nil {
			provider.WeeklyResetAt = result.WeeklyResetAt
		}
		provider.FiveHourTotal = result.FiveHourTotal
		provider.FiveHourUsed = result.FiveHourUsed
		provider.FiveHourResetAt = result.FiveHourResetAt
	}

	now := time.Now()
	provider.LastRefresh = &now
	provider.UpdatedAt = now

	if err := db.GetDB().WithContext(ctx).Save(&provider).Error; err != nil {
		return nil, fmt.Errorf("save plan provider: %w", err)
	}

	return &provider, nil
}

// UpdateProviderCredentials 更新 Plan Provider 的主凭据（api_key），可选更新转发凭据（forward_api_key）。
//
// 典型场景：控制台会话凭据（stepfun/sensenova/bailian/volcengine 的 Cookie/Token、
// mimo 的 serviceToken/passToken、codex 的 OAuth JSON）过期或即将过期，刷新时报 401/未登录，
// 需要用户重新从控制台获取凭据并替换，而非删除重建（删除会连带删掉关联的转发渠道与 channel keys 状态）。
//
// 行为：
//   - newAPIKey 与 loginUsername 至少填一个（sensenova_plan 支持账号密码模式）：
//     账号密码模式下 newAPIKey 可留空，系统自动登录拿 access_token 作为主凭据。
//   - 填了 loginUsername（仅 sensenova_plan）：保存账号密码，并保存加密 refresh_token 自动续期；
//     不填则清空账号密码模式（切回纯 Bearer Token）。
//   - newForwardAPIKey 仅控制台 token plan 类生效（normalizePlanForwardAPIKey 会清空其他类），传空串表示"清空转发凭据"。
//   - 用新凭据立即查询一次用量并更新 quota/balance 字段（等价于一次 RefreshProvider）。
//   - forward_api_key 变更且关联渠道存在时，同步更新渠道里匹配旧 forward 值的那把 key；
//     若原本没有渠道（旧 forward 为空）而本次填了新 forward，则新建/复用渠道（逻辑同 AddProvider）。
func UpdateProviderCredentials(ctx context.Context, id int, newAPIKey, newForwardAPIKey string, newTeamOrgID, newTeamProjectID, loginUsername, loginPassword string) (*model.PlanProvider, error) {
	var provider model.PlanProvider
	if err := db.GetDB().WithContext(ctx).First(&provider, id).Error; err != nil {
		return nil, fmt.Errorf("find plan provider: %w", err)
	}
	// 凭据密文解密回明文供比较/查询使用；Save 前会重新加密。
	decryptProviderSecrets(&provider)

	info := getCategoryInfo(provider.Category)
	if info == nil {
		return nil, fmt.Errorf("unknown plan provider category: %s", provider.Category)
	}

	newAPIKey = strings.TrimSpace(newAPIKey)
	loginUsername = strings.TrimSpace(loginUsername)
	loginPassword = strings.TrimSpace(loginPassword)
	if newAPIKey == "" && loginUsername == "" {
		return nil, fmt.Errorf("API key or login credentials is required")
	}
	if loginUsername != "" && provider.Category != model.PlanProviderSenseNovaPlan && provider.Category != model.PlanProviderDeepSeek {
		return nil, fmt.Errorf("account login is only supported for sensenova_plan and deepseek")
	}
	if loginUsername != "" && loginPassword == "" {
		return nil, fmt.Errorf("login password is required")
	}

	// 账号密码模式：sensenova 登录获取新 token 作为主凭据并更新登录凭据字段；
	// deepseek 只保存加密账号密码（查询官方 usage 时再登录，token 不落库）。
	// 未填账号密码：清空账号密码模式。
	if loginUsername != "" && provider.Category == model.PlanProviderSenseNovaPlan {
		loginPasswordEnc, err := crypto.Encrypt(loginPassword)
		if err != nil {
			return nil, fmt.Errorf("encrypt login password: %w", err)
		}
		sess, err := senseNovaOIDCLogin(ctx, loginUsername, loginPassword)
		if err != nil {
			return nil, fmt.Errorf("sensenova login: %w", err)
		}
		newAPIKey = sess.accessToken
		provider.LoginUsername = loginUsername
		provider.LoginPasswordEnc = loginPasswordEnc
		provider.RefreshTokenEnc = ""
		if sess.refreshToken != "" {
			if enc, err := crypto.Encrypt(sess.refreshToken); err == nil {
				provider.RefreshTokenEnc = enc
			}
		}
	} else if loginUsername != "" && provider.Category == model.PlanProviderDeepSeek {
		loginPasswordEnc, err := crypto.Encrypt(loginPassword)
		if err != nil {
			return nil, fmt.Errorf("encrypt login password: %w", err)
		}
		provider.LoginUsername = loginUsername
		provider.LoginPasswordEnc = loginPasswordEnc
		provider.RefreshTokenEnc = ""
	} else {
		provider.LoginUsername = ""
		provider.LoginPasswordEnc = ""
		provider.RefreshTokenEnc = ""
	}
	clearSenseNovaSession(id)
	clearDeepSeekSession(id)

	newForwardAPIKey = normalizePlanForwardAPIKey(provider.Category, strings.TrimSpace(newForwardAPIKey))
	newTeamOrgID = strings.TrimSpace(newTeamOrgID)
	newTeamProjectID = strings.TrimSpace(newTeamProjectID)

	oldAPIKey := provider.APIKey
	oldForwardAPIKey := provider.ForwardAPIKey

	// 1. 同步转发凭据到关联渠道（仅控制台 token plan 类，且 forward 值发生变化）。
	if isConsoleTokenPlanCategory(provider.Category) && newForwardAPIKey != oldForwardAPIKey {
		if err := updatePlanForwardChannelKey(ctx, &provider, oldForwardAPIKey, newForwardAPIKey, info); err != nil {
			return nil, fmt.Errorf("update forward channel key: %w", err)
		}
	}

	// DeepSeek 特殊：API key（查余额）与账号密码（查官方 usage）并存。
	// 只填账号密码时保留原 API key，避免被清空导致余额查询失效。
	if provider.Category == model.PlanProviderDeepSeek && newAPIKey == "" {
		newAPIKey = provider.APIKey
	}

	// 2. 用新主凭据查询用量。
	provider.APIKey = newAPIKey
	provider.ForwardAPIKey = newForwardAPIKey
	provider.TeamOrganizationID = newTeamOrgID
	provider.TeamProjectID = newTeamProjectID

	if provider.ProviderType == model.PlanProviderTypeBalance {
		// 换凭据后立即查询用量，等价于一次刷新：同样保存旧值快照，保证增量对比连续，
		// 并把差额累加进累计已用额度。
		lastBalance := provider.Balance
		result, err := QueryBalance(ctx, provider.Category, provider.APIKey, provider.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("query balance: %w", err)
		}
		provider.LastBalance = lastBalance
		provider.Balance = result.Balance
		provider.BalanceUsed = result.BalanceUsed
		provider.TotalTokens = result.TotalTokens
		provider.TotalUsed += max(0, lastBalance-provider.Balance)
	} else {
		lastQuotaUsed := provider.QuotaUsed
		result, err := QueryTokenPlan(ctx, provider.Category, provider.APIKey, provider.BaseURL, provider.ProxyMode, provider.ProxyConfigID, provider.TeamOrganizationID, provider.TeamProjectID)
		if err != nil {
			return nil, fmt.Errorf("query tokenplan: %w", err)
		}
		provider.LastQuotaUsed = lastQuotaUsed
		provider.QuotaTotal = result.QuotaTotal
		provider.QuotaUsed = result.QuotaUsed
		if result.QuotaResetAt != nil {
			provider.QuotaResetAt = result.QuotaResetAt
		}
		provider.WeeklyTotal = result.WeeklyTotal
		provider.WeeklyUsed = result.WeeklyUsed
		if result.WeeklyResetAt != nil {
			provider.WeeklyResetAt = result.WeeklyResetAt
		}
		provider.FiveHourTotal = result.FiveHourTotal
		provider.FiveHourUsed = result.FiveHourUsed
		provider.FiveHourResetAt = result.FiveHourResetAt
	}

	now := time.Now()
	provider.LastRefresh = &now
	provider.UpdatedAt = now

	// 凭据加密落库；Save 后恢复明文（返回给调用方）。
	if err := encryptProviderSecrets(&provider); err != nil {
		return nil, err
	}
	defer decryptProviderSecrets(&provider)
	if err := db.GetDB().WithContext(ctx).Save(&provider).Error; err != nil {
		return nil, fmt.Errorf("save plan provider: %w", err)
	}

	// 记录旧凭据被替换（便于排查；不输出敏感值）。
	log.Infof("planprovider: updated credentials for provider %d (category=%s, api_key changed=%v, forward_api_key changed=%v)",
		id, provider.Category, newAPIKey != oldAPIKey, newForwardAPIKey != oldForwardAPIKey)

	return &provider, nil
}

// encryptProviderSecrets 加密 PlanProvider 的 APIKey/ForwardAPIKey 字段
// （enc: 前缀）用于落库。调用方负责在落库完成后恢复明文。
func encryptProviderSecrets(p *model.PlanProvider) error {
	if p == nil {
		return nil
	}
	if p.APIKey != "" {
		enc, err := crypto.Encrypt(p.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt plan provider api key: %w", err)
		}
		p.APIKey = enc
	}
	if p.ForwardAPIKey != "" {
		enc, err := crypto.Encrypt(p.ForwardAPIKey)
		if err != nil {
			return fmt.Errorf("encrypt plan provider forward api key: %w", err)
		}
		p.ForwardAPIKey = enc
	}
	return nil
}

// decryptProviderSecrets 解密 PlanProvider 的 APIKey/ForwardAPIKey
// （无 enc: 前缀的存量明文原样保留）。
func decryptProviderSecrets(p *model.PlanProvider) {
	if p == nil {
		return
	}
	if plain, err := crypto.Decrypt(p.APIKey); err == nil {
		p.APIKey = plain
	}
	if plain, err := crypto.Decrypt(p.ForwardAPIKey); err == nil {
		p.ForwardAPIKey = plain
	}
}

// updatePlanForwardChannelKey 同步转发凭据变更到 provider 关联的渠道 key。
//
// 三种情形：
//   - 旧 forward 非空 → 新 forward 非空：在渠道 keys 中按旧值匹配那把 key 并更新（ChannelKeyUpdateRequest）。
//   - 旧 forward 非空 → 新 forward 空：匹配到的 key 删除（KeysToDelete）。仅当该 key 仅属于本 provider 的渠道时删除，
//     复用渠道（多 provider 共享）时不删以免误伤。
//   - 旧 forward 空 → 新 forward 非空：原本仅监控无渠道，本次新建渠道并归入 Plan 渠道分组（同 AddProvider）。
func updatePlanForwardChannelKey(ctx context.Context, provider *model.PlanProvider, oldForwardAPIKey, newForwardAPIKey string, info *model.PlanProviderCategoryInfo) error {
	// 原本无关联渠道且本次清空：无操作。
	if provider.ChannelID == 0 && newForwardAPIKey == "" {
		return nil
	}

	// 情形 3：旧 forward 空 → 新 forward 非空，新建渠道。
	if provider.ChannelID == 0 && newForwardAPIKey != "" {
		channelGroupID, err := ensurePlanChannelGroup(ctx)
		if err != nil {
			return fmt.Errorf("ensure plan channel group: %w", err)
		}
		channelBaseURL := planForwardAPIBaseURL(provider.Category)
		channelName := fmt.Sprintf("[%s] %s", planForwardLabel(provider.Category), provider.Name)
		channel := &model.Channel{
			Name:      channelName,
			GroupID:   channelGroupID,
			Type:      outbound.OutboundTypeOpenAIChat,
			Enabled:   true,
			BaseUrls:  []model.BaseUrl{{URL: channelBaseURL, Delay: 0}},
			Keys:      []model.ChannelKey{{Enabled: true, ChannelKey: newForwardAPIKey, Managed: true}},
			Model:     info.Models,
			AutoSync:  false,
			AutoGroup: model.AutoGroupTypeNone,
		}
		if err := op.ChannelCreate(channel, ctx); err != nil {
			return fmt.Errorf("create channel: %w", err)
		}
		provider.ChannelID = channel.ID
		return nil
	}

	// 情形 1/2：有关联渠道，匹配旧 forward 值定位 key。
	ch, err := op.ChannelGet(provider.ChannelID, ctx)
	if err != nil || ch == nil {
		return fmt.Errorf("get channel %d: %w", provider.ChannelID, err)
	}

	// 找到匹配旧 forward 值的 key 行。
	var matchedKeyID int
	for _, k := range ch.Keys {
		if k.ChannelKey == oldForwardAPIKey {
			matchedKeyID = k.ID
			break
		}
	}

	if newForwardAPIKey != "" {
		// 情形 1：更新匹配到的 key；未匹配到（可能 key 已被手动删）则追加新 key。
		if matchedKeyID > 0 {
			updateReq := &model.ChannelUpdateRequest{
				ID: provider.ChannelID,
				KeysToUpdate: []model.ChannelKeyUpdateRequest{
					{ID: matchedKeyID, ChannelKey: &newForwardAPIKey},
				},
			}
			if _, err := op.ChannelUpdate(updateReq, ctx); err != nil {
				return fmt.Errorf("update channel key: %w", err)
			}
		} else {
			addReq := &model.ChannelUpdateRequest{
				ID: provider.ChannelID,
				KeysToAdd: []model.ChannelKeyAddRequest{
					{Enabled: true, ChannelKey: newForwardAPIKey, Managed: true},
				},
			}
			if _, err := op.ChannelUpdate(addReq, ctx); err != nil {
				return fmt.Errorf("add channel key: %w", err)
			}
		}
		return nil
	}

	// 情形 2：新 forward 空，删除匹配到的 key（复用渠道不删以免误伤）。
	if matchedKeyID > 0 && !isReusedChannel(ctx, provider.ChannelID) {
		delReq := &model.ChannelUpdateRequest{
			ID:           provider.ChannelID,
			KeysToDelete: []int{matchedKeyID},
		}
		if _, err := op.ChannelUpdate(delReq, ctx); err != nil {
			return fmt.Errorf("delete channel key: %w", err)
		}
	}
	return nil
}

// DeleteProvider 删除 Plan Provider，同时删除关联的 Channel。
//
// 顺序：先删 provider 记录，再用 op.ChannelDel 清理 channel 及其依赖
// （GroupItems / ChannelKeys / StatsChannel + chCache / keyCache / stats cache + Redis scope）。
// op.ChannelDel 是 channel 删除的统一入口，内部事务性删除并触发 cache 失效，
// 不能用裸 DB 删除替代——否则会留下 chCache/keyCache/stats cache 残留，且非事务性
// （见 issue #126 修复项；对比 task/channel_expire.go 同样使用 ch.Delete + OnChannelDeleted）。
//
// 若 provider 删除成功而 channel 删除失败：provider 已不存在，留下一个 channel 孤儿，
// 用户可在渠道管理页手动删除；这比反过来（provider 指向不存在的 channel）更易恢复。
func DeleteProvider(ctx context.Context, id int) error {
	clearSenseNovaSession(id)
	clearDeepSeekSession(id)
	var provider model.PlanProvider
	if err := db.GetDB().WithContext(ctx).First(&provider, id).Error; err != nil {
		return fmt.Errorf("find plan provider: %w", err)
	}

	// 先删 provider 记录。
	if err := db.GetDB().WithContext(ctx).Delete(&provider).Error; err != nil {
		return fmt.Errorf("delete plan provider: %w", err)
	}

	// 再清理关联的 channel（含 group items / channel keys / stats channel / cache）。
	if provider.ChannelID > 0 {
		if err := op.ChannelDel(provider.ChannelID, ctx); err != nil {
			// channel 可能已被手动删除（channel.Get 失败），或删除过程出错。
			// provider 记录已删，记录日志但不阻塞返回——避免调用方卡在
			// "channel 不存在"的边缘 case 下无法重试（重试时 provider 已不存在）。
			log.Warnf("planprovider: delete channel %d for provider %d failed: %v (provider record already deleted)", provider.ChannelID, id, err)
		}
	}

	return nil
}

// GetCategories 获取所有支持的厂商分类
func GetCategories(providerType model.PlanProviderType) []model.PlanProviderCategoryInfo {
	result := make([]model.PlanProviderCategoryInfo, 0)
	for _, c := range model.PlanProviderCategories {
		if c.Type == providerType {
			result = append(result, c)
		}
	}
	return result
}

// --- internal helpers ---

func getCategoryInfo(category model.PlanProviderCategory) *model.PlanProviderCategoryInfo {
	for _, c := range model.PlanProviderCategories {
		if c.Category == category {
			return &c
		}
	}
	return nil
}

// ensurePlanChannelGroup 确保名为 Plan 的渠道分组（ChannelGroup）存在，返回其 ID。
// 与 ensurePlanGroup 不同：后者操作路由分组（Group 表），本函数操作渠道分组
// （ChannelGroup 表），用于 channel.GroupID 字段的归属。
func ensurePlanChannelGroup(ctx context.Context) (int, error) {
	groups, err := op.ChannelGroupList(ctx)
	if err != nil {
		return 0, fmt.Errorf("list channel groups: %w", err)
	}
	for _, g := range groups {
		if g.Name == planChannelGroupName {
			return g.ID, nil
		}
	}
	newGroup, err := op.ChannelGroupCreate(planChannelGroupName, ctx)
	if err != nil {
		return 0, fmt.Errorf("create plan channel group: %w", err)
	}
	return newGroup.ID, nil
}

// stepFunPlanAPIBaseURL 是 StepFun 套餐转发的 API 接入点。
const stepFunPlanAPIBaseURL = "https://api.stepfun.com/step_plan/v1"

// senseNovaPlanAPIBaseURL 是 SenseNova 套餐转发的 API 接入点。
const senseNovaPlanAPIBaseURL = "https://token.sensenova.cn/v1"

// bailianPlanAPIBaseURL 是百炼 Token Plan 转发的 API 接入点。
const bailianPlanAPIBaseURL = "https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1"

// volcenginePlanAPIBaseURL 是火山方舟 Agent Plan 转发的 API 接入点（OpenAI 兼容）。
const volcenginePlanAPIBaseURL = "https://ark.cn-beijing.volces.com/api/plan/v3"

func normalizePlanForwardAPIKey(category model.PlanProviderCategory, forwardAPIKey string) string {
	if !isConsoleTokenPlanCategory(category) {
		return ""
	}
	return forwardAPIKey
}

// isConsoleTokenPlanCategory 判断是否为"控制台 token plan"类厂商
// （使用控制台会话 token 查套餐、可选 sk- key 创建转发渠道的厂商）。
func isConsoleTokenPlanCategory(category model.PlanProviderCategory) bool {
	return category == model.PlanProviderStepFunPlan || category == model.PlanProviderSenseNovaPlan || category == model.PlanProviderBailianPlan || category == model.PlanProviderVolcenginePlan || category == model.PlanProviderVolcenginePlanAK
}

// planForwardAPIBaseURL 返回控制台 token plan 类厂商的转发 API 接入点。
func planForwardAPIBaseURL(category model.PlanProviderCategory) string {
	switch category {
	case model.PlanProviderStepFunPlan:
		return stepFunPlanAPIBaseURL
	case model.PlanProviderSenseNovaPlan:
		return senseNovaPlanAPIBaseURL
	case model.PlanProviderBailianPlan:
		return bailianPlanAPIBaseURL
	case model.PlanProviderVolcenginePlan, model.PlanProviderVolcenginePlanAK:
		return volcenginePlanAPIBaseURL
	default:
		return ""
	}
}

// planForwardLabel 返回控制台 token plan 类厂商的渠道名前缀标签。
func planForwardLabel(category model.PlanProviderCategory) string {
	switch category {
	case model.PlanProviderStepFunPlan:
		return "StepFun Plan"
	case model.PlanProviderSenseNovaPlan:
		return "SenseNova Plan"
	case model.PlanProviderBailianPlan:
		return "Bailian Plan"
	case model.PlanProviderVolcenginePlan, model.PlanProviderVolcenginePlanAK:
		return "Volcengine Plan"
	default:
		return "Plan"
	}
}

// findReusablePlanChannel 查找可复用的 Plan 渠道。
// 条件：已有同 category 的 PlanProvider 记录，且其关联渠道的接入点和模型列表均相同。
// 返回渠道 ID，未找到返回 0。
func findReusablePlanChannel(ctx context.Context, category model.PlanProviderCategory, baseURL, modelList string) int {
	var providers []model.PlanProvider
	if err := db.GetDB().WithContext(ctx).
		Where("category = ? AND channel_id > 0", category).
		Find(&providers).Error; err != nil {
		return 0
	}
	normalizedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	for _, p := range providers {
		ch, err := op.ChannelGet(p.ChannelID, ctx)
		if err != nil || ch == nil {
			continue
		}
		if len(ch.BaseUrls) == 0 {
			continue
		}
		chBaseURL := strings.TrimRight(strings.TrimSpace(ch.BaseUrls[0].URL), "/")
		if chBaseURL != normalizedBaseURL {
			continue
		}
		// 模型列表相同则复用（模型相同的合并）
		if normalizeModelList(ch.Model) == normalizeModelList(modelList) {
			return ch.ID
		}
	}
	return 0
}

// isReusedChannel 判断渠道是否是已有 provider 关联的复用渠道（非本次新建）。
// 通过查 PlanProvider 表看是否有其他 provider 也指向该渠道。
func isReusedChannel(ctx context.Context, channelID int) bool {
	var count int64
	db.GetDB().WithContext(ctx).Model(&model.PlanProvider{}).
		Where("channel_id = ?", channelID).Count(&count)
	return count > 0
}

// normalizeModelList 规范化模型列表用于比较：去空格、去重、排序后拼接。
func normalizeModelList(modelList string) string {
	parts := strings.Split(modelList, ",")
	seen := make(map[string]struct{})
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		result = append(result, p)
	}
	// 排序保证顺序无关
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i] > result[j] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return strings.Join(result, ",")
}
