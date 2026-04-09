'use client';

import { useCallback, useMemo } from 'react';
import { useLogs } from '@/api/endpoints/log';
import { LogCard } from './Item';
import { Loader2, RefreshCw } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/common/Toast';

/**
 * 日志页面组件
 * - 初始加载 pageSize 条历史日志
 * - SSE 实时推送新日志
 * - 滚动自动加载更多
 */
export function Log() {
    const t = useTranslations('log');
    const { logs, hasMore, isLoading, isLoadingMore, isRefreshing, loadMore, refresh } = useLogs({ pageSize: 10 });

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

    const handleRefresh = useCallback(async () => {
        try {
            await refresh();
            toast.success(t('actions.refreshSuccess'));
        } catch {
            toast.error(t('actions.refreshFailed'));
        }
    }, [refresh, t]);

    return (
        <div className="flex h-full min-h-0 flex-col gap-4">
            <div className="flex justify-end">
                <Button
                    variant="outline"
                    size="sm"
                    onClick={() => void handleRefresh()}
                    disabled={isRefreshing}
                    className="rounded-xl"
                >
                    <RefreshCw className={`mr-2 h-4 w-4 ${isRefreshing ? 'animate-spin' : ''}`} />
                    {t('actions.refresh')}
                </Button>
            </div>

            <div className="min-h-0 flex-1">
                <VirtualizedGrid
                    items={logs}
                    layout="list"
                    columns={{ default: 1 }}
                    estimateItemHeight={80}
                    overscan={8}
                    getItemKey={(log) => `log-${log.id}`}
                    renderItem={(log) => <LogCard log={log} />}
                    footer={footer}
                    onReachEnd={handleReachEnd}
                    reachEndEnabled={canLoadMore}
                    reachEndOffset={2}
                />
            </div>
        </div>
    );
}
