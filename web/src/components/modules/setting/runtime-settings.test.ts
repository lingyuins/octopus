import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';

import { AUTO_STRATEGY_FIELDS, RETRY_FIELDS } from './runtime-settings.ts';

const retrySource = readFileSync(new URL('./Retry.tsx', import.meta.url), 'utf8');

test('reasoning strategy defaults to immediate in initialization and selection', () => {
    assert.match(retrySource, /nextValues\[SettingKey\.ReasoningBufferStrategy\] = .*\?\? 'immediate';/);
    assert.match(retrySource, /value=\{values\[SettingKey\.ReasoningBufferStrategy\] \|\| 'immediate'\}/);
});

test('retry fields expose count, route retries, total attempts, cooldown and 429 hold timings in order', () => {
    assert.deepEqual(
        RETRY_FIELDS.map((field) => field.key),
        [
            'relay_retry_count',
            'relay_route_retries',
            'relay_max_total_attempts',
            'ratelimit_cooldown',
            'rate_limit_hold_interval',
            'rate_limit_hold_max_wait',
        ]
    );
});

test('429 hold timings require at least one second', () => {
    for (const key of ['rate_limit_hold_interval', 'rate_limit_hold_max_wait']) {
        const field = RETRY_FIELDS.find((item) => item.key === key);

        assert.ok(field, `${key} should be exposed as a retry field`);
        assert.equal(field.min, '1');
        assert.ok(field.hintKey);
    }
});

test('retry count allows zero to disable per-channel key retries', () => {
    const retryCount = RETRY_FIELDS.find((field) => field.key === 'relay_retry_count');

    assert.ok(retryCount);
    assert.equal(retryCount.min, '0');
});

test('route retries requires at least one route round', () => {
    const routeRetries = RETRY_FIELDS.find((field) => field.key === 'relay_route_retries');

    assert.ok(routeRetries);
    assert.equal(routeRetries.min, '1');
    assert.ok(routeRetries.hintKey);
});

test('auto strategy fields expose latency weight with bounded range', () => {
    const latencyWeight = AUTO_STRATEGY_FIELDS.find((field) => field.key === 'auto_strategy_latency_weight');

    assert.ok(latencyWeight);
    assert.equal(latencyWeight.min, '0');
    assert.equal(latencyWeight.max, '100');
});

test('auto strategy exposes ttft, price, explore rate and bucket tolerance with 0-100 range', () => {
    const expectedKeys = [
        'auto_strategy_ttft_weight',
        'auto_strategy_price_weight',
        'auto_strategy_explore_rate',
        'auto_strategy_bucket_tolerance',
    ];

    for (const key of expectedKeys) {
        const field = AUTO_STRATEGY_FIELDS.find((item) => item.key === key);

        assert.ok(field, `${key} should be exposed as an auto strategy field`);
        assert.equal(field.min, '0', `${key} min should be 0`);
        assert.equal(field.max, '100', `${key} max should be 100`);
        assert.ok(field.hintKey, `${key} should carry a hint key`);
        assert.ok(field.labelKey, `${key} should carry a label key`);
        assert.ok(field.placeholderKey, `${key} should carry a placeholder key`);
    }
});
