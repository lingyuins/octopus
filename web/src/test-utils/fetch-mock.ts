export interface MockFetchCall {
    url: string;
    method: string;
    headers: Record<string, string>;
    body: unknown;
    signal: AbortSignal | null;
}

export type FetchHandler = (
    call: MockFetchCall,
) => Response | Promise<Response>;

/**
 * 安装可控 fetch mock，返回 handler 注册函数与调用历史。
 *
 * 用法：
 *   const mock = installFetchMock();
 *   mock.handleNext(async (call) => new Response(JSON.stringify({data: []}), {status: 200}));
 *   // 或 mock.setDefaultHandler(handler)
 *   const calls = mock.calls();
 */
export function installFetchMock() {
    const origFetch = globalThis.fetch;
    const callLog: MockFetchCall[] = [];
    let nextHandler: FetchHandler | null = null;
    let defaultHandler: FetchHandler | null = null;

    globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
        const method = init?.method ?? 'GET';
        const headers: Record<string, string> = {};
        if (init?.headers) {
            if (init.headers instanceof Headers) {
                init.headers.forEach((v, k) => (headers[k] = v));
            } else if (Array.isArray(init.headers)) {
                (init.headers as [string, string][]).forEach(([k, v]) => (headers[k] = v));
            } else {
                Object.entries(init.headers as Record<string, string>).forEach(([k, v]) => (headers[k] = v));
            }
        }
        const call: MockFetchCall = {
            url,
            method,
            headers,
            body: init?.body,
            signal: init?.signal ?? null,
        };
        callLog.push(call);

        const handler = nextHandler ?? defaultHandler;
        if (!handler) {
            throw new Error(`Unexpected fetch call to ${url} — no handler registered`);
        }
        nextHandler = null;
        return handler(call);
    }) as typeof fetch;

    return {
        handleNext(handler: FetchHandler) {
            nextHandler = handler;
        },
        setDefaultHandler(handler: FetchHandler) {
            defaultHandler = handler;
        },
        calls(): MockFetchCall[] {
            return callLog;
        },
        clearCalls() {
            callLog.length = 0;
        },
        uninstall() {
            globalThis.fetch = origFetch;
        },
    };
}
