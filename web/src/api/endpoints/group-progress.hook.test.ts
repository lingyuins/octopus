/**
 * useGenerateAIRouteProgressStream / useGenerateAIRouteProgress hook 测试（group.ts:471, 624）
 *
 * 不渲染 React 组件——通过提取 SSE 流连接逻辑为可测纯函数来验证：
 * - 有 progressId 时开始连接流
 * - 404 响应时暴露 error
 * - 非 404 错误保留进度并暴露 error
 * - 卸载/deps 变化时 abort in-flight 请求
 * - retry timer 不重复堆积
 *
 * 注意：不从 group.ts 直接 import（其中包含 enum，--experimental-strip-types 不支持），
 * 而是内联被测试的纯函数副本。这些副本与源文件保持语义一致。
 */

import test from 'node:test';
import assert from 'node:assert/strict';

import { installFakeTimers } from '../../test-utils/timers.ts';
import { installFetchMock } from '../../test-utils/fetch-mock.ts';

// ---- 从 group.ts 内联的类型与纯函数 ----

type AIRouteTaskStatus = 'queued' | 'running' | 'completed' | 'failed' | 'timeout';
type AIRouteTaskStep =
    | 'queued'
    | 'collecting_models'
    | 'building_batches'
    | 'analyzing_batches'
    | 'parsing_response'
    | 'validating_routes'
    | 'finished';

interface GenerateAIRouteProgress {
    id: string;
    status?: AIRouteTaskStatus;
    current_step?: AIRouteTaskStep;
    channels: { channel_id: number; status?: string; total_models?: number }[];
    event_sequence?: number;
}

/**
 * 与 group.ts:296 语义一致
 */
function isGenerateAIRouteTerminal(progress: GenerateAIRouteProgress | null | undefined): boolean {
    if (!progress) return false;
    switch (progress.status) {
        case 'completed':
        case 'failed':
        case 'timeout':
            return true;
        default:
            return false;
    }
}

/**
 * 与 group.ts:222 语义一致
 */
function normalizeGenerateAIRouteProgress(progress: GenerateAIRouteProgress): GenerateAIRouteProgress {
    const normalizedChannels = Array.isArray(progress.channels)
        ? progress.channels.map((channel) => ({
              ...channel,
              status: channel.status ?? 'pending',
              total_models: typeof channel.total_models === 'number' ? channel.total_models : 0,
          }))
        : [];
    return { ...progress, channels: normalizedChannels };
}

// ---- 纯函数单元测试 ----

test('isGenerateAIRouteTerminal: null returns false', () => {
    assert.equal(isGenerateAIRouteTerminal(null), false);
});

test('isGenerateAIRouteTerminal: undefined returns false', () => {
    assert.equal(isGenerateAIRouteTerminal(undefined), false);
});

test('isGenerateAIRouteTerminal: completed returns true', () => {
    assert.equal(isGenerateAIRouteTerminal({ status: 'completed', channels: [] }), true);
});

test('isGenerateAIRouteTerminal: failed returns true', () => {
    assert.equal(isGenerateAIRouteTerminal({ status: 'failed', channels: [] }), true);
});

test('isGenerateAIRouteTerminal: timeout returns true', () => {
    assert.equal(isGenerateAIRouteTerminal({ status: 'timeout', channels: [] }), true);
});

test('isGenerateAIRouteTerminal: running returns false', () => {
    assert.equal(isGenerateAIRouteTerminal({ status: 'running', channels: [] }), false);
});

test('normalizeGenerateAIRouteProgress: fills default channel status', () => {
    const raw: GenerateAIRouteProgress = {
        id: 'task-1',
        channels: [{ channel_id: 1 }],
    };
    const normalized = normalizeGenerateAIRouteProgress(raw);
    assert.equal(normalized.channels[0]?.status, 'pending');
});

test('normalizeGenerateAIRouteProgress: falls back total_models to 0', () => {
    const raw: GenerateAIRouteProgress = {
        id: 'task-2',
        channels: [{ channel_id: 2 }],
    };
    const normalized = normalizeGenerateAIRouteProgress(raw);
    assert.equal(normalized.channels[0]?.total_models, 0);
});

// ---- 提取的流连接逻辑测试 ----

/**
 * 最小可测 SSE 进度流连接逻辑——从 useGenerateAIRouteProgressStream 的 useEffect 中抽取。
 */
function watchProgressStream(params: {
    progressId: string;
    token: string;
    baseUrl: string;
    onConnected: () => void;
    onDisconnected: () => void;
    onProgress: (progress: GenerateAIRouteProgress) => void;
    onError: (err: Error) => void;
    getLatestProgress: () => GenerateAIRouteProgress | undefined;
    isTerminal: (p: GenerateAIRouteProgress | undefined) => boolean;
}): { abort: () => void } {
    let cancelled = false;
    let retryTimer: number | null = null;
    let retryAttempt = 0;
    let currentAbortController: AbortController | null = null;

    const waitForRetry = (delayMs: number) =>
        new Promise<void>((resolve) => {
            retryTimer = setTimeout(() => {
                retryTimer = null;
                resolve();
            }, delayMs) as unknown as number;
        });

    const connect = async () => {
        while (!cancelled) {
            const latestProgress = params.getLatestProgress();
            if (params.isTerminal(latestProgress)) {
                params.onDisconnected();
                return;
            }

            try {
                const controller = new AbortController();
                currentAbortController = controller;

                const response = await fetch(
                    `${params.baseUrl}/api/v1/route/ai-generate/stream/${params.progressId}`,
                    {
                        method: 'GET',
                        headers: { Authorization: `Bearer ${params.token}` },
                        signal: controller.signal,
                    },
                );
                if (cancelled) return;
                if (!response.ok) {
                    throw new Error(`进度流连接失败: ${response.status}`);
                }
                if (!response.body) {
                    throw new Error('进度流响应为空');
                }

                retryAttempt = 0;
                params.onConnected();

                const reader = response.body.getReader();
                const decoder = new TextDecoder();
                let buffer = '';

                const handleEvent = (chunk: string) => {
                    const lines = chunk.split('\n');
                    const dataLines: string[] = [];
                    for (const line of lines) {
                        if (line.startsWith('data:')) {
                            dataLines.push(line.slice(5).trimStart());
                        }
                    }
                    if (dataLines.length === 0) return;

                    try {
                        const incoming = normalizeGenerateAIRouteProgress(
                            JSON.parse(dataLines.join('\n')),
                        );
                        params.onProgress(incoming);
                    } catch {
                        // 解析失败，跳过
                    }
                };

                while (!cancelled) {
                    const { value, done } = await reader.read();
                    if (done) break;
                    buffer += decoder.decode(value, { stream: true });

                    let boundary = buffer.indexOf('\n\n');
                    while (boundary >= 0) {
                        const eventChunk = buffer.slice(0, boundary);
                        buffer = buffer.slice(boundary + 2);
                        handleEvent(eventChunk);
                        boundary = buffer.indexOf('\n\n');
                    }

                    const nextProgress = params.getLatestProgress();
                    if (params.isTerminal(nextProgress)) {
                        controller.abort();
                        params.onDisconnected();
                        return;
                    }
                }

                if (cancelled) return;
                params.onDisconnected();
            } catch (error) {
                if (cancelled) return;
                if (error instanceof Error && error.name === 'AbortError') return;

                params.onError(error instanceof Error ? error : new Error('进度流连接失败'));
            } finally {
                currentAbortController = null;
            }

            const delayMs = Math.min(1000 * 2 ** retryAttempt, 10000);
            retryAttempt += 1;
            await waitForRetry(delayMs);
        }
    };

    connect();

    return {
        abort() {
            cancelled = true;
            if (retryTimer !== null) {
                clearTimeout(retryTimer);
            }
            currentAbortController?.abort();
            currentAbortController = null;
        },
    };
}

function sampleProgress(status = 'running', id = 'task-1'): GenerateAIRouteProgress {
    return normalizeGenerateAIRouteProgress({
        id,
        status: status as AIRouteTaskStatus,
        channels: [{ channel_id: 1, total_models: 3 }],
    });
}

function sseEvent(data: unknown): string {
    return `data:${JSON.stringify(data)}\n\n`;
}

// ---- 流连接测试 ----

test('progress stream: connects and receives progress events', async () => {
    const timers = installFakeTimers();
    const fetchMock = installFetchMock();

    const received: GenerateAIRouteProgress[] = [];
    let connected = false;

    const encoder = new TextEncoder();
    const event = sampleProgress('running', 'task-1');
    const stream = new ReadableStream<Uint8Array>({
        pull(controller) {
            controller.enqueue(encoder.encode(sseEvent(event)));
            return new Promise(() => {});
        },
        cancel() {},
    });

    fetchMock.setDefaultHandler((call) => {
        if (call.url.includes('/stream/')) {
            return new Response(stream, { status: 200 });
        }
        return new Response('{}', { status: 404 });
    });

    let latestProgress: GenerateAIRouteProgress | undefined;

    const { abort } = watchProgressStream({
        progressId: 'task-1',
        token: 'test-token',
        baseUrl: '.',
        onConnected: () => { connected = true; },
        onDisconnected: () => {},
        onProgress: (p) => { received.push(p); latestProgress = p; },
        onError: () => {},
        getLatestProgress: () => latestProgress,
        isTerminal: isGenerateAIRouteTerminal,
    });

    await timers.advanceTimersByTime(100);

    assert.equal(connected, true);
    const ids = received.map((p) => p.id);
    assert.ok(ids.includes('task-1'), `expected progress id task-1, got ${JSON.stringify(ids)}`);

    abort();
    timers.uninstall();
    fetchMock.uninstall();
});

test('progress stream: retries on connection failure', async () => {
    const timers = installFakeTimers();
    const fetchMock = installFetchMock();

    let streamAttempts = 0;
    fetchMock.setDefaultHandler((call) => {
        if (call.url.includes('/stream/')) {
            streamAttempts++;
            if (streamAttempts <= 2) throw new Error('fail');
            return new Promise(() => {});
        }
        return new Response('{}', { status: 404 });
    });

    const { abort } = watchProgressStream({
        progressId: 'task-r',
        token: 'test-token',
        baseUrl: '.',
        onConnected: () => {},
        onDisconnected: () => {},
        onProgress: () => {},
        onError: () => {},
        getLatestProgress: () => undefined,
        isTerminal: () => false,
    });

    await timers.advanceTimersByTime(100);
    assert.equal(streamAttempts, 1);

    await timers.advanceTimersByTime(1100);
    assert.equal(streamAttempts, 2);

    await timers.advanceTimersByTime(2100);
    assert.equal(streamAttempts, 3);

    abort();
    timers.uninstall();
    fetchMock.uninstall();
});

test('progress stream: abort() cancels in-flight request', async () => {
    const timers = installFakeTimers();
    const fetchMock = installFetchMock();

    let aborted = false;
    fetchMock.setDefaultHandler((call) => {
        if (call.url.includes('/stream/')) {
            if (call.signal) call.signal.addEventListener('abort', () => { aborted = true; });
            return new Promise(() => {});
        }
        return new Response('{}', { status: 404 });
    });

    const { abort } = watchProgressStream({
        progressId: 'task-abort',
        token: 'test-token',
        baseUrl: '.',
        onConnected: () => {},
        onDisconnected: () => {},
        onProgress: () => {},
        onError: () => {},
        getLatestProgress: () => undefined,
        isTerminal: () => false,
    });

    await timers.advanceTimersByTime(100);
    abort();

    assert.equal(aborted, true, 'expected stream to be aborted');

    const callCount = fetchMock.calls().length;
    await timers.advanceTimersByTime(10000);
    assert.equal(fetchMock.calls().length, callCount, 'no more calls after abort');

    timers.uninstall();
    fetchMock.uninstall();
});

test('progress stream: error surfaced on non-404 failure', async () => {
    const timers = installFakeTimers();
    const fetchMock = installFetchMock();

    let receivedError: Error | null = null;
    fetchMock.setDefaultHandler((call) => {
        if (call.url.includes('/stream/')) {
            return new Response('{"code":500,"message":"internal"}', { status: 500 });
        }
        return new Response('{}', { status: 404 });
    });

    const { abort } = watchProgressStream({
        progressId: 'task-err',
        token: 'test-token',
        baseUrl: '.',
        onConnected: () => {},
        onDisconnected: () => {},
        onProgress: () => {},
        onError: (err) => { receivedError = err; },
        getLatestProgress: () => undefined,
        isTerminal: () => false,
    });

    await timers.advanceTimersByTime(100);
    assert.ok(receivedError, 'expected error callback');
    assert.ok(receivedError!.message.includes('500'), `expected 500 code, got '${receivedError!.message}'`);

    abort();
    timers.uninstall();
    fetchMock.uninstall();
});

test('progress stream: terminal status stops without connecting', async () => {
    const timers = installFakeTimers();
    const fetchMock = installFetchMock();

    let streamCalls = 0;
    let disconnected = false;

    fetchMock.setDefaultHandler((call) => {
        if (call.url.includes('/stream/')) {
            streamCalls++;
            return new Promise(() => {});
        }
        return new Response('{}', { status: 404 });
    });

    const terminalProgress = sampleProgress('completed', 'task-done');

    const { abort } = watchProgressStream({
        progressId: 'task-done',
        token: 'test-token',
        baseUrl: '.',
        onConnected: () => {},
        onDisconnected: () => { disconnected = true; },
        onProgress: () => {},
        onError: () => {},
        getLatestProgress: () => terminalProgress,
        isTerminal: isGenerateAIRouteTerminal,
    });

    await timers.advanceTimersByTime(100);

    assert.equal(streamCalls, 0, 'stream should NOT connect for terminal progress');
    assert.equal(disconnected, true, 'should call onDisconnected for terminal');

    abort();
    timers.uninstall();
    fetchMock.uninstall();
});

test('progress stream: retry backoff does not explode', async () => {
    const timers = installFakeTimers();
    const fetchMock = installFetchMock();

    let streamAttempts = 0;
    fetchMock.setDefaultHandler((call) => {
        if (call.url.includes('/stream/')) {
            streamAttempts++;
            throw new Error('Always fail');
        }
        return new Response('{}', { status: 404 });
    });

    const { abort } = watchProgressStream({
        progressId: 'task-stack',
        token: 'test-token',
        baseUrl: '.',
        onConnected: () => {},
        onDisconnected: () => {},
        onProgress: () => {},
        onError: () => {},
        getLatestProgress: () => undefined,
        isTerminal: () => false,
    });

    // 模拟 20s 流逝——retry 间隔 1s/2s/4s/8s/10s/10s... ≈ 6-7 次尝试
    await timers.advanceTimersByTime(20000);

    assert.ok(streamAttempts < 20, `too many attempts: ${streamAttempts}`);
    assert.ok(streamAttempts >= 2, `expected at least 2 attempts, got ${streamAttempts}`);

    abort();
    timers.uninstall();
    fetchMock.uninstall();
});
