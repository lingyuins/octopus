package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/apikey"
	"github.com/lingyuins/octopus/internal/op/cacheusage"
	"github.com/lingyuins/octopus/internal/op/relaylog"
	"github.com/lingyuins/octopus/internal/op/stats"
	"github.com/lingyuins/octopus/internal/price"
	transformerModel "github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/lingyuins/octopus/internal/utils/telemetry"
)

const relayLogTextFieldMaxBytes = 4096

const relayLogJSONFieldMaxBytes = 16384

// RelayMetrics 负责最终的日志收集与持久化
type RelayMetrics struct {
	APIKeyID     int
	RequestModel string
	EndpointType string
	ClientIP     string
	StartTime    time.Time

	// 首 Token 时间
	FirstTokenTime time.Time

	// 请求和响应内容
	InternalRequest  *transformerModel.InternalLLMRequest
	InternalResponse *transformerModel.InternalLLMResponse

	// 统计指标
	ActualModel string
	Stats       model.StatsMetrics
}

func NewRelayMetrics(apiKeyID int, requestModel string, requestedEndpointType string, matchedGroupEndpointType string, clientIP string, req *transformerModel.InternalLLMRequest) *RelayMetrics {
	return &RelayMetrics{
		APIKeyID:        apiKeyID,
		RequestModel:    requestModel,
		EndpointType:    resolveRelayLogEndpointType(requestedEndpointType, matchedGroupEndpointType),
		ClientIP:        clientIP,
		StartTime:       time.Now(),
		InternalRequest: req,
	}
}

func (m *RelayMetrics) SetFirstTokenTime(t time.Time) {
	m.FirstTokenTime = t
}

func (m *RelayMetrics) SetInternalResponse(resp *transformerModel.InternalLLMResponse, actualModel string) {
	m.InternalResponse = resp
	m.ActualModel = actualModel

	if resp == nil || resp.Usage == nil {
		return
	}

	usage := resp.Usage
	m.Stats.InputToken = usage.PromptTokens
	m.Stats.OutputToken = usage.CompletionTokens

	modelPrice := price.GetLLMPrice(actualModel)
	if modelPrice == nil {
		return
	}
	if usage.PromptTokensDetails == nil {
		usage.PromptTokensDetails = &transformerModel.PromptTokensDetails{
			CachedTokens: 0,
		}
	}
	if usage.AnthropicUsage {
		m.Stats.InputCost = (float64(usage.PromptTokensDetails.CachedTokens)*modelPrice.CacheRead +
			float64(usage.PromptTokens)*modelPrice.Input +
			float64(usage.CacheCreationInputTokens)*modelPrice.CacheWrite) * 1e-6
	} else {
		m.Stats.InputCost = (float64(usage.PromptTokensDetails.CachedTokens)*modelPrice.CacheRead + float64(usage.PromptTokens-usage.PromptTokensDetails.CachedTokens)*modelPrice.Input) * 1e-6
	}
	m.Stats.OutputCost = float64(usage.CompletionTokens) * modelPrice.Output * 1e-6
}

func (m *RelayMetrics) Save(success bool, err error, attempts []model.ChannelAttempt) {
	ctx, cancel := newRelayPersistenceContext()
	defer cancel()

	duration := time.Since(m.StartTime)
	totalAttempts := len(attempts)
	forwardedAttempts := countForwardedAttempts(attempts)

	useTimeMs := duration.Milliseconds()

	globalStats := model.StatsMetrics{
		WaitTime:    useTimeMs,
		InputToken:  m.Stats.InputToken,
		OutputToken: m.Stats.OutputToken,
		InputCost:   m.Stats.InputCost,
		OutputCost:  m.Stats.OutputCost,
	}
	if success {
		globalStats.RequestSuccess = 1
	} else {
		globalStats.RequestFailed = 1
	}

	// Latency histogram bucket assignment
	switch {
	case useTimeMs < 100:
		globalStats.HistogramLt100 = 1
	case useTimeMs < 500:
		globalStats.Histogram100to500 = 1
	case useTimeMs < 1000:
		globalStats.Histogram500to1k = 1
	case useTimeMs < 5000:
		globalStats.Histogram1kto5k = 1
	default:
		globalStats.HistogramGt5k = 1
	}

	// FTUT: first token time
	if !m.FirstTokenTime.IsZero() {
		ftutMs := m.FirstTokenTime.Sub(m.StartTime).Milliseconds()
		globalStats.FtutAvg = ftutMs
		globalStats.FtutP50 = ftutMs
		globalStats.FtutP95 = ftutMs
		globalStats.FtutP99 = ftutMs
	}

	// Latency percentiles from telemetry ring buffer (approximate)
	snap := telemetry.Global().Snapshot()
	globalStats.LatencyP50 = int64(snap.AvgLatencyMs)
	globalStats.LatencyP95 = int64(snap.P95LatencyMs)
	globalStats.LatencyP99 = int64(snap.P99LatencyMs)

	channelID, channelName := finalChannel(attempts)
	stats.TotalUpdate(globalStats)
	stats.HourlyUpdate(globalStats)
	if statsErr := stats.DailyUpdate(ctx, globalStats); statsErr != nil {
		log.Warnf("failed to update daily stats: %v", statsErr)
	}
	stats.APIKeyUpdate(m.APIKeyID, globalStats)

	log.Infof("relay complete: model=%s, channel=%d(%s), success=%t, duration=%dms, input_token=%d, output_token=%d, input_cost=%f, output_cost=%f, total_cost=%f, attempts=%d, forwarded_attempts=%d",
		m.RequestModel, channelID, channelName, success, duration.Milliseconds(),
		m.Stats.InputToken, m.Stats.OutputToken,
		m.Stats.InputCost, m.Stats.OutputCost, m.Stats.InputCost+m.Stats.OutputCost,
		totalAttempts, forwardedAttempts)

	m.saveLog(ctx, err, duration, attempts, channelID, channelName)
	telemetry.Global().RecordRequest(duration.Milliseconds(), success)
}

func finalChannel(attempts []model.ChannelAttempt) (int, string) {
	var fallbackID int
	var fallbackName string
	var lastID int
	var lastName string
	for i := len(attempts) - 1; i >= 0; i-- {
		a := attempts[i]
		if fallbackID == 0 && a.ChannelID != 0 {
			fallbackID = a.ChannelID
			fallbackName = a.ChannelName
		}
		if a.Status == model.AttemptSuccess {
			return a.ChannelID, a.ChannelName
		}
		if a.Status == model.AttemptFailed && lastID == 0 {
			lastID = a.ChannelID
			lastName = a.ChannelName
		}
	}
	if lastID != 0 {
		return lastID, lastName
	}
	return fallbackID, fallbackName
}

func countForwardedAttempts(attempts []model.ChannelAttempt) int {
	count := 0
	for _, attempt := range attempts {
		if attempt.Status == model.AttemptSkipped || attempt.Status == model.AttemptCircuitBreak {
			continue
		}
		count++
	}
	return count
}

func (m *RelayMetrics) saveLog(ctx context.Context, err error, duration time.Duration, attempts []model.ChannelAttempt, channelID int, channelName string) {
	actualModel := m.ActualModel
	if actualModel == "" {
		actualModel = m.RequestModel
	}

	relayLog := model.RelayLog{
		Time:             m.StartTime.Unix(),
		RequestModelName: m.RequestModel,
		RequestAPIKeyID:  m.APIKeyID,
		ClientIP:         m.ClientIP,
		EndpointType:     m.EndpointType,
		ChannelName:      channelName,
		ChannelId:        channelID,
		ActualModelName:  actualModel,
		UseTime:          int(duration.Milliseconds()),
		Attempts:         attempts,
		TotalAttempts:    len(attempts),
	}

	if apiKey, getErr := apikey.Get(m.APIKeyID, ctx); getErr == nil {
		relayLog.RequestAPIKeyName = apiKey.Name
	}

	// 首字时间
	if !m.FirstTokenTime.IsZero() {
		relayLog.Ftut = int(m.FirstTokenTime.Sub(m.StartTime).Milliseconds())
	}

	// Usage
	if m.InternalResponse != nil && m.InternalResponse.Usage != nil {
		relayLog.InputTokens = int(m.InternalResponse.Usage.PromptTokens)
		relayLog.OutputTokens = int(m.InternalResponse.Usage.CompletionTokens)
		relayLog.Cost = m.Stats.InputCost + m.Stats.OutputCost
	}

	// 请求内容
	if m.InternalRequest != nil {
		if reqJSON, jsonErr := json.Marshal(m.filterRequestForLog(m.InternalRequest)); jsonErr == nil {
			relayLog.RequestContent = string(reqJSON)
		}
	}

	// 响应内容
	if m.InternalResponse != nil {
		respForLog := m.filterResponseForLog(m.InternalResponse)
		if respJSON, jsonErr := json.Marshal(respForLog); jsonErr == nil {
			if m.InternalResponse.Usage != nil && m.InternalResponse.Usage.AnthropicUsage {
				respStr := string(respJSON)
				old := `"usage":{`
				insert := fmt.Sprintf(`"usage":{"cache_creation_input_tokens":%d,`, m.InternalResponse.Usage.CacheCreationInputTokens)
				respJSON = []byte(strings.Replace(respStr, old, insert, 1))
			}
			if isSemanticCacheHitRequest(m.InternalRequest) {
				relayLog.SemanticCacheHit = true
				if relayLog.ChannelName == "" {
					relayLog.ChannelName = "Semantic Cache"
				}
				respJSON = semanticCacheHitPayload(respJSON, m.InternalRequest)
			}
			relayLog.ResponseContent = string(respJSON)
		}
	}

	if !relayLog.SemanticCacheHit {
		relayLog.CacheReadTokens = opRelayLogCacheReadTokens(relayLog.ResponseContent)
	}

	// 错误信息
	if err != nil {
		relayLog.Error = err.Error()
	}

	if logErr := relaylog.RelayLogAdd(ctx, relayLog); logErr != nil {
		log.Warnf("failed to save relay log: %v", logErr)
	}

	// 把每次尝试（含失败）落表，使失败渠道可按 channel_id 检索（issue #67）。
	// relayLog.ID 已由 RelayLogAdd 分配。
	if attemptsErr := relaylog.RelayLogAttemptsAdd(ctx, relayLog.ID, attempts, relayLog.Time); attemptsErr != nil {
		log.Warnf("failed to save relay log attempts: %v", attemptsErr)
	}
}

func opRelayLogCacheReadTokens(responseContent string) int {
	signals, ok := cacheusage.ParseProviderPromptCacheUsageSignals(responseContent)
	if !ok || signals.SemanticCacheHit || signals.CachedTokens <= 0 {
		return 0
	}
	return int(signals.CachedTokens)
}

func filterMessageForLog(msg *transformerModel.Message) *transformerModel.Message {
	if msg == nil {
		return nil
	}
	c := *msg
	if c.Content.Content != nil {
		content := truncateRelayLogString(*c.Content.Content, relayLogTextFieldMaxBytes)
		c.Content.Content = &content
	}
	if c.ReasoningContent != nil {
		reasoningContent := truncateRelayLogString(*c.ReasoningContent, relayLogTextFieldMaxBytes)
		c.ReasoningContent = &reasoningContent
	}
	if c.Reasoning != nil {
		reasoning := truncateRelayLogString(*c.Reasoning, relayLogTextFieldMaxBytes)
		c.Reasoning = &reasoning
	}
	if len(c.ToolCalls) > 0 {
		c.ToolCalls = make([]transformerModel.ToolCall, len(msg.ToolCalls))
		for i, toolCall := range msg.ToolCalls {
			c.ToolCalls[i] = toolCall
			c.ToolCalls[i].Function.Arguments = truncateRelayLogString(toolCall.Function.Arguments, relayLogTextFieldMaxBytes)
		}
	}
	c.Images = nil
	if len(c.Content.MultipleContent) > 0 {
		parts := make([]transformerModel.MessageContentPart, 0, len(c.Content.MultipleContent))
		for _, p := range c.Content.MultipleContent {
			switch {
			case p.Type == "text" && p.Text != nil:
				text := truncateRelayLogString(*p.Text, relayLogTextFieldMaxBytes)
				parts = append(parts, transformerModel.MessageContentPart{
					Type: p.Type,
					Text: &text,
				})
			case p.Type == "image_url" && p.ImageURL != nil:
				parts = append(parts, transformerModel.MessageContentPart{
					Type:     "image_url",
					ImageURL: &transformerModel.ImageURL{URL: "[image data omitted for storage]", Detail: p.ImageURL.Detail},
				})
			case p.Type == "input_audio" && p.Audio != nil:
				audio := *p.Audio
				audio.Data = "[audio data omitted for storage]"
				parts = append(parts, transformerModel.MessageContentPart{
					Type:  p.Type,
					Audio: &audio,
				})
			case p.Type == "file" && p.File != nil && p.File.FileData != "":
				file := *p.File
				file.FileData = "[file data omitted for storage]"
				parts = append(parts, transformerModel.MessageContentPart{
					Type: p.Type,
					File: &file,
				})
			default:
				parts = append(parts, p)
			}
		}
		c.Content = transformerModel.MessageContent{Content: c.Content.Content, MultipleContent: parts}
	}
	if c.Audio != nil && c.Audio.Data != "" {
		a := *c.Audio
		a.Data = "[audio data omitted for storage]"
		c.Audio = &a
	}
	return &c
}

func truncateRelayLogString(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}

	truncated := value[:maxBytes]
	for !utf8.ValidString(truncated) && len(truncated) > 0 {
		truncated = truncated[:len(truncated)-1]
	}
	return fmt.Sprintf("%s...[truncated %d bytes for storage]", truncated, len(value)-len(truncated))
}

func filterEmbeddingInputForLog(input *transformerModel.EmbeddingInput) *transformerModel.EmbeddingInput {
	if input == nil {
		return nil
	}
	cloned := *input
	if len(input.Multiple) > 0 {
		cloned.Multiple = make([]string, len(input.Multiple))
		copy(cloned.Multiple, input.Multiple)
	}
	for i, value := range cloned.Multiple {
		cloned.Multiple[i] = truncateRelayLogString(value, relayLogTextFieldMaxBytes)
	}
	if cloned.Single != nil {
		truncated := truncateRelayLogString(*cloned.Single, relayLogTextFieldMaxBytes)
		cloned.Single = &truncated
	}
	return &cloned
}

func filterToolsForLog(tools []transformerModel.Tool) []transformerModel.Tool {
	if len(tools) == 0 {
		return nil
	}
	filtered := make([]transformerModel.Tool, len(tools))
	for i, tool := range tools {
		filtered[i] = tool
		filtered[i].Function.Description = truncateRelayLogString(tool.Function.Description, relayLogTextFieldMaxBytes)
		if len(tool.Function.Parameters) > relayLogJSONFieldMaxBytes {
			filtered[i].Function.Parameters = json.RawMessage(strconv.Quote(truncateRelayLogString(string(tool.Function.Parameters), relayLogJSONFieldMaxBytes)))
		}
	}
	return filtered
}

func (m *RelayMetrics) filterRequestForLog(req *transformerModel.InternalLLMRequest) *transformerModel.InternalLLMRequest {
	if req == nil {
		return nil
	}

	filtered := *req
	if len(req.Messages) > 0 {
		filtered.Messages = make([]transformerModel.Message, len(req.Messages))
		for i, msg := range req.Messages {
			filteredMsg := filterMessageForLog(&msg)
			if filteredMsg != nil {
				filtered.Messages[i] = *filteredMsg
			}
		}
	}
	filtered.EmbeddingInput = filterEmbeddingInputForLog(req.EmbeddingInput)
	filtered.Tools = filterToolsForLog(req.Tools)
	filtered.ExtraBody = nil
	filtered.RawRequest = nil
	return &filtered
}

// filterResponseForLog 创建响应的浅拷贝，过滤掉 images、MultipleContent 中的图片数据和 Audio.Data 以减少存储压力
func (m *RelayMetrics) filterResponseForLog(resp *transformerModel.InternalLLMResponse) *transformerModel.InternalLLMResponse {
	if resp == nil {
		return nil
	}

	filtered := *resp
	filtered.Choices = make([]transformerModel.Choice, len(resp.Choices))
	for i, choice := range resp.Choices {
		filtered.Choices[i] = choice
		filtered.Choices[i].Message = filterMessageForLog(choice.Message)
		filtered.Choices[i].Delta = filterMessageForLog(choice.Delta)
	}
	return &filtered
}
