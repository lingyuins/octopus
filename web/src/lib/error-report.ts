/**
 * 前端错误捕获与上报。
 *
 * 捕获三类崩溃：
 * 1. window.onerror —— 未捕获的 JS 运行时错误
 * 2. unhandledrejection —— 未处理的 Promise 拒绝
 * 3. React 渲染错误（ErrorBoundary 单独调用 reportError 上报）
 *
 * 上报到后端 /api/v1/error-log/report（需登录态，手动携带 JWT；不走 apiClient
 * 以避免其 401 全局处理把用户强制登出）。
 * 策略：
 * - 去重：相同 message+stack 在 DEDUP_WINDOW_MS 内只上报一次，避免循环错误刷屏
 * - 静默：上报失败不抛错、不打断用户操作（console.warn 记录）
 * - 有 token 才上报（未登录崩溃无诊断上下文，避免无效请求）
 */

import { API_BASE_URL } from '@/api/client';
import { useAuthStore } from '@/api/endpoints/user';
import { useNavStore } from '@/components/modules/navbar';
import { APP_VERSION } from '@/lib/info';
import { ErrorReportDedupe } from '@/lib/error-report-core';

let initialized = false;
const dedupe = new ErrorReportDedupe();

export interface ErrorReportPayload {
    level: 'error' | 'unhandledrejection' | 'uncaught' | 'panic';
    message: string;
    stack?: string;
    page_url?: string;
    route_id?: string;
    version?: string;
}

function currentRouteId(): string {
    // 优先从导航 store 取当前模块 id（比 pathname 更稳定），取不到时回退 pathname。
    try {
        const active = useNavStore.getState().activeItem;
        if (active) return active;
    } catch {
        // ignore
    }
    return window.location.pathname || '';
}

function normalizeMessage(message: string, fallback: string): string {
    const trimmed = message?.trim();
    return trimmed || fallback;
}

export function resetErrorReportStateForTest(): void {
    dedupe.reset();
}

/**
 * 上报一条错误。调用方（全局监听 / ErrorBoundary）应先过滤无诊断价值的错误。
 * 静默失败：任何异常只 console.warn，绝不向上抛出。
 */
export async function reportError(payload: ErrorReportPayload): Promise<void> {
    try {
        const message = normalizeMessage(payload.message, 'unknown error');
        if (!message || dedupe.isDuplicate(message, payload.stack)) return;

        // 未登录（无 JWT）时不上报：report 端点需要登录态，避免无效请求。
        const token = useAuthStore.getState().token;
        if (!token) return;

        // 用裸 fetch 而非 apiClient：apiClient 的全局错误处理对 401 会强制
        // logout。上报是零交互的后台行为，token 恰好过期时不应把用户登出。
        await fetch(`${API_BASE_URL}/api/v1/error-log/report`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${token}`,
            },
            body: JSON.stringify({
                level: payload.level,
                message,
                stack: payload.stack ?? '',
                page_url: payload.page_url ?? window.location.href,
                route_id: payload.route_id ?? currentRouteId(),
                version: payload.version ?? APP_VERSION,
            }),
        });
    } catch (e) {
        console.warn('[error-report] failed to report error:', e);
    }
}

/**
 * 初始化全局错误监听（幂等，多次调用只注册一次）。
 * 建议在 App 顶层挂载后调用一次。
 */
export function initErrorReporting(): void {
    if (initialized || typeof window === 'undefined') return;
    initialized = true;

    window.addEventListener('error', (event) => {
        // 资源加载错误（img/script 等）没有 message，且多为网络问题，跳过。
        if (!event.message) return;
        void reportError({
            level: 'uncaught',
            message: event.message,
            stack: event.error?.stack,
        });
    });

    window.addEventListener('unhandledrejection', (event) => {
        const reason = event.reason;
        let message = '';
        let stack: string | undefined;
        if (reason instanceof Error) {
            message = reason.message;
            stack = reason.stack;
        } else if (reason && typeof reason === 'object' && 'message' in reason) {
            message = String((reason as { message: unknown }).message);
        } else {
            message = String(reason);
        }
        void reportError({
            level: 'unhandledrejection',
            message: normalizeMessage(message, 'Unhandled promise rejection'),
            stack,
        });
    });
}
