package channel

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/store"
	"github.com/lingyuins/octopus/internal/utils/cache"
	"github.com/lingyuins/octopus/internal/utils/xstrings"
	"gorm.io/gorm/clause"
)

var chCache = cache.New[int, model.Channel](16)
var keyCache = cache.New[int, model.ChannelKey](16)
var keyCacheNeedUpdate = make(map[int]struct{})
var keyCacheNeedUpdateLock sync.Mutex
var runtimeUpdateLock sync.Mutex

// GetCache returns the internal channel cache (for backward compatibility).
func GetCache() cache.Cache[int, model.Channel] { return chCache }

// GetKeyCache returns the internal channel key cache (for backward compatibility).
func GetKeyCache() cache.Cache[int, model.ChannelKey] { return keyCache }

// GetKeyCacheNeedUpdate returns the key cache dirty set (for backward compatibility).
func GetKeyCacheNeedUpdate() (map[int]struct{}, *sync.Mutex) {
	return keyCacheNeedUpdate, &keyCacheNeedUpdateLock
}

// GetRuntimeUpdateLock returns the runtime update mutex (for backward compatibility).
func GetRuntimeUpdateLock() *sync.Mutex { return &runtimeUpdateLock }

func List(ctx context.Context) ([]model.Channel, error) {
	channels := make([]model.Channel, 0, chCache.Len())
	for _, ch := range chCache.GetAll() {
		channels = append(channels, ch)
	}
	return channels, nil
}

func Create(ch *model.Channel, ctx context.Context) error {
	if ch != nil {
		if err := ch.RequestRewrite.Validate(ch.Type); err != nil {
			return err
		}
		if err := normalizeChannelProxyFields(ch); err != nil {
			return err
		}
		if ch.GroupID == 0 {
			defaultGroupID, err := GroupDefaultID(ctx)
			if err != nil {
				return err
			}
			ch.GroupID = defaultGroupID
		} else if _, err := GroupGet(ch.GroupID, ctx); err != nil {
			return err
		}
	}
	if err := db.GetDB().WithContext(ctx).Create(ch).Error; err != nil {
		return err
	}
	runtimeUpdateLock.Lock()
	defer runtimeUpdateLock.Unlock()
	chCache.Set(ch.ID, *ch)
	for _, k := range ch.Keys {
		if k.ID != 0 {
			keyCache.Set(k.ID, k)
		}
	}
	return nil
}

// normalizeChannelProxyFields 把 ProxyMode / ProxyConfigID / 旧 Proxy+ChannelProxy
// 统一成可持久化的一致状态：
// - ProxyMode 为空时从旧字段推导
// - pool 模式必须带 ProxyConfigID 或 ChannelProxy
// - 非 pool 模式清空 ProxyConfigID
// - Proxy 布尔值与 ProxyMode 同步
func normalizeChannelProxyFields(ch *model.Channel) error {
	if ch == nil {
		return nil
	}

	customProxy := ""
	if ch.ChannelProxy != nil {
		customProxy = strings.TrimSpace(*ch.ChannelProxy)
		if customProxy == "" {
			ch.ChannelProxy = nil
		} else {
			ch.ChannelProxy = &customProxy
		}
	}

	mode := ch.ProxyMode
	if strings.TrimSpace(string(mode)) == "" {
		mode, ch.ProxyConfigID = deriveProxyModeFromLegacy(ch.Proxy, ch.ChannelProxy)
	}
	if err := mode.Validate(false); err != nil {
		return err
	}

	switch mode {
	case model.ProxyUsageModeDirect:
		ch.ProxyMode = model.ProxyUsageModeDirect
		ch.ProxyConfigID = nil
		ch.Proxy = false
		// direct 模式下运行时不会使用 ChannelProxy，但仍允许 UI 暂存自定义地址。
	case model.ProxyUsageModeSystem:
		ch.ProxyMode = model.ProxyUsageModeSystem
		ch.ProxyConfigID = nil
		ch.Proxy = true
	case model.ProxyUsageModePool:
		ch.ProxyMode = model.ProxyUsageModePool
		ch.Proxy = true
		if ch.ProxyConfigID != nil && *ch.ProxyConfigID <= 0 {
			ch.ProxyConfigID = nil
		}
		// 投影渠道/代理池：必须有 config id；旧自定义 URL 可无 config id
		if (ch.ProxyConfigID == nil || *ch.ProxyConfigID <= 0) && customProxy == "" {
			return fmt.Errorf("proxy config id is required when proxy mode is pool")
		}
	default:
		return fmt.Errorf("unsupported proxy mode: %s", mode)
	}
	return nil
}

func deriveProxyModeFromLegacy(proxy bool, channelProxy *string) (model.ProxyUsageMode, *int) {
	if !proxy {
		return model.ProxyUsageModeDirect, nil
	}
	if channelProxy != nil && strings.TrimSpace(*channelProxy) != "" {
		// 旧自定义代理 URL：没有 proxy_config_id，运行时直接使用 ChannelProxy
		return model.ProxyUsageModePool, nil
	}
	return model.ProxyUsageModeSystem, nil
}

func KeyUpdate(key model.ChannelKey) error {
	if key.ID == 0 || key.ChannelID == 0 {
		return fmt.Errorf("invalid channel key")
	}
	runtimeUpdateLock.Lock()
	defer runtimeUpdateLock.Unlock()

	ch, ok := chCache.Get(key.ChannelID)
	if !ok {
		return fmt.Errorf("channel not found")
	}
	if len(ch.Keys) > 0 {
		keys := make([]model.ChannelKey, len(ch.Keys))
		copy(keys, ch.Keys)
		for i := range keys {
			if keys[i].ID == key.ID {
				keys[i] = key
				break
			}
		}
		ch.Keys = keys
	}
	chCache.Set(key.ChannelID, ch)
	keyCache.Set(key.ID, key)
	keyCacheNeedUpdateLock.Lock()
	keyCacheNeedUpdate[key.ID] = struct{}{}
	keyCacheNeedUpdateLock.Unlock()
	return nil
}

func BaseUrlUpdate(channelID int, baseUrl []model.BaseUrl) error {
	runtimeUpdateLock.Lock()
	defer runtimeUpdateLock.Unlock()

	ch, ok := chCache.Get(channelID)
	if !ok {
		return fmt.Errorf("channel not found")
	}
	if baseUrl == nil {
		ch.BaseUrls = nil
	} else {
		cp := make([]model.BaseUrl, len(baseUrl))
		copy(cp, baseUrl)
		ch.BaseUrls = cp
	}
	chCache.Set(channelID, ch)

	// Redis 后端：同步探测延迟到 ChannelDelayStore，重启后保留（issue #123）。
	// 内存模式下 ChannelDelayStore 为 no-op，行为与旧版一致（重启丢失探测结果）。
	if store.Enabled() && len(ch.BaseUrls) > 0 {
		ds := store.GetChannelDelay()
		for _, bu := range ch.BaseUrls {
			_ = ds.SetDelay(context.Background(), channelID, bu.URL, bu.Delay)
		}
	}
	return nil
}

func KeySaveDB(ctx context.Context) error {
	keyCacheNeedUpdateLock.Lock()
	keyIDs := make([]int, 0, len(keyCacheNeedUpdate))
	for id := range keyCacheNeedUpdate {
		keyIDs = append(keyIDs, id)
	}
	for id := range keyCacheNeedUpdate {
		delete(keyCacheNeedUpdate, id)
	}
	keyCacheNeedUpdateLock.Unlock()

	if len(keyIDs) == 0 {
		return nil
	}

	keys := make([]model.ChannelKey, 0, len(keyIDs))
	for _, id := range keyIDs {
		k, ok := keyCache.Get(id)
		if !ok {
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return nil
	}

	if err := db.GetDB().WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"channel_id",
			"enabled",
			"channel_key",
			"status_code",
			"last_use_time_stamp",
			"total_cost",
			"priority",
			"remark",
			"managed",
		}),
	}).Create(&keys).Error; err != nil {
		keyCacheNeedUpdateLock.Lock()
		for _, id := range keyIDs {
			keyCacheNeedUpdate[id] = struct{}{}
		}
		keyCacheNeedUpdateLock.Unlock()
		return err
	}
	return nil
}

func Update(req *model.ChannelUpdateRequest, ctx context.Context) (*model.Channel, error) {
	current, ok := chCache.Get(req.ID)
	if !ok {
		return nil, fmt.Errorf("channel not found")
	}

	effectiveType := current.Type
	if req.Type != nil {
		effectiveType = *req.Type
	}

	effectiveRewrite := current.RequestRewrite
	if req.RequestRewrite != nil {
		effectiveRewrite = req.RequestRewrite
	}
	if err := effectiveRewrite.Validate(effectiveType); err != nil {
		return nil, err
	}

	groupID := 0
	if req.GroupID != nil {
		groupID = *req.GroupID
		if groupID == 0 {
			defaultGroupID, err := GroupDefaultID(ctx)
			if err != nil {
				return nil, err
			}
			groupID = defaultGroupID
		} else if _, err := GroupGet(groupID, ctx); err != nil {
			return nil, err
		}
	}

	tx := db.GetDB().WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var selectFields []string
	updates := model.Channel{ID: req.ID}

	if req.Name != nil {
		selectFields = append(selectFields, "name")
		updates.Name = *req.Name
	}
	if req.GroupID != nil {
		selectFields = append(selectFields, "group_id")
		updates.GroupID = groupID
	}
	if req.Type != nil {
		selectFields = append(selectFields, "type")
		updates.Type = *req.Type
	}
	if req.Enabled != nil {
		selectFields = append(selectFields, "enabled")
		updates.Enabled = *req.Enabled
	}
	if req.BaseUrls != nil {
		selectFields = append(selectFields, "base_urls")
		updates.BaseUrls = *req.BaseUrls
	}
	if req.Model != nil {
		selectFields = append(selectFields, "model")
		updates.Model = *req.Model
	}
	if req.CustomModel != nil {
		selectFields = append(selectFields, "custom_model")
		updates.CustomModel = *req.CustomModel
	}
	if req.ProxyMode != nil || req.ProxyConfigID != nil || req.Proxy != nil || req.ChannelProxy != nil {
		mode := current.ProxyMode
		configID := current.ProxyConfigID
		legacyProxy := current.Proxy
		legacyChannelProxy := current.ChannelProxy

		if req.ProxyMode != nil {
			mode = *req.ProxyMode
		}
		if req.ProxyConfigID != nil {
			configID = req.ProxyConfigID
		}
		if req.Proxy != nil {
			legacyProxy = *req.Proxy
		}
		if req.ChannelProxy != nil {
			legacyChannelProxy = req.ChannelProxy
		}

		// 仅传旧字段时，从 legacy 推导新模式。
		// issue #195：旧字段（proxy 开关 / channel_proxy 文本）表达不了代理池绑定，
		// 已绑定代理池的渠道若被 legacy-only 请求推导覆盖，proxy_config_id 会被静默清成
		// NULL 落库。此处保留原有 pool 绑定，解绑必须显式传 proxy_mode。
		if req.ProxyMode == nil && (req.Proxy != nil || req.ChannelProxy != nil) {
			if current.ProxyMode == model.ProxyUsageModePool && current.ProxyConfigID != nil {
				mode, configID = current.ProxyMode, current.ProxyConfigID
			} else {
				mode, configID = deriveProxyModeFromLegacy(legacyProxy, legacyChannelProxy)
			}
		}

		tmp := model.Channel{
			ProxyMode:     mode,
			ProxyConfigID: configID,
			Proxy:         legacyProxy,
			ChannelProxy:  legacyChannelProxy,
		}
		if err := normalizeChannelProxyFields(&tmp); err != nil {
			tx.Rollback()
			return nil, err
		}

		selectFields = append(selectFields, "proxy_mode", "proxy_config_id", "proxy", "channel_proxy")
		updates.ProxyMode = tmp.ProxyMode
		updates.ProxyConfigID = tmp.ProxyConfigID
		updates.Proxy = tmp.Proxy
		updates.ChannelProxy = tmp.ChannelProxy
	}
	if req.AutoSync != nil {
		selectFields = append(selectFields, "auto_sync")
		updates.AutoSync = *req.AutoSync
	}
	if req.SkipModelTest != nil {
		selectFields = append(selectFields, "skip_model_test")
		updates.SkipModelTest = *req.SkipModelTest
	}
	if req.Disposable != nil {
		selectFields = append(selectFields, "disposable")
		updates.Disposable = *req.Disposable
	}
	if req.ExpireAt != nil {
		selectFields = append(selectFields, "expire_at")
		updates.ExpireAt = req.ExpireAt
	}
	if req.NotifChannelID != nil {
		selectFields = append(selectFields, "notif_channel_id")
		updates.NotifChannelID = req.NotifChannelID
	}
	if req.KeySelectionStrategy != nil {
		selectFields = append(selectFields, "key_selection_strategy")
		updates.KeySelectionStrategy = *req.KeySelectionStrategy
	}
	if req.AutoGroup != nil {
		selectFields = append(selectFields, "auto_group")
		updates.AutoGroup = *req.AutoGroup
	}
	if req.CustomHeader != nil {
		selectFields = append(selectFields, "custom_header")
		updates.CustomHeader = *req.CustomHeader
	}
	if req.PoolID != nil {
		selectFields = append(selectFields, "pool_id")
		updates.PoolID = *req.PoolID
	}
	if req.ParamOverride != nil {
		selectFields = append(selectFields, "param_override")
		updates.ParamOverride = req.ParamOverride
	}
	if req.RequestRewrite != nil {
		selectFields = append(selectFields, "request_rewrite")
		updates.RequestRewrite = req.RequestRewrite
	}
	if req.MatchRegex != nil {
		selectFields = append(selectFields, "match_regex")
		updates.MatchRegex = req.MatchRegex
	}

	if len(selectFields) > 0 {
		if err := tx.Model(&model.Channel{}).Where("id = ?", req.ID).Select(selectFields).Updates(&updates).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update channel: %w", err)
		}
	}

	if len(req.KeysToDelete) > 0 {
		if err := tx.Where("id IN ? AND channel_id = ?", req.KeysToDelete, req.ID).Delete(&model.ChannelKey{}).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to delete channel keys: %w", err)
		}
	}

	if len(req.KeysToUpdate) > 0 {
		for _, ku := range req.KeysToUpdate {
			updates := map[string]interface{}{}
			if ku.Enabled != nil {
				updates["enabled"] = *ku.Enabled
			}
			if ku.ChannelKey != nil {
				updates["channel_key"] = *ku.ChannelKey
			}
			if ku.Priority != nil {
				updates["priority"] = *ku.Priority
			}
			if ku.Remark != nil {
				updates["remark"] = *ku.Remark
			}
			if ku.Managed != nil {
				updates["managed"] = *ku.Managed
			}
			if len(updates) == 0 {
				continue
			}
			if err := tx.Model(&model.ChannelKey{}).
				Where("id = ? AND channel_id = ?", ku.ID, req.ID).
				Updates(updates).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to update channel key %d: %w", ku.ID, err)
			}
		}
	}

	if len(req.KeysToAdd) > 0 {
		newKeys := make([]model.ChannelKey, 0, len(req.KeysToAdd))
		for _, ka := range req.KeysToAdd {
			newKeys = append(newKeys, model.ChannelKey{
				ChannelID:  req.ID,
				Enabled:    ka.Enabled,
				ChannelKey: ka.ChannelKey,
				Priority:   ka.Priority,
				Remark:     ka.Remark,
				Managed:    ka.Managed,
			})
		}
		if err := tx.Create(&newKeys).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create channel keys: %w", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	if err := RefreshCacheByID(req.ID, ctx); err != nil {
		return nil, err
	}

	ch, _ := chCache.Get(req.ID)
	return &ch, nil
}

// BatchUpdateGroup 批量将多个渠道移动到同一个目标分组（覆盖原有分组）。
// 逐个更新，单个失败不影响其余渠道，返回成功与失败明细。
// groupID 为 0 时解析为默认分组，非 0 时校验分组存在。
func BatchUpdateGroup(ids []int, groupID int, ctx context.Context) (*model.ChannelBatchGroupResult, error) {
	result := &model.ChannelBatchGroupResult{
		SuccessIDs:  make([]int, 0, len(ids)),
		FailedItems: make([]model.ChannelBatchGroupFailure, 0),
	}
	for _, id := range ids {
		gid := groupID
		req := &model.ChannelUpdateRequest{ID: id, GroupID: &gid}
		if _, err := Update(req, ctx); err != nil {
			result.FailedItems = append(result.FailedItems, model.ChannelBatchGroupFailure{ID: id, Message: err.Error()})
			continue
		}
		result.SuccessIDs = append(result.SuccessIDs, id)
	}
	return result, nil
}

func Enabled(id int, enabled bool, ctx context.Context) error {
	runtimeUpdateLock.Lock()
	defer runtimeUpdateLock.Unlock()

	oldCh, ok := chCache.Get(id)
	if !ok {
		return fmt.Errorf("channel not found")
	}
	if err := db.GetDB().WithContext(ctx).Model(&model.Channel{}).Where("id = ?", id).Update("enabled", enabled).Error; err != nil {
		return err
	}
	oldCh.Enabled = enabled
	chCache.Set(id, oldCh)
	return nil
}

func LLMList(ctx context.Context) ([]model.LLMChannel, error) {
	models := []model.LLMChannel{}
	seen := make(map[string]struct{})

	// 投影渠道：channel_id → (site_account_id, base_group_key)
	channelIDs := make([]int, 0, chCache.Len())
	for _, ch := range chCache.GetAll() {
		channelIDs = append(channelIDs, ch.ID)
	}
	bindingByChannel := loadProjectedChannelBindings(ctx, channelIDs)
	accountBalanceByID, accountTodayIncomeByID, modelsByAccountGroup := loadProjectedChannelMetaIndex(ctx, bindingByChannel)

	for _, ch := range chCache.GetAll() {
		modelNames := xstrings.SplitTrimCompact(",", ch.Model, ch.CustomModel)
		binding, isProjected := bindingByChannel[ch.ID]
		var accountBalance *float64
		var accountTodayIncome *float64
		if isProjected {
			if bal, ok := accountBalanceByID[binding.SiteAccountID]; ok {
				copied := bal
				accountBalance = &copied
			}
			if income, ok := accountTodayIncomeByID[binding.SiteAccountID]; ok {
				copied := income
				accountTodayIncome = &copied
			}
		}
		for _, modelName := range modelNames {
			if modelName == "" {
				continue
			}
			item := model.LLMChannel{
				Name:               modelName,
				Enabled:            ch.Enabled,
				ChannelID:          ch.ID,
				ChannelName:        ch.Name,
				ChannelBalance:     accountBalance,
				ChannelTodayIncome: accountTodayIncome,
			}
			if isProjected {
				lookupKey := projectedModelLookupKey(binding.SiteAccountID, binding.BaseGroupKey, modelName)
				if meta, ok := modelsByAccountGroup[lookupKey]; ok {
					if meta.Price != nil {
						copied := *meta.Price
						item.UpstreamPrice = &copied
					}
					if meta.Metrics != nil {
						copied := *meta.Metrics
						item.UpstreamMetrics = &copied
					}
				}
			}
			key := fmt.Sprintf("%d|%s", item.ChannelID, item.Name)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			models = append(models, item)
		}
	}
	return models, nil
}

type projectedChannelBindingInfo struct {
	SiteAccountID int
	BaseGroupKey  string
}

func loadProjectedChannelBindings(ctx context.Context, channelIDs []int) map[int]projectedChannelBindingInfo {
	result := make(map[int]projectedChannelBindingInfo)
	if len(channelIDs) == 0 {
		return result
	}
	var bindings []model.SiteChannelBinding
	if err := db.GetDB().WithContext(ctx).Where("channel_id IN ?", channelIDs).Find(&bindings).Error; err != nil {
		return result
	}
	for _, binding := range bindings {
		baseGroupKey, _ := model.ParseSiteChannelBindingKey(binding.GroupKey)
		result[binding.ChannelID] = projectedChannelBindingInfo{
			SiteAccountID: binding.SiteAccountID,
			BaseGroupKey:  model.NormalizeSiteGroupKey(baseGroupKey),
		}
	}
	return result
}

type projectedChannelModelMeta struct {
	Price   *model.ChannelUpstreamPrice
	Metrics *model.ChannelUpstreamMetrics
}

func loadProjectedChannelMetaIndex(
	ctx context.Context,
	bindingByChannel map[int]projectedChannelBindingInfo,
) (map[int]float64, map[int]float64, map[string]projectedChannelModelMeta) {
	balances := make(map[int]float64)
	todayIncomes := make(map[int]float64)
	metaByKey := make(map[string]projectedChannelModelMeta)
	if len(bindingByChannel) == 0 {
		return balances, todayIncomes, metaByKey
	}

	accountIDs := make([]int, 0, len(bindingByChannel))
	seenAccounts := make(map[int]struct{}, len(bindingByChannel))
	for _, binding := range bindingByChannel {
		if binding.SiteAccountID <= 0 {
			continue
		}
		if _, ok := seenAccounts[binding.SiteAccountID]; ok {
			continue
		}
		seenAccounts[binding.SiteAccountID] = struct{}{}
		accountIDs = append(accountIDs, binding.SiteAccountID)
	}
	if len(accountIDs) == 0 {
		return balances, todayIncomes, metaByKey
	}

	type accountBalanceRow struct {
		ID          int
		Balance     float64
		TodayIncome float64
	}
	var accountRows []accountBalanceRow
	if err := db.GetDB().WithContext(ctx).
		Model(&model.SiteAccount{}).
		Select("id, balance, today_income").
		Where("id IN ?", accountIDs).
		Find(&accountRows).Error; err == nil {
		for _, row := range accountRows {
			balances[row.ID] = row.Balance
			todayIncomes[row.ID] = row.TodayIncome
		}
	}

	var siteModels []model.SiteModel
	if err := db.GetDB().WithContext(ctx).
		Where("site_account_id IN ? AND disabled = ?", accountIDs, false).
		Find(&siteModels).Error; err != nil {
		return balances, todayIncomes, metaByKey
	}
	for _, siteModel := range siteModels {
		key := projectedModelLookupKey(siteModel.SiteAccountID, siteModel.GroupKey, siteModel.ModelName)
		meta := metaByKey[key]
		if siteModelHasUpstreamPriceLocal(siteModel) {
			price := model.ChannelUpstreamPrice{
				BillingMode: strings.TrimSpace(siteModel.PriceBillingMode),
				Input:       siteModel.PriceInput,
				Output:      siteModel.PriceOutput,
				CacheRead:   siteModel.PriceCacheRead,
				CacheWrite:  siteModel.PriceCacheWrite,
			}
			meta.Price = &price
		}
		if siteModelHasUpstreamPerfLocal(siteModel) {
			metrics := model.ChannelUpstreamMetrics{
				LatencyMs:   siteModel.PerfLatencyMs,
				AvgTps:      siteModel.PerfAvgTps,
				SuccessRate: siteModel.PerfSuccessRate,
			}
			meta.Metrics = &metrics
		}
		if meta.Price != nil || meta.Metrics != nil {
			metaByKey[key] = meta
		}
	}
	return balances, todayIncomes, metaByKey
}

func projectedModelLookupKey(accountID int, groupKey, modelName string) string {
	return fmt.Sprintf("%d|%s|%s",
		accountID,
		model.NormalizeSiteGroupKey(groupKey),
		strings.ToLower(strings.TrimSpace(modelName)),
	)
}

func siteModelHasUpstreamPriceLocal(item model.SiteModel) bool {
	if strings.TrimSpace(item.PriceBillingMode) == "" {
		return false
	}
	return item.PriceInput > 0 || item.PriceOutput > 0 || item.PriceCacheRead > 0 || item.PriceCacheWrite > 0
}

func siteModelHasUpstreamPerfLocal(item model.SiteModel) bool {
	return item.PerfLatencyMs > 0 || item.PerfAvgTps > 0 || item.PerfSuccessRate > 0
}

func Get(id int, ctx context.Context) (*model.Channel, error) {
	ch, ok := chCache.Get(id)
	if !ok {
		return nil, fmt.Errorf("channel not found")
	}
	return &ch, nil
}

func RefreshCache(ctx context.Context) error {
	channels := []model.Channel{}
	if err := db.GetDB().WithContext(ctx).
		Preload("Keys").
		Preload("Stats").
		Find(&channels).Error; err != nil {
		return err
	}
	runtimeUpdateLock.Lock()
	defer runtimeUpdateLock.Unlock()

	chCache.Clear()
	keyCache.Clear()
	keyCacheNeedUpdateLock.Lock()
	for id := range keyCacheNeedUpdate {
		delete(keyCacheNeedUpdate, id)
	}
	keyCacheNeedUpdateLock.Unlock()

	for i := range channels {
		ch := channels[i]
		// Redis 后端：恢复频道 base URL 延迟探测结果（重启后保留）。
		// 内存模式下 GetDelays 返回空（memoryChannelDelay 是 no-op），不影响行为。
		if store.Enabled() {
			if delays, err := store.GetChannelDelay().GetDelays(ctx, ch.ID); err == nil && len(delays) > 0 {
				for j := range ch.BaseUrls {
					if d, ok := delays[ch.BaseUrls[j].URL]; ok {
						ch.BaseUrls[j].Delay = d
					}
				}
			}
		}
		chCache.Set(ch.ID, ch)
		for _, k := range ch.Keys {
			if k.ID != 0 {
				keyCache.Set(k.ID, k)
			}
		}
	}
	return nil
}

func RefreshCacheByID(id int, ctx context.Context) error {
	runtimeUpdateLock.Lock()
	defer runtimeUpdateLock.Unlock()

	if old, ok := chCache.Get(id); ok {
		for _, k := range old.Keys {
			if k.ID != 0 {
				keyCache.Del(k.ID)
			}
		}
	}
	var ch model.Channel
	if err := db.GetDB().WithContext(ctx).
		Preload("Keys").
		Preload("Stats").
		First(&ch, id).Error; err != nil {
		return err
	}
	chCache.Set(ch.ID, ch)
	for _, k := range ch.Keys {
		if k.ID != 0 {
			keyCache.Set(k.ID, k)
		}
	}
	return nil
}

// Delete performs channel DB deletion transaction (without stats/group cache cleanup).
func Delete(id int, ctx context.Context) error {
	ch, ok := chCache.Get(id)
	if !ok {
		return fmt.Errorf("channel not found")
	}

	tx := db.GetDB().WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&model.GroupItem{}).
		Where("channel_id = ?", id).
		Delete(&model.GroupItem{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete group items: %w", err)
	}

	if err := tx.Where("channel_id = ?", id).Delete(&model.ChannelKey{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete channel keys: %w", err)
	}

	if err := tx.Where("channel_id = ?", id).Delete(&model.StatsChannel{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete channel stats: %w", err)
	}

	if err := tx.Delete(&model.Channel{}, id).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete channel: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	runtimeUpdateLock.Lock()
	chCache.Del(id)
	for _, k := range ch.Keys {
		if k.ID != 0 {
			keyCache.Del(k.ID)
		}
	}
	runtimeUpdateLock.Unlock()

	return nil
}

// GroupDefaultID is a placeholder; replaced by op via callback.
var GroupDefaultID = func(ctx context.Context) (int, error) {
	return 0, fmt.Errorf("channel: GroupDefaultID not registered")
}

// GroupGet is a placeholder; replaced by op via callback.
var GroupGet = func(id int, ctx context.Context) (*model.ChannelGroup, error) {
	return nil, fmt.Errorf("channel: GroupGet not registered")
}
