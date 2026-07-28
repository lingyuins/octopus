export interface RuntimeSettingField {
    key: string;
    labelKey: string;
    placeholderKey: string;
    hintKey?: string;
    min: string;
    max?: string;
}

export const RETRY_FIELDS: RuntimeSettingField[] = [
    {
        key: 'relay_retry_count',
        labelKey: 'retry.count.label',
        hintKey: 'retry.count.hint',
        placeholderKey: 'retry.count.placeholder',
        min: '0',
    },
    {
        key: 'relay_route_retries',
        labelKey: 'retry.routeRetries.label',
        hintKey: 'retry.routeRetries.hint',
        placeholderKey: 'retry.routeRetries.placeholder',
        min: '1',
    },
    {
        key: 'relay_max_total_attempts',
        labelKey: 'retry.maxTotalAttempts.label',
        placeholderKey: 'retry.maxTotalAttempts.placeholder',
        hintKey: 'retry.maxTotalAttempts.hint',
        min: '0',
    },
    {
        key: 'ratelimit_cooldown',
        labelKey: 'retry.ratelimitCooldown.label',
        placeholderKey: 'retry.ratelimitCooldown.placeholder',
        hintKey: 'retry.ratelimitCooldown.hint',
        min: '0',
    },
    {
        key: 'rate_limit_hold_interval',
        labelKey: 'retry.rateLimitHold.interval.label',
        placeholderKey: 'retry.rateLimitHold.interval.placeholder',
        hintKey: 'retry.rateLimitHold.interval.hint',
        min: '1',
    },
    {
        key: 'rate_limit_hold_max_wait',
        labelKey: 'retry.rateLimitHold.maxWait.label',
        placeholderKey: 'retry.rateLimitHold.maxWait.placeholder',
        hintKey: 'retry.rateLimitHold.maxWait.hint',
        min: '1',
    },
];

export const AUTO_STRATEGY_FIELDS: RuntimeSettingField[] = [
    {
        key: 'auto_strategy_min_samples',
        labelKey: 'autoStrategy.minSamples.label',
        placeholderKey: 'autoStrategy.minSamples.placeholder',
        hintKey: 'autoStrategy.minSamples.hint',
        min: '1',
    },
    {
        key: 'auto_strategy_time_window',
        labelKey: 'autoStrategy.timeWindow.label',
        placeholderKey: 'autoStrategy.timeWindow.placeholder',
        hintKey: 'autoStrategy.timeWindow.hint',
        min: '1',
    },
    {
        key: 'auto_strategy_sample_threshold',
        labelKey: 'autoStrategy.sampleThreshold.label',
        placeholderKey: 'autoStrategy.sampleThreshold.placeholder',
        hintKey: 'autoStrategy.sampleThreshold.hint',
        min: '1',
    },
    {
        key: 'auto_strategy_latency_weight',
        labelKey: 'autoStrategy.latencyWeight.label',
        placeholderKey: 'autoStrategy.latencyWeight.placeholder',
        hintKey: 'autoStrategy.latencyWeight.hint',
        min: '0',
        max: '100',
    },
];
