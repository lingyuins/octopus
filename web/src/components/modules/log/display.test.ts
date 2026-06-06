import test from 'node:test';
import assert from 'node:assert/strict';

import type { RelayLog, RelayLogDetail } from '@/api/endpoints/log';
import { resolveLogDisplayFields } from './display.ts';

function buildLog(overrides: Partial<RelayLog> = {}): RelayLog {
    return {
        id: 1,
        time: 0,
        request_model_name: 'deepseek-v4-pro-max',
        request_api_key_name: '',
        endpoint_type: '',
        channel: 0,
        channel_name: '',
        actual_model_name: '',
        input_tokens: 0,
        output_tokens: 0,
        ftut: 0,
        use_time: 0,
        cost: 0,
        error: '',
        attempts: [],
        total_attempts: 0,
        ...overrides,
    };
}

test('resolveLogDisplayFields infers deepseek endpoint and uses attempt channel fallback', () => {
    const log = buildLog({
        attempts: [
            {
                channel_id: 12,
                channel_name: 'DeepSeek Channel',
                model_name: 'deepseek-v4-pro-max',
                attempt_num: 1,
                status: 'success',
                duration: 10,
            },
        ],
    });

    const result = resolveLogDisplayFields(log);
    assert.equal(result.endpointType, 'deepseek');
    assert.equal(result.channelName, 'DeepSeek Channel');
    assert.equal(result.actualModelName, 'deepseek-v4-pro-max');
});

test('resolveLogDisplayFields prefers detail payload over list payload', () => {
    const log = buildLog({
        endpoint_type: '',
        channel_name: '',
        actual_model_name: '',
    });
    const detail: RelayLogDetail = {
        ...log,
        endpoint_type: 'deepseek',
        channel_name: 'Relay Channel',
        actual_model_name: 'deepseek-v4-pro-max',
        request_content: '{}',
        response_content: '{}',
    };

    const result = resolveLogDisplayFields(log, detail);
    assert.equal(result.endpointType, 'deepseek');
    assert.equal(result.channelName, 'Relay Channel');
    assert.equal(result.actualModelName, 'deepseek-v4-pro-max');
});

test('resolveLogDisplayFields falls back to channel id mapping when channel names are empty', () => {
    const log = buildLog({
        channel: 42,
        channel_name: '',
        attempts: [
            {
                channel_id: 42,
                channel_name: '',
                model_name: 'deepseek-v4-pro-max',
                attempt_num: 1,
                status: 'success',
                duration: 10,
            },
        ],
    });

    const result = resolveLogDisplayFields(log, null, new Map([[42, 'DeepSeek Fallback Channel']]));
    assert.equal(result.channelId, 42);
    assert.equal(result.channelName, 'DeepSeek Fallback Channel');
});

test('resolveLogDisplayFields falls back to channel id label when no channel name sources exist', () => {
    const log = buildLog({
        channel: 77,
        channel_name: '',
        attempts: [
            {
                channel_id: 77,
                channel_name: '',
                model_name: 'gpt-4o-mini',
                attempt_num: 1,
                status: 'success',
                duration: 10,
            },
        ],
    });

    const result = resolveLogDisplayFields(log);
    assert.equal(result.channelId, 77);
    assert.equal(result.channelName, 'channel_fallback');
});

test('resolveLogDisplayFields falls back to chat when only generic chat models exist', () => {
    const log = buildLog({
        request_model_name: 'gpt-4o-mini',
        actual_model_name: 'gpt-4o-mini',
    });

    const result = resolveLogDisplayFields(log);
    assert.equal(result.endpointType, 'chat');
});

test('resolveLogDisplayFields exposes cache read tokens from detail or list payload', () => {
    const log = buildLog({
        cache_read_tokens: 120,
    });

    const fromList = resolveLogDisplayFields(log);
    assert.equal(fromList.cacheReadTokens, 120);

    const detail: RelayLogDetail = {
        ...log,
        cache_read_tokens: 240,
        request_content: '{}',
        response_content: '{}',
    };
    const fromDetail = resolveLogDisplayFields(log, detail);
    assert.equal(fromDetail.cacheReadTokens, 240);
});

test('resolveLogDisplayFields exposes semantic cache hit flag from detail or list payload', () => {
    const log = buildLog({
        semantic_cache_hit: true,
    });

    const fromList = resolveLogDisplayFields(log);
    assert.equal(fromList.semanticCacheHit, true);

    const detail: RelayLogDetail = {
        ...log,
        semantic_cache_hit: false,
        request_content: '{}',
        response_content: '{}',
    };
    const fromDetail = resolveLogDisplayFields(log, detail);
    assert.equal(fromDetail.semanticCacheHit, false);
});

test('resolveLogDisplayFields infers MiMo Chat request type label', () => {
    const log = buildLog({
        request_model_name: 'mimo-v2.5-pro',
        actual_model_name: 'mimo-v2.5-pro',
    });

    const result = resolveLogDisplayFields(log);
    assert.equal(result.requestTypeKey, 'mimoChat');
});

test('resolveLogDisplayFields infers streaming chat request type label from request content', () => {
    const log = buildLog({
        request_model_name: 'gpt-4o-mini',
        actual_model_name: 'gpt-4o-mini',
    });
    const detail: RelayLogDetail = {
        ...log,
        request_content: '{"stream":true}',
        response_content: '{}',
    };

    const result = resolveLogDisplayFields(log, detail);
    assert.equal(result.requestTypeKey, 'streamingChat');
});

test('resolveLogDisplayFields infers embedding request type label', () => {
    const log = buildLog({
        endpoint_type: 'embeddings',
        request_model_name: 'text-embedding-3-small',
        actual_model_name: 'text-embedding-3-small',
    });

    const result = resolveLogDisplayFields(log);
    assert.equal(result.requestTypeKey, 'embedding');
});



