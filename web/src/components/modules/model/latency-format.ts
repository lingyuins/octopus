export type LatencyUnitMode = 'auto' | 'ms' | 's' | 'h';

const MILLISECONDS_PER_SECOND = 1000;
const MILLISECONDS_PER_HOUR = 60 * 60 * MILLISECONDS_PER_SECOND;

function trimFraction(value: number): string {
    return value.toFixed(2).replace(/\.00$/, '').replace(/(\.\d)0$/, '$1');
}

function resolveLatencyUnit(latencyMs: number, mode: LatencyUnitMode): Exclude<LatencyUnitMode, 'auto'> {
    if (mode !== 'auto') return mode;
    if (latencyMs >= MILLISECONDS_PER_HOUR) return 'h';
    if (latencyMs >= MILLISECONDS_PER_SECOND) return 's';
    return 'ms';
}

export function formatAverageLatency(latencyMs: number, requestCount: number, mode: LatencyUnitMode): string {
    if (requestCount <= 0 || latencyMs <= 0) return '—';

    const unit = resolveLatencyUnit(latencyMs, mode);
    switch (unit) {
        case 'h':
            return `${trimFraction(latencyMs / MILLISECONDS_PER_HOUR)}h`;
        case 's':
            return `${trimFraction(latencyMs / MILLISECONDS_PER_SECOND)}s`;
        case 'ms':
            return `${Math.round(latencyMs)}ms`;
    }
}
