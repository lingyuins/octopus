import assert from 'node:assert/strict';
import test from 'node:test';
import { formatAverageLatency, type LatencyUnitMode } from './latency-format.ts';

test('formatAverageLatency returns dash when there are no completed requests', () => {
    assert.equal(formatAverageLatency(250, 0, 'auto'), '—');
});

test('formatAverageLatency keeps millisecond precision for explicit ms mode', () => {
    assert.equal(formatAverageLatency(1534, 3, 'ms'), '1534ms');
});

test('formatAverageLatency converts to seconds and trims insignificant zeros', () => {
    assert.equal(formatAverageLatency(1534, 3, 's'), '1.53s');
    assert.equal(formatAverageLatency(2000, 3, 's'), '2s');
});

test('formatAverageLatency converts to hours for explicit h mode', () => {
    assert.equal(formatAverageLatency(7_200_000, 3, 'h'), '2h');
    assert.equal(formatAverageLatency(5_400_000, 3, 'h'), '1.5h');
});

test('formatAverageLatency auto-selects ms, s, and h by magnitude', () => {
    const cases: Array<[number, LatencyUnitMode, string]> = [
        [999, 'auto', '999ms'],
        [1500, 'auto', '1.5s'],
        [3_600_000, 'auto', '1h'],
    ];

    for (const [latencyMs, mode, expected] of cases) {
        assert.equal(formatAverageLatency(latencyMs, 1, mode), expected);
    }
});
