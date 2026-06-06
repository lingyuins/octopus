import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '../client'
import { REFETCH_INTERVAL_DEFAULT } from '../constants'

export interface WebDAVConfig {
  enabled: boolean
  base_url: string
  username: string
  password: string
  remote_path: string
  interval_hours: number
  include_stats: boolean
  include_logs: boolean
  max_backups: number
}

export interface WebDAVFile {
  name: string
  path: string
  size: number
  last_modified: string
  is_dir: boolean
}

export function useWebDAVConfig() {
  return useQuery({
    queryKey: ['webdav', 'config'],
    queryFn: () => apiClient.get<WebDAVConfig>('/api/v1/backup/webdav/config'),
    refetchInterval: REFETCH_INTERVAL_DEFAULT,
  })
}

export function useUpdateWebDAVConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (config: WebDAVConfig) =>
      apiClient.post('/api/v1/backup/webdav/config', config),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webdav', 'config'] })
    },
  })
}

export function useTestWebDAVConnection() {
  return useMutation({
    mutationFn: (config?: Partial<WebDAVConfig>) =>
      apiClient.post('/api/v1/backup/webdav/test', config || {}),
  })
}

export function useTriggerWebDAVBackup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => apiClient.post('/api/v1/backup/webdav/backup'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webdav', 'files'] })
    },
  })
}

export function useWebDAVFiles() {
  return useQuery({
    queryKey: ['webdav', 'files'],
    queryFn: () => apiClient.get<WebDAVFile[]>('/api/v1/backup/webdav/list'),
    refetchInterval: REFETCH_INTERVAL_DEFAULT,
  })
}

export function useRestoreWebDAVBackup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (filename: string) =>
      apiClient.post('/api/v1/backup/webdav/restore', { filename }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
    },
  })
}

export function useDeleteWebDAVBackup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (filename: string) =>
      apiClient.delete('/api/v1/backup/webdav/delete', { filename }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webdav', 'files'] })
    },
  })
}
