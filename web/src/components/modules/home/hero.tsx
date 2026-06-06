'use client';

import { motion } from 'motion/react';
import { Activity, ArrowRight, DollarSign, Leaf, ShieldCheck, Waves } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useStatsToday } from '@/api/endpoints/stats';
import { AnimatedNumber } from '@/components/common/AnimatedNumber';
import { EASING } from '@/lib/animations/fluid-transitions';
import { formatCount, formatMoney, formatTime } from '@/lib/utils';

export function HomeHero() {
    const t = useTranslations('home.hero');
    const { data: statsToday } = useStatsToday();

    const requestCount = (statsToday?.request_success ?? 0) + (statsToday?.request_failed ?? 0);
    const successCount = statsToday?.request_success ?? 0;
    const totalCost = (statsToday?.input_cost ?? 0) + (statsToday?.output_cost ?? 0);
    const totalWaitTime = statsToday?.wait_time ?? 0;
    const successRate = requestCount > 0 ? (successCount / requestCount) * 100 : 0;
    const avgWait = requestCount > 0 ? totalWaitTime / requestCount : 0;

    const signals = [
        {
            key: 'requests',
            label: t('signals.requests'),
            value: formatCount(requestCount).formatted.value,
            unit: formatCount(requestCount).formatted.unit,
            icon: Activity,
            accent: 'bg-emerald-500/10 text-emerald-700',
        },
        {
            key: 'successRate',
            label: t('signals.successRate'),
            value: successRate.toFixed(2),
            unit: '%',
            icon: ShieldCheck,
            accent: 'bg-primary/10 text-primary',
        },
        {
            key: 'cost',
            label: t('signals.cost'),
            value: formatMoney(totalCost).formatted.value,
            unit: formatMoney(totalCost).formatted.unit,
            icon: DollarSign,
            accent: 'bg-amber-500/10 text-amber-700',
        },
    ];

    const metrics = [
        {
            key: 'avgWait',
            label: t('metrics.avgWait'),
            value: formatTime(avgWait).formatted.value,
            unit: formatTime(avgWait).formatted.unit,
        },
        {
            key: 'successful',
            label: t('metrics.successful'),
            value: formatCount(successCount).formatted.value,
            unit: formatCount(successCount).formatted.unit,
        },
        {
            key: 'tokens',
            label: t('metrics.tokens'),
            value: formatCount((statsToday?.input_token ?? 0) + (statsToday?.output_token ?? 0)).formatted.value,
            unit: formatCount((statsToday?.input_token ?? 0) + (statsToday?.output_token ?? 0)).formatted.unit,
        },
        {
            key: 'costPerReq',
            label: t('metrics.costPerRequest'),
            value: formatMoney(requestCount > 0 ? totalCost / requestCount : 0).formatted.value,
            unit: formatMoney(requestCount > 0 ? totalCost / requestCount : 0).formatted.unit,
        },
    ];

    return (
        <motion.section
            className="relative rounded-xl border border-border bg-card p-5 text-card-foreground md:p-6 xl:p-7"
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.45, ease: EASING.easeOutExpo }}
        >
            <div className="grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(20rem,0.88fr)]">
                <div className="space-y-5">
                    <div className="space-y-3">
                        <div className="flex items-center gap-3 sm:gap-4">
                            <div className="grid h-11 w-11 sm:h-14 sm:w-14 shrink-0 place-items-center overflow-hidden rounded-lg border border-border bg-card text-primary">
                                <Waves className="h-5 w-5 sm:h-6 sm:w-6" strokeWidth={1.5} />
                            </div>
                            <div className="space-y-1">
                                <h1 className="text-2xl font-semibold tracking-tight md:text-3xl lg:text-4xl">{t('title')}</h1>
                                {t('subtitle') ? (
                                    <p className="text-sm leading-6 text-muted-foreground md:text-base">{t('subtitle')}</p>
                                ) : null}
                            </div>
                        </div>

                        {t('description') ? (
                            <p className="max-w-2xl text-sm leading-7 text-muted-foreground md:text-[15px]">
                                {t('description')}
                            </p>
                        ) : null}
                    </div>

                    <div className="grid gap-3 sm:grid-cols-2">
                        {metrics.map((metric) => (
                            <div key={metric.key} className="group rounded-lg border border-border bg-card px-4 py-3 transition-colors duration-200 hover:border-border/80 hover:bg-muted/30">
                                <div className="mb-2 h-1 w-10 rounded-full bg-primary/20 transition-all duration-300 group-hover:w-14 group-hover:bg-primary/30" />
                                <div className="text-xs font-medium text-muted-foreground">{metric.label}</div>
                                <div className="mt-1 flex items-baseline gap-1">
                                    <span className="text-2xl font-semibold">
                                        <AnimatedNumber value={metric.value} />
                                    </span>
                                    {metric.unit ? <span className="text-sm text-muted-foreground">{metric.unit}</span> : null}
                                </div>
                            </div>
                        ))}
                    </div>
                </div>

                <div className="grid auto-rows-fr gap-3 sm:grid-cols-2 xl:grid-cols-2">
                    {signals.map((signal, index) => (
                        <article
                            key={signal.key}
                            className={`group rounded-lg border border-border bg-card p-3 sm:p-4 transition-[transform,border-color] duration-300 hover:-translate-y-0.5 hover:border-border/80 ${index === 0 ? 'sm:col-span-2 xl:min-h-[11rem]' : ''}`}
                        >
                            <div className="flex items-start justify-between gap-3">
                                <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${signal.accent}`}>
                                    <signal.icon className="h-[1.125rem] w-[1.125rem]" strokeWidth={1.5} />
                                </div>
                                <div className="flex items-center gap-2 text-muted-foreground/50">
                                    {index === 0 ? <Leaf className="h-4 w-4 text-primary/50" strokeWidth={1.5} /> : null}
                                    <ArrowRight className="h-4 w-4 transition-transform duration-200 group-hover:translate-x-0.5" strokeWidth={1.5} />
                                </div>
                            </div>
                            <div className="mt-5 text-xs text-muted-foreground">{signal.label}</div>
                            <div className="mt-2 flex items-baseline gap-1">
                                <span className={`${index === 0 ? 'text-3xl md:text-4xl' : 'text-2xl'} font-semibold tracking-tight`}>
                                    <AnimatedNumber value={signal.value} />
                                </span>
                                {signal.unit ? <span className="text-sm text-muted-foreground">{signal.unit}</span> : null}
                            </div>
                        </article>
                    ))}
                </div>
            </div>
        </motion.section>
    );
}
