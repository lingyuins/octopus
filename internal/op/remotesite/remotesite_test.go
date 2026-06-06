package remotesite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	internaldb "github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

func initTestDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	if err := internaldb.InitDB("sqlite", dbPath, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = internaldb.Close() })
	crypto.Init("test-encryption-key")
}

func TestCreateAndList(t *testing.T) {
	initTestDB(t)
	ctx := context.Background()

	created, err := Create(ctx, &model.RemoteSiteCreateRequest{
		Name:        "test-site",
		BaseURL:     "https://example.com",
		SiteType:    model.SiteTypeNewAPI,
		AccessToken: "my-token",
		Password:    "my-pass",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create returns masked secrets.
	if created.AccessToken != "***" {
		t.Errorf("expected masked access token \"***\", got %q", created.AccessToken)
	}
	if created.Password != "***" {
		t.Errorf("expected masked password \"***\", got %q", created.Password)
	}
	if created.Name != "test-site" {
		t.Errorf("expected name \"test-site\", got %q", created.Name)
	}
	if created.HealthStatus != model.HealthStatusUnknown {
		t.Errorf("expected health status %q, got %q", model.HealthStatusUnknown, created.HealthStatus)
	}
	if created.ExchangeRate != 7.0 {
		t.Errorf("expected default exchange rate 7.0, got %f", created.ExchangeRate)
	}
	if !created.Enabled {
		t.Error("expected enabled to default to true")
	}

	// List should return the created site with masked secrets.
	sites, err := List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sites) != 1 {
		t.Fatalf("expected 1 site, got %d", len(sites))
	}
	if sites[0].AccessToken != "***" {
		t.Errorf("list: expected masked access token \"***\", got %q", sites[0].AccessToken)
	}
	if sites[0].Password != "***" {
		t.Errorf("list: expected masked password \"***\", got %q", sites[0].Password)
	}
	if sites[0].Name != "test-site" {
		t.Errorf("list: expected name \"test-site\", got %q", sites[0].Name)
	}
}

func TestCreateEncryptsSecrets(t *testing.T) {
	initTestDB(t)
	ctx := context.Background()

	_, err := Create(ctx, &model.RemoteSiteCreateRequest{
		Name:        "encrypt-test",
		BaseURL:     "https://example.com",
		SiteType:    model.SiteTypeOctopus,
		AccessToken: "secret-token",
		Password:    "secret-password",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Read directly from DB to verify values are encrypted.
	var site model.RemoteSite
	if err := internaldb.GetDB().First(&site).Error; err != nil {
		t.Fatalf("direct DB read: %v", err)
	}

	if !strings.HasPrefix(site.AccessToken, "enc:") {
		t.Errorf("expected access token to have \"enc:\" prefix, got %q", site.AccessToken)
	}
	if site.AccessToken == "secret-token" {
		t.Error("access token stored as plaintext, expected encrypted")
	}
	if !strings.HasPrefix(site.Password, "enc:") {
		t.Errorf("expected password to have \"enc:\" prefix, got %q", site.Password)
	}
	if site.Password == "secret-password" {
		t.Error("password stored as plaintext, expected encrypted")
	}
}

func TestDelete(t *testing.T) {
	initTestDB(t)
	ctx := context.Background()

	created, err := Create(ctx, &model.RemoteSiteCreateRequest{
		Name:     "delete-me",
		BaseURL:  "https://example.com",
		SiteType: model.SiteTypeNewAPI,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	sites, err := List(ctx)
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(sites) != 0 {
		t.Errorf("expected 0 sites after delete, got %d", len(sites))
	}

	// Verify Get also fails for the deleted ID.
	if _, err := Get(ctx, created.ID); err == nil {
		t.Error("expected error from Get after delete, got nil")
	}
}

func TestListOrderByPinned(t *testing.T) {
	initTestDB(t)
	ctx := context.Background()

	// Create non-pinned site first so it gets a lower ID.
	_, err := Create(ctx, &model.RemoteSiteCreateRequest{
		Name:     "not-pinned",
		BaseURL:  "https://a.example.com",
		SiteType: model.SiteTypeNewAPI,
	})
	if err != nil {
		t.Fatalf("Create non-pinned: %v", err)
	}

	// Create pinned site second (higher ID).
	_, err = Create(ctx, &model.RemoteSiteCreateRequest{
		Name:     "pinned",
		BaseURL:  "https://b.example.com",
		SiteType: model.SiteTypeNewAPI,
	})
	if err != nil {
		t.Fatalf("Create pinned: %v", err)
	}

	// Pin the second site via direct DB update.
	if err := internaldb.GetDB().Model(&model.RemoteSite{}).Where("name = ?", "pinned").Update("pinned", true).Error; err != nil {
		t.Fatalf("pin site: %v", err)
	}

	sites, err := List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sites) != 2 {
		t.Fatalf("expected 2 sites, got %d", len(sites))
	}

	// Pinned site should come first (pinned DESC, sort_order ASC, id ASC).
	if sites[0].Name != "pinned" {
		t.Errorf("expected first site to be \"pinned\", got %q", sites[0].Name)
	}
	if !sites[0].Pinned {
		t.Error("expected first site to have Pinned=true")
	}
	if sites[1].Name != "not-pinned" {
		t.Errorf("expected second site to be \"not-pinned\", got %q", sites[1].Name)
	}
	if sites[1].Pinned {
		t.Error("expected second site to have Pinned=false")
	}
}
