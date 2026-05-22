package relay

import (
	"testing"

	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

func TestItemSupportsEndpoint_ModelNameHeuristic_Image(t *testing.T) {
	ch := dbmodel.Channel{
		ID:   1,
		Type: outbound.OutboundTypeOpenAIChat,
	}
	item := dbmodel.GroupItem{ChannelID: 1, ModelName: "dall-e-3"}

	if !itemSupportsEndpoint(item, ch, dbmodel.EndpointTypeImageGeneration) {
		t.Error("dall-e-3 should match image_generation via model name heuristic")
	}
}

func TestItemSupportsEndpoint_ModelNameHeuristic_Music(t *testing.T) {
	ch := dbmodel.Channel{
		ID:   2,
		Type: outbound.OutboundTypeOpenAIChat,
	}
	item := dbmodel.GroupItem{ChannelID: 2, ModelName: "music-2.6"}

	if !itemSupportsEndpoint(item, ch, dbmodel.EndpointTypeMusicGeneration) {
		t.Error("music-2.6 should match music_generation via model name heuristic")
	}
}

func TestItemSupportsEndpoint_ModelNameHeuristic_Video(t *testing.T) {
	ch := dbmodel.Channel{
		ID:   3,
		Type: outbound.OutboundTypeOpenAIChat,
	}
	item := dbmodel.GroupItem{ChannelID: 3, ModelName: "veo-2"}

	if !itemSupportsEndpoint(item, ch, dbmodel.EndpointTypeVideoGeneration) {
		t.Error("veo-2 should match video_generation via model name heuristic")
	}
}

func TestItemSupportsEndpoint_ModelNameHeuristic_AudioSpeech(t *testing.T) {
	ch := dbmodel.Channel{
		ID:   4,
		Type: outbound.OutboundTypeOpenAIChat,
	}
	item := dbmodel.GroupItem{ChannelID: 4, ModelName: "tts-1"}

	if !itemSupportsEndpoint(item, ch, dbmodel.EndpointTypeAudioSpeech) {
		t.Error("tts-1 should match audio_speech via model name heuristic")
	}
}

func TestItemSupportsEndpoint_ModelNameHeuristic_AudioTranscription(t *testing.T) {
	ch := dbmodel.Channel{
		ID:   5,
		Type: outbound.OutboundTypeOpenAIChat,
	}
	item := dbmodel.GroupItem{ChannelID: 5, ModelName: "whisper-1"}

	if !itemSupportsEndpoint(item, ch, dbmodel.EndpointTypeAudioTranscription) {
		t.Error("whisper-1 should match audio_transcription via model name heuristic")
	}
}

func TestItemSupportsEndpoint_ModelNameHeuristic_Embedding(t *testing.T) {
	ch := dbmodel.Channel{
		ID:   6,
		Type: outbound.OutboundTypeOpenAIChat,
	}
	item := dbmodel.GroupItem{ChannelID: 6, ModelName: "text-embedding-3-large"}

	if !itemSupportsEndpoint(item, ch, dbmodel.EndpointTypeEmbeddings) {
		t.Error("text-embedding-3-large should match embeddings via model name heuristic")
	}
}

func TestItemSupportsEndpoint_ModelNameHeuristic_Rerank(t *testing.T) {
	ch := dbmodel.Channel{
		ID:   7,
		Type: outbound.OutboundTypeOpenAIChat,
	}
	item := dbmodel.GroupItem{ChannelID: 7, ModelName: "bge-rerank-v2"}

	if !itemSupportsEndpoint(item, ch, dbmodel.EndpointTypeRerank) {
		t.Error("bge-rerank-v2 should match rerank via model name heuristic")
	}
}

func TestItemSupportsEndpoint_ModelNameHeuristic_Moderation(t *testing.T) {
	ch := dbmodel.Channel{
		ID:   8,
		Type: outbound.OutboundTypeOpenAIChat,
	}
	item := dbmodel.GroupItem{ChannelID: 8, ModelName: "text-moderation-stable"}

	if !itemSupportsEndpoint(item, ch, dbmodel.EndpointTypeModerations) {
		t.Error("text-moderation-stable should match moderations via model name heuristic")
	}
}

func TestItemSupportsEndpoint_ChatModel_Mismatch(t *testing.T) {
	ch := dbmodel.Channel{
		ID:   9,
		Type: outbound.OutboundTypeOpenAIChat,
	}
	item := dbmodel.GroupItem{ChannelID: 9, ModelName: "gpt-4o"}

	if itemSupportsEndpoint(item, ch, dbmodel.EndpointTypeImageGeneration) {
		t.Error("gpt-4o should NOT match image_generation (no image/heuristic keyword)")
	}
	if itemSupportsEndpoint(item, ch, dbmodel.EndpointTypeMusicGeneration) {
		t.Error("gpt-4o should NOT match music_generation")
	}
}

func TestItemSupportsEndpoint_EmptyModelName(t *testing.T) {
	ch := dbmodel.Channel{
		ID:   10,
		Type: outbound.OutboundTypeOpenAIChat,
	}
	item := dbmodel.GroupItem{ChannelID: 10, ModelName: ""}

	if itemSupportsEndpoint(item, ch, dbmodel.EndpointTypeImageGeneration) {
		t.Error("empty model name should NOT match anything")
	}
}

func TestNarrowGroupItemsForEndpoint_KeepsMatchingOnly(t *testing.T) {
	chCache := channel.GetCache()
	chCache.Set(101, dbmodel.Channel{ID: 101, Type: outbound.OutboundTypeOpenAIChat, Enabled: true})
	chCache.Set(102, dbmodel.Channel{ID: 102, Type: outbound.OutboundTypeOpenAIChat, Enabled: true})

	group := dbmodel.Group{
		ID:           1,
		Name:         "universal",
		EndpointType: dbmodel.EndpointTypeAll,
		Items: []dbmodel.GroupItem{
			{ID: 1, ChannelID: 101, ModelName: "gpt-4o", Priority: 1, Weight: 1},
			{ID: 2, ChannelID: 102, ModelName: "dall-e-3", Priority: 2, Weight: 1},
		},
	}

	narrowed := narrowGroupItemsForEndpoint(group, dbmodel.EndpointTypeImageGeneration)
	if len(narrowed.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(narrowed.Items))
	}
	if narrowed.Items[0].ModelName != "dall-e-3" {
		t.Errorf("expected dall-e-3, got %s", narrowed.Items[0].ModelName)
	}

	chCache.Clear()
}

func TestNarrowGroupItemsForEndpoint_EmptyResult(t *testing.T) {
	chCache := channel.GetCache()
	chCache.Set(201, dbmodel.Channel{ID: 201, Type: outbound.OutboundTypeOpenAIChat, Enabled: true})

	group := dbmodel.Group{
		ID:           2,
		Name:         "universal",
		EndpointType: dbmodel.EndpointTypeAll,
		Items: []dbmodel.GroupItem{
			{ID: 1, ChannelID: 201, ModelName: "gpt-4o", Priority: 1, Weight: 1},
		},
	}

	narrowed := narrowGroupItemsForEndpoint(group, dbmodel.EndpointTypeMusicGeneration)
	if len(narrowed.Items) != 0 {
		t.Fatalf("expected 0 items (no music model), got %d", len(narrowed.Items))
	}

	chCache.Clear()
}
