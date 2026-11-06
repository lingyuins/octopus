package channel

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/nodetransport"
	nodeop "github.com/lingyuins/octopus/internal/op/node"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

func TestNodeChannelCreateUpdateAndDetach(t *testing.T) {
	setupBatchGroupTest(t)
	crypto.Init("octopus-test-encryption-key")
	ctx := context.Background()
	node, err := nodeop.Create(ctx, "local", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	item := model.Channel{Name: "node-channel", GroupID: 1, NodeID: node.Node.ID, Enabled: true, BaseUrls: []model.BaseUrl{{URL: "http://169.254.169.254"}}, Keys: []model.ChannelKey{{Enabled: true, ChannelKey: "do-not-store"}}}
	if err := Create(&item, ctx); err != nil {
		t.Fatal(err)
	}
	if item.NodeID != node.Node.ID || item.GetNormalizedBaseUrl() != nodetransport.BaseURL || len(item.Keys) != 1 || item.Keys[0].ChannelKey != NodeChannelKey {
		t.Fatalf("node channel not normalized: %+v", item)
	}
	if err := nodeop.Delete(ctx, node.Node.ID); !errors.Is(err, nodeop.ErrReferenced) {
		t.Fatalf("referenced node deletion: %v", err)
	}
	name := "new-name"
	updated, err := Update(&model.ChannelUpdateRequest{ID: item.ID, Name: &name}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if updated.NodeID != item.NodeID || updated.Name != name || len(updated.Keys) != 1 {
		t.Fatalf("patch changed transport: %+v", updated)
	}
	poolID := 5
	if _, err := Update(&model.ChannelUpdateRequest{ID: item.ID, PoolID: &poolID}, ctx); !errors.Is(err, ErrInvalidNodeChannel) {
		t.Fatalf("pool accepted: %v", err)
	}
	var persisted model.Channel
	if err := db.GetDB().First(&persisted, item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.PoolID != 0 {
		t.Fatal("failed patch persisted pool")
	}
	zero := 0
	urls := []model.BaseUrl{{URL: "https://api.example.com"}}
	updated, err = Update(&model.ChannelUpdateRequest{ID: item.ID, NodeID: &zero, BaseUrls: &urls, KeysToAdd: []model.ChannelKeyAddRequest{{Enabled: true, ChannelKey: "new-http-key"}}}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if updated.NodeID != 0 || len(updated.Keys) != 1 || updated.Keys[0].ChannelKey != "new-http-key" {
		t.Fatalf("detach retained placeholder: %+v", updated)
	}
	if err := nodeop.Delete(ctx, node.Node.ID); err != nil {
		t.Fatal(err)
	}
}

func TestNodeDetachRejectsPlaceholderDestinationsAndCredentials(t *testing.T) {
	setupBatchGroupTest(t)
	crypto.Init("octopus-test-encryption-key")
	ctx := context.Background()
	created, err := nodeop.Create(ctx, "local", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	item := model.Channel{Name: "bound", GroupID: 1, NodeID: created.Node.ID}
	if err := Create(&item, ctx); err != nil {
		t.Fatal(err)
	}
	zero := 0
	cases := []struct {
		name, url, key string
		enabled        bool
	}{
		{"empty URL", "", "http-key", true},
		{"relative URL", "/v1", "http-key", true},
		{"non HTTP URL", "file:///credentials", "http-key", true},
		{"reserved URL", "https://NODE.OCTOPUS.INVALID./v1", "http-key", true},
		{"URL credentials", "https://user:secret@example.com/v1", "http-key", true},
		{"empty key", "https://example.com/v1", "", true},
		{"whitespace key", "https://example.com/v1", "  ", true},
		{"placeholder key", "https://example.com/v1", NodeChannelKey, true},
		{"disabled key", "https://example.com/v1", "http-key", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			urls := []model.BaseUrl{{URL: tc.url}}
			_, err := Update(&model.ChannelUpdateRequest{ID: item.ID, NodeID: &zero, BaseUrls: &urls, KeysToAdd: []model.ChannelKeyAddRequest{{Enabled: tc.enabled, ChannelKey: tc.key}}}, ctx)
			if !errors.Is(err, ErrInvalidNodeChannel) {
				t.Fatalf("unsafe detach accepted: %v", err)
			}
			stored, err := Get(item.ID, ctx)
			if err != nil {
				t.Fatal(err)
			}
			if stored.NodeID != created.Node.ID || len(stored.Keys) != 1 || stored.Keys[0].ChannelKey != NodeChannelKey {
				t.Fatal("rejected detach mutated binding or keys")
			}
		})
	}
}

func TestNodeChannelOrdinaryPatchPreservesKeysAndCollapsesPlaceholder(t *testing.T) {
	setupBatchGroupTest(t)
	crypto.Init("octopus-test-encryption-key")
	ctx := context.Background()
	httpChannel := model.Channel{Name: "http", GroupID: 1, Keys: []model.ChannelKey{{Enabled: true, ChannelKey: "key-one"}, {Enabled: true, ChannelKey: "key-two"}}}
	if err := Create(&httpChannel, ctx); err != nil {
		t.Fatal(err)
	}
	name := "renamed"
	updated, err := Update(&model.ChannelUpdateRequest{ID: httpChannel.ID, Name: &name}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Keys) != 2 || updated.Keys[0].ID != httpChannel.Keys[0].ID || updated.Keys[1].ID != httpChannel.Keys[1].ID {
		t.Fatal("ordinary HTTP patch lost keys")
	}
	created, err := nodeop.Create(ctx, "local", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	updated, err = Update(&model.ChannelUpdateRequest{ID: httpChannel.ID, NodeID: &created.Node.ID}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Keys) != 1 || updated.Keys[0].ChannelKey != NodeChannelKey {
		t.Fatal("binding did not replace HTTP credentials")
	}
	keyID := updated.Keys[0].ID
	updated, err = Update(&model.ChannelUpdateRequest{ID: httpChannel.ID, Name: &name, KeysToDelete: []int{keyID}, KeysToAdd: []model.ChannelKeyAddRequest{{ChannelKey: "injected"}}}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Keys) != 1 || updated.Keys[0].ID != keyID || !updated.Keys[0].Enabled || updated.Keys[0].ChannelKey != NodeChannelKey {
		t.Fatal("node key edits escaped single placeholder normalization")
	}
}

func TestNodeDeleteAndChannelBindingShareLifecycleLock(t *testing.T) {
	setupBatchGroupTest(t)
	crypto.Init("octopus-test-encryption-key")
	ctx := context.Background()
	for i := 0; i < 40; i++ {
		created, err := nodeop.Create(ctx, "local", true, 16)
		if err != nil {
			t.Fatal(err)
		}
		item := model.Channel{Name: "concurrent", GroupID: 1, NodeID: created.Node.ID}
		bind := func() error { return Create(&item, ctx) }
		if i%2 == 1 {
			item.NodeID = 0
			if err := Create(&item, ctx); err != nil {
				t.Fatal(err)
			}
			bind = func() error {
				_, err := Update(&model.ChannelUpdateRequest{ID: item.ID, NodeID: &created.Node.ID}, ctx)
				return err
			}
		}
		var bindingErr, deleteErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; bindingErr = bind() }()
		go func() { defer wg.Done(); <-start; deleteErr = nodeop.Delete(ctx, created.Node.ID) }()
		close(start)
		wg.Wait()
		if bindingErr == nil {
			if !errors.Is(deleteErr, nodeop.ErrReferenced) {
				t.Fatalf("bound node deleted: %v", deleteErr)
			}
			if err := Delete(item.ID, ctx); err != nil {
				t.Fatal(err)
			}
			if err := nodeop.Delete(ctx, created.Node.ID); err != nil {
				t.Fatal(err)
			}
		} else if deleteErr != nil || !errors.Is(bindingErr, ErrInvalidNodeChannel) {
			t.Fatalf("unexpected concurrent result: bind=%v delete=%v", bindingErr, deleteErr)
		} else if i%2 == 1 {
			if err := Delete(item.ID, ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestNodeChannelRejectsMissingNodeAndProxy(t *testing.T) {
	setupBatchGroupTest(t)
	crypto.Init("octopus-test-encryption-key")
	ctx := context.Background()
	missing := model.Channel{Name: "missing", GroupID: 1, NodeID: 999}
	if err := Create(&missing, ctx); !errors.Is(err, ErrInvalidNodeChannel) {
		t.Fatalf("missing node accepted: %v", err)
	}
	node, err := nodeop.Create(ctx, "local", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	proxied := model.Channel{Name: "proxy", GroupID: 1, NodeID: node.Node.ID, Proxy: true}
	if err := Create(&proxied, ctx); !errors.Is(err, ErrInvalidNodeChannel) {
		t.Fatalf("proxy accepted: %v", err)
	}
}
