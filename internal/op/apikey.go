package op

import (
	"context"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/apikey"
	"github.com/lingyuins/octopus/internal/op/stats"
)

// apiKeyCache is retained for backward compatibility (used by tests).
var apiKeyCache = apikey.GetCache()
var apiKeyIDMap = apikey.GetIDMap()

func init() {
	apikey.DeleteStatsFunc = func(id int) {
		stats.OnAPIKeyDeleted(id)
	}
}

// Deprecated: Use apikey.Create from internal/op/apikey instead.
func APIKeyCreate(key *model.APIKey, ctx context.Context) error {
	return apikey.Create(key, ctx)
}

// Deprecated: Use apikey.Update from internal/op/apikey instead.
func APIKeyUpdate(key *model.APIKey, ctx context.Context) error {
	return apikey.Update(key, ctx)
}

// Deprecated: Use apikey.List from internal/op/apikey instead.
func APIKeyList(ctx context.Context) ([]model.APIKey, error) {
	return apikey.List(ctx)
}

// Deprecated: Use apikey.Get from internal/op/apikey instead.
func APIKeyGet(id int, ctx context.Context) (model.APIKey, error) {
	return apikey.Get(id, ctx)
}

// Deprecated: Use apikey.GetByKey from internal/op/apikey instead.
func APIKeyGetByAPIKey(key string, ctx context.Context) (model.APIKey, error) {
	return apikey.GetByKey(key, ctx)
}

// Deprecated: Use apikey.Delete from internal/op/apikey instead.
func APIKeyDelete(id int, ctx context.Context) error {
	return apikey.Delete(id, ctx)
}

// apiKeyRefreshCache is called by cache.go (same package)
func apiKeyRefreshCache(ctx context.Context) error {
	return apikey.RefreshCache(ctx)
}
