package price

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/client"
	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/llm"
	"github.com/lingyuins/octopus/internal/utils/log"
)

const maxPriceResponseBytes = 10 << 20 // 10 MiB — models.dev API response is typically < 2 MiB

func getLLMPriceURL() string {
	if u := conf.AppConfig.External.LLMPriceURL; u != "" {
		return u
	}
	return "https://models.dev/api.json"
}

var Provider = []string{
	"openai",     // GPT 系列
	"anthropic",  // Claude 系列
	"google",     // Gemini 系列
	"deepseek",   // DeepSeek 系列
	"xai",        // Grok 系列
	"alibaba",    // Qwen 系列
	"zhipuai",    // GLM 系列
	"minimax",    // MiniMax 系列
	"moonshotai", // Kimi/Moonshot
	"v0",         // v0 系列
}

var (
	lastUpdateTime   time.Time
	lastUpdateTimeMu sync.RWMutex
)

func UpdateLLMPrice(ctx context.Context) error {
	log.Debugf("update LLM price task started")
	startTime := time.Now()
	defer func() {
		log.Debugf("update LLM price task finished, update time: %s", time.Since(startTime))
	}()
	client, err := client.GetHTTPClientSystemProxy(false)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getLLMPriceURL(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch LLM info: %s", resp.Status)
	}
	var rawPrice map[string]struct {
		Models map[string]struct {
			ID   string         `json:"id"`
			Cost model.LLMPrice `json:"cost"`
		} `json:"models"`
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPriceResponseBytes+1))
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	if len(body) > maxPriceResponseBytes {
		return fmt.Errorf("price response exceeds %d bytes limit; upstream may be misbehaving", maxPriceResponseBytes)
	}
	if err := json.Unmarshal(body, &rawPrice); err != nil {
		return fmt.Errorf("failed to parse LLM info: %w", err)
	}
	llmPriceLock.Lock()
	for _, provider := range Provider {
		for _, model := range rawPrice[provider].Models {
			model.ID = strings.ToLower(model.ID)
			llmPrice[model.ID] = model.Cost
		}
	}
	llmPriceLock.Unlock()
	lastUpdateTimeMu.Lock()
	lastUpdateTime = time.Now()
	lastUpdateTimeMu.Unlock()
	return nil
}

func GetLastUpdateTime() time.Time {
	lastUpdateTimeMu.RLock()
	defer lastUpdateTimeMu.RUnlock()
	return lastUpdateTime
}

func GetLLMPrice(modelName string) *model.LLMPrice {
	modelName = strings.ToLower(modelName)
	price, err := llm.Get(modelName)
	// 渠道同步（LLMPriceAddToDB）会给未知价模型插入四价全 0 的行，DB 命中
	// 不能无条件短路，否则分类兜底恰好对它设计要覆盖的"同步过但没匹配到
	// 价格"的模型不生效。0 价行继续走兜底链。
	if err == nil && !isZeroPrice(price) {
		return &price
	}
	llmPriceLock.RLock()
	defer llmPriceLock.RUnlock()
	if p, ok := llmPrice[modelName]; ok {
		return &p
	}
	// 分类表兜底：按规则匹配的模型价格分类（优先级高于整词子串兜底）。
	if cat := llm.PriceCategoryMatch(modelName); cat != nil {
		return cat
	}
	// Fallback: try matching by base model name
	if fallback := matchFallbackPrice(modelName); fallback != nil {
		return fallback
	}
	if err == nil {
		// DB 有 0 价行且所有兜底都未命中：维持原语义，返回 DB 行（0 价）。
		return &price
	}
	return nil
}

// isZeroPrice 报告四项价格是否全为 0（渠道同步为未知价模型写入的占位行）。
func isZeroPrice(p model.LLMPrice) bool {
	return p.Input == 0 && p.Output == 0 && p.CacheRead == 0 && p.CacheWrite == 0
}

// GetLLMPriceFromUpstream 仅从同步价格源（外部价格文件 + presets.go +
// presets_manual.go 托底价格）中查找价格，不查询数据库。
//
// 用于"同步价格"刷新已有模型价格的场景：外部同步文件优先，未命中时回落到
// 内置/手工托底价格（presets_manual.go），仍未命中返回 nil（调用方决定写 0）。
// 与 GetLLMPrice 的区别在于跳过 DB 查询层，避免 DB 中的旧值挡住新同步的价格。
func GetLLMPriceFromUpstream(modelName string) *model.LLMPrice {
	modelName = strings.ToLower(modelName)
	llmPriceLock.RLock()
	defer llmPriceLock.RUnlock()
	price, ok := llmPrice[modelName]
	if ok {
		return &price
	}
	// Fallback: try matching by base model name
	if fallback := matchFallbackPrice(modelName); fallback != nil {
		return fallback
	}
	return nil
}

// matchFallbackPrice attempts to find a price for modelName using two heuristics:
//  1. Strip "provider/" prefix (e.g. "openai/gpt-4o" -> "gpt-4o")
//  2. Find the longest known model name that appears as a whole-word substring
//     of modelName — delimited by non-alphanumeric characters (or the string
//     bounds) on BOTH sides — so variants like "xgpt-4o" or "gpt-4ox" do not
//     steal the price of "gpt-4o".
//
// Must be called with llmPriceLock held (RLock).
func matchFallbackPrice(modelName string) *model.LLMPrice {
	// 1. Strip prefix before first '/'
	if idx := strings.Index(modelName, "/"); idx >= 0 && idx < len(modelName)-1 {
		base := modelName[idx+1:]
		if p, ok := llmPrice[base]; ok {
			return &p
		}
	}

	// 2. Longest whole-word substring match.
	var bestKey string
	for known := range llmPrice {
		if len(known) < 3 {
			continue // skip very short names to avoid false positives
		}
		if !containsWholeWord(modelName, known) {
			continue
		}
		if len(known) > len(bestKey) {
			bestKey = known
		}
	}
	if bestKey != "" {
		p := llmPrice[bestKey]
		return &p
	}
	return nil
}

// containsWholeWord reports whether substr occurs in s at least once with
// non-alphanumeric characters (or the string bounds) on both sides, i.e. as a
// delimited token rather than a fragment of a larger identifier. It scans every
// occurrence so a later valid boundary can rescue an earlier invalid one.
func containsWholeWord(s, substr string) bool {
	searchFrom := 0
	for searchFrom <= len(s)-len(substr) {
		idx := strings.Index(s[searchFrom:], substr)
		if idx < 0 {
			return false
		}
		pos := searchFrom + idx
		end := pos + len(substr)
		if !isAlphaNumAt(s, pos-1) && !isAlphaNumAt(s, end) {
			return true
		}
		searchFrom = pos + 1
	}
	return false
}

// isAlphaNumAt reports whether the byte at index i is an ASCII letter or digit.
// Out-of-range indices return false, treating the string bounds as a word boundary.
func isAlphaNumAt(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}
	c := s[i]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
