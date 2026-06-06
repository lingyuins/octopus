'use client';

import {
    useAnnouncementList,
    useRefreshAllAnnouncements,
    type SiteAnnouncement,
} from '@/api/endpoints/announcement';
import { LoadingState } from '@/components/common/LoadingState';
import { ErrorState } from '@/components/common/ErrorState';
import { Bell, RefreshCw, Megaphone } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/common/Toast';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { useTranslations } from 'next-intl';

function AnnouncementCard({ item }: { item: SiteAnnouncement }) {
    return (
        <div className="rounded-lg border bg-card text-card-foreground shadow-sm p-4 flex flex-col gap-2">
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                    <Megaphone className="h-4 w-4 text-muted-foreground" />
                    <span className="text-sm font-medium">Site #{item.remote_site_id}</span>
                </div>
                <span className="text-xs text-muted-foreground">
                    {new Date(item.fetched_at).toLocaleString()}
                </span>
            </div>
            <div className="text-sm whitespace-pre-wrap leading-relaxed max-h-48 overflow-y-auto">
                {item.content}
            </div>
        </div>
    );
}

export function AnnouncementPanel() {
    const { data: announcements, isLoading, isError, refetch } = useAnnouncementList();
    const refreshAll = useRefreshAllAnnouncements();
    const t = useTranslations('announcement');

    const handleRefreshAll = () => {
        refreshAll.mutate(undefined, {
            onSuccess: (data) => toast.success(t('refreshed', { count: data.refreshed })),
            onError: (err) => toast.error(err.message),
        });
    };

    if (isLoading) return <LoadingState />;
    if (isError) return <ErrorState onRetry={refetch} />;

    return (
        <div className="flex flex-col gap-4">
            <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2 min-w-0">
                    <Bell className="h-5 w-5 shrink-0" />
                    <h2 className="text-lg font-semibold truncate">{t('title')}</h2>
                    <Badge variant="secondary" className="shrink-0">{announcements?.length ?? 0}</Badge>
                </div>
                <Button
                    variant="outline"
                    size="sm"
                    onClick={handleRefreshAll}
                    disabled={refreshAll.isPending}
                    className="shrink-0"
                >
                    <RefreshCw className={cn('h-4 w-4 sm:mr-1', refreshAll.isPending && 'animate-spin')} />
                    <span className="hidden sm:inline">{t('refreshAll')}</span>
                </Button>
            </div>

            {announcements && announcements.length > 0 ? (
                <div className="grid gap-3">
                    {announcements.map((item) => (
                        <AnnouncementCard key={item.id} item={item} />
                    ))}
                </div>
            ) : (
                <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
                    <Bell className="h-12 w-12 mb-3 opacity-50" />
                    <p>{t('empty')}</p>
                </div>
            )}
        </div>
    );
}
