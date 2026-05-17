'use client';

import { useState } from 'react';
import { Activity, Coins, Database, Gauge, HardDrive, Layers3, SlidersHorizontal } from 'lucide-react';
import { useTranslations } from 'next-intl';
import type { OpsCacheStatus, OpsProviderPromptCacheProviderItem, OpsProviderPromptCacheSummary } from '@/api/endpoints/ops';
import { useOpsCacheStatus } from '@/api/endpoints/ops';
import { useNavStore } from '@/components/modules/navbar';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { MetricCard, QueryState, StatusBadge, formatPercent, formatUnixTime } from '@/components/modules/analytics/shared';

type CacheView = 'semantic' | 'providerPrompt';
type CacheTranslations = (key: string) => string;

function formatCount(n: number | undefined) {
    const value = n ?? 0;
    if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
    if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`;
    return String(value);
}

function formatCurrency(value: number | undefined) {
    return (value ?? 0).toFixed(4);
}

function ViewSwitcher({
    value,
    onChange,
    semanticLabel,
    providerLabel,
}: {
    value: CacheView;
    onChange: (value: CacheView) => void;
    semanticLabel: string;
    providerLabel: string;
}) {
    const items: Array<{ key: CacheView; label: string }> = [
        { key: 'providerPrompt', label: providerLabel },
        { key: 'semantic', label: semanticLabel },
    ];

    return (
        <div className="inline-flex rounded-xl border border-border/50 bg-muted/30 p-1">
            {items.map((item) => (
                <button
                    key={item.key}
                    type="button"
                    onClick={() => onChange(item.key)}
                    className={cn(
                        'rounded-lg px-3 py-1.5 text-sm font-medium transition-colors',
                        value === item.key
                            ? 'bg-card text-foreground shadow-sm'
                            : 'text-muted-foreground hover:text-foreground',
                    )}
                >
                    {item.label}
                </button>
            ))}
        </div>
    );
}

function SemanticCacheView({ data, t }: { data: OpsCacheStatus; t: CacheTranslations }) {
    return (
        <div className="space-y-4">
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
                <MetricCard
                    title={t('cache.metrics.hitRate')}
                    value={formatPercent(data.hit_rate).formatted.value}
                    unit={formatPercent(data.hit_rate).formatted.unit}
                    icon={Gauge}
                    accentClassName="bg-emerald-500/10 text-emerald-600"
                />
                <MetricCard
                    title={t('cache.metrics.currentEntries')}
                    value={data.current_entries}
                    helper={`${data.current_entries} / ${data.max_entries}`}
                    icon={Database}
                />
                <MetricCard
                    title={t('cache.metrics.ttlSeconds')}
                    value={data.ttl_seconds}
                    unit="s"
                    icon={Activity}
                />
                <MetricCard
                    title={t('cache.metrics.threshold')}
                    value={data.threshold}
                    unit="%"
                    icon={SlidersHorizontal}
                    accentClassName="bg-chart-4/10 text-chart-4"
                />
            </div>

            <article className="rounded-xl border border-border/60 bg-card p-4">
                <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
                    <div className="space-y-3">
                        <div className="flex flex-wrap items-center gap-2">
                            <StatusBadge
                                label={data.enabled ? t('cache.status.configuredOn') : t('cache.status.configuredOff')}
                                tone={data.enabled ? 'success' : 'neutral'}
                            />
                            <StatusBadge
                                label={data.runtime_enabled ? t('cache.status.runtimeOn') : t('cache.status.runtimeOff')}
                                tone={data.runtime_enabled ? 'success' : (data.enabled ? 'warning' : 'neutral')}
                            />
                        </div>
                        <p className="text-sm leading-6 text-muted-foreground">
                            {data.runtime_enabled ? t('cache.status.runtimeHint') : t('cache.status.runtimeMissing')}
                        </p>
                    </div>

                    <div className="grid grid-cols-2 gap-3 lg:min-w-[320px]">
                        <div className="rounded-lg border border-border/40 bg-card p-3">
                            <div className="text-xs text-muted-foreground">{t('cache.detail.hits')}</div>
                            <div className="mt-2 text-xl font-semibold">{data.hits}</div>
                        </div>
                        <div className="rounded-lg border border-border/40 bg-card p-3">
                            <div className="text-xs text-muted-foreground">{t('cache.detail.misses')}</div>
                            <div className="mt-2 text-xl font-semibold">{data.misses}</div>
                        </div>
                        <div className="rounded-lg border border-border/40 bg-card p-3">
                            <div className="text-xs text-muted-foreground">{t('cache.detail.maxEntries')}</div>
                            <div className="mt-2 text-sm font-semibold">{data.max_entries}</div>
                        </div>
                        <div className="rounded-lg border border-border/40 bg-card p-3">
                            <div className="text-xs text-muted-foreground">{t('cache.detail.usageRate')}</div>
                            <div className="mt-2 text-sm font-semibold">
                                {formatPercent(data.usage_rate).formatted.value}
                                {formatPercent(data.usage_rate).formatted.unit}
                            </div>
                        </div>
                    </div>
                </div>
            </article>
        </div>
    );
}

function ProviderPromptCacheRow({
    provider,
}: {
    provider: OpsProviderPromptCacheProviderItem;
}) {
    return (
        <tr className="border-b border-border/40 align-top last:border-0">
            <td className="py-3 pr-3">
                <div className="text-sm font-semibold">{provider.channel_name}</div>
            </td>
            <td className="px-3 py-3 text-right text-sm tabular-nums">{formatCount(provider.request_count)}</td>
            <td className="px-3 py-3 text-right text-sm tabular-nums">
                {formatPercent(provider.cache_rate).formatted.value}
                {formatPercent(provider.cache_rate).formatted.unit}
            </td>
            <td className="px-3 py-3 text-right text-sm tabular-nums">{formatCount(provider.cache_read_tokens)}</td>
            <td className="px-3 py-3 text-right text-sm tabular-nums">{formatCount(provider.cache_write_tokens)}</td>
            <td className="pl-3 py-3 text-right text-sm font-semibold tabular-nums">
                ${formatCurrency(provider.estimated_cost_saved)}
            </td>
        </tr>
    );
}

function ProviderPromptCacheView({
    data,
    t,
}: {
    data: OpsProviderPromptCacheSummary;
    t: CacheTranslations;
}) {
    const trend = data.trend ?? [];
    const maxReadTokens = Math.max(...trend.map((item) => item.cache_read_tokens), 1);

    return (
        <div className="space-y-4">
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-5">
                <MetricCard
                    title={t('cache.providerPrompt.metrics.cacheRate')}
                    value={formatPercent(data.cache_rate).formatted.value}
                    unit={formatPercent(data.cache_rate).formatted.unit}
                    icon={Gauge}
                    accentClassName="bg-emerald-500/10 text-emerald-600"
                />
                <MetricCard
                    title={t('cache.providerPrompt.metrics.cacheReuseRatio')}
                    value={formatPercent(data.cache_reuse_ratio).formatted.value}
                    unit={formatPercent(data.cache_reuse_ratio).formatted.unit}
                    icon={Layers3}
                />
                <MetricCard
                    title={t('cache.providerPrompt.metrics.cacheReadTokens')}
                    value={formatCount(data.cache_read_tokens)}
                    icon={HardDrive}
                />
                <MetricCard
                    title={t('cache.providerPrompt.metrics.cacheWriteTokens')}
                    value={formatCount(data.cache_write_tokens)}
                    icon={Database}
                />
                <MetricCard
                    title={t('cache.providerPrompt.metrics.estimatedCostSaved')}
                    value={formatCurrency(data.estimated_cost_saved)}
                    unit="$"
                    icon={Coins}
                    accentClassName="bg-chart-4/10 text-chart-4"
                />
            </div>

            <article className="rounded-xl border border-border/60 bg-card p-4">
                <div className="grid gap-6 lg:grid-cols-[1.1fr_0.9fr]">
                    <div className="space-y-3">
                        <div>
                            <h4 className="text-sm font-semibold">{t('cache.providerPrompt.providers.title')}</h4>
                            <p className="mt-1 text-sm leading-6 text-muted-foreground">
                                {t('cache.providerPrompt.providers.description')}
                            </p>
                        </div>
                        {data.providers.length === 0 ? (
                            <div className="rounded-lg border border-dashed border-border/40 px-4 py-6 text-center text-sm text-muted-foreground">
                                {t('cache.providerPrompt.providers.empty')}
                            </div>
                        ) : (
                            <div className="overflow-x-auto rounded-xl border border-border/60">
                                <table className="w-full min-w-[720px] text-left">
                                    <thead>
                                        <tr className="border-b border-border/40 bg-muted/30">
                                            <th className="px-3 py-2.5 text-xs font-medium text-muted-foreground">
                                                {t('cache.providerPrompt.providers.columns.name')}
                                            </th>
                                            <th className="px-3 py-2.5 text-right text-xs font-medium text-muted-foreground">
                                                {t('cache.providerPrompt.providers.columns.requests')}
                                            </th>
                                            <th className="px-3 py-2.5 text-right text-xs font-medium text-muted-foreground">
                                                {t('cache.providerPrompt.providers.columns.cacheRate')}
                                            </th>
                                            <th className="px-3 py-2.5 text-right text-xs font-medium text-muted-foreground">
                                                {t('cache.providerPrompt.providers.columns.cacheReadTokens')}
                                            </th>
                                            <th className="px-3 py-2.5 text-right text-xs font-medium text-muted-foreground">
                                                {t('cache.providerPrompt.providers.columns.cacheWriteTokens')}
                                            </th>
                                            <th className="px-3 py-2.5 text-right text-xs font-medium text-muted-foreground">
                                                {t('cache.providerPrompt.providers.columns.estimatedCostSaved')}
                                            </th>
                                        </tr>
                                    </thead>
                                        <tbody>
                                            {data.providers.map((provider) => (
                                                <ProviderPromptCacheRow key={provider.channel_id} provider={provider} />
                                            ))}
                                        </tbody>
                                    </table>
                            </div>
                        )}
                    </div>

                    <div className="space-y-3">
                        <div>
                            <h4 className="text-sm font-semibold">{t('cache.providerPrompt.trend.title')}</h4>
                            <p className="mt-1 text-sm leading-6 text-muted-foreground">
                                {t('cache.providerPrompt.trend.description')}
                            </p>
                        </div>
                        <div className="rounded-xl border border-border/60 bg-card p-4">
                            <div className="flex h-40 items-end gap-2">
                                {trend.map((point) => {
                                    const height = `${Math.max(8, (point.cache_read_tokens / maxReadTokens) * 100)}%`;
                                    return (
                                        <div key={point.timestamp} className="flex flex-1 flex-col items-center gap-2">
                                            <div
                                                className="w-full rounded-t-md bg-primary/20"
                                                style={{ height }}
                                                title={`${formatUnixTime(point.timestamp)} | ${formatCount(point.cache_read_tokens)} read`}
                                            />
                                            <div className="text-[10px] text-muted-foreground">
                                                {formatUnixTime(point.timestamp)}
                                            </div>
                                        </div>
                                    );
                                })}
                            </div>
                        </div>
                    </div>
                </div>
            </article>
        </div>
    );
}

export function Cache() {
    const t = useTranslations('ops');
    const { setActiveItem } = useNavStore();
    const { data, isLoading, error } = useOpsCacheStatus();
    const [view, setView] = useState<CacheView>('providerPrompt');

    return (
        <section className="rounded-xl border border-border/35 bg-card p-5 text-card-foreground">
            <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                <div className="space-y-1">
                    <h3 className="text-base font-semibold">{t('tabs.cache')}</h3>
                    <p className="text-sm leading-6 text-muted-foreground">
                        {view === 'semantic' ? t('cache.description') : t('cache.providerPrompt.description')}
                    </p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                    <ViewSwitcher
                        value={view}
                        onChange={setView}
                        semanticLabel={t('cache.views.semantic')}
                        providerLabel={t('cache.views.providerPrompt')}
                    />
                    <Button
                        variant="outline"
                        size="sm"
                        className="rounded-xl"
                        onClick={() => setActiveItem('setting')}
                    >
                        {t('actions.openSettings')}
                    </Button>
                </div>
            </div>

            <QueryState
                loading={isLoading}
                error={error}
                empty={!data}
                emptyLabel={t('states.loading')}
            >
                {data ? (
                    view === 'semantic' ? (
                        <SemanticCacheView data={data} t={t} />
                    ) : (
                        <ProviderPromptCacheView data={data.provider_prompt_cache} t={t} />
                    )
                ) : null}
            </QueryState>
        </section>
    );
}
