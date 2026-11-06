package handlers

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/helper"
	"github.com/lingyuins/octopus/internal/nodetransport"
	"github.com/lingyuins/octopus/internal/op/node"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
	"gorm.io/gorm"
)

func init() {
	registerNodeRoutes(router.NewGroupRouter("/api/v1/nodes"))
	router.NewGroupRouter("/node/v1").AddRoute(router.NewRoute("/connect", http.MethodGet).Handle(connectNode))
	node.NodeRevokedFunc = func(id int) { nodetransport.DefaultManager.Disconnect(id) }
	node.DisconnectAllFunc = func() { nodetransport.DefaultManager.DisconnectAll() }
	nodetransport.FeatureEnabledFunc = node.FeatureEnabled
	node.RuntimeSnapshotFunc = func(id int) node.RuntimeState {
		s := nodetransport.DefaultManager.Snapshot(id)
		return node.RuntimeState{Online: s.Online, InFlight: s.InFlight, MaxConcurrency: s.MaxConcurrency, LastSeenAt: s.LastSeenAt, ConnectorVersion: s.ConnectorVersion, LastError: s.LastError}
	}
}

func registerNodeRoutes(group *router.GroupRouter) {
	group.Use(middleware.Auth()).Use(requireNodesFeature).
		AddRoute(router.NewRoute("", http.MethodGet).Use(middleware.RequirePermission(auth.PermNodesRead)).Handle(listNodes)).
		AddRoute(router.NewRoute("", http.MethodPost).Use(middleware.RequirePermission(auth.PermNodesWrite), middleware.RequireJSON()).Handle(createNode)).
		AddRoute(router.NewRoute("/:id", http.MethodPut).Use(middleware.RequirePermission(auth.PermNodesWrite), middleware.RequireJSON()).Handle(updateNode)).
		AddRoute(router.NewRoute("/:id/rotate-token", http.MethodPost).Use(middleware.RequirePermission(auth.PermNodesWrite), middleware.RequireJSON()).Handle(rotateNodeToken)).
		AddRoute(router.NewRoute("/:id/test", http.MethodPost).Use(middleware.RequirePermission(auth.PermNodesWrite), middleware.RequireJSON()).Handle(testNode)).
		AddRoute(router.NewRoute("/:id", http.MethodDelete).Use(middleware.RequirePermission(auth.PermNodesWrite)).Handle(deleteNode))
}

// requireNodesFeature rejects management traffic when the Magpie nodes feature is
// globally disabled. Keep the JSON envelope; the tunnel endpoint uses plain status.
func requireNodesFeature(c *gin.Context) {
	if !node.FeatureEnabled() {
		resp.ErrorWithKey(c, http.StatusForbidden, "magpie nodes feature is disabled", "node.errors.featureDisabled", nil)
		c.Abort()
		return
	}
	c.Next()
}

func nodeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		resp.ErrorWithKey(c, http.StatusNotFound, "node not found", "node.errors.notFound", nil)
	case errors.Is(err, node.ErrInvalidName):
		resp.InvalidParam(c)
	case errors.Is(err, node.ErrReferenced):
		resp.ErrorWithKey(c, http.StatusConflict, "node is referenced by channels", "node.errors.referenced", nil)
	case errors.Is(err, node.ErrFeatureDisabled):
		resp.ErrorWithKey(c, http.StatusForbidden, "magpie nodes feature is disabled", "node.errors.featureDisabled", nil)
	default:
		resp.InternalError(c)
	}
}

func nodeID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		resp.InvalidParam(c)
		return 0, false
	}
	return id, true
}

func listNodes(c *gin.Context) {
	nodes, err := node.List(c.Request.Context())
	if err != nil {
		nodeError(c, err)
		return
	}
	resp.Success(c, gin.H{"nodes": nodes, "can_write": auth.HasPermission(c.GetString("user_role"), auth.PermNodesWrite)})
}

func createNode(c *gin.Context) {
	var req struct {
		Name           string `json:"name"`
		Enabled        *bool  `json:"enabled"`
		MaxConcurrency *int   `json:"max_concurrency"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.InvalidJSON(c)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	maxConcurrency := 16
	if req.MaxConcurrency != nil {
		maxConcurrency = *req.MaxConcurrency
	}
	result, err := node.Create(c.Request.Context(), req.Name, enabled, maxConcurrency)
	if err != nil {
		nodeError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	resp.Success(c, result)
}

func updateNode(c *gin.Context) {
	id, ok := nodeID(c)
	if !ok {
		return
	}
	var req struct {
		Name           *string `json:"name"`
		Enabled        *bool   `json:"enabled"`
		MaxConcurrency *int    `json:"max_concurrency"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.InvalidJSON(c)
		return
	}
	result, err := node.Update(c.Request.Context(), id, req.Name, req.Enabled, req.MaxConcurrency)
	if err != nil {
		nodeError(c, err)
		return
	}
	resp.Success(c, result)
}

func rotateNodeToken(c *gin.Context) {
	id, ok := nodeID(c)
	if !ok {
		return
	}
	result, err := node.RotateToken(c.Request.Context(), id)
	if err != nil {
		nodeError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	resp.Success(c, result)
}

// Send the upgrade through net/http without Gin's optional WriteHeaderNow:
// Gin 1.11 refuses Hijack after that flush. Gin still owns Hijack and records
// the response as written, so it cannot append a second response afterwards.
type nodeUpgradeWriter struct {
	http.ResponseWriter
	framework gin.ResponseWriter
}

func (w nodeUpgradeWriter) WriteHeader(status int) {
	w.framework.WriteHeader(status)
	if status == http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w nodeUpgradeWriter) Write(body []byte) (int, error) { return w.framework.Write(body) }
func (w nodeUpgradeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.framework.Hijack()
}

// connectNode uses the tunnel protocol, not the management envelope.
// Only a node credential is accepted; JWT and relay keys are unrelated.
func connectNode(c *gin.Context) {
	if !node.FeatureEnabled() {
		c.Status(http.StatusForbidden)
		return
	}
	parts := strings.Fields(c.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		c.Status(http.StatusUnauthorized)
		return
	}
	token := parts[1]
	id, err := node.Authenticate(c.Request.Context(), token)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	raw := http.ResponseWriter(c.Writer)
	for {
		wrapper, ok := raw.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			break
		}
		raw = wrapper.Unwrap()
	}
	// 读取节点配置的并发上限；未配置或非法值回落到 16。
	maxConcurrency := 16
	if n, err := node.Get(c.Request.Context(), id); err == nil && n.Node.MaxConcurrency >= 1 && n.Node.MaxConcurrency <= 32 {
		maxConcurrency = n.Node.MaxConcurrency
	}
	_ = nodetransport.DefaultManager.Accept(c.Request.Context(), id, nodeUpgradeWriter{raw, c.Writer}, c.Request, nodetransport.AcceptOptions{
		MaxConcurrency: maxConcurrency,
		Validate:       func() bool { return node.ValidateCredential(c.Request.Context(), id, token) },
	})
}

func testNode(c *gin.Context) {
	id, ok := nodeID(c)
	if !ok {
		return
	}
	result, err := helper.TestNode(c.Request.Context(), id)
	if err != nil {
		nodeError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	resp.Success(c, result)
}

func deleteNode(c *gin.Context) {
	id, ok := nodeID(c)
	if !ok {
		return
	}
	if err := node.Delete(c.Request.Context(), id); err != nil {
		nodeError(c, err)
		return
	}
	resp.Success(c, nil)
}
