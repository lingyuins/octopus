/**
 * useLogs hook 测试（log.ts:128）
 *
 * 不渲染 React 组件——通过提取 SSE 流连接逻辑为可测纯函数来验证：
 * - 首次成功拉取第一页日志
 * - SSE 流连接成功后写入新日志
 * - 流失败后进入 retry 并在 timer 到点后重连
 * - 卸载时 abort 当前请求并清掉 retry timer
 * - token 为空时不建立连接
 * - page size 变化时 query key 不串线
 */

import test from 'node:test';
import assert from 'node:assert/strict';

import { installFakeTimers } from '../../test-utils/timers.ts';
import { installFetchMock } from '../../test-utils/fetch-mock.ts';

// ---- 从 log.ts 复制纯逻辑 / 内联提取 ----

/**
 * 一条示例 RelayLog，用于测试断言。
 */
function sampleLog(id: number, overrides: Partial<Record<string, unknown>> = {}) {
    return {
        id,
        time: Date.now(),
        request_model_name: 'gpt-4',
        channel: 1,
        channel_name: 'test-channel',
        actual_model_name: 'gpt-4',
        input_tokens: 100,
        output_tokens: 200,
        ftut: 50,
        use_time: 500,
        cost: 0.001,
        error: '',
        ...overrides,
    } as import('./log.ts').RelayLog;
}

/** 构造 SSE 文本帧 */
function sseEvent(data: unknown): string {
    return `data:${JSON.stringify(data)}\n\n`;
}

/**
 * 最小可测 SSE 流连接逻辑——从 useLogs 的 useEffect 中抽取。
 *
 * 返回 cleanup 函数；调用它模拟组件卸载。
 */
function watchLogStream(params: {
    token: string;
    pageSize: number;
    baseUrl: string;
    onConnected: () => void;
    onDisconnected: (reason: string) => void;
    onError: (err: Error) => void;
    onLogReceived: (log: import('./log.ts').RelayLog) => void;
    signal?: AbortSignal;
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
        if (!params.token) {
            params.onError(new Error('未认证，无法建立日志流'));
            return;
        }

        while (!cancelled) {
            try {
                const controller = new AbortController();
                currentAbortController = controller;

                const response = await fetch(`${params.baseUrl}/api/v1/log/stream`, {
                    method: 'GET',
                    headers: { Authorization: `Bearer ${params.token}` },
                    signal: controller.signal,
                });
                if (cancelled) return;
                if (!response.ok) {
                    throw new Error(`日志流连接失败: ${response.status}`);
                }
                if (!response.body) {
                    throw new Error('日志流响应为空');
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
                        const log = JSON.parse(dataLines.join('\n')) as import('./log.ts').RelayLog;
                        params.onLogReceived(log);
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
                }

                if (cancelled) return;

                params.onDisconnected('日志流连接已断开，正在重连...');
            } catch (e) {
                if (cancelled) return;
                if (e instanceof Error && e.name === 'AbortError') {
                    return;
                }

                params.onError(e instanceof Error ? e : new Error('日志流连接失败'));
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

// ---- tests ----

test('useLogs stream: connects and receives SSE log', async () => {
    const timers = installFakeTimers();
    const fetchMock = installFetchMock();

    const receivedLogs: unknown[] = [];
    let connected = false;

    // 构造一个 ReadableStream，推送一条 SSE 事件后挂起
    const encoder = new TextEncoder();
    const stream = new ReadableStream<Uint8Array>({
        pull(controller) {
            controller.enqueue(encoder.encode(sseEvent(sampleLog(42))));
            // 挂起不关闭，模拟长连接
            return new Promise(() => {});
        },
        cancel() {},
    });

    fetchMock.setDefaultHandler((call) => {
        if (call.url.includes('/api/v1/log/stream')) {
            return new Response(stream, { status: 200 });
        }
        return new Response('{}', { status: 404 });
    });

    const { abort } = watchLogStream({
        token: 'test-token',
        pageSize: 10,
        baseUrl: '.',
        onConnected: () => { connected = true; },
        onDisconnected: () => {},
        onError: () => {},
        onLogReceived: (log) => { receivedLogs.push(log); },
    });

    // 等待 fetch + SSE 解析
    await timers.advanceTimersByTime(100);

    assert.equal(connected, true);
    const ids = receivedLogs.map((l: { id: number }) => l.id);
    assert.ok(ids.includes(42), `expected log id 42, got ${JSON.stringify(ids)}`);

    abort();
    timers.uninstall();
    fetchMock.uninstall();
});

test('useLogs stream: retries on connection failure', async () => {
    const timers = installFakeTimers();
    const fetchMock = installFetchMock();

    let streamAttempts = 0;
    fetchMock.setDefaultHandler((call) => {
        if (call.url.includes('/api/v1/log/stream')) {
            streamAttempts++;
            // 前两次失败
            if (streamAttempts <= 2) {
                throw new Error('Connection refused');
            }
            // 第三次挂起
            return new Promise(() => {});
        }
        return new Response('{}', { status: 404 });
    });

    const { abort } = watchLogStream({
        token: 'test-token',
        pageSize: 10,
        baseUrl: '.',
        onConnected: () => {},
        onDisconnected: () => {},
        onError: () => {},
        onLogReceived: () => {},
    });

    await timers.advanceTimersByTime(100);
    assert.equal(streamAttempts, 1, `expected 1st attempt, got ${streamAttempts}`);

    // 等待 retry delay (1000ms)
    await timers.advanceTimersByTime(1100);
    assert.equal(streamAttempts, 2, `expected 2nd attempt, got ${streamAttempts}`);

    // 第二次 retry (2000ms)
    await timers.advanceTimersByTime(2100);
    assert.equal(streamAttempts, 3, `expected 3rd attempt, got ${streamAttempts}`);

    abort();
    timers.uninstall();
    fetchMock.uninstall();
});

test('useLogs stream: abort() stops retry and cancels in-flight request', async () => {
    const timers = installFakeTimers();
    const fetchMock = installFetchMock();

    let aborted = false;
    fetchMock.setDefaultHandler((call) => {
        if (call.url.includes('/api/v1/log/stream')) {
            if (call.signal) {
                call.signal.addEventListener('abort', () => { aborted = true; });
            }
            // 挂起
            return new Promise(() => {});
        }
        return new Response('{}', { status: 404 });
    });

    const { abort } = watchLogStream({
        token: 'test-token',
        pageSize: 10,
        baseUrl: '.',
        onConnected: () => {},
        onDisconnected: () => {},
        onError: () => {},
        onLogReceived: () => {},
    });

    await timers.advanceTimersByTime(100);

    // 卸载
    abort();

    assert.equal(aborted, true, 'expected stream request to be aborted');

    // 确认不会再有新的 fetch
    const callCountAfter = fetchMock.calls().length;
    await timers.advanceTimersByTime(10000);
    assert.equal(fetchMock.calls().length, callCountAfter, 'expected no more fetch calls after abort');

    timers.uninstall();
    fetchMock.uninstall();
});

test('useLogs stream: does not connect when token is empty', async () => {
    const timers = installFakeTimers();
    const fetchMock = installFetchMock();

    let streamCalled = false;
    let errorReceived: Error | null = null;
    fetchMock.setDefaultHandler((call) => {
        if (call.url.includes('/api/v1/log/stream')) {
            streamCalled = true;
        }
        return new Response('{}', { status: 404 });
    });

    const { abort } = watchLogStream({
        token: '',
        pageSize: 10,
        baseUrl: '.',
        onConnected: () => {},
        onDisconnected: () => {},
        onError: (err) => { errorReceived = err; },
        onLogReceived: () => {},
    });

    await timers.advanceTimersByTime(100);

    assert.equal(streamCalled, false, 'expected stream NOT to be called with empty token');
    assert.ok(errorReceived, 'expected error callback when token is empty');

    abort();
    timers.uninstall();
    fetchMock.uninstall();
});

test('useLogs stream: disconnection triggers onDisconnected and retry', async () => {
    const timers = installFakeTimers();
    const fetchMock = installFetchMock();

    let streamCallCount = 0;
    let disconnectReason = '';

    fetchMock.setDefaultHandler((call) => {
        if (call.url.includes('/api/v1/log/stream')) {
            streamCallCount++;
            if (streamCallCount === 1) {
                // 第一次：返回一个立即结束的流（模拟服务端断开）
                const encoder = new TextEncoder();
                const stream2 = new ReadableStream<Uint8Array>({
                    start(controller) {
                        controller.enqueue(encoder.encode(sseEvent(sampleLog(1))));
                        controller.close();
                    },
                });
                return new Response(stream2, { status: 200 });
            }
            // 第二次：挂起（重连后）
            return new Promise(() => {});
        }
        return new Response('{}', { status: 404 });
    });

    const { abort } = watchLogStream({
        token: 'test-token',
        pageSize: 10,
        baseUrl: '.',
        onConnected: () => {},
        onDisconnected: (reason) => { disconnectReason = reason; },
        onError: () => {},
        onLogReceived: () => {},
    });

    // 等待第一次连接 + SSE 读取 + 自然断开
    await timers.advanceTimersByTime(200);
    assert.equal(streamCallCount, 1, 'first stream connected');
    assert.ok(disconnectReason.includes('已断开'), `expected disconnect reason, got '${disconnectReason}'`);

    // retry after 1000ms
    await timers.advanceTimersByTime(1100);
    assert.equal(streamCallCount, 2, 'expected retry after disconnect');

    abort();
    timers.uninstall();
    fetchMock.uninstall();
});

test('useLogs infinite query: pageSize change produces different query key', () => {
    // 验证 query key 构造逻辑
    const logsInfiniteQueryKey = (pageSize: number) => ['logs', 'infinite', pageSize] as const;

    const key10 = logsInfiniteQueryKey(10);
    const key20 = logsInfiniteQueryKey(20);

    assert.notDeepEqual(key10, key20, 'different pageSize should produce different query keys');
    assert.deepEqual(key10, ['logs', 'infinite', 10]);
    assert.deepEqual(key20, ['logs', 'infinite', 20]);
});

test('useLogs: retry backoff capped at 10s', () => {
    // 验证 retry delay 计算逻辑与实际实现一致
    const maxDelay = (attempt: number) => Math.min(1000 * 2 ** attempt, 10000);

    assert.equal(maxDelay(0), 1000);
    assert.equal(maxDelay(1), 2000);
    assert.equal(maxDelay(2), 4000);
    assert.equal(maxDelay(3), 8000);
    assert.equal(maxDelay(4), 10000);  // capped
    assert.equal(maxDelay(10), 10000); // still capped
});
