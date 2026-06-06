import type {
    NotifChannelType,
    GotifyConfig,
    EmailConfig,
    TelegramConfig,
    FeishuConfig,
    DingTalkConfig,
    WeComConfig,
    NtfyConfig,
} from '@/api/endpoints/alert';

export interface AlertRuleDraft {
    name: string;
    condition_type: string;
    threshold: number;
    notif_channel_id: number;
    cooldown_sec: number;
}

export interface AlertRuleEditable extends AlertRuleDraft {
    id: number;
    enabled: boolean;
    condition_json?: string;
    scope_channel_id?: number;
    scope_api_key_id?: number;
}

export interface AlertChannelDraft {
    name: string;
    type: NotifChannelType;
    url: string;
    secret: string;
    gotify: GotifyConfig;
    email: EmailConfig;
    telegram: TelegramConfig;
    feishu: FeishuConfig;
    dingtalk: DingTalkConfig;
    wecom: WeComConfig;
    ntfy: NtfyConfig;
}

export interface AlertChannelEditable {
    id: number;
    type: string;
    name: string;
    url: string;
    secret?: string;
    headers?: string;
    config?: string;
}

export function createAlertRuleDraft(rule: Partial<AlertRuleDraft> = {}): AlertRuleDraft {
    return {
        name: rule.name ?? '',
        condition_type: rule.condition_type ?? 'error_rate',
        threshold: rule.threshold ?? 10,
        notif_channel_id: rule.notif_channel_id ?? 0,
        cooldown_sec: rule.cooldown_sec ?? 300,
    };
}

export function applyAlertRuleDraft<T extends AlertRuleEditable>(rule: T, draft: AlertRuleDraft): T {
    return {
        ...rule,
        ...draft,
    };
}

function parseGotifyConfig(config?: string): GotifyConfig {
    if (!config) return { server_url: '', token: '' };
    try {
        return JSON.parse(config);
    } catch {
        return { server_url: '', token: '' };
    }
}

function parseEmailConfig(config?: string): EmailConfig {
    if (!config) return { smtp_host: '', smtp_port: 587, username: '', password: '', from: '', to: '', use_tls: true };
    try {
        return { smtp_host: '', smtp_port: 587, username: '', password: '', from: '', to: '', use_tls: true, ...JSON.parse(config) };
    } catch {
        return { smtp_host: '', smtp_port: 587, username: '', password: '', from: '', to: '', use_tls: true };
    }
}

function parseTelegramConfig(config?: string): TelegramConfig {
    if (!config) return { bot_token: '', chat_id: '' };
    try {
        return { bot_token: '', chat_id: '', ...JSON.parse(config) };
    } catch {
        return { bot_token: '', chat_id: '' };
    }
}

function parseFeishuConfig(config?: string): FeishuConfig {
    if (!config) return { webhook_key: '' };
    try {
        return { webhook_key: '', ...JSON.parse(config) };
    } catch {
        return { webhook_key: '' };
    }
}

function parseDingTalkConfig(config?: string): DingTalkConfig {
    if (!config) return { webhook_key: '' };
    try {
        return { webhook_key: '', ...JSON.parse(config) };
    } catch {
        return { webhook_key: '' };
    }
}

function parseWeComConfig(config?: string): WeComConfig {
    if (!config) return { webhook_key: '' };
    try {
        return { webhook_key: '', ...JSON.parse(config) };
    } catch {
        return { webhook_key: '' };
    }
}

function parseNtfyConfig(config?: string): NtfyConfig {
    if (!config) return { topic_url: '' };
    try {
        return { topic_url: '', ...JSON.parse(config) };
    } catch {
        return { topic_url: '' };
    }
}

export function createAlertChannelDraft<T extends Partial<AlertChannelEditable>>(channel: T = {} as T): AlertChannelDraft {
    const chType = (channel.type || 'webhook') as NotifChannelType;
    return {
        name: channel.name ?? '',
        type: chType,
        url: channel.url ?? '',
        secret: channel.secret ?? '',
        gotify: chType === 'gotify' ? parseGotifyConfig(channel.config) : { server_url: '', token: '' },
        email: chType === 'email' ? parseEmailConfig(channel.config) : { smtp_host: '', smtp_port: 587, username: '', password: '', from: '', to: '', use_tls: true },
        telegram: chType === 'telegram' ? parseTelegramConfig(channel.config) : { bot_token: '', chat_id: '' },
        feishu: chType === 'feishu' ? parseFeishuConfig(channel.config) : { webhook_key: '' },
        dingtalk: chType === 'dingtalk' ? parseDingTalkConfig(channel.config) : { webhook_key: '' },
        wecom: chType === 'wecom' ? parseWeComConfig(channel.config) : { webhook_key: '' },
        ntfy: chType === 'ntfy' ? parseNtfyConfig(channel.config) : { topic_url: '' },
    };
}

export function applyAlertChannelDraft<T extends AlertChannelEditable>(channel: T, draft: AlertChannelDraft): T {
    const result: T = {
        ...channel,
        name: draft.name,
        type: draft.type,
        url: draft.url,
        secret: draft.secret,
    };

    // Build config JSON based on channel type
    switch (draft.type) {
        case 'gotify': {
            const config: GotifyConfig = {
                server_url: draft.gotify.server_url,
                token: draft.gotify.token,
                priority: draft.gotify.priority,
            };
            result.config = JSON.stringify(config);
            // Also set url and secret from gotify config for backward compat
            if (!draft.url && config.server_url) {
                result.url = config.server_url;
            }
            if (!draft.secret && config.token) {
                result.secret = config.token;
            }
            break;
        }
        case 'email': {
            const config: EmailConfig = {
                smtp_host: draft.email.smtp_host,
                smtp_port: draft.email.smtp_port || 587,
                username: draft.email.username,
                password: draft.email.password,
                from: draft.email.from,
                to: draft.email.to,
                use_tls: draft.email.use_tls,
            };
            result.config = JSON.stringify(config);
            break;
        }
        case 'telegram': {
            const config: TelegramConfig = {
                bot_token: draft.telegram.bot_token,
                chat_id: draft.telegram.chat_id,
            };
            result.config = JSON.stringify(config);
            break;
        }
        case 'feishu': {
            const config: FeishuConfig = {
                webhook_key: draft.feishu.webhook_key,
            };
            result.config = JSON.stringify(config);
            break;
        }
        case 'dingtalk': {
            const config: DingTalkConfig = {
                webhook_key: draft.dingtalk.webhook_key,
            };
            if (draft.dingtalk.secret) {
                config.secret = draft.dingtalk.secret;
            }
            result.config = JSON.stringify(config);
            break;
        }
        case 'wecom': {
            const config: WeComConfig = {
                webhook_key: draft.wecom.webhook_key,
            };
            result.config = JSON.stringify(config);
            break;
        }
        case 'ntfy': {
            const config: NtfyConfig = {
                topic_url: draft.ntfy.topic_url,
            };
            if (draft.ntfy.access_token) {
                config.access_token = draft.ntfy.access_token;
            }
            result.config = JSON.stringify(config);
            break;
        }
        default:
            // webhook: no config needed
            result.config = '';
    }

    return result;
}
