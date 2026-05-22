package group

import (
	"context"
	"testing"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/channel"
)

func TestGroupListModel_EmptyGroupNotListed(t *testing.T) {
	groupCache.Clear()
	groupCache.Set(1, model.Group{
		ID:           1,
		Name:         "music-2.6",
		EndpointType: model.EndpointTypeMusicGeneration,
		Items:        nil,
	})

	models, err := GroupListModel(context.Background())
	if err != nil {
		t.Fatalf("GroupListModel returned error: %v", err)
	}

	for _, m := range models {
		if m == "music-2.6" {
			t.Errorf("music-2.6 should NOT appear in /v1/models when group has no items")
		}
	}
}

func TestGroupListModel_DisabledChannelNotListed(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Set(100, model.Channel{
		ID:      100,
		Name:    "test-channel",
		Enabled: false,
	})

	groupCache.Set(2, model.Group{
		ID:           2,
		Name:         "music-2.6",
		EndpointType: model.EndpointTypeMusicGeneration,
		Items: []model.GroupItem{
			{ID: 1, GroupID: 2, ChannelID: 100, ModelName: "music-2.6", Priority: 1, Weight: 1},
		},
	})

	models, err := GroupListModel(context.Background())
	if err != nil {
		t.Fatalf("GroupListModel returned error: %v", err)
	}

	for _, m := range models {
		if m == "music-2.6" {
			t.Errorf("music-2.6 should NOT appear when channel is disabled")
		}
	}

	// Cleanup
	chCache.Del(100)
}

func TestGroupListModel_ValidGroupListed(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Set(200, model.Channel{
		ID:      200,
		Name:    "enabled-channel",
		Enabled: true,
	})

	groupCache.Set(3, model.Group{
		ID:           3,
		Name:         "music-2.6",
		EndpointType: model.EndpointTypeMusicGeneration,
		Items: []model.GroupItem{
			{ID: 2, GroupID: 3, ChannelID: 200, ModelName: "music-2.6", Priority: 1, Weight: 1},
		},
	})

	models, err := GroupListModel(context.Background())
	if err != nil {
		t.Fatalf("GroupListModel returned error: %v", err)
	}

	found := false
	for _, m := range models {
		if m == "music-2.6" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("music-2.6 SHOULD appear when group has valid item and enabled channel")
	}

	// Cleanup
	chCache.Del(200)
}
