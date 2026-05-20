package stats

import (
	"context"
	"errors"
	"fmt"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

// RefreshCache loads all statistics from the database into the in-memory caches.
func RefreshCache(ctx context.Context) error {
	dbConn := db.GetDB().WithContext(ctx)
	todayDate := today()

	var loadedDaily model.StatsDaily
	result := dbConn.Last(&loadedDaily)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to get daily stats: %v", result.Error)
	}
	if result.RowsAffected == 0 || loadedDaily.Date != todayDate {
		loadedDaily = model.StatsDaily{Date: todayDate}
	}

	var loadedTotal model.StatsTotal
	result = dbConn.First(&loadedTotal)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to get total stats: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		loadedTotal = model.StatsTotal{ID: 1}
	} else if loadedTotal.ID == 0 {
		loadedTotal.ID = 1
	}

	var loadedChannels []model.StatsChannel
	result = dbConn.Find(&loadedChannels)
	if result.Error != nil {
		return fmt.Errorf("failed to get channels: %v", result.Error)
	}

	var loadedModels []model.StatsModel
	result = dbConn.Find(&loadedModels)
	if result.Error != nil {
		return fmt.Errorf("failed to get model stats: %v", result.Error)
	}

	var loadedHourly []model.StatsHourly
	result = dbConn.Where("date = ?", todayDate).Find(&loadedHourly)
	if result.Error != nil {
		return fmt.Errorf("failed to get hourly stats: %v", result.Error)
	}

	dailyCacheLock.Lock()
	dailyCache = loadedDaily
	dailyCacheLock.Unlock()

	totalCacheLock.Lock()
	totalCache = loadedTotal
	totalCacheLock.Unlock()

	channelCache.Clear()
	channelCacheNeedUpdateLock.Lock()
	channelCacheNeedUpdate = make(map[int]struct{})
	channelCacheNeedUpdateLock.Unlock()
	for _, v := range loadedChannels {
		channelCache.Set(v.ChannelID, v)
	}

	modelCache.Clear()
	modelCacheNeedUpdateLock.Lock()
	modelCacheNeedUpdate = make(map[int]struct{})
	modelCacheNeedUpdateLock.Unlock()
	for _, v := range loadedModels {
		modelCache.Set(v.ID, v)
	}

	var loadedAPIKeys []model.StatsAPIKey
	result = dbConn.Find(&loadedAPIKeys)
	if result.Error != nil {
		return fmt.Errorf("failed to get api key stats: %v", result.Error)
	}

	apiKeyCache.Clear()
	apiKeyCacheNeedUpdateLock.Lock()
	apiKeyCacheNeedUpdate = make(map[int]struct{})
	apiKeyCacheNeedUpdateLock.Unlock()
	for _, v := range loadedAPIKeys {
		apiKeyCache.Set(v.APIKeyID, v)
	}

	hourlyCacheLock.Lock()
	hourlyCache = [24]model.StatsHourly{}
	for _, v := range loadedHourly {
		if v.Hour >= 0 && v.Hour < 24 {
			hourlyCache[v.Hour] = v
		}
	}
	hourlyCacheLock.Unlock()

	return nil
}

