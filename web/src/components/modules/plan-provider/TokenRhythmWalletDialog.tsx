'use client';

import { Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { cn } from '@/lib/utils';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { Hint } from '@/components/ui/hint';
import {
    useTokenRhythmWallet,
    type PlanProvider,
    type TokenRhythmWalletCredit,
} from '@/api/endpoints/plan-provider';

// 金额格式：与基元律动官网一致 —— 分位向上取整（ceil 到 2 位小数）。
// 依据（官网显示值 vs 上游原始值实测）：
//   220.82454716 → 220.83、66.82454716 → 66.83（四舍五入会得 .82，故为 ceil）
//   −1.17545284 → −1.17（向零取整）、0.019507 → 0.02
// 减 1e-6 抵消浮点乘法误差（如 18*100 可能得 1800.0000000000002 → 否则会多进 1 分）。
const formatMoney = (val: number) => {
    if (!val) return '0.00';
    // 直接对带符号值向上取整（负数向零）：−1.17545284 → −1.17，与官网一致。
    // 不能取绝对值再补负号，那会得到 −1.18。
    const cents = Math.ceil(val * 100 - 1e-6) / 100;
    return cents.toLocaleString(undefined, {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
    });
};

// YYYY/MM/DD（官网资金明细「有效期」列的写法，北京时间）
const formatDate = (val: string | null | undefined) => {
    if (!val) return '';
    const d = new Date(val);
    if (Number.isNaN(d.getTime())) return val;
    return new Intl.DateTimeFormat('zh-CN', {
        timeZone: 'Asia/Shanghai',
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
    })
        .format(d)
        .replaceAll('-', '/');
};

// 明细列宽（桌面端 6 列）：来源 | 到账金额 | 已消费 | 剩余可用 | 状态 | 有效期
const GRID_COLS = 'grid-cols-[minmax(0,1fr)_92px_92px_92px_76px_168px]';

/**
 * TokenRhythmWalletDialog 基元律动「资金明细」弹窗。
 *
 * 数据源与官网「用户中心 → 账户」页（/account/account）的「资金明细」表一致：
 * 后端 /api/v1/plan-provider/wallet/transactions/:id 代理
 * wallet/summary（账户余额）+ wallet/expiring-credits（赠送额度账本：逐笔赠金 + 充值本金行）
 * + usage-summary（累计消费）。
 *
 * ⚠️ 不要改回 wallet/transactions 全量流水口径：那个接口按时间倒序返回**所有**余额变动，
 * 活跃账号首屏 20 条几乎全是「模型调用扣费」，到账记录被埋在数百条之后（实测 635 / 220 条），
 * 过滤 DEBIT 后列表为空（曾长期显示「暂无资金明细」）。赠送额度账本本身就是逐笔到账列表。
 *
 * 口径（与官网账户页一致）：
 *   - 顶部：剩余可用（summary.availableBalanceCny）、累计获赠（账本 cumulativeGiftGrantedCny）、
 *     累计消费（usage-summary.costCny，与卡片「已用额度」同源）
 *   - 明细：来源 / 到账金额 / 已消费 / 剩余可用 / 状态 / 有效期，含已用尽与已到期的历史记录
 *   - 充值本金行仅当 granted_cny > 0 时展示，有效期显示「长期有效」
 */
export function TokenRhythmWalletDialog({
    provider,
    open,
    onOpenChange,
}: {
    provider: PlanProvider | null;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}) {
    const t = useTranslations('hub');
    // 未打开 / 无 provider 时传 null → hook 不发请求
    const { data, isLoading, error } = useTokenRhythmWallet(open && provider ? provider.id : null);

    // 来源标签：已知枚举走 i18n，未知回退上游中文标签（source_label），再回退 source
    const sourceLabel = (credit: TokenRhythmWalletCredit) => {
        switch (credit.source) {
            case 'RECHARGE':
            case 'TOPUP':
                return t('plan.wallet.sourceRecharge') || '充值本金';
            case 'INITIAL_BALANCE':
                return t('plan.wallet.sourceInitialBalance') || '新用户赠送额度';
            case 'IDENTITY_VERIFICATION_REWARD':
                return t('plan.wallet.sourceIdentityReward') || '实名认证奖励';
            case 'INVITE_REWARD':
                return t('plan.wallet.sourceInviteReward') || '邀请奖励';
            default:
                return credit.source_label || credit.source || '-';
        }
    };

    // 状态映射（官网：生效中 / 暂停中 / 已用尽 / 已到期）；未知枚举回退原文
    const statusLabel = (status: string) => {
        switch (status) {
            case 'ACTIVE':
                return t('plan.wallet.statusActive') || '生效中';
            case 'PAUSED':
                return t('plan.wallet.statusPaused') || '暂停中';
            case 'USED_UP':
                return t('plan.wallet.statusUsedUp') || '已用尽';
            case 'EXPIRED':
                return t('plan.wallet.statusExpired') || '已到期';
            default:
                return status || '-';
        }
    };

    const statusTone = (status: string) => {
        switch (status) {
            case 'ACTIVE':
                return 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400';
            case 'PAUSED':
                return 'bg-amber-500/10 text-amber-600 dark:text-amber-400';
            case 'EXPIRED':
                return 'bg-destructive/10 text-destructive';
            default:
                return 'bg-muted text-muted-foreground';
        }
    };

    // 行序与官网一致：充值本金行（仅当有金额）插在最前，其后是逐笔赠金（最新在前）。
    const principal = data?.recharge_principal ?? null;
    const showPrincipal = !!principal && principal.granted_cny > 0;
    const rows: TokenRhythmWalletCredit[] = [
        ...(showPrincipal && principal ? [principal] : []),
        ...(data?.credits ?? []),
    ];
    const totalRows = (data?.total ?? 0) + (showPrincipal ? 1 : 0);

    const validityText = (credit: TokenRhythmWalletCredit) => {
        if (!credit.granted_at) return t('plan.wallet.principalLongTerm') || '长期有效';
        const from = formatDate(credit.granted_at);
        const to = credit.expires_at ? formatDate(credit.expires_at) : '—';
        return `${from} 至 ${to}`;
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-4xl">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-1.5">
                        {t('plan.wallet.title') || '资金明细'}
                        <Hint text={t('plan.wallet.hint')} side="bottom" />
                    </DialogTitle>
                    <DialogDescription>
                        {provider?.name ? `${provider.name} · ` : ''}
                        {t('plan.wallet.desc') || '基元律动账户余额与逐笔赠金'}
                    </DialogDescription>
                </DialogHeader>

                {isLoading ? (
                    <div className="flex items-center justify-center gap-2 py-10 text-sm text-muted-foreground">
                        <Loader2 className="size-4 animate-spin" />
                        {t('plan.wallet.loading') || '加载中…'}
                    </div>
                ) : error ? (
                    <div className="py-8 text-center text-sm text-destructive">
                        {t('plan.wallet.error') || '获取资金明细失败'}
                        <p className="mt-1 break-all text-xs text-muted-foreground">
                            {String((error as Error)?.message || '')}
                        </p>
                    </div>
                ) : (
                    <>
                        {/* 汇总：剩余可用 / 累计获赠 / 累计消费 */}
                        <div className="grid grid-cols-3 gap-3">
                            <div className="rounded-lg bg-muted/50 p-3">
                                <p className="mb-1 text-xs text-muted-foreground">
                                    {t('plan.wallet.balanceAvailable') || '剩余可用'}
                                </p>
                                <p className="text-lg font-bold tabular-nums">
                                    {formatMoney(data?.available_balance_cny ?? 0)}
                                </p>
                            </div>
                            <div className="rounded-lg bg-muted/50 p-3">
                                <p className="mb-1 text-xs text-muted-foreground">
                                    {t('plan.wallet.totalReceived') || '累计获赠'}
                                </p>
                                <p className="text-lg font-bold tabular-nums text-emerald-600">
                                    +{formatMoney(data?.total_received_cny ?? 0)}
                                </p>
                            </div>
                            <div className="rounded-lg bg-muted/50 p-3">
                                <p className="mb-1 text-xs text-muted-foreground">
                                    {t('plan.wallet.totalConsumed') || '累计消费'}
                                </p>
                                <p className="text-lg font-bold tabular-nums text-destructive">
                                    -{formatMoney(data?.total_consumed_cny ?? 0)}
                                </p>
                            </div>
                        </div>

                        {/* 明细表：桌面 6 列 / 移动端堆叠 */}
                        <div className="overflow-hidden rounded-lg border border-border">
                            <div
                                className={cn(
                                    'hidden border-b border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground sm:grid sm:gap-2',
                                    GRID_COLS,
                                )}
                            >
                                <span>{t('plan.wallet.colSource') || '来源'}</span>
                                <span className="text-right">{t('plan.wallet.colGranted') || '到账金额'}</span>
                                <span className="text-right">{t('plan.wallet.colConsumed') || '已消费'}</span>
                                <span className="text-right">{t('plan.wallet.colRemaining') || '剩余可用'}</span>
                                <span className="text-center">{t('plan.wallet.colStatus') || '状态'}</span>
                                <span className="text-right">{t('plan.wallet.colValidity') || '有效期'}</span>
                            </div>
                            <div className="max-h-[46vh] divide-y divide-border overflow-y-auto">
                                {rows.length === 0 ? (
                                    <p className="py-8 text-center text-sm text-muted-foreground">
                                        {t('plan.wallet.empty') || '暂无资金明细'}
                                    </p>
                                ) : (
                                    rows.map((credit) => (
                                        <div key={credit.id} className="px-3 py-2 hover:bg-muted/30">
                                            <div
                                                className={cn(
                                                    'hidden items-center gap-2 text-xs sm:grid',
                                                    GRID_COLS,
                                                )}
                                            >
                                                <span className="flex min-w-0 items-center gap-1.5">
                                                    <span
                                                        className={cn(
                                                            'size-1.5 shrink-0 rounded-full',
                                                            credit.is_principal ? 'bg-primary' : 'bg-emerald-500',
                                                        )}
                                                    />
                                                    <span className="truncate">{sourceLabel(credit)}</span>
                                                </span>
                                                <span className="text-right font-medium tabular-nums text-emerald-600">
                                                    +{formatMoney(credit.granted_cny)}
                                                </span>
                                                <span className="text-right tabular-nums text-destructive">
                                                    {credit.consumed_cny > 0
                                                        ? `-${formatMoney(credit.consumed_cny)}`
                                                        : '—'}
                                                </span>
                                                <span className="text-right font-medium tabular-nums">
                                                    {formatMoney(credit.remaining_cny)}
                                                </span>
                                                <span className="text-center">
                                                    <span
                                                        className={cn(
                                                            'inline-block rounded px-1.5 py-0.5 text-[11px] leading-tight',
                                                            statusTone(credit.status),
                                                        )}
                                                    >
                                                        {statusLabel(credit.status)}
                                                    </span>
                                                </span>
                                                <span className="text-right tabular-nums text-muted-foreground">
                                                    {validityText(credit)}
                                                </span>
                                            </div>

                                            {/* 移动端：堆叠卡片 */}
                                            <div className="space-y-1.5 sm:hidden">
                                                <div className="flex items-center justify-between gap-2">
                                                    <span className="flex min-w-0 items-center gap-1.5">
                                                        <span
                                                            className={cn(
                                                                'size-1.5 shrink-0 rounded-full',
                                                                credit.is_principal
                                                                    ? 'bg-primary'
                                                                    : 'bg-emerald-500',
                                                            )}
                                                        />
                                                        <span className="truncate text-sm font-medium">
                                                            {sourceLabel(credit)}
                                                        </span>
                                                    </span>
                                                    <span
                                                        className={cn(
                                                            'shrink-0 rounded px-1.5 py-0.5 text-[11px] leading-tight',
                                                            statusTone(credit.status),
                                                        )}
                                                    >
                                                        {statusLabel(credit.status)}
                                                    </span>
                                                </div>
                                                <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs tabular-nums text-muted-foreground">
                                                    <span className="font-medium text-emerald-600">
                                                        {t('plan.wallet.colGranted') || '到账金额'} +
                                                        {formatMoney(credit.granted_cny)}
                                                    </span>
                                                    <span className="text-destructive">
                                                        {t('plan.wallet.colConsumed') || '已消费'}{' '}
                                                        {credit.consumed_cny > 0
                                                            ? `-${formatMoney(credit.consumed_cny)}`
                                                            : '—'}
                                                    </span>
                                                    <span className="text-foreground/80">
                                                        {t('plan.wallet.colRemaining') || '剩余可用'}{' '}
                                                        {formatMoney(credit.remaining_cny)}
                                                    </span>
                                                </div>
                                                <div className="text-xs text-muted-foreground">
                                                    {validityText(credit)}
                                                </div>
                                            </div>
                                        </div>
                                    ))
                                )}
                            </div>
                        </div>

                        <p className="text-right text-xs text-muted-foreground">
                            {data && data.total + (showPrincipal ? 1 : 0) > rows.length
                                ? t('plan.wallet.countLimited', { count: totalRows, shown: rows.length })
                                : t('plan.wallet.count', { count: totalRows })}
                        </p>
                    </>
                )}
            </DialogContent>
        </Dialog>
    );
}
