import test from 'node:test';
import assert from 'node:assert/strict';
import { formatUnixSeconds, formatUnixMillis, formatDateTime, formatDateOnly, getCurrentTimeZone } from './time.ts';

test('formatUnixSeconds returns full locale string for valid timestamp', () => {
    const result = formatUnixSeconds(1700000000);
    assert.ok(result.length > 5, `expected readable string, got '${result}'`);
});

test('formatUnixSeconds returns fallback for zero', () => {
    assert.equal(formatUnixSeconds(0), '-');
});

test('formatUnixSeconds returns fallback for negative', () => {
    assert.equal(formatUnixSeconds(-1), '-');
});

test('formatUnixMillis converts ms timestamp', () => {
    const result = formatUnixMillis(1700000000000);
    assert.ok(result.length > 5);
});

test('formatUnixMillis returns fallback for zero', () => {
    assert.equal(formatUnixMillis(0), '-');
});

test('formatDateTime returns full string for ISO date string', () => {
    const result = formatDateTime('2024-11-15T10:30:00Z');
    assert.ok(result.length > 5);
});

test('formatDateTime returns fallback for empty', () => {
    assert.equal(formatDateTime(''), '-');
});

test('formatDateTime returns fallback for invalid', () => {
    assert.equal(formatDateTime('not-a-date'), '-');
});

test('formatDateOnly returns date part', () => {
    const result = formatDateOnly('2024-11-15');
    assert.ok(result.length >= 4, `expected readable date, got '${result}'`);
});

test('getCurrentTimeZone returns a non-empty string', () => {
    const tz = getCurrentTimeZone();
    assert.ok(tz.length > 0, 'timezone should not be empty');
});
