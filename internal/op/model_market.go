package op

import (
	"context"
	"slices"
	"strings"
	"time"

	"sync"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/modelnormalize"
	"github.com/lingyuins/octopus/internal/op/setting"
	"golang.org/x/sync/singleflight"
)

type modelMarketStatsAggregate struct {
	waitTime       int64
	requestSuccess int64
	requestFailed  int64
}

type modelMarketLLMAggregate struct {
	name          string
	price         model.LLMPrice
	hasPrice      bool
	hasExactPrice bool
}

// marketCache provides a short-lived TTL cache for the aggregated model market
// response. This avoids redundant recomputation when multiple clients request
// the data within a short window (e.g. concurrent polling).
var marketCache struct {
	mu        sync.RWMutex
	result    model.ModelMarketResponse
	expiresAt time.Time
}

const marketCacheTTL = 5 * time.Second

// marketSingleFlight 合并缓存 miss 时的并发重算：模型广场多个客户端/轮询
// 同时请求时，只让一个请求真正执行 DB 查询与聚合，其余等待共享结果，
// 避免在 SQLite 单连接 + 写任务竞争下并发放大延迟（网关 504 的候选成因）。
var marketSingleFlight singleflight.Group

// ModelMarketInvalidateCache forces the next ModelMarketGet call to recompute.
func ModelMarketInvalidateCache() {
	marketCache.mu.Lock()
	marketCache.expiresAt = time.Time{}
	marketCache.mu.Unlock()
}

func ModelMarketGet(ctx context.Context, lastUpdateTime time.Time) (model.ModelMarketResponse, error) {
	// Fast path: return cached result if still valid.
	marketCache.mu.RLock()
	if time.Now().Before(marketCache.expiresAt) {
		cached := marketCache.result
		marketCache.mu.RUnlock()
		return cached, nil
	}
	marketCache.mu.RUnlock()

	// 缓存 miss：合并并发重算，避免多请求各自全量执行（DB 查询 + 聚合）。
	result, err, _ := marketSingleFlight.Do("market", func() (any, error) {
		// singleflight 的执行结果由所有并发等待者共享，不能绑定首个进入者的
		// 请求 ctx——该客户端断开会让其余等待者一起收到 context.Canceled。
		sfCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		models, err := LLMList(sfCtx)
		if err != nil {
			return model.ModelMarketResponse{}, err
		}

		modelChannels, err := ChannelLLMList(sfCtx)
		if err != nil {
			return model.ModelMarketResponse{}, err
		}

		items, summary := buildModelMarket(models, modelChannels, channelCache.GetAll(), StatsModelList(), lastUpdateTime)
		resp := model.ModelMarketResponse{
			Summary: summary,
			Items:   items,
		}

		// Store in cache.
		marketCache.mu.Lock()
		marketCache.result = resp
		marketCache.expiresAt = time.Now().Add(marketCacheTTL)
		marketCache.mu.Unlock()

		return resp, nil
	})
	if err != nil {
		return model.ModelMarketResponse{}, err
	}
	return result.(model.ModelMarketResponse), nil
}

func buildModelMarket(
	models []model.LLMInfo,
	modelChannels []model.LLMChannel,
	channelsByID map[int]model.Channel,
	stats []model.StatsModel,
	lastUpdateTime time.Time,
) ([]model.ModelMarketItem, model.ModelMarketSummary) {
	statsByModelName := make(map[string]modelMarketStatsAggregate)
	for _, item := range stats {
		name := normalizeMarketModelName(item.Name)
		if name == "" {
			continue
		}
		aggregate := statsByModelName[name]
		aggregate.waitTime += item.WaitTime
		aggregate.requestSuccess += item.RequestSuccess
		aggregate.requestFailed += item.RequestFailed
		statsByModelName[name] = aggregate
	}

	channelsByModelName := make(map[string][]model.ModelMarketChannel)
	seenChannelsByModel := make(map[string]map[int]struct{})
	for _, item := range modelChannels {
		name := normalizeMarketModelName(item.Name)
		if name == "" {
			continue
		}
		if seenChannelsByModel[name] == nil {
			seenChannelsByModel[name] = make(map[int]struct{})
		}
		if _, exists := seenChannelsByModel[name][item.ChannelID]; exists {
			continue
		}
		seenChannelsByModel[name][item.ChannelID] = struct{}{}

		channel := channelsByID[item.ChannelID]
		enabledKeyCount := 0
		for _, key := range channel.Keys {
			if key.Enabled {
				enabledKeyCount++
			}
		}

		channelsByModelName[name] = append(channelsByModelName[name], model.ModelMarketChannel{
			ChannelID:       item.ChannelID,
			ChannelName:     item.ChannelName,
			Enabled:         item.Enabled,
			EnabledKeyCount: enabledKeyCount,
		})
	}

	llmByModelName := make(map[string]modelMarketLLMAggregate)
	modelNames := make([]string, 0, len(models))
	for _, llm := range models {
		name := normalizeMarketModelName(llm.Name)
		if name == "" {
			continue
		}
		aggregate, exists := llmByModelName[name]
		if !exists {
			aggregate.name = name
			modelNames = append(modelNames, name)
		}
		aggregate = chooseModelMarketPrice(aggregate, llm, name)
		llmByModelName[name] = aggregate
	}

	items := make([]model.ModelMarketItem, 0, len(modelNames))
	totalWaitTime := int64(0)
	totalRequests := int64(0)
	uniqueChannels := make(map[int]struct{})
	coverageCount := 0

	for _, name := range modelNames {
		llm := llmByModelName[name]
		modelItemChannels := channelsByModelName[name]
		if modelItemChannels == nil {
			modelItemChannels = make([]model.ModelMarketChannel, 0)
		}
		slices.SortFunc(modelItemChannels, func(a, b model.ModelMarketChannel) int {
			return strings.Compare(a.ChannelName, b.ChannelName)
		})

		enabledKeyCount := 0
		for _, channel := range modelItemChannels {
			enabledKeyCount += channel.EnabledKeyCount
			uniqueChannels[channel.ChannelID] = struct{}{}
		}

		statsAggregate := statsByModelName[name]
		requestCount := statsAggregate.requestSuccess + statsAggregate.requestFailed
		averageLatency := int64(0)
		successRate := 0.0
		if requestCount > 0 {
			averageLatency = statsAggregate.waitTime / requestCount
			successRate = float64(statsAggregate.requestSuccess) / float64(requestCount)
		}

		totalWaitTime += statsAggregate.waitTime
		totalRequests += requestCount
		coverageCount += len(modelItemChannels)

		items = append(items, model.ModelMarketItem{
			Name:             llm.name,
			Input:            llm.price.Input,
			Output:           llm.price.Output,
			CacheRead:        llm.price.CacheRead,
			CacheWrite:       llm.price.CacheWrite,
			ChannelCount:     len(modelItemChannels),
			EnabledKeyCount:  enabledKeyCount,
			AverageLatencyMS: averageLatency,
			SuccessRate:      successRate,
			RequestSuccess:   statsAggregate.requestSuccess,
			RequestFailed:    statsAggregate.requestFailed,
			Channels:         modelItemChannels,
		})
	}

	slices.SortFunc(items, func(a, b model.ModelMarketItem) int {
		leftRequests := a.RequestSuccess + a.RequestFailed
		rightRequests := b.RequestSuccess + b.RequestFailed

		switch {
		case leftRequests == 0 && rightRequests > 0:
			return 1
		case leftRequests > 0 && rightRequests == 0:
			return -1
		case leftRequests > 0 && rightRequests > 0:
			leftRatio := a.RequestSuccess * rightRequests
			rightRatio := b.RequestSuccess * leftRequests
			if leftRatio != rightRatio {
				if leftRatio > rightRatio {
					return -1
				}
				return 1
			}
		}

		if a.RequestSuccess != b.RequestSuccess {
			if a.RequestSuccess > b.RequestSuccess {
				return -1
			}
			return 1
		}
		if leftRequests != rightRequests {
			if leftRequests > rightRequests {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})

	summaryAverageLatency := int64(0)
	if totalRequests > 0 {
		summaryAverageLatency = totalWaitTime / totalRequests
	}

	return items, model.ModelMarketSummary{
		ModelCount:         len(items),
		CoverageCount:      coverageCount,
		UniqueChannelCount: len(uniqueChannels),
		AverageLatencyMS:   summaryAverageLatency,
		LastUpdateTime:     lastUpdateTime,
	}
}

func normalizeMarketModelName(name string) string {
	if enabled, err := setting.GetBool(model.SettingKeyModelNormalizeMarketDedupeDefault); err == nil && enabled {
		return modelnormalize.Normalize(name)
	}
	return strings.ToLower(strings.TrimSpace(name))
}

func chooseModelMarketPrice(aggregate modelMarketLLMAggregate, llm model.LLMInfo, canonical string) modelMarketLLMAggregate {
	price := llm.LLMPrice
	hasPrice := price.Input != 0 || price.Output != 0 || price.CacheRead != 0 || price.CacheWrite != 0
	isExactCanonical := strings.ToLower(strings.TrimSpace(llm.Name)) == canonical

	if !aggregate.hasPrice || (isExactCanonical && hasPrice && !aggregate.hasExactPrice) {
		aggregate.price = price
		aggregate.hasPrice = hasPrice
		aggregate.hasExactPrice = isExactCanonical && hasPrice
	}
	return aggregate
}
