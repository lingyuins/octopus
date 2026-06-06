import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';

export interface RedemptionRecord {
  id: number;
  remote_site_id: number;
  code: string;
  status: 'success' | 'already_used' | 'invalid' | 'failed';
  quota_awarded: number;
  message: string;
  executed_at: string;
}

export interface RedemptionBatchResult {
  total_codes: number;
  success_count: number;
  failed_count: number;
  results: RedemptionRecord[];
}

export interface RedemptionRequest {
  site_id: number;
  codes: string[];
}

export interface RedeemAllSitesRequest {
  codes: string[];
}

const REDEMPTION_KEYS = {
  all: ['redemption'] as const,
  history: (siteId: number) => [...REDEMPTION_KEYS.all, 'history', siteId] as const,
};

export function useRedemptionHistory(siteId: number, limit = 50) {
  return useQuery({
    queryKey: REDEMPTION_KEYS.history(siteId),
    queryFn: () => apiClient.get<RedemptionRecord[]>(`/api/v1/redemption/history/${siteId}`, { limit }),
  });
}

export function useRedeemCodes() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: RedemptionRequest) =>
      apiClient.post<RedemptionBatchResult>('/api/v1/redemption/redeem', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: REDEMPTION_KEYS.all });
    },
  });
}

export function useRedeemAllSites() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: RedeemAllSitesRequest) =>
      apiClient.post<RedemptionRecord[]>('/api/v1/redemption/redeem-all', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: REDEMPTION_KEYS.all });
    },
  });
}
