import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';
import { REFETCH_INTERVAL_DEFAULT } from '../constants';
import { logger } from '@/lib/logger';
import { formatCount, formatMoney, formatTime } from '@/lib/utils';
import { StatsChannel, type StatsMetricsFormatted } from './stats';
import { type GroupTestProgress } from './group';
import type { ProxyMode } from './proxy-pool';

/**
 * 渠道代理模式（渠道无上级实体，不支持 inherit）
 */
export type ChannelProxyMode = Exclude<ProxyMode, 'inherit'>;
/**
 * 渠道类型枚举
 */
export enum ChannelType {
    OpenAIChat = 0,
    OpenAIResponse = 1,
    Anthropic = 2,
    Gemini = 3,
    Volcengine = 4,
    OpenAIEmbedding = 5,
    MiMoChat = 6,
    Cloudflare = 7,
}

/**
 * 自动分组类型枚举
 */
export enum AutoGroupType {
    None = 0,   // 不自动分组
    Fuzzy = 1,  // 模糊匹配
    Exact = 2,  // 准确匹配
    Regex = 3,  // 正则匹配
}

export enum RequestRewriteProfile {
    Preserve = 'preserve',
    OpenAIChatCompat = 'openai_chat_compat',
}

export enum RequestRewriteHeaderProfile {
    None = '',
    Codex = 'codex',
}

export enum ToolRoleStrategy {
    Keep = 'keep',
    StringifyToUser = 'stringify_to_user',
}

export enum SystemMessageStrategy {
    Keep = 'keep',
    Merge = 'merge',
}

export type RequestRewriteConfig = {
    enabled: boolean;
    profile?: RequestRewriteProfile | null;
    tool_role_strategy?: ToolRoleStrategy | null;
    system_message_strategy?: SystemMessageStrategy | null;
    header_profile?: RequestRewriteHeaderProfile | '' | null;
};

export type BaseUrl = {
    url: string;
    delay: number;
    suffix_mode?: 'auto' | 'openai_compat' | 'anthropic' | 'gemini' | 'volcengine' | 'custom' | '';
};

export type CustomHeader = {
    header_key: string;
    header_value: string;
};

export type ChannelKey = {
    id: number;
    channel_id: number;
    enabled: boolean;
    channel_key: string;
    status_code: number;
    last_use_time_stamp: number;
    total_cost: number;
    priority: number;
    remark: string;
};

export type ChannelGroup = {
    id: number;
    name: string;
    is_default: boolean;
    created_at: number;
    updated_at: number;
};

export type ChannelBatchGroupResult = {
    success_ids: number[];
    failed_items: Array<{ id: number; message: string }>;
};

/**
 * 渠道完整数据（与后端 model.Channel 对齐；数组字段在前端保证为 []）
 */
export type Channel = {
    id: number;
    name: string;
    group_id: number;
    type: ChannelType;
    enabled: boolean;
    base_urls: BaseUrl[];
    keys: ChannelKey[];
    model: string;
    custom_model: string;
    proxy_mode: ChannelProxyMode;
    proxy_config_id?: number | null;
    proxy: boolean;
    auto_sync: boolean;
    auto_group: AutoGroupType;
    skip_model_test: boolean;
    disposable: boolean;
    expire_at?: string | null;
    notif_channel_id?: number | null;
    key_selection_strategy: string;
    custom_header: CustomHeader[];
    param_override?: string | null;
    channel_proxy?: string | null;
    request_rewrite?: RequestRewriteConfig | null;
    match_regex?: string | null;
    key_health_passed?: boolean | null;
    key_health_all_failed?: boolean | null;
    key_health_at?: number;
    pool_id: number;
    stats?: StatsChannel;
};

// Internal type: backend may return null for slice fields; normalize to [] in select()
type ChannelServer = Omit<Channel, 'base_urls' | 'custom_header' | 'keys'> & {
    base_urls: BaseUrl[] | null;
    custom_header: CustomHeader[] | null;
    keys: ChannelKey[] | null;
};

/**
 * 创建渠道请求：必填字段 + 可选字段
 */
export type CreateChannelRequest = {
    name: string;
    group_id?: number;
    type: ChannelType;
    enabled?: boolean;
    base_urls: BaseUrl[];
    keys: Array<Pick<ChannelKey, 'enabled' | 'channel_key' | 'priority' | 'remark'>>;
    model: string;
    custom_model?: string;
    proxy_mode?: ChannelProxyMode;
    proxy_config_id?: number | null;
    proxy?: boolean;
    auto_sync?: boolean;
    skip_model_test?: boolean;
    disposable?: boolean;
    expire_at?: string | null;
    notif_channel_id?: number | null;
    key_selection_strategy?: string;
    auto_group?: AutoGroupType;
    custom_header?: CustomHeader[];
    channel_proxy?: string | null;
    param_override?: string | null;
    request_rewrite?: RequestRewriteConfig;
    match_regex?: string | null;
    pool_id?: number;
};

/**
 * 更新渠道请求：id + 可选字段 + keys diff
 */
export type UpdateChannelRequest = {
    id: number;
    name?: string;
    group_id?: number;
    type?: ChannelType;
    enabled?: boolean;
    base_urls?: BaseUrl[];
    model?: string;
    custom_model?: string;
    proxy_mode?: ChannelProxyMode;
    proxy_config_id?: number | null;
    proxy?: boolean;
    auto_sync?: boolean;
    key_selection_strategy?: string;
    skip_model_test?: boolean;
    disposable?: boolean;
    expire_at?: string | null;
    notif_channel_id?: number | null;
    auto_group?: AutoGroupType;
    custom_header?: CustomHeader[];
    channel_proxy?: string | null;
    param_override?: string | null;
    request_rewrite?: RequestRewriteConfig;
    match_regex?: string | null;
    pool_id?: number;
    // keys diff
    keys_to_add?: Array<Pick<ChannelKey, 'enabled' | 'channel_key' | 'priority' | 'remark'>>;
    keys_to_update?: Array<{ id: number; enabled?: boolean; channel_key?: string; priority?: number; remark?: string }>;
    keys_to_delete?: number[];
};

export type FetchModelRequest = {
    type: ChannelType;
    base_urls: BaseUrl[];
    keys: Array<Pick<ChannelKey, 'enabled' | 'channel_key'>>;
    proxy_mode?: ChannelProxyMode;
    proxy_config_id?: number | null;
    proxy?: boolean;
    channel_proxy?: string | null;
    match_regex?: string | null;
    custom_header?: CustomHeader[];
};

export type TestChannelResult = {
    base_url: string;
    key_remark?: string;
    key_masked?: string;
    status_code: number;
    passed: boolean;
    latency_ms: number;
    message?: string;
    response_body?: string;
};

export type TestChannelSummary = {
    passed: boolean;
    results: TestChannelResult[];
};

export type KeyModelResult = {
    key_remark?: string;
    key_masked?: string;
    models: string[];
    status_code: number;
    passed: boolean;
    message?: string;
};

export type FetchModelsPerKeyResponse = {
    results: KeyModelResult[];
    all_models: string[];
};

/**
 * 获取渠道列表 Hook
 *
 * @example
 * const { data: channels, isLoading, error } = useChannelList();
 *
 * if (isLoading) return <Loading />;
 * if (error) return <Error message={error.message} />;
 *
 * channels?.forEach(channel => console.log(channel.raw.name));
 */
export function useChannelList() {
    return useQuery({
        queryKey: ['channels', 'list'],
        queryFn: async () => {
            return apiClient.get<ChannelServer[]>('/api/v1/channel/list');
        },
        select: (data) => data.map((item) => ({
            raw: ({
                ...item,
                base_urls: item.base_urls ?? [],
                custom_header: item.custom_header ?? [],
                keys: item.keys ?? [],
            }) satisfies Channel,
            formatted: {
                input_token: formatCount(item.stats?.input_token ?? 0),
                output_token: formatCount(item.stats?.output_token ?? 0),
                total_token: formatCount((item.stats?.input_token ?? 0) + (item.stats?.output_token ?? 0)),
                input_cost: formatMoney(item.stats?.input_cost ?? 0),
                output_cost: formatMoney(item.stats?.output_cost ?? 0),
                total_cost: formatMoney((item.stats?.input_cost ?? 0) + (item.stats?.output_cost ?? 0)),
                request_success: formatCount(item.stats?.request_success ?? 0),
                request_failed: formatCount(item.stats?.request_failed ?? 0),
                wait_time: formatTime(item.stats?.wait_time ?? 0),
                latency_p50: formatTime(item.stats?.latency_p50 ?? 0),
                latency_p95: formatTime(item.stats?.latency_p95 ?? 0),
                latency_p99: formatTime(item.stats?.latency_p99 ?? 0),
                ftut_avg: formatTime(item.stats?.ftut_avg ?? 0),
                ftut_p50: formatTime(item.stats?.ftut_p50 ?? 0),
                ftut_p95: formatTime(item.stats?.ftut_p95 ?? 0),
                ftut_p99: formatTime(item.stats?.ftut_p99 ?? 0),
                histogram_lt_100: formatCount(item.stats?.histogram_lt_100 ?? 0),
                histogram_100_500: formatCount(item.stats?.histogram_100_500 ?? 0),
                histogram_500_1k: formatCount(item.stats?.histogram_500_1k ?? 0),
                histogram_1k_5k: formatCount(item.stats?.histogram_1k_5k ?? 0),
                histogram_gt_5k: formatCount(item.stats?.histogram_gt_5k ?? 0),
                request_count: formatCount((item.stats?.request_success ?? 0) + (item.stats?.request_failed ?? 0)),
            }
        })) as Array<{ raw: Channel; formatted: StatsMetricsFormatted }>,
        refetchInterval: REFETCH_INTERVAL_DEFAULT,
    });
}

/**
 * 创建渠道 Hook
 * 
 * @example
 * const createChannel = useCreateChannel();
 * 
 * createChannel.mutate({
 *   name: 'OpenAI',
 *   type: ChannelType.OpenAIChat,
 *   base_urls: [{ url: 'https://api.openai.com', delay: 0 }],
 *   keys: [{ enabled: true, channel_key: 'sk-xxx' }],
 *   model: 'gpt-4',
 * });
 */
export function useCreateChannel() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (data: CreateChannelRequest) => {
            return apiClient.post<ChannelServer>('/api/v1/channel/create', data);
        },
        onSuccess: (data) => {
            logger.log('渠道创建成功:', data);
            queryClient.invalidateQueries({ queryKey: ['channels', 'list'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'list'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'channel'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'market'] });
        },
        onError: (error) => {
            logger.error('渠道创建失败:', error);
        },
    });
}

export function useChannelGroupList() {
    return useQuery({
        queryKey: ['channel-groups', 'list'],
        queryFn: async () => {
            return apiClient.get<ChannelGroup[]>('/api/v1/channel/group/list');
        },
    });
}

export function useCreateChannelGroup() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (data: { name: string }) => {
            return apiClient.post<ChannelGroup>('/api/v1/channel/group/create', data);
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['channel-groups', 'list'] });
        },
    });
}

export function useUpdateChannelGroup() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (data: { id: number; name: string }) => {
            return apiClient.post<ChannelGroup>('/api/v1/channel/group/update', data);
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['channel-groups', 'list'] });
        },
    });
}

export function useDeleteChannelGroup() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (id: number) => {
            return apiClient.delete<null>(`/api/v1/channel/group/delete/${id}`);
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['channel-groups', 'list'] });
            queryClient.invalidateQueries({ queryKey: ['channels', 'list'] });
        },
    });
}

/**
 * 更新渠道 Hook
 * 
 * @example
 * const updateChannel = useUpdateChannel();
 * 
 * updateChannel.mutate({
 *   id: 1,
 *   name: 'OpenAI Updated',
 *   type: ChannelType.OpenAIChat,
 *   enabled: true,
 *   base_urls: [{ url: 'https://api.openai.com', delay: 0 }],
 *   keys_to_add: [{ enabled: true, channel_key: 'sk-xxx' }],
 *   model: 'gpt-4-turbo',
 *   proxy: false,
 * });
 */
export function useUpdateChannel() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (data: UpdateChannelRequest) => {
            return apiClient.post<ChannelServer>('/api/v1/channel/update', data);
        },
        onSuccess: (data) => {
            logger.log('渠道更新成功:', data);
            queryClient.invalidateQueries({ queryKey: ['channels', 'list'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'channel'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'market'] });
        },
        onError: (error) => {
            logger.error('渠道更新失败:', error);
        },
    });
}

/**
 * 删除渠道 Hook
 * 
 * @example
 * const deleteChannel = useDeleteChannel();
 * 
 * deleteChannel.mutate(1); // 删除 ID 为 1 的渠道
 */
export function useDeleteChannel() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (id: number) => {
            return apiClient.delete<null>(`/api/v1/channel/delete/${id}`);
        },
        onSuccess: () => {
            logger.log('渠道删除成功');
            queryClient.invalidateQueries({ queryKey: ['channels', 'list'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'channel'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'market'] });
        },
        onError: (error) => {
            logger.error('渠道删除失败:', error);
        },
    });
}

/**
 * 批量设置渠道分组 Hook
 *
 * 将选中的多个渠道统一移动到同一个目标分组（覆盖原有分组）。
 *
 * @example
 * const batchSetGroup = useBatchSetChannelGroup();
 * batchSetGroup.mutate({ ids: [1, 2, 3], group_id: 5 });
 */
export function useBatchSetChannelGroup() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (data: { ids: number[]; group_id: number }) => {
            return apiClient.post<ChannelBatchGroupResult>('/api/v1/channel/batch-group', data);
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['channels', 'list'] });
            queryClient.invalidateQueries({ queryKey: ['channel-groups', 'list'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'channel'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'market'] });
        },
        onError: (error) => {
            logger.error('渠道批量分组失败:', error);
        },
    });
}

/**
 * 启用/禁用渠道 Hook
 * 
 * @example
 * const enableChannel = useEnableChannel();
 * 
 * enableChannel.mutate({ id: 1, enabled: true }); // 启用 ID 为 1 的渠道
 * enableChannel.mutate({ id: 1, enabled: false }); // 禁用 ID 为 1 的渠道
 */
export function useEnableChannel() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (data: { id: number; enabled: boolean }) => {
            return apiClient.post<null>('/api/v1/channel/enable', data);
        },
        onSuccess: () => {
            logger.log('渠道状态更新成功');
            queryClient.invalidateQueries({ queryKey: ['channels', 'list'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'market'] });
        },
        onError: (error) => {
            logger.error('渠道状态更新失败:', error);
        },
    });
}

/**
 * 获取渠道模型列表 Hook
 * 
 * @example
 * const fetchModel = useFetchModel();
 * 
 * fetchModel.mutate({
 *   type: ChannelType.OpenAIChat,
 *   base_urls: [{ url: 'https://api.openai.com', delay: 0 }],
 *   keys: [{ enabled: true, channel_key: 'sk-xxx' }],
 *   proxy: false,
 * });
 * 
 * // 在 onSuccess 中获取模型列表
 * fetchModel.data // ['gpt-4', 'gpt-3.5-turbo', ...]
 */
export function useFetchModel() {
    return useMutation({
        mutationFn: async (data: FetchModelRequest) => {
            return apiClient.post<string[]>('/api/v1/channel/fetch-model', data);
        },
        onSuccess: (data) => {
            logger.log('模型列表获取成功:', data);
        },
        onError: (error) => {
            logger.error('模型列表获取失败:', error);
        },
    });
}

/**
 * 按 key 逐个拉取模型列表 Hook。
 * 后端路由: POST /api/v1/channel/fetch-models-per-key
 * 返回每个 key 的模型列表和所有模型的并集。
 *
 * @example
 * const fetchModelsPerKey = useFetchModelsPerKey();
 * fetchModelsPerKey.mutate({
 *   type: ChannelType.OpenAIChat,
 *   base_urls: [{ url: 'https://api.openai.com', delay: 0 }],
 *   keys: [{ enabled: true, channel_key: 'sk-xxx' }],
 * });
 * // fetchModelsPerKey.data.results[0] -> { key_masked: 'sk-o...0001', models: [...], passed: true }
 * // fetchModelsPerKey.data.all_models -> ['gpt-4o', 'gpt-4.1', ...]
 */
export function useFetchModelsPerKey() {
    return useMutation({
        mutationFn: async (data: FetchModelRequest) => {
            return apiClient.post<FetchModelsPerKeyResponse>('/api/v1/channel/fetch-models-per-key', data);
        },
        onSuccess: (data) => {
            logger.log('按 key 模型列表获取成功:', data);
        },
        onError: (error) => {
            logger.error('按 key 模型列表获取失败:', error);
        },
    });
}

export function useTestChannel() {
    return useMutation({
        mutationFn: async (data: CreateChannelRequest | UpdateChannelRequest | FetchModelRequest) => {
            return apiClient.post<TestChannelSummary>('/api/v1/channel/test', data);
        },
        onSuccess: (data) => {
            logger.log('渠道测试成功:', data);
        },
        onError: (error) => {
            logger.error('渠道测试失败:', error);
        },
    });
}

/**
 * 检查指定渠道当前已保存的全部 key 的连通性 Hook。
 * 后端按该渠道的 base_urls × keys 组合逐一探测，返回与 /test 相同的汇总结构。
 * summary.passed === false 表示没有任何可用组合（全部 key 都不可用）。
 *
 * @example
 * const checkKeys = useCheckChannelKeys();
 * const summary = await checkKeys.mutateAsync(channelId);
 * if (!summary.passed) { // 提示可删除该渠道 }
 */
export function useCheckChannelKeys() {
    return useMutation({
        mutationFn: async (id: number) => {
            return apiClient.post<TestChannelSummary>(`/api/v1/channel/check-keys/${id}`);
        },
        onError: (error) => {
            logger.error('渠道 key 检查失败:', error);
        },
    });
}

/**
 * 单独测试某渠道中某模型的请求参数。
 *
 * 后端复用分组探测管道，对指定的 (channel, model, endpoint_type) 组合发起一次真实的
 * chat/embedding 探测请求（最多重试 3 次）。异步执行，立即返回进度 ID，
 * 调用方通过 useGroupTestProgress(progress.id) 轮询结果。
 *
 * endpoint_type 推荐按渠道类型传入：OpenAIEmbedding 渠道传 "embeddings"，
 * 其余聊天类渠道传 "*"（all）。
 */
export type ChannelModelTestRequest = {
    channel_id: number;
    model_name: string;
    endpoint_type: string;
};

/**
 * 测试指定渠道的指定模型 Hook。
 *
 * 后端路由: POST /api/v1/channel/test-model
 *
 * @example
 * const testChannelModel = useTestChannelModel();
 * const progress = await testChannelModel.mutateAsync({
 *   channel_id: 1,
 *   model_name: 'gpt-4o',
 *   endpoint_type: '*',
 * });
 * // 用 useGroupTestProgress(progress.id) 轮询进度
 */
export function useTestChannelModel() {
    return useMutation({
        mutationFn: async (data: ChannelModelTestRequest) => {
            return apiClient.post<GroupTestProgress>('/api/v1/channel/test-model', data);
        },
        onSuccess: (data) => {
            logger.log('渠道模型测试已启动:', data);
        },
        onError: (error) => {
            logger.error('渠道模型测试启动失败:', error);
        },
    });
}

/**
 * 获取渠道最后同步时间 Hook
 *
 * @example
 * const lastSyncTime = useLastSyncTime();
 *
 * if (lastSyncTime) {
 *   console.log('最后同步时间:', new Date(lastSyncTime).toLocaleString());
 * }
 */
export function useLastSyncTime() {
    return useQuery({
        queryKey: ['channels', 'last-sync-time'],
        queryFn: async () => {
            return apiClient.get<string>('/api/v1/channel/last-sync-time');
        },
        refetchInterval: REFETCH_INTERVAL_DEFAULT,
    });
}
/**
 * 同步渠道 Hook
 * 
 * @example
 * const syncChannel = useSyncChannel();
 * 
 * syncChannel.mutate();
 */
export function useSyncChannel() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async () => {
            return apiClient.post<null>('/api/v1/channel/sync');
        },
        onSuccess: () => {
            logger.log('渠道同步成功');
            queryClient.invalidateQueries({ queryKey: ['channels', 'list'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'channel'] });
            queryClient.invalidateQueries({ queryKey: ['models', 'market'] });
            queryClient.invalidateQueries({ queryKey: ['channels', 'last-sync-time'] });
        },
        onError: (error) => {
            logger.error('渠道同步失败:', error);
        },
    });
}
