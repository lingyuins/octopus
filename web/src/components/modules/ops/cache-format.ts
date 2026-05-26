export function formatProviderPromptCacheCount(value: number | undefined) {
    const raw = value ?? 0;
    const formatted = raw >= 1_000_000
        ? { value: (raw / 1_000_000).toFixed(2), unit: 'M' }
        : raw >= 1_000
            ? { value: (raw / 1_000).toFixed(2), unit: 'K' }
            : { value: String(raw), unit: '' };

    return {
        value: formatted.value,
        unit: formatted.unit,
        text: `${formatted.value}${formatted.unit}`,
    };
}

export function getProviderPromptCacheTrendTokens(point: {
    cache_read_tokens?: number;
    cache_write_tokens?: number;
}) {
    return (point.cache_read_tokens ?? 0) + (point.cache_write_tokens ?? 0);
}
