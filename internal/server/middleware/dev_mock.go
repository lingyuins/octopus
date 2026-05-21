package middleware

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/utils/log"
)

const (
	devMockText          = "dev mock success"
	maxDevMockBodyBytes  = 64 << 10 // 64 KiB
)

type devMockRequestInfo struct {
	Model       string
	Stream      bool
	ContentType string
	BodyPreview string
}

func DevMockPublicSuccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !conf.IsDevMockSuccess() {
			c.Next()
			return
		}

		info := readDevMockRequestInfo(c)
		log.Infof("dev mock success request: method=%s path=%s content_type=%q model=%q stream=%t preview=%q",
			c.Request.Method, c.Request.URL.Path, info.ContentType, info.Model, info.Stream, info.BodyPreview)

		writeDevMockResponse(c, info)
		c.Abort()
	}
}

func readDevMockRequestInfo(c *gin.Context) devMockRequestInfo {
	info := devMockRequestInfo{
		ContentType: c.ContentType(),
	}

	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := c.Request.ParseMultipartForm(32 << 20); err == nil {
			info.Model = strings.TrimSpace(c.Request.FormValue("model"))
			info.Stream = strings.EqualFold(strings.TrimSpace(c.Request.FormValue("stream")), "true")
			info.BodyPreview = "multipart"
		}
		return info
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxDevMockBodyBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Request.Body = io.NopCloser(strings.NewReader(""))
		return info
	}
	c.Request.Body = io.NopCloser(strings.NewReader(string(body)))
	info.BodyPreview = trimDevMockPreview(string(body))

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return info
	}
	if modelValue, ok := raw["model"].(string); ok {
		info.Model = strings.TrimSpace(modelValue)
	}
	info.Stream = parseDevMockStreamFlag(raw["stream"])
	return info
}

func parseDevMockStreamFlag(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func trimDevMockPreview(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) <= 160 {
		return raw
	}
	return raw[:160]
}

func writeDevMockResponse(c *gin.Context, info devMockRequestInfo) {
	path := c.Request.URL.Path
	modelName := info.Model
	if modelName == "" {
		modelName = "dev-mock-model"
	}
	now := time.Now().Unix()

	switch {
	case strings.HasSuffix(path, "/chat/completions"):
		if info.Stream {
			writeDevMockChatStream(c, modelName, now)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id":      "chatcmpl_dev_mock",
			"object":  "chat.completion",
			"created": now,
			"model":   modelName,
			"choices": []gin.H{{
				"index": 0,
				"message": gin.H{
					"role":    "assistant",
					"content": devMockText,
				},
				"finish_reason": "stop",
			}},
			"usage": gin.H{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		})
	case strings.HasSuffix(path, "/responses"):
		if info.Stream {
			writeDevMockResponsesStream(c, modelName, now)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"object":     "response",
			"id":         "resp_dev_mock",
			"model":      modelName,
			"created_at": now,
			"status":     "completed",
			"output": []gin.H{{
				"id":     "msg_dev_mock",
				"type":   "message",
				"role":   "assistant",
				"status": "completed",
				"content": []gin.H{{
					"type": "output_text",
					"text": devMockText,
				}},
			}},
			"usage": gin.H{
				"input_tokens":  1,
				"output_tokens": 1,
				"total_tokens":  2,
				"input_tokens_details": gin.H{
					"cached_tokens": 0,
				},
				"output_tokens_details": gin.H{
					"reasoning_tokens": 0,
				},
			},
		})
	case strings.HasSuffix(path, "/messages"):
		c.JSON(http.StatusOK, gin.H{
			"id":    "msg_dev_mock",
			"type":  "message",
			"role":  "assistant",
			"model": modelName,
			"content": []gin.H{{
				"type": "text",
				"text": devMockText,
			}},
			"stop_reason":   "end_turn",
			"stop_sequence": nil,
			"usage": gin.H{
				"input_tokens":  1,
				"output_tokens": 1,
			},
		})
	case strings.HasSuffix(path, "/embeddings"):
		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data": []gin.H{{
				"object":    "embedding",
				"index":     0,
				"embedding": []float64{0.01, 0.02, 0.03},
			}},
			"model": modelName,
			"usage": gin.H{
				"prompt_tokens": 1,
				"total_tokens":  1,
			},
		})
	case strings.HasSuffix(path, "/images/generations"), strings.HasSuffix(path, "/images/edits"), strings.HasSuffix(path, "/images/variations"):
		c.JSON(http.StatusOK, gin.H{
			"created": now,
			"data": []gin.H{{
				"b64_json":       base64.StdEncoding.EncodeToString([]byte("dev-mock-image")),
				"revised_prompt": devMockText,
			}},
		})
	case strings.HasSuffix(path, "/audio/speech"):
		payload := []byte("dev mock audio")
		c.Data(http.StatusOK, "audio/mpeg", payload)
	case strings.HasSuffix(path, "/audio/transcriptions"):
		c.JSON(http.StatusOK, gin.H{
			"text": devMockText,
		})
	case strings.HasSuffix(path, "/videos/generations"), strings.HasSuffix(path, "/music/generations"):
		c.JSON(http.StatusOK, gin.H{
			"id":     "job_dev_mock",
			"status": "succeeded",
			"data": []gin.H{{
				"url": "https://example.com/dev-mock",
			}},
		})
	case strings.HasSuffix(path, "/search"):
		c.JSON(http.StatusOK, gin.H{
			"data": []gin.H{{
				"title":   "dev mock result",
				"url":     "https://example.com/dev-mock",
				"content": devMockText,
			}},
		})
	case strings.HasSuffix(path, "/rerank"):
		c.JSON(http.StatusOK, gin.H{
			"results": []gin.H{{
				"index":           0,
				"relevance_score": 0.99,
			}},
		})
	case strings.HasSuffix(path, "/moderations"):
		c.JSON(http.StatusOK, gin.H{
			"id":    "modr_dev_mock",
			"model": modelName,
			"results": []gin.H{{
				"flagged":         false,
				"categories":      gin.H{},
				"category_scores": gin.H{},
			}},
		})
	default:
		c.JSON(http.StatusOK, gin.H{
			"message": devMockText,
		})
	}
}

func writeDevMockChatStream(c *gin.Context, modelName string, now int64) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	chunks := []string{
		`data: {"id":"chatcmpl_dev_mock","object":"chat.completion.chunk","created":` + jsonNumber(now) + `,"model":"` + modelName + `","choices":[{"index":0,"delta":{"role":"assistant","content":"` + devMockText + `"},"finish_reason":null}]}` + "\n\n",
		`data: {"id":"chatcmpl_dev_mock","object":"chat.completion.chunk","created":` + jsonNumber(now) + `,"model":"` + modelName + `","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n",
		"data: [DONE]\n\n",
	}
	for _, chunk := range chunks {
		_, _ = c.Writer.Write([]byte(chunk))
		c.Writer.Flush()
	}
}

func writeDevMockResponsesStream(c *gin.Context, modelName string, now int64) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	events := []string{
		`data: {"type":"response.created","sequence_number":0,"response":{"object":"response","id":"resp_dev_mock","model":"` + modelName + `","created_at":` + jsonNumber(now) + `,"output":[],"status":"in_progress"}}` + "\n\n",
		`data: {"type":"response.output_text.delta","sequence_number":1,"item_id":"msg_dev_mock","output_index":0,"content_index":0,"delta":"` + devMockText + `"}` + "\n\n",
		`data: {"type":"response.completed","sequence_number":2,"response":{"object":"response","id":"resp_dev_mock","model":"` + modelName + `","created_at":` + jsonNumber(now) + `,"output":[],"status":"completed","usage":{"input_tokens":1,"input_tokens_details":{"cached_tokens":0},"output_tokens":1,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":2}}}` + "\n\n",
		"data: [DONE]\n\n",
	}
	for _, event := range events {
		_, _ = c.Writer.Write([]byte(event))
		c.Writer.Flush()
	}
}

func jsonNumber(value int64) string {
	return strconv.FormatInt(value, 10)
}
