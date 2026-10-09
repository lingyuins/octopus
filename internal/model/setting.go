package model

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SettingKey string

const (
	SettingKeyProxyURL                           SettingKey = "proxy_url"
	SettingKeyStatsSaveInterval                  SettingKey = "stats_save_interval"                     // 将统计信息写入数据库的周期(分钟)
	SettingKeyModelInfoUpdateInterval            SettingKey = "model_info_update_interval"              // 模型信息更新间隔(小时)
	SettingKeySyncLLMInterval                    SettingKey = "sync_llm_interval"                       // LLM 同步间隔(小时)
	SettingKeyRelayLogKeepPeriod                 SettingKey = "relay_log_keep_period"                   // 日志保存时间范围(天)
	SettingKeyRelayLogKeepCount                  SettingKey = "relay_log_keep_count"                    // 日志保留条数(0=不按条数)
	SettingKeyRelayLogKeepEnabled                SettingKey = "relay_log_keep_enabled"                  // 是否保留历史日志
	SettingKeyRelayLogContentEnabled             SettingKey = "relay_log_content_enabled"               // 是否记录请求/响应内容大字段（关闭可大幅降低写入量与磁盘 IO）
	SettingKeyRelayLogQueueDropPolicy            SettingKey = "relay_log_queue_drop_policy"             // 日志队列满时的丢弃策略：disabled(阻塞触发刷盘) | oldest(丢弃最旧) | newest(丢弃最新)
	SettingKeyStreamSessionReplayEnabled         SettingKey = "stream_session_replay_enabled"           // 是否保留完成会话的缓冲区以支持断线重连重放（关闭可降低内存占用）
	SettingKeyCORSAllowOrigins                   SettingKey = "cors_allow_origins"                      // 跨域白名单(逗号分隔, 如 "example.com,example2.com"). 为空不允许跨域, "*"允许所有
	SettingKeyRelayRetryCount                    SettingKey = "relay_retry_count"                       // 单个候选渠道内 Key 级最大重试次数
	SettingKeyRelayRouteRetries                  SettingKey = "relay_route_retries"                     // 路由级最大重试次数（全部渠道遍历一轮算一次）
	SettingKeyCircuitBreakerThreshold            SettingKey = "circuit_breaker_threshold"               // 熔断触发阈值（连续失败次数）
	SettingKeyCircuitBreakerCooldown             SettingKey = "circuit_breaker_cooldown"                // 熔断基础冷却时间（秒）
	SettingKeyCircuitBreakerMaxCooldown          SettingKey = "circuit_breaker_max_cooldown"            // 熔断最大冷却时间（秒），指数退避上限
	SettingKeyCircuitBreakerHalfOpenProbeTimeout SettingKey = "circuit_breaker_half_open_probe_timeout" // 熔断 HalfOpen 探测超时（秒），0=禁用；试探请求被中途放弃时避免永久跳过（issue #162）
	SettingKeyPublicAPIBaseURL                   SettingKey = "public_api_base_url"                     // 对外可访问的 API 基础地址，用于生成示例
	SettingKeyAlertNotifyLanguage                SettingKey = "alert_notify_language"                   // 告警通知发送语言
	SettingKeyRatelimitCooldown                  SettingKey = "ratelimit_cooldown"                      // Key 错误冷却时间（秒），0=关闭
	SettingKeyKeySelectionStrategy               SettingKey = "key_selection_strategy"                  // Key 选择策略：cost(默认) | availability | priority
	SettingKeyRelayMaxTotalAttempts              SettingKey = "relay_max_total_attempts"                // 所有候选渠道的最大决策纪录次数，0/负数回退到内置默认上限（issue #192）
	SettingKeyRetryEmptyOutput                   SettingKey = "retry_empty_output"                      // 输出为空(无可见内容)时自动重试，流式与非流式均适用（issue #106/#155）
	SettingKeyReasoningBufferStrategy            SettingKey = "reasoning_buffer_strategy"               // 推理内容缓冲策略：buffer(缓冲) | immediate(立即)，默认 immediate
	SettingKeyRateLimitHoldEnabled               SettingKey = "rate_limit_hold_enabled"                 // 429 限流时是否在当前渠道内延时重试（默认关闭，保持立即换 Key/渠道）
	SettingKeyRateLimitHoldInterval              SettingKey = "rate_limit_hold_interval"                // 429 渠道内延时重试间隔（秒）
	SettingKeyRateLimitHoldMaxWait               SettingKey = "rate_limit_hold_max_wait"                // 429 渠道内延时重试总等待上限（秒），超时后才换下一渠道

	SettingKeyPoolTokenRefreshInterval             SettingKey = "pool_token_refresh_interval"              // 号池 OAuth token 刷新检查间隔（分钟）
	SettingKeyPoolQuotaSyncInterval                SettingKey = "pool_quota_sync_interval"                 // 号池额度同步间隔（分钟）
	SettingKeyPlanProviderRefreshInterval          SettingKey = "plan_provider_refresh_interval"           // 额度监控自动刷新默认间隔（分钟）
	SettingKeyPoolMinPriority                      SettingKey = "pool_min_priority"                        // 号池分层过滤 minPriority 阈值（默认 -9999 表示关闭）
	SettingKeyPoolLayeredFilterEnabled             SettingKey = "pool_layered_filter_enabled"              // 号池分层过滤开关：开启后 SelectAccount 过滤掉 priority < min_priority 的候选
	SettingKeyPoolStickyEscapeEnabled              SettingKey = "pool_sticky_escape_enabled"               // Pool sticky escape switch: temporarily bypass the sticky binding when the account's EWMA stats degrade (off by default)
	SettingKeyPoolStickyEscapeErrorRate            SettingKey = "pool_sticky_escape_error_rate"            // Pool sticky escape error-rate threshold (escape when EWMA errorRate exceeds it)
	SettingKeyPoolStickyEscapeTTFTMs               SettingKey = "pool_sticky_escape_ttft_ms"               // Pool sticky escape TTFT threshold (ms; escape when EWMA TTFT exceeds it; 0 disables the dimension)
	SettingKeyPoolSchedulerWeightReset             SettingKey = "pool_scheduler_weight_reset"              // Pool EWMA factor weight: reset inverse-readiness factor (default 0 = off, no behavior change)
	SettingKeyPoolSchedulerWeightQuota             SettingKey = "pool_scheduler_weight_quota"              // Pool EWMA factor weight: quota-headroom factor (default 0 = off, no behavior change)
	SettingKeyPoolGeminiClientSecret               SettingKey = "pool_gemini_client_secret"                // Gemini OAuth client secret override for the built-in client (empty = env -> built-in public credential)
	SettingKeyPoolHealthCheckEnabled               SettingKey = "pool_health_check_enabled"                // 号池账号健康巡检开关
	SettingKeyPoolHealthCheckInterval              SettingKey = "pool_health_check_interval_minutes"       // 号池账号健康巡检间隔（分钟）
	SettingKeyPoolHealthCheckFailThreshold         SettingKey = "pool_health_check_fail_threshold"         // 号池账号健康巡检失败阈值（连续 N 次后 SetError）
	SettingKeyAutoStrategyMinSamples               SettingKey = "auto_strategy_min_samples"                // Auto策略最小样本数阈值
	SettingKeyAutoStrategyTimeWindow               SettingKey = "auto_strategy_time_window"                // Auto策略时间窗口（秒）
	SettingKeyAutoStrategySampleThreshold          SettingKey = "auto_strategy_sample_threshold"           // Auto策略滑动窗口大小
	SettingKeyAutoStrategyLatencyWeight            SettingKey = "auto_strategy_latency_weight"             // Auto策略延迟权重（0-100）
	SettingKeyAutoStrategyTTFTWeight               SettingKey = "auto_strategy_ttft_weight"                // Auto策略TTFT权重（0-100），启用后流式评分用TTFT EMA替代总延迟EMA
	SettingKeyAutoStrategyPriceWeight              SettingKey = "auto_strategy_price_weight"               // Auto策略成本权重（0-100），启用后按渠道模型单价对评分降权
	SettingKeyAutoStrategyExploreRate              SettingKey = "auto_strategy_explore_rate"               // Auto策略探索概率（0-100），>0时按评分softmax随机化候选顺序
	SettingKeyAutoStrategyBucketTolerance          SettingKey = "auto_strategy_bucket_tolerance"           // Auto策略同分桶容差（0-100评分点），桶内随机打散避免轮流垄断
	SettingKeySemanticCacheEnabled                 SettingKey = "semantic_cache_enabled"                   // 语义缓存开关
	SettingKeySemanticCacheTTL                     SettingKey = "semantic_cache_ttl"                       // 语义缓存 TTL（秒）
	SettingKeySemanticCacheThreshold               SettingKey = "semantic_cache_threshold"                 // 语义缓存相似度阈值（0-1）
	SettingKeySemanticCacheMaxEntries              SettingKey = "semantic_cache_max_entries"               // 语义缓存最大条目数
	SettingKeySemanticCacheEmbeddingBaseURL        SettingKey = "semantic_cache_embedding_base_url"        // 语义缓存 embedding 服务 Base URL
	SettingKeySemanticCacheEmbeddingAPIKey         SettingKey = "semantic_cache_embedding_api_key"         // 语义缓存 embedding 服务 API Key
	SettingKeySemanticCacheEmbeddingModel          SettingKey = "semantic_cache_embedding_model"           // 语义缓存 embedding 模型名称
	SettingKeySemanticCacheEmbeddingTimeoutSeconds SettingKey = "semantic_cache_embedding_timeout_seconds" // 语义缓存 embedding 请求超时（秒）
	SettingKeyNavOrder                             SettingKey = "nav_order"                                // 顶级页面顺序(JSON)
	SettingKeyNavVisible                           SettingKey = "nav_visible"                              // 顶级页面显示状态(JSON)
	SettingKeyHubTabOrder                          SettingKey = "hub_tab_order"                            // Hub 子标签顺序(JSON)
	SettingKeyHubTabVisible                        SettingKey = "hub_tab_visible"                          // Hub 子标签可见性(JSON)
	SettingKeyAnalyticsTabOrder                    SettingKey = "analytics_tab_order"                      // 分析中心子标签顺序(JSON)
	SettingKeyAnalyticsTabVisible                  SettingKey = "analytics_tab_visible"                    // 分析中心子标签可见性(JSON)
	SettingKeyOpsTabOrder                          SettingKey = "ops_tab_order"                            // 运维中心子标签顺序(JSON)
	SettingKeyOpsTabVisible                        SettingKey = "ops_tab_visible"                          // 运维中心子标签可见性(JSON)
	SettingKeyAIRouteGroupID                       SettingKey = "ai_route_group_id"                        // AI路由目标分组 ID
	SettingKeyAIRouteBaseURL                       SettingKey = "ai_route_base_url"                        // AI路由分析服务 Base URL
	SettingKeyAIRouteAPIKey                        SettingKey = "ai_route_api_key"                         // AI路由分析服务 API Key
	SettingKeyAIRouteModel                         SettingKey = "ai_route_model"                           // AI路由分析模型名称
	SettingKeyAIRouteTimeoutSeconds                SettingKey = "ai_route_timeout_seconds"                 // AI路由分析单次请求超时（秒）
	SettingKeyAIRouteParallelism                   SettingKey = "ai_route_parallelism"                     // AI路由分析批次最大并发数
	SettingKeyAIRouteServices                      SettingKey = "ai_route_services"                        // AI路由分析服务池(JSON)
	SettingKeyAIRouteMaxModelsPerRequest           SettingKey = "ai_route_max_models_per_request"          // AI路由分析单批次最大模型数，超过按模型家族切分批次
	SettingKeyStatsTimezone                        SettingKey = "stats_timezone"                           // 统计时区（IANA 名，如 Asia/Shanghai）；空串回退到 stats_timezone_offset
	SettingKeyStatsTimezoneOffset                  SettingKey = "stats_timezone_offset"                    // [已弃用] 统计时区偏移（小时），整型；stats_timezone 为空时回退使用
	SettingKeyJWTDefaultExpiryMinutes              SettingKey = "jwt_default_expiry_minutes"               // 默认JWT过期时间（分钟）
	SettingKeyJWTRememberMeExpiryDays              SettingKey = "jwt_remember_me_expiry_days"              // 记住我JWT过期时间（天）
	SettingKeyLoginRateLimitWindow                 SettingKey = "login_rate_limit_window"                  // 登录限流时间窗口（分钟）
	SettingKeyLoginRateLimitMaxFailed              SettingKey = "login_rate_limit_max_failed"              // 登录限流最大失败次数
	SettingKeyStreamSessionTTLMinutes              SettingKey = "stream_session_ttl_minutes"               // 流会话TTL（分钟）
	SettingKeyStreamSessionMaxEvents               SettingKey = "stream_session_max_events"                // 流会话最大事件数
	SettingKeyStreamSessionMaxBytesMB              SettingKey = "stream_session_max_bytes_mb"              // 流会话最大字节数（MB）
	SettingKeyStreamSessionMaxSessions             SettingKey = "stream_session_max_sessions"              // 流会话全局并发上限（超限驱逐最旧会话；内存上限≈本值×最大字节数）
	SettingKeyNotifyHTTPTimeoutSeconds             SettingKey = "notify_http_timeout_seconds"              // 通知HTTP请求超时（秒）
	SettingKeyFailureHintTTLUnauthorized           SettingKey = "failure_hint_ttl_unauthorized"            // 认证失败提示缓存TTL（秒）
	SettingKeyFailureHintTTLRateLimit              SettingKey = "failure_hint_ttl_rate_limit"              // 限流失败提示缓存TTL（秒）
	SettingKeyFailureHintTTLNetwork                SettingKey = "failure_hint_ttl_network"                 // 网络失败提示缓存TTL（秒）
	SettingKeyWebDAVConfig                         SettingKey = "webdav_config"                            // WebDAV 云备份配置（JSON）
	SettingKeySiteSyncInterval                     SettingKey = "site_sync_interval"                       // 站点账号同步间隔（小时）
	SettingKeySiteCheckinInterval                  SettingKey = "site_checkin_interval"                    // 站点自动签到间隔（小时）
	SettingKeyStatsSiteModelBackfilled             SettingKey = "stats_site_model_backfilled"              // 站点模型统计回填标记
	SettingKeyProjectedChannelAutoGroupEnabled     SettingKey = "projected_channel_auto_group_enabled"     // 站点投影渠道自动分组全局开关
	SettingKeyResponseFilterEnabled                SettingKey = "response_filter_enabled"                  // 输出结果关键词拦截开关
	SettingKeyResponseFilterKeywords               SettingKey = "response_filter_keywords"                 // 拦截关键词列表(JSON 数组)
	SettingKeyResponseFilterAction                 SettingKey = "response_filter_action"                   // 拦截动作: block(阻断) / replace(替换为*)
	SettingKeyResponseFilterErrorMessage           SettingKey = "response_filter_error_message"            // 阻断时返回的错误信息
	SettingKeyLogLevel                             SettingKey = "log_level"                                // 应用日志级别: debug, info, warn, error
	SettingKeyLogExcludedGroups                    SettingKey = "log_excluded_groups"                      // 在日志列表/实时流中屏蔽的分组名称列表(JSON 数组)
	SettingKeyModelNormalizeRouterPrefixes         SettingKey = "model_normalize_router_prefixes"          // 模型名归一化: 路由商/平台前缀列表(JSON 数组，元素如 "dmxapi-")
	SettingKeyModelNormalizeFunctionalSuffixes     SettingKey = "model_normalize_functional_suffixes"      // 模型名归一化: 功能性后缀列表(JSON 数组，元素如 "-cc")
	SettingKeyModelNormalizeExplicitMappings       SettingKey = "model_normalize_explicit_mappings"        // 模型名归一化: 显式变体→基准名映射(JSON 数组，元素如 {"variant":"...","canonical":"..."})
	SettingKeyModelNormalizeMarketDedupeDefault    SettingKey = "model_normalize_market_dedupe_default"    // 模型名归一化: 模型广场默认开启归一化去重("true"/"false")
	SettingKeyWebAuthnRPID                         SettingKey = "webauthn_rp_id"                           // WebAuthn RP ID（域名，不含协议/端口）
	SettingKeyWebAuthnRPName                       SettingKey = "webauthn_rp_name"                         // WebAuthn RP 展示名
	SettingKeyWebAuthnOrigins                      SettingKey = "webauthn_origins"                         // WebAuthn 允许的 Origin 列表（逗号分隔，完整 scheme://host[:port]）
	SettingKeyTrustedProxies                       SettingKey = "trusted_proxies"                          // 可信反向代理 CIDR/IP 列表（逗号分隔，解析 X-Forwarded-For 取真实客户端 IP）；空=不信任任何代理，*=信任所有（有风险）；需重启生效
	SettingKeyKeyHealthCheckEnabled                SettingKey = "key_health_check_enabled"                 // 定时 Key 可用性验证开关（issue #142）
	SettingKeyKeyHealthCheckInterval               SettingKey = "key_health_check_interval"                // 定时 Key 验证间隔（分钟）
	SettingKeyKeyHealthCheckFailThreshold          SettingKey = "key_health_check_fail_threshold"          // 连续失败多少次后标记异常
	SettingKeyKeyHealthCheckNotifyEnabled          SettingKey = "key_health_check_notify_enabled"          // 是否发送 Key 验证失败通知
	SettingKeyKeyHealthCheckRecoveryNotify         SettingKey = "key_health_check_recovery_notify"         // 是否发送 Key 验证恢复通知
	SettingKeyKeyHealthCheckNotifyCooldown         SettingKey = "key_health_check_notify_cooldown"         // Key 验证通知冷却时间（秒）
	SettingKeyGroupUpstreamMetaDisplayEnabled      SettingKey = "group_upstream_meta_display_enabled"      // 分组编辑页展示上游价/余额/今日收入/性能指标
)

type Setting struct {
	Key   SettingKey `json:"key" gorm:"primaryKey"`
	Value string     `json:"value" gorm:"not null"`
}

func DefaultSettings() []Setting {
	return []Setting{
		{Key: SettingKeyProxyURL, Value: ""},
		{Key: SettingKeyStatsSaveInterval, Value: "10"},                  // 默认10分钟保存一次统计信息
		{Key: SettingKeyCORSAllowOrigins, Value: ""},                     // CORS 默认不允许跨域，设置为 "*" 才允许所有来源
		{Key: SettingKeyModelInfoUpdateInterval, Value: "24"},            // 默认24小时更新一次模型信息
		{Key: SettingKeySyncLLMInterval, Value: "24"},                    // 默认24小时同步一次LLM
		{Key: SettingKeyRelayLogKeepPeriod, Value: "7"},                  // 默认日志保存7天
		{Key: SettingKeyRelayLogKeepCount, Value: "0"},                   // 默认不按条数保留(0=禁用)
		{Key: SettingKeyRelayLogContentEnabled, Value: "true"},           // 默认记录请求/响应内容，保持兼容；高负载可关闭以降低 IO
		{Key: SettingKeyRelayLogQueueDropPolicy, Value: "oldest"},        // 默认丢弃最旧日志，防止队列溢出 OOM（高 QPS 下推荐）
		{Key: SettingKeyStreamSessionReplayEnabled, Value: "true"},       // 默认启用重连重放，保持兼容；内存受限可关闭
		{Key: SettingKeyRelayLogKeepEnabled, Value: "true"},              // 默认保留历史日志
		{Key: SettingKeyRelayRetryCount, Value: "3"},                     // 默认单个渠道内 Key 级重试3次
		{Key: SettingKeyRelayRouteRetries, Value: "2"},                   // 默认路由级重试2次（全部渠道遍历两轮）
		{Key: SettingKeyCircuitBreakerThreshold, Value: "5"},             // 默认连续失败5次触发熔断
		{Key: SettingKeyCircuitBreakerCooldown, Value: "60"},             // 默认熔断基础冷却60秒，指数退避的基数（与 balancer/circuit.go 的兜底值一致）
		{Key: SettingKeyCircuitBreakerMaxCooldown, Value: "600"},         // 默认最大冷却600秒（10分钟）
		{Key: SettingKeyCircuitBreakerHalfOpenProbeTimeout, Value: "60"}, // 默认 HalfOpen 探测超时60秒；试探被中途放弃后避免永久跳过（issue #162）
		{Key: SettingKeyRatelimitCooldown, Value: "300"},                 // 默认 Key 错误冷却300秒（5分钟），0=关闭
		{Key: SettingKeyKeySelectionStrategy, Value: "cost"},             // 默认 Key 选择策略：成本最低优先；可选 availability（可用度优先）
		{Key: SettingKeyRelayMaxTotalAttempts, Value: "0"},               // 0 回退到内置默认上限（issue #192 防止 attempts 无限膨胀）
		{Key: SettingKeyRetryEmptyOutput, Value: "true"},                 // 默认启用空输出重试
		{Key: SettingKeyReasoningBufferStrategy, Value: "immediate"},     // 推理内容立即发送；已发送后不能安全重试空输出
		{Key: SettingKeyRateLimitHoldEnabled, Value: "false"},            // 默认关闭：429 仍立即换 Key/渠道
		{Key: SettingKeyRateLimitHoldInterval, Value: "10"},              // 默认每 10 秒重试一次
		{Key: SettingKeyRateLimitHoldMaxWait, Value: "60"},               // 默认最多坚持 60 秒

		{Key: SettingKeyPublicAPIBaseURL, Value: ""},
		{Key: SettingKeyAlertNotifyLanguage, Value: "en"},
		{Key: SettingKeyAutoStrategyMinSamples, Value: "10"},       // 默认最小样本数10次
		{Key: SettingKeyAutoStrategyTimeWindow, Value: "300"},      // 默认时间窗口300秒（5分钟）
		{Key: SettingKeyAutoStrategySampleThreshold, Value: "100"}, // 默认滑动窗口大小100条
		{Key: SettingKeyAutoStrategyLatencyWeight, Value: "30"},    // 默认延迟权重30%
		{Key: SettingKeyAutoStrategyTTFTWeight, Value: "0"},        // 默认0=关闭，TTFT EMA有样本后可设50切换延迟评分输入
		{Key: SettingKeyAutoStrategyPriceWeight, Value: "0"},       // 默认0=关闭，价格目录有数据后可开启省钱降权
		{Key: SettingKeyAutoStrategyExploreRate, Value: "0"},       // 默认0=纯贪心（保持旧行为），>0启用softmax探索
		{Key: SettingKeyAutoStrategyBucketTolerance, Value: "5"},   // 默认5评分点容差，桶内随机打散
		{Key: SettingKeySemanticCacheEnabled, Value: "false"},      // 默认关闭语义缓存
		{Key: SettingKeySemanticCacheTTL, Value: "3600"},           // 默认TTL 1小时
		{Key: SettingKeySemanticCacheThreshold, Value: "98"},       // 默认相似度阈值 0.98（0-100）
		{Key: SettingKeySemanticCacheMaxEntries, Value: "1000"},    // 默认最大1000条
		{Key: SettingKeySemanticCacheEmbeddingBaseURL, Value: ""},
		{Key: SettingKeySemanticCacheEmbeddingAPIKey, Value: ""},
		{Key: SettingKeySemanticCacheEmbeddingModel, Value: ""},
		{Key: SettingKeySemanticCacheEmbeddingTimeoutSeconds, Value: "10"},
		{Key: SettingKeyNavOrder, Value: `["home","hub","channel","pool","group","model","analytics","log","notification","ops","apikey","setting","user"]`},
		{Key: SettingKeyNavVisible, Value: `["home","hub","channel","pool","group","model","analytics","log","notification","ops","apikey","setting","user"]`},
		{Key: SettingKeyHubTabOrder, Value: `["sites","site-channels","automation","balance","tokenplan"]`},
		{Key: SettingKeyHubTabVisible, Value: `["sites","site-channels","automation","balance","tokenplan"]`},
		{Key: SettingKeyAnalyticsTabOrder, Value: `["cache","utilization","route-health","channel-model","evaluation","latency"]`},
		{Key: SettingKeyAnalyticsTabVisible, Value: `["cache","utilization","route-health","channel-model","evaluation","latency"]`},
		{Key: SettingKeyOpsTabOrder, Value: `["telemetry","quota","health","maintenance","system","audit"]`},
		{Key: SettingKeyOpsTabVisible, Value: `["telemetry","quota","health","maintenance","system","audit"]`},
		{Key: SettingKeyAIRouteGroupID, Value: "0"},
		{Key: SettingKeyAIRouteBaseURL, Value: ""},
		{Key: SettingKeyAIRouteAPIKey, Value: ""},
		{Key: SettingKeyAIRouteModel, Value: ""},
		{Key: SettingKeyAIRouteTimeoutSeconds, Value: "180"},
		{Key: SettingKeyAIRouteParallelism, Value: "3"},
		{Key: SettingKeyAIRouteServices, Value: "[]"},
		{Key: SettingKeyAIRouteMaxModelsPerRequest, Value: "120"},
		{Key: SettingKeyStatsTimezone, Value: ""}, // 空=未配置，回退到 stats_timezone_offset 再回退 UTC
		{Key: SettingKeyStatsTimezoneOffset, Value: "0"},
		{Key: SettingKeyJWTDefaultExpiryMinutes, Value: "15"},    // 默认15分钟
		{Key: SettingKeyJWTRememberMeExpiryDays, Value: "30"},    // 默认30天
		{Key: SettingKeyLoginRateLimitWindow, Value: "10"},       // 默认10分钟
		{Key: SettingKeyLoginRateLimitMaxFailed, Value: "5"},     // 默认5次
		{Key: SettingKeyStreamSessionTTLMinutes, Value: "30"},    // 默认30分钟
		{Key: SettingKeyStreamSessionMaxEvents, Value: "4096"},   // 默认4096条
		{Key: SettingKeyStreamSessionMaxBytesMB, Value: "4"},     // 默认4MB
		{Key: SettingKeyStreamSessionMaxSessions, Value: "512"},  // 默认512个并发流会话（512×4MB≈2GB 最坏内存上限）
		{Key: SettingKeyNotifyHTTPTimeoutSeconds, Value: "10"},   // 默认10秒
		{Key: SettingKeyFailureHintTTLUnauthorized, Value: "10"}, // 默认10秒
		{Key: SettingKeyFailureHintTTLRateLimit, Value: "5"},     // 默认5秒
		{Key: SettingKeyFailureHintTTLNetwork, Value: "2"},       // 默认2秒
		{Key: SettingKeyWebDAVConfig, Value: `{"enabled":false,"base_url":"","username":"","password":"","remote_path":"/octopus-backup/","interval_hours":6,"include_stats":true,"include_logs":false,"max_backups":10}`},
		{Key: SettingKeySiteSyncInterval, Value: "12"},
		{Key: SettingKeySiteCheckinInterval, Value: "24"},
		{Key: SettingKeyStatsSiteModelBackfilled, Value: "false"},
		{Key: SettingKeyProjectedChannelAutoGroupEnabled, Value: "0"}, // 默认不自动分组
		{Key: SettingKeyResponseFilterEnabled, Value: "false"},
		{Key: SettingKeyResponseFilterKeywords, Value: "[]"},
		{Key: SettingKeyResponseFilterAction, Value: "block"},
		{Key: SettingKeyResponseFilterErrorMessage, Value: "The response contains blocked keywords and has been intercepted."},
		{Key: SettingKeyLogLevel, Value: "info"},
		{Key: SettingKeyLogExcludedGroups, Value: "[]"},
		{Key: SettingKeyModelNormalizeRouterPrefixes, Value: "[]"},        // 默认无自定义路由前缀，回退到前端内置默认
		{Key: SettingKeyModelNormalizeFunctionalSuffixes, Value: "[]"},    // 默认无自定义功能后缀，回退到前端内置默认
		{Key: SettingKeyModelNormalizeExplicitMappings, Value: "[]"},      // 默认无显式变体→基准名映射
		{Key: SettingKeyModelNormalizeMarketDedupeDefault, Value: "true"}, // 模型广场默认开启归一化去重
		{Key: SettingKeyWebAuthnRPID, Value: ""},
		{Key: SettingKeyWebAuthnRPName, Value: "Octopus"},
		{Key: SettingKeyWebAuthnOrigins, Value: ""},
		{Key: SettingKeyTrustedProxies, Value: ""},                      // 默认不信任任何代理（安全默认，防 XFF 伪造）；反代/Docker 部署需配置实际代理网段
		{Key: SettingKeyKeyHealthCheckEnabled, Value: "false"},          // 默认关闭定时 Key 巡检（issue #142）
		{Key: SettingKeyKeyHealthCheckInterval, Value: "30"},            // 默认 30 分钟
		{Key: SettingKeyKeyHealthCheckFailThreshold, Value: "3"},        // 默认连续失败 3 次标记异常
		{Key: SettingKeyKeyHealthCheckNotifyEnabled, Value: "true"},     // 默认发送失败通知
		{Key: SettingKeyKeyHealthCheckRecoveryNotify, Value: "true"},    // 默认发送恢复通知
		{Key: SettingKeyKeyHealthCheckNotifyCooldown, Value: "300"},     // 默认通知冷却 5 分钟
		{Key: SettingKeyGroupUpstreamMetaDisplayEnabled, Value: "true"}, // 默认开启分组上游元信息展示
		{Key: SettingKeyPoolTokenRefreshInterval, Value: "10"},          // 默认 10 分钟检查号池 OAuth token 刷新
		{Key: SettingKeyPoolQuotaSyncInterval, Value: "360"},            // 默认 6 小时同步号池额度
		{Key: SettingKeyPlanProviderRefreshInterval, Value: "30"},       // 默认 30 分钟自动刷新额度监控
		{Key: SettingKeyPoolMinPriority, Value: "-9999"},                // 默认关闭分层过滤
		{Key: SettingKeyPoolLayeredFilterEnabled, Value: "false"},       // 默认关闭号池分层过滤
		{Key: SettingKeyPoolStickyEscapeEnabled, Value: "false"},        // Sticky escape off by default (sub2api defaults to on; octopus policy: new behavior is off by default)
		{Key: SettingKeyPoolStickyEscapeErrorRate, Value: "0.5"},        // Default escape error-rate threshold 0.5
		{Key: SettingKeyPoolStickyEscapeTTFTMs, Value: "15000"},         // Default escape TTFT threshold 15000ms
		{Key: SettingKeyPoolSchedulerWeightReset, Value: "0"},           // Default 0 = reset factor off (mirrors sub2api: default 0 keeps existing behavior)
		{Key: SettingKeyPoolSchedulerWeightQuota, Value: "0"},           // Default 0 = quota-headroom factor off (also avoids per-candidate quota snapshot decryption)
		{Key: SettingKeyPoolGeminiClientSecret, Value: ""},              // Default empty: env GEMINI_CLI_OAUTH_CLIENT_SECRET -> built-in public credential (B3-#5)
		{Key: SettingKeyPoolHealthCheckEnabled, Value: "false"},         // 默认关闭号池巡检
		{Key: SettingKeyPoolHealthCheckInterval, Value: "30"},           // 默认 30 分钟巡检
		{Key: SettingKeyPoolHealthCheckFailThreshold, Value: "3"},       // 默认 3 次失败后 SetError
	}
}

// DefaultSettingValue 返回指定设置键在 DefaultSettings 中登记的默认值；
// 未登记的键返回 ("", false)。
func DefaultSettingValue(key SettingKey) (string, bool) {
	for _, s := range DefaultSettings() {
		if s.Key == key {
			return s.Value, true
		}
	}
	return "", false
}

func (s *Setting) Validate() error {
	switch s.Key {
	case SettingKeyStatsTimezone:
		// 空串合法（=未配置，回退到 stats_timezone_offset）。非空必须是合法 IANA 时区名。
		if s.Value == "" {
			return nil
		}
		if _, err := time.LoadLocation(s.Value); err != nil {
			return fmt.Errorf("invalid IANA timezone: %s", s.Value)
		}
		return nil
	case SettingKeyModelInfoUpdateInterval, SettingKeySyncLLMInterval, SettingKeyRelayLogKeepPeriod, SettingKeyRelayLogKeepCount,
		SettingKeyStatsSaveInterval, SettingKeySiteSyncInterval, SettingKeySiteCheckinInterval,
		SettingKeyRelayRetryCount, SettingKeyRelayRouteRetries, SettingKeyCircuitBreakerThreshold, SettingKeyCircuitBreakerCooldown,
		SettingKeyCircuitBreakerMaxCooldown, SettingKeyCircuitBreakerHalfOpenProbeTimeout, SettingKeyRatelimitCooldown, SettingKeyRelayMaxTotalAttempts,
		SettingKeyRateLimitHoldInterval, SettingKeyRateLimitHoldMaxWait,
		SettingKeySemanticCacheTTL, SettingKeySemanticCacheThreshold, SettingKeySemanticCacheMaxEntries,
		SettingKeySemanticCacheEmbeddingTimeoutSeconds,
		SettingKeyAutoStrategyMinSamples, SettingKeyAutoStrategyTimeWindow, SettingKeyAutoStrategySampleThreshold,
		SettingKeyAutoStrategyLatencyWeight,
		SettingKeyAutoStrategyTTFTWeight, SettingKeyAutoStrategyPriceWeight,
		SettingKeyAutoStrategyExploreRate, SettingKeyAutoStrategyBucketTolerance,
		SettingKeyAIRouteGroupID, SettingKeyAIRouteTimeoutSeconds, SettingKeyAIRouteParallelism,
		SettingKeyAIRouteMaxModelsPerRequest,
		SettingKeyStatsTimezoneOffset,
		SettingKeyJWTDefaultExpiryMinutes, SettingKeyJWTRememberMeExpiryDays,
		SettingKeyLoginRateLimitWindow, SettingKeyLoginRateLimitMaxFailed,
		SettingKeyStreamSessionTTLMinutes, SettingKeyStreamSessionMaxEvents, SettingKeyStreamSessionMaxBytesMB,
		SettingKeyStreamSessionMaxSessions,
		SettingKeyNotifyHTTPTimeoutSeconds,
		SettingKeyFailureHintTTLUnauthorized, SettingKeyFailureHintTTLRateLimit, SettingKeyFailureHintTTLNetwork,
		SettingKeyKeyHealthCheckInterval, SettingKeyKeyHealthCheckFailThreshold, SettingKeyKeyHealthCheckNotifyCooldown,
		SettingKeyPoolTokenRefreshInterval, SettingKeyPoolQuotaSyncInterval, SettingKeyPlanProviderRefreshInterval,
		SettingKeyPoolMinPriority, SettingKeyPoolHealthCheckInterval, SettingKeyPoolHealthCheckFailThreshold,
		SettingKeyPoolStickyEscapeTTFTMs:
		v, err := strconv.Atoi(s.Value)
		if err != nil {
			return fmt.Errorf("setting value must be an integer")
		}
		// 允许设为 0：0 表示该候选渠道内不进行 Key 级重试（只试一次，失败即换渠道）。
		if s.Key == SettingKeyRelayRetryCount && v < 0 {
			return fmt.Errorf("relay retry count must be greater than or equal to 0")
		}
		if s.Key == SettingKeyRelayRouteRetries && v < 1 {
			return fmt.Errorf("relay route retries must be greater than or equal to 1")
		}
		if (s.Key == SettingKeyRatelimitCooldown || s.Key == SettingKeyRelayMaxTotalAttempts) && v < 0 {
			return fmt.Errorf("setting value must be greater than or equal to 0")
		}
		// 允许设为 0：0 表示不限制流会话总数（不推荐，最坏情况会吃光内存）。
		if s.Key == SettingKeyStreamSessionMaxSessions && v < 0 {
			return fmt.Errorf("stream session max sessions must be greater than or equal to 0")
		}
		if (s.Key == SettingKeyRateLimitHoldInterval || s.Key == SettingKeyRateLimitHoldMaxWait) && v < 1 {
			return fmt.Errorf("rate limit hold setting must be greater than 0")
		}
		if (s.Key == SettingKeyAutoStrategyMinSamples || s.Key == SettingKeyAutoStrategyTimeWindow || s.Key == SettingKeyAutoStrategySampleThreshold) && v < 1 {
			return fmt.Errorf("auto strategy setting must be greater than 0")
		}
		if s.Key == SettingKeyAutoStrategyLatencyWeight && (v < 0 || v > 100) {
			return fmt.Errorf("auto strategy latency weight must be between 0 and 100")
		}
		if (s.Key == SettingKeyAutoStrategyTTFTWeight || s.Key == SettingKeyAutoStrategyPriceWeight || s.Key == SettingKeyAutoStrategyExploreRate || s.Key == SettingKeyAutoStrategyBucketTolerance) && (v < 0 || v > 100) {
			return fmt.Errorf("auto strategy weight must be between 0 and 100")
		}
		if s.Key == SettingKeySemanticCacheTTL && v < 1 {
			return fmt.Errorf("semantic cache TTL must be greater than 0")
		}
		if s.Key == SettingKeySemanticCacheThreshold && (v < 0 || v > 100) {
			return fmt.Errorf("semantic cache threshold must be between 0 and 100")
		}
		if s.Key == SettingKeySemanticCacheMaxEntries && v < 1 {
			return fmt.Errorf("semantic cache max entries must be greater than 0")
		}
		if s.Key == SettingKeySemanticCacheEmbeddingTimeoutSeconds && v < 1 {
			return fmt.Errorf("semantic cache embedding timeout must be greater than 0")
		}
		if s.Key == SettingKeyAIRouteGroupID && v < 0 {
			return fmt.Errorf("ai route group id must be greater than or equal to 0")
		}
		if s.Key == SettingKeyAIRouteTimeoutSeconds && v < 1 {
			return fmt.Errorf("ai route timeout must be greater than 0")
		}
		if s.Key == SettingKeyAIRouteParallelism && v < 1 {
			return fmt.Errorf("ai route parallelism must be greater than 0")
		}
		if s.Key == SettingKeyAIRouteMaxModelsPerRequest && v < 1 {
			return fmt.Errorf("ai route max models per request must be greater than 0")
		}
		if s.Key == SettingKeyStatsTimezoneOffset && (v < -12 || v > 14) {
			return fmt.Errorf("stats timezone offset must be between -12 and 14")
		}
		switch s.Key {
		case SettingKeyJWTDefaultExpiryMinutes, SettingKeyJWTRememberMeExpiryDays,
			SettingKeyLoginRateLimitWindow, SettingKeyLoginRateLimitMaxFailed,
			SettingKeyStreamSessionTTLMinutes, SettingKeyStreamSessionMaxEvents, SettingKeyStreamSessionMaxBytesMB,
			SettingKeyFailureHintTTLUnauthorized, SettingKeyFailureHintTTLRateLimit, SettingKeyFailureHintTTLNetwork,
			SettingKeyStatsSaveInterval, SettingKeyModelInfoUpdateInterval, SettingKeySyncLLMInterval,
			SettingKeySiteSyncInterval, SettingKeySiteCheckinInterval,
			SettingKeyKeyHealthCheckInterval, SettingKeyKeyHealthCheckFailThreshold, SettingKeyKeyHealthCheckNotifyCooldown,
			SettingKeyPoolTokenRefreshInterval, SettingKeyPoolQuotaSyncInterval, SettingKeyPlanProviderRefreshInterval,
			SettingKeyPoolHealthCheckInterval, SettingKeyPoolHealthCheckFailThreshold:
			if v < 1 {
				return fmt.Errorf("setting value must be greater than 0")
			}
		}
		// 周期类设置换算成 time.Duration 时必须不能溢出（否则回绕成负数/极小值，
		// 周期任务行为不可预期）。上界 = MaxInt64 纳秒 / 单位纳秒。
		switch s.Key {
		case SettingKeyModelInfoUpdateInterval, SettingKeySyncLLMInterval, SettingKeySiteSyncInterval, SettingKeySiteCheckinInterval:
			if int64(v) > math.MaxInt64/int64(time.Hour) {
				return fmt.Errorf("setting value is too large")
			}
		case SettingKeyStatsSaveInterval, SettingKeyKeyHealthCheckInterval,
			SettingKeyPoolTokenRefreshInterval, SettingKeyPoolQuotaSyncInterval, SettingKeyPlanProviderRefreshInterval,
			SettingKeyPoolHealthCheckInterval:
			if int64(v) > math.MaxInt64/int64(time.Minute) {
				return fmt.Errorf("setting value is too large")
			}
		case SettingKeyKeyHealthCheckNotifyCooldown:
			if int64(v) > math.MaxInt64/int64(time.Second) {
				return fmt.Errorf("setting value is too large")
			}
		}
	case SettingKeyRelayLogKeepEnabled, SettingKeyRelayLogContentEnabled, SettingKeyStreamSessionReplayEnabled, SettingKeySemanticCacheEnabled, SettingKeyModelNormalizeMarketDedupeDefault, SettingKeyRetryEmptyOutput, SettingKeyRateLimitHoldEnabled, SettingKeyKeyHealthCheckEnabled, SettingKeyKeyHealthCheckNotifyEnabled, SettingKeyKeyHealthCheckRecoveryNotify,
		SettingKeyPoolLayeredFilterEnabled, SettingKeyPoolHealthCheckEnabled, SettingKeyPoolStickyEscapeEnabled:
		if s.Value != "true" && s.Value != "false" {
			return fmt.Errorf("setting value must be true or false")
		}
		return nil
	case SettingKeyPoolStickyEscapeErrorRate:
		v, err := strconv.ParseFloat(s.Value, 64)
		if err != nil || v <= 0 || v > 1 {
			return fmt.Errorf("pool_sticky_escape_error_rate must be a float in (0, 1]")
		}
		return nil
	case SettingKeyPoolSchedulerWeightReset, SettingKeyPoolSchedulerWeightQuota:
		v, err := strconv.ParseFloat(s.Value, 64)
		if err != nil || v < 0 {
			return fmt.Errorf("pool scheduler weight must be a float >= 0")
		}
		return nil
	case SettingKeyReasoningBufferStrategy:
		if s.Value != "buffer" && s.Value != "immediate" {
			return fmt.Errorf("reasoning buffer strategy must be 'buffer' or 'immediate'")
		}
	case SettingKeyKeySelectionStrategy:
		if s.Value != "cost" && s.Value != "availability" && s.Value != "speed" && s.Value != "priority" {
			return fmt.Errorf("key selection strategy must be cost, availability, speed or priority")
		}
		return nil
	case SettingKeyRelayLogQueueDropPolicy:
		if s.Value != "disabled" && s.Value != "oldest" && s.Value != "newest" {
			return fmt.Errorf("relay log queue drop policy must be disabled, oldest or newest")
		}
		return nil
	case SettingKeyProxyURL, SettingKeySemanticCacheEmbeddingBaseURL, SettingKeyAIRouteBaseURL:
		if s.Value == "" {
			return nil
		}
		parsedURL, err := url.Parse(s.Value)
		if err != nil {
			if s.Key == SettingKeySemanticCacheEmbeddingBaseURL {
				return fmt.Errorf("semantic cache embedding base URL is invalid: %w", err)
			}
			if s.Key == SettingKeyAIRouteBaseURL {
				return fmt.Errorf("ai route base URL is invalid: %w", err)
			}
			return fmt.Errorf("proxy URL is invalid: %w", err)
		}
		if s.Key == SettingKeySemanticCacheEmbeddingBaseURL {
			if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
				return fmt.Errorf("semantic cache embedding base URL scheme must be http or https")
			}
			if parsedURL.Host == "" {
				return fmt.Errorf("semantic cache embedding base URL must have a host")
			}
			return nil
		}
		if s.Key == SettingKeyAIRouteBaseURL {
			if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
				return fmt.Errorf("ai route base URL scheme must be http or https")
			}
			if parsedURL.Host == "" {
				return fmt.Errorf("ai route base URL must have a host")
			}
			return nil
		}

		validSchemes := map[string]bool{
			"http":   true,
			"https":  true,
			"socks":  true,
			"socks5": true,
			"ss":     true,
			"vmess":  true,
			"vless":  true,
			"trojan": true,
		}
		if !validSchemes[parsedURL.Scheme] {
			return fmt.Errorf("proxy URL scheme must be http, https, socks, socks5, ss, vmess, vless, or trojan")
		}
		if parsedURL.Host == "" {
			return fmt.Errorf("proxy URL must have a host")
		}
		return nil
	case SettingKeyPublicAPIBaseURL:
		if s.Value == "" {
			return nil
		}
		parsedURL, err := url.Parse(s.Value)
		if err != nil {
			return fmt.Errorf("public API base URL is invalid: %w", err)
		}
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return fmt.Errorf("public API base URL scheme must be http or https")
		}
		if parsedURL.Host == "" {
			return fmt.Errorf("public API base URL must have a host")
		}
		return nil
	case SettingKeyAlertNotifyLanguage:
		switch s.Value {
		case "zh-Hans", "zh-Hant", "en":
			return nil
		default:
			return fmt.Errorf("alert notify language must be zh-Hans, zh-Hant, or en")
		}
	case SettingKeyNavOrder, SettingKeyNavVisible,
		SettingKeyHubTabOrder, SettingKeyHubTabVisible,
		SettingKeyAnalyticsTabOrder, SettingKeyAnalyticsTabVisible,
		SettingKeyOpsTabOrder, SettingKeyOpsTabVisible:
		var navOrder []string
		if err := json.Unmarshal([]byte(s.Value), &navOrder); err != nil {
			return fmt.Errorf("nav setting must be a valid JSON array of strings")
		}
		return nil
	case SettingKeyAIRouteServices:
		return ValidateAIRouteServiceConfigs(s.Value)
	case SettingKeyWebDAVConfig:
		// 与专用保存端点（POST /api/v1/backup/webdav/config）对齐：interval_hours
		// 必须是 1..168 的整数，max_backups >= 1（专用端点把 <1 归一化为 10），
		// 其余字段仅做类型校验。用 typed struct 反序列化，非整数（如 1.5）直接失败。
		var cfg struct {
			Enabled       *bool   `json:"enabled"`
			BaseURL       *string `json:"base_url"`
			Username      *string `json:"username"`
			Password      *string `json:"password"`
			RemotePath    *string `json:"remote_path"`
			IntervalHours *int    `json:"interval_hours"`
			IncludeStats  *bool   `json:"include_stats"`
			IncludeLogs   *bool   `json:"include_logs"`
			MaxBackups    *int    `json:"max_backups"`
		}
		if err := json.Unmarshal([]byte(s.Value), &cfg); err != nil {
			return fmt.Errorf("webdav config is invalid: %v", err)
		}
		if cfg.IntervalHours == nil || *cfg.IntervalHours < 1 || *cfg.IntervalHours > 168 {
			return fmt.Errorf("webdav config interval_hours must be an integer between 1 and 168")
		}
		if cfg.MaxBackups != nil && *cfg.MaxBackups < 1 {
			return fmt.Errorf("webdav config max_backups must be greater than or equal to 1")
		}
		return nil
	case SettingKeyResponseFilterEnabled, SettingKeyGroupUpstreamMetaDisplayEnabled:
		if s.Value != "true" && s.Value != "false" {
			return fmt.Errorf("setting value must be true or false")
		}
		return nil
	case SettingKeyResponseFilterKeywords:
		var keywords []string
		if err := json.Unmarshal([]byte(s.Value), &keywords); err != nil {
			return fmt.Errorf("response filter keywords must be a valid JSON array of strings")
		}
		return nil
	case SettingKeyLogExcludedGroups:
		var groups []string
		if err := json.Unmarshal([]byte(s.Value), &groups); err != nil {
			return fmt.Errorf("log excluded groups must be a valid JSON array of strings")
		}
		return nil
	case SettingKeyModelNormalizeRouterPrefixes, SettingKeyModelNormalizeFunctionalSuffixes:
		var items []string
		if err := json.Unmarshal([]byte(s.Value), &items); err != nil {
			return fmt.Errorf("model normalize rules must be a valid JSON array of strings")
		}
		return nil
	case SettingKeyModelNormalizeExplicitMappings:
		// 显式归一映射：[{ "variant": "...", "canonical": "..." }]。
		// 校验为对象数组且每条含非空 variant / canonical 字符串。
		var mappings []map[string]string
		if err := json.Unmarshal([]byte(s.Value), &mappings); err != nil {
			return fmt.Errorf("model normalize explicit mappings must be a valid JSON array of objects")
		}
		for _, m := range mappings {
			if strings.TrimSpace(m["variant"]) == "" || strings.TrimSpace(m["canonical"]) == "" {
				return fmt.Errorf("each explicit mapping must have non-empty variant and canonical")
			}
		}
		return nil
	case SettingKeyResponseFilterAction:
		switch s.Value {
		case "block", "replace":
			return nil
		default:
			return fmt.Errorf("response filter action must be block or replace")
		}
	case SettingKeyResponseFilterErrorMessage:
		return nil
	case SettingKeyLogLevel:
		switch s.Value {
		case "debug", "info", "warn", "error":
			return nil
		default:
			return fmt.Errorf("log level must be one of: debug, info, warn, error")
		}
	case SettingKeyTrustedProxies:
		// 空=不信任任何代理，*=信任所有。其余值按逗号分隔，每段必须是合法 CIDR 或 IP。
		raw := strings.TrimSpace(s.Value)
		if raw == "" || raw == "*" {
			return nil
		}
		for _, p := range strings.Split(raw, ",") {
			v := strings.TrimSpace(p)
			if v == "" {
				continue
			}
			if _, _, err := net.ParseCIDR(v); err == nil {
				continue
			}
			if ip := net.ParseIP(v); ip != nil {
				continue
			}
			return fmt.Errorf("trusted_proxies entry %q is not a valid CIDR or IP", v)
		}
		return nil
	case SettingKeyProjectedChannelAutoGroupEnabled:
		_, ok := ParseAutoGroupSettingValue(s.Value)
		if !ok {
			return fmt.Errorf("projected channel auto group mode must be 0 (none), 1 (fuzzy), 2 (exact), or 3 (regex)")
		}
		return nil
	}

	return nil
}
