package remotesite

import (
	"context"
	"fmt"

	"github.com/lingyuins/octopus/internal/hub"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// MigrateChannelRequest describes a channel migration operation.
type MigrateChannelRequest struct {
	SourceSiteID int `json:"source_site_id" binding:"required"`
	TargetSiteID int `json:"target_site_id"` // 0 = import to local Octopus
	ChannelID    int `json:"channel_id" binding:"required"`
}

// MigrateChannel copies a channel from a source site to either a target remote site or local Octopus.
func MigrateChannel(ctx context.Context, req *MigrateChannelRequest) error {
	srcSite, err := Get(ctx, req.SourceSiteID)
	if err != nil {
		return fmt.Errorf("get source site: %w", err)
	}

	srcAdapter, err := hub.Get(srcSite.SiteType)
	if err != nil {
		return fmt.Errorf("get source adapter: %w", err)
	}

	channels, err := srcAdapter.ListChannels(ctx, srcSite)
	if err != nil {
		return fmt.Errorf("list source channels: %w", err)
	}

	var srcChannel *hub.RemoteChannel
	for i := range channels {
		if channels[i].ID == req.ChannelID {
			srcChannel = &channels[i]
			break
		}
	}
	if srcChannel == nil {
		return fmt.Errorf("channel %d not found on source site", req.ChannelID)
	}

	if req.TargetSiteID == 0 {
		return importToLocalChannel(ctx, srcSite, srcChannel)
	}

	return importToRemoteSite(ctx, srcSite, srcChannel, req.TargetSiteID)
}

// MigrateAllChannels migrates all channels from a source site to the target.
func MigrateAllChannels(ctx context.Context, sourceSiteID, targetSiteID int) (int, error) {
	srcSite, err := Get(ctx, sourceSiteID)
	if err != nil {
		return 0, err
	}

	srcAdapter, err := hub.Get(srcSite.SiteType)
	if err != nil {
		return 0, err
	}

	channels, err := srcAdapter.ListChannels(ctx, srcSite)
	if err != nil {
		return 0, fmt.Errorf("list channels: %w", err)
	}

	count := 0
	for _, ch := range channels {
		c := ch
		var migrateErr error
		if targetSiteID == 0 {
			migrateErr = importToLocalChannel(ctx, srcSite, &c)
		} else {
			migrateErr = importToRemoteSite(ctx, srcSite, &c, targetSiteID)
		}
		if migrateErr != nil {
			log.Warnf("migrate channel %s: %v", c.Name, migrateErr)
			continue
		}
		count++
	}
	return count, nil
}

func importToLocalChannel(ctx context.Context, srcSite *model.RemoteSite, ch *hub.RemoteChannel) error {
	localCh := &model.Channel{
		Name:    fmt.Sprintf("%s-%s", srcSite.Name, ch.Name),
		Type:    outbound.OutboundTypeOpenAIChat,
		Enabled: ch.Status == 1,
		BaseUrls: []model.BaseUrl{
			{URL: ch.BaseURL},
		},
		Model: ch.Models,
	}
	return channel.Create(localCh, ctx)
}

func importToRemoteSite(ctx context.Context, srcSite *model.RemoteSite, ch *hub.RemoteChannel, targetSiteID int) error {
	targetSite, err := Get(ctx, targetSiteID)
	if err != nil {
		return err
	}

	targetAdapter, err := hub.Get(targetSite.SiteType)
	if err != nil {
		return err
	}

	// Fetch tokens from source to find an active key for the new channel.
	srcAdapter, err := hub.Get(srcSite.SiteType)
	if err != nil {
		return err
	}
	srcTokens, err := srcAdapter.FetchTokens(ctx, srcSite)
	if err != nil {
		log.Warnf("fetch tokens for migration: %v", err)
	}

	key := ""
	if len(srcTokens) > 0 {
		for _, t := range srcTokens {
			if t.Status == 1 {
				key = t.Key // plaintext from remote API
				break
			}
		}
	}

	return targetAdapter.CreateChannel(ctx, targetSite, hub.RemoteChannelCreateReq{
		Name:    ch.Name,
		Type:    ch.Type,
		Key:     key,
		BaseURL: ch.BaseURL,
		Models:  ch.Models,
		Group:   ch.Group,
	})
}
