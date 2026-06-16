'use client';

import { useEffect, useMemo } from 'react';
import { useChannelGroupList, useChannelList, useLastSyncTime, useSyncChannel } from '@/api/endpoints/channel';
import { Card } from './Card';
import { useToolbarViewOptionsStore } from '@/components/modules/toolbar/view-options-store';
import { useSearchableList, useChannelFilter, createChannelFilterPredicate } from '@/hooks/use-searchable-list';
import type { Channel as ChannelModel, ChannelGroup } from '@/api/endpoints/channel';
import type { StatsMetricsFormatted } from '@/api/endpoints/stats';

import { LoadingState } from '@/components/common/LoadingState';
import { ErrorState } from '@/components/common/ErrorState';
import { Radio, RefreshCw, Clock3, Layers } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/common/Toast';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { getChannelGroupDisplayName } from './GroupManager';

type ChannelListItem = {
    raw: ChannelModel;
    formatted: StatsMetricsFormatted;
};

export function Channel() {
    const { data: channelsData, isLoading, isError, refetch } = useChannelList();
    const { data: channelGroupsData = [] } = useChannelGroupList();
    const { data: lastSyncTime } = useLastSyncTime();
    const syncChannel = useSyncChannel();
    const pageKey = 'channel' as const;
    const layout = useToolbarViewOptionsStore((s) => s.getLayout(pageKey));
    const selectedGroupId = useToolbarViewOptionsStore((s) => s.selectedChannelGroupId);
    const setSelectedGroupId = useToolbarViewOptionsStore((s) => s.setSelectedChannelGroupId);
    const filter = useChannelFilter();
    const t = useTranslations('channel');
    const settingT = useTranslations('setting');
    const defaultGroupName = t('groupManager.defaultName');

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

    const activeGroup = useMemo(() => {
        if (channelGroups.length === 0) {
            return null;
        }
        return channelGroups.find((group) => group.id === selectedGroupId) ?? channelGroups[0];
    }, [channelGroups, selectedGroupId]);

    useEffect(() => {
        if (activeGroup && selectedGroupId !== activeGroup.id) {
            setSelectedGroupId(activeGroup.id);
        }
    }, [activeGroup, selectedGroupId, setSelectedGroupId]);

    const activeGroupChannels = useMemo(() => {
        if (!activeGroup) {
            return channelsData;
        }
        return (channelsData ?? []).filter((item) => item.raw.group_id === activeGroup.id);
    }, [activeGroup, channelsData]);

    const { visibleItems: visibleChannels } = useSearchableList<ChannelListItem>({
        data: activeGroupChannels,
        pageKey,
        filter,
        getItemId: (item) => item.raw.id,
        getItemName: (item) => item.raw.name,
        filterPredicate: (item, f) => createChannelFilterPredicate(f as 'all' | 'enabled' | 'disabled')(item.raw),
    });

    const channelGridClassName = layout === 'compact'
        ? 'flex flex-col gap-1'
        : layout === 'list'
            ? 'grid grid-cols-1 gap-4'
            : 'grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3';

    return (
        <section className="relative flex h-full min-h-0 flex-col overflow-y-auto overscroll-contain rounded-t-xl pb-3 md:pb-4" aria-label={pageKey}>
            <div className="relative flex flex-col gap-4 rounded-xl border border-border bg-card p-3 text-card-foreground md:p-4">
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

                <div className="relative">
                    {isLoading ? (
                        <LoadingState />
                    ) : isError ? (
                        <ErrorState onRetry={() => refetch()} />
                    ) : (channelsData?.length ?? 0) > 0 ? (
                        <section className="rounded-xl border border-border/30 bg-card/70 p-3 md:p-4">
                            {activeGroup ? (
                                <header className="mb-3 flex flex-wrap items-center gap-2">
                                    <h3 className="text-sm font-semibold text-card-foreground">
                                        {getChannelGroupDisplayName(activeGroup, defaultGroupName)}
                                    </h3>
                                    {activeGroup.is_default ? (
                                        <Badge variant="secondary" className="rounded-full">
                                            {t('groupManager.defaultBadge')}
                                        </Badge>
                                    ) : null}
                                    <Badge variant="secondary" className="rounded-full">
                                        {t('groupManager.visibleCount', { count: visibleChannels.length })}
                                    </Badge>
                                </header>
                            ) : null}

                            {visibleChannels.length > 0 ? (
                                <div className={cn(channelGridClassName, 'pr-1')}>
                                    {visibleChannels.map((item) => (
                                        <Card key={`channel-${item.raw.id}`} channel={item.raw} stats={item.formatted} layout={layout} />
                                    ))}
                                </div>
                            ) : (
                                <div className="flex min-h-[10rem] flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-border/40 bg-muted/20 px-4 py-6 text-center">
                                    <Layers className="h-8 w-8 text-muted-foreground/40" strokeWidth={1.5} />
                                    <p className="text-sm text-muted-foreground">{t('groupManager.emptyGroup')}</p>
                                </div>
                            )}
                        </section>
                    ) : (
                        <div className="flex min-h-[18rem] flex-col items-center justify-center gap-4 rounded-xl border border-dashed border-border/40 bg-muted/10 px-6 py-8 text-center">
                            <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted/30">
                                <Radio className="h-8 w-8 text-muted-foreground/40" strokeWidth={1.5} />
                            </div>
                            <p className="max-w-xs text-sm leading-relaxed text-muted-foreground">{t('empty')}</p>
                        </div>
                    )}
                </div>
            </div>
        </section>
    );
}
