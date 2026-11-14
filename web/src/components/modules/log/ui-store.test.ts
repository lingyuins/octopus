import assert from 'node:assert/strict';
import test, { after } from 'node:test';

const storageDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
const windowDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'window');
const storage = new Map<string, string>();
const localStorage = {
    getItem: (key: string) => storage.get(key) ?? null,
    setItem: (key: string, value: string) => { storage.set(key, value); },
    removeItem: (key: string) => { storage.delete(key); },
};
Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: { localStorage },
});
Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: localStorage,
});
const { DEFAULT_LOG_FIELD_VISIBILITY, normalizeLogFieldVisibility, shouldAutoRefreshLogs, useLogFieldVisibilityStore } = await import('./ui-store.ts');
after(() => {
    if (storageDescriptor) Object.defineProperty(globalThis, 'localStorage', storageDescriptor);
    else Reflect.deleteProperty(globalThis, 'localStorage');
    if (windowDescriptor) Object.defineProperty(globalThis, 'window', windowDescriptor);
    else Reflect.deleteProperty(globalThis, 'window');
});

test('log visibility restores missing defaults while preserving explicit false', () => {
    assert.deepEqual(normalizeLogFieldVisibility({ cost: false, clientIP: true }), {
        ...DEFAULT_LOG_FIELD_VISIBILITY,
        cost: false,
    });
});

test('log visibility discards invalid values and unknown fields', () => {
    assert.deepEqual(normalizeLogFieldVisibility({ cost: 'false', tps: null, removedField: false }), DEFAULT_LOG_FIELD_VISIBILITY);
    for (const value of [undefined, null, [], 'invalid', 0]) {
        assert.deepEqual(normalizeLogFieldVisibility(value), DEFAULT_LOG_FIELD_VISIBILITY);
    }
});

test('log persistence merge fills missing keys without replacing store actions', () => {
    const current = useLogFieldVisibilityStore.getState();
    const merge = useLogFieldVisibilityStore.persist.getOptions().merge!;
    const merged = merge({ visibility: { cost: false } }, current);
    assert.deepEqual(merged.visibility, { ...DEFAULT_LOG_FIELD_VISIBILITY, cost: false });
    assert.equal(merged.toggleField, current.toggleField);
    assert.equal(merged.resetFields, current.resetFields);
});

test('automatic log refresh only runs for the visible relay log view', () => {
    assert.equal(shouldAutoRefreshLogs(5, 'log', 'relay', 'visible'), true);
    assert.equal(shouldAutoRefreshLogs(0, 'log', 'relay', 'visible'), false);
    assert.equal(shouldAutoRefreshLogs(5, 'home', 'relay', 'visible'), false);
    assert.equal(shouldAutoRefreshLogs(5, 'log', 'error', 'visible'), false);
    assert.equal(shouldAutoRefreshLogs(5, 'log', 'relay', 'hidden'), false);
});
