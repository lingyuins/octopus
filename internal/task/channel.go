package task

import (
	"context"
	"time"

	"github.com/lingyuins/octopus/internal/helper"
	"github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// delayFailureTracker 延迟探测任务的失败追踪器（进程生命周期内有效）
var delayFailureTracker = NewFailureTracker()

func ChannelBaseUrlDelayTask() {
	log.Debugf("channel base url delay task started")
	startTime := time.Now()
	defer func() {
		delayFailureTracker.Cleanup()
		log.Debugf("channel base url delay task finished, update time: %s", time.Since(startTime))
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	channels, err := channel.List(ctx)
	if err != nil {
		log.Errorf("failed to list channels: %v", err)
		return
	}
	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}
		if delayFailureTracker.ShouldSkip(ch.ID) {
			log.Debugf("skipping channel %s (id=%d) — in cooldown", ch.Name, ch.ID)
			continue
		}
		if err := helper.ChannelBaseUrlDelayUpdate(&ch, ctx); err != nil {
			delayFailureTracker.RecordFailure(ch.ID, ch.Name)
			continue
		}
		delayFailureTracker.RecordSuccess(ch.ID)
	}
}
