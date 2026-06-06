import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';

export interface RemoteSite {
    id: number;
    name: string;
    base_url: string;
    site_type: string;
    auth_type: string;
    access_token: string;
    username: string;
    password: string;
    exchange_rate: number;
    enabled: boolean;
    tags: string;
    notes: string;
    pinned: boolean;
    sort_order: number;
    remote_user_id: number;
    remote_username: string;
    quota: number;
    health_status: string;
    health_message: string;
    last_sync_at: string | null;
    created_at: string;
    updated_at: string;
}

export interface RemoteSiteCreateRequest {
    name: string;
    base_url: string;
    site_type: string;
    auth_type?: string;
    access_token?: string;
    username?: string;
    password?: string;
    exchange_rate?: number;
    enabled?: boolean;
    tags?: string;
    notes?: string;
}

export interface RemoteSiteUpdateRequest {
    id: number;
    name?: string;
    base_url?: string;
    site_type?: string;
    auth_type?: string;
    access_token?: string;
    username?: string;
    password?: string;
    exchange_rate?: number;
    enabled?: boolean;
    tags?: string;
    notes?: string;
    pinned?: boolean;
    sort_order?: number;
}

export interface RefreshResult {
    user_info?: {
        id: number;
        username: string;
        quota: number;
    };
    site_status?: {
        checkin_enabled: boolean;
        price: number;
        system_name: string;
    };
    quota: number;
    health_status: string;
    health_message: string;
    synced_at: string;
}

export interface ModelPricingEntry {
    model_name: string;
    quota: number;
    completion_ratio: number;
    group_ratio: number;
}

export const SITE_TYPES = [
    'new-api', 'veloera', 'done-hub', 'one-hub', 'sub2api',
    'octopus', 'sapi', 'anyrouter', 'aihubmix', 'axonhub', 'claude-code-hub', 'unknown',
] as const;

export type SiteType = (typeof SITE_TYPES)[number];

const REMOTE_SITE_KEYS = {
    all: ['remote-sites'] as const,
    list: () => [...REMOTE_SITE_KEYS.all, 'list'] as const,
    detail: (id: number) => [...REMOTE_SITE_KEYS.all, 'detail', id] as const,
    models: (id: number) => [...REMOTE_SITE_KEYS.all, 'models', id] as const,
    pricing: (id: number) => [...REMOTE_SITE_KEYS.all, 'pricing', id] as const,
};

export function useRemoteSiteList() {
    return useQuery({
        queryKey: REMOTE_SITE_KEYS.list(),
        queryFn: () => apiClient.get<RemoteSite[]>('/api/v1/remote-site/list'),
    });
}

export function useRemoteSiteModels(id: number, enabled = true) {
    return useQuery({
        queryKey: REMOTE_SITE_KEYS.models(id),
        queryFn: () => apiClient.get<string[]>(`/api/v1/remote-site/models/${id}`),
        enabled: enabled && id > 0,
    });
}

export function useRemoteSitePricing(id: number, enabled = true) {
    return useQuery({
        queryKey: REMOTE_SITE_KEYS.pricing(id),
        queryFn: () => apiClient.get<ModelPricingEntry[]>(`/api/v1/remote-site/pricing/${id}`),
        enabled: enabled && id > 0,
    });
}

export function useCreateRemoteSite() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: RemoteSiteCreateRequest) =>
            apiClient.post<RemoteSite>('/api/v1/remote-site/create', data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: REMOTE_SITE_KEYS.all });
        },
    });
}

export function useUpdateRemoteSite() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: RemoteSiteUpdateRequest) =>
            apiClient.post<RemoteSite>('/api/v1/remote-site/update', data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: REMOTE_SITE_KEYS.all });
        },
    });
}

export function useDeleteRemoteSite() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (id: number) =>
            apiClient.delete<null>(`/api/v1/remote-site/delete/${id}`),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: REMOTE_SITE_KEYS.all });
        },
    });
}

export function useRefreshRemoteSite() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (id: number) =>
            apiClient.post<RefreshResult>(`/api/v1/remote-site/refresh/${id}`),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: REMOTE_SITE_KEYS.all });
        },
    });
}

export function useRefreshAllRemoteSites() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: () =>
            apiClient.post<Record<number, RefreshResult>>('/api/v1/remote-site/refresh-all'),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: REMOTE_SITE_KEYS.all });
        },
    });
}

export function useDetectSiteType() {
    return useMutation({
        mutationFn: (data: { base_url: string; access_token?: string }) =>
            apiClient.post<{ site_type: string }>('/api/v1/remote-site/detect', data),
    });
}
