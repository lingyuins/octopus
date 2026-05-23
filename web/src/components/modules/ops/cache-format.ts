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
