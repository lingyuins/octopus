import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient, API_BASE_URL } from '../client';
import { REFETCH_INTERVAL_DEFAULT } from '../constants';
import { logger } from '@/lib/logger';
import { useAuthStore } from './user';

/**
 * Setting 数据
 */
export interface Setting {
    key: string;
    value: string;
}

export const SettingKey = {
    ProxyURL: 'proxy_url',
    PublicAPIBaseURL: 'public_api_base_url',
    StatsSaveInterval: 'stats_save_interval',
    ModelInfoUpdateInterval: 'model_info_update_interval',
    SyncLLMInterval: 'sync_llm_interval',
    RelayLogKeepEnabled: 'relay_log_keep_enabled',
    RelayLogContentEnabled: 'relay_log_content_enabled',
    RelayLogQueueDropPolicy: 'relay_log_queue_drop_policy',
    StreamSessionReplayEnabled: 'stream_session_replay_enabled',
    RelayLogKeepPeriod: 'relay_log_keep_period',
    RelayLogKeepCount: 'relay_log_keep_count',
    CORSAllowOrigins: 'cors_allow_origins',
    RelayRetryCount: 'relay_retry_count',
    RelayRouteRetries: 'relay_route_retries',
    RatelimitCooldown: 'ratelimit_cooldown',
    KeySelectionStrategy: 'key_selection_strategy',
    KeyHealthCheckEnabled: 'key_health_check_enabled',
    KeyHealthCheckInterval: 'key_health_check_interval',
    KeyHealthCheckFailThreshold: 'key_health_check_fail_threshold',
    KeyHealthCheckNotifyEnabled: 'key_health_check_notify_enabled',
    KeyHealthCheckRecoveryNotify: 'key_health_check_recovery_notify',
    KeyHealthCheckNotifyCooldown: 'key_health_check_notify_cooldown',
    RelayMaxTotalAttempts: 'relay_max_total_attempts',
    RetryEmptyOutput: 'retry_empty_output',
    RateLimitHoldEnabled: 'rate_limit_hold_enabled',
    RateLimitHoldInterval: 'rate_limit_hold_interval',
    RateLimitHoldMaxWait: 'rate_limit_hold_max_wait',
    ReasoningBufferStrategy: 'reasoning_buffer_strategy',
    CircuitBreakerThreshold: 'circuit_breaker_threshold',
    CircuitBreakerCooldown: 'circuit_breaker_cooldown',
    CircuitBreakerMaxCooldown: 'circuit_breaker_max_cooldown',
    CircuitBreakerHalfOpenProbeTimeout: 'circuit_breaker_half_open_probe_timeout',
    AutoStrategyMinSamples: 'auto_strategy_min_samples',
    AutoStrategyTimeWindow: 'auto_strategy_time_window',
    AutoStrategySampleThreshold: 'auto_strategy_sample_threshold',
    AlertNotifyLanguage: 'alert_notify_language',
    AutoStrategyLatencyWeight: 'auto_strategy_latency_weight',
    SemanticCacheEnabled: 'semantic_cache_enabled',
    SemanticCacheTTL: 'semantic_cache_ttl',
    SemanticCacheThreshold: 'semantic_cache_threshold',
    SemanticCacheMaxEntries: 'semantic_cache_max_entries',
    SemanticCacheEmbeddingBaseURL: 'semantic_cache_embedding_base_url',
    SemanticCacheEmbeddingAPIKey: 'semantic_cache_embedding_api_key',
    SemanticCacheEmbeddingModel: 'semantic_cache_embedding_model',
    SemanticCacheEmbeddingTimeoutSeconds: 'semantic_cache_embedding_timeout_seconds',
    NavOrder: 'nav_order',
    NavVisible: 'nav_visible',
    HubTabOrder: 'hub_tab_order',
    HubTabVisible: 'hub_tab_visible',
    AnalyticsTabOrder: 'analytics_tab_order',
    AnalyticsTabVisible: 'analytics_tab_visible',
    OpsTabOrder: 'ops_tab_order',
    OpsTabVisible: 'ops_tab_visible',
    AIRouteGroupID: 'ai_route_group_id',
    AIRouteBaseURL: 'ai_route_base_url',
    AIRouteAPIKey: 'ai_route_api_key',
    AIRouteModel: 'ai_route_model',
    AIRouteTimeoutSeconds: 'ai_route_timeout_seconds',
    AIRouteParallelism: 'ai_route_parallelism',
    AIRouteServices: 'ai_route_services',
    StatsTimezone: 'stats_timezone',
    StatsTimezoneOffset: 'stats_timezone_offset',
    JWTDefaultExpiryMinutes: 'jwt_default_expiry_minutes',
    JWTRememberMeExpiryDays: 'jwt_remember_me_expiry_days',
    LoginRateLimitWindow: 'login_rate_limit_window',
    LoginRateLimitMaxFailed: 'login_rate_limit_max_failed',
    StreamSessionTTLMinutes: 'stream_session_ttl_minutes',
    StreamSessionMaxEvents: 'stream_session_max_events',
    StreamSessionMaxBytesMB: 'stream_session_max_bytes_mb',
    NotifyHTTPTimeoutSeconds: 'notify_http_timeout_seconds',
    FailureHintTTLUnauthorized: 'failure_hint_ttl_unauthorized',
    FailureHintTTLRateLimit: 'failure_hint_ttl_rate_limit',
    FailureHintTTLNetwork: 'failure_hint_ttl_network',
    WebDAVConfig: 'webdav_config',
    SiteSyncInterval: 'site_sync_interval',
    SiteCheckinInterval: 'site_checkin_interval',
    ProjectedChannelAutoGroupEnabled: 'projected_channel_auto_group_enabled',
    ResponseFilterEnabled: 'response_filter_enabled',
    ResponseFilterKeywords: 'response_filter_keywords',
    ResponseFilterAction: 'response_filter_action',
    ResponseFilterErrorMessage: 'response_filter_error_message',
    LogLevel: 'log_level',
    LogExcludedGroups: 'log_excluded_groups',
    ModelNormalizeRouterPrefixes: 'model_normalize_router_prefixes',
    ModelNormalizeFunctionalSuffixes: 'model_normalize_functional_suffixes',
    ModelNormalizeExplicitMappings: 'model_normalize_explicit_mappings',
    ModelNormalizeMarketDedupeDefault: 'model_normalize_market_dedupe_default',
    WebAuthnRPID: 'webauthn_rp_id',
    WebAuthnRPName: 'webauthn_rp_name',
    WebAuthnOrigins: 'webauthn_origins',
    TrustedProxies: 'trusted_proxies',
    GroupUpstreamMetaDisplayEnabled: 'group_upstream_meta_display_enabled',
} as const;

/**
 * 获取 Setting 列表 Hook
 * 
 * @example
 * const { data: settings, isLoading, error } = useSettingList();
 * 
 * if (isLoading) return <Loading />;
 * if (error) return <Error message={error.message} />;
 * 
 * settings?.forEach(setting => console.log(setting.key, setting.value));
 */
export function useSettingList() {
    return useQuery({
        queryKey: ['settings', 'list'],
        queryFn: async () => {
            return apiClient.get<Setting[]>('/api/v1/setting/list');
        },
        refetchInterval: REFETCH_INTERVAL_DEFAULT,
    });
}

/**
 * 设置 Setting Hook
 * 
 * @example
 * const setSetting = useSetSetting();
 * 
 * setSetting.mutate({
 *   key: 'theme',
 *   value: 'dark',
 * });
 */
export function useSetSetting() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (data: Setting) => {
            return apiClient.post<Setting>('/api/v1/setting/set', data);
        },
        onSuccess: (data) => {
            logger.log('Setting 设置成功:', data);
            queryClient.invalidateQueries({ queryKey: ['settings', 'list'] });
        },
        onError: (error) => {
            logger.error('Setting 设置失败:', error);
        },
    });
}

/**
 * 数据库导入/导出
 */
export interface DBImportStep {
    table: string;
    mode: string;
    rows_affected: number;
    ok: boolean;
    error?: string;
}

export interface DBImportResult {
    rows_affected: Record<string, number>;
    progress: DBImportStep[];
}

export interface DBExportOptions {
    include_logs?: boolean;
    include_stats?: boolean;
}

export interface DatabaseMigrationRequest {
    type: 'sqlite' | 'mysql' | 'postgres' | 'postgresql';
    path: string;
    include_logs?: boolean;
    include_stats?: boolean;
}

export interface DatabaseMigrationResult {
    type: string;
    path: string;
    include_logs: boolean;
    include_stats: boolean;
    restart_needed: boolean;
    // 迁移成功后已删除的旧 SQLite 文件路径（issue #118）。
    // 仅当源库为 SQLite、目标库为非 SQLite 时非空。
    cleaned_files?: string[];
    import_result: DBImportResult;
}

type ApiResponse<T> = {
    code?: number;
    message?: string;
    data?: T;
};

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null;
}

function getMessageField(value: unknown): string | undefined {
    if (!isRecord(value)) return undefined;
    const msg = value.message;
    return typeof msg === 'string' ? msg : undefined;
}

function getDataField<T>(value: unknown): T | undefined {
    if (!isRecord(value)) return undefined;
    return (value as ApiResponse<T>).data;
}

function getAuthHeader(): string {
    const token = useAuthStore.getState().token;
    if (!token) throw new Error('Not authenticated');
    return `Bearer ${token}`;
}

function parseFilename(contentDisposition: string | null): string | null {
    if (!contentDisposition) return null;
    const match = contentDisposition.match(/filename="([^"]+)"/i);
    return match?.[1] ?? null;
}

function exportFallbackFilename() {
    const d = new Date();
    const pad = (n: number) => String(n).padStart(2, '0');
    const ts = `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`;
    return `octopus-export-${ts}.json`;
}

async function downloadBlob(blob: Blob, filename: string) {
    const url = URL.createObjectURL(blob);
    try {
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        a.remove();
    } finally {
        URL.revokeObjectURL(url);
    }
}

/**
 * 导出数据库（下载 JSON 文件）
 *
 * 本接口直接返回原始 JSON 文件流（Content-Disposition: attachment），
 * 不使用管理端标准 {code, message, data} envelope，因此用原生 fetch + blob
 * 消费，不走 apiClient.get<T> 的 handleResponse 自动解包。
 */
export function useExportDB() {
    return useMutation({
        mutationFn: async (options: DBExportOptions = {}) => {
            const params = new URLSearchParams();
            params.set('include_logs', String(!!options.include_logs));
            params.set('include_stats', String(!!options.include_stats));

            // 下载型接口不使用标准 envelope——直接 fetch + blob，不经过 apiClient
            const res = await fetch(`${API_BASE_URL}/api/v1/setting/export?${params.toString()}`, {
                method: 'GET',
                headers: {
                    Authorization: getAuthHeader(),
                },
            });

            if (!res.ok) {
                const text = await res.text();
                throw new Error(text || res.statusText);
            }

            // 响应体是原始 JSON dump 文件，按 blob 消费触发浏览器下载
            const blob = await res.blob();
            const filename = parseFilename(res.headers.get('content-disposition')) || exportFallbackFilename();
            await downloadBlob(blob, filename);
            return { filename };
        },
        onError: (error) => {
            logger.error('导出数据库失败:', error);
        },
    });
}

/**
 * 导入数据库（上传 JSON 文件，支持增量/完整导入）
 */
export function useImportDB() {
    return useMutation({
        mutationFn: async ({ file, mode }: { file: File; mode: 'incremental' | 'full' }) => {
            const form = new FormData();
            form.append('file', file);

            const params = new URLSearchParams();
            params.set('mode', mode);

            const res = await fetch(`${API_BASE_URL}/api/v1/setting/import?${params.toString()}`, {
                method: 'POST',
                headers: {
                    Authorization: getAuthHeader(),
                },
                body: form,
            });

            const contentType = res.headers.get('content-type') || '';
            const isJson = contentType.includes('application/json');
            const data = isJson ? await res.json() : await res.text();

            if (!res.ok) {
                const message = getMessageField(data) ?? (typeof data === 'string' ? data : res.statusText);
                throw new Error(message);
            }

            const nested = getDataField<DBImportResult>(data);
            return nested ?? (data as DBImportResult);
        },
        onError: (error) => {
            logger.error('导入数据库失败:', error);
        },
    });
}

export function useTestDatabaseConnection() {
    return useMutation({
        mutationFn: async (data: DatabaseMigrationRequest) => {
            return apiClient.post<boolean>('/api/v1/setting/database/test', data);
        },
        onError: (error) => {
            logger.error('测试数据库连接失败:', error);
        },
    });
}

export function useMigrateDatabase() {
    return useMutation({
        mutationFn: async (data: DatabaseMigrationRequest) => {
            return apiClient.post<DatabaseMigrationResult>('/api/v1/setting/database/migrate', data);
        },
        onError: (error) => {
            logger.error('迁移数据库失败:', error);
        },
    });
}

/**
 * 缓存后端配置（Redis 可选，issue #123）
 */
export interface CacheRedisConfig {
    addr: string;
    password: string;
    username: string;
    db: number;
    pool_size: number;
    dial_timeout: string;
    read_timeout: string;
}

export interface CacheConfig {
    type: '' | 'redis';
    redis: CacheRedisConfig;
}

export interface CacheConfigRequest {
    type: '' | 'redis';
    redis: CacheRedisConfig;
}

export interface CacheConfigResult {
    type: string;
    restart_needed: boolean;
}

/**
 * 获取当前缓存后端配置（回显 config.json 的 cache 字段）
 */
export function useGetCacheConfig() {
    return useQuery({
        queryKey: ['settings', 'cache-config'],
        queryFn: async () => {
            return apiClient.get<CacheConfig>('/api/v1/setting/cache/config');
        },
    });
}

/**
 * 测试 Redis 连接连通性（不改变运行中状态）
 */
export function useTestCacheConnection() {
    return useMutation({
        mutationFn: async (data: CacheConfigRequest) => {
            return apiClient.post<boolean>('/api/v1/setting/cache/test', data);
        },
        onError: (error) => {
            logger.error('测试 Redis 连接失败:', error);
        },
    });
}

/**
 * 保存缓存配置到 config.json（需重启生效）
 */
export function useSaveCacheConfig() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (data: CacheConfigRequest) => {
            return apiClient.post<CacheConfigResult>('/api/v1/setting/cache/save', data);
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['settings', 'cache-config'] });
        },
        onError: (error) => {
            logger.error('保存缓存配置失败:', error);
        },
    });
}
