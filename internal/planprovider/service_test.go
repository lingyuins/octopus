package planprovider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
	"github.com/lingyuins/octopus/internal/op/channel"
	stats "github.com/lingyuins/octopus/internal/op/stats"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

// withVolcenginePlanServer 起一个 mock GetAgentPlanAFPUsage 服务，返回固定用量。
// 返回的请求头捕获变量供断言新凭据是否被携带。
func withVolcenginePlanServer(t *testing.T) *string {
	t.Helper()
	gotCred := ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCred = r.Header.Get("Cookie") + "|||" + r.Header.Get("x-csrf-token")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(volcengineUsageResponse))
	}))
	old := volcenginePlanUsageURL
	volcenginePlanUsageURL = ts.URL
	t.Cleanup(func() {
		volcenginePlanUsageURL = old
		ts.Close()
	})
	return &gotCred
}

// withDeepSeekBalanceServer 起一个 mock DeepSeek 余额查询服务。
func withDeepSeekBalanceServer(t *testing.T, balance string) *string {
	t.Helper()
	gotKey := ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"is_available":true,"balance_infos":[{"total_balance":"` + balance + `","currency":"CNY"}]}`))
	}))
	old := deepSeekBalanceURL
	deepSeekBalanceURL = ts.URL
	t.Cleanup(func() {
		deepSeekBalanceURL = old
		ts.Close()
	})
	return &gotKey
}

// createProviderRow 直接写一条 PlanProvider 行（绕过 AddProvider 的渠道创建链路）。
func createProviderRow(t *testing.T, p *model.PlanProvider) *model.PlanProvider {
	t.Helper()
	now := time.Now()
	p.LastRefresh = &now
	p.CreatedAt = now
	p.UpdatedAt = now
	if err := db.GetDB().Create(p).Error; err != nil {
		t.Fatalf("create provider row: %v", err)
	}
	return p
}

// TestUpdateProviderCredentials_VolcengineNewAPIKeyRefreshesUsage 验证：
// 控制台会话凭据（火山方舟）失效后，仅换主凭据（Cookie|||csrf）、forward 不变时，
// 新凭据落库、用量用新凭据查询、forward_api_key 与 channel 不被触碰。
func TestUpdateProviderCredentials_VolcengineNewAPIKeyRefreshesUsage(t *testing.T) {
	setupPlanProviderDB(t)
	gotCred := withVolcenginePlanServer(t)

	const oldAPIKey = "sessionid=expired|||csrf-old"
	const newAPIKey = "sessionid=fresh|||csrf-new"
	const forwardKey = "ark-forward-secret"

	provider := createProviderRow(t, &model.PlanProvider{
		Name:          "Volcengine monitor",
		Category:      model.PlanProviderVolcenginePlan,
		ProviderType:  model.PlanProviderTypeTokenPlan,
		APIKey:        oldAPIKey,
		ForwardAPIKey: forwardKey,
		BaseURL:       "https://console.volcengine.com",
		QuotaTotal:    1,
		QuotaUsed:     1,
	})

	updated, err := UpdateProviderCredentials(context.Background(), provider.ID, newAPIKey, forwardKey, "", "", "", "")
	if err != nil {
		t.Fatalf("UpdateProviderCredentials() error = %v", err)
	}

	if updated.APIKey != newAPIKey {
		t.Errorf("APIKey = %q, want %q", updated.APIKey, newAPIKey)
	}
	if updated.ForwardAPIKey != forwardKey {
		t.Errorf("ForwardAPIKey = %q, want %q (不应改变)", updated.ForwardAPIKey, forwardKey)
	}
	// 新凭据被携带到查询请求
	if *gotCred != newAPIKey {
		t.Errorf("查询请求携带的凭据 = %q, want %q", *gotCred, newAPIKey)
	}
	// 用量被刷新（mock 返回 QuotaTotal=100000, QuotaUsed=35210.3625）
	if updated.QuotaTotal != 100000 {
		t.Errorf("QuotaTotal = %v, want 100000", updated.QuotaTotal)
	}
	if updated.QuotaUsed != 35210.3625 {
		t.Errorf("QuotaUsed = %v, want 35210.3625", updated.QuotaUsed)
	}
	// 换凭据等价于一次刷新：旧已用量应存入快照，保证增量对比连续
	if updated.LastQuotaUsed != 1 {
		t.Errorf("LastQuotaUsed = %v, want 1 (旧 QuotaUsed 快照)", updated.LastQuotaUsed)
	}

	// DB 持久化校验
	var stored model.PlanProvider
	if err := db.GetDB().First(&stored, provider.ID).Error; err != nil {
		t.Fatalf("load stored: %v", err)
	}
	// APIKey/ForwardAPIKey 密文落库（enc: 前缀），解密后应与原文一致。
	decAPIKey, err1 := crypto.Decrypt(stored.APIKey)
	decForward, err2 := crypto.Decrypt(stored.ForwardAPIKey)
	if err1 != nil || !strings.HasPrefix(stored.APIKey, "enc:") {
		t.Errorf("stored APIKey = %q, want enc: prefixed ciphertext (err=%v)", stored.APIKey, err1)
	}
	if decAPIKey != newAPIKey {
		t.Errorf("decrypted APIKey = %q, want %q", decAPIKey, newAPIKey)
	}
	if err2 != nil || !strings.HasPrefix(stored.ForwardAPIKey, "enc:") {
		t.Errorf("stored ForwardAPIKey = %q, want enc: prefixed ciphertext (err=%v)", stored.ForwardAPIKey, err2)
	}
	if decForward != forwardKey {
		t.Errorf("decrypted ForwardAPIKey = %q, want %q", decForward, forwardKey)
	}
}

// TestUpdateProviderCredentials_BalanceClearsForwardAPIKey 验证：
// 非控制台类（balance deepseek）换 api_key 时，forward_api_key 被清空（normalizePlanForwardAPIKey 行为）。
func TestUpdateProviderCredentials_BalanceClearsForwardAPIKey(t *testing.T) {
	setupPlanProviderDB(t)
	gotKey := withDeepSeekBalanceServer(t, "123.45")

	const oldAPIKey = "sk-old-deepseek"
	const newAPIKey = "sk-new-deepseek"

	provider := createProviderRow(t, &model.PlanProvider{
		Name:          "DeepSeek monitor",
		Category:      model.PlanProviderDeepSeek,
		ProviderType:  model.PlanProviderTypeBalance,
		APIKey:        oldAPIKey,
		ForwardAPIKey: "should-be-cleared",
		BaseURL:       "https://api.deepseek.com/v1",
		Balance:       0,
	})

	updated, err := UpdateProviderCredentials(context.Background(), provider.ID, newAPIKey, "ignored-forward", "", "", "", "")
	if err != nil {
		t.Fatalf("UpdateProviderCredentials() error = %v", err)
	}

	if updated.APIKey != newAPIKey {
		t.Errorf("APIKey = %q, want %q", updated.APIKey, newAPIKey)
	}
	if updated.ForwardAPIKey != "" {
		t.Errorf("ForwardAPIKey = %q, want empty (非控制台类应清空)", updated.ForwardAPIKey)
	}
	// 新凭据携带 Bearer 前缀
	if !strings.HasPrefix(*gotKey, "Bearer sk-new-deepseek") {
		t.Errorf("查询请求 Authorization = %q, 应携带新 key", *gotKey)
	}
	if updated.Balance != 123.45 {
		t.Errorf("Balance = %v, want 123.45", updated.Balance)
	}
}

// TestUpdateProviderCredentials_EmptyAPIKeyErrors 验证空 apiKey 报错。
func TestUpdateProviderCredentials_EmptyAPIKeyErrors(t *testing.T) {
	setupPlanProviderDB(t)
	provider := createProviderRow(t, &model.PlanProvider{
		Name:         "test",
		Category:     model.PlanProviderVolcenginePlan,
		ProviderType: model.PlanProviderTypeTokenPlan,
		APIKey:       "old",
		BaseURL:      "https://console.volcengine.com",
	})

	if _, err := UpdateProviderCredentials(context.Background(), provider.ID, "   ", "", "", "", "", ""); err == nil {
		t.Fatal("空 apiKey 应报错")
	}
}

// TestUpdateProviderCredentials_NotFound 验证不存在的 ID 报错。
func TestUpdateProviderCredentials_NotFound(t *testing.T) {
	setupPlanProviderDB(t)
	if _, err := UpdateProviderCredentials(context.Background(), 99999, "k", "", "", "", "", ""); err == nil {
		t.Fatal("不存在的 provider 应报错")
	}
}

// TestAddProviderDeepSeekUsesFetchedModels 验证：添加 DeepSeek 额度监控时，
// 自动创建的渠道模型列表来自上游拉取的最新模型，而非硬编码默认值。
func TestAddProviderDeepSeekUsesFetchedModels(t *testing.T) {
	setupPlanProviderDB(t)
	withDeepSeekBalanceServer(t, "123.45")

	orig := planFetchModels
	planFetchModels = func(ctx context.Context, channelType outbound.OutboundType, baseURL, apiKey string) ([]string, error) {
		if channelType != outbound.OutboundTypeOpenAIChat {
			t.Errorf("fetch channel type = %q, want %q", channelType, outbound.OutboundTypeOpenAIChat)
		}
		return []string{"deepseek-reasoner", "deepseek-chat", "deepseek-v4-pro"}, nil
	}
	t.Cleanup(func() { planFetchModels = orig })

	provider, err := AddProvider(context.Background(), model.PlanProviderDeepSeek, "sk-test-deepseek", "", "", 0, model.ProxyUsageModeDirect, nil, "", "", "", "")
	if err != nil {
		t.Fatalf("AddProvider() error = %v", err)
	}
	if provider.ChannelID <= 0 {
		t.Fatal("provider.ChannelID = 0, want created channel")
	}
	var ch model.Channel
	if err := db.GetDB().First(&ch, provider.ChannelID).Error; err != nil {
		t.Fatalf("load channel: %v", err)
	}
	want := normalizeModelList("deepseek-reasoner,deepseek-chat,deepseek-v4-pro")
	if normalizeModelList(ch.Model) != want {
		t.Errorf("channel.Model = %q, want %q (应来自上游拉取)", ch.Model, want)
	}
}

// TestAddProviderDeepSeekFallsBackToDefaultModels 验证：上游拉取失败时回退硬编码默认列表。
func TestAddProviderDeepSeekFallsBackToDefaultModels(t *testing.T) {
	setupPlanProviderDB(t)
	withDeepSeekBalanceServer(t, "123.45")

	orig := planFetchModels
	planFetchModels = func(ctx context.Context, channelType outbound.OutboundType, baseURL, apiKey string) ([]string, error) {
		return nil, fmt.Errorf("upstream /models unavailable")
	}
	t.Cleanup(func() { planFetchModels = orig })

	provider, err := AddProvider(context.Background(), model.PlanProviderDeepSeek, "sk-test-deepseek", "", "", 0, model.ProxyUsageModeDirect, nil, "", "", "", "")
	if err != nil {
		t.Fatalf("AddProvider() error = %v", err)
	}
	var ch model.Channel
	if err := db.GetDB().First(&ch, provider.ChannelID).Error; err != nil {
		t.Fatalf("load channel: %v", err)
	}
	want := normalizeModelList("deepseek-v4-flash,deepseek-v4-pro")
	if normalizeModelList(ch.Model) != want {
		t.Errorf("channel.Model = %q, want fallback %q", ch.Model, "deepseek-v4-flash,deepseek-v4-pro")
	}
}

// TestResolvePlanChannelModelsSkipsCodex 验证：Codex 跳过上游拉取（OAuth JSON 无法作 Bearer），
// 直接返回默认列表且不发起请求。
func TestResolvePlanChannelModelsSkipsCodex(t *testing.T) {
	called := false
	orig := planFetchModels
	planFetchModels = func(ctx context.Context, channelType outbound.OutboundType, baseURL, apiKey string) ([]string, error) {
		called = true
		return nil, nil
	}
	t.Cleanup(func() { planFetchModels = orig })

	got := resolvePlanChannelModels(context.Background(), model.PlanProviderCodex, outbound.OutboundTypeCodex, "https://chatgpt.com", "oauth-json", "gpt-5")
	if got != "gpt-5" {
		t.Errorf("resolvePlanChannelModels(codex) = %q, want %q", got, "gpt-5")
	}
	if called {
		t.Error("codex should not trigger model fetch")
	}
}

// TestRefreshProviderBalanceSnapshot 验证：balance 类刷新时旧余额被存入 LastBalance，
// 两次检测之间的增量 = 上次余额 − 本次余额。
func TestRefreshProviderBalanceSnapshot(t *testing.T) {
	setupPlanProviderDB(t)
	gotKey := withDeepSeekBalanceServer(t, "100")

	provider := createProviderRow(t, &model.PlanProvider{
		Name:         "DeepSeek monitor",
		Category:     model.PlanProviderDeepSeek,
		ProviderType: model.PlanProviderTypeBalance,
		APIKey:       "sk-test-deepseek",
		BaseURL:      "https://api.deepseek.com/v1",
		Balance:      120, // 首次快照后的余额
		LastBalance:  120, // 首次添加时建立的快照
	})

	refreshed, err := RefreshProvider(context.Background(), provider.ID)
	if err != nil {
		t.Fatalf("RefreshProvider() error = %v", err)
	}
	if refreshed.LastBalance != 120 {
		t.Errorf("LastBalance = %v, want 120 (上次检测时的余额)", refreshed.LastBalance)
	}
	if refreshed.Balance != 100 {
		t.Errorf("Balance = %v, want 100", refreshed.Balance)
	}
	if *gotKey == "" {
		t.Error("balance query should carry Authorization header")
	}

	// 再刷新一次：上次快照滚动为 100
	withDeepSeekBalanceServer(t, "80")
	refreshed, err = RefreshProvider(context.Background(), provider.ID)
	if err != nil {
		t.Fatalf("RefreshProvider() second error = %v", err)
	}
	if refreshed.LastBalance != 100 {
		t.Errorf("LastBalance = %v, want 100", refreshed.LastBalance)
	}
	if refreshed.Balance != 80 {
		t.Errorf("Balance = %v, want 80", refreshed.Balance)
	}
}

// TestRefreshProviderTokenPlanSnapshot 验证：tokenplan 类刷新时旧已用量被存入 LastQuotaUsed。
func TestRefreshProviderTokenPlanSnapshot(t *testing.T) {
	setupPlanProviderDB(t)
	withVolcenginePlanServer(t)

	provider := createProviderRow(t, &model.PlanProvider{
		Name:          "Volcengine monitor",
		Category:      model.PlanProviderVolcenginePlan,
		ProviderType:  model.PlanProviderTypeTokenPlan,
		APIKey:        "sessionid=abc|||csrf-def",
		ForwardAPIKey: "ark-forward-secret",
		BaseURL:       "https://console.volcengine.com",
		QuotaUsed:     10,
		LastQuotaUsed: 10,
	})

	refreshed, err := RefreshProvider(context.Background(), provider.ID)
	if err != nil {
		t.Fatalf("RefreshProvider() error = %v", err)
	}
	if refreshed.LastQuotaUsed != 10 {
		t.Errorf("LastQuotaUsed = %v, want 10", refreshed.LastQuotaUsed)
	}
	if refreshed.QuotaUsed != 35210.3625 {
		t.Errorf("QuotaUsed = %v, want 35210.3625 (mock 返回值)", refreshed.QuotaUsed)
	}
}

// TestListProvidersDeltas 验证：列表响应中的增量字段——balance 类为余额减少额，
// tokenplan 类为已用量增量；充值/周期重置导致的负值按 0 处理。
func TestListProvidersDeltas(t *testing.T) {
	setupPlanProviderDB(t)

	// balance 类：正常消耗 20
	createProviderRow(t, &model.PlanProvider{
		Name:         "DeepSeek spent",
		Category:     model.PlanProviderDeepSeek,
		ProviderType: model.PlanProviderTypeBalance,
		APIKey:       "sk-1",
		BaseURL:      "https://api.deepseek.com/v1",
		Balance:      80,
		LastBalance:  100,
	})
	// balance 类：充值导致余额增加 → delta 按 0
	createProviderRow(t, &model.PlanProvider{
		Name:         "DeepSeek topped-up",
		Category:     model.PlanProviderDeepSeek,
		ProviderType: model.PlanProviderTypeBalance,
		APIKey:       "sk-2",
		BaseURL:      "https://api.deepseek.com/v1",
		Balance:      150,
		LastBalance:  100,
	})
	// tokenplan 类：已用增加 500
	createProviderRow(t, &model.PlanProvider{
		Name:          "Volcengine used",
		Category:      model.PlanProviderVolcenginePlan,
		ProviderType:  model.PlanProviderTypeTokenPlan,
		APIKey:        "session=1",
		BaseURL:       "https://console.volcengine.com",
		QuotaUsed:     1000,
		LastQuotaUsed: 500,
	})
	// tokenplan 类：周期重置（已用减少）→ delta 按 0
	createProviderRow(t, &model.PlanProvider{
		Name:          "Volcengine reset",
		Category:      model.PlanProviderVolcenginePlan,
		ProviderType:  model.PlanProviderTypeTokenPlan,
		APIKey:        "session=2",
		BaseURL:       "https://console.volcengine.com",
		QuotaUsed:     50,
		LastQuotaUsed: 1000,
	})

	items, err := ListProviders(context.Background(), model.PlanProviderTypeBalance)
	if err != nil {
		t.Fatalf("ListProviders(balance) error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("balance providers = %d, want 2", len(items))
	}
	for _, item := range items {
		switch item.Name {
		case "DeepSeek spent":
			if item.BalanceDelta != 20 {
				t.Errorf("spent BalanceDelta = %v, want 20", item.BalanceDelta)
			}
		case "DeepSeek topped-up":
			if item.BalanceDelta != 0 {
				t.Errorf("topped-up BalanceDelta = %v, want 0 (充值负值按 0)", item.BalanceDelta)
			}
		}
	}

	tokenItems, err := ListProviders(context.Background(), model.PlanProviderTypeTokenPlan)
	if err != nil {
		t.Fatalf("ListProviders(tokenplan) error = %v", err)
	}
	if len(tokenItems) != 2 {
		t.Fatalf("tokenplan providers = %d, want 2", len(tokenItems))
	}
	for _, item := range tokenItems {
		switch item.Name {
		case "Volcengine used":
			if item.QuotaUsedDelta != 500 {
				t.Errorf("used QuotaUsedDelta = %v, want 500", item.QuotaUsedDelta)
			}
		case "Volcengine reset":
			if item.QuotaUsedDelta != 0 {
				t.Errorf("reset QuotaUsedDelta = %v, want 0 (周期重置负值按 0)", item.QuotaUsedDelta)
			}
		}
	}
}

// TestAddProviderStoresRefreshInterval 验证：refreshIntervalMin 参数被持久化。
func TestAddProviderStoresRefreshInterval(t *testing.T) {
	setupPlanProviderDB(t)
	withDeepSeekBalanceServer(t, "100")

	provider, err := AddProvider(context.Background(), model.PlanProviderDeepSeek, "sk-test-deepseek", "", "", 15, model.ProxyUsageModeDirect, nil, "", "", "", "")
	if err != nil {
		t.Fatalf("AddProvider() error = %v", err)
	}
	if provider.RefreshIntervalMin != 15 {
		t.Errorf("RefreshIntervalMin = %d, want 15", provider.RefreshIntervalMin)
	}
	if provider.LastBalance != provider.Balance {
		t.Errorf("LastBalance = %v, want = Balance %v (首次添加快照)", provider.LastBalance, provider.Balance)
	}

	// 负值应报错
	if _, err := AddProvider(context.Background(), model.PlanProviderDeepSeek, "sk-test-deepseek", "", "", -1, model.ProxyUsageModeDirect, nil, "", "", "", ""); err == nil {
		t.Error("negative refresh interval should error")
	}
}

// TestRefreshProviderTotalUsedAccumulates 验证：累计已用额度逐次累加消费增量，
// 充值导致的负增量不累加（也不减少累计值）。
func TestRefreshProviderTotalUsedAccumulates(t *testing.T) {
	setupPlanProviderDB(t)
	withDeepSeekBalanceServer(t, "100")

	provider := createProviderRow(t, &model.PlanProvider{
		Name:         "DeepSeek monitor",
		Category:     model.PlanProviderDeepSeek,
		ProviderType: model.PlanProviderTypeBalance,
		APIKey:       "sk-test-deepseek",
		BaseURL:      "https://api.deepseek.com/v1",
		Balance:      200,
		LastBalance:  200,
	})

	// 第一次刷新：消费 100（200 → 100），累计 100
	withDeepSeekBalanceServer(t, "100")
	refreshed, err := RefreshProvider(context.Background(), provider.ID)
	if err != nil {
		t.Fatalf("RefreshProvider() error = %v", err)
	}
	if refreshed.TotalUsed != 100 {
		t.Errorf("TotalUsed = %v, want 100 (200→100)", refreshed.TotalUsed)
	}

	// 第二次刷新：充值 400（100 → 500），负增量不累加
	withDeepSeekBalanceServer(t, "500")
	refreshed, err = RefreshProvider(context.Background(), provider.ID)
	if err != nil {
		t.Fatalf("RefreshProvider() second error = %v", err)
	}
	if refreshed.TotalUsed != 100 {
		t.Errorf("TotalUsed = %v, want 100 (充值段不累加)", refreshed.TotalUsed)
	}

	// 第三次刷新：消费 300（500 → 200），累计 400
	withDeepSeekBalanceServer(t, "200")
	refreshed, err = RefreshProvider(context.Background(), provider.ID)
	if err != nil {
		t.Fatalf("RefreshProvider() third error = %v", err)
	}
	if refreshed.TotalUsed != 400 {
		t.Errorf("TotalUsed = %v, want 400 (100+300)", refreshed.TotalUsed)
	}
}

// TestUpdateProviderCredentialsAccumulatesTotalUsed 验证：换凭据（等价刷新）也累加累计已用。
func TestUpdateProviderCredentialsAccumulatesTotalUsed(t *testing.T) {
	setupPlanProviderDB(t)
	withDeepSeekBalanceServer(t, "80")

	provider := createProviderRow(t, &model.PlanProvider{
		Name:         "DeepSeek monitor",
		Category:     model.PlanProviderDeepSeek,
		ProviderType: model.PlanProviderTypeBalance,
		APIKey:       "sk-old",
		BaseURL:      "https://api.deepseek.com/v1",
		Balance:      100,
		LastBalance:  100,
		TotalUsed:    50,
	})

	updated, err := UpdateProviderCredentials(context.Background(), provider.ID, "sk-new", "", "", "", "", "")
	if err != nil {
		t.Fatalf("UpdateProviderCredentials() error = %v", err)
	}
	if updated.Balance != 80 {
		t.Errorf("Balance = %v, want 80", updated.Balance)
	}
	if updated.TotalUsed != 70 {
		t.Errorf("TotalUsed = %v, want 70 (50 + 20)", updated.TotalUsed)
	}
}

// TestMigrateLegacyDeepSeekChannels 验证：模型列表恰为旧默认（deepseek-chat,deepseek-reasoner）
// 的 DeepSeek 额度渠道被迁移为 v4 默认；用户自定义模型的渠道不被触碰；幂等。
func TestMigrateLegacyDeepSeekChannels(t *testing.T) {
	setupPlanProviderDB(t)

	// Plan 分组
	group := &model.ChannelGroup{Name: "Plan"}
	if err := db.GetDB().Create(group).Error; err != nil {
		t.Fatalf("create plan group: %v", err)
	}

	mkChannel := func(name, models string) *model.Channel {
		ch := &model.Channel{
			Name:    name,
			GroupID: group.ID,
			Type:    outbound.OutboundTypeOpenAIChat,
			Enabled: true,
			BaseUrls: []model.BaseUrl{
				{URL: "https://api.deepseek.com/v1", Delay: 0},
			},
			Keys:      []model.ChannelKey{{Enabled: true, ChannelKey: "sk-test"}},
			Model:     models,
			AutoSync:  false,
			AutoGroup: model.AutoGroupTypeNone,
		}
		if err := op.ChannelCreate(ch, context.Background()); err != nil {
			t.Fatalf("create channel %s: %v", name, err)
		}
		return ch
	}

	legacyCh := mkChannel("[DeepSeek] legacy", "deepseek-chat,deepseek-reasoner")
	customCh := mkChannel("[DeepSeek] custom", "deepseek-v4-pro,my-custom-model")

	createProviderRow(t, &model.PlanProvider{
		Name:         "DeepSeek legacy",
		Category:     model.PlanProviderDeepSeek,
		ProviderType: model.PlanProviderTypeBalance,
		APIKey:       "sk-1",
		BaseURL:      "https://api.deepseek.com/v1",
		ChannelID:    legacyCh.ID,
	})
	createProviderRow(t, &model.PlanProvider{
		Name:         "DeepSeek custom",
		Category:     model.PlanProviderDeepSeek,
		ProviderType: model.PlanProviderTypeBalance,
		APIKey:       "sk-2",
		BaseURL:      "https://api.deepseek.com/v1",
		ChannelID:    customCh.ID,
	})

	migrated, err := MigrateLegacyDeepSeekChannels(context.Background())
	if err != nil {
		t.Fatalf("MigrateLegacyDeepSeekChannels() error = %v", err)
	}
	if migrated != 1 {
		t.Errorf("migrated = %d, want 1", migrated)
	}

	var legacyAfter model.Channel
	if err := db.GetDB().First(&legacyAfter, legacyCh.ID).Error; err != nil {
		t.Fatalf("load legacy channel: %v", err)
	}
	if normalizeModelList(legacyAfter.Model) != "deepseek-v4-flash,deepseek-v4-pro" {
		t.Errorf("legacy channel Model = %q, want v4 default", legacyAfter.Model)
	}

	var customAfter model.Channel
	if err := db.GetDB().First(&customAfter, customCh.ID).Error; err != nil {
		t.Fatalf("load custom channel: %v", err)
	}
	if customAfter.Model != "deepseek-v4-pro,my-custom-model" {
		t.Errorf("custom channel Model = %q, want unchanged", customAfter.Model)
	}

	// 幂等：再次运行不再迁移
	migrated, err = MigrateLegacyDeepSeekChannels(context.Background())
	if err != nil {
		t.Fatalf("MigrateLegacyDeepSeekChannels() second error = %v", err)
	}
	if migrated != 0 {
		t.Errorf("second migrated = %d, want 0 (幂等)", migrated)
	}
}

// TestQueryPlanChannelStatsTodayDate 验证：今日统计使用与写入端一致的日期格式
// （YYYYMMDD + stats 时区），仅匹配今日行，不串入历史行。
func TestQueryPlanChannelStatsTodayDate(t *testing.T) {
	setupPlanProviderDB(t)

	today := stats.Now().Format("20060102")
	yesterday := time.Now().AddDate(0, 0, -1).Format("20060102")
	rows := []model.StatsDailyChannel{
		{Date: today, ChannelID: 42, StatsMetrics: model.StatsMetrics{RequestSuccess: 7, RequestFailed: 1, InputToken: 1000, OutputToken: 2000}},
		{Date: yesterday, ChannelID: 42, StatsMetrics: model.StatsMetrics{RequestSuccess: 99, RequestFailed: 1, InputToken: 10000, OutputToken: 20000}},
	}
	if err := db.GetDB().Create(&rows).Error; err != nil {
		t.Fatalf("create daily stats: %v", err)
	}

	got := queryPlanChannelStats(context.Background(), nil, 42)
	if got == nil {
		t.Fatal("queryPlanChannelStats() = nil")
	}
	if got.TodayRequests != 8 {
		t.Errorf("TodayRequests = %d, want 8 (仅今日行)", got.TodayRequests)
	}
	if got.TodayTokens != 3000 {
		t.Errorf("TodayTokens = %d, want 3000 (仅今日行 token 合计)", got.TodayTokens)
	}
	if got.TotalRequests != 0 {
		t.Errorf("TotalRequests = %d, want 0 (未写 stats_channel 累计表)", got.TotalRequests)
	}
}

// TestUpdateProviderCredentials_VolcengineNewForwardKeySyncsChannelKey 验证：
// 控制台类换 forward key 时，渠道里匹配旧 forward 值的那把 key 被更新为新值。
func TestUpdateProviderCredentials_VolcengineNewForwardKeySyncsChannelKey(t *testing.T) {
	setupPlanProviderDB(t)
	withVolcenginePlanServer(t)

	const oldAPIKey = "sessionid=ok|||csrf"
	const oldForward = "ark-old-forward"
	const newForward = "ark-new-forward"

	// 直接构造 channel + key + provider 行（绕过 AddProvider）
	planGroup := &model.ChannelGroup{Name: "Plan"}
	if err := db.GetDB().Create(planGroup).Error; err != nil {
		t.Fatalf("create channel group: %v", err)
	}
	ch := &model.Channel{
		Name:     "[Volcengine] test",
		Type:     outbound.OutboundTypeOpenAIChat,
		Enabled:  true,
		BaseUrls: []model.BaseUrl{{URL: "https://ark.cn-beijing.volces.com/api/plan/v3"}},
		Keys: []model.ChannelKey{
			{Enabled: true, ChannelKey: oldForward},
		},
		Model: "auto,doubao-seed-evolving",
	}
	if err := db.GetDB().Create(ch).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := channel.RefreshCache(context.Background()); err != nil {
		t.Fatalf("RefreshCache: %v", err)
	}

	provider := createProviderRow(t, &model.PlanProvider{
		Name:          "Volcengine monitor",
		Category:      model.PlanProviderVolcenginePlan,
		ProviderType:  model.PlanProviderTypeTokenPlan,
		APIKey:        oldAPIKey,
		ForwardAPIKey: oldForward,
		BaseURL:       "https://console.volcengine.com",
		ChannelID:     ch.ID,
	})

	if _, err := UpdateProviderCredentials(context.Background(), provider.ID, oldAPIKey, newForward, "", "", "", ""); err != nil {
		t.Fatalf("UpdateProviderCredentials() error = %v", err)
	}

	// 校验渠道 key 已更新为新值
	updatedCh, err := op.ChannelGet(ch.ID, context.Background())
	if err != nil {
		t.Fatalf("ChannelGet: %v", err)
	}
	var keyValues []string
	for _, k := range updatedCh.Keys {
		keyValues = append(keyValues, k.ChannelKey)
	}
	foundNew := false
	for _, v := range keyValues {
		if v == newForward {
			foundNew = true
		}
		if v == oldForward {
			t.Errorf("旧 forward key %q 仍存在，应已被更新", oldForward)
		}
	}
	if !foundNew {
		t.Errorf("新 forward key %q 未在渠道 keys %v 中找到", newForward, keyValues)
	}

	// DB 持久化（密文落库，解密后应与原文一致）
	var stored model.PlanProvider
	if err := db.GetDB().First(&stored, provider.ID).Error; err != nil {
		t.Fatalf("load stored: %v", err)
	}
	decForward, err := crypto.Decrypt(stored.ForwardAPIKey)
	if err != nil || !strings.HasPrefix(stored.ForwardAPIKey, "enc:") {
		t.Errorf("stored ForwardAPIKey = %q, want enc: prefixed ciphertext (err=%v)", stored.ForwardAPIKey, err)
	}
	if decForward != newForward {
		t.Errorf("decrypted ForwardAPIKey = %q, want %q", decForward, newForward)
	}
}
