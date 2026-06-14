'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { PageWrapper } from '@/components/common/PageWrapper';
import { Tabs, TabsContents, TabsContent, TabsList, TabsTrigger } from '@/components/animate-ui/components/animate/tabs';
import type { AnalyticsRange } from '@/api/endpoints/analytics';
import { Utilization } from './Utilization';
import { GroupHealth } from './GroupHealth';
import { ChannelModel } from './ChannelModel';
import { Evaluation } from './Evaluation';
import { LatencyDistribution } from './LatencyDistribution';
import { ShareSnapshot } from './ShareSnapshot';
import { Cache } from '@/components/modules/ops/Cache';
import { useAnalyticsOverview, useAnalyticsEvaluationSummary } from '@/api/endpoints/analytics';
import { formatCount, formatMoney } from '@/lib/utils';
import { formatPercent } from './shared';

type AnalyticsTab = 'utilization' | 'route-health' | 'channel-model' | 'cache' | 'evaluation' | 'latency';

const RANGE_OPTIONS: AnalyticsRange[] = ['1d', '7d', '30d', '90d', 'ytd', 'all'];

export function Analytics() {
    const t = useTranslations('analytics');
    const opsT = useTranslations('ops');
    const [activeTab, setActiveTab] = useState<AnalyticsTab>('cache');
    const [range, setRange] = useState<AnalyticsRange>('7d');
    const { data: overview } = useAnalyticsOverview(range);
    const { data: evaluationData } = useAnalyticsEvaluationSummary();

    return (
        <PageWrapper className="h-full min-h-0 overflow-y-auto overscroll-contain space-y-6 rounded-t-xl pb-3 md:pb-4">
            <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as AnalyticsTab)}>
                <section className="relative overflow-hidden rounded-xl border border-border/35 bg-card p-4 text-card-foreground md:p-5">
                    <div className="relative flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
                        <div className="-mx-1 overflow-x-auto overscroll-x-contain scroll-smooth px-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                            <TabsList className="flex w-max min-w-max flex-nowrap rounded-lg border-border/30 bg-card p-1 xl:min-w-0 xl:flex-wrap">
                                <TabsTrigger value="cache">{opsT('tabs.cache')}</TabsTrigger>
                                <TabsTrigger value="utilization">{t('cards.utilization.title')}</TabsTrigger>
                                <TabsTrigger value="route-health">{t('cards.routeHealth.title')}</TabsTrigger>
                                <TabsTrigger value="channel-model">{t('cards.channelModel.title')}</TabsTrigger>
                                <TabsTrigger value="evaluation">{t('evaluation.title')}</TabsTrigger>
                                <TabsTrigger value="latency">{t('latency.title')}</TabsTrigger>
                            </TabsList>
                        </div>

                        <div className="flex flex-wrap items-center gap-2">
                            <Tabs value={range} onValueChange={(value) => setRange(value as AnalyticsRange)}>
                                <div className="-mx-1 overflow-x-auto overscroll-x-contain scroll-smooth px-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                                    <TabsList className="flex w-max min-w-max flex-nowrap rounded-lg border-border/30 bg-card p-1 xl:min-w-0 xl:flex-wrap">
                                        {RANGE_OPTIONS.map((option) => (
                                            <TabsTrigger key={option} value={option}>
                                                {t(`range.${option}`)}
                                            </TabsTrigger>
                                        ))}
                                    </TabsList>
                                </div>
                            </Tabs>
                            <ShareSnapshot
                                data={{
                                    title: t('title'),
                                    subtitle: t('subtitle'),
                                    stats: overview
                                        ? [
                                            { label: t('metrics.requestCount'), value: formatCount(overview.request_count).formatted.value + formatCount(overview.request_count).formatted.unit },
                                            { label: t('metrics.totalTokens'), value: formatCount(overview.total_tokens).formatted.value + formatCount(overview.total_tokens).formatted.unit },
                                            { label: t('metrics.totalCost'), value: formatMoney(overview.total_cost).formatted.value + formatMoney(overview.total_cost).formatted.unit },
                                            { label: t('metrics.providerCount'), value: `${overview.provider_count}` },
                                            { label: t('cache.metrics.hitRate'), value: evaluationData?.semantic_cache.enabled ? `${formatPercent(evaluationData.semantic_cache.hit_rate).formatted.value}%` : t('cache.status.configuredOff') },
                                        ]
                                        : [],
                                    timestamp: new Date().toLocaleString(),
                                }}
                            />
                        </div>
                    </div>
                </section>

                <TabsContents>
                    <TabsContent value="cache">
                        <Cache />
                    </TabsContent>
                    <TabsContent value="utilization">
                        <Utilization range={range} />
                    </TabsContent>
                    <TabsContent value="route-health">
                        <GroupHealth />
                    </TabsContent>
                    <TabsContent value="channel-model">
                        <ChannelModel range={range} />
                    </TabsContent>
                    <TabsContent value="evaluation">
                        <Evaluation />
                    </TabsContent>
                    <TabsContent value="latency">
                        <LatencyDistribution range={range} />
                    </TabsContent>
                </TabsContents>
            </Tabs>
        </PageWrapper>
    );
}
