import { apiClient } from '../client'

export interface ModelMapping {
  id: number
  name: string
  pattern: string
  match_type: 'exact' | 'wildcard' | 'regex'
  target_model: string
  priority: number
  enabled: boolean
  scope_group_id?: number | null
  created_at: string
  updated_at: string
}

export interface CreateModelMappingRequest {
  name: string
  pattern: string
  match_type: 'exact' | 'wildcard' | 'regex'
  target_model: string
  priority?: number
  enabled?: boolean
  scope_group_id?: number | null
}

export interface UpdateModelMappingRequest {
  name?: string
  pattern?: string
  match_type?: 'exact' | 'wildcard' | 'regex'
  target_model?: string
  priority?: number
  enabled?: boolean
  scope_group_id?: number | null
}

export function listModelMappings() {
  return apiClient.get<ModelMapping[]>('/api/v1/model-mapping')
}

export function getModelMapping(id: number) {
  return apiClient.get<ModelMapping>(`/api/v1/model-mapping/${id}`)
}

export function createModelMapping(data: CreateModelMappingRequest) {
  return apiClient.post<ModelMapping>('/api/v1/model-mapping', data)
}

export function updateModelMapping(id: number, data: UpdateModelMappingRequest) {
  return apiClient.put<ModelMapping>(`/api/v1/model-mapping/${id}`, data)
}

export function deleteModelMapping(id: number) {
  return apiClient.delete<void>(`/api/v1/model-mapping/${id}`)
}
