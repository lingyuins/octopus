import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { buildNodeCloudURL, buildNodeConfig } from './node-config.ts';

test('buildNodeCloudURL derives a wss connect URL preserving any deployment prefix', () => {
    assert.equal(buildNodeCloudURL('https://cloud.example.com/octopus/'), 'wss://cloud.example.com/octopus/node/v1/connect');
    assert.equal(buildNodeCloudURL('http://cloud.example.com'), 'wss://cloud.example.com/node/v1/connect');
    assert.equal(buildNodeCloudURL('https://cloud.example.com/path?x=1#y'), 'wss://cloud.example.com/path/node/v1/connect');
    assert.equal(buildNodeCloudURL('http://localhost:8080/octopus/'), 'ws://127.0.0.1:8080/octopus/node/v1/connect');
    assert.equal(buildNodeCloudURL('http://127.0.0.1:8080/octopus/'), 'ws://127.0.0.1:8080/octopus/node/v1/connect');
});

test('plugin setup separates public options from private credentials and preserves deployment prefix', () => {
    const config = JSON.parse(buildNodeConfig('once-only-token', 'https://cloud.example.com/octopus/'));
    assert.deepEqual(config, {
        plugin_options: {
            version: 1,
            connect: false,
            cloud_url: 'wss://cloud.example.com/octopus/node/v1/connect',
            models: ['<MODEL_ID>'],
            max_concurrency: 4,
            request_timeout_seconds: 300,
        },
        private_config: {
            node_token: 'once-only-token',
            magpie_url: 'http://127.0.0.1:3425',
            magpie_key: '<LOCAL_MAGPIE_GATEWAY_KEY>',
        },
    });
    assert.equal(JSON.parse(buildNodeConfig('token', 'http://cloud.example.com')).plugin_options.cloud_url, 'wss://cloud.example.com/node/v1/connect');
    assert.equal(JSON.stringify(config.plugin_options).includes('once-only-token'), false);
    assert.equal('node_token' in config.plugin_options, false);

    const local = JSON.parse(buildNodeConfig('local-token', 'http://localhost:8080/octopus/'));
    assert.equal(local.plugin_options.cloud_url, 'ws://127.0.0.1:8080/octopus/node/v1/connect');
    assert.equal(local.private_config.allow_insecure_loopback, true);
});

test('node confirmations remain in the settings panel without nested portal or form submission', () => {
    const ui = readFileSync(new URL('./Nodes.tsx', import.meta.url), 'utf8');
    assert.doesNotMatch(ui, /AlertDialog|Portal|aria-modal/);
    const confirmation = ui.slice(ui.indexOf('{confirmation && <div'));
    assert.match(confirmation, /role="alertdialog"/);
    assert.match(confirmation, /aria-labelledby=\{`\$\{confirmationId\}-title`\}/);
    assert.match(confirmation, /aria-describedby=\{`\$\{confirmationId\}-description`\}/);
    assert.ok(ui.indexOf('</form>') < ui.indexOf('{confirmation && <div'));
    assert.match(confirmation, /<Button type="button" variant="outline" onClick=\{\(event\) => \{ event.stopPropagation\(\); setConfirmation\(null\); \}\}/);
    assert.match(confirmation, /<Button type="button" disabled=\{action.isPending \|\| !canWrite\} onClick=\{\(event\) => \{ event.stopPropagation\(\); run\(confirmation\); setConfirmation\(null\); \}\}/);
    assert.match(confirmation, /event.key === 'Escape'[\s\S]*?event.stopPropagation\(\);[\s\S]*?setConfirmation\(null\)/);
    assert.match(ui, /confirmationRef.current\?\.focus\(\)/);
    assert.equal((ui.match(/disabled=\{action.isPending \|\| confirmation !== null\}/g) ?? []).length, 3);
    assert.match(ui, /onClick=\{\(\) => runTest\(node.id\)\}/);
    assert.match(ui, /<NodeTestResult result=\{testResults\[node.id\]\}/);
});

test('one-time tokens live only in component state and never mutation/query data or persistence', () => {
    const api = readFileSync(new URL('../../../api/endpoints/node.ts', import.meta.url), 'utf8');
    const ui = readFileSync(new URL('./Nodes.tsx', import.meta.url), 'utf8');
    assert.match(api, /gcTime: 0/);
    assert.match(api, /mutationFn: async \(input: NodeAction\): Promise<void>/);
    assert.doesNotMatch(api, /return result|setQueryData|logger\.|console\./);
    assert.match(api, /onToken\(result.node, result.token\)/);
    assert.doesNotMatch(ui + api, /localStorage|sessionStorage|persist\(/);
    assert.match(ui, /if \(!active \|\| !authenticated \|\| !canWrite\) \{ setSecret\(null\); setTestResults\(\{\}\); \}/);
    assert.match(ui, /canWrite = data\?\.can_write === true/);
    assert.match(ui, /if \(!canWrite\) return/);
    assert.match(ui, /\{confirmation && <div[\s\S]*?role="alertdialog"/);
    assert.match(ui, /disabled=\{action.isPending \|\| !canWrite\}/);
    assert.match(ui, /formatDateTime\(node.last_seen_at\)/);
    assert.doesNotMatch(ui, /type="password"|magpie_key.*onChange/);
});
