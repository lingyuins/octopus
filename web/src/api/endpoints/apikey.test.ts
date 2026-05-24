import assert from 'node:assert/strict';
import test from 'node:test';

import { formatAPIKeyStatsResponse } from './apikey-format.ts';

test('formatAPIKeyStatsResponse formats nested stats response without flattening info', () => {
    const formatted = formatAPIKeyStatsResponse({
        stats: {
            api_key_id: 7,
            input_token: 1200,
            output_token: 800,
            input_cost: 0.12,
            output_cost: 0.24,
            wait_time: 1500,
            request_success: 3,
            request_failed: 1,
        },
        info: {
            id: 7,
            name: 'dashboard key',
            api_key: 'sk-octopus-********1234',
            enabled: true,
            supported_models: 'gpt-4o',
        },
    });

    assert.equal(formatted.stats.api_key_id, 7);
    assert.equal(formatted.stats.total_token.formatted.value, '2.00');
    assert.equal(formatted.stats.total_token.formatted.unit, 'K');
    assert.equal(formatted.stats.request_count.formatted.value, '4.00');
    assert.equal(formatted.stats.request_count.formatted.unit, '');
    assert.equal(formatted.info.name, 'dashboard key');
    assert.equal(formatted.info.supported_models, 'gpt-4o');
});
