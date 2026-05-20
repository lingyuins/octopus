package llm

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/cache"
)

var modelCache = cache.New[string, model.LLMPrice](16)

// GetCache returns the internal model cache (for backward compatibility).
func GetCache() cache.Cache[string, model.LLMPrice] { return modelCache }

func List(ctx context.Context) ([]model.LLMInfo, error) {
	models := make([]model.LLMInfo, 0, modelCache.Len())
	for m, cost := range modelCache.GetAll() {
		models = append(models, model.LLMInfo{
			Name:     m,
			LLMPrice: cost,
		})
	}
	return models, nil
}

func ListWithStats(ctx context.Context, statsByName map[string]model.StatsMetrics) ([]model.LLMInfo, error) {
	models := make([]model.LLMInfo, 0, modelCache.Len())
	for m, cost := range modelCache.GetAll() {
		models = append(models, model.LLMInfo{
			Name:     m,
			LLMPrice: cost,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return compareRank(models[i].Name, models[j].Name, statsByName)
	})
	return models, nil
}

func compareRank(leftName string, rightName string, statsByName map[string]model.StatsMetrics) bool {
	leftStats := statsByName[leftName]
	rightStats := statsByName[rightName]

	leftTotal := leftStats.RequestSuccess + leftStats.RequestFailed
	rightTotal := rightStats.RequestSuccess + rightStats.RequestFailed

	switch {
	case leftTotal == 0 && rightTotal > 0:
		return false
	case leftTotal > 0 && rightTotal == 0:
		return true
	case leftTotal > 0 && rightTotal > 0:
		leftRatio := leftStats.RequestSuccess * rightTotal
		rightRatio := rightStats.RequestSuccess * leftTotal
		if leftRatio != rightRatio {
			return leftRatio > rightRatio
		}
	}

	if leftStats.RequestSuccess != rightStats.RequestSuccess {
		return leftStats.RequestSuccess > rightStats.RequestSuccess
	}
	if leftTotal != rightTotal {
		return leftTotal > rightTotal
	}
	return leftName < rightName
}

func Update(m model.LLMInfo, ctx context.Context) error {
	_, ok := modelCache.Get(m.Name)
	if !ok {
		return fmt.Errorf("model not found")
	}
	if err := db.GetDB().WithContext(ctx).Save(m).Error; err != nil {
		return err
	}
	modelCache.Set(m.Name, m.LLMPrice)
	return nil
}

func Delete(name string, ctx context.Context) error {
	_, ok := modelCache.Get(name)
	if !ok {
		return fmt.Errorf("model not found")
	}
	if err := db.GetDB().WithContext(ctx).Delete(&model.LLMInfo{Name: name}).Error; err != nil {
		return err
	}
	modelCache.Del(name)
	return nil
}

func BatchDelete(names []string, ctx context.Context) error {
	if len(names) == 0 {
		return nil
	}
	if err := db.GetDB().WithContext(ctx).Where("name IN ?", names).Delete(&model.LLMInfo{}).Error; err != nil {
		return err
	}
	modelCache.Del(names...)
	return nil
}

func Create(m model.LLMInfo, ctx context.Context) error {
	m.Name = strings.ToLower(m.Name)
	_, ok := modelCache.Get(m.Name)
	if ok {
		return fmt.Errorf("model already exists")
	}
	if err := db.GetDB().WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	modelCache.Set(m.Name, m.LLMPrice)
	return nil
}

func BatchCreate(infos []model.LLMInfo, ctx context.Context) error {
	if len(infos) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(infos))
	newInfos := make([]model.LLMInfo, 0, len(infos))
	for _, info := range infos {
		info.Name = strings.ToLower(info.Name)
		if _, ok := seen[info.Name]; ok {
			continue
		}
		if _, ok := modelCache.Get(info.Name); ok {
			continue
		}
		seen[info.Name] = struct{}{}
		newInfos = append(newInfos, info)
	}
	if len(newInfos) == 0 {
		return nil
	}
	if err := db.GetDB().WithContext(ctx).Create(&newInfos).Error; err != nil {
		return err
	}
	for _, info := range newInfos {
		modelCache.Set(info.Name, info.LLMPrice)
	}
	return nil
}

func BatchUpdate(infos []model.LLMInfo, ctx context.Context) error {
	if len(infos) == 0 {
		return nil
	}

	for _, info := range infos {
		info.Name = strings.ToLower(strings.TrimSpace(info.Name))
		if info.Name == "" {
			continue
		}
		if err := db.GetDB().WithContext(ctx).
			Model(&model.LLMInfo{}).
			Where("name = ?", info.Name).
			Updates(map[string]any{
				"input":       info.Input,
				"output":      info.Output,
				"cache_read":  info.CacheRead,
				"cache_write": info.CacheWrite,
			}).Error; err != nil {
			return err
		}
		modelCache.Set(info.Name, info.LLMPrice)
	}

	return nil
}

func Get(name string) (model.LLMPrice, error) {
	price, ok := modelCache.Get(name)
	if !ok {
		return model.LLMPrice{}, fmt.Errorf("model not found")
	}
	return price, nil
}

func RefreshCache(ctx context.Context) error {
	models := []model.LLMInfo{}
	if err := db.GetDB().WithContext(ctx).Find(&models).Error; err != nil {
		return err
	}
	for _, m := range models {
		modelCache.Set(m.Name, m.LLMPrice)
	}
	return nil
}
