import assert from 'node:assert/strict';
import test from 'node:test';
import { formatTelemetryPercent, getTelemetryErrorRateTone } from './telemetry-format.ts';

test('formatTelemetryPercent keeps backend percentage values without scaling again', () => {
    assert.equal(formatTelemetryPercent(9.5238), '9.5%');
    assert.equal(formatTelemetryPercent(75), '75.0%');
});

test('getTelemetryErrorRateTone uses percentage thresholds', () => {
    assert.equal(getTelemetryErrorRateTone(4.9), 'success');
    assert.equal(getTelemetryErrorRateTone(5.1), 'warning');
    assert.equal(getTelemetryErrorRateTone(10.1), 'danger');
});
