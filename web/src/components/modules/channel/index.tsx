'use client';

import { useMemo } from 'react';
import { useChannelGroupList, useChannelList, useLastSyncTime, useSyncChannel } from '@/api/endpoints/channel';
import { Card } from './Card';
import { useToolbarViewOptionsStore } from '@/components/modules/toolbar';
import { useSearchableList, useChannelFilter, createChannelFilterPredicate } from '@/hooks/use-searchable-list';
import type { Channel as ChannelModel, ChannelGroup } from '@/api/endpoints/channel';
import type { StatsMetricsFormatted } from '@/api/endpoints/stats';

import { LoadingState } from '@/components/common/LoadingState';
import { ErrorState } from '@/components/common/ErrorState';
import { Radio, RefreshCw, Clock3 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/common/Toast';
import { Badge } from '@/components/ui/badge';
import { ChannelGroupManager } from './GroupManager';

type ChannelListItem = {
    raw: ChannelModel;
    formatted: StatsMetricsFormatted;
};

export function Channel() {
    const { data: channelsData, isLoading, isError, refetch } = useChannelList();
    const { data: channelGroupsData = [], isLoading: isGroupLoading, isError: isGroupError } = useChannelGroupList();
    const { data: lastSyncTime } = useLastSyncTime();
    const syncChannel = useSyncChannel();
    const pageKey = 'channel' as const;
    const layout = useToolbarViewOptionsStore((s) => s.getLayout(pageKey));
    const filter = useChannelFilter();
    const t = useTranslations('channel');
    const settingT = useTranslations('setting');

    const formatLastSyncTime = (timeStr: string | undefined) => {
        if (!timeStr) return settingT('llmSync.neverSynced');
        const date = new Date(timeStr);
        if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1) {
            return settingT('llmSync.neverSynced');
        }
        return date.toLocaleString();
    };

    const handleManualSync = () => {
        syncChannel.mutate(undefined, {
            onSuccess: () => {
                toast.success(settingT('llmSync.syncSuccess'));
            },
            onError: () => {
                toast.error(settingT('llmSync.syncFailed'));
            },
        });
    };

    const { visibleItems: visibleChannels } = useSearchableList<ChannelListItem>({
        data: channelsData,
        pageKey,
        filter,
        getItemId: (item) => item.raw.id,
        getItemName: (item) => item.raw.name,
        filterPredicate: (item, f) => createChannelFilterPredicate(f as 'all' | 'enabled' | 'disabled')(item.raw),
    });

    const channelGroups = useMemo<ChannelGroup[]>(() => {
        if (channelGroupsData.length > 0) {
            return [...channelGroupsData].sort((a, b) => {
                if (a.is_default !== b.is_default) {
                    return a.is_default ? -1 : 1;
                }
                if (a.created_at !== b.created_at) {
                    return a.created_at - b.created_at;
                }
                return a.id - b.id;
            });
        }

        const fallbackIDs = Array.from(new Set((channelsData ?? []).map((item) => item.raw.group_id))).filter((id) => id > 0);
        return fallbackIDs.map((id, index) => ({
            id,
            name: index === 0 ? t('groupManager.fallbackName') : t('groupManager.fallbackNameWithID', { id }),
            is_default: index === 0,
            created_at: index,
            updated_at: index,
        }));
    }, [channelGroupsData, channelsData, t]);

    const channelCountByGroup = useMemo(() => {
        const counts = new Map<number, number>();
        for (const item of channelsData ?? []) {
            counts.set(item.raw.group_id, (counts.get(item.raw.group_id) ?? 0) + 1);
        }
        return counts;
    }, [channelsData]);

    const groupedVisibleChannels = useMemo(() => {
        const groups = channelGroups.map((group) => ({ group, items: [] as ChannelListItem[] }));
        const groupMap = new Map(groups.map((entry) => [entry.group.id, entry]));

        for (const item of visibleChannels) {
            const entry = groupMap.get(item.raw.group_id);
            if (entry) {
                entry.items.push(item);
                continue;
            }
            const fallback = groups[0];
            if (fallback) {
                fallback.items.push(item);
            }
        }

        return groups;
    }, [channelGroups, visibleChannels]);

    const channelGridClassName = layout === 'list'
        ? 'grid grid-cols-1 gap-4'
        : 'grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3';

    return (
        <section className="relative flex h-full min-h-0 flex-col overflow-y-auto overscroll-contain rounded-t-xl pb-24 md:pb-4" aria-label={pageKey}>
            <div className="relative flex min-h-full flex-col gap-4 rounded-xl border border-border bg-card p-3 text-card-foreground md:p-4">
                <ChannelGroupManager
                    groups={channelGroups}
                    channelCountByGroup={channelCountByGroup}
                    isLoading={isGroupLoading}
                    isError={isGroupError}
                />

                <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-end">
                    <div className="flex items-center gap-2 rounded-lg border-border/30 bg-card px-3 py-2 text-xs text-muted-foreground sm:text-sm">
                        <Clock3 className="h-4 w-4 text-primary" />
                        <span className="truncate">{settingT('llmSync.lastSync')}: {formatLastSyncTime(lastSyncTime)}</span>
                    </div>
                    <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={handleManualSync}
                        disabled={syncChannel.isPending}
                        className="h-10 rounded-lg border-border/30 bg-card px-3.5"
                    >
                        <RefreshCw className={`mr-2 h-4 w-4 ${syncChannel.isPending ? 'animate-spin' : ''}`} />
                        {syncChannel.isPending ? settingT('llmSync.manualSync.syncing') : settingT('llmSync.manualSync.button')}
                    </Button>
                </div>

                <div className="relative flex-1">
                    {isLoading ? (
                        <LoadingState />
                    ) : isError ? (
                        <ErrorState onRetry={() => refetch()} />
                    ) : (channelsData?.length ?? 0) > 0 ? (
                        <div className="space-y-4 pr-1">
                            {groupedVisibleChannels.map(({ group, items }) => (
                                <section key={group.id} className="rounded-xl border border-border/30 bg-card/70 p-3 md:p-4">
                                    <header className="mb-3 flex flex-wrap items-center gap-2">
                                        <h3 className="text-sm font-semibold text-card-foreground">{group.name}</h3>
                                        {group.is_default ? (
                                            <Badge variant="secondary" className="rounded-full">
                                                {t('groupManager.defaultBadge')}
                                            </Badge>
                                        ) : null}
                                        <Badge variant="secondary" className="rounded-full">
                                            {t('groupManager.visibleCount', { count: items.length })}
                                        </Badge>
                                    </header>

                                    {items.length > 0 ? (
                                        <div className={channelGridClassName}>
                                            {items.map((item) => (
                                                <Card key={`channel-${item.raw.id}`} channel={item.raw} stats={item.formatted} layout={layout} />
                                            ))}
                                        </div>
                                    ) : (
                                        <div className="flex min-h-[8rem] items-center justify-center rounded-lg border border-dashed border-border/30 bg-card text-sm text-muted-foreground">
                                            {t('groupManager.emptyGroup')}
                                        </div>
                                    )}
                                </section>
                            ))}
                        </div>
                    ) : (
                        <div className="flex min-h-[18rem] flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border bg-card px-4 py-6 text-center">
                            <Radio className="h-12 w-12 text-muted-foreground/30" strokeWidth={1.5} />
                            <p className="text-sm text-muted-foreground">{t('empty')}</p>
                        </div>
                    )}
                </div>
            </div>
        </section>
    );
}
