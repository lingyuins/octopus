export const CC_SWITCH_APPS = [
    { value: 'claude', label: 'Claude Code' },
    { value: 'codex', label: 'Codex' },
    { value: 'gemini', label: 'Gemini' },
    { value: 'opencode', label: 'OpenCode' },
    { value: 'openclaw', label: 'OpenClaw' },
] as const;

export type CCSwitchApp = (typeof CC_SWITCH_APPS)[number]['value'];

export function normalizeCCSwitchEndpoint(baseUrl: string): string {
    const trimmed = baseUrl.trim().replace(/\/+$/, '');
    if (!trimmed) return '';
    return trimmed.endsWith('/v1') ? trimmed : `${trimmed}/v1`;
}

export function maskCCSwitchSecret(value: string): string {
    const trimmed = value.trim();
    if (trimmed.length <= 10) return trimmed;
    return `${trimmed.slice(0, 6)}...${trimmed.slice(-4)}`;
}

export function buildCCSwitchProviderLink({
    app,
    endpoint,
    apiKey,
    name,
    model,
    notes,
}: {
    app: CCSwitchApp;
    endpoint: string;
    apiKey: string;
    name: string;
    model?: string;
    notes?: string;
}): string {
    const params = new URLSearchParams();
    params.set('resource', 'provider');
    params.set('app', app);
    params.set('name', name.trim());
    params.set('endpoint', endpoint.trim());
    params.set('apiKey', apiKey.trim());

    const normalizedModel = model?.trim();
    if (normalizedModel) {
        params.set('model', normalizedModel);
    }

    const normalizedNotes = notes?.trim();
    if (normalizedNotes) {
        params.set('notes', normalizedNotes);
    }

    params.set('enabled', 'true');
    return `ccswitch://v1/import?${params.toString()}`;
}
