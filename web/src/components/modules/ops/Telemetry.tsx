'use client';

import {
    Activity,
    ArrowRight,
    Braces,
    Cpu,
    Database,
    HardDrive,
    Timer,
    TrendingUp,
    Zap,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import type {
    OpsTelemetryHeroMetrics,
    OpsTelemetryTrendPoint,
    OpsTelemetryDatabaseHealth,
    OpsTelemetrySessionQuotaActivity,
    OpsTelemetryPromptCache,
    OpsTelemetryProviderItem,
    OpsTelemetryDrilldownShortcut,
} from '@/api/endpoints/ops';
import { useOpsTelemetrySummary } from '@/api/endpoints/ops';
import { QueryState, StatusBadge } from '@/components/modules/analytics/shared';
import { formatTelemetryPercent, getTelemetryErrorRateTone } from './telemetry-format';

function formatUptime(seconds: number): string {
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
    if (seconds < 86400) return `${Math.floor(seconds / 3600)}h`;
    const d = Math.floor(seconds / 86400);
    const h = Math.floor((seconds % 86400) / 3600);
    return `${d}d ${h}h`;
}

function formatCount(n: number): string {
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
    if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
    return String(n);
}

function HeroMetricCard({
    icon: Icon,
    label,
    value,
    unit,
    tone,
}: {
    icon: typeof Cpu;
    label: string;
    value: string;
    unit?: string;
    tone?: 'success' | 'warning' | 'danger';
}) {
    const toneClass = tone === 'success' ? 'text-emerald-500' : tone === 'warning' ? 'text-amber-500' : tone === 'danger' ? 'text-red-500' : '';
    return (
        <article className="rounded-xl border border-border/60 bg-card p-4">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <Icon className="h-3.5 w-3.5" />
                {label}
            </div>
            <div className="mt-2 flex items-baseline gap-1">
                <span className={`text-xl font-semibold tracking-tight ${toneClass || 'text-foreground'}`}>{value}</span>
                {unit && <span className="text-xs text-muted-foreground">{unit}</span>}
            </div>
        </article>
    );
}

function HeroMetrics({ hero }: { hero: OpsTelemetryHeroMetrics }) {
    const t = useTranslations('ops');
    const errTone = getTelemetryErrorRateTone(hero.error_rate);
    return (
        <section className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
            <HeroMetricCard icon={Timer} label={t('telemetry.hero.uptime')} value={formatUptime(hero.uptime_seconds)} />
            <HeroMetricCard icon={TrendingUp} label={t('telemetry.hero.total_requests')} value={formatCount(hero.total_requests)} />
            <HeroMetricCard icon={Zap} label={t('telemetry.hero.avg_latency')} value={hero.avg_latency_ms.toFixed(0)} unit="ms" />
            <HeroMetricCard icon={Activity} label={t('telemetry.hero.error_rate')} value={formatTelemetryPercent(hero.error_rate)} tone={errTone} />
            <HeroMetricCard icon={Braces} label={t('telemetry.hero.active_connections')} value={String(hero.active_connections)} />
            <HeroMetricCard icon={Cpu} label={t('telemetry.hero.memory_usage')} value={String(hero.memory_usage_mb)} unit="MB" />
        </section>
    );
}

function RuntimeSignals({
    signals,
}: {
    signals: {
        p95_latency_ms: number;
        throughput_rps: number;
        memory_mb: number;
        trend_snapshots: OpsTelemetryTrendPoint[];
    };
}) {
    const t = useTranslations('ops');
    const snaps = signals.trend_snapshots ?? [];
    const maxReq = Math.max(...snaps.map((s) => s.request_delta), 1);
    return (
        <div className="space-y-4">
            <h3 className="text-sm font-medium">{t('telemetry.runtime_signals.title')}</h3>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <HeroMetricCard icon={Timer} label={t('telemetry.runtime_signals.p95_latency')} value={signals.p95_latency_ms.toFixed(0)} unit="ms" />
                <HeroMetricCard icon={TrendingUp} label={t('telemetry.runtime_signals.throughput')} value={signals.throughput_rps.toFixed(1)} unit="r/s" />
                <HeroMetricCard icon={Cpu} label={t('telemetry.runtime_signals.memory')} value={String(signals.memory_mb)} unit="MB" />
            </div>
            {snaps.length > 0 && (
                <div className="flex items-end gap-1 h-10">
                    {snaps.map((snap, i) => {
                        const h = `${Math.max(4, (snap.request_delta / maxReq) * 100)}%`;
                        return (
                            <div
                                key={i}
                                className="flex-1 rounded-t bg-primary/20"
                                style={{ height: h }}
                                title={`${snap.request_delta} req / ${snap.avg_latency_ms.toFixed(0)}ms`}
                            />
                        );
                    })}
                </div>
            )}
        </div>
    );
}

function DatabaseHealth({ db }: { db: OpsTelemetryDatabaseHealth }) {
    const t = useTranslations('ops');
    const ok = db.status === 'healthy';
    return (
        <div className="space-y-4">
            <h3 className="text-sm font-medium">{t('telemetry.database_health.title')}</h3>
            <div className="flex items-center gap-3">
                <div
                    className={`flex h-10 w-10 items-center justify-center rounded-xl ${
                        ok ? 'bg-emerald-500/10 text-emerald-600' : 'bg-amber-500/10 text-amber-600'
                    }`}
                >
                    <Database className="h-4 w-4" />
                </div>
                <div className="min-w-0">
                    <div className="text-sm font-medium">
                        {t(`telemetry.database_health.status.${ok ? 'healthy' : 'degraded'}`)}
                    </div>
                    {(db.issues ?? []).length > 0 && (
                        <div className="mt-1 text-xs text-muted-foreground">
                            {(db.issues ?? []).length}
                            {t('telemetry.database_health.issues_label')}: {(db.issues ?? []).slice(0, 3).join(', ')}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}

function SessionQuota({ activity }: { activity: OpsTelemetrySessionQuotaActivity }) {
    const t = useTranslations('ops');
    const rows: { label: string; value: number; alert?: boolean }[] = [
        { label: t('telemetry.session_quota.active_sessions'), value: activity.active_sessions },
        { label: t('telemetry.session_quota.sticky_sessions'), value: activity.sticky_bound_sessions },
        { label: t('telemetry.session_quota.quota_alerts'), value: activity.quota_alerts, alert: activity.quota_alerts > 0 },
    ];
    return (
        <div className="space-y-4">
            <h3 className="text-sm font-medium">{t('telemetry.session_quota.title')}</h3>
            <div className="space-y-2">
                {rows.map((r) => (
                    <div key={r.label} className="flex items-center justify-between rounded-lg border border-border/60 p-3">
                        <span className="text-xs text-muted-foreground">{r.label}</span>
                        <span className={`text-sm font-medium ${r.alert ? 'text-amber-500' : ''}`}>{r.value}</span>
                    </div>
                ))}
            </div>
        </div>
    );
}

function PromptCache({ cache }: { cache: OpsTelemetryPromptCache }) {
    const t = useTranslations('ops');
    return (
        <div className="space-y-4">
            <h3 className="text-sm font-medium">{t('telemetry.prompt_cache.title')}</h3>
            <div className="grid grid-cols-2 gap-3">
                <HeroMetricCard icon={HardDrive} label={t('telemetry.prompt_cache.entries')} value={`${cache.entries}/${cache.max_entries}`} />
                <HeroMetricCard icon={Activity} label={t('telemetry.prompt_cache.hit_rate')} value={formatTelemetryPercent(cache.hit_rate)} />
                <div className="rounded-xl border border-border/60 bg-card p-3">
                    <div className="text-xs text-muted-foreground">{t('telemetry.prompt_cache.hits')}</div>
                    <div className="mt-1 text-lg font-semibold">{formatCount(cache.hits)}</div>
                </div>
                <div className="rounded-xl border border-border/60 bg-card p-3">
                    <div className="text-xs text-muted-foreground">{t('telemetry.prompt_cache.misses')}</div>
                    <div className="mt-1 text-lg font-semibold">{formatCount(cache.misses)}</div>
                </div>
            </div>
        </div>
    );
}

function ProviderRow({ provider }: { provider: OpsTelemetryProviderItem }) {
    const t = useTranslations('ops');
    const statusTone = provider.health_status === 'healthy'
        ? 'success'
        : provider.health_status === 'warning' || provider.health_status === 'degraded'
            ? 'warning'
            : provider.health_status === 'down'
                ? 'danger'
                : 'neutral';

    return (
        <tr className="border-b border-border/40 align-top last:border-0">
            <td className="py-3 pr-3">
                <div className="min-w-0">
                    <div className="truncate text-sm font-semibold">{provider.channel_name}</div>
                    <div className="mt-1 truncate text-xs text-muted-foreground">
                        {provider.health_hint || provider.base_url}
                    </div>
                </div>
            </td>
            <td className="py-3 px-3">
                <StatusBadge
                    label={provider.health_status === 'disabled' ? t('system.fields.disabled') : t(`health.groupStatuses.${provider.health_status}`)}
                    tone={statusTone}
                />
            </td>
            <td className="py-3 px-3 text-right">
                <div className="text-sm font-medium tabular-nums">{provider.average_latency_ms.toFixed(0)} ms</div>
            </td>
            <td className="py-3 px-3 text-right">
                <div className="text-sm font-medium tabular-nums">{formatCount(provider.request_count)}</div>
            </td>
            <td className="py-3 pl-3 text-right">
                <div className="text-sm font-semibold tabular-nums">{formatTelemetryPercent(provider.success_rate)}</div>
            </td>
        </tr>
    );
}

function ProviderHealthTable({
    ph,
}: {
    ph: { providers: OpsTelemetryProviderItem[]; active: number; monitored: number };
}) {
    const t = useTranslations('ops');
    const providers = ph.providers ?? [];
    return (
        <div className="space-y-4">
            <div className="flex items-center justify-between">
                <h3 className="text-sm font-medium">{t('telemetry.provider_health.title')}</h3>
                <span className="text-xs text-muted-foreground">
                    {ph.active} {t('telemetry.provider_health.active')} / {ph.monitored} {t('telemetry.provider_health.monitored')}
                </span>
            </div>
            {providers.length === 0 ? (
                <p className="py-4 text-center text-xs text-muted-foreground">{t('telemetry.provider_health.empty')}</p>
            ) : (
                <div className="overflow-x-auto rounded-xl border border-border/60">
                    <table className="w-full min-w-[640px] text-left">
                        <thead>
                            <tr className="border-b border-border/40 bg-muted/30">
                                <th className="py-2.5 px-3 text-xs font-medium text-muted-foreground">
                                    {t('telemetry.provider_health.name')}
                                </th>
                                <th className="py-2.5 px-3 text-xs font-medium text-muted-foreground">
                                    {t('telemetry.provider_health.status')}
                                </th>
                                <th className="py-2.5 px-3 text-xs font-medium text-muted-foreground text-right">
                                    {t('telemetry.provider_health.latency')}
                                </th>
                                <th className="py-2.5 px-3 text-xs font-medium text-muted-foreground text-right">
                                    {t('telemetry.provider_health.requests')}
                                </th>
                                <th className="py-2.5 px-3 text-xs font-medium text-muted-foreground text-right">
                                    {t('telemetry.provider_health.success_rate')}
                                </th>
                            </tr>
                        </thead>
                        <tbody>
                            {providers.map((p) => (
                                <ProviderRow key={p.channel_id} provider={p} />
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    );
}

function Drilldown({
    shortcuts,
    onNavigate,
}: {
    shortcuts: OpsTelemetryDrilldownShortcut[];
    onNavigate: (tab: string) => void;
}) {
    const t = useTranslations('ops');
    const items = (shortcuts ?? []).filter((shortcut) => shortcut.key !== 'cache');

    if (items.length === 0) {
        return null;
    }

    return (
        <div className="space-y-3">
            <h3 className="text-sm font-medium">{t('telemetry.drilldown.title')}</h3>
            <div className="flex flex-wrap gap-2">
                {items.map((s) => (
                    <button
                        key={s.key}
                        type="button"
                        onClick={() => onNavigate(s.key)}
                        className="inline-flex items-center gap-1.5 rounded-lg border border-border/60 bg-card px-3 py-2 text-xs font-medium text-muted-foreground transition-colors hover:border-primary/30 hover:text-foreground"
                    >
                        {t(`tabs.${s.key}`)}
                        <ArrowRight className="h-3 w-3" />
                    </button>
                ))}
            </div>
        </div>
    );
}

export function Telemetry({ onNavigate }: { onNavigate: (tab: string) => void }) {
    const { data, isLoading, error } = useOpsTelemetrySummary();

    return (
        <QueryState loading={isLoading} error={error} empty={!data} emptyLabel="No data">
            {data ? (
                <div className="space-y-8">
                    <HeroMetrics hero={data.hero} />

                    <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
                        <div className="space-y-6">
                            <div className="rounded-xl border border-border bg-card p-5">
                                <RuntimeSignals signals={data.runtime_signals} />
                            </div>
                            <div className="rounded-xl border border-border bg-card p-5">
                                <PromptCache cache={data.prompt_cache} />
                            </div>
                        </div>
                        <div className="space-y-6">
                            <div className="rounded-xl border border-border bg-card p-5">
                                <DatabaseHealth db={data.database_health} />
                            </div>
                            <div className="rounded-xl border border-border bg-card p-5">
                                <SessionQuota activity={data.session_quota_activity} />
                            </div>
                        </div>
                    </div>

                    <div className="rounded-xl border border-border bg-card p-5">
                        <ProviderHealthTable ph={data.provider_health} />
                    </div>

                    <div className="rounded-xl border border-border bg-card p-5">
                        <Drilldown shortcuts={data.drilldown_shortcuts} onNavigate={onNavigate} />
                    </div>
                </div>
            ) : null}
        </QueryState>
    );
}
