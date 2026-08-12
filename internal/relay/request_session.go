package relay

import (
	"strconv"
	"strings"

	"github.com/cespare/xxhash/v2"
	"github.com/gin-gonic/gin"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
)

func populateRelayRequestSessionFields(c *gin.Context, req *transmodel.InternalLLMRequest, body []byte) {
	if req == nil {
		return
	}

	req.RawRequest = append([]byte(nil), body...)
	req.ConversationID = strings.TrimSpace(c.GetHeader("X-Conversation-ID"))
	req.ResumeFromEventID = parseRelayEventSequence(c.GetHeader("Last-Event-ID"))

	if req.ConversationID == "" {
		req.ConversationID = strings.TrimSpace(c.Query("conversation_id"))
	}
	if req.ResumeFromEventID == 0 {
		req.ResumeFromEventID = parseRelayEventSequence(c.Query("last_event_id"))
	}
	if req.ResumeFromEventID == 0 {
		req.ResumeFromEventID = parseRelayEventSequence(c.Query("resume_from_sequence"))
	}

	var raw map[string]RawMessage
	if err := jsonAPI.Unmarshal(body, &raw); err != nil {
		return
	}

	if req.ConversationID == "" {
		req.ConversationID = parseRelayRawStringField(raw, "conversation_id")
	}
	if req.ResumeFromEventID == 0 {
		req.ResumeFromEventID = parseRelayRawIntField(raw, "last_event_id")
	}
	if req.ResumeFromEventID == 0 {
		req.ResumeFromEventID = parseRelayRawIntField(raw, "resume_from_sequence")
	}

	// 会话 ID 来自客户端（头/query/body），会被写回响应头并作为会话 map key：
	// 限长并拒绝控制字符，防响应头注入与超长 key 内存放大。
	req.ConversationID = sanitizeRelayConversationID(req.ConversationID)
}

// maxRelayConversationIDLen 是会话 ID 的最大长度（runes）。
const maxRelayConversationIDLen = 128

// sanitizeRelayConversationID 清理会话 ID：TrimSpace、拒绝控制字符（CR/LF 等）、
// 超长返回空串。返回空串时该请求不启用 stream session。
func sanitizeRelayConversationID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.IndexFunc(raw, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return ""
	}
	if len([]rune(raw)) > maxRelayConversationIDLen {
		return ""
	}
	return raw
}

func parseRelayRawStringField(raw map[string]RawMessage, field string) string {
	value, ok := raw[field]
	if !ok || len(value) == 0 {
		return ""
	}

	var s string
	if err := jsonAPI.Unmarshal(value, &s); err == nil {
		return strings.TrimSpace(s)
	}
	return ""
}

func parseRelayRawIntField(raw map[string]RawMessage, field string) int64 {
	value, ok := raw[field]
	if !ok || len(value) == 0 {
		return 0
	}

	var n int64
	if err := jsonAPI.Unmarshal(value, &n); err == nil && n > 0 {
		return n
	}

	var s string
	if err := jsonAPI.Unmarshal(value, &s); err == nil {
		return parseRelayEventSequence(s)
	}
	return 0
}

func parseRelayEventSequence(value string) int64 {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}

	n, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func shouldUseRelayStreamSession(req *transmodel.InternalLLMRequest) bool {
	if req == nil || req.Stream == nil || !*req.Stream {
		return false
	}
	return strings.TrimSpace(req.ConversationID) != ""
}

func resolveRelayStreamSessionIdentity(endpointType string, inboundType int, apiKeyID int, req *transmodel.InternalLLMRequest) (string, uint64, bool) {
	if !shouldUseRelayStreamSession(req) {
		return "", 0, false
	}

	requestHash := buildRelayStreamSessionHash(endpointType, inboundType, apiKeyID, req.RawRequest)
	if requestHash == 0 {
		return "", 0, false
	}

	conversationID := strings.TrimSpace(req.ConversationID)
	return conversationID, requestHash, true
}

func buildRelayStreamSessionHash(endpointType string, inboundType int, apiKeyID int, rawRequest []byte) uint64 {
	normalizedRequest := normalizeRelayRequestHashBody(rawRequest)
	hasher := xxhash.New()
	_, _ = hasher.WriteString(strings.TrimSpace(endpointType))
	_, _ = hasher.WriteString("\n")
	_, _ = hasher.WriteString(strconv.Itoa(inboundType))
	_, _ = hasher.WriteString("\n")
	_, _ = hasher.WriteString(strconv.Itoa(apiKeyID))
	_, _ = hasher.WriteString("\n")
	_, _ = hasher.Write(normalizedRequest)
	return hasher.Sum64()
}

func normalizeRelayRequestHashBody(rawRequest []byte) []byte {
	if len(rawRequest) == 0 {
		return nil
	}

	var payload any
	if err := jsonAPI.Unmarshal(rawRequest, &payload); err != nil {
		return rawRequest
	}

	root, ok := payload.(map[string]any)
	if !ok {
		return rawRequest
	}

	delete(root, "conversation_id")
	delete(root, "last_event_id")
	delete(root, "resume_from_sequence")

	normalized, err := jsonAPI.Marshal(root)
	if err != nil {
		return rawRequest
	}
	return normalized
}
