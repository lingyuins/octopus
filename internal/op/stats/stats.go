package stats

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/store"
	"github.com/lingyuins/octopus/internal/utils/cache"
	"github.com/lingyuins/octopus/internal/utils/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// --- Redis 后端 scope 常量（issue #123） ---
// 统计指标在 StatsStore 中的 scope 命名空间。启用 Redis 时，每次 *Update
// 实时增量写入 Redis（HINCRBY/HINCRBYFLOAT + Lua max），SaveDB 时 SnapshotAll
// 读全量落盘 DB 并清空 Redis scope；RefreshCache 启动时从 DB + Redis 叠加恢复。
// 未启用 Redis 时这些常量不参与逻辑，行为与旧版完全一致。
const (
	statsScopeTotal             = "total"
	statsScopeDaily             = "daily"
	statsScopeHourly            = "hourly"
	statsScopeChannel           = "channel"
	statsScopeModel             = "model"
	statsScopeAPIKey            = "apikey"
	statsScopeDailyChannel      = "daily_channel"
	statsScopeDailyModel        = "daily_model"
	statsScopeDailyAPIKey       = "daily_apikey"
	statsScopeDailyChannelModel = "daily_channel_model"
)

// statsIDTotal 是 total scope 的固定 id（StatsTotal 单行表，主键 ID=1）。
const statsIDTotal = "1"

// incrStatsRedis 将 delta 增量写入 Redis（启用时）。降级（err/未启用）静默忽略，
// 内存镜像仍由调用方维护，保证统计不丢。
func incrStatsRedis(scope, id string, delta model.StatsMetrics) {
	if !store.Enabled() || id == "" {
		return
	}
	_ = store.GetStats().IncrMetrics(context.Background(), scope, id, delta)
}

var dailyCache model.StatsDaily
var dailyCacheLock sync.RWMutex
var pendingDailyOverride atomic.Pointer[model.StatsDaily]

var totalCache model.StatsTotal
var totalCacheLock sync.RWMutex

// timeNow allows tests to override time.Now.
var timeNow = time.Now

// StatsLocation 返回配置的统计时区。优先 stats_timezone（IANA 名，如 Asia/Shanghai），
// 回退到已弃用的 stats_timezone_offset（整型小时偏移），再回退 time.Local（容器运行时区）。
// IANA 名加载失败（系统无 tzdata / 非法名）也回退，绝不返回 nil。
// 供 stats/ops/report 共用，消除各处重复的偏移计算逻辑。未配置任何时区时使用
// time.Local，保持与历史行为一致（容器 TZ 生效，如 Docker ENV TZ=Asia/Shanghai）。
func StatsLocation() *time.Location {
	if tzName, err := setting.GetString(model.SettingKeyStatsTimezone); err == nil && tzName != "" {
		if loc, err := time.LoadLocation(tzName); err == nil {
			return loc
		}
		// 加载失败：回退到 offset，再回退 time.Local，避免静默返回错误时区。
	}
	if offset, err := setting.GetInt(model.SettingKeyStatsTimezoneOffset); err == nil && offset != 0 {
		return time.FixedZone(fmt.Sprintf("UTC%+d", offset), offset*3600)
	}
	return time.Local
}

// now returns the current time in the configured stats timezone.
// When the container runs in UTC but users are in a different timezone, this ensures
// hourly/daily statistics align with the user's local date/hour boundaries.
// .Location() 反映真实时区（而非 UTC），day-boundary 计算用 time.Date(...,loc) 仍正确。
func now() time.Time {
	return timeNow().In(StatsLocation())
}

// Now returns the current time in the configured stats timezone.
func Now() time.Time { return now() }

// today returns the current date string (YYYYMMDD) in the configured timezone.
func today() string {
	return now().Format("20060102")
}

var hourlyCache [24]model.StatsHourly
var hourlyCacheLock sync.RWMutex

var channelCache = cache.New[int, model.StatsChannel](16)
var channelCacheNeedUpdate = make(map[int]struct{})
var channelCacheNeedUpdateLock sync.Mutex
var channelMutationLock sync.Mutex

var modelCache = cache.New[int64, model.StatsModel](16)
var modelCacheNeedUpdate = make(map[int64]struct{})
var modelCacheNeedUpdateLock sync.Mutex
var modelMutationLock sync.Mutex

// modelLastActivity 追踪每个 model 统计条目的最后活跃时间（unixNano），用于
// 周期回收空闲条目。与 balancer.ChannelStats.lastActivity() 等价，但因
// model.StatsModel 是 DB 映射结构体（改动面大），这里用独立的 sync.Map 追踪，
// 不污染 DB schema。key: modelID(int64) -> *int64(unixNano)。
//
// 背景：modelCache 的 key = FNV(channelID:clientModelName)，model 名由客户端
// 请求携带、基数不受控；此前仅测试代码 Clear()，无空闲回收，刷量/随机 model 名
// 会导致 map 终生驻留（见 issue #124）。
var modelLastActivity sync.Map // int64(modelID) -> *int64(unixNano)

var apiKeyCache = cache.New[int, model.StatsAPIKey](16)
var apiKeyCacheNeedUpdate = make(map[int]struct{})
var apiKeyCacheNeedUpdateLock sync.Mutex
var apiKeyMutationLock sync.Mutex

// dailyChannelModelRetryQueue 存储写入失败的渠道×模型统计条目，供 SaveDB 重试。
// 使用 ring buffer 限制内存（最多保留 1000 条），FIFO 淘汰最旧条目。
type dailyChannelModelRetryQueue struct {
	mu      sync.Mutex
	entries []model.StatsDailyChannelModel
	maxSize int
}

var retryQueue = &dailyChannelModelRetryQueue{maxSize: 1000}

func (q *dailyChannelModelRetryQueue) push(entry model.StatsDailyChannelModel) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.entries) >= q.maxSize {
		// FIFO 淘汰：移除最旧的一半，为新条目腾出空间
		copy(q.entries, q.entries[q.maxSize/2:])
		q.entries = q.entries[:q.maxSize/2]
	}
	q.entries = append(q.entries, entry)
}

func (q *dailyChannelModelRetryQueue) drain() []model.StatsDailyChannelModel {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.entries) == 0 {
		return nil
	}
	snapshot := make([]model.StatsDailyChannelModel, len(q.entries))
	copy(snapshot, q.entries)
	q.entries = q.entries[:0]
	return snapshot
}

func (q *dailyChannelModelRetryQueue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.entries)
}

// SaveDBTask is a convenience wrapper that creates a 2-minute context and calls SaveDB.
func SaveDBTask() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	log.Debugf("stats save db task started")
	startTime := time.Now()
	defer func() {
		log.Debugf("stats save db task finished, save time: %s", time.Since(startTime))
	}()
	if err := SaveDB(ctx); err != nil {
		log.Errorf("stats save db error: %v", err)
		return
	}
}

// SaveDB persists cached statistics to the database.
//
// Design note: stats caches are read under RLock to produce snapshots, then
// the lock is released before DB writes. In-flight updates (by concurrent
// relay goroutines) between snapshot and persist are NOT captured in this
// cycle but will be persisted in the next SaveDB call. This is an
// intentional eventually-consistent design that avoids holding locks across
// I/O operations.
func SaveDB(ctx context.Context) error {
	if pending := pendingDailyOverride.Swap(nil); pending != nil {
		if err := saveDBWithDailyOverride(ctx, *pending); err != nil {
			log.Warnf("failed to persist pending daily override during SaveDB: %v", err)
		}
	}

	totalCacheLock.RLock()
	totalSnap := totalCache
	totalCacheLock.RUnlock()
	if totalSnap.ID == 0 {
		totalSnap.ID = 1
	}

	dailyCacheLock.RLock()
	dailySnap := dailyCache
	dailyCacheLock.RUnlock()

	hourlyCacheLock.RLock()
	hourlyAll := hourlyCache
	hourlyCacheLock.RUnlock()

	channelCacheNeedUpdateLock.Lock()
	channelIDs := make([]int, 0, len(channelCacheNeedUpdate))
	for id := range channelCacheNeedUpdate {
		channelIDs = append(channelIDs, id)
	}
	channelCacheNeedUpdate = make(map[int]struct{})
	channelCacheNeedUpdateLock.Unlock()

	modelCacheNeedUpdateLock.Lock()
	modelIDs := make([]int64, 0, len(modelCacheNeedUpdate))
	for id := range modelCacheNeedUpdate {
		modelIDs = append(modelIDs, id)
	}
	modelCacheNeedUpdate = make(map[int64]struct{})
	modelCacheNeedUpdateLock.Unlock()

	apiKeyCacheNeedUpdateLock.Lock()
	apiKeyIDs := make([]int, 0, len(apiKeyCacheNeedUpdate))
	for id := range apiKeyCacheNeedUpdate {
		apiKeyIDs = append(apiKeyIDs, id)
	}
	apiKeyCacheNeedUpdate = make(map[int]struct{})
	apiKeyCacheNeedUpdateLock.Unlock()

	if err := persistSnapshots(ctx, totalSnap, dailySnap, hourlyAll, channelIDs, modelIDs, apiKeyIDs); err != nil {
		requeueDirtyIDs(channelIDs, modelIDs, apiKeyIDs)
		return err
	}
	// 重试失败的渠道×模型统计条目（issue #检查渠道×模型的聚合链路）
	if retryEntries := retryQueue.drain(); len(retryEntries) > 0 {
		if err := retryDailyChannelModelEntries(ctx, retryEntries); err != nil {
			log.Warnf("failed to retry %d daily channel-model entries: %v", len(retryEntries), err)
		} else {
			log.Debugf("successfully retried %d daily channel-model entries", len(retryEntries))
		}
	}
	// Redis 后端：落盘成功后清除已持久化的 scope，开始下一轮增量累积（issue #123）。
	// 失败仅记录日志，不影响主流程（下次 SaveDB 会重新落盘累积值，幂等）。
	clearRedisStatsScopes(ctx)
	return nil
}

// retryDailyChannelModelEntries 批量重试失败的渠道×模型统计条目。
// 按日期+渠道+模型分组聚合，避免重复 upsert 同一 (date, channel_id, model_name)。
// 失败条目不重新入队（避免死循环），但会记录日志。
func retryDailyChannelModelEntries(ctx context.Context, entries []model.StatsDailyChannelModel) error {
	if len(entries) == 0 {
		return nil
	}
	// 聚合：key = "date\x00channelID\x00modelName"
	merged := make(map[string]*model.StatsDailyChannelModel)
	for i := range entries {
		entry := &entries[i]
		key := entry.Date + "\x00" + strconv.Itoa(entry.ChannelID) + "\x00" + entry.ModelName
		if existing, ok := merged[key]; ok {
			existing.StatsMetrics.RequestSuccess += entry.StatsMetrics.RequestSuccess
			existing.StatsMetrics.RequestFailed += entry.StatsMetrics.RequestFailed
			existing.StatsMetrics.InputToken += entry.StatsMetrics.InputToken
			existing.StatsMetrics.OutputToken += entry.StatsMetrics.OutputToken
			existing.StatsMetrics.InputCost += entry.StatsMetrics.InputCost
			existing.StatsMetrics.OutputCost += entry.StatsMetrics.OutputCost
			if existing.ChannelName == "" {
				existing.ChannelName = entry.ChannelName
			}
		} else {
			entryCopy := *entry
			merged[key] = &entryCopy
		}
	}
	batch := make([]model.StatsDailyChannelModel, 0, len(merged))
	for _, entry := range merged {
		batch = append(batch, *entry)
	}
	return upsertDailyChannelModels(db.GetDB().WithContext(ctx), batch)
}

// clearRedisStatsScopes 清除所有统计 scope 的 Redis 增量，仅在 SaveDB 成功后调用。
// 用 SCAN + DEL 逐 scope 清理（stats key 形如 octopus:stats:{scope}:{id}）。
func clearRedisStatsScopes(ctx context.Context) {
	if !store.Enabled() {
		return
	}
	scopes := []string{statsScopeTotal, statsScopeDaily, statsScopeHourly, statsScopeChannel, statsScopeModel, statsScopeAPIKey}
	for _, scope := range scopes {
		// SnapshotAll 用 SCAN，这里复用其命名空间；Delete 逐 id 删除较慢，
		// 直接用 DelByPrefix 清理整个 scope 命名空间（KVStore 接口）。
		_ = store.GetKV().DelByPrefix(ctx, "stats:"+scope+":")
	}
}

func persistSnapshots(
	ctx context.Context,
	totalSnap model.StatsTotal,
	dailySnap model.StatsDaily,
	hourlyAll [24]model.StatsHourly,
	channelIDs []int,
	modelIDs []int64,
	apiKeyIDs []int,
) error {
	todayDate := today()
	hourlyStats := make([]model.StatsHourly, 0, 24)
	for hour := 0; hour < 24; hour++ {
		if hourlyAll[hour].Date == todayDate {
			hourlyStats = append(hourlyStats, hourlyAll[hour])
		}
	}

	channelStats := make([]model.StatsChannel, 0, len(channelIDs))
	for _, id := range channelIDs {
		ch, ok := channelCache.Get(id)
		if !ok {
			continue
		}
		channelStats = append(channelStats, ch)
	}

	modelStats := make([]model.StatsModel, 0, len(modelIDs))
	for _, id := range modelIDs {
		m, ok := modelCache.Get(id)
		if !ok {
			continue
		}
		modelStats = append(modelStats, m)
	}

	apiKeyStats := make([]model.StatsAPIKey, 0, len(apiKeyIDs))
	for _, id := range apiKeyIDs {
		ak, ok := apiKeyCache.Get(id)
		if !ok {
			continue
		}
		apiKeyStats = append(apiKeyStats, ak)
	}

	return db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if result := tx.Save(&totalSnap); result.Error != nil {
			return result.Error
		}
		if result := tx.Save(&dailySnap); result.Error != nil {
			return result.Error
		}
		if len(hourlyStats) > 0 {
			if result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "hour"}, {Name: "date"}},
				UpdateAll: true,
			}).Create(&hourlyStats); result.Error != nil {
				return result.Error
			}
		}
		if err := upsertChannels(tx, channelStats); err != nil {
			return err
		}
		if err := upsertModels(tx, modelStats); err != nil {
			return err
		}
		if err := upsertAPIKeys(tx, apiKeyStats); err != nil {
			return err
		}
		return nil
	})
}

func upsertChannels(dbConn *gorm.DB, stats []model.StatsChannel) error {
	if len(stats) == 0 {
		return nil
	}
	return dbConn.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "channel_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"input_token",
			"output_token",
			"input_cost",
			"output_cost",
			"wait_time",
			"request_success",
			"request_failed",
		}),
	}).Create(&stats).Error
}

func upsertModels(dbConn *gorm.DB, stats []model.StatsModel) error {
	if len(stats) == 0 {
		return nil
	}
	return dbConn.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name",
			"channel_id",
			"input_token",
			"output_token",
			"input_cost",
			"output_cost",
			"wait_time",
			"request_success",
			"request_failed",
		}),
	}).Create(&stats).Error
}

func upsertAPIKeys(dbConn *gorm.DB, stats []model.StatsAPIKey) error {
	if len(stats) == 0 {
		return nil
	}
	return dbConn.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "api_key_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"input_token",
			"output_token",
			"input_cost",
			"output_cost",
			"wait_time",
			"request_success",
			"request_failed",
		}),
	}).Create(&stats).Error
}

func dailyDimensionExcludedExpr(dbConn *gorm.DB, column string) string {
	if dbConn != nil && dbConn.Dialector != nil && dbConn.Dialector.Name() == "mysql" {
		return "VALUES(`" + column + "`)"
	}
	return "excluded." + column
}

// dailyDimensionTargetExpr 返回 ON CONFLICT DO UPDATE 里对目标表列的引用。
// PostgreSQL 在 DO UPDATE 中同时看到目标表与 excluded 的同名列时，裸列名
// 会被判为 ambiguous（SQLSTATE 42702），必须带表名限定。SQLite/MySQL 不受
// 影响，保持裸列名以维持原有行为。
func dailyDimensionTargetExpr(dbConn *gorm.DB, table, column string) string {
	if dbConn != nil && dbConn.Dialector != nil && dbConn.Dialector.Name() == "postgres" {
		return table + "." + column
	}
	return column
}

func dailyDimensionMaxExpr(dbConn *gorm.DB, table, column string) string {
	excluded := dailyDimensionExcludedExpr(dbConn, column)
	target := dailyDimensionTargetExpr(dbConn, table, column)
	if dbConn != nil && dbConn.Dialector != nil && dbConn.Dialector.Name() == "sqlite" {
		return "MAX(" + target + ", " + excluded + ")"
	}
	return "GREATEST(" + target + ", " + excluded + ")"
}

func dailyDimensionUpdateColumns(dbConn *gorm.DB, table string, includeName bool) clause.Set {
	add := func(column string) clause.Assignment {
		expr := dailyDimensionTargetExpr(dbConn, table, column) + " + " + dailyDimensionExcludedExpr(dbConn, column)
		return clause.Assignment{Column: clause.Column{Name: column}, Value: gorm.Expr(expr)}
	}
	max := func(column string) clause.Assignment {
		return clause.Assignment{Column: clause.Column{Name: column}, Value: gorm.Expr(dailyDimensionMaxExpr(dbConn, table, column))}
	}
	assignments := []clause.Assignment{
		add("input_token"),
		add("output_token"),
		add("input_cost"),
		add("output_cost"),
		add("wait_time"),
		add("request_success"),
		add("request_failed"),
		max("latency_p50"),
		max("latency_p95"),
		max("latency_p99"),
		max("ftut_avg"),
		max("ftut_p50"),
		max("ftut_p95"),
		max("ftut_p99"),
		// 列名须与 GORM 对字段的蛇形命名一致（HistogramLt100 → histogram_lt100，
		// 数字前不插下划线）；用 JSON tag 名（histogram_lt_100）会引用不存在的列，
		// 导致 stats_daily_* 的 ON CONFLICT upsert 全部失败（历史 bug）。
		add("histogram_lt100"),
		add("histogram100to500"),
		add("histogram500to1k"),
		add("histogram1kto5k"),
		add("histogram_gt5k"),
	}
	if includeName {
		assignments = append([]clause.Assignment{{Column: clause.Column{Name: "name"}, Value: gorm.Expr(dailyDimensionExcludedExpr(dbConn, "name"))}}, assignments...)
	}
	return assignments
}

func dailyDimensionChannelUpdateColumns(dbConn *gorm.DB, table string) clause.Set {
	assignments := dailyDimensionUpdateColumns(dbConn, table, false)
	return append([]clause.Assignment{{Column: clause.Column{Name: "channel_name"}, Value: gorm.Expr(dailyDimensionExcludedExpr(dbConn, "channel_name"))}}, assignments...)
}

func upsertDailyChannels(dbConn *gorm.DB, stats []model.StatsDailyChannel) error {
	if len(stats) == 0 {
		return nil
	}
	return dbConn.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}, {Name: "channel_id"}},
		DoUpdates: dailyDimensionChannelUpdateColumns(dbConn, "stats_daily_channels"),
	}).Create(&stats).Error
}

func upsertDailyModels(dbConn *gorm.DB, stats []model.StatsDailyModel) error {
	if len(stats) == 0 {
		return nil
	}
	return dbConn.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}, {Name: "model_name"}},
		DoUpdates: dailyDimensionUpdateColumns(dbConn, "stats_daily_models", false),
	}).Create(&stats).Error
}

func upsertDailyAPIKeys(dbConn *gorm.DB, stats []model.StatsDailyAPIKey) error {
	if len(stats) == 0 {
		return nil
	}
	return dbConn.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}, {Name: "api_key_id"}},
		DoUpdates: dailyDimensionUpdateColumns(dbConn, "stats_daily_api_keys", true),
	}).Create(&stats).Error
}

func upsertDailyChannelModels(dbConn *gorm.DB, stats []model.StatsDailyChannelModel) error {
	if len(stats) == 0 {
		return nil
	}
	return dbConn.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}, {Name: "channel_id"}, {Name: "model_name"}},
		DoUpdates: dailyDimensionChannelUpdateColumns(dbConn, "stats_daily_channel_models"),
	}).Create(&stats).Error
}

func saveDBWithDailyOverride(ctx context.Context, dailyOverride model.StatsDaily) error {
	totalCacheLock.RLock()
	totalSnap := totalCache
	totalCacheLock.RUnlock()
	if totalSnap.ID == 0 {
		totalSnap.ID = 1
	}

	hourlyCacheLock.RLock()
	hourlyAll := hourlyCache
	hourlyCacheLock.RUnlock()

	channelCacheNeedUpdateLock.Lock()
	channelIDs := make([]int, 0, len(channelCacheNeedUpdate))
	for id := range channelCacheNeedUpdate {
		channelIDs = append(channelIDs, id)
	}
	channelCacheNeedUpdate = make(map[int]struct{})
	channelCacheNeedUpdateLock.Unlock()

	modelCacheNeedUpdateLock.Lock()
	modelIDs := make([]int64, 0, len(modelCacheNeedUpdate))
	for id := range modelCacheNeedUpdate {
		modelIDs = append(modelIDs, id)
	}
	modelCacheNeedUpdate = make(map[int64]struct{})
	modelCacheNeedUpdateLock.Unlock()

	apiKeyCacheNeedUpdateLock.Lock()
	apiKeyIDs := make([]int, 0, len(apiKeyCacheNeedUpdate))
	for id := range apiKeyCacheNeedUpdate {
		apiKeyIDs = append(apiKeyIDs, id)
	}
	apiKeyCacheNeedUpdate = make(map[int]struct{})
	apiKeyCacheNeedUpdateLock.Unlock()

	if err := persistSnapshots(ctx, totalSnap, dailyOverride, hourlyAll, channelIDs, modelIDs, apiKeyIDs); err != nil {
		requeueDirtyIDs(channelIDs, modelIDs, apiKeyIDs)
		return err
	}
	return nil
}

func requeueDirtyIDs(channelIDs []int, modelIDs []int64, apiKeyIDs []int) {
	channelCacheNeedUpdateLock.Lock()
	for _, id := range channelIDs {
		channelCacheNeedUpdate[id] = struct{}{}
	}
	channelCacheNeedUpdateLock.Unlock()

	modelCacheNeedUpdateLock.Lock()
	for _, id := range modelIDs {
		modelCacheNeedUpdate[id] = struct{}{}
	}
	modelCacheNeedUpdateLock.Unlock()

	apiKeyCacheNeedUpdateLock.Lock()
	for _, id := range apiKeyIDs {
		apiKeyCacheNeedUpdate[id] = struct{}{}
	}
	apiKeyCacheNeedUpdateLock.Unlock()
}

// DailyUpdate adds metrics to the current day's stats, persisting the previous day if a date boundary is crossed.
func DailyUpdate(ctx context.Context, metrics model.StatsMetrics) error {
	todayDate := today()

	// Redis 增量：实时累加到今日 daily scope，崩溃不丢（issue #123）。
	if store.Enabled() {
		_ = store.GetStats().IncrMetrics(context.Background(), statsScopeDaily, todayDate, metrics)
	}

	dailyCacheLock.Lock()
	if dailyCache.Date == todayDate {
		dailyCache.StatsMetrics.Add(metrics)
		updatedDaily := dailyCache
		dailyCacheLock.Unlock()
		upsertDailyAllCache(updatedDaily)
		return nil
	}

	prevDaily := dailyCache
	dailyCache = model.StatsDaily{Date: todayDate}
	dailyCache.StatsMetrics.Add(metrics)
	updatedDaily := dailyCache
	dailyCacheLock.Unlock()

	upsertDailyAllCache(prevDaily, updatedDaily)
	pendingDailyOverride.Store(&prevDaily)
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := saveDBWithDailyOverride(bgCtx, prevDaily); err != nil {
			log.Errorf("async daily boundary persist failed: %v", err)
			return
		}
		pendingDailyOverride.CompareAndSwap(&prevDaily, nil)
	}()
	return nil
}

func DailyDimensionChannelUpdate(ctx context.Context, channelID int, channelName string, metrics model.StatsMetrics) error {
	if channelID == 0 {
		return nil
	}
	entry := model.StatsDailyChannel{
		Date:         today(),
		ChannelID:    channelID,
		ChannelName:  strings.TrimSpace(channelName),
		StatsMetrics: metrics,
	}
	return upsertDailyChannels(db.GetDB().WithContext(ctx), []model.StatsDailyChannel{entry})
}

func DailyDimensionModelUpdate(ctx context.Context, modelName string, metrics model.StatsMetrics) error {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil
	}
	entry := model.StatsDailyModel{
		Date:         today(),
		ModelName:    modelName,
		StatsMetrics: metrics,
	}
	return upsertDailyModels(db.GetDB().WithContext(ctx), []model.StatsDailyModel{entry})
}

func DailyDimensionAPIKeyUpdate(ctx context.Context, apiKeyID int, name string, metrics model.StatsMetrics) error {
	if apiKeyID == 0 && strings.TrimSpace(name) == "" {
		return nil
	}
	entry := model.StatsDailyAPIKey{
		Date:         today(),
		APIKeyID:     apiKeyID,
		Name:         strings.TrimSpace(name),
		StatsMetrics: metrics,
	}
	return upsertDailyAPIKeys(db.GetDB().WithContext(ctx), []model.StatsDailyAPIKey{entry})
}

func DailyDimensionChannelModelUpdate(ctx context.Context, channelID int, channelName, modelName string, metrics model.StatsMetrics) error {
	modelName = strings.TrimSpace(modelName)
	if channelID == 0 {
		return nil
	}
	if modelName == "" {
		log.Warnf("skipped daily channel-model update: channelID=%d, modelName empty", channelID)
		return nil
	}
	entry := model.StatsDailyChannelModel{
		Date:         today(),
		ChannelID:    channelID,
		ChannelName:  strings.TrimSpace(channelName),
		ModelName:    modelName,
		StatsMetrics: metrics,
	}
	if err := upsertDailyChannelModels(db.GetDB().WithContext(ctx), []model.StatsDailyChannelModel{entry}); err != nil {
		// 写入失败：入队重试（SaveDB 时批量补偿）
		retryQueue.push(entry)
		return err
	}
	return nil
}

// TotalUpdate adds metrics to the running total statistics.
func TotalUpdate(metrics model.StatsMetrics) error {
	totalCacheLock.Lock()
	defer totalCacheLock.Unlock()
	if totalCache.ID == 0 {
		totalCache.ID = 1
	}
	totalCache.StatsMetrics.Add(metrics)
	// Redis 增量：实时累加到 Redis，崩溃不丢（issue #123）。内存镜像仍更新供同步读。
	if store.Enabled() {
		_ = store.GetStats().IncrMetrics(context.Background(), statsScopeTotal, statsIDTotal, metrics)
	}
	return nil
}

// ChannelUpdate adds metrics to a specific channel's statistics.
func ChannelUpdate(channelID int, metrics model.StatsMetrics) error {
	channelMutationLock.Lock()
	defer channelMutationLock.Unlock()

	channelEntry, ok := channelCache.Get(channelID)
	if !ok {
		channelEntry = model.StatsChannel{
			ChannelID: channelID,
		}
	}
	channelEntry.StatsMetrics.Add(metrics)
	channelCache.Set(channelID, channelEntry)
	channelCacheNeedUpdateLock.Lock()
	channelCacheNeedUpdate[channelID] = struct{}{}
	channelCacheNeedUpdateLock.Unlock()
	// Redis 增量：实时累加到 Redis，崩溃不丢（issue #123）。内存镜像仍更新供同步读。
	if store.Enabled() {
		_ = store.GetStats().IncrMetrics(context.Background(), statsScopeChannel, strconv.Itoa(channelID), metrics)
	}
	return nil
}

// HourlyUpdate adds metrics to the current hour's statistics.
func HourlyUpdate(metrics model.StatsMetrics) error {
	nowTime := now()
	nowHour := nowTime.Hour()
	todayDate := nowTime.Format("20060102")

	hourlyCacheLock.Lock()
	defer hourlyCacheLock.Unlock()

	if hourlyCache[nowHour].Date != todayDate {
		hourlyCache[nowHour] = model.StatsHourly{
			Hour: nowHour,
			Date: todayDate,
		}
	}

	hourlyCache[nowHour].StatsMetrics.Add(metrics)
	// Redis 增量：实时累加到 Redis（issue #123）。scope=id 按 "date:hour" 维度。
	if store.Enabled() {
		_ = store.GetStats().IncrMetrics(context.Background(), statsScopeHourly,
			fmt.Sprintf("%s:%d", todayDate, nowHour), metrics)
	}
	return nil
}

// ModelUpdate updates or creates a model's statistics entry.
func ModelUpdate(s model.StatsModel) error {
	modelMutationLock.Lock()
	defer modelMutationLock.Unlock()

	modelEntry, ok := modelCache.Get(s.ID)
	if !ok {
		modelEntry = model.StatsModel{
			ID:        s.ID,
			Name:      s.Name,
			ChannelID: s.ChannelID,
		}
	}
	if s.Name != "" {
		modelEntry.Name = s.Name
	}
	if s.ChannelID != 0 {
		modelEntry.ChannelID = s.ChannelID
	}
	modelEntry.StatsMetrics.Add(s.StatsMetrics)
	modelCache.Set(s.ID, modelEntry)
	modelCacheNeedUpdateLock.Lock()
	modelCacheNeedUpdate[s.ID] = struct{}{}
	modelCacheNeedUpdateLock.Unlock()
	// 记录最后活跃时间，供 PurgeIdleModelStats 周期回收判定（见 issue #124）。
	touchModelActivity(s.ID)
	// Redis 增量：实时累加到 Redis，崩溃不丢（issue #123）。
	if store.Enabled() {
		_ = store.GetStats().IncrMetrics(context.Background(), statsScopeModel, strconv.FormatInt(s.ID, 10), s.StatsMetrics)
	}
	return nil
}

// touchModelActivity 更新某 modelID 的最后活跃时间为当前时刻。
// 使用 *int64 原地写入，避免每次记录都向 sync.Map 分配新指针。
func touchModelActivity(modelID int64) {
	now := time.Now().UnixNano()
	if v, ok := modelLastActivity.Load(modelID); ok {
		if p, ok := v.(*int64); ok {
			atomic.StoreInt64(p, now)
			return
		}
	}
	var pVal int64
	p := &pVal
	atomic.StoreInt64(p, now)
	actual, _ := modelLastActivity.LoadOrStore(modelID, p)
	if ap, ok := actual.(*int64); ok && ap != p {
		atomic.StoreInt64(ap, now)
	}
}

// ModelList returns all cached model statistics.
func ModelList() []model.StatsModel {
	stats := modelCache.GetAll()
	if len(stats) == 0 {
		return nil
	}

	result := make([]model.StatsModel, 0, len(stats))
	for _, item := range stats {
		result = append(result, item)
	}
	return result
}

// ModelRecord records metrics for a specific model on a specific channel.
func ModelRecord(channelID int, modelName string, metrics model.StatsMetrics) error {
	normalizedName := strings.TrimSpace(modelName)
	if normalizedName == "" {
		return nil
	}
	return ModelUpdate(model.StatsModel{
		ID:           buildModelID(channelID, normalizedName),
		Name:         normalizedName,
		ChannelID:    channelID,
		StatsMetrics: metrics,
	})
}

func buildModelID(channelID int, modelName string) int64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(fmt.Sprintf("%d:%s", channelID, strings.ToLower(strings.TrimSpace(modelName)))))
	return int64(hash.Sum64() & 0x7fffffffffffffff)
}

// PurgeIdleModelStats 清理长时间未活动的 per-model 统计条目，防止 modelCache 无界增长。
// idleFor 为最大空闲时长，超过则删除。由 relay log flush 定时任务周期性调用
// （见 issue #124）。
//
// 安全性：
//  1. 若某 modelID 仍处于 modelCacheNeedUpdate（自上次 SaveDB 尚未落盘），则跳过
//     本次回收，避免丢失未持久化的统计增量。
//  2. 仅回收零统计条目（StatsMetrics.IsZero()==true）。有真实统计数据的条目即使
//     空闲也必须保留在内存 cache 中——模型广场（ModelMarket）通过
//     modelCache.GetAll() 读取数据，而 RefreshCache 只在启动时执行一次；若把有
//     数据的条目从内存删除，DB 虽仍有记录，但模型广场会持续显示该模型统计数据为空，
//     直到下次重启才会恢复（issue #126）。
//  3. 真正可安全回收的是"零统计"条目：刷量/随机 model 名探测产生的空壳条目，
//     从未真正完成过有效请求。这类条目既不在内存有意义，DB 中也是零行，删除
//     不会影响模型广场展示。
//
// 返回删除的条目数。
func PurgeIdleModelStats(idleFor time.Duration) int {
	if idleFor <= 0 {
		return 0
	}
	threshold := time.Now().Add(-idleFor).UnixNano()
	removed := 0
	modelLastActivity.Range(func(key, value any) bool {
		modelID, ok := key.(int64)
		if !ok {
			modelLastActivity.Delete(key)
			removed++
			return true
		}
		p, ok := value.(*int64)
		if !ok {
			modelLastActivity.Delete(key)
			removed++
			return true
		}
		last := atomic.LoadInt64(p)
		// 零值（从未记录，理论不会发生）或最后活跃时间早于阈值（空闲超过 idleFor）才回收。
		// last >= threshold 表示近期有活动，保留。
		if last == 0 || last < threshold {
			// 仍在 dirty 集合中：尚未落盘，跳过以防丢失增量。
			modelCacheNeedUpdateLock.Lock()
			_, dirty := modelCacheNeedUpdate[modelID]
			modelCacheNeedUpdateLock.Unlock()
			if dirty {
				return true
			}
			// 仅回收零统计条目。有真实统计数据的条目即使空闲也保留在内存 cache，
			// 否则模型广场会丢失该模型的数据展示（issue #126）。
			entry, ok := modelCache.Get(modelID)
			if !ok {
				// cache 与 activity 索引不一致：cache 已无此条目，清理 activity 索引。
				modelLastActivity.Delete(key)
				removed++
				return true
			}
			if !entry.StatsMetrics.IsZero() {
				// 有统计数据：保留在内存中供模型广场读取，但刷新活动时间戳，
				// 避免每轮都重复走到这里（轻微优化）。
				touchModelActivity(modelID)
				return true
			}
			modelCache.Del(modelID)
			modelLastActivity.Delete(key)
			removed++
		}
		return true
	})
	return removed
}

// APIKeyUpdate adds metrics to a specific API key's statistics.
func APIKeyUpdate(apiKeyID int, metrics model.StatsMetrics) error {
	apiKeyMutationLock.Lock()
	defer apiKeyMutationLock.Unlock()

	apiKeyEntry, ok := apiKeyCache.Get(apiKeyID)
	if !ok {
		apiKeyEntry = model.StatsAPIKey{
			APIKeyID: apiKeyID,
		}
	}
	apiKeyEntry.StatsMetrics.Add(metrics)
	apiKeyCache.Set(apiKeyID, apiKeyEntry)
	apiKeyCacheNeedUpdateLock.Lock()
	apiKeyCacheNeedUpdate[apiKeyID] = struct{}{}
	apiKeyCacheNeedUpdateLock.Unlock()
	// Redis 增量：实时累加到 Redis，崩溃不丢（issue #123）。
	if store.Enabled() {
		_ = store.GetStats().IncrMetrics(context.Background(), statsScopeAPIKey, strconv.Itoa(apiKeyID), metrics)
	}
	return nil
}

// ChannelDel removes a channel's statistics cache entry and database record.
func ChannelDel(id int) error {
	channelMutationLock.Lock()
	defer channelMutationLock.Unlock()

	if _, ok := channelCache.Get(id); !ok {
		return nil
	}
	channelCache.Del(id)
	channelCacheNeedUpdateLock.Lock()
	delete(channelCacheNeedUpdate, id)
	channelCacheNeedUpdateLock.Unlock()
	// Redis 后端：同步删除该渠道的增量 scope，避免残留（issue #123）。
	if store.Enabled() {
		_ = store.GetStats().Delete(context.Background(), statsScopeChannel, strconv.Itoa(id))
	}
	return db.GetDB().Delete(&model.StatsChannel{}, id).Error
}

// APIKeyDel removes an API key's statistics cache entry and database record.
func APIKeyDel(id int) error {
	apiKeyMutationLock.Lock()
	defer apiKeyMutationLock.Unlock()

	if _, ok := apiKeyCache.Get(id); !ok {
		return nil
	}
	apiKeyCache.Del(id)
	apiKeyCacheNeedUpdateLock.Lock()
	delete(apiKeyCacheNeedUpdate, id)
	apiKeyCacheNeedUpdateLock.Unlock()
	// Redis 后端：同步删除该 apikey 的增量 scope，避免残留（issue #123）。
	if store.Enabled() {
		_ = store.GetStats().Delete(context.Background(), statsScopeAPIKey, strconv.Itoa(id))
	}
	return db.GetDB().Delete(&model.StatsAPIKey{}, id).Error
}

// TotalGet returns the cached total statistics.
func TotalGet() model.StatsTotal {
	totalCacheLock.RLock()
	defer totalCacheLock.RUnlock()
	return totalCache
}

// TodayGet returns the cached daily statistics for today.
func TodayGet() model.StatsDaily {
	dailyCacheLock.RLock()
	defer dailyCacheLock.RUnlock()
	return dailyCache
}

// ChannelGet returns statistics for a specific channel, creating an empty entry if not cached.
func ChannelGet(id int) model.StatsChannel {
	channelMutationLock.Lock()
	defer channelMutationLock.Unlock()

	stats, ok := channelCache.Get(id)
	if !ok {
		tmp := model.StatsChannel{
			ChannelID: id,
		}
		channelCache.Set(id, tmp)
		channelCacheNeedUpdateLock.Lock()
		channelCacheNeedUpdate[id] = struct{}{}
		channelCacheNeedUpdateLock.Unlock()
		return tmp
	}
	return stats
}

// APIKeyGet returns statistics for a specific API key, creating an empty entry if not cached.
func APIKeyGet(id int) model.StatsAPIKey {
	apiKeyMutationLock.Lock()
	defer apiKeyMutationLock.Unlock()

	stats, ok := apiKeyCache.Get(id)
	if !ok {
		tmp := model.StatsAPIKey{
			APIKeyID: id,
		}
		apiKeyCache.Set(id, tmp)
		apiKeyCacheNeedUpdateLock.Lock()
		apiKeyCacheNeedUpdate[id] = struct{}{}
		apiKeyCacheNeedUpdateLock.Unlock()
		return tmp
	}
	return stats
}

// APIKeyList returns all cached API key statistics.
func APIKeyList() []model.StatsAPIKey {
	apiKeys := make([]model.StatsAPIKey, 0, apiKeyCache.Len())
	for _, v := range apiKeyCache.GetAll() {
		apiKeys = append(apiKeys, v)
	}
	return apiKeys
}

// ChannelList returns all cached channel statistics.
func ChannelList() []model.StatsChannel {
	channels := make([]model.StatsChannel, 0, channelCache.Len())
	for _, v := range channelCache.GetAll() {
		channels = append(channels, v)
	}
	return channels
}

// HourlyGet returns statistics for the current day's hours (up to the current hour).
func HourlyGet() []model.StatsHourly {
	nowTime := now()
	currentHour := nowTime.Hour()
	todayDate := nowTime.Format("20060102")

	hourlyCacheLock.RLock()
	defer hourlyCacheLock.RUnlock()

	result := make([]model.StatsHourly, 0, currentHour+1)

	for hour := 0; hour <= currentHour; hour++ {
		if hourlyCache[hour].Date == todayDate {
			result = append(result, hourlyCache[hour])
		} else {
			result = append(result, model.StatsHourly{
				Hour: hour,
				Date: todayDate,
			})
		}
	}

	return result
}

var (
	dailyAllCacheMu sync.RWMutex
	dailyAllCache   []model.StatsDaily
	dailyAllCached  bool
)

// InvalidateDailyCache clears the cached daily statistics list.
func InvalidateDailyCache() {
	dailyAllCacheMu.Lock()
	dailyAllCached = false
	dailyAllCache = nil
	dailyAllCacheMu.Unlock()
}

func mergeDailyRowsWithCurrent(rows []model.StatsDaily, current model.StatsDaily) []model.StatsDaily {
	result := append([]model.StatsDaily{}, rows...)
	if current.Date == "" {
		return result
	}
	for i := range result {
		if result[i].Date == current.Date {
			result[i] = current
			return result
		}
	}
	if !current.StatsMetrics.IsZero() {
		result = append(result, current)
	}
	return result
}

func upsertDailyAllCache(entries ...model.StatsDaily) {
	dailyAllCacheMu.Lock()
	defer dailyAllCacheMu.Unlock()
	if !dailyAllCached {
		return
	}
	for _, entry := range entries {
		if entry.Date == "" {
			continue
		}
		replaced := false
		for i := range dailyAllCache {
			if dailyAllCache[i].Date == entry.Date {
				dailyAllCache[i] = entry
				replaced = true
				break
			}
		}
		if !replaced && !entry.StatsMetrics.IsZero() {
			dailyAllCache = append(dailyAllCache, entry)
		}
	}
}

// GetDaily retrieves all daily statistics records from the database.
func GetDaily(ctx context.Context) ([]model.StatsDaily, error) {
	dailyAllCacheMu.RLock()
	if dailyAllCached {
		result := append([]model.StatsDaily(nil), dailyAllCache...)
		dailyAllCacheMu.RUnlock()
		return mergeDailyRowsWithCurrent(result, TodayGet()), nil
	}
	dailyAllCacheMu.RUnlock()

	var statsDaily []model.StatsDaily
	result := db.GetDB().WithContext(ctx).Find(&statsDaily)
	if result.Error != nil {
		return nil, result.Error
	}
	dailyAllCacheMu.Lock()
	dailyAllCache = append([]model.StatsDaily(nil), statsDaily...)
	dailyAllCached = true
	dailyAllCacheMu.Unlock()
	return mergeDailyRowsWithCurrent(append([]model.StatsDaily{}, statsDaily...), TodayGet()), nil
}

// OnChannelDeleted is called by the op package when a channel is deleted,
// so that the channel's stats cache entry is cleaned up.
func OnChannelDeleted(channelID int) {
	channelMutationLock.Lock()
	channelCache.Del(channelID)
	channelCacheNeedUpdateLock.Lock()
	delete(channelCacheNeedUpdate, channelID)
	channelCacheNeedUpdateLock.Unlock()
	channelMutationLock.Unlock()
	// Redis 后端：同步删除该 channel 的 stats 增量 scope，避免残留（issue #123）。
	if store.Enabled() {
		_ = store.GetStats().Delete(context.Background(), statsScopeChannel, strconv.Itoa(channelID))
	}
}

// OnAPIKeyDeleted is called by the op package when an API key is deleted,
// so that the API key's stats cache entry is cleaned up.
func OnAPIKeyDeleted(apiKeyID int) {
	apiKeyMutationLock.Lock()
	apiKeyCache.Del(apiKeyID)
	apiKeyCacheNeedUpdateLock.Lock()
	delete(apiKeyCacheNeedUpdate, apiKeyID)
	apiKeyCacheNeedUpdateLock.Unlock()
	apiKeyMutationLock.Unlock()
	// Redis 后端：同步删除该 apikey 的 stats scope，避免残留增量（issue #123）。
	if store.Enabled() {
		_ = store.GetStats().Delete(context.Background(), statsScopeAPIKey, strconv.Itoa(apiKeyID))
	}
}

// ModelMetricsByName aggregates model statistics by model name (across all channels).
func ModelMetricsByName() map[string]model.StatsMetrics {
	statsByName := make(map[string]model.StatsMetrics, modelCache.Len())
	for _, stats := range modelCache.GetAll() {
		name := strings.TrimSpace(stats.Name)
		if name == "" {
			continue
		}
		aggregated := statsByName[name]
		aggregated.Add(stats.StatsMetrics)
		statsByName[name] = aggregated
	}
	return statsByName
}

// ---------------------------------------------------------------------------
// Test helpers (exported for use by tests in the op package)
// ---------------------------------------------------------------------------

// SetTimeNowForTest replaces time.Now for testing. Returns a cleanup function.
func SetTimeNowForTest(fn func() time.Time) func() {
	orig := timeNow
	timeNow = fn
	return func() { timeNow = orig }
}

// ClearAllCachesForTest clears all stats caches for test isolation.
func ClearAllCachesForTest() {
	totalCacheLock.Lock()
	totalCache = model.StatsTotal{}
	totalCacheLock.Unlock()

	dailyCacheLock.Lock()
	dailyCache = model.StatsDaily{}
	dailyCacheLock.Unlock()
	InvalidateDailyCache()

	hourlyCacheLock.Lock()
	hourlyCache = [24]model.StatsHourly{}
	hourlyCacheLock.Unlock()

	channelCache.Clear()
	channelCacheNeedUpdateLock.Lock()
	channelCacheNeedUpdate = make(map[int]struct{})
	channelCacheNeedUpdateLock.Unlock()

	modelCache.Clear()
	modelCacheNeedUpdateLock.Lock()
	modelCacheNeedUpdate = make(map[int64]struct{})
	modelCacheNeedUpdateLock.Unlock()
	clearModelActivityForTest()

	apiKeyCache.Clear()
	apiKeyCacheNeedUpdateLock.Lock()
	apiKeyCacheNeedUpdate = make(map[int]struct{})
	apiKeyCacheNeedUpdateLock.Unlock()
}

// clearModelActivityForTest 清空 modelLastActivity，供测试隔离使用。
func clearModelActivityForTest() {
	modelLastActivity.Range(func(key, _ any) bool {
		modelLastActivity.Delete(key)
		return true
	})
	retryQueue.mu.Lock()
	retryQueue.entries = retryQueue.entries[:0]
	retryQueue.mu.Unlock()
}

// ResetCachesForTest resets all stats caches to a known state for testing.
// channelID/apiKeyID set to 0 means skip that cache. modelID set to 0 means skip model cache.
func ResetCachesForTest(total model.StatsTotal, daily model.StatsDaily, channelID int, modelID int64, apiKeyID int) {
	totalCacheLock.Lock()
	totalCache = total
	totalCacheLock.Unlock()

	dailyCacheLock.Lock()
	dailyCache = daily
	dailyCacheLock.Unlock()

	if channelID != 0 {
		channelCache.Set(channelID, model.StatsChannel{ChannelID: channelID})
		channelCacheNeedUpdateLock.Lock()
		channelCacheNeedUpdate[channelID] = struct{}{}
		channelCacheNeedUpdateLock.Unlock()
	}
	if modelID != 0 {
		modelCache.Set(modelID, model.StatsModel{ID: modelID, Name: "gpt-4o", ChannelID: channelID})
		modelCacheNeedUpdateLock.Lock()
		modelCacheNeedUpdate[modelID] = struct{}{}
		modelCacheNeedUpdateLock.Unlock()
		touchModelActivity(modelID)
	}
	if apiKeyID != 0 {
		apiKeyCache.Set(apiKeyID, model.StatsAPIKey{APIKeyID: apiKeyID})
		apiKeyCacheNeedUpdateLock.Lock()
		apiKeyCacheNeedUpdate[apiKeyID] = struct{}{}
		apiKeyCacheNeedUpdateLock.Unlock()
	}
}

// GetChannelDirtyIDs returns the set of channel IDs marked as dirty.
func GetChannelDirtyIDs() []int {
	channelCacheNeedUpdateLock.Lock()
	defer channelCacheNeedUpdateLock.Unlock()
	ids := make([]int, 0, len(channelCacheNeedUpdate))
	for id := range channelCacheNeedUpdate {
		ids = append(ids, id)
	}
	return ids
}

// GetRetryQueueLen returns the current retry queue size (for tests and monitoring).
func GetRetryQueueLen() int {
	return retryQueue.len()
}

// GetModelDirtyIDs returns the set of model IDs marked as dirty.
func GetModelDirtyIDs() []int64 {
	modelCacheNeedUpdateLock.Lock()
	defer modelCacheNeedUpdateLock.Unlock()
	ids := make([]int64, 0, len(modelCacheNeedUpdate))
	for id := range modelCacheNeedUpdate {
		ids = append(ids, id)
	}
	return ids
}

// GetAPIKeyDirtyIDs returns the set of API key IDs marked as dirty.
func GetAPIKeyDirtyIDs() []int {
	apiKeyCacheNeedUpdateLock.Lock()
	defer apiKeyCacheNeedUpdateLock.Unlock()
	ids := make([]int, 0, len(apiKeyCacheNeedUpdate))
	for id := range apiKeyCacheNeedUpdate {
		ids = append(ids, id)
	}
	return ids
}
