import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';
import type { ProxyMode } from './proxy-pool';

export interface PlanProviderCategoryInfo {
    category: string;
    name: string;
    type: 'balance' | 'tokenplan';
    base_url: string;
    models: string;
    description: string;
    help_url: string;
}

export interface PlanChannelStats {
    total_requests: number;
    total_tokens: number;
    today_requests: number;
    today_tokens: number;
    source?: 'official' | 'local';
}

export interface PlanProvider {
    id: number;
    name: string;
    category: string;
    provider_type: 'balance' | 'tokenplan';
    api_key: string;
    forward_api_key: string;
    team_organization_id: string;
    team_project_id: string;
    login_username: string;
    login_configured: boolean;
    base_url: string;
    channel_id: number;
    balance: number;
    balance_used: number;
    // 历史累计 Token 用量（输入+输出）。仅部分厂商（基元律动）提供，其他厂商为 0。
    total_tokens: number;
    total_used: number;
    quota_total: number;
    quota_used: number;
    quota_reset_at: string | null;
    weekly_total: number;
    weekly_used: number;
    weekly_reset_at: string | null;
    five_hour_total: number;
    five_hour_used: number;
    five_hour_reset_at: string | null;
    dedicated_pool_name: string;
    dedicated_weekly_total: number;
    dedicated_weekly_used: number;
    dedicated_weekly_reset_at: string | null;
    dedicated_five_hour_total: number;
    dedicated_five_hour_used: number;
    dedicated_five_hour_reset_at: string | null;
    refresh_interval_min: number;
    balance_delta: number;
    quota_used_delta: number;
    channel_stats?: PlanChannelStats | null;
    status: string;
    last_refresh: string | null;
    channel_name: string;
    channel_enabled: boolean;
    models: string;
}

// --- Balance Providers ---

export function useBalanceProviders() {
    return useQuery<PlanProvider[]>({
        queryKey: ['plan-provider', 'balance', 'list'],
        queryFn: () => apiClient.get<PlanProvider[]>('/api/v1/plan-provider/balance/list'),
        refetchInterval: 60000,
    });
}

export function useTokenPlanProviders() {
    return useQuery<PlanProvider[]>({
        queryKey: ['plan-provider', 'tokenplan', 'list'],
        queryFn: () => apiClient.get<PlanProvider[]>('/api/v1/plan-provider/tokenplan/list'),
        refetchInterval: 60000,
    });
}

export function useBalanceCategories() {
    return useQuery<PlanProviderCategoryInfo[]>({
        queryKey: ['plan-provider', 'balance', 'categories'],
        queryFn: () => apiClient.get<PlanProviderCategoryInfo[]>('/api/v1/plan-provider/balance/categories'),
        staleTime: Infinity,
    });
}

export function useTokenPlanCategories() {
    return useQuery<PlanProviderCategoryInfo[]>({
        queryKey: ['plan-provider', 'tokenplan', 'categories'],
        queryFn: () => apiClient.get<PlanProviderCategoryInfo[]>('/api/v1/plan-provider/tokenplan/categories'),
        staleTime: Infinity,
    });
}

export function useAddPlanProvider() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: {
            category: string;
            api_key: string;
            forward_api_key?: string;
            name?: string;
            // 自动刷新间隔（分钟），0 = 跟随全局默认
            refresh_interval_min?: number;
            // 代理配置：目前仅 Codex 类生效（chatgpt.com 国内不可直连）
            // 智谱团队版专用：组织 ID / 项目 ID
            team_organization_id?: string;
            team_project_id?: string;
            // 商汤日日新账号密码自动登录（sensenova_plan 专用）：
            // 填了则系统自动登录/续期控制台 Token，api_key 可留空。
            login_username?: string;
            login_password?: string;
            proxy_mode?: ProxyMode;
            proxy_config_id?: number | null;
        }) => apiClient.post('/api/v1/plan-provider/add', data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['plan-provider'] });
        },
    });
}

export function useRefreshPlanProvider() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (id: number) =>
            apiClient.post(`/api/v1/plan-provider/refresh/${id}`),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['plan-provider'] });
        },
    });
}

export function useUpdatePlanProviderCredentials() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: {
            id: number;
            api_key: string;
            forward_api_key?: string;
            team_organization_id?: string;
            team_project_id?: string;
            // 商汤日日新账号密码自动登录（sensenova_plan 专用）：
            // 填了则切换为账号密码模式并自动登录；不填则清除账号密码模式。
            login_username?: string;
            login_password?: string;
        }) => apiClient.put(`/api/v1/plan-provider/credentials/${data.id}`, {
            api_key: data.api_key,
            forward_api_key: data.forward_api_key,
            team_organization_id: data.team_organization_id,
            team_project_id: data.team_project_id,
            login_username: data.login_username,
            login_password: data.login_password,
        }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['plan-provider'] });
        },
    });
}

export function useDeletePlanProvider() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (id: number) =>
            apiClient.delete(`/api/v1/plan-provider/${id}`),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['plan-provider'] });
        },
    });
}

// --- 基元律动钱包明细（资金明细） ---

// 单笔到账额度（后端 planprovider.TokenRhythmWalletCredit）。
// 口径与官网「账户」页资金明细表一致：来源 / 到账金额 / 已消费 / 剩余可用 / 状态 / 有效期。
export interface TokenRhythmWalletCredit {
    id: string;
    // RECHARGE（充值本金）/ IDENTITY_VERIFICATION_REWARD / INVITE_REWARD / INITIAL_BALANCE ...
    source: string;
    // 上游中文标签：充值本金 / 实名认证奖励 / 邀请奖励 ...
    source_label: string;
    granted_cny: number;
    consumed_cny: number;
    remaining_cny: number;
    // ACTIVE 生效中 / PAUSED 暂停中 / USED_UP 已用尽 / EXPIRED 已到期
    status: string;
    // 本金行上游不返回 → 空串，前端显示「长期有效」
    granted_at: string;
    expires_at: string | null;
    is_principal: boolean;
}

// 资金明细查询结果（后端 planprovider.TokenRhythmWallet）
export interface TokenRhythmWallet {
    currency: string;
    available_balance_cny: number;
    gift_balance_cny: number;
    recharge_balance_cny: number;
    debt_balance_cny: number;
    frozen_balance_cny: number;
    // 累计获赠（账本 cumulativeGiftGrantedCny）；累计消费 = usage-summary 成本（与卡片「已用额度」同源）
    total_received_cny: number;
    total_consumed_cny: number;
    // 即将到期余额与最近到期时间（账本 summary）
    expiring_balance_cny: number;
    next_expiry_at: string;
    as_of: string;
    // 账本条目总数（不含本金行）；credits 为本次返回的一页（最多 50 笔，最新在前）
    total: number;
    page: number;
    page_size: number;
    credits: TokenRhythmWalletCredit[];
    // 充值本金行（granted_cny > 0 时官网才展示）
    recharge_principal: TokenRhythmWalletCredit | null;
}

// useTokenRhythmWallet 拉取基元律动钱包明细。
// providerId 传 null 时不发请求（弹窗打开才传 id），因此不会产生轮询流量。
export function useTokenRhythmWallet(providerId: number | null) {
    return useQuery<TokenRhythmWallet>({
        queryKey: ['plan-provider', 'tokenrhythm', 'wallet', providerId],
        queryFn: () =>
            apiClient.get<TokenRhythmWallet>(`/api/v1/plan-provider/wallet/transactions/${providerId}`),
        enabled: providerId != null && providerId > 0,
        staleTime: 30_000,
    });
}
