package task

import (
	"context"
	"time"

	"github.com/lingyuins/octopus/internal/helper"
	"github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/utils/log"
)

func ChannelBaseUrlDelayTask() {
	log.Debugf("channel base url delay task started")
	startTime := time.Now()
	defer func() {
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
		helper.ChannelBaseUrlDelayUpdate(&ch, ctx)
	}
}
