import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';

export interface APICredentialProfile {
    id: number;
    name: string;
    api_type: string;
    base_url: string;
    api_key: string;
    tags: string;
    notes: string;
    last_verified_at: string | null;
    health_status: string;
    created_at: string;
    updated_at: string;
}

export interface APICredentialCreateRequest {
    name: string;
    api_type?: string;
    base_url: string;
    api_key: string;
    tags?: string;
    notes?: string;
}

export interface APICredentialUpdateRequest {
    id: number;
    name?: string;
    api_type?: string;
    base_url?: string;
    api_key?: string;
    tags?: string;
    notes?: string;
}

export interface VerificationResult {
    probe: string;
    success: boolean;
    latency_ms: number;
    error?: string;
    output?: string;
}

export interface CLIExportResult {
    tool: string;
    format: string;
    content: string;
    filename: string;
    description: string;
}

const CREDENTIAL_KEYS = {
    all: ['api-credentials'] as const,
    list: () => [...CREDENTIAL_KEYS.all, 'list'] as const,
};

export function useCredentialList() {
    return useQuery({
        queryKey: CREDENTIAL_KEYS.list(),
        queryFn: () => apiClient.get<APICredentialProfile[]>('/api/v1/api-credential/list'),
    });
}

export function useCreateCredential() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (data: APICredentialCreateRequest) =>
            apiClient.post<APICredentialProfile>('/api/v1/api-credential/create', data),
        onSuccess: () => qc.invalidateQueries({ queryKey: CREDENTIAL_KEYS.all }),
    });
}

export function useUpdateCredential() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (data: APICredentialUpdateRequest) =>
            apiClient.post<APICredentialProfile>('/api/v1/api-credential/update', data),
        onSuccess: () => qc.invalidateQueries({ queryKey: CREDENTIAL_KEYS.all }),
    });
}

export function useDeleteCredential() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (id: number) =>
            apiClient.delete<null>(`/api/v1/api-credential/delete/${id}`),
        onSuccess: () => qc.invalidateQueries({ queryKey: CREDENTIAL_KEYS.all }),
    });
}

export function useRunVerification() {
    return useMutation({
        mutationFn: (data: { base_url: string; api_key: string; api_type?: string; model?: string; probes?: string[] }) =>
            apiClient.post<VerificationResult[]>('/api/v1/verification/run', data),
    });
}

export function useRunVerificationForProfile() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: ({ id, model, probes }: { id: number; model?: string; probes?: string[] }) =>
            apiClient.post<VerificationResult[]>(`/api/v1/verification/run-for/${id}`, { model, probes }),
        onSuccess: () => qc.invalidateQueries({ queryKey: CREDENTIAL_KEYS.all }),
    });
}

export function useGenerateCLIExport() {
    return useMutation({
        mutationFn: (data: { base_url: string; api_key: string; api_type?: string; tool: string }) =>
            apiClient.post<CLIExportResult>('/api/v1/cli-export/generate', data),
    });
}

export const API_TYPES = ['openai', 'anthropic', 'gemini'] as const;
export const CLI_TOOLS = ['claude_code', 'codex', 'gemini_cli', 'cherry_studio', 'kilo_code'] as const;
