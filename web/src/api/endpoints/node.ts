import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';
import type { NodeProbeResult } from './node-probe';

export function useTestNode() {
    const queryClient = useQueryClient();
    return useMutation({
        gcTime: 0,
        mutationFn: (id: number) => apiClient.post<NodeProbeResult>(`/api/v1/nodes/${id}/test`),
        onSettled: () => queryClient.invalidateQueries({ queryKey: ['nodes'] }),
        meta: { skipGlobalErrorHandler: true },
    });
}

export type LocalNode = {
    id: number;
    name: string;
    enabled: boolean;
    online: boolean;
    last_seen_at: string | null;
    connector_version: string;
    in_flight: number;
    max_concurrency: number;
    last_error: string;
    created_at: string;
    updated_at: string;
};

export function useNodeList(enabled = true) {
    return useQuery({
        queryKey: ['nodes'],
        queryFn: () => apiClient.get<{ nodes: LocalNode[] | null; can_write: boolean }>('/api/v1/nodes'),
        select: (data) => ({ nodes: data.nodes ?? [], can_write: data.can_write === true }),
        enabled,
        refetchInterval: enabled ? 10_000 : false,
    });
}

export type NodeAction =
    | { action: 'create'; name: string; max_concurrency?: number }
    | { action: 'update'; id: number; name?: string; enabled?: boolean; max_concurrency?: number }
    | { action: 'rotate'; id: number }
    | { action: 'delete'; id: number };

/** Secrets go straight to the caller's transient state, never into mutation/query cache. */
export function useNodeAction(onToken: (node: LocalNode, token: string) => void) {
    const queryClient = useQueryClient();
    return useMutation({
        gcTime: 0,
        mutationFn: async (input: NodeAction): Promise<void> => {
            switch (input.action) {
                case 'create': {
                    const result = await apiClient.post<{ node: LocalNode; token: string }>('/api/v1/nodes', { name: input.name, max_concurrency: input.max_concurrency });
                    onToken(result.node, result.token);
                    break;
                }
                case 'rotate': {
                    const result = await apiClient.post<{ node: LocalNode; token: string }>(`/api/v1/nodes/${input.id}/rotate-token`);
                    onToken(result.node, result.token);
                    break;
                }
                case 'update':
                    await apiClient.put<LocalNode>(`/api/v1/nodes/${input.id}`, { name: input.name, enabled: input.enabled, max_concurrency: input.max_concurrency });
                    break;
                case 'delete':
                    await apiClient.delete(`/api/v1/nodes/${input.id}`);
            }
        },
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['nodes'] }),
        meta: { skipGlobalErrorHandler: true },
    });
}
