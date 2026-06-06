import { apiClient } from '../client'

export interface RemoteUsageRecord {
  id: number
  remote_site_id: number
  day_key: string
  hour: number
  model_name: string
  token_name: string
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  quota_consumed: number
  remote_log_id: number
  fingerprint: string
  synced_at: string
}

export interface RemoteUsageSummary {
  day_key: string
  model_name?: string
  token_name?: string
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  quota_consumed: number
}

export interface RemoteUsageHourly {
  hour: number
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
}

export interface UsageQuery {
  site_id?: number
  day_from?: string
  day_to?: string
  model_name?: string
  token_name?: string
  limit?: number
  offset?: number
}

export function queryUsageHistory(query: UsageQuery) {
  const params = new URLSearchParams()
  if (query.site_id) params.append('site_id', String(query.site_id))
  if (query.day_from) params.append('day_from', query.day_from)
  if (query.day_to) params.append('day_to', query.day_to)
  if (query.model_name) params.append('model_name', query.model_name)
  if (query.token_name) params.append('token_name', query.token_name)
  if (query.limit) params.append('limit', String(query.limit))
  if (query.offset) params.append('offset', String(query.offset))

  return apiClient.get<{ records: RemoteUsageRecord[]; total: number; limit: number; offset: number }>(
    `/api/v1/usage-history?${params.toString()}`
  )
}

export function queryUsageSummary(query: Omit<UsageQuery, 'limit' | 'offset'>) {
  const params = new URLSearchParams()
  if (query.site_id) params.append('site_id', String(query.site_id))
  if (query.day_from) params.append('day_from', query.day_from)
  if (query.day_to) params.append('day_to', query.day_to)
  if (query.model_name) params.append('model_name', query.model_name)
  if (query.token_name) params.append('token_name', query.token_name)

  return apiClient.get<RemoteUsageSummary[]>(`/api/v1/usage-history/summary?${params.toString()}`)
}

export function queryUsageHourly(siteId: number, dayKey: string) {
  const params = new URLSearchParams()
  if (siteId) params.append('site_id', String(siteId))
  if (dayKey) params.append('day_key', dayKey)

  return apiClient.get<RemoteUsageHourly[]>(`/api/v1/usage-history/hourly?${params.toString()}`)
}

export function getUsageModels(siteId: number) {
  return apiClient.get<string[]>(`/api/v1/usage-history/models/${siteId}`)
}

export function syncUsageHistory(siteId: number) {
  return apiClient.post<{ inserted: number }>(`/api/v1/usage-history/sync/${siteId}`)
}

export function syncAllUsageHistory() {
  return apiClient.post<{ inserted: number }>('/api/v1/usage-history/sync-all')
}
