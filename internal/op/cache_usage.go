package op

import (
	"github.com/lingyuins/octopus/internal/op/cacheusage"
)

type providerPromptCacheUsageSignals = cacheusage.ProviderPromptCacheUsageSignals

func parseProviderPromptCacheUsageSignals(responseContent string) (providerPromptCacheUsageSignals, bool) {
	return cacheusage.ParseProviderPromptCacheUsageSignals(responseContent)
}
