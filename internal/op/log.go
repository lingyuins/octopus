package op

import (
	"context"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/relaylog"
)

var relayLogCache, relayLogCacheLock = relaylog.GetCacheAndLock()

// relayLogCacheReadTokens is kept lowercase for backward compatibility with tests.
func relayLogCacheReadTokens(responseContent string) int {
	return relaylog.RelayLogCacheReadTokens(responseContent)
}

func RelayLogStreamTokenCreate() (string, error) { return relaylog.RelayLogStreamTokenCreate() }

func RelayLogStreamTokenVerify(token string) bool { return relaylog.RelayLogStreamTokenVerify(token) }

func RelayLogStreamTokenRevoke(token string) { relaylog.RelayLogStreamTokenRevoke(token) }

func RelayLogSubscribe() chan model.RelayLog { return relaylog.RelayLogSubscribe() }

func RelayLogUnsubscribe(ch chan model.RelayLog) { relaylog.RelayLogUnsubscribe(ch) }

func RelayLogAdd(ctx context.Context, relayLog model.RelayLog) error {
	return relaylog.RelayLogAdd(ctx, relayLog)
}

func RelayLogSaveDBTask(ctx context.Context) error { return relaylog.RelayLogSaveDBTask(ctx) }

func RelayLogList(ctx context.Context, startTime, endTime *int, page, pageSize int) ([]model.RelayLogListItem, error) {
	return relaylog.RelayLogList(ctx, startTime, endTime, page, pageSize)
}

func RelayLogClear(ctx context.Context) error { return relaylog.RelayLogClear(ctx) }

func RelayLogGetByID(ctx context.Context, id int64) (*model.RelayLog, error) {
	return relaylog.RelayLogGetByID(ctx, id)
}
