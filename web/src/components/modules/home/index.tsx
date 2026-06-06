'use client';

import { Activity } from './activity';
import { HomeHero } from './hero';
import { HomeAnalyticsOverview } from './analytics-overview';
import { StatsChart } from './chart';
import { Rank } from './rank';
import { PageWrapper } from '@/components/common/PageWrapper';

export function Home() {
    return (
        <PageWrapper className="h-full min-h-0 overflow-y-auto overscroll-contain space-y-5 md:space-y-7 rounded-t-xl pb-3 md:pb-4">
            <HomeHero />
            <HomeAnalyticsOverview />
            <StatsChart />
            <div className="grid items-start gap-4 md:gap-5 xl:grid-cols-[minmax(0,1.08fr)_minmax(320px,0.4fr)]">
                <div className="min-w-0">
                    <Activity />
                </div>
                <div className="min-w-0">
                    <Rank />
                </div>
            </div>
        </PageWrapper>
    );
}
