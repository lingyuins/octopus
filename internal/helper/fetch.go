package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dlclark/regexp2"
	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

func FetchModels(ctx context.Context, request model.Channel) ([]string, error) {
	if conf.IsDevMockSuccess() {
		return filterDevMockModels(request)
	}

	client, err := ChannelHttpClient(&request)
	if err != nil {
		return nil, err
	}
	return fetchModelsWithClient(client, ctx, request)
}

// FetchModelsShortTimeout 使用短超时(30s) HTTP 客户端获取模型列表
// 用于后台同步任务，避免不可达 endpoint 长时间占用连接
func FetchModelsShortTimeout(ctx context.Context, request model.Channel) ([]string, error) {
	if conf.IsDevMockSuccess() {
		return filterDevMockModels(request)
	}

	client, err := ChannelShortTimeoutHttpClient(&request)
	if err != nil {
		return nil, err
	}
	return fetchModelsWithClient(client, ctx, request)
}

func fetchModelsWithClient(client *http.Client, ctx context.Context, request model.Channel) ([]string, error) {
	fetchModel := make([]string, 0)
	var err error
	switch request.Type {
	case outbound.OutboundTypeAnthropic:
		fetchModel, err = fetchAnthropicModels(client, ctx, request)
	case outbound.OutboundTypeGemini:
		fetchModel, err = fetchGeminiModels(client, ctx, request)
	default:
		fetchModel, err = fetchOpenAIModels(client, ctx, request)
	}
	if err != nil {
		return nil, err
	}
	if request.MatchRegex != nil && *request.MatchRegex != "" {
		matchModel := make([]string, 0)
		re, err := regexp2.Compile(*request.MatchRegex, regexp2.ECMAScript)
		if err != nil {
			return nil, err
		}
		for _, model := range fetchModel {
			matched, err := re.MatchString(model)
			if err != nil {
				return nil, err
			}
			if matched {
				matchModel = append(matchModel, model)
			}
		}
		return matchModel, nil
	}
	return fetchModel, nil
}

func filterDevMockModels(request model.Channel) ([]string, error) {
	models := []string{
		"gpt-4o",
		"gpt-4.1",
		"text-embedding-3-small",
		"claude-3-7-sonnet",
		"gemini-2.5-pro",
		"mimo-v2.5",
	}
	if request.MatchRegex == nil || strings.TrimSpace(*request.MatchRegex) == "" {
		return models, nil
	}

	re, err := regexp2.Compile(*request.MatchRegex, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}
	filtered := make([]string, 0, len(models))
	for _, item := range models {
		matched, err := re.MatchString(item)
		if err != nil {
			return nil, err
		}
		if matched {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

// refer: https://platform.openai.com/docs/api-reference/models/list
func fetchOpenAIModels(client *http.Client, ctx context.Context, request model.Channel) ([]string, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		request.GetNormalizedBaseUrl()+"/models",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+request.GetChannelKey().ChannelKey)
	for _, header := range request.CustomHeader {
		if header.HeaderKey != "" {
			req.Header.Set(header.HeaderKey, header.HeaderValue)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return nil, fmt.Errorf("fetch models failed: status %d: %s", resp.StatusCode, message)
	}

	var result model.OpenAIModelList

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

// refer: https://ai.google.dev/api/models
func fetchGeminiModels(client *http.Client, ctx context.Context, request model.Channel) ([]string, error) {
	var allModels []string
	pageToken := ""

	for {
		err := func() error {
			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodGet,
				request.GetNormalizedBaseUrl()+"/models",
				nil,
			)
			if err != nil {
				return fmt.Errorf("create request: %w", err)
			}
			req.Header.Set("X-Goog-Api-Key", request.GetChannelKey().ChannelKey)
			for _, header := range request.CustomHeader {
				if header.HeaderKey != "" {
					req.Header.Set(header.HeaderKey, header.HeaderValue)
				}
			}
			if pageToken != "" {
				q := req.URL.Query()
				q.Add("pageToken", pageToken)
				req.URL.RawQuery = q.Encode()
			}

			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			var result model.GeminiModelList

			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return err
			}

			for _, m := range result.Models {
				name := strings.TrimPrefix(m.Name, "models/")
				allModels = append(allModels, name)
			}

			if result.NextPageToken == "" {
				pageToken = ""
				return nil
			}
			pageToken = result.NextPageToken
			return nil
		}()
		if err != nil {
			return nil, err
		}
		if pageToken == "" {
			break
		}
	}
	if len(allModels) == 0 {
		return fetchOpenAIModels(client, ctx, request)
	}
	return allModels, nil
}

// refer: https://platform.claude.com/docs
func fetchAnthropicModels(client *http.Client, ctx context.Context, request model.Channel) ([]string, error) {

	var allModels []string
	var afterID string
	for {

		err := func() error {
			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodGet,
				request.GetNormalizedBaseUrl()+"/models",
				nil,
			)
			if err != nil {
				return fmt.Errorf("create request: %w", err)
			}
			req.Header.Set("X-Api-Key", request.GetChannelKey().ChannelKey)
			req.Header.Set("Anthropic-Version", "2023-06-01")
			for _, header := range request.CustomHeader {
				if header.HeaderKey != "" {
					req.Header.Set(header.HeaderKey, header.HeaderValue)
				}
			}
			// 设置多页参数
			q := req.URL.Query()

			if afterID != "" {
				q.Set("after_id", afterID)
			}
			req.URL.RawQuery = q.Encode()

			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			var result model.AnthropicModelList

			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return err
			}

			for _, m := range result.Data {
				allModels = append(allModels, m.ID)
			}

			if !result.HasMore {
				afterID = ""
				return nil
			}

			afterID = result.LastID
			return nil
		}()
		if err != nil {
			return nil, err
		}
		if afterID == "" {
			break
		}
	}
	if len(allModels) == 0 {
		return fetchOpenAIModels(client, ctx, request)
	}
	return allModels, nil
}
