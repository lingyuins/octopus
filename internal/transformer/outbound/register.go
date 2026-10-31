package outbound

import (
	"strings"

	"github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound/anthropic"
	"github.com/lingyuins/octopus/internal/transformer/outbound/cloudflare"
	"github.com/lingyuins/octopus/internal/transformer/outbound/codex"
	"github.com/lingyuins/octopus/internal/transformer/outbound/gemini"
	"github.com/lingyuins/octopus/internal/transformer/outbound/mimo"
	"github.com/lingyuins/octopus/internal/transformer/outbound/openai"
	"github.com/lingyuins/octopus/internal/transformer/outbound/passthrough"
	"github.com/lingyuins/octopus/internal/transformer/outbound/volcengine"
)

type OutboundType int

const (
	OutboundTypeOpenAIChat OutboundType = iota
	OutboundTypeOpenAIResponse
	OutboundTypeAnthropic
	OutboundTypeGemini
	OutboundTypeVolcengine
	OutboundTypeOpenAIEmbedding
	OutboundTypeMimo
	OutboundTypeCloudflare
	OutboundTypePassthrough
	OutboundTypeCodex
	// OutboundTypeRaw 原始穿透（信息体）：保留客户端原始请求体与请求路径，
	// 仅改写 model 字段。仅由分组出站格式 "raw" 触发，不是渠道类型。
	OutboundTypeRaw
)

func (t OutboundType) String() string {
	switch t {
	case OutboundTypeOpenAIChat:
		return "chat"
	case OutboundTypeOpenAIResponse:
		return "response"
	case OutboundTypeAnthropic:
		return "anthropic"
	case OutboundTypeGemini:
		return "gemini"
	case OutboundTypeVolcengine:
		return "volcengine"
	case OutboundTypeOpenAIEmbedding:
		return "embedding"
	case OutboundTypeMimo:
		return "mimo"
	case OutboundTypeCloudflare:
		return "cloudflare"
	case OutboundTypePassthrough:
		return "passthrough"
	case OutboundTypeCodex:
		return "codex"
	case OutboundTypeRaw:
		return "raw"
	default:
		return "unknown"
	}
}

// EmbeddingChannelTypes 定义支持 embedding 请求的 channel 类型集合
var EmbeddingChannelTypes = map[OutboundType]bool{
	OutboundTypeOpenAIEmbedding: true,
}

// ChatChannelTypes 定义支持 chat 请求的 channel 类型集合
var ChatChannelTypes = map[OutboundType]bool{
	OutboundTypeOpenAIChat:     true,
	OutboundTypeOpenAIResponse: true,
	OutboundTypeAnthropic:      true,
	OutboundTypeGemini:         true,
	OutboundTypeVolcengine:     true,
	OutboundTypeMimo:           true,
	OutboundTypeCloudflare:     true,
	OutboundTypeCodex:          true,
}

// IsEmbeddingChannelType 判断 channel 类型是否支持 embedding 请求
func IsEmbeddingChannelType(channelType OutboundType) bool {
	return EmbeddingChannelTypes[channelType]
}

// IsChatChannelType 判断 channel 类型是否支持 chat 请求
func IsChatChannelType(channelType OutboundType) bool {
	return ChatChannelTypes[channelType]
}

var outboundFactories = map[OutboundType]func() model.Outbound{
	OutboundTypeOpenAIChat:      func() model.Outbound { return &openai.ChatOutbound{} },
	OutboundTypeOpenAIResponse:  func() model.Outbound { return &openai.ResponseOutbound{} },
	OutboundTypeOpenAIEmbedding: func() model.Outbound { return &openai.EmbeddingOutbound{} },
	OutboundTypeAnthropic:       func() model.Outbound { return &anthropic.MessageOutbound{} },
	OutboundTypeGemini:          func() model.Outbound { return &gemini.MessagesOutbound{} },
	OutboundTypeVolcengine:      func() model.Outbound { return &volcengine.ResponseOutbound{} },
	OutboundTypeMimo:            func() model.Outbound { return &mimo.ChatOutbound{} },
	OutboundTypeCloudflare:      func() model.Outbound { return &cloudflare.ChatOutbound{} },
	OutboundTypePassthrough:     func() model.Outbound { return &passthrough.Outbound{} },
	OutboundTypeCodex:           func() model.Outbound { return &codex.Outbound{} },
	OutboundTypeRaw:             func() model.Outbound { return &passthrough.Outbound{PreservePath: true} },
}

func Get(outboundType OutboundType) model.Outbound {
	if factory, ok := outboundFactories[outboundType]; ok {
		return factory()
	}
	return nil
}

// IsLLMRequestFormat 判断请求是否为 LLM 对话格式（ChatCompletion / Response / Anthropic Message）。
// 从 relay 包提取以供 helper 包的探测逻辑复用，避免 helper 导入 relay 造成循环依赖。
func IsLLMRequestFormat(request *model.InternalLLMRequest) bool {
	if request == nil {
		return false
	}
	switch request.RawAPIFormat {
	case model.APIFormatOpenAIChatCompletion, model.APIFormatOpenAIResponse, model.APIFormatAnthropicMessage:
		return true
	default:
		return false
	}
}

// ResolveAttemptTypes 根据 channel type、请求格式和 group outbound format 决定出站 adapter
// 尝试顺序。从 relay 包提取以供 helper 探测逻辑复用。
//
// 对 OpenAIChat / OpenAIResponse 类型，提供可配置的 adapter 回退优先级：
//   - "passthrough": 原样转发 inbound JSON body，禁用 adapter 回退。
//   - "raw": 与 passthrough 相同，但额外保留客户端原始请求路径。
//   - 默认(auto): Anthropic 入站优先 Anthropic Messages，其他入站优先
//     Chat Completions；失败回退顺序为 Anthropic → Chat → Responses。
//   - "chat": 优先 Chat Completions，回退 Responses API（不识别入站格式）。
//   - "responses": 优先 Responses API，回退 Chat Completions。
//   - "messages": 优先 Anthropic Messages，回退 Chat → Responses。
//   - "chat_only" / "responses_only" / "messages_only": 禁用跨格式回退。
//
// 其它 channel type 直接返回 [channelType]。
func ResolveAttemptTypes(channelType OutboundType, request *model.InternalLLMRequest, outboundFormat string) []OutboundType {
	format := strings.ToLower(strings.TrimSpace(outboundFormat))
	if format == "passthrough" && request != nil {
		switch request.RawAPIFormat {
		case model.APIFormatOpenAIChatCompletion, model.APIFormatOpenAIResponse, model.APIFormatAnthropicMessage:
			return []OutboundType{OutboundTypePassthrough}
		}
	}
	if format == "raw" && request != nil {
		switch request.RawAPIFormat {
		case model.APIFormatOpenAIChatCompletion, model.APIFormatOpenAIResponse, model.APIFormatAnthropicMessage:
			return []OutboundType{OutboundTypeRaw}
		}
	}
	if request != nil && IsLLMRequestFormat(request) && (channelType == OutboundTypeOpenAIChat || channelType == OutboundTypeOpenAIResponse) {
		switch format {
		case "responses":
			return []OutboundType{OutboundTypeOpenAIResponse, OutboundTypeOpenAIChat}
		case "messages":
			return []OutboundType{OutboundTypeAnthropic, OutboundTypeOpenAIChat, OutboundTypeOpenAIResponse}
		case "chat_only":
			return []OutboundType{OutboundTypeOpenAIChat}
		case "responses_only":
			return []OutboundType{OutboundTypeOpenAIResponse}
		case "messages_only":
			return []OutboundType{OutboundTypeAnthropic}
		case "chat":
			return []OutboundType{OutboundTypeOpenAIChat, OutboundTypeOpenAIResponse}
		default: // auto：Anthropic 入站优先 Anthropic 出站，其余优先 Chat
			if request.RawAPIFormat == model.APIFormatAnthropicMessage {
				return []OutboundType{OutboundTypeAnthropic, OutboundTypeOpenAIChat, OutboundTypeOpenAIResponse}
			}
			return []OutboundType{OutboundTypeOpenAIChat, OutboundTypeOpenAIResponse}
		}
	}
	return []OutboundType{channelType}
}
