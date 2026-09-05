'use client';

import { useStatsDaily, useStatsHourly, type StatsDailyFormatted, type StatsHourlyFormatted } from '@/api/endpoints/stats';
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart';
import { useMemo } from 'react';
import { Area, ComposedChart, CartesianGrid, XAxis, YAxis } from 'recharts';
import { useTranslations } from 'next-intl';
import { formatCount, formatMoney } from '@/lib/utils';
import { formatDateOnly } from '@/lib/time';
import { AnimatedNumber } from '@/components/common/AnimatedNumber';
import { useHomeStatsRefreshMs, useHomeViewStore, type ChartMetricType, type ChartPeriod } from '@/components/modules/home/store';
import { useSettingStore } from '@/stores/setting';
import { BarChart3, CalendarClock } from 'lucide-react';
import { cn } from '@/lib/utils';

const METRIC_DATA_KEYS: Record<ChartMetricType, string> = {
    cost: 'total_cost',
    count: 'request_count',
    tokens: 'total_token',
    'success-rate': 'success_rate',
};

export function StatsChart() {
    const PERIODS: readonly ChartPeriod[] = ['1', '7', '30'];
    const statsRefreshMs = useHomeStatsRefreshMs();
    const { data: statsDaily } = useStatsDaily({ refetchIntervalMs: statsRefreshMs });
    const { data: statsHourly } = useStatsHourly({ refetchIntervalMs: statsRefreshMs });
    const t = useTranslations('home.chart');
    const { chinaMode } = useSettingStore();

    const chartMetrics = useHomeViewStore((state) => state.chartMetrics);
    const toggleChartMetric = useHomeViewStore((state) => state.toggleChartMetric);
    const period = useHomeViewStore((state) => state.chartPeriod);
    const setChartPeriod = useHomeViewStore((state) => state.setChartPeriod);

    const sortedDaily = useMemo(() => {
        if (!statsDaily) return [];
        return [...statsDaily].sort((a, b) => a.date.localeCompare(b.date));
    }, [statsDaily]);

    const buildPointValue = (type: ChartMetricType, stat?: StatsDailyFormatted | StatsHourlyFormatted): number => {
        if (!stat) return 0;
        if (type === 'cost') return stat.total_cost.raw;
        if (type === 'success-rate') {
            const total = stat.request_success.raw + stat.request_failed.raw;
            return total > 0 ? (stat.request_success.raw / total) * 100 : 0;
        }
        if (type === 'count') return stat.request_count.raw;
        return stat.input_token.raw + stat.output_token.raw;
    };

    const chartData = useMemo(() => {
        if (period === '1') {
            if (!statsHourly) return [];
            return statsHourly.map((stat) => {
                const point: Record<string, number | string> = { date: `${stat.hour}:00` };
                for (const type of chartMetrics) {
                    point[METRIC_DATA_KEYS[type]] = buildPointValue(type, stat);
                }
                return point;
            });
        } else {
            const days = Number(period);
            return sortedDaily.slice(-days).map((stat) => {
                const point: Record<string, number | string> = {
                    date: /^\d{8}$/.test(stat.date)
                        ? `${stat.date.slice(4, 6)}/${stat.date.slice(6, 8)}`
                        : formatDateOnly(stat.date, { month: '2-digit', day: '2-digit' }),
                };
                for (const type of chartMetrics) {
                    point[METRIC_DATA_KEYS[type]] = buildPointValue(type, stat);
                }
                return point;
            });
        }
    }, [sortedDaily, statsHourly, period, chartMetrics]);

    const totals = useMemo(() => {
        if (period === '1') {
            if (!statsHourly) return { requests: 0, cost: 0, tokens: 0, successRate: 0 };
            const requests = statsHourly.reduce((acc, stat) => acc + stat.request_count.raw, 0);
            const cost = statsHourly.reduce((acc, stat) => acc + stat.total_cost.raw, 0);
            const tokens = statsHourly.reduce((acc, stat) => acc + stat.input_token.raw + stat.output_token.raw, 0);
            const success = statsHourly.reduce((acc, stat) => acc + stat.request_success.raw, 0);
            const failed = statsHourly.reduce((acc, stat) => acc + stat.request_failed.raw, 0);
            return {
                requests,
                cost,
                tokens,
                successRate: success+failed > 0 ? (success / (success + failed)) * 100 : 0,
            };
        } else {
            const days = Number(period);
            const recentStats = sortedDaily.slice(-days);
            const requests = recentStats.reduce((acc, stat) => acc + stat.request_success.raw + stat.request_failed.raw, 0);
            const cost = recentStats.reduce((acc, stat) => acc + stat.total_cost.raw, 0);
            const tokens = recentStats.reduce((acc, stat) => acc + stat.input_token.raw + stat.output_token.raw, 0);
            const success = recentStats.reduce((acc, stat) => acc + stat.request_success.raw, 0);
            const failed = recentStats.reduce((acc, stat) => acc + stat.request_failed.raw, 0);
            return {
                requests,
                cost,
                tokens,
                successRate: success+failed > 0 ? (success / (success + failed)) * 100 : 0,
            };
        }
    }, [sortedDaily, statsHourly, period]);

    const chartConfig = useMemo(() => {
        const labels = {
            'total_cost': t('totalCost'),
            'request_count': t('totalRequests'),
            'total_token': t('totalTokens'),
            'success_rate': t('successRate'),
        };
        const config: Record<string, { label: string }> = {};
        for (const type of chartMetrics) {
            config[METRIC_DATA_KEYS[type]] = { label: labels[METRIC_DATA_KEYS[type] as keyof typeof labels] };
        }
        return config;
    }, [chartMetrics, t]);

    const getPeriodLabel = (p: ChartPeriod) => {
        const labels = {
            '1': t('period.today'),
            '7': t('period.last7Days'),
            '30': t('period.last30Days'),
        };
        return labels[p];
    };

    const handlePeriodClick = () => {
        const currentIndex = PERIODS.indexOf(period);
        const nextIndex = (currentIndex + 1) % PERIODS.length;
        setChartPeriod(PERIODS[nextIndex]);
    };

    const getChartStroke = (type: ChartMetricType) => {
        if (type === 'cost') return 'var(--chart-1)';
        if (type === 'count') return 'var(--chart-2)';
        if (type === 'success-rate') return 'var(--chart-4)';
        return 'var(--chart-3)';
    };

    const getChartFill = (type: ChartMetricType) => {
        if (type === 'cost') return 'url(#fillMetric1)';
        if (type === 'count') return 'url(#fillMetric2)';
        if (type === 'success-rate') return 'url(#fillMetric4)';
        return 'url(#fillMetric3)';
    };

    // total_cost already went through formatMoney() in the stats select mapper,
    // so in chinaMode stat.total_cost.raw is already CNY (USD x exchangeRate once).
    // Formatting it again via formatMoney would apply the exchange rate a second
    // time and desync the chart from Overview. Use formatDisplayCost for any value
    // that is already in display currency.
    const formatDisplayCost = (value: number): { value: string; unit: string } => {
        if (chinaMode) {
            const v = Number(value);
            if (v >= 100_000_000) return { value: (v / 100_000_000).toFixed(2), unit: '亿元' };
            if (v >= 10_000) return { value: (v / 10_000).toFixed(2), unit: '万元' };
            return { value: v.toFixed(2), unit: '元' };
        }
        return formatMoney(value).formatted;
    };

    const costAxisFormatter = (value: number): string => {
        const formatted = formatDisplayCost(Number(value));
        return `${formatted.value}${formatted.unit}`;
    };

    // Y 轴刻度：大跨度不需要小数点精度，中国化模式整数化（1500.00万 → 1500万），
    // 避免 4 位数字 + 两位小数把刻度挤爆被截断；非中国化保持原样。
    const axisCountTick = (value: number): string => {
        if (chinaMode) {
            const v = Number(value);
            if (v >= 100_000_000) return `${Math.round(v / 100_000_000)}亿`;
            if (v >= 10_000) return `${Math.round(v / 10_000)}万`;
            return `${Math.round(v)}`;
        }
        const formatted = formatCount(value).formatted;
        return `${formatted.value}${formatted.unit}`;
    };

    const axisCostTick = (value: number): string => {
        if (chinaMode) {
            const v = Number(value);
            if (v >= 100_000_000) return `${Math.round(v / 100_000_000)}亿元`;
            if (v >= 10_000) return `${Math.round(v / 10_000)}万元`;
            return `${Math.round(v)}元`;
        }
        return costAxisFormatter(value);
    };

    const costSummary = formatDisplayCost(totals.cost);


    const summaryMetrics = [
        {
            key: 'requests',
            label: t('totalRequests'),
            value: formatCount(totals.requests).formatted.value,
            unit: formatCount(totals.requests).formatted.unit,
        },
        {
            key: 'cost',
            label: t('totalCost'),
            value: costSummary.value,
            unit: costSummary.unit,
        },
        {
            key: 'tokens',
            label: t('totalTokens'),
            value: formatCount(totals.tokens).formatted.value,
            unit: formatCount(totals.tokens).formatted.unit,
        },
        {
            key: 'successRate',
            label: t('successRate'),
            value: totals.successRate.toFixed(2),
            unit: '%',
        },
    ];

    return (
        <div className="relative rounded-xl border border-border bg-card pt-5 text-card-foreground">
            <div className="space-y-4 px-4 pb-3 md:px-5">
                <div className="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
                    <div className="flex items-center gap-2">
                        <div className="inline-flex h-9 items-center gap-2 rounded-lg border border-primary/12 bg-card px-3 text-sm font-medium text-primary">
                            <BarChart3 className="h-4 w-4" strokeWidth={1.5} />
                            <span>{t('title')}</span>
                        </div>
                        <button
                            type="button"
                            className="inline-flex h-9 items-center gap-2 rounded-lg border border-border bg-card px-3 text-left text-sm transition-[transform,border-color] duration-200 hover:-translate-y-0.5 hover:border-border/80"
                            onClick={handlePeriodClick}
                        >
                            <CalendarClock className="h-4 w-4 text-primary/60" strokeWidth={1.5} />
                            <span className="text-xs text-muted-foreground">{t('timePeriod')}</span>
                            <span className="font-semibold">{getPeriodLabel(period)}</span>
                        </button>
                    </div>
                    <div className="flex flex-wrap gap-1.5" role="group" aria-label={t('title')}>
                        {(['cost', 'count', 'tokens', 'success-rate'] as ChartMetricType[]).map((type) => {
                            const active = chartMetrics.includes(type);
                            return (
                                <button
                                    key={type}
                                    type="button"
                                    onClick={() => toggleChartMetric(type)}
                                    aria-pressed={active}
                                    className={cn(
                                        'inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors',
                                        active
                                            ? 'border-primary/40 bg-primary/10 text-primary'
                                            : 'border-border bg-card text-muted-foreground hover:text-foreground',
                                    )}
                                >
                                    <span
                                        className="h-2 w-2 rounded-full"
                                        style={{ backgroundColor: getChartStroke(type) }}
                                    />
                                    {t(`metricType.${type === 'success-rate' ? 'successRate' : type}`)}
                                </button>
                            );
                        })}
                    </div>
                </div>

                <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
                    {summaryMetrics.map((metric) => (
                        <div key={metric.key} className="rounded-lg border border-border bg-card px-3.5 py-3">
                            <div className="mb-2 h-1 w-9 rounded-full bg-primary/18" />
                            <div className="text-xs text-muted-foreground">{metric.label}</div>
                            <div className="mt-1 flex items-baseline gap-1">
                                <span className="text-xl font-semibold tracking-tight">
                                    <AnimatedNumber value={metric.value} />
                                </span>
                                <span className="text-sm text-muted-foreground">{metric.unit}</span>
                            </div>
                        </div>
                    ))}
                </div>
            </div>
            <div className="relative mx-3 mb-3 overflow-hidden rounded-lg border border-border bg-card pt-3">
                <ChartContainer config={chartConfig} className="h-[20rem] w-full md:h-[24rem]">
                <ComposedChart accessibilityLayer data={chartData}>
                    <defs>
                        <linearGradient id="fillMetric1" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="5%" stopColor="var(--chart-1)" stopOpacity={1.0} />
                            <stop offset="95%" stopColor="var(--chart-1)" stopOpacity={0.1} />
                        </linearGradient>
                        <linearGradient id="fillMetric2" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="5%" stopColor="var(--chart-2)" stopOpacity={1.0} />
                            <stop offset="95%" stopColor="var(--chart-2)" stopOpacity={0.1} />
                        </linearGradient>
                        <linearGradient id="fillMetric3" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="5%" stopColor="var(--chart-3)" stopOpacity={1.0} />
                            <stop offset="95%" stopColor="var(--chart-3)" stopOpacity={0.1} />
                        </linearGradient>
                        <linearGradient id="fillMetric4" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="5%" stopColor="var(--chart-4)" stopOpacity={1.0} />
                            <stop offset="95%" stopColor="var(--chart-4)" stopOpacity={0.1} />
                        </linearGradient>
                    </defs>
                    <CartesianGrid strokeDasharray="3 3" vertical={false} />
                    <XAxis dataKey="date" tickLine={false} axisLine={false} />
                    <YAxis
                        tickLine={false}
                        axisLine={false}
                        tickFormatter={(value) => {
                            const primary = chartMetrics[0] ?? 'cost';
                            if (primary === 'cost') {
                                return axisCostTick(Number(value));
                            } else if (primary === 'success-rate') {
                                return `${value.toFixed(0)}%`;
                            } else if (primary === 'count' || primary === 'tokens') {
                                return axisCountTick(Number(value));
                            }
                            return value.toString();
                        }}
                    />
                    <ChartTooltip
                        cursor={false}
                        content={
                            <ChartTooltipContent
                                indicator="line"
                                valueFormatter={(value) => {
                                    const primary = chartMetrics[0] ?? 'cost';
                                    if (primary === 'cost') return costAxisFormatter(Number(value));
                                    if (primary === 'success-rate') return `${Number(value).toFixed(2)}%`;
                                    const f = formatCount(Number(value));
                                    return `${f.formatted.value}${f.formatted.unit}`;
                                }}
                            />
                        }
                    />
                    <Area
                        type="monotone"
                        dataKey={METRIC_DATA_KEYS[chartMetrics[0]]}
                        stroke={getChartStroke(chartMetrics[0])}
                        fill={getChartFill(chartMetrics[0])}
                        strokeWidth={2}
                    />
                </ComposedChart>
                </ChartContainer>
            </div>
        </div>
    );
}
