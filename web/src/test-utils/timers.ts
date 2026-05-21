/**
 * Fake timer 工具——用于 hook 测试中控制 setTimeout / clearTimeout，
 * 不依赖真实 wall-clock 时间。
 *
 * 用法：
 *   const timers = installFakeTimers();
 *   // ... 触发逻辑，useEffect 调度 retryTimeout
 *   await timers.advanceTimersByTime(1000);  // 快进 1s
 *   timers.uninstall();
 */
export interface FakeTimers {
    advanceTimersByTime(ms: number): Promise<void>;
    uninstall(): void;
}

interface PendingTask {
    callback: () => void;
    at: number;
    cancelled?: boolean;
}

export function installFakeTimers(): FakeTimers {
    const origSetTimeout = globalThis.setTimeout;
    const origClearTimeout = globalThis.clearTimeout;

    let currentTime = 0;
    const pending: (PendingTask | undefined)[] = [];

    globalThis.setTimeout = ((callback: () => void, ms?: number) => {
        const id = pending.length;
        pending.push({ callback, at: currentTime + (ms ?? 0) });
        return id as unknown as ReturnType<typeof setTimeout>;
    }) as typeof setTimeout;

    globalThis.clearTimeout = ((id: ReturnType<typeof setTimeout> | undefined) => {
        if (id === undefined || id === null) return;
        const idx = id as unknown as number;
        if (idx >= 0 && idx < pending.length) {
            const task = pending[idx];
            if (task) {
                task.cancelled = true;
            }
        }
    }) as typeof clearTimeout;

    return {
        async advanceTimersByTime(ms: number) {
            // 先让微任务队列排空，确保 async 函数体已启动
            await new Promise<void>((r) => origSetTimeout(r, 0));

            const target = currentTime + ms;
            let ran = 0;
            while (ran < 10000) {
                // 先收集所有到期且未取消的任务索引
                const ready: number[] = [];
                for (let i = 0; i < pending.length; i++) {
                    const task = pending[i];
                    if (task && !task.cancelled && task.at <= target) {
                        ready.push(i);
                    }
                }
                if (ready.length === 0) break;
                // 按到期时间排序，取最早的
                ready.sort((a, b) => {
                    const ta = pending[a]!;
                    const tb = pending[b]!;
                    return ta.at - tb.at;
                });
                const idx = ready[0];
                const task = pending[idx]!;
                currentTime = task.at;
                const cb = task.callback;
                // 将槽位置空，防止回调内 setTimeout 复用该索引时产生误解
                pending[idx] = undefined;
                cb();
                ran++;
            }
            currentTime = target;
            // 让微任务队列排空
            await new Promise<void>((r) => origSetTimeout(r, 0));
        },
        uninstall() {
            globalThis.setTimeout = origSetTimeout;
            globalThis.clearTimeout = origClearTimeout;
        },
    };
}
