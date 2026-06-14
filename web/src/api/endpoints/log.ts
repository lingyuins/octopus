import type { InfiniteData } from '@tanstack/react-query';
import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient, API_BASE_URL } from '../client';
import { logger } from '@/lib/logger';
import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import { useAuthStore } from './user';

/**
 * 尝试状态
 */
export type AttemptStatus = 'success' | 'failed' | 'circuit_break' | 'skipped';

/**
 * 单次渠道尝试信息
 */
export interface ChannelAttempt {
    channel_id: number;
    channel_key_id?: number;
    channel_name: string;
    model_name: string;
    adapter_type?: string;   // 适配器类型: response, chat, anthropic, gemini 等
    attempt_num: number;    // 第几次尝试
    status: AttemptStatus;
    duration: number;       // 耗时(毫秒)
    sticky?: boolean;
    msg?: string;
}

/**
 * 日志数据（列表条目，不含 request_content / response_content）
 */
export interface RelayLog {
    id: number;
    time: number;                // 时间戳
    request_model_name: string;  // 请求模型名称
    request_api_key_id?: number;   // 请求使用的 API Key ID
    request_api_key_name?: string; // 请求使用的 API Key 名称
    client_ip?: string;          // 客户端 IP
    endpoint_type?: string;      // 命中的端点分类
    channel: number;             // 实际使用的渠道ID
    channel_name: string;        // 渠道名称
    actual_model_name: string;   // 实际使用模型名称
    input_tokens: number;        // 输入Token
    output_tokens: number;       // 输出Token
    semantic_cache_hit?: boolean;// 语义缓存命中
    cache_read_tokens?: number;  // 提供方提示缓存命中 Token
    ftut: number;                // 首字时间(毫秒)
    use_time: number;            // 总用时(毫秒)
    cost: number;                // 消耗费用
    error: string;               // 错误信息
    attempts?: ChannelAttempt[]; // 所有尝试记录
    total_attempts?: number;     // 总尝试次数
}

/**
 * 日志详情（包含 request_content 和 response_content）
 */
export interface RelayLogDetail extends RelayLog {
    request_content: string;     // 请求内容
    response_content: string;    // 响应内容
}

/**
 * 日志列表查询参数
 */
export interface LogListParams {
    page?: number;
    page_size?: number;
    start_time?: number;
    end_time?: number;
    model?: string;
    channel_id?: number;
    api_key_id?: number;
    endpoint_type?: string;
    status?: 'success' | 'error';
}

/**
 * 日志筛选条件（前端用）
 */
export interface LogFilter {
    model?: string;
    channel_id?: number;
    api_key_id?: number;
    endpoint_type?: string;
    status?: 'success' | 'error';
    start_time?: number;
    end_time?: number;
    // 是否"穿透"到单次尝试维度：按渠道A 筛选时也能命中"在A 失败、重试到B 成功"
    // 的请求。后端在指定 channel_id 且未显式传 include_attempts 时默认开启。
    include_attempts?: boolean;
}

/**
 * 清空日志 Hook
 * 
 * @example
 * const clearLogs = useClearLogs();
 * 
 * clearLogs.mutate();
 */
export function useClearLogs() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async () => {
            return apiClient.delete<null>('/api/v1/log/clear');
        },
        onSuccess: () => {
            logger.log('日志清空成功');
            queryClient.invalidateQueries({ queryKey: ['logs'] });
        },
        onError: (error) => {
            logger.error('日志清空失败:', error);
        },
    });
}

const logsInfiniteQueryKey = (pageSize: number, filter: LogFilter) => ['logs', 'infinite', pageSize, filter] as const;

export const DEFAULT_LOG_PAGE_SIZE = 10;

const logRefreshState = new Map<number, boolean>();
const logRefreshListeners = new Set<() => void>();

function setLogRefreshState(pageSize: number, isRefreshing: boolean) {
    if (logRefreshState.get(pageSize) === isRefreshing) return;
    logRefreshState.set(pageSize, isRefreshing);
    logRefreshListeners.forEach((listener) => listener());
}

function subscribeLogRefresh(listener: () => void) {
    logRefreshListeners.add(listener);
    return () => {
        logRefreshListeners.delete(listener);
    };
}

/**
 * 日志管理 Hook
 * 整合初始加载、SSE 实时推送、滚动加载更多
 * 
 * @example
 * const { logs, isConnected, hasMore, isLoadingMore, loadMore, clear } = useLogs();
 * 
 * // logs 自动包含历史日志和实时日志，按时间倒序
 * logs.forEach(log => console.log(log.request_model_name));
 * 
 * // 滚动到底部时加载更多
 * if (hasMore && !isLoadingMore) loadMore();
 */
export function useLogs(options: { pageSize?: number; filter?: LogFilter } = {}) {
    const { pageSize = DEFAULT_LOG_PAGE_SIZE, filter = {} } = options;
    const { refresh } = useLogRefresh(pageSize, filter);
    const token = useAuthStore((state) => state.token);

    const [isConnected, setIsConnected] = useState(false);
    const [error, setError] = useState<Error | null>(null);
    const abortRef = useRef<AbortController | null>(null);

    const queryClient = useQueryClient();

    // Stable filter reference to avoid unnecessary re-renders
    // eslint-disable-next-line react-hooks/exhaustive-deps
    const stableFilter = useMemo(() => filter, [
        filter.model,
        filter.channel_id,
        filter.api_key_id,
        filter.endpoint_type,
        filter.status,
        filter.start_time,
        filter.end_time,
        filter.include_attempts,
    ]);

    const logsQuery = useInfiniteQuery({
        queryKey: logsInfiniteQueryKey(pageSize, stableFilter),
        initialPageParam: 1,
        queryFn: async ({ pageParam }) => {
            const params = new URLSearchParams();
            params.set('page', String(pageParam));
            params.set('page_size', String(pageSize));
            if (stableFilter.model) params.set('model', stableFilter.model);
            if (stableFilter.channel_id != null) params.set('channel_id', String(stableFilter.channel_id));
            if (stableFilter.api_key_id != null) params.set('api_key_id', String(stableFilter.api_key_id));
            if (stableFilter.endpoint_type) params.set('endpoint_type', stableFilter.endpoint_type);
            if (stableFilter.status) params.set('status', stableFilter.status);
            if (stableFilter.start_time != null) params.set('start_time', String(stableFilter.start_time));
            if (stableFilter.end_time != null) params.set('end_time', String(stableFilter.end_time));
            if (stableFilter.include_attempts != null) params.set('include_attempts', String(stableFilter.include_attempts));
            const result = await apiClient.get<RelayLog[] | null>(`/api/v1/log/list?${params.toString()}`);
            return result ?? [];
        },
        getNextPageParam: (lastPage, allPages) => {
            if (!lastPage || lastPage.length < pageSize) return undefined;
            return allPages.length + 1;
        },
        staleTime: Infinity,
        refetchOnMount: 'always',
    });

    const logs = useMemo(() => {
        const pages = logsQuery.data?.pages ?? [];
        const seen = new Set<number>();
        const merged: RelayLog[] = [];

        for (const page of pages) {
            for (const log of page) {
                if (seen.has(log.id)) continue;
                seen.add(log.id);
                merged.push(log);
            }
        }

        merged.sort((a, b) => b.time - a.time);
        return merged;
    }, [logsQuery.data]);

    const loadMore = useCallback(async () => {
        if (!logsQuery.hasNextPage) return;
        if (logsQuery.isFetchingNextPage) return;

        try {
            await logsQuery.fetchNextPage();
        } catch (e) {
            logger.error('加载更多日志失败:', e);
        }
    }, [logsQuery]);

    useEffect(() => {
        let cancelled = false;
        let retryTimer: number | null = null;
        let retryAttempt = 0;

        const waitForRetry = (delayMs: number) =>
            new Promise<void>((resolve) => {
                retryTimer = window.setTimeout(() => {
                    retryTimer = null;
                    resolve();
                }, delayMs);
            });

        const hasActiveFilter = !!(stableFilter.model || stableFilter.channel_id != null || stableFilter.api_key_id != null || stableFilter.endpoint_type || stableFilter.status || stableFilter.start_time != null || stableFilter.end_time != null || stableFilter.include_attempts != null);

        const mergeIncomingLog = (log: RelayLog) => {
            // When a filter is active, skip merging SSE logs to avoid showing unfiltered results
            if (hasActiveFilter) return;
            queryClient.setQueryData(
                logsInfiniteQueryKey(pageSize, stableFilter),
                (old: InfiniteData<RelayLog[], number> | undefined) => {
                    if (!old) {
                        return { pages: [[log]], pageParams: [1] };
                    }

                    const exists = old.pages.some((page) => page?.some((item) => item.id === log.id));
                    if (exists) return old;

                    const firstPage = old.pages[0] ?? [];
                    return { ...old, pages: [[log, ...firstPage], ...old.pages.slice(1)] };
                }
            );
        };

        const connect = async () => {
            if (!token) {
                setIsConnected(false);
                setError(new Error('Not authenticated, cannot establish log stream'));
                return;
            }

            while (!cancelled) {
                try {
                    const controller = new AbortController();
                    abortRef.current = controller;

                    const response = await fetch(`${API_BASE_URL}/api/v1/log/stream`, {
                        method: 'GET',
                        headers: {
                            Authorization: `Bearer ${token}`,
                        },
                        signal: controller.signal,
                    });
                    if (cancelled) return;
                    if (!response.ok) {
                        throw new Error(`Log stream connection failed: ${response.status}`);
                    }
                    if (!response.body) {
                        throw new Error('Log stream response is empty');
                    }

                    retryAttempt = 0;
                    setIsConnected(true);
                    setError(null);

                    const reader = response.body.getReader();
                    const decoder = new TextDecoder();
                    let buffer = '';

                    const handleEvent = (chunk: string) => {
                        const lines = chunk.split('\n');
                        const dataLines: string[] = [];
                        for (const line of lines) {
                            if (line.startsWith('data:')) {
                                dataLines.push(line.slice(5).trimStart());
                            }
                        }
                        if (dataLines.length === 0) return;

                        try {
                            const log: RelayLog = JSON.parse(dataLines.join('\n'));
                            mergeIncomingLog(log);
                        } catch (e) {
                            logger.error('解析日志数据失败:', e);
                        }
                    };

                    while (!cancelled) {
                        const { value, done } = await reader.read();
                        if (done) break;
                        buffer += decoder.decode(value, { stream: true });

                        let boundary = buffer.indexOf('\n\n');
                        while (boundary >= 0) {
                            const eventChunk = buffer.slice(0, boundary);
                            buffer = buffer.slice(boundary + 2);
                            handleEvent(eventChunk);
                            boundary = buffer.indexOf('\n\n');
                        }
                    }

                    if (cancelled) return;

                    setIsConnected(false);
                    setError(new Error('Log stream disconnected, reconnecting...'));
                    logger.warn('日志流连接已断开，准备重连');
                } catch (e) {
                    if (cancelled) return;
                    if (e instanceof Error && e.name === 'AbortError') {
                        return;
                    }

                    setIsConnected(false);
                    setError(e instanceof Error ? e : new Error('Log stream connection failed'));
                    logger.warn('日志流连接失败，准备重连:', e);
                } finally {
                    abortRef.current = null;
                }

                const delayMs = Math.min(1000 * 2 ** retryAttempt, 10000);
                retryAttempt += 1;
                await waitForRetry(delayMs);
            }
        };

        connect();

        return () => {
            cancelled = true;
            if (retryTimer !== null) {
                window.clearTimeout(retryTimer);
            }
            abortRef.current?.abort();
            abortRef.current = null;
            setIsConnected(false);
        };
    }, [pageSize, stableFilter, queryClient, token]);

    const clear = useCallback(() => {
        queryClient.removeQueries({ queryKey: logsInfiniteQueryKey(pageSize, stableFilter) });
    }, [pageSize, stableFilter, queryClient]);

    return {
        logs,
        isConnected,
        error,
        hasMore: !!logsQuery.hasNextPage,
        isLoading: logsQuery.isLoading,
        isLoadingMore: logsQuery.isFetchingNextPage,
        isRefreshing: logsQuery.isRefetching,
        loadMore,
        refresh,
        clear,
    };
}

export function useLogRefresh(pageSize = DEFAULT_LOG_PAGE_SIZE, filter: LogFilter = {}) {
    const queryClient = useQueryClient();
    const isRefreshing = useSyncExternalStore(
        subscribeLogRefresh,
        () => logRefreshState.get(pageSize) ?? false,
        () => false,
    );

    const refresh = useCallback(async () => {
        setLogRefreshState(pageSize, true);
        try {
            await queryClient.refetchQueries({ queryKey: logsInfiniteQueryKey(pageSize, filter) });
        } catch (e) {
            logger.error('手动刷新日志失败:', e);
            throw e;
        } finally {
            setLogRefreshState(pageSize, false);
        }
    }, [pageSize, filter, queryClient]);

    return { isRefreshing, refresh };
}

/**
 * 日志详情 Hook
 * 按需加载单条日志的 request_content 和 response_content
 *
 * @example
 * const { detail, isLoading, fetchDetail } = useLogDetail();
 * await fetchDetail(logId);
 */
export function useLogDetail() {
    const [detail, setDetail] = useState<RelayLogDetail | null>(null);
    const [isLoading, setIsLoading] = useState(false);

    const fetchDetail = useCallback(async (id: number) => {
        setIsLoading(true);
        try {
            const result = await apiClient.get<RelayLogDetail | null>(`/api/v1/log/detail?id=${id}`);
            setDetail(result);
        } catch (e) {
            logger.error('获取日志详情失败:', e);
            setDetail(null);
        } finally {
            setIsLoading(false);
        }
    }, []);

    const reset = useCallback(() => {
        setDetail(null);
    }, []);

    return { detail, isLoading, fetchDetail, reset };
}
