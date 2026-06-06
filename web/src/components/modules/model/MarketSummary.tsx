'use client';

import { Boxes, Clock3, RefreshCw, RadioTower, Rows3 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { formatDateTime } from '@/lib/time';
import { formatAverageLatency, type LatencyUnitMode } from './latency-format';

type MarketSummaryValue = {
    model_count: number;
    coverage_count: number;
    unique_channel_count: number;
    average_latency_ms: number;
    last_update_time?: string;
};

function formatLastUpdate(value: string | undefined, fallback: string) {
    if (!value) return fallback;
    const formatted = formatDateTime(value);
    if (formatted === '-') return fallback;
    const date = new Date(value);
    if (!Number.isNaN(date.getTime()) && date.getFullYear() <= 1) return fallback;
    return formatted;
}

export function ModelMarketSummary({
    summary,
    onRefresh,
    isRefreshing,
    triggerClassName,
    compact = false,
    latencyUnit = 'auto',
}: {
    summary: MarketSummaryValue;
    onRefresh: () => void;
    isRefreshing: boolean;
    triggerClassName?: string;
    compact?: boolean;
    latencyUnit?: LatencyUnitMode;
}) {
    const t = useTranslations('model');
    const lastUpdateLabel = formatLastUpdate(summary.last_update_time, t('summary.neverUpdated'));
    const requestCount = summary.model_count > 0 ? 1 : 0;

    const metrics = [
        {
            key: 'models',
            icon: Boxes,
            label: t('summary.modelCount'),
            value: summary.model_count.toLocaleString(),
        },
        {
            key: 'coverage',
            icon: Rows3,
            label: t('summary.coverage'),
            value: summary.coverage_count.toLocaleString(),
        },
        {
            key: 'unique',
            icon: RadioTower,
            label: t('summary.uniqueChannels'),
            value: summary.unique_channel_count.toLocaleString(),
        },
        {
            key: 'latency',
            icon: Clock3,
            label: t('summary.averageLatency'),
            value: formatAverageLatency(summary.average_latency_ms, requestCount, latencyUnit),
        },
    ];

    return (
        <Popover>
            <PopoverTrigger asChild>
                <Button type="button" variant="ghost" size="default" className={triggerClassName}>
                    <Clock3 className="size-4 shrink-0 transition-colors duration-300" />
                    <span className={compact ? 'sr-only sm:not-sr-only' : undefined}>{t('summary.trigger')}</span>
                </Button>
            </PopoverTrigger>
            <PopoverContent align="end" className="w-[min(100vw-1rem,40rem)] rounded-xl border border-border/35 bg-card p-2 text-card-foreground shadow-lg sm:w-[min(100vw-2rem,40rem)] sm:p-3 md:p-4">
                <section className="relative overflow-hidden">
                    <div className="relative flex flex-col gap-2 sm:gap-3">
                        <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
                            <div className="min-w-0">
                                <div className="text-xs font-semibold text-foreground sm:text-sm">{t('summary.title')}</div>
                                <div className="mt-0.5 text-[0.68rem] text-muted-foreground sm:mt-1 sm:text-xs">{t('summary.description')}</div>
                            </div>
                            <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                onClick={onRefresh}
                                disabled={isRefreshing}
                                className="h-8 rounded-lg border-border/30 bg-card px-2.5 text-xs sm:h-9 sm:px-3 sm:text-sm"
                            >
                                <RefreshCw className={`mr-1.5 h-3.5 w-3.5 sm:mr-2 sm:h-4 sm:w-4 ${isRefreshing ? 'animate-spin' : ''}`} />
                                {isRefreshing ? t('summary.refreshing') : t('summary.refresh')}
                            </Button>
                        </div>

                        <div className="flex items-center gap-1.5 rounded-lg border border-border/30 bg-card px-2.5 py-1.5 text-[0.65rem] text-muted-foreground sm:gap-2 sm:px-3 sm:py-2 sm:text-[11px]">
                            <Clock3 className="h-3.5 w-3.5 shrink-0 text-primary sm:h-4 sm:w-4" />
                            <span className="min-w-0 truncate">{t('summary.lastUpdate')}: {lastUpdateLabel}</span>
                        </div>

                        <div className="grid grid-cols-2 gap-1.5 sm:gap-2 lg:grid-cols-4">
                            {metrics.map((metric) => (
                                <div key={metric.key} className="group relative min-w-0 overflow-hidden rounded-lg border border-border/30 bg-card px-2 py-1.5 transition-[transform,border-color] duration-300 hover:-translate-y-0.5 hover:border-primary/18 sm:px-2.5 sm:py-2 md:px-3 md:py-2.5">
                                    <div className="relative flex min-w-0 items-center gap-1 text-[0.6rem] text-muted-foreground sm:gap-1.5 sm:text-[10px] md:gap-2 md:text-sm">
                                        <metric.icon className="h-3 w-3 shrink-0 text-primary sm:h-3.5 sm:w-3.5 md:h-4 md:w-4" />
                                        <span className="min-w-0 truncate leading-tight">{metric.label}</span>
                                    </div>
                                    <div className="relative mt-0.5 min-w-0 truncate text-[1.15rem] font-semibold leading-none tracking-tight sm:mt-1 sm:text-[1.45rem] md:mt-2 md:text-[1.75rem]">{metric.value}</div>
                                </div>
                            ))}
                        </div>
                    </div>
                </section>
            </PopoverContent>
        </Popover>
    );
}
