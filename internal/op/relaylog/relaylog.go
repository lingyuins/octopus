package relaylog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/cacheusage"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/lingyuins/octopus/internal/utils/snowflake"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const relayLogMaxSize = 200
const relayLogMaxSizeNoDB = 200 // 当不保存到数据库时，允许更大的缓存用于实时查询
const relayLogStreamTokenTTL = 5 * time.Minute

var relayLogCache = make([]model.RelayLog, 0, relayLogMaxSize)
var relayLogCacheLock sync.Mutex

func GetCacheAndLock() ([]model.RelayLog, *sync.Mutex) { return relayLogCache, &relayLogCacheLock }

var relayLogFlushLock sync.Mutex

var flushCh = make(chan struct{}, 1)

// notifyCh 缓冲待广播给实时订阅者的日志。RelayLogAdd 在每条日志写入时把日志
// 非阻塞地推入该 channel，由单个常驻分发 goroutine（见 startNotifyWorker）
// 顺序广播给订阅者。这样避免了每条日志都启动一个短命 goroutine（高 QPS 下的
// goroutine 风暴），并保证订阅者按写入顺序收到日志。channel 满时丢弃最新日志，
// 与 notifySubscribers 对慢订阅者的丢弃语义一致。
var notifyCh = make(chan model.RelayLog, 1024)

func startNotifyWorker(ctx context.Context) {
	go func() {
		for {
			select {
			case relayLog := <-notifyCh:
				notifySubscribers(relayLog)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func triggerFlush() {
	select {
	case flushCh <- struct{}{}:
	default:
	}
}

func StartFlushWorker(ctx context.Context) {
	startNotifyWorker(ctx)
	go func() {
		for {
			select {
			case <-flushCh:
				if db.IsLogSQLite() {
					db.EnqueueWrite(db.WriteJob{Name: "relay_log_flush", Fn: func(_ context.Context) error {
						return relayLogFlushToDB(context.Background())
					}})
				} else {
					if err := relayLogFlushToDB(context.Background()); err != nil {
						log.Warnf("async relay log flush failed: %v", err)
					}
				}
			case <-ctx.Done():
				_ = relayLogFlushToDB(context.Background())
				return
			}
		}
	}()
}

var relayLogSubscribers = make(map[chan model.RelayLog]struct{})
var relayLogSubscribersLock sync.RWMutex

var relayLogStreamTokens = make(map[string]time.Time)
var relayLogStreamTokensLock sync.RWMutex

func RelayLogStreamTokenCreate() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)
	createdAt := time.Now()

	relayLogStreamTokensLock.Lock()
	relayLogStreamTokens[token] = createdAt
	relayLogStreamTokensLock.Unlock()

	return token, nil
}

func RelayLogStreamTokenVerify(token string) bool {
	now := time.Now()

	relayLogStreamTokensLock.Lock()
	createdAt, ok := relayLogStreamTokens[token]
	if !ok {
		relayLogStreamTokensLock.Unlock()
		return false
	}
	if now.Sub(createdAt) > relayLogStreamTokenTTL {
		delete(relayLogStreamTokens, token)
		relayLogStreamTokensLock.Unlock()
		return false
	}
	relayLogStreamTokensLock.Unlock()
	return true
}

func RelayLogStreamTokenRevoke(token string) {
	relayLogStreamTokensLock.Lock()
	delete(relayLogStreamTokens, token)
	relayLogStreamTokensLock.Unlock()
}

func RelayLogSubscribe() chan model.RelayLog {
	ch := make(chan model.RelayLog, 10)
	relayLogSubscribersLock.Lock()
	relayLogSubscribers[ch] = struct{}{}
	relayLogSubscribersLock.Unlock()
	return ch
}

func RelayLogUnsubscribe(ch chan model.RelayLog) {
	relayLogSubscribersLock.Lock()
	if _, ok := relayLogSubscribers[ch]; ok {
		delete(relayLogSubscribers, ch)
		close(ch)
	}
	relayLogSubscribersLock.Unlock()
}

func notifySubscribers(relayLog model.RelayLog) {
	relayLogSubscribersLock.RLock()
	defer relayLogSubscribersLock.RUnlock()

	for ch := range relayLogSubscribers {
		select {
		case ch <- relayLog:
		default:
		}
	}
}

func relayLogStreamTokenCleanup(now time.Time) {
	relayLogStreamTokensLock.Lock()
	for token, createdAt := range relayLogStreamTokens {
		if now.Sub(createdAt) > relayLogStreamTokenTTL {
			delete(relayLogStreamTokens, token)
		}
	}
	relayLogStreamTokensLock.Unlock()
}

func relayLogFlushToDB(ctx context.Context) error {
	relayLogFlushLock.Lock()
	defer relayLogFlushLock.Unlock()

	relayLogCacheLock.Lock()
	if len(relayLogCache) == 0 {
		relayLogCacheLock.Unlock()
		return nil
	}
	batch := make([]model.RelayLog, len(relayLogCache))
	copy(batch, relayLogCache)
	// 记录 batch 中最后一条日志的 ID，用于安全截断
	lastFlushedID := batch[len(batch)-1].ID
	relayLogCacheLock.Unlock()

	// 独立日志库被关闭（CloseLogDB）时 GetLogDB 返回 nil；此时不应写入，
	// 保留缓存等待下次（重开后）刷盘。共用主库模式下永远非 nil。
	conn := db.GetLogDB()
	if conn == nil {
		return nil
	}

	result := conn.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&batch)
	if result.Error != nil {
		return result.Error
	}

	relayLogCacheLock.Lock()
	// 安全截断：只移除 ID <= lastFlushedID 的前缀部分
	cutIdx := 0
	for i, l := range relayLogCache {
		if l.ID == lastFlushedID {
			cutIdx = i + 1
			break
		}
		if l.ID > lastFlushedID {
			// 遇到比 batch 更新的日志，说明截断点已过
			break
		}
	}
	if cutIdx > 0 {
		relayLogCache = relayLogCache[cutIdx:]
	}
	if len(relayLogCache) == 0 {
		relayLogCache = make([]model.RelayLog, 0, relayLogMaxSize)
	}
	relayLogCacheLock.Unlock()

	return nil
}

func RelayLogAdd(ctx context.Context, relayLog model.RelayLog) error {
	enabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return err
	}
	maxSize := relayLogMaxSize
	if !enabled {
		maxSize = relayLogMaxSizeNoDB
	}
	relayLog.ID = snowflake.GenerateID()
	// 非阻塞地推入通知 channel，由常驻分发 goroutine 顺序广播给订阅者。
	// 避免每条日志启动一个短命 goroutine；channel 满时丢弃。
	select {
	case notifyCh <- relayLog:
	default:
	}

	relayLogCacheLock.Lock()
	relayLogCache = append(relayLogCache, relayLog)
	if len(relayLogCache) >= maxSize {
		if enabled {
			relayLogCacheLock.Unlock()
			triggerFlush()
			return nil
		}
		keepSize := maxSize / 2
		if len(relayLogCache) > keepSize {
			relayLogCache = relayLogCache[len(relayLogCache)-keepSize:]
		}
	}
	relayLogCacheLock.Unlock()
	return nil
}

// RelayLogAttemptsAdd 把一条 RelayLog 的各次尝试写入 relay_log_attempts 关联表，
// 使失败尝试（尤其"渠道A 失败→重试到B 成功"中的渠道A）可被按 channel_id 过滤/聚合。
// 日志关闭时不写（enabled=false）。写入走日志库连接，SQLite 下排队进写队列避免争用。
// relayLogID 必须已分配（即 RelayLogAdd 之后调用）。返回错误仅供记录，调用方通常忽略。
func RelayLogAttemptsAdd(ctx context.Context, relayLogID int64, attempts []model.ChannelAttempt, logTime int64) error {
	if len(attempts) == 0 {
		return nil
	}
	enabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	rows := make([]model.RelayLogAttempt, 0, len(attempts))
	for _, a := range attempts {
		if a.ChannelID == 0 {
			continue // 跳过无渠道归属的占位尝试
		}
		rows = append(rows, model.RelayLogAttempt{
			RelayLogID:  relayLogID,
			ChannelID:   a.ChannelID,
			ChannelName: a.ChannelName,
			ModelName:   a.ModelName,
			Status:      string(a.Status),
			Duration:    a.Duration,
			Time:        logTime,
		})
	}
	if len(rows) == 0 {
		return nil
	}

	write := func(c context.Context) error {
		conn := db.GetLogDB()
		if conn == nil {
			return nil
		}
		return conn.WithContext(c).Create(&rows).Error
	}
	if db.IsLogSQLite() {
		db.EnqueueWrite(db.WriteJob{Name: "relay_log_attempts", Fn: write})
		return nil
	}
	return write(ctx)
}

func RelayLogSaveDBTask(ctx context.Context) error {
	log.Debugf("relay log save db task started")
	startTime := time.Now()
	defer func() {
		log.Debugf("relay log save db task finished, save time: %s", time.Since(startTime))
	}()
	now := time.Now()
	defer relayLogStreamTokenCleanup(now)
	enabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return err
	}

	if enabled {
		if err := relayLogFlushToDB(ctx); err != nil {
			return err
		}
		return relayLogCleanup(ctx)
	}

	// 日志关闭：清空数据库中所有历史日志以释放磁盘空间
	if err := relayLogCleanupAll(ctx); err != nil {
		log.Warnf("failed to cleanup all logs from DB: %v", err)
	}

	relayLogCacheLock.Lock()
	if len(relayLogCache) > relayLogMaxSizeNoDB {
		keepSize := relayLogMaxSizeNoDB / 2
		relayLogCache = relayLogCache[len(relayLogCache)-keepSize:]
	}
	relayLogCacheLock.Unlock()

	return nil
}

// ApplyKeepEnabledChange 在「保留历史日志」开关变更后调整独立日志库连接：
//   - 关闭日志：先清空日志表释放空间，再断开独立日志库连接（释放文件句柄/连接池）；
//   - 开启日志：重连独立日志库。
//
// 仅在「独立日志库」模式下有实际效果；共用主库时 db.CloseLogDB/ReopenLogDB 均为
// 空操作，绝不会触碰主库连接。关闭日志后 RelayLogAdd 不再触发 DB 写入，因此断连安全。
func ApplyKeepEnabledChange(ctx context.Context, enabled bool) error {
	if !db.IsLogDBSeparate() {
		return nil
	}
	if enabled {
		return db.ReopenLogDB()
	}
	// 关闭：先清空（此时连接仍在），再断开。
	if err := relayLogCleanupAll(ctx); err != nil {
		log.Warnf("failed to clear logs before closing log DB: %v", err)
	}
	return db.CloseLogDB()
}

func relayLogCleanup(ctx context.Context) error {
	conn := db.GetLogDB()
	if conn == nil {
		// 独立日志库已断开（如日志已关闭），无需清理。
		return nil
	}

	// Priority: keep count > keep period (days)
	keepCount, err := setting.GetInt(model.SettingKeyRelayLogKeepCount)
	if err != nil {
		return err
	}

	if keepCount > 0 {
		// Count-based cleanup with batch deletion (50% when over threshold)
		// Avoids high-frequency small deletes under heavy load
		var total int64
		if err := conn.Model(&model.RelayLog{}).Count(&total).Error; err != nil {
			return err
		}
		if total <= int64(keepCount) {
			return nil // Under threshold, no cleanup needed
		}
		// Delete 50% of current records to create buffer before next cleanup.
		// 先取出待删区间的边界 id（按 id 升序第 deleteCount 个），再以
		// `id < threshold` 做范围删除——走主键范围扫描，避免 `id IN (子查询)`
		// 在大表上重复扫描子查询结果集，显著加快清理速度。
		deleteCount := total / 2
		var thresholdID int64
		if err := conn.WithContext(ctx).Model(&model.RelayLog{}).
			Order("id ASC").
			Offset(int(deleteCount)).
			Limit(1).
			Pluck("id", &thresholdID).Error; err != nil {
			return err
		}
		if thresholdID == 0 {
			return nil
		}
		return conn.WithContext(ctx).
			Where("id < ?", thresholdID).
			Delete(&model.RelayLog{}).Error
	}

	// Fallback to days-based cleanup
	keepPeriod, err := setting.GetInt(model.SettingKeyRelayLogKeepPeriod)
	if err != nil {
		return err
	}

	if keepPeriod <= 0 {
		return nil
	}

	cutoffTime := time.Now().Add(-time.Duration(keepPeriod) * 24 * time.Hour).Unix()
	return conn.WithContext(ctx).Where("time < ?", cutoffTime).Delete(&model.RelayLog{}).Error
}

// relayLogCleanupAll 删除数据库中所有日志记录，用于日志关闭时释放磁盘空间。
//
// 使用 db.FastClearTable（TRUNCATE / DROP+重建）而非逐行 DELETE：百万级日志在
// SQLite + WAL + 单连接下逐行删可能耗时数十分钟，整表清空近乎瞬时。
func relayLogCleanupAll(ctx context.Context) error {
	conn := db.GetLogDB()
	if conn == nil {
		// 独立日志库已断开（日志关闭场景）：无需清理。
		return nil
	}
	return db.FastClearTable(conn.WithContext(ctx), &model.RelayLog{}, "relay_logs")
}

// loadExcludedGroupSet 读取被屏蔽分组的设置（JSON 字符串数组），返回以分组
// 名称为键的集合。配置缺失或解析失败时返回空集合（即不屏蔽任何分组）。
//
// 设计说明：本系统中分组的 Name 即客户端请求时使用的“模型名”，日志的
// request_model_name 正是这个值（见 internal/relay/metrics.go 与
// op/group.GroupGetEnabledMapByEndpoint 的解析逻辑）。因此按分组名屏蔽日志
// 只需匹配 request_model_name，无需把分组解析为渠道集合，也不会因渠道被多个
// 分组共享而误伤其它分组的日志。
func loadExcludedGroupSet() map[string]struct{} {
	raw, err := setting.GetString(model.SettingKeyLogExcludedGroups)
	if err != nil || raw == "" {
		return nil
	}
	var names []string
	if err := json.Unmarshal([]byte(raw), &names); err != nil {
		return nil
	}
	if len(names) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		set[name] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

// RelayLogStreamExcluded 判断某条实时日志是否应在 SSE 流中被屏蔽。
// 供 streamLog 处理器在广播前过滤被屏蔽分组的日志。
func RelayLogStreamExcluded(requestModelName string) bool {
	excluded := loadExcludedGroupSet()
	if excluded == nil {
		return false
	}
	_, ok := excluded[requestModelName]
	return ok
}

// LogFilter 日志列表筛选参数
type LogFilter struct {
	StartTime    *int
	EndTime      *int
	Model        string // 模糊匹配 request_model_name 或 actual_model_name
	ChannelID    *int
	APIKeyID     *int
	EndpointType string
	HasError     *bool // nil=全部, false=仅成功, true=仅失败
	// IncludeAttempts 控制 channel_id / HasError 过滤是否"穿透"到单次尝试维度。
	// 为 true 时，"在渠道A 失败→重试到B 成功"的请求也会被 ChannelID=A 命中，
	// HasError=true 也会命中整体成功但含失败尝试的请求（issue #67）。
	IncludeAttempts bool
}

// logHasFailedAttempt 报告该日志是否存在任意一次失败的渠道尝试。
func logHasFailedAttempt(l model.RelayLog) bool {
	for _, a := range l.Attempts {
		if a.Status == model.AttemptFailed {
			return true
		}
	}
	return false
}

// logMatchesAttemptChannel 报告该日志是否在指定渠道上有过尝试（任意状态）。
// 成败维度由调用方经 HasError 单独判定，避免 ChannelID 与 HasError 语义耦合。
func logMatchesAttemptChannel(l model.RelayLog, channelID int) bool {
	for _, a := range l.Attempts {
		if a.ChannelID == channelID {
			return true
		}
	}
	return false
}

// RelayLogList 查询日志列表，支持可选的筛选条件
// 返回轻量条目，不包含 request_content 和 response_content 大字段
func RelayLogList(ctx context.Context, filter LogFilter, page, pageSize int) ([]model.RelayLogListItem, error) {
	enabled, err := setting.GetBool(model.SettingKeyRelayLogKeepEnabled)
	if err != nil {
		return nil, err
	}
	hasFilter := filter.StartTime != nil || filter.EndTime != nil ||
		filter.Model != "" || filter.ChannelID != nil || filter.APIKeyID != nil ||
		filter.EndpointType != "" || filter.HasError != nil
	excludedGroups := loadExcludedGroupSet()

	matchesFilter := func(log model.RelayLog) bool {
		if filter.StartTime != nil && log.Time < int64(*filter.StartTime) {
			return false
		}
		if filter.EndTime != nil && log.Time > int64(*filter.EndTime) {
			return false
		}
		if excludedGroups != nil {
			if _, ok := excludedGroups[log.RequestModelName]; ok {
				return false
			}
		}
		if filter.Model != "" {
			modelLower := strings.ToLower(filter.Model)
			if !strings.Contains(strings.ToLower(log.RequestModelName), modelLower) &&
				!strings.Contains(strings.ToLower(log.ActualModelName), modelLower) {
				return false
			}
		}
		if filter.ChannelID != nil {
			if log.ChannelId == *filter.ChannelID {
				// 顶层渠道命中，直接通过
			} else if filter.IncludeAttempts && logMatchesAttemptChannel(log, *filter.ChannelID) {
				// 该请求在某次尝试中用到了目标渠道，命中（成败由 HasError 单独判定）
			} else {
				return false
			}
		}
		if filter.APIKeyID != nil && log.RequestAPIKeyID != *filter.APIKeyID {
			return false
		}
		if filter.EndpointType != "" && log.EndpointType != filter.EndpointType {
			return false
		}
		if filter.HasError != nil {
			if *filter.HasError {
				// 只看"失败"：整体失败 或（开启穿透时）任一次尝试失败
				if log.Error == "" && !(filter.IncludeAttempts && logHasFailedAttempt(log)) {
					return false
				}
			} else {
				// 只看"成功"：整体成功 且（开启穿透时）不含失败尝试
				if log.Error != "" {
					return false
				}
				if filter.IncludeAttempts && logHasFailedAttempt(log) {
					return false
				}
			}
		}
		return true
	}

	// 锁内只做一次预分配的整体拷贝（单次 memmove，无分支、无扩容），尽量缩短
	// 与热路径 RelayLogAdd 争用同一把 Mutex 的时间；时间过滤移到锁外执行。
	relayLogCacheLock.Lock()
	snapshot := make([]model.RelayLog, len(relayLogCache))
	copy(snapshot, relayLogCache)
	relayLogCacheLock.Unlock()

	// 在锁外按条件过滤（保持原始顺序：旧 -> 新）
	var cachedLogs []model.RelayLog
	if hasFilter || excludedGroups != nil {
		cachedLogs = make([]model.RelayLog, 0, len(snapshot))
		for _, log := range snapshot {
			if !matchesFilter(log) {
				continue
			}
			cachedLogs = append(cachedLogs, log)
		}
	} else {
		cachedLogs = snapshot
	}

	cacheCount := len(cachedLogs)
	offset := (page - 1) * pageSize

	var result []model.RelayLogListItem

	// 先从缓存中按"新 -> 旧"顺序分页提取，不再整段 reverse。
	if offset < cacheCount {
		cacheTake := min(pageSize, cacheCount-offset)
		start := cacheCount - offset - 1
		for i := 0; i < cacheTake; i++ {
			idx := start - i
			if idx < 0 {
				break
			}
			result = append(result, cachedLogs[idx].ToListItem())
		}
	}

	// 如果启用了日志保存，缓存不够时从数据库补充。
	// conn 可能为 nil（独立日志库已被 CloseLogDB 断开），此时只返回缓存结果。
	conn := db.GetLogDB()
	if enabled && conn != nil {
		remaining := pageSize - len(result)
		if remaining > 0 {
			dbOffset := 0
			if offset > cacheCount {
				dbOffset = offset - cacheCount
			}

			query := conn.WithContext(ctx).
				Select("id", "time", "request_model_name", "request_api_key_id", "request_api_key_name",
					"client_ip",
					"endpoint_type", "channel_id", "channel_name", "actual_model_name",
					"input_tokens", "output_tokens", "semantic_cache_hit", "cache_read_tokens",
					"ftut", "use_time",
					"cost", "error", "attempts", "total_attempts")
			if filter.StartTime != nil {
				query = query.Where("time >= ?", *filter.StartTime)
			}
			if filter.EndTime != nil {
				query = query.Where("time <= ?", *filter.EndTime)
			}
			if len(excludedGroups) > 0 {
				names := make([]string, 0, len(excludedGroups))
				for name := range excludedGroups {
					names = append(names, name)
				}
				query = query.Where("request_model_name NOT IN ?", names)
			}
			if filter.Model != "" {
				modelPattern := "%" + strings.ToLower(filter.Model) + "%"
				query = query.Where("LOWER(request_model_name) LIKE ? OR LOWER(actual_model_name) LIKE ?", modelPattern, modelPattern)
			}
			if filter.ChannelID != nil {
				if filter.IncludeAttempts {
					// 顶层渠道 或 该请求在目标渠道上有过任意尝试（成败由 HasError 单独判定）
					query = query.Where(
						"channel_id = ? OR id IN (SELECT relay_log_id FROM relay_log_attempts WHERE channel_id = ?)",
						*filter.ChannelID, *filter.ChannelID,
					)
				} else {
					query = query.Where("channel_id = ?", *filter.ChannelID)
				}
			}
			if filter.APIKeyID != nil {
				query = query.Where("request_api_key_id = ?", *filter.APIKeyID)
			}
			if filter.EndpointType != "" {
				query = query.Where("endpoint_type = ?", filter.EndpointType)
			}
			if filter.HasError != nil {
				if *filter.HasError {
					if filter.IncludeAttempts {
						// 整体失败 或 含任意失败尝试
						query = query.Where("error != '' OR id IN (SELECT relay_log_id FROM relay_log_attempts WHERE status = ?)", string(model.AttemptFailed))
					} else {
						query = query.Where("error != ''")
					}
				} else {
					if filter.IncludeAttempts {
						// 整体成功 且 不含任何失败尝试
						query = query.Where("(error = '' OR error IS NULL) AND id NOT IN (SELECT relay_log_id FROM relay_log_attempts WHERE status = ?)", string(model.AttemptFailed))
					} else {
						query = query.Where("error = '' OR error IS NULL")
					}
				}
			}

			var dbLogs []model.RelayLogListItem
			if err := query.Order("id DESC").Offset(dbOffset).Limit(remaining).Find(&dbLogs).Error; err != nil {
				return nil, err
			}
			// semantic_cache_hit / cache_read_tokens 已在写入时落库，直接返回，
			// 无需读取并重新解析 response_content 大字段。
			result = append(result, dbLogs...)
		}
	}

	return result, nil
}

// SetCacheForTest replaces the in-memory relay log cache for testing.
// Returns a cleanup function that restores the previous cache.
func SetCacheForTest(logs []model.RelayLog) func() {
	relayLogCacheLock.Lock()
	old := relayLogCache
	relayLogCache = make([]model.RelayLog, len(logs))
	copy(relayLogCache, logs)
	relayLogCacheLock.Unlock()
	return func() {
		relayLogCacheLock.Lock()
		relayLogCache = old
		relayLogCacheLock.Unlock()
	}
}

func RelayLogCacheReadTokens(responseContent string) int {
	usage, ok := cacheusage.ParseProviderPromptCacheUsageSignals(responseContent)
	if !ok || usage.CachedTokens <= 0 {
		return 0
	}
	return int(usage.CachedTokens)
}

func RelayLogClear(ctx context.Context) error {
	relayLogCacheLock.Lock()
	relayLogCache = make([]model.RelayLog, 0, relayLogMaxSize)
	relayLogCacheLock.Unlock()
	conn := db.GetLogDB()
	if conn == nil {
		// 独立日志库已断开（后台日志关闭中）：内存缓存已清，无需触碰数据库。
		return nil
	}
	// 整表清空走 FastClearTable，避免百万级逐行 DELETE 卡住数十分钟。
	return db.FastClearTable(conn.WithContext(ctx), &model.RelayLog{}, "relay_logs")
}

// RelayLogGetByID 根据ID获取完整日志详情（包含 request_content 和 response_content）
func RelayLogGetByID(ctx context.Context, id int64) (*model.RelayLog, error) {
	// 在缓存中查找的闭包：日志库关闭或 DB 未命中时回落到内存缓存。
	lookupCache := func() (*model.RelayLog, error) {
		relayLogCacheLock.Lock()
		defer relayLogCacheLock.Unlock()
		for i := range relayLogCache {
			if relayLogCache[i].ID == id {
				cached := relayLogCache[i]
				if usage, ok := cacheusage.ParseProviderPromptCacheUsageSignals(cached.ResponseContent); ok {
					cached.SemanticCacheHit = usage.SemanticCacheHit
					if !usage.SemanticCacheHit {
						cached.CacheReadTokens = int(usage.CachedTokens)
					}
				}
				return &cached, nil
			}
		}
		return nil, nil
	}

	conn := db.GetLogDB()
	if conn == nil {
		// 日志库已断开（如关闭后台日志）：只查内存缓存。
		return lookupCache()
	}

	var relayLog model.RelayLog
	if err := conn.WithContext(ctx).Where("id = ?", id).First(&relayLog).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return lookupCache()
	}
	if usage, ok := cacheusage.ParseProviderPromptCacheUsageSignals(relayLog.ResponseContent); ok {
		relayLog.SemanticCacheHit = usage.SemanticCacheHit
		if !usage.SemanticCacheHit {
			relayLog.CacheReadTokens = int(usage.CachedTokens)
		}
	}
	return &relayLog, nil
}
