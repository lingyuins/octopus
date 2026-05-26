package relay

import (
	"context"
	"fmt"
	"time"

	dbmodel "github.com/lingyuins/octopus/internal/model"
	ch "github.com/lingyuins/octopus/internal/op/channel"
	st "github.com/lingyuins/octopus/internal/op/stats"
	"github.com/lingyuins/octopus/internal/relay/balancer"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// PrepareCandidateResult 准备候选的结果
type PrepareCandidateResult struct {
	Channel       *dbmodel.Channel
	UsedKey       dbmodel.ChannelKey
	SkipReason    string
	SkipStatus    dbmodel.AttemptStatus
	ResolvedModel string
}

func PrepareCandidate(
	ctx context.Context,
	item dbmodel.GroupItem,
	iter *balancer.Iterator,
	ratelimitCooldown int,
	requestModel string,
	zenPreferredCheck func(channelType int) bool,
) PrepareCandidateResult {
	result := PrepareCandidateResult{}

	channel, err := ch.Get(item.ChannelID, ctx)
	if err != nil {
		log.Warnf("failed to get channel %d: %v", item.ChannelID, err)
		result.SkipReason = fmt.Sprintf("channel not found: %v", err)
		result.SkipStatus = dbmodel.AttemptSkipped
		return result
	}
	result.Channel = channel

	if !channel.Enabled {
		result.SkipReason = "channel disabled"
		result.SkipStatus = dbmodel.AttemptSkipped
		return result
	}

	usedKey := channel.GetChannelKeyWithCooldown(ratelimitCooldown)
	if usedKey.ChannelKey == "" {
		result.SkipReason = "no available key"
		result.SkipStatus = dbmodel.AttemptSkipped
		return result
	}
	result.UsedKey = usedKey

	resolvedModel := resolveCandidateModelName(requestModel, item)
	if resolvedModel == "" {
		result.SkipReason = "resolved upstream model is empty"
		result.SkipStatus = dbmodel.AttemptSkipped
		return result
	}
	result.ResolvedModel = resolvedModel

	if hint, ok := globalFailureHintCache.get(channel.ID, usedKey.ID, resolvedModel); ok {
		result.SkipReason = failureHintSkipReason(hint)
		result.SkipStatus = dbmodel.AttemptSkipped
		return result
	}

	if iter.SkipCircuitBreak(channel.ID, usedKey.ID, channel.Name, resolvedModel) {
		result.SkipReason = "circuit breaker tripped"
		result.SkipStatus = dbmodel.AttemptCircuitBreak
		return result
	}

	if zenPreferredCheck != nil && !zenPreferredCheck(int(channel.Type)) {
		result.SkipReason = "channel type not preferred for zen model prefix"
		result.SkipStatus = dbmodel.AttemptSkipped
		return result
	}

	return result
}

func PrepareCandidateForRetry(
	channel *dbmodel.Channel,
	failedKeyIDs []int,
	iter *balancer.Iterator,
	ratelimitCooldown int,
	modelName string,
) (dbmodel.ChannelKey, string) {
	usedKey := channel.GetChannelKeyExcludingWithCooldown(failedKeyIDs, ratelimitCooldown)
	if usedKey.ChannelKey == "" {
		return dbmodel.ChannelKey{}, "no more keys to retry"
	}

	if hint, ok := globalFailureHintCache.get(channel.ID, usedKey.ID, modelName); ok {
		return usedKey, failureHintSkipReason(hint)
	}

	if iter.SkipCircuitBreak(channel.ID, usedKey.ID, channel.Name, modelName) {
		return usedKey, "circuit breaker tripped on retry key"
	}

	return usedKey, ""
}

func RecordSuccessSideEffects(
	channel *dbmodel.Channel,
	usedKey dbmodel.ChannelKey,
	span *balancer.AttemptSpan,
	statusCode int,
	modelName string,
	apiKeyID int,
	requestModel string,
	cost float64,
) {
	usedKey.StatusCode = statusCode
	usedKey.LastUseTimeStamp = time.Now().Unix()
	usedKey.TotalCost += cost
	ch.KeyUpdate(usedKey)

	span.End(dbmodel.AttemptSuccess, statusCode, "")

	st.ChannelUpdate(channel.ID, dbmodel.StatsMetrics{
		WaitTime:       span.Duration().Milliseconds(),
		RequestSuccess: 1,
	})

	balancer.RecordSuccess(channel.ID, usedKey.ID, modelName)
	balancer.RecordAutoSuccess(channel.ID, modelName)
	balancer.SetSticky(apiKeyID, requestModel, channel.ID, usedKey.ID)
}

func RecordFailureSideEffects(
	channel *dbmodel.Channel,
	usedKey dbmodel.ChannelKey,
	span *balancer.AttemptSpan,
	statusCode int,
	modelName string,
	decision RetryDecision,
	tryIndex int,
	tryTotal int,
) {
	usedKey.StatusCode = statusCode
	usedKey.LastUseTimeStamp = time.Now().Unix()
	ch.KeyUpdate(usedKey)

	msg := decision.String()
	if tryTotal > 1 {
		msg = fmt.Sprintf("retry %d/%d failed: %s", tryIndex, tryTotal, msg)
	}

	span.End(dbmodel.AttemptFailed, statusCode, msg)

	st.ChannelUpdate(channel.ID, dbmodel.StatsMetrics{
		WaitTime:      span.Duration().Milliseconds(),
		RequestFailed: 1,
	})

	if decision.Scope == ScopeNextChannel || decision.Scope == ScopeAbortAll {
		balancer.RecordFailure(channel.ID, usedKey.ID, modelName)
		balancer.RecordAutoFailure(channel.ID, modelName)
	}

	if decision.IsError {
		log.Warnf("channel %s failed: %s (decision: %s)", channel.Name, msg, decision.Scope.String())
	}
}

func IsRetryAllowed(decision RetryDecision) (continueRetry bool, switchChannel bool) {
	switch decision.Scope {
	case ScopeNone:
		return false, false
	case ScopeSameChannel:
		return true, false
	case ScopeNextChannel:
		return true, true
	case ScopeAbortAll:
		return false, false
	default:
		return false, false
	}
}
