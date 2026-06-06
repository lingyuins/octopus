import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';
import { REFETCH_INTERVAL_DEFAULT } from '../constants';
import { logger } from '@/lib/logger';
import { useAuthStore } from './user';
import {
    formatAPIKeyStatsResponse,
    type APIKeyStatsResponse as APIKeyStatsResponseBase,
    type APIKeyStatsResponseFormatted as APIKeyStatsResponseFormattedBase,
} from './apikey-format';

/**
 * API Key 数据
 */
export interface APIKey {
    id: number;
    name: string;
    api_key: string;
    enabled: boolean;
    expire_at?: number; // Unix 时间戳（秒），不传表示永不过期
    max_cost?: number; // 不传表示无限制
    supported_models?: string; // 不传表示支持所有模型
    rate_limit_rpm?: number; // 每分钟请求数限制，0 表示无限制
    rate_limit_tpm?: number; // 每分钟 token 数限制，0 表示无限制
    per_model_quota_json?: string; // 按模型的配额 JSON: {"gpt-4o":{"rpm":5,"tpm":50000}}
    allowed_ips?: string; // 逗号分隔的允许 IP/CIDR 列表
}

/**
 * API Key Stats 响应（包含 stats 和 info）
 */
export interface APIKeyStatsResponse {
    stats: APIKeyStatsResponseBase<APIKey>['stats'];
    info: APIKey;
}

export type APIKeyStatsResponseFormatted = APIKeyStatsResponseFormattedBase<APIKey>;
export { formatAPIKeyStatsResponse };

/**
 * API Key 登录 Hook（仅校验 key 是否有效）
 */
export function useAPIKeyLogin() {
    const { setAPIKeyAuth, logout } = useAuthStore();

    return useMutation({
        mutationFn: async (apiKey: string) => {
            // 先设置以便 apiClient 发送请求时带上 token
            setAPIKeyAuth(apiKey);
            await apiClient.get<null>('/api/v1/apikey/login');
            return apiKey;
        },
        onError: (error) => {
            logout();
            logger.error('API Key 登录失败:', error);
        },
    });
}

/**
 * 获取当前 API Key 的详细统计数据 Hook（仅 API Key 登录用户使用）
 */
export function useAPIKeyDashboardStats() {
    const { isAPIKeyAuth, isAuthenticated } = useAuthStore();

    return useQuery({
        queryKey: ['apikey', 'dashboard', 'stats'],
        queryFn: () => apiClient.get<APIKeyStatsResponse>('/api/v1/apikey/stats'),
        select: formatAPIKeyStatsResponse,
        enabled: isAPIKeyAuth && isAuthenticated,
        refetchInterval: REFETCH_INTERVAL_DEFAULT,
    });
}

/**
 * 创建 API Key 请求
 */
export type CreateAPIKeyRequest = Omit<APIKey, 'id' | 'api_key'> & { enabled?: boolean };

/**
 * 更新 API Key 请求
 */
export type UpdateAPIKeyRequest = Pick<APIKey, 'id'> & CreateAPIKeyRequest;

/**
 * 获取 API Key 列表 Hook
 * 
 * @example
 * const { data: apiKeys, isLoading, error } = useAPIKeyList();
 * 
 * if (isLoading) return <Loading />;
 * if (error) return <Error message={error.message} />;
 * 
 * apiKeys?.forEach(key => console.log(key.name));
 */
export function useAPIKeyList() {
    return useQuery({
        queryKey: ['apikeys', 'list'],
        queryFn: async () => {
            return apiClient.get<APIKey[]>('/api/v1/apikey/list');
        },
        refetchInterval: REFETCH_INTERVAL_DEFAULT,
    });
}

/**
 * 创建 API Key Hook
 * 
 * @example
 * const createAPIKey = useCreateAPIKey();
 * 
 * createAPIKey.mutate({
 *   name: 'My API Key',
 * });
 */
export function useCreateAPIKey() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (data: CreateAPIKeyRequest) => {
            return apiClient.post<APIKey>('/api/v1/apikey/create', data);
        },
        onSuccess: (data) => {
            logger.log('API Key 创建成功:', data);
            queryClient.invalidateQueries({ queryKey: ['apikeys', 'list'] });
        },
        onError: (error) => {
            logger.error('API Key 创建失败:', error);
        },
    });
}

/**
 * 更新 API Key Hook
 * 
 * @example
 * const updateAPIKey = useUpdateAPIKey();
 * 
 * updateAPIKey.mutate({
 *   id: 1,
 *   name: 'Updated API Key',
 *   enabled: false,
 * });
 */
export function useUpdateAPIKey() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (data: UpdateAPIKeyRequest) => {
            return apiClient.post<APIKey>('/api/v1/apikey/update', data);
        },
        onSuccess: (data) => {
            logger.log('API Key 更新成功:', data);
            queryClient.invalidateQueries({ queryKey: ['apikeys', 'list'] });
        },
        onError: (error) => {
            logger.error('API Key 更新失败:', error);
        },
    });
}

/**
 * 删除 API Key Hook
 * 
 * @example
 * const deleteAPIKey = useDeleteAPIKey();
 * 
 * deleteAPIKey.mutate(1); // 删除 ID 为 1 的 API Key
 */
export function useDeleteAPIKey() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (id: number) => {
            return apiClient.delete<null>(`/api/v1/apikey/delete/${id}`);
        },
        onSuccess: () => {
            logger.log('API Key 删除成功');
            queryClient.invalidateQueries({ queryKey: ['apikeys', 'list'] });
        },
        onError: (error) => {
            logger.error('API Key 删除失败:', error);
        },
    });
}

/**
 * 获取当前 API Key 的统计数据 Hook
 * 
 * 此接口使用 API Key 认证，通过 API Key 获取对应的统计数据
 * 
 * @example
 * const { data: stats, isLoading } = useAPIKeyStats();
 */
export function useAPIKeyStats() {
    return useQuery({
        queryKey: ['apikey', 'stats'],
        queryFn: async () => {
            return apiClient.get<APIKeyStatsResponse>('/api/v1/apikey/stats');
        },
        select: formatAPIKeyStatsResponse,
        refetchInterval: REFETCH_INTERVAL_DEFAULT,
        refetchOnMount: 'always',
    });
}
