'use client';

import {
    useRemoteSiteList,
    type RemoteSite,
} from '@/api/endpoints/remote-site';
import {
    useCheckInStatus,
    useCheckInHistory,
    useExecuteCheckIn,
    useExecuteCheckInAll,
} from '@/api/endpoints/checkin';
import { LoadingState } from '@/components/common/LoadingState';
import { ErrorState } from '@/components/common/ErrorState';
import { CalendarCheck, RefreshCw, Check, X, Clock } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/common/Toast';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { useTranslations } from 'next-intl';

function statusIcon(status: string) {
    switch (status) {
        case 'success': return <Check className="h-3.5 w-3.5 text-green-500" />;
        case 'already_checked': return <Clock className="h-3.5 w-3.5 text-yellow-500" />;
        case 'failed': return <X className="h-3.5 w-3.5 text-red-500" />;
        default: return null;
    }
}

function statusColor(status: string) {
    switch (status) {
        case 'success': return 'bg-green-500/10 text-green-600';
        case 'already_checked': return 'bg-yellow-500/10 text-yellow-600';
        case 'failed': return 'bg-red-500/10 text-red-600';
        default: return '';
    }
}

function SiteCheckInCard({ site }: { site: RemoteSite }) {
    const t = useTranslations('checkin');
    const { data: todayStatus } = useCheckInStatus(site.id);
    const { data: history } = useCheckInHistory(site.id, 7);
    const executeCheckIn = useExecuteCheckIn();

    const handleCheckIn = () => {
        executeCheckIn.mutate(site.id, {
            onSuccess: (record) => {
                if (record?.status === 'success') {
                    toast.success(`${site.name}: ${t('success')}`);
                } else if (record?.status === 'already_checked') {
                    toast.info(`${site.name}: ${t('alreadyChecked')}`);
                } else {
                    toast.error(`${site.name}: ${record?.message || t('failed')}`);
                }
            },
            onError: (err) => toast.error(`${site.name}: ${err.message}`),
        });
    };

    const checkedToday = todayStatus?.status === 'success' || todayStatus?.status === 'already_checked';

    return (
        <div className="rounded-lg border bg-card p-4 flex flex-col gap-3">
            <div className="flex items-center justify-between">
                <div className="font-medium truncate">{site.name}</div>
                <Button
                    variant={checkedToday ? 'outline' : 'default'}
                    size="sm"
                    onClick={handleCheckIn}
                    disabled={executeCheckIn.isPending}
                >
                    {checkedToday ? (
                        <><Check className="h-3.5 w-3.5 mr-1" />{t('done')}</>
                    ) : (
                        <>{t('checkIn')}</>
                    )}
                </Button>
            </div>
            <div className="text-xs text-muted-foreground">
                <Badge variant="outline" className="text-xs font-normal">{site.site_type}</Badge>
            </div>
            {history && history.length > 0 && (
                <div className="flex gap-1 flex-wrap">
                    {history.slice(0, 7).map((record) => (
                        <div
                            key={record.id}
                            className={cn('flex items-center gap-1 rounded px-1.5 py-0.5 text-xs', statusColor(record.status))}
                            title={`${record.check_in_date}: ${record.message || record.status}`}
                        >
                            {statusIcon(record.status)}
                            <span>{record.check_in_date.slice(5)}</span>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}

export function CheckInPanel() {
    const { data: sites, isLoading, isError, refetch } = useRemoteSiteList();
    const executeAll = useExecuteCheckInAll();
    const t = useTranslations('checkin');

    const handleCheckInAll = () => {
        executeAll.mutate(undefined, {
            onSuccess: (records) => {
                const successCount = records?.filter(r => r.status === 'success').length ?? 0;
                toast.success(t('allDone', { count: successCount }));
            },
            onError: (err) => toast.error(err.message),
        });
    };

    if (isLoading) return <LoadingState />;
    if (isError) return <ErrorState onRetry={refetch} />;

    const enabledSites = sites?.filter(s => s.enabled) ?? [];

    return (
        <div className="flex flex-col gap-4">
            <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2 min-w-0">
                    <CalendarCheck className="h-5 w-5 shrink-0" />
                    <h2 className="text-lg font-semibold truncate">{t('title')}</h2>
                    <Badge variant="secondary" className="shrink-0">{enabledSites.length}</Badge>
                </div>
                <Button
                    variant="outline"
                    size="sm"
                    onClick={handleCheckInAll}
                    disabled={executeAll.isPending}
                    className="shrink-0"
                >
                    <RefreshCw className={cn('h-4 w-4 sm:mr-1', executeAll.isPending && 'animate-spin')} />
                    <span className="hidden sm:inline">{t('checkInAll')}</span>
                </Button>
            </div>

            {enabledSites.length > 0 ? (
                <div className="grid gap-3 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
                    {enabledSites.map((site) => (
                        <SiteCheckInCard key={site.id} site={site} />
                    ))}
                </div>
            ) : (
                <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
                    <CalendarCheck className="h-12 w-12 mb-3 opacity-50" />
                    <p>{t('empty')}</p>
                </div>
            )}
        </div>
    );
}
