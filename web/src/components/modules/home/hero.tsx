'use client';

import { motion } from 'motion/react';
import { Waves } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useStatsToday } from '@/api/endpoints/stats';
import { useOpsCacheStatus, type OpsProviderPromptCacheTrendPoint } from '@/api/endpoints/ops';
import { useHomeStatsRefreshMs } from './store';
import { StatsRefreshControls } from './refresh-controls';
import { AnimatedNumber } from '@/components/common/AnimatedNumber';
import { EASING } from '@/lib/animations/fluid-transitions';
import { formatCount, formatMoney, formatTime } from '@/lib/utils';

// Adaptive metric font size: CJK units (万/亿/元) count double-width, so long
// values (e.g. "2,008万" / "1,234.56万元") shrink instead of overflowing the
// 2-column card grid (chinaMode 千万级 4 位数字场景).
function visualWidthOf(value: string | undefined, unit: string | undefined): number {
    let w = 0;
    for (const ch of value ?? '') w += ch.charCodeAt(0) > 255 ? 2 : 1;
    for (const ch of unit ?? '') w += ch.charCodeAt(0) > 255 ? 2 : 1;
    return w;
}

function metricFontClass(width: number): string {
    if (width <= 6) return 'text-xl sm:text-2xl';
    if (width <= 8) return 'text-lg sm:text-xl';
    if (width <= 10) return 'text-base sm:text-lg';
    return 'text-sm sm:text-base';
}

// 中国化模式万级整数化：794.25万 → 794万。大跨度展示两位小数无意义，
// 整数化后保留单位、缩短宽度（794万 / 978万），窄卡片不再挤。
// 亿级保留两位小数（1.02亿），k/m/b 模式与原始数值保持原样。
function chinaIntValue(value: string, unit: string): string {
    if (unit === '万' && value.includes('.')) {
        return String(Math.round(parseFloat(value)));
    }
    return value;
}

export function HomeHero() {
    const t = useTranslations('home.hero');
    const statsRefreshMs = useHomeStatsRefreshMs();
    const { data: statsToday } = useStatsToday({ refetchIntervalMs: statsRefreshMs });
    const { data: cacheStatus } = useOpsCacheStatus();

    const requestCount = (statsToday?.request_success ?? 0) + (statsToday?.request_failed ?? 0);
    const successCount = statsToday?.request_success ?? 0;
    const totalCost = (statsToday?.input_cost ?? 0) + (statsToday?.output_cost ?? 0);
    const totalTokens = (statsToday?.input_token ?? 0) + (statsToday?.output_token ?? 0);
    const totalWaitTime = statsToday?.wait_time ?? 0;
    const successRate = requestCount > 0 ? (successCount / requestCount) * 100 : 0;
    const avgWait = requestCount > 0 ? totalWaitTime / requestCount : 0;

    // 今日缓存复用 Tokens：对 provider prompt cache 的 24h 趋势点求和（timestamp >= 本地时区今日 0 点）。
    // trend 按 UTC 整点对齐（后端 opsHourlyWindowStart），+8 时区下今日 0 点恰为整点边界，过滤无污染。
    const cacheTrend = cacheStatus?.provider_prompt_cache?.trend ?? ([] as OpsProviderPromptCacheTrendPoint[]);
    const todayCacheReadTokens = cacheTrend
        .filter((point) => point.timestamp >= Math.floor(new Date(new Date().getFullYear(), new Date().getMonth(), new Date().getDate()).getTime() / 1000))
        .reduce((sum, point) => sum + (point.cache_read_tokens ?? 0), 0);
    const cacheRate = totalTokens > 0 ? (todayCacheReadTokens / totalTokens) * 100 : 0;

    // 复合卡数值中国化整数化（794.25万 → 794万），单位保留；k/m/b 模式原样。
    const fmtCount = (n?: number) => {
        const f = formatCount(n).formatted;
        return { value: chinaIntValue(f.value, f.unit), unit: f.unit };
    };

    const callsSuccess = fmtCount(successCount);
    const callsTotal = fmtCount(requestCount);
    const cacheRead = fmtCount(todayCacheReadTokens);
    const tokens = fmtCount(totalTokens);

    // 2 列 × 2 行：第一行普通指标（平均响应时延 / 今日花费），第二行复合卡（今日调用 / 今日Token使用）。
    const cards = [
        {
            key: 'avgWait',
            label: t('metrics.avgWait'),
            value: formatTime(avgWait).formatted.value,
            unit: formatTime(avgWait).formatted.unit,
        },
        {
            key: 'cost',
            label: t('signals.cost'),
            value: formatMoney(totalCost).formatted.value,
            unit: formatMoney(totalCost).formatted.unit,
        },
        {
            key: 'calls',
            label: t('signals.requests'),
            isComposite: true,
            mainValue: callsSuccess.value,
            mainUnit: callsSuccess.unit,
            dividerValue: callsTotal.value,
            dividerUnit: callsTotal.unit,
            rate: successRate.toFixed(2),
            rateLabel: t('signals.successRateShort'),
        },
        {
            key: 'cacheRate',
            label: t('signals.cacheRate'),
            isComposite: true,
            mainValue: cacheRead.value,
            mainUnit: cacheRead.unit,
            dividerValue: tokens.value,
            dividerUnit: tokens.unit,
            rate: cacheRate.toFixed(2),
            rateLabel: t('signals.cacheRateShort'),
        },
    ];

    return (
        <motion.section
            className="relative rounded-xl border border-border bg-card p-5 text-card-foreground md:p-6 xl:p-7"
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.45, ease: EASING.easeOutExpo }}
        >
            <div className="space-y-5">
                <div className="space-y-3">
                    <div className="flex flex-wrap items-center justify-between gap-3 sm:gap-4">
                        <div className="flex items-center gap-3 sm:gap-4">
                            <div className="grid h-11 w-11 sm:h-14 sm:w-14 shrink-0 place-items-center overflow-hidden rounded-lg border border-border bg-card text-primary">
                                <Waves className="h-5 w-5 sm:h-6 sm:w-6" strokeWidth={1.5} />
                            </div>
                            <div className="space-y-1">
                                <h1 className="text-[1.65rem] font-semibold tracking-tight sm:text-2xl md:text-3xl lg:text-4xl">{t('title')}</h1>
                                {t('subtitle') ? (
                                    <p className="text-sm leading-6 text-muted-foreground md:text-base">{t('subtitle')}</p>
                                ) : null}
                            </div>
                        </div>
                        <StatsRefreshControls />
                    </div>

                    {t('description') ? (
                        <p className="max-w-2xl text-sm leading-7 text-muted-foreground md:text-[15px]">
                            {t('description')}
                        </p>
                    ) : null}
                </div>

                <div className="grid grid-cols-2 gap-2.5 sm:gap-3">
                    {cards.map((card) => (
                        <article
                            key={card.key}
                            className="group rounded-lg border border-border bg-card px-3 py-2.5 transition-colors duration-200 hover:border-border/80 hover:bg-muted/30 sm:px-4 sm:py-3"
                        >
                            <div className="mb-2 h-1 w-10 rounded-full bg-primary/20 transition-all duration-300 group-hover:w-14 group-hover:bg-primary/30" />
                            <div className="text-xs font-medium text-muted-foreground">{card.label}</div>
                            {card.isComposite ? (
                                <div className="mt-1 space-y-0.5">
                                    <div className="flex items-baseline gap-1.5 min-w-0">
                                        <span className={`${metricFontClass(visualWidthOf(card.mainValue, card.mainUnit))} font-semibold tracking-tight whitespace-nowrap`}>
                                            <AnimatedNumber value={card.mainValue} />
                                            {card.mainUnit ? <span className="text-sm text-muted-foreground">{card.mainUnit}</span> : null}
                                        </span>
                                        <span className="text-sm text-muted-foreground whitespace-nowrap">
                                            / {card.dividerValue}{card.dividerUnit}
                                        </span>
                                    </div>
                                    <div className="text-xs text-muted-foreground">
                                        {card.rateLabel} {card.rate}%
                                    </div>
                                </div>
                            ) : (
                                <div className="mt-1 flex items-baseline gap-1 min-w-0">
                                    <span className={`${metricFontClass(visualWidthOf(card.value, card.unit))} font-semibold tracking-tight`}>
                                        <AnimatedNumber value={card.value} />
                                    </span>
                                    {card.unit ? <span className="text-sm text-muted-foreground">{card.unit}</span> : null}
                                </div>
                            )}
                        </article>
                    ))}
                </div>
            </div>
        </motion.section>
    );
}
