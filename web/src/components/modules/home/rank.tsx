'use client';

import { useStatsAPIKey, useStatsChannel } from '@/api/endpoints/stats';
import { useMemo } from 'react';
import { useTranslations } from 'next-intl';
import { Leaf, Loader2, TrendingUp, Trophy } from 'lucide-react';
import { Tabs, TabsList, TabsTrigger, TabsContents, TabsContent } from '@/components/animate-ui/components/animate/tabs';
import { useHomeViewStore, type RankSortMode } from '@/components/modules/home/store';
import { cn } from '@/lib/utils';

type ChannelData = NonNullable<ReturnType<typeof useStatsChannel>['data']>[number];
type APIKeyStatsData = NonNullable<ReturnType<typeof useStatsAPIKey>['data']>[number];
type APIKeyRankData = APIKeyStatsData & { name: string };

export function Rank() {
    const {
        data: channelData,
        isLoading: isChannelListLoading,
    } = useStatsChannel();
    const t = useTranslations('home.rank');
    const rankSortMode = useHomeViewStore((state) => state.rankSortMode);
    const setRankSortMode = useHomeViewStore((state) => state.setRankSortMode);
    const {
        data: apiKeyStats,
        isLoading: isAPIKeyStatsLoading,
    } = useStatsAPIKey({ enabled: rankSortMode === 'key-usage' });

    const channelsWithUsage = useMemo<ChannelData[]>(() => {
        if (!channelData) return [];
        return channelData.filter((channel) => channel.request_count.raw > 0);
    }, [channelData]);

    const rankedByCost = useMemo<ChannelData[]>(() => {
        return [...channelsWithUsage].sort((a, b) => b.total_cost.raw - a.total_cost.raw);
    }, [channelsWithUsage]);

    const rankedByCount = useMemo<ChannelData[]>(() => {
        return [...channelsWithUsage].sort((a, b) => b.request_count.raw - a.request_count.raw);
    }, [channelsWithUsage]);

    const rankedByTokens = useMemo<ChannelData[]>(() => {
        return [...channelsWithUsage].sort((a, b) => b.total_token.raw - a.total_token.raw);
    }, [channelsWithUsage]);

    const rankedByKeyUsage = useMemo<APIKeyRankData[]>(() => {
        if (!apiKeyStats) return [];

        return apiKeyStats
            .filter((stats) => stats.request_count.raw > 0)
            .map((stats) => ({
                ...stats,
                name: stats.name || `Key #${stats.api_key_id}`,
            }))
            .sort((a, b) => b.request_count.raw - a.request_count.raw);
    }, [apiKeyStats]);

    const getRankToneClass = (rank: number): string => {
        if (rank === 1) return 'border-primary/30 bg-primary/14 text-primary';
        if (rank === 2) return 'border-emerald-500/20 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300';
        if (rank === 3) return 'border-amber-500/20 bg-amber-500/10 text-amber-700 dark:text-amber-300';
        return 'border-border bg-card text-muted-foreground';
    };

    const renderChannelList = (channels: ChannelData[], mode: Exclude<RankSortMode, 'key-usage'>, isLoading: boolean) => {
        if (isLoading) {
            return (
                <div className="flex min-h-44 flex-col items-center justify-center rounded-lg border border-border bg-card py-8 text-muted-foreground">
                    <Loader2 className="mb-3 h-10 w-10 animate-spin opacity-50" />
                    <p className="text-sm">{t('loading')}</p>
                </div>
            );
        }

        if (channels.length === 0) {
            return (
                <div className="flex min-h-44 flex-col items-center justify-center rounded-lg border border-border bg-card py-8 text-muted-foreground">
                    <TrendingUp className="mb-3 h-10 w-10 opacity-30" strokeWidth={1.5} />
                    <p className="text-sm">{t('noData')}</p>
                </div>
            );
        }
        return (
            <div className="max-h-[196px] space-y-2.5 overflow-y-auto pr-1 [scrollbar-width:thin]">
                {channels.map((channel, index) => {
                    const rank = index + 1;

                    return (
                        <div
                            key={channel.channel_id}
                            className={cn(
                                'group flex items-center gap-3 rounded-lg border border-border bg-card p-2.5 transition-[transform,border-color] duration-200 hover:-translate-y-0.5 hover:border-border/80',
                            )}
                        >
                            <div className={cn('flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border text-sm font-bold', getRankToneClass(rank))}>
                                {rank === 1 ? <Trophy className="h-4 w-4" strokeWidth={1.5} /> : rank}
                            </div>

                            <div className="min-w-0 flex-1">
                                <p className="truncate text-sm font-semibold">{channel.channel_name}</p>
                                {mode === 'count' && (() => {
                                    const successCount = channel.request_success.raw;
                                    const failedCount = channel.request_failed.raw;
                                    const totalCount = successCount + failedCount;
                                    const successRate = totalCount > 0 ? (successCount / totalCount) * 100 : 0;

                                    return (
                                        <div className="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
                                            <span>{t('successRate')}:</span>
                                            <span>{successRate.toFixed(1)}%</span>
                                        </div>
                                    );
                                })()}
                            </div>

                            <div className="flex shrink-0 items-center gap-1 text-right">
                                {mode === 'count' ? (
                                    <div className="flex items-center gap-1 text-sm font-medium tabular-nums">
                                        <span className="text-accent">
                                            {channel.request_success.formatted.value}
                                            <span className="text-xs text-muted-foreground">
                                                {channel.request_success.formatted.unit}
                                            </span>
                                        </span>
                                        <span className="text-muted-foreground/40 font-light">/</span>
                                        <span className="text-destructive">
                                            {channel.request_failed.formatted.value}
                                            <span className="text-xs text-muted-foreground">
                                                {channel.request_failed.formatted.unit}
                                            </span>
                                        </span>
                                    </div>
                                ) : mode === 'tokens' ? (
                                    <span className="font-semibold text-base">
                                        {channel.total_token.formatted.value}
                                        <span className="text-xs text-muted-foreground">
                                            {channel.total_token.formatted.unit}
                                        </span>
                                    </span>
                                ) : (
                                    <span className="font-semibold text-base">
                                        {channel.total_cost.formatted.value}
                                        <span className="text-xs text-muted-foreground">
                                            {channel.total_cost.formatted.unit}
                                        </span>
                                    </span>
                                )}
                            </div>
                        </div>
                    );
                })}
            </div>
        );
    };

    const renderAPIKeyList = (apiKeys: APIKeyRankData[], isLoading: boolean) => {
        if (isLoading) {
            return (
                <div className="flex min-h-44 flex-col items-center justify-center rounded-lg border border-border bg-card py-8 text-muted-foreground">
                    <Loader2 className="mb-3 h-10 w-10 animate-spin opacity-50" />
                    <p className="text-sm">{t('loading')}</p>
                </div>
            );
        }

        if (apiKeys.length === 0) {
            return (
                <div className="flex min-h-44 flex-col items-center justify-center rounded-lg border border-border bg-card py-8 text-muted-foreground">
                    <TrendingUp className="mb-3 h-10 w-10 opacity-30" strokeWidth={1.5} />
                    <p className="text-sm">{t('noData')}</p>
                </div>
            );
        }

        return (
            <div className="max-h-[196px] space-y-2.5 overflow-y-auto pr-1 [scrollbar-width:thin]">
                {apiKeys.map((apiKey, index) => {
                    const rank = index + 1;
                    const successCount = apiKey.request_success.raw;
                    const failedCount = apiKey.request_failed.raw;
                    const totalCount = successCount + failedCount;
                    const successRate = totalCount > 0 ? (successCount / totalCount) * 100 : 0;

                    return (
                        <div
                            key={apiKey.api_key_id}
                            className={cn(
                                'group flex items-center gap-3 rounded-lg border border-border bg-card p-2.5 transition-[transform,border-color] duration-200 hover:-translate-y-0.5 hover:border-border/80',
                            )}
                        >
                            <div className={cn('flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border text-sm font-bold', getRankToneClass(rank))}>
                                {rank === 1 ? <Trophy className="h-4 w-4" strokeWidth={1.5} /> : rank}
                            </div>

                            <div className="min-w-0 flex-1">
                                <p className="truncate text-sm font-semibold">{apiKey.name}</p>
                                <div className="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
                                    <span>{t('successRate')}:</span>
                                    <span>{successRate.toFixed(1)}%</span>
                                </div>
                            </div>

                            <div className="flex shrink-0 items-center gap-1 text-right text-sm font-medium tabular-nums">
                                <span className="text-accent">
                                    {apiKey.request_success.formatted.value}
                                    <span className="text-xs text-muted-foreground">
                                        {apiKey.request_success.formatted.unit}
                                    </span>
                                </span>
                                <span className="text-muted-foreground/40 font-light">/</span>
                                <span className="text-destructive">
                                    {apiKey.request_failed.formatted.value}
                                    <span className="text-xs text-muted-foreground">
                                        {apiKey.request_failed.formatted.unit}
                                    </span>
                                </span>
                            </div>
                        </div>
                    );
                })}
            </div>
        );
    };

    return (
        <div className="relative h-full rounded-lg border border-border bg-card p-3.5 text-card-foreground md:p-4">
            <Tabs
                value={rankSortMode}
                onValueChange={(value) => {
                    setRankSortMode(value as RankSortMode);
                }}
            >
                <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                    <div className="inline-flex w-max items-center gap-2 rounded-md border border-primary/10 bg-card px-2.5 py-1 text-xs font-medium text-primary">
                        <Leaf className="h-3.5 w-3.5" strokeWidth={1.5} />
                        <span>{t('title')}</span>
                    </div>
                    <div className="w-full overflow-x-auto lg:w-auto">
                        <TabsList className="flex w-max min-w-full flex-nowrap rounded-lg border border-border bg-card p-1 lg:min-w-0">
                            <TabsTrigger value="cost">{t('sortByCost')}</TabsTrigger>
                            <TabsTrigger value="count">{t('sortByCount')}</TabsTrigger>
                            <TabsTrigger value="tokens">{t('sortByTokens')}</TabsTrigger>
                            <TabsTrigger value="key-usage">{t('sortByKeyUsage')}</TabsTrigger>
                        </TabsList>
                    </div>
                </div>
                <TabsContents className="relative mt-4">
                    <TabsContent value="cost">
                        {renderChannelList(rankedByCost, 'cost', isChannelListLoading)}
                    </TabsContent>
                    <TabsContent value="count">
                        {renderChannelList(rankedByCount, 'count', isChannelListLoading)}
                    </TabsContent>
                    <TabsContent value="tokens">
                        {renderChannelList(rankedByTokens, 'tokens', isChannelListLoading)}
                    </TabsContent>
                    <TabsContent value="key-usage">
                        {renderAPIKeyList(rankedByKeyUsage, isAPIKeyStatsLoading)}
                    </TabsContent>
                </TabsContents>
            </Tabs>
        </div>
    );
}
