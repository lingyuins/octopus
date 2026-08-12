package sitesync

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	dbpkg "github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

func TestBuildProjectedChannelBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		site     *model.Site
		expected string
	}{
		{
			name:     "new api appends v1",
			site:     &model.Site{Platform: model.SitePlatformNewAPI, BaseURL: "https://example.com"},
			expected: "https://example.com/v1",
		},
		{
			name:     "one hub preserves existing v1",
			site:     &model.Site{Platform: model.SitePlatformOneHub, BaseURL: "https://example.com/v1"},
			expected: "https://example.com/v1",
		},
		{
			name:     "openai preserves custom path and appends v1",
			site:     &model.Site{Platform: model.SitePlatformOpenAI, BaseURL: "https://example.com/openai"},
			expected: "https://example.com/openai/v1",
		},
		{
			name:     "claude appends v1",
			site:     &model.Site{Platform: model.SitePlatformClaude, BaseURL: "https://api.anthropic.com"},
			expected: "https://api.anthropic.com/v1",
		},
		{
			name:     "gemini appends v1",
			site:     &model.Site{Platform: model.SitePlatformGemini, BaseURL: "https://gemini.example.com"},
			expected: "https://gemini.example.com/v1",
		},
		{
			name:     "nil site returns empty",
			site:     nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if actual := buildProjectedChannelBaseURL(tt.site); actual != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}

func TestProjectAccountSplitsManagedChannelsByOutboundType(t *testing.T) {
	ctx := setupProjectTestDB(t)
	_, account := createProjectionFixture(t, ctx)

	channelIDs, err := ProjectAccount(ctx, account.ID)
	if err != nil {
		t.Fatalf("ProjectAccount returned error: %v", err)
	}
	// new-api is an OpenAI-compatible aggregation platform.
	// All models (gpt-4o-mini, claude-3-5-sonnet, gemini-2.0-flash) should
	// be projected as OpenAIChat channels because aggregation sites do not
	// expose native Anthropic / Gemini endpoints.
	if len(channelIDs) != 1 {
		t.Fatalf("expected 1 managed channel for aggregation platform, got %d", len(channelIDs))
	}

	channelsByGroup := loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	if len(channelsByGroup) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(channelsByGroup))
	}

	assertProjectedChannel(t, channelsByGroup, "default", outbound.OutboundTypeOpenAIChat, "claude-3-5-sonnet,gemini-2.0-flash,gpt-4o-mini", false)

	secondRunIDs, err := ProjectAccount(ctx, account.ID)
	if err != nil {
		t.Fatalf("second ProjectAccount returned error: %v", err)
	}
	if len(secondRunIDs) != 1 {
		t.Fatalf("expected 1 managed channel on second projection, got %d", len(secondRunIDs))
	}

	channelsAfterSecondRun := loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	for groupKey, channel := range channelsByGroup {
		reloaded, ok := channelsAfterSecondRun[groupKey]
		if !ok {
			t.Fatalf("expected binding %q to remain after second projection", groupKey)
		}
		if reloaded.ID != channel.ID {
			t.Fatalf("expected binding %q to reuse channel %d, got %d", groupKey, channel.ID, reloaded.ID)
		}
	}
}

func TestProjectAccountSupportsAllConfiguredRouteBuckets(t *testing.T) {
	ctx := setupProjectTestDB(t)
	_, account := createProjectionFixture(t, ctx)

	extraModels := []model.SiteModel{
		{SiteAccountID: account.ID, GroupKey: model.SiteDefaultGroupKey, ModelName: "text-embedding-3-large", Source: "sync", RouteType: model.SiteModelRouteTypeOpenAIEmbedding},
		{SiteAccountID: account.ID, GroupKey: model.SiteDefaultGroupKey, ModelName: "doubao-seed-1-6", Source: "sync", RouteType: model.SiteModelRouteTypeVolcengine},
	}
	if err := dbpkg.GetDB().WithContext(ctx).Create(&extraModels).Error; err != nil {
		t.Fatalf("create extra site models failed: %v", err)
	}

	channelIDs, err := ProjectAccount(ctx, account.ID)
	if err != nil {
		t.Fatalf("ProjectAccount returned error: %v", err)
	}
	// new-api is OpenAI-only: Anthropic, Gemini, Volcengine models are
	// collapsed into the OpenAIChat bucket.  Embeddings remain separate.
	// Expected: OpenAIChat (gpt-4o-mini + claude + gemini + doubao) + OpenAIEmbedding
	if len(channelIDs) != 2 {
		t.Fatalf("expected 2 managed channels for aggregation platform, got %d", len(channelIDs))
	}

	channelsByGroup := loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	if len(channelsByGroup) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(channelsByGroup))
	}

	assertProjectedChannel(t, channelsByGroup, "default", outbound.OutboundTypeOpenAIChat, "claude-3-5-sonnet,doubao-seed-1-6,gemini-2.0-flash,gpt-4o-mini", false)
	assertProjectedChannel(t, channelsByGroup, "default::openai-embedding", outbound.OutboundTypeOpenAIEmbedding, "text-embedding-3-large", true)
}

func TestProjectAccountRewritesGroupItemsBeforeRemovingStaleManagedBindings(t *testing.T) {
	ctx := setupProjectTestDB(t)
	_, account := createProjectionFixture(t, ctx)

	// Add an embedding model so we get a separate embedding channel
	// (embeddings are NOT overridden to OpenAIChat on aggregation platforms).
	embeddingModel := model.SiteModel{
		SiteAccountID: account.ID,
		GroupKey:      model.SiteDefaultGroupKey,
		ModelName:     "text-embedding-3-large",
		Source:        "sync",
		RouteType:     model.SiteModelRouteTypeOpenAIEmbedding,
	}
	if err := dbpkg.GetDB().WithContext(ctx).Create(&embeddingModel).Error; err != nil {
		t.Fatalf("create embedding site model failed: %v", err)
	}

	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("initial ProjectAccount returned error: %v", err)
	}

	channelsByGroup := loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	openAIChannel, ok := channelsByGroup["default"]
	if !ok {
		t.Fatalf("expected default projected channel to exist")
	}
	embeddingChannel, ok := channelsByGroup["default::openai-embedding"]
	if !ok {
		t.Fatalf("expected embedding projected channel to exist")
	}

	group := &model.Group{Name: "rewrite-managed-items", Mode: model.GroupModeFailover}
	if err := op.GroupCreate(group, ctx); err != nil {
		t.Fatalf("GroupCreate failed: %v", err)
	}
	if err := op.GroupItemAdd(&model.GroupItem{
		GroupID:   group.ID,
		ChannelID: embeddingChannel.ID,
		ModelName: "text-embedding-3-large",
		Priority:  1,
		Weight:    1,
	}, ctx); err != nil {
		t.Fatalf("GroupItemAdd failed: %v", err)
	}

	// Change the embedding model's route_type to OpenAIChat
	if err := dbpkg.GetDB().WithContext(ctx).
		Model(&model.SiteModel{}).
		Where("site_account_id = ? AND group_key = ? AND model_name = ?", account.ID, model.SiteDefaultGroupKey, "text-embedding-3-large").
		Update("route_type", model.SiteModelRouteTypeOpenAIChat).Error; err != nil {
		t.Fatalf("updating site model route_type failed: %v", err)
	}

	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("second ProjectAccount returned error: %v", err)
	}

	items, err := op.GroupItemList(group.ID, ctx)
	if err != nil {
		t.Fatalf("GroupItemList failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected rewritten group item to remain, got %d items", len(items))
	}
	if items[0].ChannelID != openAIChannel.ID {
		t.Fatalf("expected group item to be rewritten onto OpenAI channel %d, got %d", openAIChannel.ID, items[0].ChannelID)
	}

	bindings := loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	if _, ok := bindings["default::openai-embedding"]; ok {
		t.Fatalf("expected stale embedding binding to be removed after route rewrite")
	}
}

// Regression test for issue #86: Re-projecting a NewAPI (OpenAI-only)
// account must not delete GroupItems that target models whose stored
// SiteModel.RouteType is a native conversation route (Gemini / Anthropic /
// Volcengine).  On OpenAI-only platforms those routes collapse onto
// OpenAIChat at projection time; the rewrite path must use the same
// interpretation so that the GroupItem keeps pointing at the OpenAIChat
// projected channel instead of being dropped because no native binding
// exists.
func TestProjectAccountPreservesGroupItemForOpenAIOnlyNativeRoute(t *testing.T) {
	ctx := setupProjectTestDB(t)
	_, account := createProjectionFixture(t, ctx)
	// Fixture site is NewAPI (IsOpenAIOnlyPlatform=true) and contains a
	// gemini-2.0-flash SiteModel with RouteType=Gemini stored in DB.

	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("initial ProjectAccount returned error: %v", err)
	}

	channelsByGroup := loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	openAIChannel, ok := channelsByGroup["default"]
	if !ok {
		t.Fatalf("expected default OpenAIChat projected channel to exist")
	}

	// Mimic auto-grouping: pin gemini-2.0-flash onto the OpenAIChat channel.
	group := &model.Group{Name: "gemini-models", Mode: model.GroupModeFailover, MatchRegex: "^gemini-"}
	if err := op.GroupCreate(group, ctx); err != nil {
		t.Fatalf("GroupCreate failed: %v", err)
	}
	if err := op.GroupItemAdd(&model.GroupItem{
		GroupID:   group.ID,
		ChannelID: openAIChannel.ID,
		ModelName: "gemini-2.0-flash",
		Priority:  1,
		Weight:    1,
	}, ctx); err != nil {
		t.Fatalf("GroupItemAdd failed: %v", err)
	}

	// Re-project — this triggers rewriteManagedGroupItemsForAccount.
	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("second ProjectAccount returned error: %v", err)
	}

	items, err := op.GroupItemList(group.ID, ctx)
	if err != nil {
		t.Fatalf("GroupItemList failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected gemini GroupItem to survive re-projection, got %d items", len(items))
	}
	if items[0].ChannelID != openAIChannel.ID {
		t.Fatalf("expected GroupItem to remain on OpenAIChat channel %d, got %d", openAIChannel.ID, items[0].ChannelID)
	}
	if items[0].ModelName != "gemini-2.0-flash" {
		t.Fatalf("expected GroupItem model name preserved, got %q", items[0].ModelName)
	}
}

func TestProjectAccountRemovesUnsupportedModelsFromProjectedChannels(t *testing.T) {
	ctx := setupProjectTestDB(t)
	_, account := createProjectionFixture(t, ctx)

	extraModel := model.SiteModel{
		SiteAccountID: account.ID,
		GroupKey:      model.SiteDefaultGroupKey,
		ModelName:     "vendor-embedding-x",
		Source:        "sync",
		RouteType:     model.SiteModelRouteTypeOpenAIChat,
		RouteSource:   model.SiteModelRouteSourceSyncInferred,
	}
	if err := dbpkg.GetDB().WithContext(ctx).Create(&extraModel).Error; err != nil {
		t.Fatalf("create extra site model failed: %v", err)
	}

	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("initial ProjectAccount returned error: %v", err)
	}

	channelsByGroup := loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	openAIChannel, ok := channelsByGroup["default"]
	if !ok {
		t.Fatalf("expected default projected channel to exist")
	}
	if openAIChannel.Model != "claude-3-5-sonnet,gemini-2.0-flash,gpt-4o-mini,vendor-embedding-x" {
		t.Fatalf("expected default channel to include all models before vendor becomes unsupported, got %q", openAIChannel.Model)
	}

	group := &model.Group{Name: "remove-unsupported-managed-items", Mode: model.GroupModeFailover}
	if err := op.GroupCreate(group, ctx); err != nil {
		t.Fatalf("GroupCreate failed: %v", err)
	}
	if err := op.GroupItemAdd(&model.GroupItem{
		GroupID:   group.ID,
		ChannelID: openAIChannel.ID,
		ModelName: "vendor-embedding-x",
		Priority:  1,
		Weight:    1,
	}, ctx); err != nil {
		t.Fatalf("GroupItemAdd failed: %v", err)
	}

	unsupportedPayload := model.SiteModelRouteMetadata{
		Source:                 "/api/pricing",
		RouteSupported:         false,
		SupportedEndpointTypes: []string{"/vendor/embeddings"},
		UnsupportedReason:      "site reports endpoint types outside current supported route buckets",
	}.Marshal()
	if err := dbpkg.GetDB().WithContext(ctx).
		Model(&model.SiteModel{}).
		Where("site_account_id = ? AND group_key = ? AND model_name = ?", account.ID, model.SiteDefaultGroupKey, "vendor-embedding-x").
		Updates(map[string]any{
			"route_type":        model.SiteModelRouteTypeUnknown,
			"route_raw_payload": unsupportedPayload,
			"route_source":      model.SiteModelRouteSourceSyncInferred,
		}).Error; err != nil {
		t.Fatalf("updating vendor model route_type failed: %v", err)
	}

	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("second ProjectAccount returned error: %v", err)
	}

	reloadedChannels := loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	reloadedOpenAIChannel, ok := reloadedChannels["default"]
	if !ok {
		t.Fatalf("expected default projected channel to remain after unsupported model removal")
	}
	if reloadedOpenAIChannel.Model != "claude-3-5-sonnet,gemini-2.0-flash,gpt-4o-mini" {
		t.Fatalf("expected unsupported model to be removed from default channel, got %q", reloadedOpenAIChannel.Model)
	}

	items, err := op.GroupItemList(group.ID, ctx)
	if err != nil {
		t.Fatalf("GroupItemList failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected group items for unsupported model to be removed, got %d items", len(items))
	}
}

func TestProjectAccountReusesOrphanManagedChannelWithSameName(t *testing.T) {
	ctx := setupProjectTestDB(t)

	site := &model.Site{
		Name:     "DoneHub Projection Site",
		Platform: model.SitePlatformDoneHub,
		BaseURL:  "https://donehub.example.com",
		Enabled:  true,
	}
	if err := op.SiteCreate(site, ctx); err != nil {
		t.Fatalf("SiteCreate failed: %v", err)
	}

	account := &model.SiteAccount{
		SiteID:         site.ID,
		Name:           "Primary Account",
		CredentialType: model.SiteCredentialTypeAccessToken,
		AccessToken:    "donehub-session-token",
		Enabled:        true,
	}
	if err := op.SiteAccountCreate(account, ctx); err != nil {
		t.Fatalf("SiteAccountCreate failed: %v", err)
	}

	token := model.SiteToken{
		SiteAccountID: account.ID,
		Name:          "primary",
		Token:         "key-primary",
		GroupKey:      model.SiteDefaultGroupKey,
		GroupName:     model.SiteDefaultGroupName,
		Enabled:       true,
	}
	if err := dbpkg.GetDB().WithContext(ctx).Create(&token).Error; err != nil {
		t.Fatalf("create site token failed: %v", err)
	}

	siteModel := model.SiteModel{
		SiteAccountID: account.ID,
		GroupKey:      model.SiteDefaultGroupKey,
		ModelName:     "gpt-4o-mini",
		Source:        "sync",
		RouteType:     model.SiteModelRouteTypeOpenAIChat,
		RouteSource:   model.SiteModelRouteSourceSyncInferred,
	}
	if err := dbpkg.GetDB().WithContext(ctx).Create(&siteModel).Error; err != nil {
		t.Fatalf("create site model failed: %v", err)
	}

	group := model.SiteUserGroup{GroupKey: model.SiteDefaultGroupKey, Name: model.SiteDefaultGroupName}
	orphanName := buildLegacyManagedChannelName(site, account, group, outbound.OutboundTypeOpenAIChat, shouldSplitByOutboundType(site))
	orphanChannel := model.Channel{
		Name:      orphanName,
		Type:      outbound.OutboundTypeOpenAIChat,
		Enabled:   true,
		BaseUrls:  []model.BaseUrl{{URL: "https://donehub.example.com/v1", Delay: 0}},
		Model:     "stale-model",
		AutoGroup: model.AutoGroupTypeNone,
	}
	if err := op.ChannelCreate(&orphanChannel, ctx); err != nil {
		t.Fatalf("creating orphan channel failed: %v", err)
	}

	channelIDs, err := ProjectAccount(ctx, account.ID)
	if err != nil {
		t.Fatalf("ProjectAccount returned error: %v", err)
	}
	if len(channelIDs) != 1 {
		t.Fatalf("expected one projected channel, got %d", len(channelIDs))
	}
	if channelIDs[0] != orphanChannel.ID {
		t.Fatalf("expected orphan channel %d to be reused, got %v", orphanChannel.ID, channelIDs)
	}

	var binding model.SiteChannelBinding
	if err := dbpkg.GetDB().WithContext(ctx).Where("site_account_id = ?", account.ID).First(&binding).Error; err != nil {
		t.Fatalf("expected reused channel to gain binding: %v", err)
	}
	if binding.ChannelID != orphanChannel.ID {
		t.Fatalf("expected binding to point to reused orphan channel %d, got %d", orphanChannel.ID, binding.ChannelID)
	}

	reloaded, err := op.ChannelGet(orphanChannel.ID, ctx)
	if err != nil {
		t.Fatalf("ChannelGet failed: %v", err)
	}
	if reloaded.Name != "DoneHub Projection Site/Primary Account/default-Chat" {
		t.Fatalf("expected reused orphan channel to be renamed, got %q", reloaded.Name)
	}
}

func TestProjectAccountPreservesManagedKeyUsageForUnchangedTokens(t *testing.T) {
	ctx := setupProjectTestDB(t)
	_, account := createProjectionFixture(t, ctx)

	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("initial ProjectAccount returned error: %v", err)
	}

	channelsByGroup := loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	channel := channelsByGroup["default"]
	if len(channel.Keys) == 0 {
		t.Fatalf("expected projected channel keys to exist")
	}

	firstKey := channel.Keys[0]
	firstKey.TotalCost = 12.34
	firstKey.StatusCode = 200
	if err := op.ChannelKeyUpdate(firstKey); err != nil {
		t.Fatalf("ChannelKeyUpdate failed: %v", err)
	}
	if err := op.ChannelKeySaveDB(ctx); err != nil {
		t.Fatalf("ChannelKeySaveDB failed: %v", err)
	}

	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("second ProjectAccount returned error: %v", err)
	}

	reloadedChannel, err := op.ChannelGet(channel.ID, ctx)
	if err != nil {
		t.Fatalf("ChannelGet failed: %v", err)
	}
	if len(reloadedChannel.Keys) != len(channel.Keys) {
		t.Fatalf("expected %d keys after reprojection, got %d", len(channel.Keys), len(reloadedChannel.Keys))
	}

	var preserved *model.ChannelKey
	for i := range reloadedChannel.Keys {
		if reloadedChannel.Keys[i].ChannelKey == firstKey.ChannelKey {
			preserved = &reloadedChannel.Keys[i]
			break
		}
	}
	if preserved == nil {
		t.Fatalf("expected key %q to remain after reprojection", firstKey.ChannelKey)
	}
	if preserved.ID != firstKey.ID {
		t.Fatalf("expected unchanged token to keep key id %d, got %d", firstKey.ID, preserved.ID)
	}
	if preserved.TotalCost != firstKey.TotalCost {
		t.Fatalf("expected unchanged token to preserve total cost %.2f, got %.2f", firstKey.TotalCost, preserved.TotalCost)
	}
}

func TestProjectAccountPreservesManuallyAddedKeyOnResync(t *testing.T) {
	ctx := setupProjectTestDB(t)
	_, account := createProjectionFixture(t, ctx)

	// 首次投影：生成两个 managed key
	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("initial ProjectAccount returned error: %v", err)
	}
	channelsByGroup := loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	channel := channelsByGroup[model.SiteDefaultGroupKey]
	if len(channel.Keys) != 2 {
		t.Fatalf("expected two projected keys, got %d", len(channel.Keys))
	}
	for _, k := range channel.Keys {
		if !k.Managed {
			t.Fatalf("expected projected key %q to be managed=true", k.ChannelKey)
		}
	}

	// 模拟用户手动添加一个 key（Managed=false，走前端 ChannelUpdate 路径）
	manualKey := model.ChannelKeyAddRequest{
		Enabled:    true,
		ChannelKey: "sk-manual-extra",
		Priority:   0,
		Remark:     "manual",
		Managed:    false,
	}
	if _, err := op.ChannelUpdate(&model.ChannelUpdateRequest{
		ID:        channel.ID,
		KeysToAdd: []model.ChannelKeyAddRequest{manualKey},
	}, ctx); err != nil {
		t.Fatalf("ChannelUpdate add manual key failed: %v", err)
	}

	// 再次投影：上游 token 列表未变，手动 key 应保留
	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("second ProjectAccount returned error: %v", err)
	}
	reloaded, err := op.ChannelGet(channel.ID, ctx)
	if err != nil {
		t.Fatalf("ChannelGet failed: %v", err)
	}
	var foundManual bool
	for _, k := range reloaded.Keys {
		if k.ChannelKey == "sk-manual-extra" {
			foundManual = true
			if k.Managed {
				t.Fatalf("expected manual key to remain managed=false, got true")
			}
		}
	}
	if !foundManual {
		t.Fatalf("expected manual key sk-manual-extra to survive resync, keys=%+v", reloaded.Keys)
	}
	if len(reloaded.Keys) != 3 {
		t.Fatalf("expected 3 keys (2 managed + 1 manual) after resync, got %d", len(reloaded.Keys))
	}

	// 上游删除一个 token 后再投影：被删的 managed key 应清除，手动 key 仍保留
	if err := dbpkg.GetDB().WithContext(ctx).Where("token = ?", "key-backup").Delete(&model.SiteToken{}).Error; err != nil {
		t.Fatalf("delete site token failed: %v", err)
	}
	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("third ProjectAccount returned error: %v", err)
	}
	reloaded, err = op.ChannelGet(channel.ID, ctx)
	if err != nil {
		t.Fatalf("ChannelGet after token removal failed: %v", err)
	}
	var foundManual2 bool
	var foundBackup bool
	for _, k := range reloaded.Keys {
		if k.ChannelKey == "sk-manual-extra" {
			foundManual2 = true
		}
		if k.ChannelKey == "sk-key-backup" {
			foundBackup = true
		}
	}
	if !foundManual2 {
		t.Fatalf("expected manual key to survive token removal, keys=%+v", reloaded.Keys)
	}
	if foundBackup {
		t.Fatalf("expected removed managed key sk-key-backup to be deleted, keys=%+v", reloaded.Keys)
	}
	if len(reloaded.Keys) != 2 {
		t.Fatalf("expected 2 keys (1 managed + 1 manual) after token removal, got %d", len(reloaded.Keys))
	}
}

func TestProjectAccountSyncsProjectedModelPrices(t *testing.T) {
	ctx := setupProjectTestDB(t)
	_, account := createProjectionFixture(t, ctx)

	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("ProjectAccount returned error: %v", err)
	}

	if got, err := op.LLMGet("gpt-4o-mini"); err != nil {
		t.Fatalf("expected gpt-4o-mini price to be inserted: %v", err)
	} else if got.Input <= 0 || got.Output <= 0 {
		t.Fatalf("unexpected projected price for gpt-4o-mini: %+v", got)
	}
}

func TestDeleteSiteAccountRemovesManagedChannelChain(t *testing.T) {
	ctx := setupProjectTestDB(t)
	site, account := createProjectionFixture(t, ctx)

	channelIDs, err := ProjectAccount(ctx, account.ID)
	if err != nil {
		t.Fatalf("ProjectAccount returned error: %v", err)
	}
	if len(channelIDs) == 0 {
		t.Fatalf("expected managed channels to be created")
	}

	group := &model.Group{Name: "managed-delete-group", Mode: model.GroupModeFailover}
	if err := op.GroupCreate(group, ctx); err != nil {
		t.Fatalf("GroupCreate failed: %v", err)
	}
	if err := op.GroupItemAdd(&model.GroupItem{
		GroupID:   group.ID,
		ChannelID: channelIDs[0],
		ModelName: "gpt-4o-mini",
		Priority:  1,
		Weight:    1,
	}, ctx); err != nil {
		t.Fatalf("GroupItemAdd failed: %v", err)
	}
	if err := op.StatsChannelUpdate(channelIDs[0], model.StatsMetrics{InputCost: 1, OutputCost: 2, RequestSuccess: 1}); err != nil {
		t.Fatalf("StatsChannelUpdate failed: %v", err)
	}
	if err := op.StatsSaveDB(ctx); err != nil {
		t.Fatalf("StatsSaveDB failed: %v", err)
	}

	if err := DeleteSiteAccount(ctx, account.ID); err != nil {
		t.Fatalf("DeleteSiteAccount returned error: %v", err)
	}

	if _, err := op.SiteGet(site.ID, ctx); err != nil {
		t.Fatalf("site should remain after account deletion: %v", err)
	}
	if _, err := op.SiteAccountGet(account.ID, ctx); err == nil {
		t.Fatalf("expected site account to be deleted")
	}

	var bindingCount int64
	if err := dbpkg.GetDB().WithContext(ctx).Model(&model.SiteChannelBinding{}).Where("site_account_id = ?", account.ID).Count(&bindingCount).Error; err != nil {
		t.Fatalf("count bindings failed: %v", err)
	}
	if bindingCount != 0 {
		t.Fatalf("expected bindings to be deleted, got %d", bindingCount)
	}

	var tokenCount int64
	if err := dbpkg.GetDB().WithContext(ctx).Model(&model.SiteToken{}).Where("site_account_id = ?", account.ID).Count(&tokenCount).Error; err != nil {
		t.Fatalf("count tokens failed: %v", err)
	}
	if tokenCount != 0 {
		t.Fatalf("expected tokens to be deleted, got %d", tokenCount)
	}

	var modelCount int64
	if err := dbpkg.GetDB().WithContext(ctx).Model(&model.SiteModel{}).Where("site_account_id = ?", account.ID).Count(&modelCount).Error; err != nil {
		t.Fatalf("count models failed: %v", err)
	}
	if modelCount != 0 {
		t.Fatalf("expected site models to be deleted, got %d", modelCount)
	}

	for _, channelID := range channelIDs {
		if _, err := op.ChannelGet(channelID, ctx); err == nil {
			t.Fatalf("expected managed channel %d to be deleted", channelID)
		}
		stats := op.StatsChannelGet(channelID)
		if stats.ChannelID != channelID || stats.InputCost != 0 || stats.OutputCost != 0 || stats.RequestSuccess != 0 {
			t.Fatalf("expected in-memory stats for channel %d to be cleared, got %+v", channelID, stats)
		}
		var statsCount int64
		if err := dbpkg.GetDB().WithContext(ctx).Model(&model.StatsChannel{}).Where("channel_id = ?", channelID).Count(&statsCount).Error; err != nil {
			t.Fatalf("count stats failed: %v", err)
		}
		if statsCount != 0 {
			t.Fatalf("expected persisted stats for channel %d to be deleted, got %d", channelID, statsCount)
		}
	}

	items, err := op.GroupItemList(group.ID, ctx)
	if err != nil {
		t.Fatalf("GroupItemList failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected group items referencing managed channels to be deleted, got %d", len(items))
	}
}

func TestDeleteSiteRemovesManagedChannelChainForAllAccounts(t *testing.T) {
	ctx := setupProjectTestDB(t)
	site, primary := createProjectionFixture(t, ctx)

	secondary := &model.SiteAccount{
		SiteID:         site.ID,
		Name:           "Secondary Account",
		CredentialType: model.SiteCredentialTypeAccessToken,
		AccessToken:    "site-access-token-2",
		Enabled:        true,
		AutoSync:       false,
		AutoCheckin:    false,
	}
	if err := op.SiteAccountCreate(secondary, ctx); err != nil {
		t.Fatalf("SiteAccountCreate secondary failed: %v", err)
	}
	if err := dbpkg.GetDB().WithContext(ctx).Create(&[]model.SiteToken{
		{SiteAccountID: secondary.ID, Name: "secondary", Token: "key-secondary", GroupKey: "default", GroupName: "default", Enabled: true},
	}).Error; err != nil {
		t.Fatalf("create secondary site tokens failed: %v", err)
	}
	if err := dbpkg.GetDB().WithContext(ctx).Create(&[]model.SiteModel{
		{SiteAccountID: secondary.ID, GroupKey: model.SiteDefaultGroupKey, ModelName: "gpt-4o-mini", Source: "sync", RouteType: model.SiteModelRouteTypeOpenAIChat, RouteSource: model.SiteModelRouteSourceSyncInferred},
	}).Error; err != nil {
		t.Fatalf("create secondary site models failed: %v", err)
	}

	primaryChannels, err := ProjectAccount(ctx, primary.ID)
	if err != nil {
		t.Fatalf("ProjectAccount primary returned error: %v", err)
	}
	secondaryChannels, err := ProjectAccount(ctx, secondary.ID)
	if err != nil {
		t.Fatalf("ProjectAccount secondary returned error: %v", err)
	}
	channelIDs := append(append([]int{}, primaryChannels...), secondaryChannels...)
	if len(channelIDs) == 0 {
		t.Fatalf("expected managed channels to be created")
	}

	group := &model.Group{Name: "managed-site-delete-group", Mode: model.GroupModeFailover}
	if err := op.GroupCreate(group, ctx); err != nil {
		t.Fatalf("GroupCreate failed: %v", err)
	}
	if err := op.GroupItemAdd(&model.GroupItem{
		GroupID:   group.ID,
		ChannelID: channelIDs[0],
		ModelName: "gpt-4o-mini",
		Priority:  1,
		Weight:    1,
	}, ctx); err != nil {
		t.Fatalf("GroupItemAdd failed: %v", err)
	}
	for _, channelID := range channelIDs {
		if err := op.StatsChannelUpdate(channelID, model.StatsMetrics{InputCost: 1, OutputCost: 2, RequestSuccess: 1}); err != nil {
			t.Fatalf("StatsChannelUpdate failed: %v", err)
		}
	}
	if err := op.StatsSaveDB(ctx); err != nil {
		t.Fatalf("StatsSaveDB failed: %v", err)
	}

	now := time.Now()
	hour := int(now.Unix() / 3600)
	accountIDs := []int{primary.ID, secondary.ID}
	if err := dbpkg.GetDB().WithContext(ctx).Create(&[]model.StatsSiteModelHourly{
		{Hour: hour, SiteAccountID: primary.ID, GroupKey: model.SiteDefaultGroupKey, ModelName: "gpt-4o-mini", Date: now.Format("20060102"), LastRequestAt: now.Unix(), StatsMetrics: model.StatsMetrics{RequestSuccess: 1}},
		{Hour: hour, SiteAccountID: secondary.ID, GroupKey: model.SiteDefaultGroupKey, ModelName: "gpt-4o-mini", Date: now.Format("20060102"), LastRequestAt: now.Unix(), StatsMetrics: model.StatsMetrics{RequestFailed: 1}},
	}).Error; err != nil {
		t.Fatalf("create site model hourly rows failed: %v", err)
	}
	// Pending in-memory hourly stats must not be flushed after the account/site is deleted.
	op.StatsSiteModelHourlyUpdate(channelIDs[0], "gpt-4o-mini", model.StatsMetrics{RequestSuccess: 1})

	if err := DeleteSite(ctx, site.ID); err != nil {
		t.Fatalf("DeleteSite returned error: %v", err)
	}
	if err := op.StatsSiteModelHourlySaveDB(ctx); err != nil {
		t.Fatalf("StatsSiteModelHourlySaveDB after delete failed: %v", err)
	}

	if _, err := op.SiteGet(site.ID, ctx); err == nil {
		t.Fatalf("expected site to be deleted")
	}

	for _, table := range []struct {
		name  string
		model any
		where string
		args  []any
	}{
		{name: "site accounts", model: &model.SiteAccount{}, where: "id IN ?", args: []any{accountIDs}},
		{name: "site tokens", model: &model.SiteToken{}, where: "site_account_id IN ?", args: []any{accountIDs}},
		{name: "site user groups", model: &model.SiteUserGroup{}, where: "site_account_id IN ?", args: []any{accountIDs}},
		{name: "site models", model: &model.SiteModel{}, where: "site_account_id IN ?", args: []any{accountIDs}},
		{name: "site channel bindings", model: &model.SiteChannelBinding{}, where: "site_account_id IN ?", args: []any{accountIDs}},
		{name: "site model hourly stats", model: &model.StatsSiteModelHourly{}, where: "site_account_id IN ?", args: []any{accountIDs}},
	} {
		var count int64
		if err := dbpkg.GetDB().WithContext(ctx).Model(table.model).Where(table.where, table.args...).Count(&count).Error; err != nil {
			t.Fatalf("count %s failed: %v", table.name, err)
		}
		if count != 0 {
			t.Fatalf("expected %s to be deleted, got %d", table.name, count)
		}
	}

	for _, channelID := range channelIDs {
		if _, err := op.ChannelGet(channelID, ctx); err == nil {
			t.Fatalf("expected managed channel %d to be deleted", channelID)
		}
		var statsCount int64
		if err := dbpkg.GetDB().WithContext(ctx).Model(&model.StatsChannel{}).Where("channel_id = ?", channelID).Count(&statsCount).Error; err != nil {
			t.Fatalf("count channel stats failed: %v", err)
		}
		if statsCount != 0 {
			t.Fatalf("expected persisted stats for channel %d to be deleted, got %d", channelID, statsCount)
		}
	}

	items, err := op.GroupItemList(group.ID, ctx)
	if err != nil {
		t.Fatalf("GroupItemList failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected group items referencing managed channels to be deleted, got %d", len(items))
	}
}

func setupProjectTestDB(t *testing.T) context.Context {
	t.Helper()

	if dbpkg.GetDB() != nil {
		_ = dbpkg.Close()
	}

	dbPath := filepath.Join(t.TempDir(), "octopus-project-test.db")
	if err := dbpkg.InitDB("sqlite", dbPath, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() {
		_ = dbpkg.Close()
	})

	return context.Background()
}

func createProjectionFixture(t *testing.T, ctx context.Context) (*model.Site, *model.SiteAccount) {
	t.Helper()

	site := &model.Site{
		Name:     "Projection Site",
		Platform: model.SitePlatformNewAPI,
		BaseURL:  "https://example.com",
		Enabled:  true,
	}
	if err := op.SiteCreate(site, ctx); err != nil {
		t.Fatalf("SiteCreate failed: %v", err)
	}

	account := &model.SiteAccount{
		SiteID:         site.ID,
		Name:           "Primary Account",
		CredentialType: model.SiteCredentialTypeAccessToken,
		AccessToken:    "site-access-token",
		Enabled:        true,
		AutoSync:       false,
		AutoCheckin:    false,
	}
	if err := op.SiteAccountCreate(account, ctx); err != nil {
		t.Fatalf("SiteAccountCreate failed: %v", err)
	}

	tokens := []model.SiteToken{
		{SiteAccountID: account.ID, Name: "primary", Token: "key-primary", GroupKey: "default", GroupName: "default", Enabled: true},
		{SiteAccountID: account.ID, Name: "backup", Token: "key-backup", GroupKey: "default", GroupName: "default", Enabled: true},
	}
	if err := dbpkg.GetDB().WithContext(ctx).Create(&tokens).Error; err != nil {
		t.Fatalf("create site tokens failed: %v", err)
	}

	models := []model.SiteModel{
		{SiteAccountID: account.ID, GroupKey: model.SiteDefaultGroupKey, ModelName: "gpt-4o-mini", Source: "sync", RouteType: model.SiteModelRouteTypeOpenAIChat, RouteSource: model.SiteModelRouteSourceSyncInferred},
		{SiteAccountID: account.ID, GroupKey: model.SiteDefaultGroupKey, ModelName: "claude-3-5-sonnet", Source: "sync", RouteType: model.SiteModelRouteTypeAnthropic, RouteSource: model.SiteModelRouteSourceSyncInferred},
		{SiteAccountID: account.ID, GroupKey: model.SiteDefaultGroupKey, ModelName: "gemini-2.0-flash", Source: "sync", RouteType: model.SiteModelRouteTypeGemini, RouteSource: model.SiteModelRouteSourceSyncInferred},
	}
	if err := dbpkg.GetDB().WithContext(ctx).Create(&models).Error; err != nil {
		t.Fatalf("create site models failed: %v", err)
	}

	return site, account
}

func TestProjectAccountNormalizesProjectedChannelKeys(t *testing.T) {
	ctx := setupProjectTestDB(t)
	_, account := createProjectionFixture(t, ctx)

	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("ProjectAccount failed: %v", err)
	}

	channelsByGroup := loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	channel := channelsByGroup[model.SiteDefaultGroupKey]
	if len(channel.Keys) != 2 {
		t.Fatalf("expected projected channel to carry two keys, got %d", len(channel.Keys))
	}
	if channel.Keys[0].ChannelKey != "sk-key-primary" {
		t.Fatalf("expected first projected key to be normalized, got %q", channel.Keys[0].ChannelKey)
	}
	if channel.Keys[1].ChannelKey != "sk-key-backup" {
		t.Fatalf("expected second projected key to be normalized, got %q", channel.Keys[1].ChannelKey)
	}
}

func TestProjectAccountSkipsMaskedPendingTokens(t *testing.T) {
	ctx := setupProjectTestDB(t)
	_, account := createProjectionFixture(t, ctx)

	if err := dbpkg.GetDB().WithContext(ctx).Model(&model.SiteToken{}).Where("site_account_id = ?", account.ID).Updates(map[string]any{
		"token":        "sk-ab***xyz",
		"value_status": model.SiteTokenValueStatusMaskedPending,
		"enabled":      false,
	}).Error; err != nil {
		t.Fatalf("mark token as masked_pending failed: %v", err)
	}

	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("ProjectAccount failed: %v", err)
	}

	channelsByGroup := loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	channel := channelsByGroup[model.SiteDefaultGroupKey]
	if len(channel.Keys) != 0 {
		t.Fatalf("expected masked_pending tokens not to be projected, got %+v", channel.Keys)
	}
}

func TestProjectAccountSkipsProjectionDisabledGroup(t *testing.T) {
	ctx := setupProjectTestDB(t)
	_, account := createProjectionFixture(t, ctx)

	group := model.SiteUserGroup{SiteAccountID: account.ID, GroupKey: model.SiteDefaultGroupKey, Name: model.SiteDefaultGroupName, ProjectionDisabled: true}
	if err := dbpkg.GetDB().WithContext(ctx).Create(&group).Error; err != nil {
		t.Fatalf("create projection disabled group failed: %v", err)
	}

	managedIDs, err := ProjectAccount(ctx, account.ID)
	if err != nil {
		t.Fatalf("ProjectAccount failed: %v", err)
	}
	if len(managedIDs) != 0 {
		t.Fatalf("expected no managed channels for projection disabled group, got %+v", managedIDs)
	}
	channelsByGroup := loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	if len(channelsByGroup) != 0 {
		t.Fatalf("expected no projected channels for projection disabled group, got %+v", channelsByGroup)
	}
}

func TestProjectAccountRemovesProjectionDisabledManagedChannel(t *testing.T) {
	ctx := setupProjectTestDB(t)
	_, account := createProjectionFixture(t, ctx)

	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("initial ProjectAccount failed: %v", err)
	}
	channelsByGroup := loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	channel := channelsByGroup[model.SiteDefaultGroupKey]
	if channel.ID == 0 {
		t.Fatalf("expected initial projected channel")
	}

	group := model.SiteUserGroup{SiteAccountID: account.ID, GroupKey: model.SiteDefaultGroupKey, Name: model.SiteDefaultGroupName, ProjectionDisabled: true}
	if err := dbpkg.GetDB().WithContext(ctx).Create(&group).Error; err != nil {
		t.Fatalf("create projection disabled group failed: %v", err)
	}
	groupRecord := model.Group{Name: "consumer", MatchRegex: "consumer", Mode: model.GroupModeRoundRobin}
	if err := dbpkg.GetDB().WithContext(ctx).Create(&groupRecord).Error; err != nil {
		t.Fatalf("create group failed: %v", err)
	}
	groupItem := model.GroupItem{GroupID: groupRecord.ID, ChannelID: channel.ID, ModelName: "gpt-4o-mini", Priority: 1, Weight: 1}
	if err := dbpkg.GetDB().WithContext(ctx).Create(&groupItem).Error; err != nil {
		t.Fatalf("create group item failed: %v", err)
	}

	if _, err := ProjectAccount(ctx, account.ID); err != nil {
		t.Fatalf("ProjectAccount after disabling projection failed: %v", err)
	}
	channelsByGroup = loadProjectedChannelsByGroupKey(t, ctx, account.ID)
	if len(channelsByGroup) != 0 {
		t.Fatalf("expected projected channel to be removed, got %+v", channelsByGroup)
	}
	var count int64
	if err := dbpkg.GetDB().WithContext(ctx).Model(&model.GroupItem{}).Where("channel_id = ?", channel.ID).Count(&count).Error; err != nil {
		t.Fatalf("count group items failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected group items for removed projected channel to be deleted, got %d", count)
	}
}

func loadProjectedChannelsByGroupKey(t *testing.T, ctx context.Context, accountID int) map[string]model.Channel {
	t.Helper()

	var bindings []model.SiteChannelBinding
	if err := dbpkg.GetDB().WithContext(ctx).
		Where("site_account_id = ?", accountID).
		Order("group_key ASC").
		Find(&bindings).Error; err != nil {
		t.Fatalf("load site channel bindings failed: %v", err)
	}

	channelsByGroup := make(map[string]model.Channel, len(bindings))
	for _, binding := range bindings {
		var channel model.Channel
		if err := dbpkg.GetDB().WithContext(ctx).
			Preload("Keys").
			First(&channel, binding.ChannelID).Error; err != nil {
			t.Fatalf("load channel %d failed: %v", binding.ChannelID, err)
		}
		// 渠道 Key 密文落库：断言前解密（存量明文原样通过）。
		for i := range channel.Keys {
			if plain, err := crypto.Decrypt(channel.Keys[i].ChannelKey); err == nil {
				channel.Keys[i].ChannelKey = plain
			}
		}
		channelsByGroup[binding.GroupKey] = channel
	}

	return channelsByGroup
}

func assertProjectedChannel(t *testing.T, channelsByGroup map[string]model.Channel, groupKey string, expectedType outbound.OutboundType, expectedModel string, wantSuffix bool) {
	t.Helper()

	channel, ok := channelsByGroup[groupKey]
	if !ok {
		t.Fatalf("expected projected channel for group key %q, got %#v", groupKey, channelsByGroup)
	}
	if channel.Type != expectedType {
		t.Fatalf("expected channel %q type %q, got %q", groupKey, expectedType, channel.Type)
	}
	if channel.Model != expectedModel {
		t.Fatalf("expected channel %q model %q, got %q", groupKey, expectedModel, channel.Model)
	}
	if len(channel.BaseUrls) != 1 || channel.BaseUrls[0].URL != "https://example.com/v1" {
		t.Fatalf("expected channel %q base URL to be projected with /v1 suffix, got %#v", groupKey, channel.BaseUrls)
	}
	if len(channel.Keys) != 2 {
		t.Fatalf("expected channel %q to carry both projected keys, got %d", groupKey, len(channel.Keys))
	}
	expectedNames := map[string]string{
		"default":                   "Projection Site/Primary Account/default-Chat",
		"default::anthropic":        "Projection Site/Primary Account/default-Anthropic",
		"default::gemini":           "Projection Site/Primary Account/default-Gemini",
		"default::volcengine":       "Projection Site/Primary Account/default-Volcengine",
		"default::openai-embedding": "Projection Site/Primary Account/default-Embedding",
	}
	if expectedName, ok := expectedNames[groupKey]; ok && channel.Name != expectedName {
		t.Fatalf("expected channel %q name %q, got %q", groupKey, expectedName, channel.Name)
	}
}
