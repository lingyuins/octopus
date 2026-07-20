package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/helper"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
	ch "github.com/lingyuins/octopus/internal/op/channel"
	st "github.com/lingyuins/octopus/internal/op/stats"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
	"github.com/lingyuins/octopus/internal/task"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
	"github.com/lingyuins/octopus/internal/utils/log"
)

func init() {
	router.NewGroupRouter("/api/v1/channel").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermChannelsRead)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/list", http.MethodGet).
				Handle(listChannel),
		).
		AddRoute(
			router.NewRoute("/create", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(createChannel),
		).
		AddRoute(
			router.NewRoute("/update", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(updateChannel),
		).
		AddRoute(
			router.NewRoute("/enable", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(enableChannel),
		).
		AddRoute(
			router.NewRoute("/batch-group", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(batchGroupChannel),
		).
		AddRoute(
			router.NewRoute("/delete/:id", http.MethodDelete).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(deleteChannel),
		).
		AddRoute(
			router.NewRoute("/fetch-model", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(fetchModel),
		).
		AddRoute(
			router.NewRoute("/fetch-models-per-key", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(fetchModelsPerKey),
		).
		AddRoute(
			router.NewRoute("/test", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(testChannel),
		).
		AddRoute(
			router.NewRoute("/check-keys/:id", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(checkChannelKeys),
		).
		AddRoute(
			router.NewRoute("/test-model", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(testChannelModel),
		).
		AddRoute(
			router.NewRoute("/group/list", http.MethodGet).
				Handle(listChannelGroup),
		).
		AddRoute(
			router.NewRoute("/group/create", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(createChannelGroup),
		).
		AddRoute(
			router.NewRoute("/group/update", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(updateChannelGroup),
		).
		AddRoute(
			router.NewRoute("/group/delete/:id", http.MethodDelete).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(deleteChannelGroup),
		)
	router.NewGroupRouter("/api/v1/channel").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermChannelsRead)).
		AddRoute(
			router.NewRoute("/sync", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(syncChannel),
		).
		AddRoute(
			router.NewRoute("/last-sync-time", http.MethodGet).
				Handle(getLastSyncTime),
		)
}

func listChannel(c *gin.Context) {
	channels, err := ch.List(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	role := c.GetString("user_role")
	canViewRawKeys := auth.HasPermission(role, auth.PermChannelsWrite)
	if isViewerRole(role) {
		redactChannelBaseURLsForViewer(channels)
	}
	for i, channel := range channels {
		if !canViewRawKeys {
			channels[i].Keys = maskChannelKeys(channel.Keys)
		}
		normalizeChannelListSlices(&channels[i])
		stats := st.ChannelGet(channel.ID)
		channels[i].Stats = &stats
	}
	resp.Success(c, channels)
}

func createChannel(c *gin.Context) {
	var req channelRequestPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	channel := req.toChannel()
	if err := ch.Create(&channel, c.Request.Context()); err != nil {
		if status, msg, ok := classifyChannelMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.InternalError(c)
		return
	}
	stats := st.ChannelGet(channel.ID)
	channel.Stats = &stats
	go func(channel *model.Channel) {
		defer func() {
			if r := recover(); r != nil {
				log.Warnf("post-create channel async task panic recovered: channel_id=%d panic=%v", channel.ID, r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		modelStr := channel.Model + "," + channel.CustomModel
		modelArray := strings.Split(modelStr, ",")
		helper.LLMPriceAddToDB(modelArray, ctx)
		helper.ChannelBaseUrlDelayUpdate(channel, ctx)
		helper.ChannelAutoGroup(channel, ctx)
	}(&channel)
	resp.Success(c, channel)
}

func updateChannel(c *gin.Context) {
	var req model.ChannelUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	channel, err := ch.Update(&req, c.Request.Context())
	if err != nil {
		if status, msg, ok := classifyChannelMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.InternalError(c)
		return
	}
	stats := st.ChannelGet(channel.ID)
	channel.Stats = &stats
	go func(channel *model.Channel) {
		defer func() {
			if r := recover(); r != nil {
				log.Warnf("post-update channel async task panic recovered: channel_id=%d panic=%v", channel.ID, r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		modelStr := channel.Model + "," + channel.CustomModel
		modelArray := strings.Split(modelStr, ",")
		helper.LLMPriceAddToDB(modelArray, ctx)
		helper.ChannelBaseUrlDelayUpdate(channel, ctx)
		helper.ChannelAutoGroup(channel, ctx)
	}(channel)
	resp.Success(c, channel)
}

func enableChannel(c *gin.Context) {
	var request struct {
		ID      int  `json:"id"`
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	if err := ch.Enabled(request.ID, request.Enabled, c.Request.Context()); err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

func batchGroupChannel(c *gin.Context) {
	var req model.ChannelBatchGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	if len(req.IDs) == 0 {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	result, err := ch.BatchUpdateGroup(req.IDs, req.GroupID, c.Request.Context())
	if err != nil {
		if status, msg, ok := classifyChannelMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, result)
}

func deleteChannel(c *gin.Context) {
	id := c.Param("id")
	idNum, err := strconv.Atoi(id)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	if err := ch.Delete(idNum, c.Request.Context()); err != nil {
		resp.InternalError(c)
		return
	}
	st.OnChannelDeleted(idNum)
	resp.Success(c, nil)
}
func fetchModel(c *gin.Context) {
	var payload channelRequestPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	request := payload.toChannel()
	models, err := helper.FetchModels(c.Request.Context(), request)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, models)
}

// fetchModelsPerKey 逐个 key 拉取模型列表，返回每个 key 的模型列表和并集
// 用于诊断同一个 channel 内不同 key 是否拥有不同的模型访问权限
func fetchModelsPerKey(c *gin.Context) {
	var payload channelRequestPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	request := payload.toChannel()
	result, err := helper.FetchModelsPerKey(c.Request.Context(), request)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, result)
}

func testChannel(c *gin.Context) {
	var payload channelRequestPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	request := payload.toChannel()
	summary, err := helper.TestChannel(c.Request.Context(), request)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, summary)
}

func syncChannel(c *gin.Context) {
	task.SyncModelsTask()
	resp.Success(c, nil)
}

// checkChannelKeys 针对已保存的渠道，按其当前的 base_urls × keys 组合做一次连通性
// 探测，返回与 /test 相同的汇总结构。前端据此判断"全部 key 是否都不可用"
// （summary.passed === false 即代表没有任何可用组合）。只读操作，不回写 key 状态。
func checkChannelKeys(c *gin.Context) {
	id := c.Param("id")
	idNum, err := strconv.Atoi(id)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	channel, err := ch.Get(idNum, c.Request.Context())
	if err != nil {
		resp.Error(c, http.StatusNotFound, err.Error())
		return
	}
	summary, err := helper.TestChannel(c.Request.Context(), *channel)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, summary)
}

// testChannelModel 针对已保存的渠道，用指定的模型名 + 端点类型发起一次真实的
// chat/embedding 探测请求（经 outbound adapter 转发，与分组测试同源）。异步执行，
// 返回进度 ID，前端通过 GET /api/v1/group/test/progress/:id 轮询结果。
func testChannelModel(c *gin.Context) {
	var req helper.ChannelModelTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}

	channel, err := ch.Get(req.ChannelID, c.Request.Context())
	if err != nil {
		resp.Error(c, http.StatusNotFound, err.Error())
		return
	}

	// issue #98：部分上游渠道会因低字节请求扣额度/封禁，允许按渠道排除模型可用性测试。
	if channel.SkipModelTest {
		resp.Error(c, http.StatusBadRequest, "该渠道已设置跳过模型可用性测试（issue #98），可在渠道编辑中关闭该开关后重试")
		return
	}

	progress, err := helper.StartChannelModelTest(req, *channel)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, progress)
}

type channelRequestPayload struct {
	ID                   int                         `json:"id"`
	Name                 string                      `json:"name"`
	GroupID              int                         `json:"group_id"`
	Type                 outbound.OutboundType       `json:"type"`
	Enabled              bool                        `json:"enabled"`
	BaseUrls             []model.BaseUrl             `json:"base_urls"`
	Keys                 []channelKeyRequestPayload  `json:"keys"`
	Model                string                      `json:"model"`
	CustomModel          string                      `json:"custom_model"`
	Proxy                bool                        `json:"proxy"`
	AutoSync             bool                        `json:"auto_sync"`
	AutoGroup            model.AutoGroupType         `json:"auto_group"`
	SkipModelTest        bool                        `json:"skip_model_test"`
	Disposable           bool                        `json:"disposable"`
	ExpireAt             *time.Time                  `json:"expire_at,omitempty"`
	NotifChannelID       *int                        `json:"notif_channel_id,omitempty"`
	KeySelectionStrategy string                      `json:"key_selection_strategy"`
	CustomHeader         []model.CustomHeader        `json:"custom_header"`
	ParamOverride        *string                     `json:"param_override"`
	ChannelProxy         *string                     `json:"channel_proxy"`
	RequestRewrite       *model.RequestRewriteConfig `json:"request_rewrite"`
	MatchRegex           *string                     `json:"match_regex"`
	Stats                *model.StatsChannel         `json:"stats"`
}

type channelKeyRequestPayload struct {
	ID               int     `json:"id"`
	ChannelID        int     `json:"channel_id"`
	Enabled          bool    `json:"enabled"`
	ChannelKey       string  `json:"channel_key"`
	StatusCode       int     `json:"status_code"`
	LastUseTimeStamp int64   `json:"last_use_time_stamp"`
	TotalCost        float64 `json:"total_cost"`
	Priority         int     `json:"priority"`
	Remark           string  `json:"remark"`
}

func (p channelRequestPayload) toChannel() model.Channel {
	keys := make([]model.ChannelKey, 0, len(p.Keys))
	for _, key := range p.Keys {
		if strings.TrimSpace(key.ChannelKey) == "" {
			continue
		}
		keys = append(keys, model.ChannelKey{
			Enabled:    key.Enabled,
			ChannelKey: key.ChannelKey,
			Priority:   key.Priority,
			Remark:     key.Remark,
		})
	}

	// 兼容旧前端：proxy + channel_proxy；新前端可直接传 proxy_mode/proxy_config_id。
	// 这里先写入旧字段，Create 内 normalizeChannelProxyFields 会推导 ProxyMode。
	var channelProxy *string
	if p.ChannelProxy != nil {
		trimmed := strings.TrimSpace(*p.ChannelProxy)
		if trimmed != "" {
			channelProxy = &trimmed
		}
	}

	return model.Channel{
		Name:                 p.Name,
		GroupID:              p.GroupID,
		Type:                 p.Type,
		Enabled:              p.Enabled,
		BaseUrls:             p.BaseUrls,
		Keys:                 keys,
		Model:                p.Model,
		CustomModel:          p.CustomModel,
		Proxy:                p.Proxy,
		AutoSync:             p.AutoSync,
		SkipModelTest:        p.SkipModelTest,
		Disposable:           p.Disposable,
		ExpireAt:             p.ExpireAt,
		NotifChannelID:       p.NotifChannelID,
		KeySelectionStrategy: p.KeySelectionStrategy,
		AutoGroup:            p.AutoGroup,
		CustomHeader:         p.CustomHeader,
		ParamOverride:        p.ParamOverride,
		ChannelProxy:         channelProxy,
		RequestRewrite:       p.RequestRewrite,
		MatchRegex:           p.MatchRegex,
	}
}

func listChannelGroup(c *gin.Context) {
	groups, err := op.ChannelGroupList(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, groups)
}

func createChannelGroup(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}

	group, err := op.ChannelGroupCreate(req.Name, c.Request.Context())
	if err != nil {
		if status, msg, ok := classifyChannelMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, group)
}

func updateChannelGroup(c *gin.Context) {
	var req struct {
		ID   int    `json:"id" binding:"required"`
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}

	group, err := op.ChannelGroupUpdate(req.ID, req.Name, c.Request.Context())
	if err != nil {
		if status, msg, ok := classifyChannelMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, group)
}

func deleteChannelGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}

	if err := op.ChannelGroupDelete(id, c.Request.Context()); err != nil {
		if status, msg, ok := classifyChannelMutationError(err); ok {
			resp.Error(c, status, msg)
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

func getLastSyncTime(c *gin.Context) {
	time := task.GetLastSyncModelsTime()
	resp.Success(c, time)
}

func classifyChannelMutationError(err error) (int, string, bool) {
	if err == nil {
		return 0, "", false
	}

	msg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(msg, "channel not found"):
		return http.StatusNotFound, "channel not found", true
	case strings.Contains(msg, "channel group not found"):
		return http.StatusNotFound, "channel group not found", true
	case strings.Contains(msg, "channel group name is required"):
		return http.StatusBadRequest, "channel group name is required", true
	case strings.Contains(msg, "default channel group not found"):
		return http.StatusServiceUnavailable, "default channel group not found", true
	case strings.Contains(msg, "default channel group cannot be deleted"):
		return http.StatusBadRequest, "default channel group cannot be deleted", true
	case strings.Contains(msg, "request rewrite profile is required when enabled"),
		strings.Contains(msg, "unsupported request rewrite profile"),
		strings.Contains(msg, "unsupported tool role strategy"),
		strings.Contains(msg, "unsupported system message strategy"),
		strings.Contains(msg, "request rewrite profile") && strings.Contains(msg, "is not supported for channel type"):
		return http.StatusBadRequest, err.Error(), true
	case strings.Contains(msg, "request_rewrite") &&
		(strings.Contains(msg, "no such column") ||
			strings.Contains(msg, "has no column named") ||
			strings.Contains(msg, "unknown column")):
		return http.StatusServiceUnavailable, "database schema is outdated", true
	case strings.Contains(msg, "unique constraint failed: channels.name"),
		strings.Contains(msg, "duplicate entry") && strings.Contains(msg, "channels.name"):
		return http.StatusConflict, "channel name already exists", true
	case strings.Contains(msg, "unique constraint failed: channel_groups.name"),
		strings.Contains(msg, "duplicate entry") && strings.Contains(msg, "channel_groups.name"):
		return http.StatusConflict, "channel group name already exists", true
	default:
		return 0, "", false
	}
}

func maskChannelKeys(keys []model.ChannelKey) []model.ChannelKey {
	if len(keys) == 0 {
		return nil
	}

	masked := make([]model.ChannelKey, len(keys))
	for i, key := range keys {
		key.ChannelKey = maskChannelKeyValue(key.ChannelKey)
		masked[i] = key
	}
	return masked
}

func maskChannelKeyValue(raw string) string {
	return maskSecretValue(raw)
}

func maskSecretValue(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return strings.Repeat("*", len(trimmed))
	}
	return trimmed[:4] + strings.Repeat("*", len(trimmed)-8) + trimmed[len(trimmed)-4:]
}

func normalizeChannelListSlices(channel *model.Channel) {
	if channel == nil {
		return
	}
	if channel.BaseUrls == nil {
		channel.BaseUrls = make([]model.BaseUrl, 0)
	}
	if channel.Keys == nil {
		channel.Keys = make([]model.ChannelKey, 0)
	}
	if channel.CustomHeader == nil {
		channel.CustomHeader = make([]model.CustomHeader, 0)
	}
}
