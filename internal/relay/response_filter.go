package relay

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	dbmodel "github.com/lingyuins/octopus/internal/model"
	stg "github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/transformer/model"
)

// responseFilterConfig holds the parsed response filter settings.
type responseFilterConfig struct {
	Enabled      bool
	Keywords     []string
	Action       string // "block" or "replace"
	ErrorMessage string
}

// loadResponseFilterConfig reads the response filter configuration from settings.
func loadResponseFilterConfig() responseFilterConfig {
	enabled, _ := stg.GetBool(dbmodel.SettingKeyResponseFilterEnabled)
	action, _ := stg.GetString(dbmodel.SettingKeyResponseFilterAction)
	errMsg, _ := stg.GetString(dbmodel.SettingKeyResponseFilterErrorMessage)
	raw, _ := stg.GetString(dbmodel.SettingKeyResponseFilterKeywords)

	cfg := responseFilterConfig{
		Enabled:      enabled,
		Action:       action,
		ErrorMessage: errMsg,
	}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &cfg.Keywords)
	}

	if cfg.Action == "" {
		cfg.Action = "block"
	}
	if cfg.ErrorMessage == "" {
		cfg.ErrorMessage = "The response contains blocked keywords and has been intercepted."
	}

	return cfg
}

// extractResponseText collects all text content from an InternalLLMResponse.
// It covers both Message (non-streaming) and Delta (streaming) fields,
// as well as Content.Content and Content.MultipleContent.
func extractResponseText(resp *model.InternalLLMResponse) string {
	if resp == nil {
		return ""
	}
	var buf strings.Builder
	for _, choice := range resp.Choices {
		var msg *model.Message
		if choice.Message != nil {
			msg = choice.Message
		} else if choice.Delta != nil {
			msg = choice.Delta
		}
		if msg == nil {
			continue
		}
		if msg.Content.Content != nil {
			buf.WriteString(*msg.Content.Content)
		}
		for _, part := range msg.Content.MultipleContent {
			if part.Type == "text" && part.Text != nil && *part.Text != "" {
				buf.WriteString(*part.Text)
			}
		}
	}
	return buf.String()
}

// findMatchedKeyword checks if any keyword is contained in the text (case-insensitive).
// Returns the first matched keyword, or empty string if no match.
func findMatchedKeyword(text string, keywords []string) string {
	if len(keywords) == 0 || text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(kw)) {
			return kw
		}
	}
	return ""
}

// replaceKeywordsInText replaces all occurrences of keywords in the text with asterisks.
func replaceKeywordsInText(text string, keywords []string) string {
	result := text
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		// Case-insensitive replacement
		mask := strings.Repeat("*", utf8.RuneCountInString(kw))
		lower := strings.ToLower(result)
		lowerKw := strings.ToLower(kw)
		var buf strings.Builder
		start := 0
		for {
			idx := strings.Index(lower[start:], lowerKw)
			if idx < 0 {
				buf.WriteString(result[start:])
				break
			}
			buf.WriteString(result[start : start+idx])
			buf.WriteString(mask)
			start += idx + len(kw)
		}
		result = buf.String()
	}
	return result
}

// applyResponseFilter checks and filters the response based on keyword configuration.
// Returns (shouldBlock bool, matchedKeyword string).
// If action is "replace", it modifies the response in-place and returns (false, "").
// If action is "block" and a keyword is matched, returns (true, keyword).
func applyResponseFilter(resp *model.InternalLLMResponse, cfg responseFilterConfig) (bool, string) {
	if !cfg.Enabled || len(cfg.Keywords) == 0 || resp == nil {
		return false, ""
	}

	text := extractResponseText(resp)
	matched := findMatchedKeyword(text, cfg.Keywords)
	if matched == "" {
		return false, ""
	}

	if cfg.Action == "replace" {
		// Replace keywords in response content
		for i, choice := range resp.Choices {
			var msg *model.Message
			if choice.Message != nil {
				msg = choice.Message
			} else if choice.Delta != nil {
				msg = choice.Delta
			}
			if msg == nil {
				continue
			}
			if msg.Content.Content != nil {
				replaced := replaceKeywordsInText(*msg.Content.Content, cfg.Keywords)
				msg.Content.Content = &replaced
			}
			for j, part := range msg.Content.MultipleContent {
				if part.Type == "text" && part.Text != nil && *part.Text != "" {
					replaced := replaceKeywordsInText(*part.Text, cfg.Keywords)
					msg.Content.MultipleContent[j].Text = &replaced
				}
			}
			resp.Choices[i] = choice
		}
		return false, ""
	}

	// action == "block"
	return true, matched
}
