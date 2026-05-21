package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/router"
)

// exemptFromAudit lists management write routes that intentionally skip audit
// logging because they are read-only in practice (tests, probes), authentication
// endpoints (would flood the audit log), or one-time bootstrap operations.
var exemptFromAudit = map[string]string{
	"POST /api/v1/channel/test":         "connectivity probe — no state change",
	"POST /api/v1/user/login":           "authentication — auditing every login would flood the log",
	"POST /api/v1/group/test":           "group routing test — no state change",
	"POST /api/v1/group/test-draft":     "group draft test — no state change",
	"POST /api/v1/bootstrap/create-admin": "one-time first-run bootstrap",
}

// TestAllManagementWriteRoutesAreAudited verifies that every registered
// management write route (POST/PUT/PATCH/DELETE under /api/v1/) has a
// corresponding entry in the audit whitelist. This prevents silently
// missing audit coverage when new write endpoints are added.
func TestAllManagementWriteRoutesAreAudited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	if err := router.RegisterAll(engine); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	routes := engine.Routes()
	missing := []string{}

	for _, r := range routes {
		if !strings.HasPrefix(r.Path, "/api/v1/") {
			continue
		}
		if r.Method != http.MethodPost &&
			r.Method != http.MethodPut &&
			r.Method != http.MethodPatch &&
			r.Method != http.MethodDelete {
			continue
		}

		routeKey := r.Method + " " + r.Path
		if _, exempt := exemptFromAudit[routeKey]; exempt {
			continue
		}
		if !middleware.ShouldAuditManagementWrite(r.Method, r.Path) {
			missing = append(missing, routeKey)
		}
	}

	if len(missing) > 0 {
		t.Errorf("The following management write routes are NOT in the audit whitelist:\n\t%s\n\n"+
			"Add them to auditedManagementWriteRoutes in internal/server/middleware/audit.go, "+
			"or add to exemptFromAudit in this test with a documented reason.",
			strings.Join(missing, "\n\t"))
	}
}
