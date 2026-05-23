import test from 'node:test';
import assert from 'node:assert/strict';

import {
    buildCCSwitchProviderLink,
    maskCCSwitchSecret,
    normalizeCCSwitchEndpoint,
} from './ccswitch.ts';

test('normalizeCCSwitchEndpoint appends /v1 once', () => {
    assert.equal(normalizeCCSwitchEndpoint('https://api.example.com'), 'https://api.example.com/v1');
    assert.equal(normalizeCCSwitchEndpoint('https://api.example.com/v1/'), 'https://api.example.com/v1');
});

test('buildCCSwitchProviderLink follows v1 provider import protocol', () => {
    const link = buildCCSwitchProviderLink({
        app: 'codex',
        endpoint: 'https://api.example.com/v1',
        apiKey: 'sk-octopus-test',
        name: 'claude-sonnet',
        model: 'claude-sonnet',
        notes: 'Octopus route group',
    });

    assert.ok(link.startsWith('ccswitch://v1/import?'));
    const parsed = new URL(link);
    assert.equal(parsed.searchParams.get('resource'), 'provider');
    assert.equal(parsed.searchParams.get('app'), 'codex');
    assert.equal(parsed.searchParams.get('name'), 'claude-sonnet');
    assert.equal(parsed.searchParams.get('endpoint'), 'https://api.example.com/v1');
    assert.equal(parsed.searchParams.get('apiKey'), 'sk-octopus-test');
    assert.equal(parsed.searchParams.get('model'), 'claude-sonnet');
    assert.equal(parsed.searchParams.get('enabled'), 'true');
});

test('maskCCSwitchSecret keeps short values and masks long keys', () => {
    assert.equal(maskCCSwitchSecret('short'), 'short');
    assert.equal(maskCCSwitchSecret('sk-octopus-1234567890'), 'sk-oct...7890');
});
