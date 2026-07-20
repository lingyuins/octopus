package helper

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/lingyuins/octopus/internal/client"
	"github.com/lingyuins/octopus/internal/model"
	ch "github.com/lingyuins/octopus/internal/op/channel"
	grp "github.com/lingyuins/octopus/internal/op/group"
	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/lingyuins/octopus/internal/utils/xstrings"
)

// ProxyURLByConfigFunc 由 op 包在启动时注入，用于按代理池配置 ID 解析 URL。
// helper 不能直接 import op（op/notification 反向依赖 helper，会形成循环）。
// 未注入时 pool 模式只能依赖 channel.ChannelProxy 自定义 URL。
var ProxyURLByConfigFunc func(id int, ctx context.Context) (string, error)

// ChannelHttpClient 按渠道代理配置返回转发用 HTTP 客户端。
// 优先使用 ProxyMode/ProxyConfigID（站点投影与代理池）；
// 兼容旧字段 Proxy + ChannelProxy。
func ChannelHttpClient(channel *model.Channel) (*http.Client, error) {
	return channelHTTPClient(channel, 0)
}

// ChannelShortTimeoutHttpClient 返回短超时(30s)的 HTTP 客户端。
// 用于后台任务(延迟探测、模型同步)，避免在 endpoint 不可达时 goroutine 堆积。
func ChannelShortTimeoutHttpClient(channel *model.Channel) (*http.Client, error) {
	return channelHTTPClient(channel, 30*time.Second)
}

func channelHTTPClient(channel *model.Channel, timeout time.Duration) (*http.Client, error) {
	if channel == nil {
		return nil, errors.New("channel is nil")
	}

	mode, proxyConfigID, customProxyURL := resolveChannelProxy(channel)
	short := timeout > 0 && timeout <= 30*time.Second

	switch mode {
	case model.ProxyUsageModeDirect:
		if short {
			return client.GetHTTPClientShortTimeout(false)
		}
		return client.GetHTTPClientSystemProxy(false)
	case model.ProxyUsageModeSystem:
		if short {
			return client.GetHTTPClientShortTimeout(true)
		}
		return client.GetHTTPClientSystemProxy(true)
	case model.ProxyUsageModePool:
		proxyURL := strings.TrimSpace(customProxyURL)
		if proxyURL == "" {
			if proxyConfigID == nil || *proxyConfigID <= 0 {
				return nil, fmt.Errorf("proxy config id is required when proxy mode is pool")
			}
			if ProxyURLByConfigFunc == nil {
				return nil, fmt.Errorf("proxy configuration resolver is not initialized")
			}
			resolved, err := ProxyURLByConfigFunc(*proxyConfigID, context.Background())
			if err != nil {
				return nil, err
			}
			proxyURL = resolved
		}
		if short {
			return client.GetHTTPClientCustomProxyWithTimeout(proxyURL, 30*time.Second)
		}
		return client.GetHTTPClientCustomProxy(proxyURL)
	default:
		return nil, fmt.Errorf("unsupported proxy mode: %s", mode)
	}
}

// resolveChannelProxy 解析渠道实际代理模式。
// 兼容路径：
// 1) 新模型 ProxyMode=system/pool（含 ProxyConfigID）
// 2) 旧模型仅 Proxy + ChannelProxy
// 3) 迁移后 ProxyMode 默认 direct，但 Proxy 仍为 true 的历史数据
func resolveChannelProxy(channel *model.Channel) (model.ProxyUsageMode, *int, string) {
	if channel == nil {
		return model.ProxyUsageModeDirect, nil, ""
	}

	customProxyURL := ""
	if channel.ChannelProxy != nil {
		customProxyURL = strings.TrimSpace(*channel.ChannelProxy)
	}

	mode := channel.ProxyMode
	// 历史数据：GORM 给 proxy_mode 默认值 direct，但旧 proxy 布尔可能仍为 true。
	// 此时必须回退到 legacy 字段，否则系统代理/自定义代理会静默失效。
	if mode == "" || mode == model.ProxyUsageModeDirect {
		if !channel.Proxy {
			return model.ProxyUsageModeDirect, nil, ""
		}
		if customProxyURL != "" {
			return model.ProxyUsageModePool, nil, customProxyURL
		}
		return model.ProxyUsageModeSystem, nil, ""
	}

	switch mode {
	case model.ProxyUsageModeSystem:
		// 系统代理模式下若仍残留 channel_proxy，优先走自定义 URL（兼容迁移中途数据）
		if customProxyURL != "" {
			return model.ProxyUsageModePool, nil, customProxyURL
		}
		return model.ProxyUsageModeSystem, nil, ""
	case model.ProxyUsageModePool:
		if customProxyURL != "" && (channel.ProxyConfigID == nil || *channel.ProxyConfigID <= 0) {
			return model.ProxyUsageModePool, nil, customProxyURL
		}
		return model.ProxyUsageModePool, channel.ProxyConfigID, ""
	default:
		return mode, channel.ProxyConfigID, customProxyURL
	}
}

// ChannelBaseUrlDelayUpdate 更新 channel 的 base URL 延迟信息（使用短超时客户端）
// 返回 error 表示所有 base URL 都探测失败
func ChannelBaseUrlDelayUpdate(channel *model.Channel, ctx context.Context) error {
	if channel == nil {
		return errors.New("channel is nil")
	}
	newBaseUrls := make([]model.BaseUrl, 0, len(channel.BaseUrls))
	allFailed := true

	for _, baseUrl := range channel.BaseUrls {
		if baseUrl.URL == "" {
			continue
		}
		httpClient, err := ChannelShortTimeoutHttpClient(channel)
		if err != nil {
			log.Warnf("failed to get http client (channel=%d): %v", channel.ID, err)
			continue
		}
		delay, err := GetUrlDelay(httpClient, baseUrl.URL, ctx)
		if err != nil {
			log.Warnf("failed to get url delay (channel=%d, url=%s): %v", channel.ID, baseUrl.URL, err)
			continue
		}
		allFailed = false
		newBaseUrls = append(newBaseUrls, model.BaseUrl{
			URL:        baseUrl.URL,
			Delay:      delay,
			SuffixMode: baseUrl.SuffixMode,
		})
	}
	if len(newBaseUrls) > 0 {
		ch.BaseUrlUpdate(channel.ID, newBaseUrls)
	}

	if allFailed && len(channel.BaseUrls) > 0 {
		return fmt.Errorf("all base URLs failed for channel %d", channel.ID)
	}
	return nil
}

func ChannelAutoGroup(channel *model.Channel, ctx context.Context) {
	if channel == nil {
		return
	}
	if channel.AutoGroup == model.AutoGroupTypeNone {
		return
	}
	groups, err := grp.GroupList(ctx)
	if err != nil {
		log.Warnf("get group list failed: %v", err)
		return
	}

	channelModelNames := xstrings.SplitTrimCompact(",", channel.Model, channel.CustomModel)
	if len(channelModelNames) == 0 {
		return
	}

	for _, group := range groups {
		matchedModelNames := make([]string, 0, len(channelModelNames))

		switch channel.AutoGroup {
		case model.AutoGroupTypeExact:
			for _, modelName := range channelModelNames {
				if strings.EqualFold(modelName, group.Name) {
					matchedModelNames = append(matchedModelNames, modelName)
				}
			}

		case model.AutoGroupTypeFuzzy:
			groupNameLower := strings.ToLower(strings.TrimSpace(group.Name))
			if groupNameLower == "" {
				continue
			}
			for _, modelName := range channelModelNames {
				if strings.Contains(strings.ToLower(modelName), groupNameLower) {
					matchedModelNames = append(matchedModelNames, modelName)
				}
			}

		case model.AutoGroupTypeRegex:
			if group.MatchRegex == "" {
				for _, modelName := range channelModelNames {
					if strings.EqualFold(modelName, group.Name) {
						matchedModelNames = append(matchedModelNames, modelName)
					}
				}
				break
			}

			re, err := regexp2.Compile(group.MatchRegex, regexp2.ECMAScript)
			if err != nil {
				log.Warnf("compile regex failed (channel=%d group=%d regex=%q): %v", channel.ID, group.ID, group.MatchRegex, err)
				continue
			}
			for _, modelName := range channelModelNames {
				matched, err := re.MatchString(modelName)
				if err != nil {
					log.Warnf("match regex failed (channel=%d group=%d regex=%q model=%q): %v", channel.ID, group.ID, group.MatchRegex, modelName, err)
					continue
				}
				if matched {
					matchedModelNames = append(matchedModelNames, modelName)
				}
			}
		}

		if len(matchedModelNames) > 0 {
			items := make([]model.GroupIDAndLLMName, 0, len(matchedModelNames))
			for _, modelName := range matchedModelNames {
				items = append(items, model.GroupIDAndLLMName{
					ChannelID: channel.ID,
					ModelName: modelName,
				})
			}
			if err := grp.GroupItemBatchAdd(group.ID, items, ctx); err != nil {
				log.Warnf("group item batch add failed (channel=%d group=%d): %v", channel.ID, group.ID, err)
			}
		}
	}
}
