'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { PageWrapper } from '@/components/common/PageWrapper';
import { Tabs, TabsContents, TabsContent, TabsList, TabsTrigger } from '@/components/animate-ui/components/animate/tabs';
import type { AnalyticsRange } from '@/api/endpoints/analytics';
import { Utilization } from './Utilization';
import { GroupHealth } from './GroupHealth';
import { Evaluation } from './Evaluation';
import { Cache } from '@/components/modules/ops/Cache';

type AnalyticsTab = 'utilization' | 'route-health' | 'cache' | 'evaluation';

const RANGE_OPTIONS: AnalyticsRange[] = ['1d', '7d', '30d', '90d', 'ytd', 'all'];

export function Analytics() {
    const t = useTranslations('analytics');
    const opsT = useTranslations('ops');
    const [activeTab, setActiveTab] = useState<AnalyticsTab>('cache');
    const [range, setRange] = useState<AnalyticsRange>('7d');

    return (
        <PageWrapper className="h-full min-h-0 overflow-y-auto overscroll-contain space-y-6 rounded-t-xl pb-24 md:pb-4">
            <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as AnalyticsTab)}>
                <section className="relative overflow-hidden rounded-xl border border-border/35 bg-card p-4 text-card-foreground md:p-5">
                    <div className="relative flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
                        <div className="overflow-x-auto">
                            <TabsList className="flex w-max min-w-max flex-nowrap rounded-lg border-border/30 bg-card p-1 xl:min-w-0 xl:flex-wrap">
                                <TabsTrigger value="cache">{opsT('tabs.cache')}</TabsTrigger>
                                <TabsTrigger value="utilization">{t('cards.utilization.title')}</TabsTrigger>
                                <TabsTrigger value="route-health">{t('cards.routeHealth.title')}</TabsTrigger>
                                <TabsTrigger value="evaluation">{t('evaluation.title')}</TabsTrigger>
                            </TabsList>
                        </div>

                        <Tabs value={range} onValueChange={(value) => setRange(value as AnalyticsRange)}>
                            <div className="overflow-x-auto">
                                <TabsList className="flex w-max min-w-max flex-nowrap rounded-lg border-border/30 bg-card p-1 xl:min-w-0 xl:flex-wrap">
                                    {RANGE_OPTIONS.map((option) => (
                                        <TabsTrigger key={option} value={option}>
                                            {t(`range.${option}`)}
                                        </TabsTrigger>
                                    ))}
                                </TabsList>
                            </div>
                        </Tabs>
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
                    <TabsContent value="evaluation">
                        <Evaluation />
                    </TabsContent>
                </TabsContents>
            </Tabs>
        </PageWrapper>
    );
}
