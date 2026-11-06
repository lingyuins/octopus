package node

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func setup(t *testing.T) context.Context {
	t.Helper()
	if err := db.InitDB("sqlite", filepath.Join(t.TempDir(), "nodes.db"), false); err != nil {
		t.Fatal(err)
	}
	old := NodeRevokedFunc
	t.Cleanup(func() { NodeRevokedFunc = old; _ = db.Close() })
	return context.Background()
}

func TestNodeLifecycleAndCredentialRedaction(t *testing.T) {
	ctx := setup(t)
	if _, err := Create(ctx, " ", true, 16); !errors.Is(err, ErrInvalidName) {
		t.Fatal("empty name accepted")
	}
	created, err := Create(ctx, " edge ", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	id := created.Node.ID
	if actual, err := Authenticate(ctx, created.Token); err != nil || actual != id {
		t.Fatalf("authenticate: %v", err)
	}
	var stored model.Node
	if err := db.GetDB().First(&stored, id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.TokenHash == created.Token || len(stored.TokenHash) != 64 || stored.Name != "edge" {
		t.Fatal("invalid stored node")
	}
	listed, err := List(ctx)
	if err != nil || len(listed) != 1 {
		t.Fatalf("list: %v", err)
	}
	encoded, err := json.Marshal(listed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), created.Token) || strings.Contains(string(encoded), stored.TokenHash) || strings.Contains(string(encoded), "token") {
		t.Fatal("list leaked credentials")
	}
	if _, err := Authenticate(ctx, "jwt-token"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("non-node token accepted")
	}
	revoked := 0
	NodeRevokedFunc = func(actual int) {
		if actual != id {
			t.Error("wrong revoked ID")
		}
		revoked++
	}
	rotated, err := RotateToken(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if revoked != 1 || rotated.Token == created.Token {
		t.Fatal("rotation did not revoke")
	}
	if ValidateCredential(ctx, id, created.Token) || !ValidateCredential(ctx, id, rotated.Token) {
		t.Fatal("rotation credential validity")
	}
	enabled := false
	if _, err := Update(ctx, id, nil, &enabled, nil); err != nil {
		t.Fatal(err)
	}
	if ValidateCredential(ctx, id, rotated.Token) {
		t.Fatal("disabled node authenticated")
	}
	enabled = true
	name := "renamed"
	if _, err := Update(ctx, id, &name, &enabled, nil); err != nil {
		t.Fatal(err)
	}
	if !ValidateCredential(ctx, id, rotated.Token) {
		t.Fatal("reenabled node rejected")
	}
	channel := model.Channel{Name: "bound", NodeID: id}
	if err := db.GetDB().Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	if err := Delete(ctx, id); !errors.Is(err, ErrReferenced) {
		t.Fatalf("referenced deletion: %v", err)
	}
	if err := db.GetDB().Delete(&channel).Error; err != nil {
		t.Fatal(err)
	}
	if err := Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ValidateCredential(ctx, id, rotated.Token) {
		t.Fatal("deleted node authenticated")
	}
	if _, err := RotateToken(ctx, id); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("missing rotation accepted")
	}
}

func TestRevocationSerializesWithTransportRegistration(t *testing.T) {
	ctx := setup(t)
	created, err := Create(ctx, "edge", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	// This mirrors Manager.Accept: validation and publication hold managerMu,
	// while revocation commits DB first, then acquires managerMu to disconnect.
	var managerMu sync.Mutex
	online := false
	NodeRevokedFunc = func(int) { managerMu.Lock(); online = false; managerMu.Unlock() }
	for i := 0; i < 30; i++ {
		token := created.Token
		start := make(chan struct{})
		registered := make(chan struct{})
		go func() {
			defer close(registered)
			<-start
			managerMu.Lock()
			defer managerMu.Unlock()
			if ValidateCredential(ctx, created.Node.ID, token) {
				online = true
			}
		}()
		close(start)
		rotated, err := RotateToken(ctx, created.Node.ID)
		if err != nil {
			t.Fatal(err)
		}
		<-registered
		managerMu.Lock()
		staleOnline := online
		managerMu.Unlock()
		if staleOnline || ValidateCredential(ctx, created.Node.ID, token) {
			t.Fatal("revoked session survived rotation")
		}
		created = rotated
	}
}
