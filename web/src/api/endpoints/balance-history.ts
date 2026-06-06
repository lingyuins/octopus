import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';

export interface BalanceSnapshot {
    id: number;
    remote_site_id: number;
    day_key: string;
    quota: number;
    captured_at: string;
    source: string;
}

export interface BalanceChartPoint {
    day_key: string;
    quota: number;
}

const BALANCE_KEYS = {
    all: ['balance-history'] as const,
    list: (siteId: number) => [...BALANCE_KEYS.all, 'list', siteId] as const,
    chart: (siteId: number) => [...BALANCE_KEYS.all, 'chart', siteId] as const,
};

export function useBalanceSnapshots(siteId: number, startDate?: string, endDate?: string) {
    const params: Record<string, string> = {};
    if (startDate) params.start_date = startDate;
    if (endDate) params.end_date = endDate;

    return useQuery({
        queryKey: [...BALANCE_KEYS.list(siteId), startDate, endDate],
        queryFn: () => apiClient.get<BalanceSnapshot[]>(`/api/v1/balance-history/list/${siteId}`, params),
        enabled: siteId > 0,
    });
}

export function useBalanceChart(siteId: number, startDate?: string, endDate?: string) {
    const params: Record<string, string> = {};
    if (startDate) params.start_date = startDate;
    if (endDate) params.end_date = endDate;

    return useQuery({
        queryKey: [...BALANCE_KEYS.chart(siteId), startDate, endDate],
        queryFn: () => apiClient.get<BalanceChartPoint[]>(`/api/v1/balance-history/chart/${siteId}`, params),
        enabled: siteId > 0,
    });
}

export function useCaptureBalance() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (siteId: number) =>
            apiClient.post<BalanceSnapshot>(`/api/v1/balance-history/capture/${siteId}`),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: BALANCE_KEYS.all });
        },
    });
}

export interface BalancePrediction {
    daily_burn_rate: number;
    days_remaining: number;
    estimated_zero_at: string;
    seven_day_avg_burn: number;
    thirty_day_avg_burn: number;
    current_quota: number;
    trend_points: BalanceChartPoint[];
}

export function useBalancePrediction(siteId: number) {
    return useQuery({
        queryKey: ['balance-history', 'prediction', siteId],
        queryFn: () => apiClient.get<BalancePrediction>(`/api/v1/balance-history/prediction/${siteId}`),
        enabled: siteId > 0,
    });
}
