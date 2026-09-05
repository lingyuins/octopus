'use client';

import { Plus, RefreshCw, Trash2, Key, LayoutList, ExternalLink, Loader2 } from 'lucide-react';
import { useCallback, useState, useEffect } from 'react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Hint } from '@/components/ui/hint';
import { Input } from '@/components/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from '@/components/ui/dialog';
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/animate-ui/components/animate/tooltip';
import {
    useBalanceProviders,
    useTokenPlanProviders,
    useBalanceCategories,
    useTokenPlanCategories,
    useAddPlanProvider,
    useRefreshPlanProvider,
    useUpdatePlanProviderCredentials,
    useDeletePlanProvider,
    type PlanProvider,
    type PlanProviderCategoryInfo,
} from '@/api/endpoints/plan-provider';
import { ProxySelector } from '@/components/modules/proxy-pool/ProxySelector';
import type { ProxyMode } from '@/api/endpoints/proxy-pool';
import { useSettingList, SettingKey } from '@/api/endpoints/setting';

// 与后端 model.PlanProviderDeepSeek 对应的类别标识（DeepSeek 专属统计展示）
const DEEPSEEK_PLAN_CATEGORY = 'deepseek';
// MiMo 套餐类别标识：月度订阅固定额度、无滚动刷新，仅展示月档一个用量条
const MIMO_PLAN_CATEGORY = 'mimo_plan';

// --- Balance Section ---

export function BalanceSection() {
    const t = useTranslations('hub');
    const { data: providers = [], isLoading, error } = useBalanceProviders();
    const { data: categories = [] } = useBalanceCategories();

    return (
        <PlanProviderSection
            type="balance"
            title={t('plan.balance') || '额度'}
            providers={providers}
            categories={categories}
            isLoading={isLoading}
            error={error}
        />
    );
}

// --- TokenPlan Section ---

export function TokenPlanSection() {
    const t = useTranslations('hub');
    const { data: providers = [], isLoading, error } = useTokenPlanProviders();
    const { data: categories = [] } = useTokenPlanCategories();

    return (
        <PlanProviderSection
            type="tokenplan"
            title={t('plan.tokenPlan') || 'TokenPlan'}
            providers={providers}
            categories={categories}
            isLoading={isLoading}
            error={error}
        />
    );
}

// --- Shared Section Component ---

interface PlanProviderSectionProps {
    type: 'balance' | 'tokenplan';
    title: string;
    providers: PlanProvider[];
    categories: PlanProviderCategoryInfo[];
    isLoading: boolean;
    error: unknown;
}

function PlanProviderSection({ type, title, providers, categories, isLoading, error }: PlanProviderSectionProps) {
    const t = useTranslations('hub');
    const tProxy = useTranslations('proxyPool');
    const addMutation = useAddPlanProvider();
    const refreshMutation = useRefreshPlanProvider();
    const deleteMutation = useDeletePlanProvider();
    const updateCredsMutation = useUpdatePlanProviderCredentials();
    const [editingProvider, setEditingProvider] = useState<PlanProvider | null>(null);
    const [addOpen, setAddOpen] = useState(false);
    const [selectedCategory, setSelectedCategory] = useState('');
    const [apiKey, setApiKey] = useState('');
    const [forwardApiKey, setForwardApiKey] = useState('');
    const [customName, setCustomName] = useState('');
    const [mimoAuthMode, setMimoAuthMode] = useState<'passToken' | 'serviceToken'>('serviceToken');
    // 火山方舟 Agent Plan 凭据方式：Cookie+CSRF / AK/SK（两个条目合并为一个厂商）
    const [volcengineAuthMode, setVolcengineAuthMode] = useState<'cookie' | 'aksk'>('cookie');
    // 商汤日日新凭据方式：Bearer Token（手动） / 账号密码（自动登录续期）
    const [senseNovaAuthMode, setSenseNovaAuthMode] = useState<'token' | 'account'>('token');
    const [loginUsername, setLoginUsername] = useState('');
    const [loginPassword, setLoginPassword] = useState('');
    // 智谱团队版组织/项目 ID
    const [teamOrgId, setTeamOrgId] = useState('');
    const [teamProjectId, setTeamProjectId] = useState('');
    // 代理配置（仅 Codex 类展示/提交，chatgpt.com 国内不可直连）
    const [proxyMode, setProxyMode] = useState<ProxyMode>('direct');
    const [proxyConfigId, setProxyConfigId] = useState<number | null>(null);
    // 自动刷新间隔（分钟），0 = 跟随全局默认
    const [refreshInterval, setRefreshInterval] = useState(0);
    // 全局默认刷新间隔（来自设置），用于展示"跟随全局（N 分钟）"
    const { data: settings } = useSettingList();
    const globalRefreshMin = Number(settings?.find((s) => s.key === SettingKey.PlanProviderRefreshInterval)?.value) || 30;
    
    // Compact view state with localStorage persistence
    const [compactView, setCompactView] = useState(() => {
        if (typeof window === 'undefined') return false;
        const stored = localStorage.getItem('tokenplan-compact-view');
        return stored === 'true';
    });
    
    useEffect(() => {
        localStorage.setItem('tokenplan-compact-view', String(compactView));
    }, [compactView]);
    const isConsoleTokenPlan = selectedCategory === 'stepfun_plan' || selectedCategory === 'sensenova_plan' || selectedCategory === 'mimo_plan' || selectedCategory === 'bailian_plan' || selectedCategory === 'volcengine_plan' || selectedCategory === 'volcengine_plan_ak';
    const isVolcenginePlan = selectedCategory === 'volcengine_plan';
    // AK/SK 条目从厂商下拉隐藏，由凭据方式切换决定实际提交的 category
    const isVolcengineAKSK = isVolcenginePlan && volcengineAuthMode === 'aksk';
    const visibleCategories = categories.filter((c) => c.category !== 'volcengine_plan_ak');
    const isZhipuTeam = selectedCategory === 'zhipu_team';
    const isMiMoPlan = selectedCategory === 'mimo_plan';
    const isSenseNovaPlan = selectedCategory === 'sensenova_plan';
    const isCodexPlan = selectedCategory === 'codex';
    const supportsForwardApiKey = isConsoleTokenPlan && !isMiMoPlan;
    // 商汤日日新账号密码模式：apiKey 可留空，改填账号密码
    const useAccountLogin = isSenseNovaPlan && senseNovaAuthMode === 'account';

    const handleAdd = useCallback(async () => {
        if (!selectedCategory) return;
        if (useAccountLogin) {
            if (!loginUsername.trim() || !loginPassword.trim()) {
                toast.error(t('plan.senseNovaAccountMissing') || '请填写登录账号和密码');
                return;
            }
        } else if (!apiKey.trim()) {
            return;
        }
        if (isCodexPlan && proxyMode === 'pool' && !proxyConfigId) {
            toast.error(tProxy('selectRequired'));
            return;
        }
        if (isZhipuTeam && (!teamOrgId.trim() || !teamProjectId.trim())) {
            toast.error(t('plan.zhipuTeamMissingIds') || '智谱团队版需填写组织 ID 和项目 ID');
            return;
        }
        try {
            await addMutation.mutateAsync({
                category: isVolcengineAKSK ? 'volcengine_plan_ak' : selectedCategory,
                ...(useAccountLogin
                    ? { api_key: '', login_username: loginUsername.trim(), login_password: loginPassword.trim() }
                    : { api_key: apiKey.trim() }),
                forward_api_key: supportsForwardApiKey && forwardApiKey.trim() ? forwardApiKey.trim() : undefined,
                name: customName.trim() || undefined,
                refresh_interval_min: refreshInterval,
                ...(isZhipuTeam
                    ? { team_organization_id: teamOrgId.trim(), team_project_id: teamProjectId.trim() }
                    : {}),
                ...(isCodexPlan
                    ? { proxy_mode: proxyMode, proxy_config_id: proxyMode === 'pool' ? proxyConfigId : null }
                    : {}),
            });
            toast.success('已添加');
            setAddOpen(false);
            setSelectedCategory('');
            setApiKey('');
            setForwardApiKey('');
            setCustomName('');
            setMimoAuthMode('serviceToken');
            setVolcengineAuthMode('cookie');
            setSenseNovaAuthMode('token');
            setLoginUsername('');
            setLoginPassword('');
            setTeamOrgId('');
            setTeamProjectId('');
            setProxyMode('direct');
            setProxyConfigId(null);
            setRefreshInterval(0);
        } catch (e: unknown) {
            const msg = e instanceof Error ? e.message : '添加失败';
            toast.error(msg);
        }
    }, [selectedCategory, apiKey, forwardApiKey, supportsForwardApiKey, customName, addMutation, isCodexPlan, proxyMode, proxyConfigId, tProxy, isZhipuTeam, teamOrgId, teamProjectId, t, refreshInterval, useAccountLogin, loginUsername, loginPassword, isVolcengineAKSK]);

    const handleRefresh = useCallback(async (id: number) => {
        try {
            await refreshMutation.mutateAsync(id);
            toast.success('已刷新');
        } catch (e: unknown) {
            const msg = e instanceof Error ? e.message : '刷新失败';
            toast.error(msg);
        }
    }, [refreshMutation]);

    const handleDelete = useCallback(async (id: number) => {
        try {
            await deleteMutation.mutateAsync(id);
            toast.success('已删除');
        } catch (e: unknown) {
            const msg = e instanceof Error ? e.message : '删除失败';
            toast.error(msg);
        }
    }, [deleteMutation]);

    const handleEdit = useCallback((p: PlanProvider) => {
        setEditingProvider(p);
    }, []);

    const selectedInfo = categories.find(c => c.category === selectedCategory);
    // 火山方舟合并展示：切到 AK/SK 模式时描述换成 AK/SK 条目的说明
    const selectedInfoDesc = isVolcengineAKSK
        ? (categories.find(c => c.category === 'volcengine_plan_ak')?.description ?? selectedInfo?.description ?? '')
        : (selectedInfo?.description ?? '');

    return (
        <div className="space-y-4">
            {/* Header */}
            <div className="flex items-center justify-between">
                <h2 className="text-lg font-semibold">{title}</h2>
                <div className="flex items-center gap-2">
                    {type === 'tokenplan' && (
                        <Button
                            size="sm"
                            variant="ghost"
                            className="rounded-xl gap-1.5"
                            onClick={() => setCompactView(!compactView)}
                        >
                            <LayoutList className="size-4" />
                            <span className="hidden sm:inline">{compactView ? '详细' : '极简'}</span>
                        </Button>
                    )}
                    <Dialog open={addOpen} onOpenChange={(open) => { setAddOpen(open); if (!open) { setMimoAuthMode('serviceToken'); setVolcengineAuthMode('cookie'); setSenseNovaAuthMode('token'); setApiKey(''); setLoginUsername(''); setLoginPassword(''); } }}>
                        <DialogTrigger asChild>
                            <Button size="sm" className="rounded-xl gap-1.5">
                                <Plus className="size-4" />
                                <span className="hidden sm:inline">{t('plan.addProvider') || '添加'}</span>
                            </Button>
                        </DialogTrigger>
                    <DialogContent className="sm:max-w-md">
                        <DialogHeader>
                            <DialogTitle>{t('plan.addProviderTitle') || '添加额度监控'}</DialogTitle>
                            <DialogDescription>
                                {t('plan.addProviderDesc') || '选择厂商并输入 API Key，系统将自动查询额度并创建渠道。'}
                            </DialogDescription>
                        </DialogHeader>
                        <div className="space-y-4 py-2">
                            <div className="space-y-2">
                                <label className="text-sm font-medium">{t('plan.provider') || '厂商'}</label>
                                <Select value={selectedCategory} onValueChange={setSelectedCategory}>
                                    <SelectTrigger>
                                        <SelectValue placeholder={t('plan.selectProvider') || '选择厂商'} />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {visibleCategories.map((cat) => (
                                            <SelectItem key={cat.category} value={cat.category}>
                                                {cat.name}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                                {selectedInfo && (
                                    <p className="text-xs text-muted-foreground">
                                        {selectedInfoDesc}
                                        {selectedInfo.help_url && (
                                            <a
                                                href={selectedInfo.help_url}
                                                target="_blank"
                                                rel="noopener noreferrer"
                                                className="ml-2 text-primary hover:underline inline-flex items-center gap-0.5"
                                            >
                                                <ExternalLink className="size-3" />
                                                {t('plan.getKey') || '获取 Key'}
                                            </a>
                                        )}
                                    </p>
                                )}
                            </div>

                            <div className="space-y-2">
                                {isSenseNovaPlan && (
                                    <div className="space-y-1">
                                        <label className="text-sm font-medium">
                                            {t('plan.senseNovaAuthModeLabel') || '凭据方式'}
                                        </label>
                                        <Select value={senseNovaAuthMode} onValueChange={(v: string) => { setSenseNovaAuthMode(v as 'token' | 'account'); setApiKey(''); setLoginPassword(''); }}>
                                            <SelectTrigger className="h-9 text-sm">
                                                <SelectValue />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="token">Bearer Token（3 小时过期，需手动更换）</SelectItem>
                                                <SelectItem value="account">{t('plan.senseNovaAuthModeAccount') || '账号密码（自动登录续期）'}</SelectItem>
                                            </SelectContent>
                                        </Select>
                                    </div>
                                )}
                                {isVolcenginePlan && (
                                    <div className="space-y-1">
                                        <label className="text-sm font-medium">
                                            {t('plan.volcengineAuthModeLabel') || '凭据方式'}
                                        </label>
                                        <Select value={volcengineAuthMode} onValueChange={(v: string) => { setVolcengineAuthMode(v as 'cookie' | 'aksk'); setApiKey(''); }}>
                                            <SelectTrigger className="h-9 text-sm">
                                                <SelectValue />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="cookie">{t('plan.volcengineAuthModeCookie') || 'Cookie + CSRF Token'}</SelectItem>
                                                <SelectItem value="aksk">{t('plan.volcengineAuthModeAKSK') || 'AK/SK（AccessKey 签名）'}</SelectItem>
                                            </SelectContent>
                                        </Select>
                                    </div>
                                )}
                                <label className="text-sm font-medium">
                                    {isMiMoPlan
                                        ? (t('plan.cookieLabel') || 'Cookie')
                                        : isCodexPlan
                                            ? (t('plan.codexOAuthLabel') || 'OAuth JSON')
                                            : isConsoleTokenPlan
                                                ? (t('plan.consoleTokenLabel') || '控制台 Token')
                                                : (t('plan.apiKeyLabel') || 'API Key')}
                                </label>
                                {isMiMoPlan && (
                                    <div className="space-y-1">
                                        <Select value={mimoAuthMode} onValueChange={(v: string) => { setMimoAuthMode(v as 'passToken' | 'serviceToken'); setApiKey(''); }}>
                                            <SelectTrigger className="h-9 text-sm">
                                                <SelectValue />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="serviceToken">serviceToken — 1 天有效</SelectItem>
                                                <SelectItem value="passToken">passToken — 30 天，自动刷新</SelectItem>
                                            </SelectContent>
                                        </Select>
                                    </div>
                                )}
                                {isSenseNovaPlan && senseNovaAuthMode === 'account' ? (
                                    <div className="space-y-2">
                                        <label className="text-sm font-medium">
                                            {t('plan.senseNovaUsernameLabel') || '登录账号'}
                                        </label>
                                        <Input
                                            placeholder={t('plan.senseNovaUsernamePlaceholder') || 'platform.sensenova.cn 控制台登录账号（手机号/用户名）'}
                                            value={loginUsername}
                                            onChange={(e) => setLoginUsername(e.target.value)}
                                        />
                                        <label className="text-sm font-medium">
                                            {t('plan.senseNovaPasswordLabel') || '登录密码'}
                                        </label>
                                        <Input
                                            type="password"
                                            placeholder={t('plan.senseNovaPasswordPlaceholder') || '控制台登录密码'}
                                            value={loginPassword}
                                            onChange={(e) => setLoginPassword(e.target.value)}
                                        />
                                        <p className="text-[11px] leading-tight text-emerald-600">
                                            {t('plan.senseNovaAccountHint') || '系统自动完成登录并续期控制台 Token（约 3 小时有效期），全程无需手动更换。账号密码 AES 加密存储，仅用于自动登录。'}
                                        </p>
                                    </div>
                                ) : (
                                <Input
                                    type="password"
                                    placeholder={isMiMoPlan
                                        ? (mimoAuthMode === 'passToken'
                                            ? (t('plan.mimoPassTokenPlaceholder') || '粘贴 account.xiaomi.com 的完整 Cookie（需包含 passToken=、userId=、cUserId= 字段）')
                                            : (t('plan.mimoServiceTokenPlaceholder') || '粘贴 platform.xiaomimimo.com 的完整 Cookie（需包含 api-platform_serviceToken=、userId=、api-platform_slh=、api-platform_ph= 字段）'))
                                        : isCodexPlan
                                            ? (t('plan.codexOAuthPlaceholder') || '粘贴 OAuth JSON 凭据（含 access_token 和 account_id）')
                                            : isConsoleTokenPlan
                                                ? (isVolcengineAKSK
                                                    ? (t('plan.volcengineAKSKPlaceholder') || 'AccessKey ID|||Secret Access Key（火山控制面 OpenAPI 签名用，与推理 Key 不同）')
                                                    : isVolcenginePlan
                                                        ? (t('plan.volcengineCredentialPlaceholder') || 'Cookie值|||x-csrf-token值（从控制台请求头复制，用竖线分隔）')
                                                        : selectedInfo?.category === 'sensenova_plan'
                                                            ? (t('plan.sensenovaTokenPlaceholder') || '粘贴控制台 Bearer Token 值')
                                                            : selectedInfo?.category === 'bailian_plan'
                                                                ? (t('plan.bailianTokenPlaceholder') || '粘贴控制台完整 Cookie 值')
                                                                : (t('plan.oasisTokenPlaceholder') || '粘贴控制台 Cookie 中的 Oasis-Token 值'))
                                                : (t('plan.apiKeyPlaceholder') || '请输入 API Key')}
                                    value={apiKey}
                                    onChange={(e) => setApiKey(e.target.value)}
                                />
                                )}
                                {isMiMoPlan && mimoAuthMode === 'passToken' && (
                                    <p className="text-[11px] leading-tight text-red-500">
                                        {t('plan.mimoPassTokenHint') || '⚠️ 安全风险极高：passToken 是小米账号长期会话凭证，可能可以换取小米云、小米社区、MiMo 等任何接入小米账号体系的服务的 Token（未验证）。填入后系统自动通过 SSO 刷新 serviceToken，无需手动更新。'}
                                    </p>
                                )}
                                {isMiMoPlan && mimoAuthMode === 'serviceToken' && (
                                    <p className="text-[11px] leading-tight text-amber-500">
                                        {t('plan.mimoServiceTokenHint') || '登录 platform.xiaomimimo.com → F12 → Application → Cookies，复制 api-platform 域下所有 Cookie。有效期约 1 天，过期后需手动更新。'}
                                    </p>
                                )}
                                {isConsoleTokenPlan && !isMiMoPlan && !(isSenseNovaPlan && senseNovaAuthMode === 'account') && (
                                    <p className="text-[11px] leading-tight text-amber-500">
                                        {isVolcengineAKSK
                                            ? (t('plan.volcengineAKSKHint') || '在 console.volcengine.com/iam → 密钥管理 创建 AccessKey ID 与 Secret（与推理 API Key 是两套凭据），用 ||| 连接。系统通过控制面 OpenAPI 签名查询，先查 Agent Plan，无订阅再查 Coding Plan。')
                                            : isVolcenginePlan
                                                ? (t('plan.volcengineCredentialHint') || '登录 console.volcengine.com/ark → F12 → Network → 任意 plan 接口，复制完整 Cookie 请求头和 x-csrf-token 请求头，用 ||| 连接。会话过期后需重新获取。')
                                                : selectedInfo?.category === 'bailian_plan'
                                                    ? (t('plan.bailianTokenHint') || '需登录 bailian.console.aliyun.com 控制台，按 F12 打开开发者工具 → Network（网络）→ 刷新页面，点击任意请求，从请求头（Request Headers）复制完整 Cookie 值。会话过期后需重新获取。')
                                                    : selectedInfo?.category === 'sensenova_plan'
                                                        ? (t('plan.sensenovaTokenHint') || '需登录 platform.sensenova.cn 控制台，从请求头复制 Bearer Token 值。有效期约 3 小时，过期后需重新获取。')
                                                        : (t('plan.oasisTokenHint') || '需登录 platform.stepfun.com 控制台，从浏览器 Cookie 复制 Oasis-Token 值（格式：access...refresh）。该 Token 有效期约 30 分钟，过期后需重新获取。')}
                                    </p>
                                )}
                                {isCodexPlan && (
                                    <p className="text-[11px] leading-tight text-amber-500">
                                        {t('plan.codexOAuthHint') || '从 ChatGPT 订阅账号获取 OAuth JSON 凭据（含 access_token 和 account_id）。系统将自动创建 Codex 类型转发渠道（接入点 chatgpt.com/backend-api/codex/responses）。access_token 有效期较短，过期后需重新获取。'}
                                    </p>
                                )}
                            </div>
                            {isZhipuTeam && (
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">
                                        {t('plan.zhipuTeamOrgIdLabel') || '组织 ID'}
                                        <Hint text={t('plan.zhipuTeamHint') || '智谱团队版需 API Key + 组织 ID + 项目 ID 三者齐全（从 open.bigmodel.cn 控制台团队设置页获取）。'} />
                                    </label>
                                    <Input
                                        placeholder={t('plan.zhipuTeamOrgIdPlaceholder') || 'bigmodel-organization 请求头值'}
                                        value={teamOrgId}
                                        onChange={(e) => setTeamOrgId(e.target.value)}
                                    />
                                    <label className="text-sm font-medium mt-2 block">
                                        {t('plan.zhipuTeamProjectIdLabel') || '项目 ID'}
                                    </label>
                                    <Input
                                        placeholder={t('plan.zhipuTeamProjectIdPlaceholder') || 'bigmodel-project 请求头值'}
                                        value={teamProjectId}
                                        onChange={(e) => setTeamProjectId(e.target.value)}
                                    />
                                    <p className="text-xs text-muted-foreground">
                                        {t('plan.zhipuTeamHint') || '智谱团队版需 API Key + 组织 ID + 项目 ID 三者齐全（从 open.bigmodel.cn 控制台团队设置页获取）。'}
                                    </p>
                                </div>
                            )}
                            {supportsForwardApiKey && (
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">
                                        {t('plan.forwardApiKeyLabel') || 'API Key（可选）'}
                                        <Hint text={t('plan.forwardApiKeyHint') || '填写后将自动创建或复用转发渠道，模型相同的合并为同一渠道。留空则仅监控套餐额度。'} />
                                    </label>
                                    <Input
                                        type="password"
                                        placeholder={t('plan.forwardApiKeyPlaceholder') || 'sk- 开头的 API Key，用于转发'}
                                        value={forwardApiKey}
                                        onChange={(e) => setForwardApiKey(e.target.value)}
                                    />
                                    <p className="text-xs text-muted-foreground">
                                        {t('plan.forwardApiKeyHint') || '填写后将自动创建或复用转发渠道，模型相同的合并为同一渠道。留空则仅监控套餐额度。'}
                                    </p>
                                </div>
                            )}

                            {isCodexPlan && (
                                <ProxySelector
                                    value={{ proxy_mode: proxyMode, proxy_config_id: proxyConfigId }}
                                    onChange={(next) => {
                                        setProxyMode(next.proxy_mode);
                                        setProxyConfigId(next.proxy_mode === 'pool' ? next.proxy_config_id ?? null : null);
                                    }}
                                />
                            )}

                            <div className="space-y-2">
                                <label className="text-sm font-medium">{t('plan.customName') || '自定义名称（可选）'}</label>
                                <Input
                                    placeholder={t('plan.customNamePlaceholder') || '留空则使用默认名称'}
                                    value={customName}
                                    onChange={(e) => setCustomName(e.target.value)}
                                />
                            </div>

                            <div className="space-y-2">
                                <label className="text-sm font-medium">
                                    {t('plan.refreshInterval') || '自动刷新间隔'}
                                    <Hint text={t('plan.refreshIntervalHint') || '按此间隔自动查询额度；全局默认可在 Hub 自动化面板调整'} />
                                </label>
                                <Select
                                    value={String(refreshInterval)}
                                    onValueChange={(v) => setRefreshInterval(Number(v))}
                                >
                                    <SelectTrigger className="h-9 text-sm">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="0">
                                            {t('plan.refreshFollowGlobal')?.replace('{minutes}', String(globalRefreshMin)) || `跟随全局（${globalRefreshMin} 分钟）`}
                                        </SelectItem>
                                        {[10, 15, 30, 60, 120, 360, 1440].map((m) => (
                                            <SelectItem key={m} value={String(m)}>
                                                {t('plan.refreshEvery')?.replace('{minutes}', String(m)) || `每 ${m} 分钟`}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                                <p className="text-xs text-muted-foreground">
                                    {t('plan.refreshIntervalHint') || '按此间隔自动查询额度；全局默认可在 Hub 自动化面板调整'}
                                </p>
                            </div>

                            <Button
                                className="w-full rounded-xl"
                                onClick={handleAdd}
                                disabled={!selectedCategory || (useAccountLogin ? (!loginUsername.trim() || !loginPassword.trim()) : !apiKey.trim()) || addMutation.isPending}
                            >
                                {addMutation.isPending ? (
                                    <>
                                        <Loader2 className="size-4 animate-spin mr-2" />
                                        {t('plan.querying') || '查询中...'}
                                    </>
                                ) : (
                                    t('plan.add') || '添加并查询'
                                )}
                            </Button>
                        </div>
                    </DialogContent>
                </Dialog>
            </div>
            </div>
            {/* Content */}
            {isLoading ? (
                <div className="rounded-xl border border-border bg-card p-6 text-sm text-muted-foreground text-center">
                    {t('plan.loading') || '正在加载...'}
                </div>
            ) : error ? (
                <div className="text-center py-8 text-muted-foreground">
                    {t('plan.loadError') || '加载失败'}
                </div>
            ) : providers.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground border border-dashed rounded-xl">
                    <Key className="size-8 mx-auto mb-2 opacity-30" />
                    <p>{t('plan.empty') || '暂无监控项，点击上方按钮添加'}</p>
                </div>
            ) : (
                <div className="space-y-3">
                    {providers.map((provider) => (
                        <ProviderCard
                            key={provider.id}
                            provider={provider}
                            onRefresh={handleRefresh}
                            onDelete={handleDelete}
                            onEdit={handleEdit}
                            isRefreshing={refreshMutation.isPending}
                            isDeleting={deleteMutation.isPending}
                            isEditing={updateCredsMutation.isPending}
                            compact={type === 'tokenplan' && compactView}
                        />
                    ))}
                </div>
            )}
            <EditCredentialsDialog
                provider={editingProvider}
                categories={categories}
                onOpenChange={(open) => { if (!open) setEditingProvider(null); }}
            />
        </div>
    );
}

// --- Provider Card ---

const formatBalance = (val: number) => {
    if (val === 0) return '0';
    if (Math.abs(val) < 0.01) return val.toFixed(6);
    return val.toLocaleString(undefined, { maximumFractionDigits: 2 });
};

// formatTokens 格式化 token 使用量（万/亿中文单位，与 Analytics 页 formatCount 口径一致）。
const formatTokens = (val: number) => {
    if (!val) return '0';
    if (val >= 100_000_000) return `${(val / 100_000_000).toFixed(2)}亿`;
    if (val >= 10_000) return `${(val / 10_000).toFixed(2)}万`;
    return val.toLocaleString();
};

const formatTime = (val: string | null) => {
    if (!val) return '';
    try {
        const d = new Date(val);
        return d.toLocaleString('zh-CN', {
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
        });
    } catch {
        return val;
    }
};

// 单档配额卡片（官网三档样式：标题 + 倒计时 + 已用/总量 + 进度条百分比）
function QuotaTier({
    label,
    total,
    used,
    resetAt,
    compact = false,
    className,
}: {
    label: string;
    total: number;
    used: number;
    resetAt: string | null;
    compact?: boolean;
    className?: string;
}) {
    const t = useTranslations('hub');
    const [countdown, setCountdown] = useState('');

    // 官网风格相对倒计时："13天23小时后重置" / "即将重置"。
    // Date.now() 是 impure，不能直接在 render 中调用，故在 effect 中计算并定时刷新。
    useEffect(() => {
        if (!resetAt) {
            setCountdown('');
            return;
        }
        const target = new Date(resetAt).getTime();
        const compute = () => {
            const ms = target - Date.now();
            if (Number.isNaN(ms) || ms <= 0) {
                setCountdown(t('plan.resetNow') || '即将重置');
                return;
            }
            const totalMin = Math.floor(ms / 60000);
            const d = Math.floor(totalMin / 1440);
            const h = Math.floor((totalMin % 1440) / 60);
            const m = totalMin % 60;
            let rel = '';
            if (d > 0) rel += `${d}${t('plan.days') || '天'}`;
            if (h > 0) rel += `${h}${t('plan.hours') || '小时'}`;
            if (d === 0 && h === 0) rel += `${m}${t('plan.minutes') || '分钟'}`;
            setCountdown(`${rel}${t('plan.resetSuffix') || '后重置'}`);
        };
        compute();
        const timer = setInterval(compute, 30000);
        return () => clearInterval(timer);
    }, [resetAt, t]);

    if (total <= 0) return null;
    const pct = Math.min(100, (used / total) * 100);

    // Compact mode: inline display without progress bar
    if (compact) {
        return (
            <div className="inline-flex items-center gap-2 text-xs">
                <span className="text-muted-foreground">{label}</span>
                <span className="font-semibold tabular-nums">{formatBalance(used)}</span>
                <span className="text-muted-foreground">/</span>
                <span className="tabular-nums text-muted-foreground">{formatBalance(total)}</span>
                <span className="text-muted-foreground">({pct.toFixed(0)}%)</span>
            </div>
        );
    }

    // Normal mode: card with progress bar
    return (
        <div className={cn('rounded-lg bg-muted/50 p-2.5', className)}>
            <div className="flex items-center justify-between mb-1">
                <p className="text-xs text-muted-foreground">{label}</p>
                {resetAt && (
                    <p className="text-xs text-muted-foreground tabular-nums">
                        {countdown}
                    </p>
                )}
            </div>
            <div className="flex items-baseline gap-1.5">
                <span className="font-semibold text-base tabular-nums">
                    {formatBalance(used)}
                </span>
                <span className="text-xs text-muted-foreground tabular-nums">
                    / {formatBalance(total)}
                </span>
            </div>
            <div className="h-1.5 rounded-full bg-muted overflow-hidden mt-1.5">
                <div
                    className="h-full rounded-full bg-primary transition-all"
                    style={{ width: `${pct}%` }}
                />
            </div>
            <div className="flex items-center justify-between text-[11px] text-muted-foreground mt-1">
                <span>{t('plan.usedLabel') || '已使用'}</span>
                <span className="tabular-nums">{pct.toFixed(1)}%</span>
            </div>
        </div>
    );
}

function ProviderCard({
    provider,
    onRefresh,
    onDelete,
    onEdit,
    isRefreshing,
    isDeleting,
    isEditing,
    compact = false,
}: {
    provider: PlanProvider;
    onRefresh: (id: number) => void;
    onDelete: (id: number) => void;
    onEdit: (provider: PlanProvider) => void;
    isRefreshing: boolean;
    isDeleting: boolean;
    isEditing: boolean;
    compact?: boolean;
}) {
    const t = useTranslations('hub');

    // Find category info for display
    const isBalance = provider.provider_type === 'balance';
    // MiMo 为月度订阅（固定额度、无滚动刷新），只展示月档一个用量条
    const isMiMo = provider.category === MIMO_PLAN_CATEGORY;

    // 有效配额档（total>0）。外层先过滤，使 length/idx 反映真实渲染数，
    // 用于 normal 网格布局的"奇数末项跨两列"判断（QuotaTier 内部
    // total<=0 返回 null 不产生 DOM，但若不过滤，map 的 idx 会被 null 项污染）。
    const tiers = [
        { key: 'five_hour', label: t('plan.tierFiveHour') || '近5小时用量', total: provider.five_hour_total, used: provider.five_hour_used, resetAt: provider.five_hour_reset_at },
        { key: 'weekly', label: t('plan.tierWeekly') || '近一周用量', total: provider.weekly_total, used: provider.weekly_used, resetAt: provider.weekly_reset_at },
        { key: 'monthly', label: t('plan.tierMonthly') || '近一月用量', total: provider.quota_total, used: provider.quota_used, resetAt: provider.quota_reset_at },
    ].filter((tier) => tier.total > 0 && !(isMiMo && tier.key !== 'monthly'));

    return (
        <div className={cn(
            'rounded-xl border border-border bg-card p-4 transition-colors',
            !provider.channel_enabled && 'opacity-60'
        )}>
            {/* Top Row */}
            <div className="flex items-start justify-between gap-3 mb-3">
                <div className="min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                        <h3 className="font-semibold text-sm truncate">{provider.name}</h3>
                        <Badge variant="outline" className="text-xs shrink-0">
                            {isBalance ? '余额' : '套餐'}
                        </Badge>
                        {provider.login_configured && (
                            <Badge className="text-xs shrink-0 bg-emerald-600 hover:bg-emerald-600">
                                {t('plan.senseNovaAutoLoginBadge') || '自动登录'}
                            </Badge>
                        )}
                    </div>
                    <div className="flex items-center gap-2 mt-1">
                        <p className="text-xs text-muted-foreground truncate">
                            {provider.channel_name || (provider.channel_id > 0 ? `渠道 #${provider.channel_id}` : (t('plan.monitorOnly') || '仅监控'))}
                        </p>
                        {provider.last_refresh && (
                            <span className="text-xs text-muted-foreground shrink-0">
                                {formatTime(provider.last_refresh)}
                            </span>
                        )}
                    </div>
                </div>

                <div className="flex items-center gap-1 shrink-0">
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button
                                size="icon"
                                variant="ghost"
                                className="size-8 rounded-lg"
                                onClick={() => onEdit(provider)}
                                disabled={isRefreshing || isDeleting || isEditing}
                            >
                                <Key className={cn('size-3.5', isEditing && 'animate-pulse')} />
                            </Button>
                        </TooltipTrigger>
                        <TooltipContent>{t('plan.editCredentials') || '更换凭据'}</TooltipContent>
                    </Tooltip>

                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button
                                size="icon"
                                variant="ghost"
                                className="size-8 rounded-lg"
                                onClick={() => onRefresh(provider.id)}
                                disabled={isRefreshing || isDeleting || isEditing}
                            >
                                <RefreshCw className={cn('size-3.5', isRefreshing && 'animate-spin')} />
                            </Button>
                        </TooltipTrigger>
                        <TooltipContent>{t('plan.refresh') || '刷新'}</TooltipContent>
                    </Tooltip>

                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button
                                size="icon"
                                variant="ghost"
                                className="size-8 rounded-lg text-destructive hover:text-destructive"
                                onClick={() => onDelete(provider.id)}
                                disabled={isRefreshing || isDeleting}
                            >
                                <Trash2 className="size-3.5" />
                            </Button>
                        </TooltipTrigger>
                        <TooltipContent>{t('plan.delete') || '删除'}</TooltipContent>
                    </Tooltip>
                </div>
            </div>

            {/* Balance / TokenPlan Info */}
            {isBalance ? (
                <div className="grid grid-cols-2 gap-3">
                    <div className="rounded-lg bg-muted/50 p-2.5">
                        <p className="text-xs text-muted-foreground mb-1">
                            {t('plan.balanceAvailable') || '可用余额'}
                        </p>
                        <p className="text-lg font-bold text-primary tabular-nums">
                            {formatBalance(provider.balance)}
                        </p>
                    </div>
                    <div className="rounded-lg bg-muted/50 p-2.5">
                        <p className="text-xs text-muted-foreground mb-1">
                            {provider.balance_used > 0
                                ? (t('plan.balanceUsed') || '已用额度')
                                : (t('plan.totalUsed') || '累计已用')}
                        </p>
                        <p className="text-lg font-bold tabular-nums text-muted-foreground">
                            {formatBalance(provider.balance_used > 0 ? provider.balance_used : provider.total_used)}
                        </p>
                    </div>
                </div>
            ) : compact ? (
                <div className="flex items-center gap-4 flex-wrap text-xs">
                    {!isMiMo && (
                        <QuotaTier
                            label={t('plan.tierFiveHour') || '5h'}
                            total={provider.five_hour_total}
                            used={provider.five_hour_used}
                            resetAt={provider.five_hour_reset_at}
                            compact
                        />
                    )}
                    {!isMiMo && (
                        <QuotaTier
                            label={t('plan.tierWeekly') || '周'}
                            total={provider.weekly_total}
                            used={provider.weekly_used}
                            resetAt={provider.weekly_reset_at}
                            compact
                        />
                    )}
                    <QuotaTier
                        label={t('plan.tierMonthly') || '月'}
                        total={provider.quota_total}
                        used={provider.quota_used}
                        resetAt={provider.quota_reset_at}
                        compact
                    />
                </div>
            ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                    {tiers.map((tier, idx) => (
                        <QuotaTier
                            key={tier.key}
                            label={tier.label}
                            total={tier.total}
                            used={tier.used}
                            resetAt={tier.resetAt}
                            className={idx === tiers.length - 1 && tiers.length % 2 === 1 ? 'sm:col-span-2' : undefined}
                        />
                    ))}
                </div>
            )}

            {/* 自动刷新间隔 */}
            <div className="mt-2 flex items-center gap-1 flex-wrap text-[11px] text-muted-foreground">
                <span>
                    {t('plan.refreshIntervalShort') || '自动刷新'}：
                    {provider.refresh_interval_min > 0
                        ? (t('plan.refreshEvery')?.replace('{minutes}', String(provider.refresh_interval_min)) || `每 ${provider.refresh_interval_min} 分钟`)
                        : (t('plan.refreshFollowGlobalShort') || '跟随全局')}
                </span>
            </div>

            {/* 本次与上次检测之间的消费增量 */}
            {isBalance && provider.balance_delta > 0 && (
                <div className="mt-2 rounded-lg bg-muted/50 p-2.5 flex items-center justify-between">
                    <p className="text-xs text-muted-foreground">{t('plan.deltaSpent') || '上次检测后消耗'}</p>
                    <p className="text-sm font-semibold tabular-nums text-destructive">
                        -{formatBalance(provider.balance_delta)}
                    </p>
                </div>
            )}
            {!isBalance && provider.quota_used_delta > 0 && (
                <div className="mt-2 rounded-lg bg-muted/50 p-2.5 flex items-center justify-between">
                    <p className="text-xs text-muted-foreground">{t('plan.deltaSpent') || '上次检测后消耗'}</p>
                    <p className="text-sm font-semibold tabular-nums text-destructive">
                        +{formatBalance(provider.quota_used_delta)}
                    </p>
                </div>
            )}

            {/* DeepSeek 专属：通过额度渠道转发的系统内调用统计 */}
            {provider.category === DEEPSEEK_PLAN_CATEGORY && provider.channel_stats && (
                <div className="mt-2 rounded-lg bg-muted/50 p-2.5">
                    <div className="flex items-center justify-between mb-1.5">
                        <p className="text-xs text-muted-foreground">
                            {t('plan.sysStats') || 'DeepSeek 额度调用统计'}
                        </p>
                        <span className="text-[10px] text-muted-foreground/70">
                            {provider.channel_stats.source === 'official'
                                ? (t('plan.sysStatsOfficial') || '官方用量')
                                : (t('plan.sysStatsLocal') || '本地统计')}
                        </span>
                    </div>
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                        <div>
                            <p className="text-[11px] text-muted-foreground">{t('plan.sysTotalRequests') || '累计调用'}</p>
                            <p className="text-sm font-semibold tabular-nums">{provider.channel_stats.total_requests}</p>
                        </div>
                        <div>
                            <p className="text-[11px] text-muted-foreground">{t('plan.sysTotalTokens') || '累计 Token'}</p>
                            <p className="text-sm font-semibold tabular-nums">{formatTokens(provider.channel_stats.total_tokens)}</p>
                        </div>
                        <div>
                            <p className="text-[11px] text-muted-foreground">{t('plan.sysTodayRequests') || '今日调用'}</p>
                            <p className="text-sm font-semibold tabular-nums">{provider.channel_stats.today_requests}</p>
                        </div>
                        <div>
                            <p className="text-[11px] text-muted-foreground">{t('plan.sysTodayTokens') || '今日 Token'}</p>
                            <p className="text-sm font-semibold tabular-nums">{formatTokens(provider.channel_stats.today_tokens)}</p>
                        </div>
                    </div>
                </div>
            )}

            {/* Models */}
            {provider.models && (
                <div className="mt-2 flex items-center gap-1 flex-wrap">
                    {provider.models.split(',').slice(0, 4).map((m) => (
                        <Badge key={m} variant="secondary" className="text-xs">
                            {m.trim()}
                        </Badge>
                    ))}
                    {provider.models.split(',').length > 4 && (
                        <Badge variant="secondary" className="text-xs">
                            +{provider.models.split(',').length - 4}
                        </Badge>
                    )}
                </div>
            )}
        </div>
    );
}

// --- Edit Credentials Dialog ---

// EditCredentialsDialog 更换 Plan Provider 凭据。
// 控制台会话凭据（stepfun/sensenova/bailian/volcengine/mimo/codex）会过期，
// 刷新时报 401/未登录；此对话框让用户粘贴新凭据并立即重查用量，
// 无需删除重建（避免连带删除关联的转发渠道与 channel keys 状态）。
// 凭据不回填（列表已脱敏），用户粘贴新值即可。
function EditCredentialsDialog({
    provider,
    categories,
    onOpenChange,
}: {
    provider: PlanProvider | null;
    categories: PlanProviderCategoryInfo[];
    onOpenChange: (open: boolean) => void;
}) {
    const t = useTranslations('hub');
    const updateMutation = useUpdatePlanProviderCredentials();
    const [apiKey, setApiKey] = useState('');
    const [forwardApiKey, setForwardApiKey] = useState('');
    const [teamOrgId, setTeamOrgId] = useState('');
    const [teamProjectId, setTeamProjectId] = useState('');
    // 商汤日日新凭据方式（默认跟随当前配置：已启用账号密码则切到账号密码模式）
    const [senseNovaAuthMode, setSenseNovaAuthMode] = useState<'token' | 'account'>('token');
    const [loginUsername, setLoginUsername] = useState('');
    const [loginPassword, setLoginPassword] = useState('');

    const open = provider !== null;
    const editingId = provider?.id;

    // 切换 provider 时重置输入（按 id 变化触发）
    useEffect(() => {
        if (editingId !== undefined) {
            setApiKey('');
            setForwardApiKey('');
            setTeamOrgId('');
            setTeamProjectId('');
            setSenseNovaAuthMode(provider?.login_configured ? 'account' : 'token');
            setLoginUsername(provider?.login_username || '');
            setLoginPassword('');
        }
    }, [editingId, provider]);

    if (!provider) {
        return (
            <Dialog open={false} onOpenChange={onOpenChange}>
                <DialogContent className="sm:max-w-md" />
            </Dialog>
        );
    }
    const category = provider.category;
    const isConsoleTokenPlan = category === 'stepfun_plan' || category === 'sensenova_plan' || category === 'bailian_plan' || category === 'volcengine_plan' || category === 'volcengine_plan_ak';
    const isVolcenginePlan = category === 'volcengine_plan';
    const isVolcengineAKSK = category === 'volcengine_plan_ak';
    const isZhipuTeam = category === 'zhipu_team';
    const isMiMoPlan = category === 'mimo_plan';
    const isCodexPlan = category === 'codex';
    const isDeepSeek = category === 'deepseek';
    const supportsForwardApiKey = isConsoleTokenPlan && !isMiMoPlan;

    const catInfo = categories.find(c => c.category === category);
    // 商汤日日新账号密码模式：apiKey 可留空，改填账号密码
    const useAccountLogin = category === 'sensenova_plan' && senseNovaAuthMode === 'account';

    const handleSubmit = async () => {
        if (useAccountLogin) {
            if (!loginUsername.trim() || !loginPassword.trim()) {
                toast.error(t('plan.senseNovaAccountMissing') || '请填写登录账号和密码');
                return;
            }
        } else if (!apiKey.trim()) {
            // DeepSeek 账号密码是附加数据源，API key 必须保留（余额查询用）
            if (isDeepSeek) {
                toast.error(t('plan.deepSeekApiKeyMissing') || '请填写 API Key（用于余额查询，控制台账号为可选）');
                return;
            }
            return;
        }
        try {
            await updateMutation.mutateAsync({
                id: provider.id,
                api_key: useAccountLogin ? '' : apiKey.trim(),
                forward_api_key: supportsForwardApiKey && forwardApiKey.trim() ? forwardApiKey.trim() : undefined,
                team_organization_id: isZhipuTeam ? teamOrgId.trim() : undefined,
                team_project_id: isZhipuTeam ? teamProjectId.trim() : undefined,
                ...(useAccountLogin
                    ? { login_username: loginUsername.trim(), login_password: loginPassword.trim() }
                    : isDeepSeek && loginUsername.trim() && loginPassword.trim()
                        ? { login_username: loginUsername.trim(), login_password: loginPassword.trim() }
                        // DeepSeek 密码留空视为"不修改账号密码"：不回传 login 字段，
                        // 后端保留原配置（仅当用户显式清空用户名时才清除）。
                        : {}),
            });
            toast.success(t('plan.credentialsUpdated') || '凭据已更新');
            onOpenChange(false);
        } catch (e: unknown) {
            const msg = e instanceof Error ? e.message : (t('plan.updateFailed') || '更新失败');
            toast.error(msg);
        }
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>{t('plan.editCredentialsTitle') || '更换凭据'}</DialogTitle>
                    <DialogDescription>
                        {t('plan.editCredentialsDesc') || '凭据过期或失效时粘贴新凭据，系统将立即重新查询用量。'}
                    </DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-2">
                    <div className="rounded-lg bg-muted/50 p-2.5">
                        <p className="text-xs text-muted-foreground">{catInfo?.name || provider.name}</p>
                        <p className="text-sm font-medium truncate">{provider.name}</p>
                    </div>

                    <div className="space-y-2">
                        {category === 'sensenova_plan' && (
                            <div className="space-y-1">
                                <label className="text-sm font-medium">
                                    {t('plan.senseNovaAuthModeLabel') || '凭据方式'}
                                </label>
                                <Select value={senseNovaAuthMode} onValueChange={(v: string) => { setSenseNovaAuthMode(v as 'token' | 'account'); setApiKey(''); setLoginPassword(''); }}>
                                    <SelectTrigger className="h-9 text-sm">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="token">Bearer Token（3 小时过期，需手动更换）</SelectItem>
                                        <SelectItem value="account">{t('plan.senseNovaAuthModeAccount') || '账号密码（自动登录续期）'}</SelectItem>
                                    </SelectContent>
                                </Select>
                                {provider.login_configured && senseNovaAuthMode === 'account' && (
                                    <p className="text-[11px] leading-tight text-emerald-600">
                                        {t('plan.senseNovaLoginConfiguredHint', { username: provider.login_username || '' })}
                                    </p>
                                )}
                            </div>
                        )}
                        <label className="text-sm font-medium">
                            {isMiMoPlan
                                ? (t('plan.cookieLabel') || 'Cookie')
                                : isCodexPlan
                                    ? (t('plan.codexOAuthLabel') || 'OAuth JSON')
                                    : isConsoleTokenPlan
                                        ? (t('plan.consoleTokenLabel') || '控制台 Token')
                                        : (t('plan.apiKeyLabel') || 'API Key')}
                        </label>
                        {category === 'sensenova_plan' && senseNovaAuthMode === 'account' ? (
                            <div className="space-y-2">
                                <label className="text-sm font-medium">
                                    {t('plan.senseNovaUsernameLabel') || '登录账号'}
                                </label>
                                <Input
                                    placeholder={t('plan.senseNovaUsernamePlaceholder') || 'platform.sensenova.cn 控制台登录账号（手机号/用户名）'}
                                    value={loginUsername}
                                    onChange={(e) => setLoginUsername(e.target.value)}
                                />
                                <label className="text-sm font-medium">
                                    {t('plan.senseNovaPasswordLabel') || '登录密码'}
                                </label>
                                <Input
                                    type="password"
                                    placeholder={t('plan.senseNovaPasswordPlaceholder') || '控制台登录密码'}
                                    value={loginPassword}
                                    onChange={(e) => setLoginPassword(e.target.value)}
                                />
                                <p className="text-[11px] leading-tight text-emerald-600">
                                    {t('plan.senseNovaAccountHint') || '系统自动完成登录并续期控制台 Token（约 3 小时有效期），全程无需手动更换。账号密码 AES 加密存储，仅用于自动登录。'}
                                </p>
                            </div>
                        ) : (
                        <Input
                            type="password"
                            placeholder={isMiMoPlan
                                ? (t('plan.mimoServiceTokenPlaceholder') || '粘贴 platform.xiaomimimo.com 的完整 Cookie')
                                : isCodexPlan
                                    ? (t('plan.codexOAuthPlaceholder') || '粘贴 OAuth JSON 凭据')
                                    : isConsoleTokenPlan
                                        ? (isVolcenginePlan
                                            ? (t('plan.volcengineCredentialPlaceholder') || 'Cookie值|||x-csrf-token值')
                                            : isVolcengineAKSK
                                                ? (t('plan.volcengineAKSKPlaceholder') || 'AccessKey ID|||Secret Access Key')
                                                : category === 'sensenova_plan'
                                                    ? (t('plan.sensenovaTokenPlaceholder') || '粘贴控制台 Bearer Token 值')
                                                    : category === 'bailian_plan'
                                                        ? (t('plan.bailianTokenPlaceholder') || '粘贴控制台完整 Cookie 值')
                                                        : (t('plan.oasisTokenPlaceholder') || '粘贴控制台 Cookie 中的 Oasis-Token 值'))
                                    : (t('plan.apiKeyPlaceholder') || '请输入 API Key')}
                            value={apiKey}
                            onChange={(e) => setApiKey(e.target.value)}
                        />
                        )}
                        {isConsoleTokenPlan && !(category === 'sensenova_plan' && senseNovaAuthMode === 'account') && (
                            <p className="text-[11px] leading-tight text-amber-500">
                                    {isVolcenginePlan
                                        ? (t('plan.volcengineCredentialHint') || '会话过期后需重新获取 Cookie 和 x-csrf-token。')
                                        : isVolcengineAKSK
                                            ? (t('plan.volcengineAKSKHint') || 'AK/SK 长期有效，无需频繁更新。确认账号有 Ark 用量查询(OpenAPI)权限。')
                                            : category === 'bailian_plan'
                                                ? (t('plan.bailianTokenHint') || '会话过期后需重新获取控制台 Cookie。')
                                                : category === 'sensenova_plan'
                                                    ? (t('plan.sensenovaTokenHint') || 'Token 有效期约 3 小时，过期后需重新获取。')
                                                    : (t('plan.oasisTokenHint') || 'Oasis-Token 有效期约 30 分钟，过期后需重新获取。')}
                            </p>
                        )}
                        {isDeepSeek && (
                            <div className="space-y-2 pt-2 border-t border-border/40">
                                <label className="text-sm font-medium">
                                    {t('plan.deepSeekAccountLabel') || '控制台账号（可选，用于官方用量统计）'}
                                    <Hint text={t('plan.deepSeekAccountHint') || '配置后系统自动登录控制台，把卡片统计切换为官方 token 用量（覆盖账号下所有 API key 的调用，比本地转发统计更准确）。账号密码 AES 加密存储；不填则继续使用本地统计。'} />
                                </label>
                                <Input
                                    type="text"
                                    placeholder={t('plan.deepSeekAccountPlaceholder') || 'platform.deepseek.com 登录手机号'}
                                    value={loginUsername}
                                    onChange={(e) => setLoginUsername(e.target.value)}
                                />
                                <Input
                                    type="password"
                                    placeholder={t('plan.deepSeekPasswordPlaceholder') || '控制台登录密码'}
                                    value={loginPassword}
                                    onChange={(e) => setLoginPassword(e.target.value)}
                                />
                            </div>
                        )}
                        {isCodexPlan && (
                            <p className="text-[11px] leading-tight text-amber-500">
                                {t('plan.codexOAuthHint') || 'access_token 有效期较短，过期后需重新获取。'}
                            </p>
                        )}
                    </div>
                    {isZhipuTeam && (
                        <div className="space-y-2">
                            <label className="text-sm font-medium">
                                {t('plan.zhipuTeamOrgIdLabel') || '组织 ID'}
                                <Hint text={t('plan.zhipuTeamEditHint') || '留空则清空组织/项目 ID；填写新值将一并更新。'} />
                            </label>
                            <Input
                                placeholder={t('plan.zhipuTeamOrgIdPlaceholder') || 'bigmodel-organization 请求头值'}
                                value={teamOrgId}
                                onChange={(e) => setTeamOrgId(e.target.value)}
                            />
                            <label className="text-sm font-medium mt-2 block">
                                {t('plan.zhipuTeamProjectIdLabel') || '项目 ID'}
                            </label>
                            <Input
                                placeholder={t('plan.zhipuTeamProjectIdPlaceholder') || 'bigmodel-project 请求头值'}
                                value={teamProjectId}
                                onChange={(e) => setTeamProjectId(e.target.value)}
                            />
                        </div>
                    )}

                    {supportsForwardApiKey && (
                        <div className="space-y-2">
                            <label className="text-sm font-medium">
                                {t('plan.forwardApiKeyLabel') || 'API Key（可选）'}
                                <Hint text={t('plan.forwardApiKeyEditHint') || '留空表示不更换转发凭据；填写新值将同步更新关联渠道的 key。'} />
                            </label>
                            <Input
                                type="password"
                                placeholder={t('plan.forwardApiKeyPlaceholderOptional') || '留空保持不变，填写则更新转发渠道 key'}
                                value={forwardApiKey}
                                onChange={(e) => setForwardApiKey(e.target.value)}
                            />
                        </div>
                    )}

                    <Button
                        className="w-full rounded-xl"
                        onClick={handleSubmit}
                        disabled={(useAccountLogin ? (!loginUsername.trim() || !loginPassword.trim()) : !apiKey.trim()) || updateMutation.isPending}
                    >
                        {updateMutation.isPending ? (
                            <>
                                <Loader2 className="size-4 animate-spin mr-2" />
                                {t('plan.querying') || '查询中...'}
                            </>
                        ) : (
                            t('plan.update') || '更新并查询'
                        )}
                    </Button>
                </div>
            </DialogContent>
        </Dialog>
    );
}
