import { useSettingStore } from '../stores/setting.ts';

/**
 * 获取当前应用时区（来自用户设置或默认 Asia/Shanghai）。
 */
export function getCurrentTimeZone(): string {
    return useSettingStore.getState().timeZone;
}

/**
 * 安全地将值转为 Date，失败返回 null。
 */
function safeDate(value: number | string | Date): Date | null {
    if (value instanceof Date) {
        return isNaN(value.getTime()) ? null : value;
    }
    if (typeof value === 'number') {
        if (value <= 0) return null;
        // 若 < 1e12 视为秒级，否则毫秒级
        const ms = value < 1e12 ? value * 1000 : value;
        const d = new Date(ms);
        return isNaN(d.getTime()) ? null : d;
    }
    if (typeof value === 'string') {
        if (!value) return null;
        const d = new Date(value);
        return isNaN(d.getTime()) ? null : d;
    }
    return null;
}

const DEFAULT_FALLBACK = '-';

/**
 * 格式化 Unix 秒级时间戳为完整日期时间字符串。
 * 使用应用时区。
 */
export function formatUnixSeconds(
    value: number | undefined | null,
    options?: Intl.DateTimeFormatOptions,
): string {
    if (value == null || value <= 0) return DEFAULT_FALLBACK;
    const d = safeDate(value);
    if (!d) return DEFAULT_FALLBACK;
    return new Intl.DateTimeFormat(undefined, {
        ...(options ?? { dateStyle: 'medium', timeStyle: 'medium' }),
        timeZone: getCurrentTimeZone(),
    }).format(d);
}

/**
 * 格式化 Unix 毫秒级时间戳为完整日期时间字符串。
 * 使用应用时区。
 */
export function formatUnixMillis(
    value: number | undefined | null,
    options?: Intl.DateTimeFormatOptions,
): string {
    if (value == null || value <= 0) return DEFAULT_FALLBACK;
    const d = new Date(value);
    if (isNaN(d.getTime())) return DEFAULT_FALLBACK;
    return new Intl.DateTimeFormat(undefined, {
        ...(options ?? { dateStyle: 'medium', timeStyle: 'medium' }),
        timeZone: getCurrentTimeZone(),
    }).format(d);
}

/**
 * 格式化 ISO 字符串或 Date 为完整日期时间字符串。
 * 使用应用时区。
 */
export function formatDateTime(
    value: string | Date | undefined | null,
    options?: Intl.DateTimeFormatOptions,
): string {
    if (value == null) return DEFAULT_FALLBACK;
    const d = safeDate(value);
    if (!d) return DEFAULT_FALLBACK;
    return new Intl.DateTimeFormat(undefined, {
        ...(options ?? { dateStyle: 'medium', timeStyle: 'medium' }),
        timeZone: getCurrentTimeZone(),
    }).format(d);
}

/**
 * 格式化 ISO 字符串或 Date 为仅日期（不含时间）。
 * 使用应用时区。
 */
export function formatDateOnly(
    value: string | Date | undefined | null,
    options?: Intl.DateTimeFormatOptions,
): string {
    if (value == null) return DEFAULT_FALLBACK;
    const d = safeDate(value);
    if (!d) return DEFAULT_FALLBACK;
    return new Intl.DateTimeFormat(undefined, {
        ...(options ?? { dateStyle: 'medium' }),
        timeZone: getCurrentTimeZone(),
    }).format(d);
}
