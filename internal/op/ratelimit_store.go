package op

import (
	"github.com/lingyuins/octopus/internal/op/ratelimitstore"
)

// Deprecated: Use ratelimitstore.CheckRateLimit from internal/op/ratelimitstore instead.
func CheckRateLimit(apiKeyID int, modelName string, rpm int, tpm int, tokenCount int) (allowed bool, remaining int, retryAfter int) {
	return ratelimitstore.CheckRateLimit(apiKeyID, modelName, rpm, tpm, tokenCount)
}

// Deprecated: Use ratelimitstore.ConsumeTokens from internal/op/ratelimitstore instead.
func ConsumeTokens(apiKeyID int, modelName string, tpm int, tokenCount int) {
	ratelimitstore.ConsumeTokens(apiKeyID, modelName, tpm, tokenCount)
}
