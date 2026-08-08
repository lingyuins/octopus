package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/errorlog"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

// reportRateLimit 限制单个登录用户的上报频率（每分钟条数），防止
// 刷接口把真实崩溃记录挤出保留窗口。固定窗口 + 惰性过期，量级远小于
// 全局 map 清理阈值，无需周期清理。
const reportRateLimitPerMinute = 60

var reportRateLimits = struct {
	sync.Mutex
	items map[string]struct {
		count int
		at    time.Time
	}
}{items: make(map[string]struct {
	count int
	at    time.Time
})}

func reportRateLimited(key string) bool {
	now := time.Now()
	reportRateLimits.Lock()
	defer reportRateLimits.Unlock()
	entry, ok := reportRateLimits.items[key]
	if !ok || now.Sub(entry.at) >= time.Minute {
		reportRateLimits.items[key] = struct {
			count int
			at    time.Time
		}{count: 1, at: now}
		return false
	}
	entry.count++
	if entry.count > reportRateLimitPerMinute {
		return true
	}
	reportRateLimits.items[key] = entry
	return false
}

func init() {
	// 前端崩溃上报：登录用户即可上报（viewer 也可能触发前端崩溃，不应被写权限拦截）。
	router.NewGroupRouter("/api/v1/error-log").
		Use(middleware.Auth()).
		AddRoute(
			router.NewRoute("/report", http.MethodPost).
				Handle(reportErrorLog),
		).
		AddRoute(
			router.NewRoute("/list", http.MethodGet).
				Use(middleware.RequirePermission(auth.PermLogsRead)).
				Handle(listErrorLog),
		).
		AddRoute(
			router.NewRoute("/detail", http.MethodGet).
				Use(middleware.RequirePermission(auth.PermLogsRead)).
				Handle(errorLogDetail),
		).
		AddRoute(
			router.NewRoute("/clear", http.MethodDelete).
				Use(middleware.RequirePermission(auth.PermLogsWrite)).
				Handle(clearErrorLog),
		)
}

// reportErrorLog 接收前端上报的 JS 错误/未处理 Promise 拒绝/React 渲染错误。
func reportErrorLog(c *gin.Context) {
	// 按登录用户限流，防刷库。
	if reportRateLimited(strconv.Itoa(c.GetInt("user_id"))) {
		resp.Error(c, http.StatusTooManyRequests, resp.ErrTooManyRequests)
		return
	}
	var req struct {
		Level   string `json:"level"`
		Message string `json:"message"`
		Stack   string `json:"stack"`
		PageURL string `json:"page_url"`
		RouteID string `json:"route_id"`
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		// 空 message 没有诊断价值，直接忽略（幂等，不视为错误）。
		resp.Success(c, nil)
		return
	}
	// 按 rune 截断：字节切片会切坏 UTF-8 多字节字符，MySQL/PostgreSQL
	// 会拒绝写入非法编码，导致含中文/emoji 的超长堆栈永远记录不进库。
	message = truncateStr(message, 8192)
	stack := truncateStr(req.Stack, 65536)
	level := strings.TrimSpace(req.Level)
	if level == "" {
		level = "error"
	}
	if level != "panic" && level != "error" && level != "unhandledrejection" && level != "uncaught" {
		level = "error"
	}

	entry := model.ErrorLog{
		Source:    "frontend",
		Level:     level,
		Message:   message,
		Stack:     stack,
		PageURL:   truncateStr(req.PageURL, 1024),
		RouteID:   truncateStr(req.RouteID, 128),
		UserAgent: truncateStr(c.Request.UserAgent(), 512),
		ClientIP:  c.ClientIP(),
		Version:   truncateStr(req.Version, 64),
	}
	// 上报失败不应影响前端主流程，只记录系统日志。
	if err := errorlog.Add(c.Request.Context(), entry); err != nil {
		resp.Error(c, http.StatusInternalServerError, "failed to record error log")
		return
	}
	resp.Success(c, nil)
}

func listErrorLog(c *gin.Context) {
	page, pageSize := parsePagination(c.DefaultQuery("page", "1"), c.DefaultQuery("page_size", "20"))
	filter := errorlog.Filter{Source: c.Query("source"), Level: c.Query("level")}
	if v := c.Query("start_time"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			resp.Error(c, http.StatusBadRequest, "invalid start_time")
			return
		}
		filter.StartTime = &n
	}
	if v := c.Query("end_time"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			resp.Error(c, http.StatusBadRequest, "invalid end_time")
			return
		}
		filter.EndTime = &n
	}

	entries, err := errorlog.List(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, entries)
}

func errorLogDetail(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		resp.Error(c, http.StatusBadRequest, "id is required")
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	entry, err := errorlog.GetByID(c.Request.Context(), id)
	if err != nil {
		resp.InternalError(c)
		return
	}
	if entry == nil {
		resp.Error(c, http.StatusNotFound, "error log not found")
		return
	}
	resp.Success(c, entry)
}

func clearErrorLog(c *gin.Context) {
	if err := errorlog.Clear(c.Request.Context()); err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

func truncateStr(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	// 按 rune 截断，避免按字节截断切坏 UTF-8 多字节字符。
	runes := []rune(s)
	if len(runes) > max {
		runes = runes[:max]
	}
	return string(runes)
}
