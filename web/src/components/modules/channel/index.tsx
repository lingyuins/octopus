'use client';

import { useChannelList, useLastSyncTime, useSyncChannel } from '@/api/endpoints/channel';
import { Card } from './Card';
import { useToolbarViewOptionsStore } from '@/components/modules/toolbar';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';
import { useSearchableList, useChannelFilter, createChannelFilterPredicate } from '@/hooks/use-searchable-list';
import type { Channel as ChannelModel } from '@/api/endpoints/channel';
import type { StatsMetricsFormatted } from '@/api/endpoints/stats';

import { LoadingState } from '@/components/common/LoadingState';
import { ErrorState } from '@/components/common/ErrorState';
import { Radio, RefreshCw, Clock3 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/common/Toast';

type ChannelListItem = {
    raw: ChannelModel;
    formatted: StatsMetricsFormatted;
};

export function Channel() {
    const { data: channelsData, isLoading, isError, refetch } = useChannelList();
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

    return (
        <section className="relative flex h-full min-h-0 flex-col" aria-label={pageKey}>
            <div className="relative flex h-full min-h-0 flex-col gap-4 rounded-xl border border-border bg-card p-3 text-card-foreground md:p-4">
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

                <div className="relative min-h-0 flex-1">
                    {isLoading ? (
                        <LoadingState />
                    ) : isError ? (
                        <ErrorState onRetry={() => refetch()} />
                    ) : visibleChannels.length > 0 ? (
                        <VirtualizedGrid
                            items={visibleChannels}
                            layout={layout}
                            columns={{ default: 1, sm: 2, md: 2, lg: 3 }}
                            estimateItemHeight={232}
                            getItemKey={(item) => `channel-${item.raw.id}`}
                            renderItem={(item) => <Card channel={item.raw} stats={item.formatted} layout={layout} />}
                        />
                    ) : (
                        <div className="flex h-full min-h-[18rem] flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border bg-card">
                            <Radio className="h-12 w-12 text-muted-foreground/30" strokeWidth={1.5} />
                            <p className="text-sm text-muted-foreground">{t('empty')}</p>
                        </div>
                    )}
                </div>
            </div>
        </section>
    );
}
