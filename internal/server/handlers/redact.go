package handlers

import (
	"net/url"
	"strings"

	"github.com/lingyuins/octopus/internal/model"
)

const viewerMaskedDomain = "***"

func isViewerRole(role string) bool {
	return strings.TrimSpace(role) == model.UserRoleViewer
}

func maskURLDomainForViewer(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}

	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.User = nil
		parsed.Host = viewerMaskedDomain
		return parsed.String()
	}

	return viewerMaskedDomain
}

func redactChannelBaseURLsForViewer(channels []model.Channel) {
	for channelIndex := range channels {
		for baseURLIndex := range channels[channelIndex].BaseUrls {
			channels[channelIndex].BaseUrls[baseURLIndex].URL = maskURLDomainForViewer(channels[channelIndex].BaseUrls[baseURLIndex].URL)
		}
		// Mask custom proxy addresses (may contain credentials: socks5://user:pass@host)
		if channels[channelIndex].ChannelProxy != nil {
			masked := maskURLDomainForViewer(*channels[channelIndex].ChannelProxy)
			channels[channelIndex].ChannelProxy = &masked
		}
	}
}

func redactRemoteSiteBaseURLsForViewer(sites []model.RemoteSite) {
	for siteIndex := range sites {
		sites[siteIndex].BaseURL = maskURLDomainForViewer(sites[siteIndex].BaseURL)
	}
}

func redactCredentialBaseURLsForViewer(profiles []model.APICredentialProfile) {
	for profileIndex := range profiles {
		profiles[profileIndex].BaseURL = maskURLDomainForViewer(profiles[profileIndex].BaseURL)
		profiles[profileIndex].APIKey = viewerMaskedDomain
	}
}

func redactSiteBaseURLsForViewer(sites []model.Site) {
	for siteIndex := range sites {
		sites[siteIndex].BaseURL = maskURLDomainForViewer(sites[siteIndex].BaseURL)
		// Mask site-level custom proxy addresses
		if sites[siteIndex].SiteProxy != nil {
			masked := maskURLDomainForViewer(*sites[siteIndex].SiteProxy)
			sites[siteIndex].SiteProxy = &masked
		}
		// Mask account-level custom proxy addresses (accounts are preloaded in list queries)
		for accountIndex := range sites[siteIndex].Accounts {
			account := &sites[siteIndex].Accounts[accountIndex]
			if account.AccountProxy != nil {
				masked := maskURLDomainForViewer(*account.AccountProxy)
				account.AccountProxy = &masked
			}
			// Mask upstream account credentials: viewer 只能看到站点元信息，
			// 账号密码/token/API Key 不得明文返回。
			account.Password = viewerMaskedDomain
			account.AccessToken = viewerMaskedDomain
			account.APIKey = viewerMaskedDomain
			account.RefreshToken = viewerMaskedDomain
		}
	}
}

func redactSettingsURLsForViewer(settings []model.Setting) {
	for settingIndex := range settings {
		switch settings[settingIndex].Key {
		case model.SettingKeyProxyURL,
			model.SettingKeyPublicAPIBaseURL,
			model.SettingKeySemanticCacheEmbeddingBaseURL,
			model.SettingKeyAIRouteBaseURL:
			settings[settingIndex].Value = maskURLDomainForViewer(settings[settingIndex].Value)
		case model.SettingKeyWebDAVConfig,
			model.SettingKeySemanticCacheEmbeddingAPIKey,
			model.SettingKeyAIRouteAPIKey,
			model.SettingKeyAIRouteServices:
			// 密钥类设置（WebDAV 密码、embedding/路由 API Key、服务池 JSON）对
			// viewer 整体遮蔽，避免明文凭据经设置列表泄露。
			settings[settingIndex].Value = viewerMaskedDomain
		}
	}
}
