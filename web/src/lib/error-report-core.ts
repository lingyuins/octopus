/**
 * 错误上报去重纯逻辑（无任何依赖，便于单元测试）。
 *
 * 相同 message + stack 在 DEDUP_WINDOW_MS 内只上报一次，避免循环错误刷屏。
 */

export const DEDUP_WINDOW_MS = 30_000;

/** 去重表容量上限：防止 message 含时间戳/URL 等变化片段时无限增长。 */
export const DEDUP_MAX_ENTRIES = 100;

export class ErrorReportDedupe {
    // 多槽位：单槽位在两种错误交替出现时（如两个轮询接口各自报错）
    // 每次都判为"新错误"，30 秒窗口形同虚设。
    private reports = new Map<string, number>();

    /**
     * 返回 true 表示该错误在窗口内已上报过，应跳过。
     */
    isDuplicate(message: string, stack?: string): boolean {
        const key = `${message}|${(stack ?? '').slice(0, 200)}`;
        const now = Date.now();
        const last = this.reports.get(key);
        if (last !== undefined && now - last < DEDUP_WINDOW_MS) {
            return true;
        }
        this.prune(now);
        this.reports.set(key, now);
        return false;
    }

    private prune(now: number): void {
        for (const [key, at] of this.reports) {
            if (now - at >= DEDUP_WINDOW_MS) this.reports.delete(key);
        }
        // 极端情况下（窗口内大量不同 message）按插入序淘汰最旧的。
        while (this.reports.size >= DEDUP_MAX_ENTRIES) {
            const oldest = this.reports.keys().next().value;
            if (oldest === undefined) break;
            this.reports.delete(oldest);
        }
    }

    reset(): void {
        this.reports.clear();
    }
}
