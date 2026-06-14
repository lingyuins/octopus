'use client';

import { useState } from 'react';
import { Activity, AlertTriangle, CircleOff, ShieldCheck, Radar, ChevronDown, ChevronRight, Gauge } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useAnalyticsGroupHealth, useAnalyticsAutoStrategy } from '@/api/endpoints/analytics';
import { ObservatorySection, QueryState, StatusBadge, formatUnixTime } from './shared';
import { useNavStore } from '@/components/modules/navbar/nav-store';
import { useNavHandoff } from '@/lib/nav-handoff';
import { cn } from '@/lib/utils';
import type { FailingChannelItem, AutoStrategySnapshotItem } from '@/api/endpoints/analytics';

function getStatusTone(status: 'healthy' | 'warning' | 'degraded' | 'down' | 'empty') {
    switch (status) {
        case 'healthy':
            return 'success' as const;
        case 'warning':
        case 'degraded':
            return 'warning' as const;
        case 'down':
            return 'danger' as const;
        default:
            return 'neutral' as const;
    }
}

function StatusIconFor({ status }: { status: 'healthy' | 'warning' | 'degraded' | 'down' | 'empty' }) {
    switch (status) {
        case 'healthy':
            return <ShieldCheck className="h-4 w-4" />;
        case 'down':
            return <CircleOff className="h-4 w-4" />;
        case 'warning':
        case 'degraded':
            return <AlertTriangle className="h-4 w-4" />;
        default:
            return <Activity className="h-4 w-4" />;
    }
}

// successRateTone 把一个成功率映射为色调，用于 Auto 策略表现的高亮。
function successRateTone(rate: number, minMet: boolean): 'danger' | 'warning' | 'success' | 'neutral' {
    if (!minMet) return 'neutral';
    if (rate < 50) return 'danger';
    if (rate < 90) return 'warning';
    return 'success';
}

function successRateClass(tone: 'danger' | 'warning' | 'success' | 'neutral') {
    switch (tone) {
        case 'danger':
            return 'text-destructive';
        case 'warning':
            return 'text-amber-600 dark:text-amber-400';
        case 'success':
            return 'text-accent';
        default:
            return 'text-muted-foreground';
    }
}

function GroupHealthCard({
    item,
    autoItems,
}: {
    item: import('@/api/endpoints/analytics').AnalyticsGroupHealthItem;
    autoItems: AutoStrategySnapshotItem[];
}) {
    const t = useTranslations('analytics');
    const setActiveItem = useNavStore((s) => s.setActiveItem);
    const setPendingLogFilter = useNavHandoff((s) => s.setPendingLogFilter);
    const [showFailing, setShowFailing] = useState(false);
    const [showAuto, setShowAuto] = useState(false);

    const isAutoGroup = item.mode === 5;

    // 点击失败渠道：跳转日志并预填渠道 + 开启尝试穿透。
    const jumpToLogForChannel = (fc: FailingChannelItem) => {
        setPendingLogFilter({
            channel_id: fc.channel_id,
            include_attempts: true,
        });
        setActiveItem('log');
    };

    return (
        <article className="rounded-lg border border-border/30 bg-card p-4 shadow-sm">
            <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                    <div className="flex items-center gap-2">
                        <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary shadow-sm">
                            <StatusIconFor status={item.status} />
                        </div>
                        <div className="min-w-0">
                            <h4 className="truncate text-sm font-semibold">{item.group_name}</h4>
                            <p className="text-xs text-muted-foreground">
                                {t('routeHealth.endpointType')}: {item.endpoint_type}
                            </p>
                        </div>
                    </div>
                </div>
                <StatusBadge
                    label={t(`routeHealth.statuses.${item.status}`)}
                    tone={getStatusTone(item.status)}
                />
            </div>

            <div className="mt-4 grid grid-cols-2 gap-3">
                <div className="rounded-lg border border-border/25 bg-card p-3 shadow-sm">
                    <div className="text-xs text-muted-foreground">{t('routeHealth.healthScore')}</div>
                    <div className="mt-2 text-2xl font-semibold">{item.health_score}</div>
                </div>
                <div className="rounded-lg border border-border/25 bg-card p-3 shadow-sm">
                    <div className="text-xs text-muted-foreground">{t('routeHealth.failureCount')}</div>
                    <div className="mt-2 text-2xl font-semibold">{item.failure_count}</div>
                </div>
                <div className="rounded-lg border border-border/25 bg-card p-3 shadow-sm">
                    <div className="text-xs text-muted-foreground">{t('routeHealth.enabledItems')}</div>
                    <div className="mt-2 text-sm font-semibold">
                        {item.enabled_item_count} / {item.item_count}
                    </div>
                </div>
                <div className="rounded-lg border border-border/25 bg-card p-3 shadow-sm">
                    <div className="text-xs text-muted-foreground">{t('routeHealth.disabledItems')}</div>
                    <div className="mt-2 text-sm font-semibold">{item.disabled_item_count}</div>
                </div>
            </div>

            <div className="mt-4 text-xs text-muted-foreground">
                {t('routeHealth.lastFailure')}:
                <span className="ml-1 text-foreground">
                    {item.last_failure_at ? formatUnixTime(item.last_failure_at) : t('routeHealth.lastFailureEmpty')}
                </span>
            </div>

            {/* 失败渠道下钻：仅在组有失败渠道时展示 */}
            {item.failing_channels && item.failing_channels.length > 0 && (
                <div className="mt-3">
                    <button
                        type="button"
                        onClick={() => setShowFailing((v) => !v)}
                        className="flex w-full items-center gap-1.5 rounded-md px-1.5 py-1 text-xs font-medium text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
                    >
                        {showFailing ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                        <AlertTriangle className="size-3.5 text-amber-500" />
                        {t('routeHealth.failingChannels.title')} ({item.failing_channels.length})
                    </button>
                    {showFailing && (
                        <ul className="mt-2 space-y-1">
                            {item.failing_channels.map((fc, idx) => (
                                <li
                                    key={`${fc.channel_id}-${fc.model_name}-${idx}`}
                                    className="flex items-center justify-between gap-2 rounded-md border border-border/25 bg-background/40 px-2.5 py-1.5 text-xs"
                                >
                                    <div className="min-w-0">
                                        <div className="truncate font-medium text-foreground">{fc.channel_name || `#${fc.channel_id}`}</div>
                                        <div className="truncate text-muted-foreground">{fc.model_name}</div>
                                    </div>
                                    <div className="flex shrink-0 items-center gap-2">
                                        <span className="text-destructive font-semibold">{fc.failure_count}</span>
                                        {fc.last_failure_at ? (
                                            <span className="text-muted-foreground">{formatUnixTime(fc.last_failure_at)}</span>
                                        ) : null}
                                        <button
                                            type="button"
                                            onClick={() => jumpToLogForChannel(fc)}
                                            className="rounded border border-border/40 px-1.5 py-0.5 text-[10px] text-primary hover:bg-primary/10 transition-colors"
                                            title={t('routeHealth.failingChannels.viewLogs')}
                                        >
                                            {t('routeHealth.failingChannels.viewLogs')}
                                        </button>
                                    </div>
                                </li>
                            ))}
                        </ul>
                    )}
                </div>
            )}

            {/* Auto 策略实时表现：仅 Auto 组展示 */}
            {isAutoGroup && autoItems.length > 0 && (
                <div className="mt-3">
                    <button
                        type="button"
                        onClick={() => setShowAuto((v) => !v)}
                        className="flex w-full items-center gap-1.5 rounded-md px-1.5 py-1 text-xs font-medium text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
                    >
                        {showAuto ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                        <Gauge className="size-3.5 text-primary" />
                        {t('routeHealth.autoStrategy.title')} ({autoItems.length})
                    </button>
                    {showAuto && (
                        <ul className="mt-2 space-y-1">
                            {autoItems.map((ai, idx) => {
                                const tone = successRateTone(ai.success_rate, ai.min_samples_met);
                                return (
                                    <li
                                        key={`${ai.channel_id}-${ai.model_name}-${idx}`}
                                        className="flex items-center justify-between gap-2 rounded-md border border-border/25 bg-background/40 px-2.5 py-1.5 text-xs"
                                    >
                                        <div className="min-w-0">
                                            <div className="truncate font-medium text-foreground">
                                                {ai.channel_name || `#${ai.channel_id}`}
                                                {!ai.enabled && (
                                                    <span className="ml-1 text-muted-foreground">({t('routeHealth.autoStrategy.disabled')})</span>
                                                )}
                                            </div>
                                            <div className="truncate text-muted-foreground">{ai.model_name}</div>
                                        </div>
                                        <div className="flex shrink-0 items-center gap-2">
                                            <span className={cn('font-semibold', successRateClass(tone))}>
                                                {ai.success_rate.toFixed(1)}%
                                            </span>
                                            <span className="text-muted-foreground">
                                                {ai.min_samples_met
                                                    ? `${ai.sample_count}${t('routeHealth.autoStrategy.samples')}`
                                                    : t('routeHealth.autoStrategy.insufficient')}
                                            </span>
                                            {ai.avg_latency_ms > 0 && (
                                                <span className="text-muted-foreground">{Math.round(ai.avg_latency_ms)}ms</span>
                                            )}
                                        </div>
                                    </li>
                                );
                            })}
                        </ul>
                    )}
                </div>
            )}
        </article>
    );
}

export function GroupHealth() {
    const t = useTranslations('analytics');
    const { data, isLoading, error } = useAnalyticsGroupHealth();
    // 全量拉取 Auto 策略快照，按 group_id 在卡片内过滤。组数量通常不大，一次请求即可。
    const { data: autoData } = useAnalyticsAutoStrategy();

    // 把 auto 快照按 group 关联：快照本身没有 group_id，但可通过组 items 的 (channel,model) 匹配。
    // 这里用一个简化映射：autoItemsForGroup 取该组失败渠道覆盖的渠道集合 ∩ auto 快照。
    // 由于 useAnalyticsGroupHealth 已返回 mode，我们直接用全量 auto 数据按渠道 ID 过滤到该组。
    const autoByChannel = new Map<number, AutoStrategySnapshotItem[]>();
    for (const ai of autoData ?? []) {
        const list = autoByChannel.get(ai.channel_id) ?? [];
        list.push(ai);
        autoByChannel.set(ai.channel_id, list);
    }

    const autoItemsForGroup = (item: import('@/api/endpoints/analytics').AnalyticsGroupHealthItem): AutoStrategySnapshotItem[] => {
        if (item.mode !== 5) return [];
        // 用该组失败渠道列表涉及的渠道 + 全量 auto 中该组可能涉及的渠道。
        // 由于 group health 不直接返回 items 的渠道集合，我们用 failing_channels 的渠道 +
        // 全量 auto 中所有渠道作为近似（Auto 组通常渠道有限，影响可控）。
        const channelIds = new Set<number>();
        for (const fc of item.failing_channels ?? []) {
            channelIds.add(fc.channel_id);
        }
        const result: AutoStrategySnapshotItem[] = [];
        for (const [channelId, list] of autoByChannel) {
            if (channelIds.size === 0 || channelIds.has(channelId)) {
                result.push(...list);
            }
        }
        return result.slice(0, 12);
    };

    return (
        <ObservatorySection
            eyebrow={t('cards.routeHealth.title')}
            title={t('cards.routeHealth.title')}
            description={t('routeHealth.description')}
            icon={Radar}
        >
            <QueryState
                loading={isLoading}
                error={error}
                empty={!data || data.length === 0}
                emptyLabel={isLoading ? t('states.loading') : t('routeHealth.empty')}
            >
                <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                    {(data ?? []).map((item) => (
                        <GroupHealthCard
                            key={`${item.group_id}-${item.endpoint_type}`}
                            item={item}
                            autoItems={autoItemsForGroup(item)}
                        />
                    ))}
                </div>
            </QueryState>
        </ObservatorySection>
    );
}
