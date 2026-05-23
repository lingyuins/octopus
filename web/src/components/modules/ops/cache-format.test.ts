import assert from 'node:assert/strict';
import test from 'node:test';

import { formatProviderPromptCacheCount } from './cache-format.ts';

test('formatProviderPromptCacheCount separates abbreviated value and unit for metric cards', () => {
    const formatted = formatProviderPromptCacheCount(235_500_000);

    assert.equal(formatted.value, '235.50');
    assert.equal(formatted.unit, 'M');
    assert.equal(formatted.text, '235.50M');
});

test('formatProviderPromptCacheCount keeps zero unitless', () => {
    const formatted = formatProviderPromptCacheCount(0);

    assert.equal(formatted.value, '0');
    assert.equal(formatted.unit, '');
    assert.equal(formatted.text, '0');
});
