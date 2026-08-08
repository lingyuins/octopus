/**
 * 代理池协议辅助：scheme 识别、示例、徽章展示。
 *
 * 与后端 internal/utils/proxyx 支持的 scheme 保持一致：
 * http / https / socks5 / ss / vmess / vless / trojan。
 * 历史 socks:// 视为 socks5 别名。
 */

export type ProxyScheme =
    | 'http'
    | 'https'
    | 'socks5'
    | 'ss'
    | 'vmess'
    | 'vless'
    | 'trojan';

export const PROXY_SCHEMES: ProxyScheme[] = [
    'http',
    'https',
    'socks5',
    'ss',
    'vmess',
    'vless',
    'trojan',
];

/** URL 输入框的示例占位符。 */
export const PROXY_SCHEME_PLACEHOLDERS: Record<ProxyScheme, string> = {
    http: 'http://127.0.0.1:7890',
    https: 'https://proxy.example.com:8443',
    socks5: 'socks5://127.0.0.1:1080',
    ss: 'ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@example.com:8388',
    vmess: 'vmess://eyJhZGQiOiJleGFtcGxlLmNvbSIsInBvcnQiOjQ0MywiaWQiOiJ1dWlkIn0=',
    vless: 'vless://uuid@example.com:443?security=tls&sni=example.com',
    trojan: 'trojan://password@example.com:443?security=tls&sni=example.com',
};

/** 从任意 URL 字符串识别 scheme；无法识别时返回 null。 */
export function detectProxyScheme(value: string): ProxyScheme | null {
    const trimmed = value.trim();
    if (!trimmed) return null;
    const match = /^([a-zA-Z][a-zA-Z0-9+.-]*):\/\//.exec(trimmed);
    if (!match) return null;
    const scheme = match[1].toLowerCase();
    if (scheme === 'socks') return 'socks5';
    return (PROXY_SCHEMES as string[]).includes(scheme) ? (scheme as ProxyScheme) : null;
}

/** 用 scheme 前缀 + 示例尾部拼一个可用的模板 URL。 */
export function buildSchemeTemplate(scheme: ProxyScheme): string {
    return PROXY_SCHEME_PLACEHOLDERS[scheme];
}

/** 凭据位于 userinfo 用户名位的 scheme：trojan://密码@host、ss://base64(加密法:密码)@host、vless://UUID@host。 */
const USERNAME_SECRET_SCHEMES = new Set(['trojan', 'ss', 'vless']);

/**
 * 打码代理 URL 中的凭据用于列表展示。
 * 只打码 URL password 位不够：trojan/ss/vless 的密钥在用户名位，
 * vmess 整个 payload 是含 UUID 的 base64(JSON)，无法局部打码。
 */
export function maskProxyURL(value: string): string {
    const scheme = detectProxyScheme(value);
    if (scheme === 'vmess') return 'vmess://***';
    if (scheme === 'ss' && !value.includes('@')) {
        // 全 base64 形式 ss://base64(method:pass@host:port)，密码在 payload 里。
        return 'ss://***';
    }
    try {
        const parsed = new URL(value);
        if (scheme && USERNAME_SECRET_SCHEMES.has(scheme) && parsed.username) {
            parsed.username = '***';
        }
        if (parsed.password) parsed.password = '***';
        return parsed.toString();
    } catch {
        return value;
    }
}

/** 协议徽章样式：按 scheme 返回 Tailwind 类。 */
export function schemeBadgeClass(scheme: ProxyScheme): string {
    switch (scheme) {
        case 'http':
            return 'border-sky-500/30 bg-sky-500/10 text-sky-700 dark:text-sky-400';
        case 'https':
            return 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400';
        case 'socks5':
            return 'border-violet-500/30 bg-violet-500/10 text-violet-700 dark:text-violet-400';
        case 'ss':
            return 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400';
        case 'vmess':
            return 'border-rose-500/30 bg-rose-500/10 text-rose-700 dark:text-rose-400';
        case 'vless':
            return 'border-cyan-500/30 bg-cyan-500/10 text-cyan-700 dark:text-cyan-400';
        case 'trojan':
            return 'border-fuchsia-500/30 bg-fuchsia-500/10 text-fuchsia-700 dark:text-fuchsia-400';
        default:
            return 'border-border bg-muted text-muted-foreground';
    }
}

/** 大写展示名（徽章文本）。 */
export function schemeLabel(scheme: ProxyScheme): string {
    switch (scheme) {
        case 'http':
            return 'HTTP';
        case 'https':
            return 'HTTPS';
        case 'socks5':
            return 'SOCKS5';
        case 'ss':
            return 'SS';
        case 'vmess':
            return 'VMess';
        case 'vless':
            return 'VLESS';
        case 'trojan':
            return 'Trojan';
        default:
            return scheme;
    }
}
