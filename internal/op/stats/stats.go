package stats

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/utils/cache"
	"github.com/lingyuins/octopus/internal/utils/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var dailyCache model.StatsDaily
var dailyCacheLock sync.RWMutex

var totalCache model.StatsTotal
var totalCacheLock sync.RWMutex

// timeNow allows tests to override time.Now.
var timeNow = time.Now

// now returns the current time adjusted by the configured timezone offset.
// When the container runs in UTC but users are in a different timezone, this ensures
// hourly/daily statistics align with the user's local date/hour boundaries.
func now() time.Time {
	offset, err := setting.GetInt(model.SettingKeyStatsTimezoneOffset)
	now := timeNow()
	if err != nil || offset == 0 {
		return now
	}
	return now.UTC().Add(time.Duration(offset) * time.Hour)
}

// Now returns the current time adjusted by the configured timezone offset.
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

var modelCache = cache.New[int, model.StatsModel](16)
var modelCacheNeedUpdate = make(map[int]struct{})
var modelCacheNeedUpdateLock sync.Mutex
var modelMutationLock sync.Mutex

var apiKeyCache = cache.New[int, model.StatsAPIKey](16)
var apiKeyCacheNeedUpdate = make(map[int]struct{})
var apiKeyCacheNeedUpdateLock sync.Mutex
var apiKeyMutationLock sync.Mutex

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
	modelIDs := make([]int, 0, len(modelCacheNeedUpdate))
	for id := range modelCacheNeedUpdate {
		modelIDs = append(modelIDs, id)
	}
	modelCacheNeedUpdate = make(map[int]struct{})
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
	return nil
}

func persistSnapshots(
	ctx context.Context,
	totalSnap model.StatsTotal,
	dailySnap model.StatsDaily,
	hourlyAll [24]model.StatsHourly,
	channelIDs []int,
	modelIDs []int,
	apiKeyIDs []int,
) error {
	dbConn := db.GetDB().WithContext(ctx)

	if result := dbConn.Save(&totalSnap); result.Error != nil {
		return result.Error
	}
	if result := dbConn.Save(&dailySnap); result.Error != nil {
		return result.Error
	}

	todayDate := today()
	hourlyStats := make([]model.StatsHourly, 0, 24)
	for hour := 0; hour < 24; hour++ {
		if hourlyAll[hour].Date == todayDate {
			hourlyStats = append(hourlyStats, hourlyAll[hour])
		}
	}
	if len(hourlyStats) > 0 {
		if result := dbConn.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "hour"}},
			UpdateAll: true,
		}).Create(&hourlyStats); result.Error != nil {
			return result.Error
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
	if err := upsertChannels(dbConn, channelStats); err != nil {
		return err
	}

	modelStats := make([]model.StatsModel, 0, len(modelIDs))
	for _, id := range modelIDs {
		m, ok := modelCache.Get(id)
		if !ok {
			continue
		}
		modelStats = append(modelStats, m)
	}
	if err := upsertModels(dbConn, modelStats); err != nil {
		return err
	}

	apiKeyStats := make([]model.StatsAPIKey, 0, len(apiKeyIDs))
	for _, id := range apiKeyIDs {
		ak, ok := apiKeyCache.Get(id)
		if !ok {
			continue
		}
		apiKeyStats = append(apiKeyStats, ak)
	}
	if err := upsertAPIKeys(dbConn, apiKeyStats); err != nil {
		return err
	}

	return nil
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
	modelIDs := make([]int, 0, len(modelCacheNeedUpdate))
	for id := range modelCacheNeedUpdate {
		modelIDs = append(modelIDs, id)
	}
	modelCacheNeedUpdate = make(map[int]struct{})
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

func requeueDirtyIDs(channelIDs []int, modelIDs []int, apiKeyIDs []int) {
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

	dailyCacheLock.Lock()
	if dailyCache.Date == todayDate {
		dailyCache.StatsMetrics.Add(metrics)
		dailyCacheLock.Unlock()
		return nil
	}

	prevDaily := dailyCache
	dailyCache = model.StatsDaily{Date: todayDate}
	dailyCache.StatsMetrics.Add(metrics)
	dailyCacheLock.Unlock()

	return saveDBWithDailyOverride(ctx, prevDaily)
}

// TotalUpdate adds metrics to the running total statistics.
func TotalUpdate(metrics model.StatsMetrics) error {
	totalCacheLock.Lock()
	defer totalCacheLock.Unlock()
	if totalCache.ID == 0 {
		totalCache.ID = 1
	}
	totalCache.StatsMetrics.Add(metrics)
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
	return nil
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

func buildModelID(channelID int, modelName string) int {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(fmt.Sprintf("%d:%s", channelID, strings.ToLower(strings.TrimSpace(modelName)))))
	return int(hash.Sum64() & 0x7fffffffffffffff)
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

// GetDaily retrieves all daily statistics records from the database.
func GetDaily(ctx context.Context) ([]model.StatsDaily, error) {
	var statsDaily []model.StatsDaily
	result := db.GetDB().WithContext(ctx).Find(&statsDaily)
	if result.Error != nil {
		return nil, result.Error
	}
	return statsDaily, nil
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

	hourlyCacheLock.Lock()
	hourlyCache = [24]model.StatsHourly{}
	hourlyCacheLock.Unlock()

	channelCache.Clear()
	channelCacheNeedUpdateLock.Lock()
	channelCacheNeedUpdate = make(map[int]struct{})
	channelCacheNeedUpdateLock.Unlock()

	modelCache.Clear()
	modelCacheNeedUpdateLock.Lock()
	modelCacheNeedUpdate = make(map[int]struct{})
	modelCacheNeedUpdateLock.Unlock()

	apiKeyCache.Clear()
	apiKeyCacheNeedUpdateLock.Lock()
	apiKeyCacheNeedUpdate = make(map[int]struct{})
	apiKeyCacheNeedUpdateLock.Unlock()
}

// ResetCachesForTest resets all stats caches to a known state for testing.
// channelID/modelID/apiKeyID set to 0 means skip that cache.
func ResetCachesForTest(total model.StatsTotal, daily model.StatsDaily, channelID, modelID, apiKeyID int) {
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

// GetModelDirtyIDs returns the set of model IDs marked as dirty.
func GetModelDirtyIDs() []int {
	modelCacheNeedUpdateLock.Lock()
	defer modelCacheNeedUpdateLock.Unlock()
	ids := make([]int, 0, len(modelCacheNeedUpdate))
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
