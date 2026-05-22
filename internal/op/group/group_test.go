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

// --- GroupListModelByEndpoint tests ---

func setupValidGroup(id int, name string, endpointType string) {
	chCache := channel.GetCache()
	channelID := id * 100
	chCache.Set(channelID, model.Channel{
		ID:      channelID,
		Name:    "ch-" + name,
		Enabled: true,
	})
	groupCache.Set(id, model.Group{
		ID:           id,
		Name:         name,
		EndpointType: endpointType,
		Items: []model.GroupItem{
			{ID: id, GroupID: id, ChannelID: channelID, ModelName: name, Priority: 1, Weight: 1},
		},
	})
}

func TestGroupListModelByEndpoint_AllReturnsVisibleModels(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	setupValidGroup(1, "music-2.6", model.EndpointTypeMusicGeneration)
	setupValidGroup(2, "gpt-4o", model.EndpointTypeChat)

	models, err := GroupListModelByEndpoint("", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(models, "music-2.6") || !contains(models, "gpt-4o") {
		t.Errorf("empty endpoint should return all visible models, got: %v", models)
	}

	modelsStar, err := GroupListModelByEndpoint("*", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(modelsStar, "music-2.6") || !contains(modelsStar, "gpt-4o") {
		t.Errorf("* endpoint should return all visible models, got: %v", modelsStar)
	}

	chCache.Clear()
}

func TestGroupListModelByEndpoint_MusicOnly(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	setupValidGroup(1, "music-2.6", model.EndpointTypeMusicGeneration)
	setupValidGroup(2, "gpt-4o", model.EndpointTypeChat)

	models, err := GroupListModelByEndpoint("music_generation", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(models, "music-2.6") {
		t.Errorf("music_generation should include music-2.6, got: %v", models)
	}
	if contains(models, "gpt-4o") {
		t.Errorf("music_generation should NOT include gpt-4o, got: %v", models)
	}

	chCache.Clear()
}

func TestGroupListModelByEndpoint_ChatExcludesMusicOnly(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	setupValidGroup(1, "music-2.6", model.EndpointTypeMusicGeneration)
	setupValidGroup(2, "gpt-4o", model.EndpointTypeChat)

	models, err := GroupListModelByEndpoint("chat", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contains(models, "music-2.6") {
		t.Errorf("chat should NOT include music-2.6, got: %v", models)
	}
	if !contains(models, "gpt-4o") {
		t.Errorf("chat should include gpt-4o, got: %v", models)
	}

	chCache.Clear()
}

func TestGroupListModelByEndpoint_ConversationFamilyMatchesResponses(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	setupValidGroup(1, "gpt-4.1", model.EndpointTypeResponses)

	models, err := GroupListModelByEndpoint("chat", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(models, "gpt-4.1") {
		t.Errorf("chat should include responses model gpt-4.1, got: %v", models)
	}

	models2, err := GroupListModelByEndpoint("messages", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(models2, "gpt-4.1") {
		t.Errorf("messages should include responses model gpt-4.1, got: %v", models2)
	}

	chCache.Clear()
}

func TestGroupListModelByEndpoint_GlobalGroupMatchesAll(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	setupValidGroup(1, "universal-model", model.EndpointTypeAll)

	for _, ep := range []string{"chat", "music_generation", "embeddings", "image_generation"} {
		t.Run("endpoint="+ep, func(t *testing.T) {
			models, err := GroupListModelByEndpoint(ep, context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !contains(models, "universal-model") {
				t.Errorf("%s should include universal-model (endpoint_type=*), got: %v", ep, models)
			}
		})
	}

	chCache.Clear()
}

func TestGroupListModelByEndpoint_InvalidGroupNotListed(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	chCache.Set(999, model.Channel{ID: 999, Name: "disabled", Enabled: false})
	groupCache.Set(99, model.Group{
		ID:           99,
		Name:         "dead-model",
		EndpointType: model.EndpointTypeChat,
		Items: []model.GroupItem{
			{ID: 99, GroupID: 99, ChannelID: 999, ModelName: "dead-model", Priority: 1, Weight: 1},
		},
	})

	models, err := GroupListModelByEndpoint("chat", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contains(models, "dead-model") {
		t.Errorf("disabled channel should be excluded, got: %v", models)
	}

	chCache.Clear()
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
