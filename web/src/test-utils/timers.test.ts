import test from 'node:test';
import assert from 'node:assert/strict';
import { installFakeTimers } from './timers.ts';

test('fake timers advance and expire', async () => {
    const timers = installFakeTimers();
    let fired = false;
    setTimeout(() => {
        fired = true;
    }, 2000);
    assert.equal(fired, false);
    await timers.advanceTimersByTime(1999);
    assert.equal(fired, false);
    await timers.advanceTimersByTime(1);
    assert.equal(fired, true);
    timers.uninstall();
});

test('fake timers clearTimeout cancels pending callback', async () => {
    const timers = installFakeTimers();
    let fired = false;
    const id = setTimeout(() => {
        fired = true;
    }, 1000);
    clearTimeout(id);
    await timers.advanceTimersByTime(2000);
    assert.equal(fired, false);
    timers.uninstall();
});
