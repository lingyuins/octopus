import assert from 'node:assert/strict';
import test from 'node:test';

import { getChannelMetricDisplayParts } from './metric-format.ts';

test('getChannelMetricDisplayParts keeps abbreviated request units visible', () => {
    const formatted = getChannelMetricDisplayParts({
        raw: 7040,
        formatted: { value: '7.04', unit: 'K' },
    });

    assert.equal(formatted.value, '7.04');
    assert.equal(formatted.unit, 'K');
    assert.equal(formatted.text, '7.04 K');
});

test('getChannelMetricDisplayParts keeps plain counts compact when no unit exists', () => {
    const formatted = getChannelMetricDisplayParts({
        raw: 295,
        formatted: { value: '295.00', unit: '' },
    });

    assert.equal(formatted.value, '295.00');
    assert.equal(formatted.unit, '');
    assert.equal(formatted.text, '295.00');
});
