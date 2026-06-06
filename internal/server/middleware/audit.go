package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/audit"
	"github.com/lingyuins/octopus/internal/utils/log"
)

const maxAuditTargetBodyBytes = 8 << 10

var auditedManagementWriteRoutes = map[string]struct{}{
	"POST /api/v1/alert/notif/create":                                                {},
	"POST /api/v1/alert/notif/update":                                                {},
	"DELETE /api/v1/alert/notif/delete/:id":                                          {},
	"POST /api/v1/alert/rule/create":                                                 {},
	"POST /api/v1/alert/rule/update":                                                 {},
	"DELETE /api/v1/alert/rule/delete/:id":                                           {},
	"POST /api/v1/apikey/create":                                                     {},
	"POST /api/v1/apikey/update":                                                     {},
	"DELETE /api/v1/apikey/delete/:id":                                               {},
	"POST /api/v1/channel/create":                                                    {},
	"POST /api/v1/channel/enable":                                                    {},
	"POST /api/v1/channel/fetch-model":                                               {},
	"POST /api/v1/channel/sync":                                                      {},
	"POST /api/v1/channel/update":                                                    {},
	"DELETE /api/v1/channel/delete/:id":                                              {},
	"POST /api/v1/channel/group/create":                                              {},
	"POST /api/v1/channel/group/update":                                              {},
	"DELETE /api/v1/channel/group/delete/:id":                                        {},
	"POST /api/v1/group/auto-group":                                                  {},
	"POST /api/v1/group/create":                                                      {},
	"POST /api/v1/group/update":                                                      {},
	"DELETE /api/v1/group/delete-all":                                                {},
	"DELETE /api/v1/group/delete/:id":                                                {},
	"DELETE /api/v1/log/clear":                                                       {},
	"POST /api/v1/model/create":                                                      {},
	"POST /api/v1/model/delete":                                                      {},
	"POST /api/v1/model/update":                                                      {},
	"POST /api/v1/model/update-price":                                                {},
	"POST /api/v1/route/ai-generate":                                                 {},
	"POST /api/v1/setting/database/migrate":                                          {},
	"POST /api/v1/setting/database/test":                                             {},
	"POST /api/v1/setting/import":                                                    {},
	"POST /api/v1/setting/set":                                                       {},
	"POST /api/v1/update":                                                            {},
	"POST /api/v1/user/change-password":                                              {},
	"POST /api/v1/user/change-username":                                              {},
	"POST /api/v1/user/create":                                                       {},
	"POST /api/v1/user/update-role":                                                  {},
	"DELETE /api/v1/user/delete/:id":                                                 {},
	"POST /api/v1/remote-site/create":                                                {},
	"POST /api/v1/remote-site/update":                                                {},
	"DELETE /api/v1/remote-site/delete/:id":                                          {},
	"POST /api/v1/remote-site/refresh/:id":                                           {},
	"POST /api/v1/remote-site/refresh-all":                                           {},
	"POST /api/v1/remote-site/detect":                                                {},
	"POST /api/v1/balance-history/capture/:site_id":                                  {},
	"POST /api/v1/checkin/execute/:site_id":                                          {},
	"POST /api/v1/checkin/execute-all":                                               {},
	"POST /api/v1/api-credential/create":                                             {},
	"POST /api/v1/api-credential/update":                                             {},
	"DELETE /api/v1/api-credential/delete/:id":                                       {},
	"POST /api/v1/verification/run":                                                  {},
	"POST /api/v1/verification/run-for/:id":                                          {},
	"POST /api/v1/cli-export/generate":                                               {},
	"POST /api/v1/channel-migration/migrate":                                         {},
	"POST /api/v1/channel-migration/migrate-all":                                     {},
	"POST /api/v1/announcement/refresh/:site_id":                                     {},
	"POST /api/v1/announcement/refresh-all":                                          {},
	"POST /api/v1/remote-site-token/sync/:site_id":                                   {},
	"POST /api/v1/remote-site-token/sync-to-channel":                                 {},
	"POST /api/v1/redemption/redeem":                                                 {},
	"POST /api/v1/redemption/redeem-all":                                             {},
	"POST /api/v1/usage-history/sync/:site_id":                                       {},
	"POST /api/v1/usage-history/sync-all":                                            {},
	"POST /api/v1/model-mapping":                                                     {},
	"POST /api/v1/model-mapping/create":                                              {},
	"PUT /api/v1/model-mapping/:id":                                                  {},
	"DELETE /api/v1/model-mapping/:id":                                               {},
	"POST /api/v1/backup/webdav/config":                                              {},
	"POST /api/v1/backup/webdav/test":                                                {},
	"POST /api/v1/backup/webdav/backup":                                              {},
	"POST /api/v1/backup/webdav/restore":                                             {},
	"DELETE /api/v1/backup/webdav/delete":                                            {},
	"POST /api/v1/site/account/checkin/:id":                                          {},
	"POST /api/v1/site/account/create":                                               {},
	"POST /api/v1/site/account/sync/:id":                                             {},
	"POST /api/v1/site/account/update":                                               {},
	"POST /api/v1/site/account/enable":                                               {},
	"DELETE /api/v1/site/account/delete/:id":                                         {},
	"POST /api/v1/site/archive/:id":                                                  {},
	"POST /api/v1/site/import/all-api-hub":                                           {},
	"POST /api/v1/site/import/metapi":                                                {},
	"POST /api/v1/site/checkin-all":                                                  {},
	"POST /api/v1/site/create":                                                       {},
	"POST /api/v1/site/sync-all":                                                     {},
	"POST /api/v1/site/update":                                                       {},
	"POST /api/v1/site/enable":                                                       {},
	"POST /api/v1/site/detect":                                                       {},
	"POST /api/v1/site/batch":                                                        {},
	"POST /api/v1/site/restore/:id":                                                  {},
	"DELETE /api/v1/site/delete/:id":                                                 {},
	"POST /api/v1/site-channel/:siteId/account/:accountId/manual-models":             {},
	"POST /api/v1/site-channel/:siteId/account/:accountId/manual-models/delete":      {},
	"POST /api/v1/site-channel/:siteId/account/:accountId/model-routes/reset":        {},
	"POST /api/v1/site-channel/:siteId/account/:accountId/keys":                      {},
	"PUT /api/v1/site-channel/:siteId/account/:accountId/model-routes":               {},
	"PUT /api/v1/site-channel/:siteId/account/:accountId/model-disabled":             {},
	"PUT /api/v1/site-channel/:siteId/account/:accountId/source-keys":                {},
	"PUT /api/v1/site-channel/:siteId/account/:accountId/group-projection":           {},
	"PUT /api/v1/site-channel/:siteId/account/:accountId/projected-channel-settings": {},
	"POST /api/v1/proxy-pool/create":                                                 {},
	"POST /api/v1/proxy-pool/update":                                                 {},
	"POST /api/v1/proxy-pool/test":                                                   {},
	"DELETE /api/v1/proxy-pool/delete/:id":                                           {},
}

func AuditManagementWrite() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isPotentialAuditRequest(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}

		bodyFields := readAuditBodyFields(c)
		c.Next()

		fullPath := c.FullPath()
		if !shouldAuditManagementWrite(c.Request.Method, fullPath) {
			return
		}

		userID := c.GetInt("user_id")
		if userID <= 0 {
			return
		}

		username := c.GetString("username")
		if username == "" {
			username = fmt.Sprintf("user-%d", userID)
		}

		entry := model.AuditLog{
			UserID:     userID,
			Username:   username,
			Action:     buildAuditAction(fullPath),
			Method:     c.Request.Method,
			Path:       c.Request.URL.Path,
			StatusCode: c.Writer.Status(),
			Target:     buildAuditTarget(c, fullPath, bodyFields, username),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := audit.Create(ctx, &entry); err != nil {
			log.Warnf("record audit log failed: %v", err)
		}
	}
}

func isPotentialAuditRequest(method, path string) bool {
	if !strings.HasPrefix(path, "/api/v1/") {
		return false
	}
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func shouldAuditManagementWrite(method, fullPath string) bool {
	if fullPath == "" {
		return false
	}
	_, ok := auditedManagementWriteRoutes[method+" "+fullPath]
	return ok
}

// ShouldAuditManagementWrite is the exported variant of shouldAuditManagementWrite,
// used by tests to verify route audit coverage.
func ShouldAuditManagementWrite(method, fullPath string) bool {
	return shouldAuditManagementWrite(method, fullPath)
}

func readAuditBodyFields(c *gin.Context) map[string]any {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil
	}
	if !strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		return nil
	}
	if c.Request.ContentLength <= 0 || c.Request.ContentLength > maxAuditTargetBodyBytes {
		return nil
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) == 0 {
		return nil
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	return payload
}

func buildAuditAction(fullPath string) string {
	trimmed := strings.Trim(strings.TrimPrefix(fullPath, "/api/v1/"), "/")
	if trimmed == "" {
		return "api"
	}
	parts := strings.Split(trimmed, "/")
	if last := parts[len(parts)-1]; strings.HasPrefix(last, ":") {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, ".")
}

func buildAuditTarget(c *gin.Context, fullPath string, bodyFields map[string]any, username string) string {
	if id := strings.TrimSpace(c.Param("id")); id != "" {
		return "id=" + id
	}

	switch fullPath {
	case "/api/v1/group/delete-all":
		return "all-groups"
	case "/api/v1/log/clear":
		return "relay-logs"
	case "/api/v1/model/update-price":
		return "model-prices"
	case "/api/v1/setting/import":
		return "database-import"
	case "/api/v1/setting/database/migrate":
		return "database-migration"
	case "/api/v1/setting/database/test":
		return "database-test"
	case "/api/v1/update":
		return "self-update"
	case "/api/v1/user/change-password":
		if username != "" {
			return username
		}
	}

	for _, key := range []string{
		"key",
		"name",
		"username",
		"new_username",
		"group_name",
		"model",
		"id",
		"group_id",
		"channel_id",
		"api_key_id",
		"rule_id",
		"notif_channel_id",
	} {
		if value := stringifyAuditTargetValue(bodyFields[key]); value != "" {
			if key == "id" || strings.HasSuffix(key, "_id") {
				return key + "=" + value
			}
			return value
		}
	}

	return ""
}

func stringifyAuditTargetValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}
