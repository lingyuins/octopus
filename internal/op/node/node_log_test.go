package node

import (
	"bytes"
	stdlog "log"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm/logger"
)

func TestNodeCredentialSQLNeverLoggedInDebugMode(t *testing.T) {
	ctx := setup(t)
	var output bytes.Buffer
	conn := db.GetDB()
	old := conn.Logger
	conn.Logger = logger.New(stdlog.New(&output, "", 0), logger.Config{LogLevel: logger.Info})
	t.Cleanup(func() { conn.Logger = old })
	created, err := Create(ctx, "edge", true, 16)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Authenticate(ctx, created.Token); err != nil {
		t.Fatal(err)
	}
	rotated, err := RotateToken(ctx, created.Node.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := List(ctx); err != nil {
		t.Fatal(err)
	}
	enabled := false
	if _, err := Update(ctx, created.Node.ID, nil, &enabled, nil); err != nil {
		t.Fatal(err)
	}
	if err := Delete(ctx, created.Node.ID); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{created.Token, rotated.Token, hashToken(created.Token), hashToken(rotated.Token)} {
		if strings.Contains(output.String(), secret) {
			t.Fatal("node credential logged by debug SQL")
		}
	}
	// Ensure the capture actually sees the parent debug logger; node queries must
	// explicitly override it instead of relying on debug mode being disabled.
	if err := conn.Find(&[]model.Node{}).Error; err != nil {
		t.Fatal(err)
	}
	if output.Len() == 0 {
		t.Fatal("debug SQL logger capture was not active")
	}
}
