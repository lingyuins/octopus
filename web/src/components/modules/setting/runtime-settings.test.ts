import assert from 'node:assert/strict';
import test from 'node:test';

import { AUTO_STRATEGY_FIELDS, RETRY_FIELDS } from './runtime-settings.ts';

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
