'use client';

import { useCallback, useMemo } from 'react';
import { useChannelList } from '@/api/endpoints/channel';
import { useLogs } from '@/api/endpoints/log';
import { LogCard } from './Item';
import { Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';

/**
 * 日志页面组件
 * - 初始加载 pageSize 条历史日志
 * - SSE 实时推送新日志
 * - 滚动自动加载更多
 */
export function Log() {
    const t = useTranslations('log');
    const { logs, hasMore, isLoading, isLoadingMore, loadMore } = useLogs();
    const { data: channels = [] } = useChannelList();
    const channelNameById = useMemo(() => {
        const map = new Map<number, string>();
        for (const item of channels) {
            map.set(item.raw.id, item.raw.name);
        }
        return map;
    }, [channels]);

    const canLoadMore = hasMore && !isLoading && !isLoadingMore && logs.length > 0;
    const handleReachEnd = useCallback(() => {
        if (!canLoadMore) return;
        void loadMore();
    }, [canLoadMore, loadMore]);

    const footer = useMemo(() => {
        if (hasMore && (isLoading || isLoadingMore)) {
            return (
                <div className="flex justify-center py-4">
                    <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                </div>
            );
        }
        if (!hasMore && logs.length > 0) {
            return (
                <div className="flex justify-center py-4">
                    <span className="text-sm text-muted-foreground">{t('list.noMore')}</span>
                </div>
            );
        }
        return null;
    }, [hasMore, isLoading, isLoadingMore, logs.length, t]);

    return (
        <div className="flex h-full min-h-0 flex-col gap-4 overflow-y-auto overscroll-contain rounded-t-xl pb-24 md:pb-4">
            {isLoading && logs.length === 0 ? (
                <div className="flex min-h-[18rem] items-center justify-center rounded-xl border border-border/35 bg-card">
                    <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                </div>
            ) : logs.length === 0 ? (
                <div className="flex min-h-[18rem] items-center justify-center rounded-xl border border-dashed border-border/35 bg-card px-6 py-6 text-center">
                    <p className="text-sm text-muted-foreground">{t('list.empty')}</p>
                </div>
            ) : (
                <div className="min-h-0 flex-1">
                    <VirtualizedGrid
                        items={logs}
                        layout="list"
                        columns={{ default: 1 }}
                        estimateItemHeight={80}
                        overscan={8}
                        getItemKey={(log) => `log-${log.id}`}
                        renderItem={(log) => <LogCard log={log} channelNameById={channelNameById} />}
                        footer={footer}
                        onReachEnd={handleReachEnd}
                        reachEndEnabled={canLoadMore}
                        reachEndOffset={2}
                    />
                </div>
            )}
        </div>
    );
}
