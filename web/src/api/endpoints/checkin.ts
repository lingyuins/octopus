import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';

export interface CheckInRecord {
    id: number;
    remote_site_id: number;
    check_in_date: string;
    status: string;
    message: string;
    quota_awarded: number;
    executed_at: string;
}

const CHECKIN_KEYS = {
    all: ['checkin'] as const,
    status: (siteId: number) => [...CHECKIN_KEYS.all, 'status', siteId] as const,
    history: (siteId: number) => [...CHECKIN_KEYS.all, 'history', siteId] as const,
};

export function useCheckInStatus(siteId: number) {
    return useQuery({
        queryKey: CHECKIN_KEYS.status(siteId),
        queryFn: () => apiClient.get<CheckInRecord | null>(`/api/v1/checkin/status/${siteId}`),
        enabled: siteId > 0,
    });
}

export function useCheckInHistory(siteId: number, limit = 30) {
    return useQuery({
        queryKey: CHECKIN_KEYS.history(siteId),
        queryFn: () => apiClient.get<CheckInRecord[]>(`/api/v1/checkin/history/${siteId}`, { limit }),
        enabled: siteId > 0,
    });
}

export function useExecuteCheckIn() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (siteId: number) =>
            apiClient.post<CheckInRecord>(`/api/v1/checkin/execute/${siteId}`),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: CHECKIN_KEYS.all });
        },
    });
}

export function useExecuteCheckInAll() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: () =>
            apiClient.post<CheckInRecord[]>('/api/v1/checkin/execute-all'),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: CHECKIN_KEYS.all });
        },
    });
}
