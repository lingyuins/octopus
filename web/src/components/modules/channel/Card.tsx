import {
    MorphingDialog,
    MorphingDialogTrigger,
    MorphingDialogContainer,
    MorphingDialogContent,
} from '@/components/ui/morphing-dialog';
import { CheckCircle2, DollarSign, Globe, GripVertical, Key, Layers, MessageSquare, XCircle } from 'lucide-react';
import { type StatsMetricsFormatted } from '@/api/endpoints/stats';
import { type Channel, useEnableChannel } from '@/api/endpoints/channel';
import { CardContent } from './CardContent';
import { useTranslations } from 'next-intl';
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/animate-ui/components/animate/tooltip';
import { Switch } from '@/components/ui/switch';
import { toast } from '@/components/common/Toast';
import { Badge } from '@/components/ui/badge';
import { getChannelMetricDisplayParts } from './metric-format';

export function Card({ channel, stats, layout = 'grid' }: { channel: Channel; stats: StatsMetricsFormatted; layout?: 'grid' | 'list' | 'compact' }) {
    const t = useTranslations('channel.card');
    const tForm = useTranslations('channel.form');
    const tSections = useTranslations('channel.detail.sections');
    const tMetrics = useTranslations('channel.detail.metrics');
    const enableChannel = useEnableChannel();
    const isListLayout = layout === 'list';
    const isCompactLayout = layout === 'compact';

    const splitModels = (models: string) =>
        models
            .split(',')
            .map((item) => item.trim())
            .filter(Boolean);

    const modelCount = new Set([
        ...splitModels(channel.model),
        ...splitModels(channel.custom_model),
    ]).size;
    const enabledKeyCount = channel.keys.filter((item) => item.enabled && item.channel_key.trim() !== '').length;
    const firstBaseUrl = channel.base_urls?.find((item) => item.url.trim())?.url?.trim() ?? '';
    const successRequests = getChannelMetricDisplayParts(stats.request_success);
    const failedRequests = getChannelMetricDisplayParts(stats.request_failed);

    const handleEnableChange = (checked: boolean) => {
        enableChannel.mutate(
            { id: channel.id, enabled: checked },
            {
                onSuccess: () => {
                    toast.success(checked ? t('toast.enabled') : t('toast.disabled'));
                },
                onError: (error) => {
                    toast.error(error.message);
                },
            }
        );
    };

    // ── Compact layout: single-line row ──────────────────────────────────
    if (isCompactLayout) {
        const successRaw = stats.request_success.raw;
        const failedRaw = stats.request_failed.raw;
        const totalRaw = successRaw + failedRaw;
        const successRate = totalRaw > 0 ? ((successRaw / totalRaw) * 100).toFixed(1) : '0.0';
        const failRate = totalRaw > 0 ? ((failedRaw / totalRaw) * 100).toFixed(1) : '0.0';

        return (
            <MorphingDialog>
                <MorphingDialogTrigger
                    className="group w-full rounded-lg border border-border bg-card px-3 py-2 text-card-foreground transition-all duration-150 hover:border-border/80 hover:bg-muted/20 active:scale-[0.998]"
                >
                    <div className="flex items-center gap-2">
                        {/* Status dot - vertically centered across both lines */}
                        <span className={`h-2 w-2 shrink-0 self-center rounded-full ${channel.enabled ? 'bg-emerald-500' : 'bg-destructive'}`} />

                        <div className="min-w-0 flex-1">
                            {/* Line 1: name + key badge */}
                            <div className="flex items-center gap-1.5">
                                <Tooltip side="top" sideOffset={8}>
                                    <TooltipTrigger asChild>
                                        <span className="truncate text-sm font-medium">{channel.name}</span>
                                    </TooltipTrigger>
                                    <TooltipContent key={channel.name}>{channel.name}</TooltipContent>
                                </Tooltip>
                                <Badge variant="secondary" className="shrink-0 rounded-md border border-border bg-muted px-1.5 py-0 text-[0.6rem] tabular-nums">
                                    #{channel.id} · {enabledKeyCount}/{channel.keys.length}
                                </Badge>
                            </div>

                            {/* Line 2: metrics row */}
                            <div className="mt-0.5 flex items-center gap-3 text-xs text-muted-foreground">
                                <span className="tabular-nums">
                                    <MessageSquare className="mr-1 inline-block size-3 text-primary/60" strokeWidth={1.5} />
                                    {stats.request_count.formatted.value}
                                    {stats.request_count.formatted.unit ? <span className="ml-0.5 text-[0.6rem]">{stats.request_count.formatted.unit}</span> : null}
                                </span>
                                <span className="tabular-nums">
                                    <span className="text-emerald-500">{successRate}%</span>
                                    <span className="mx-0.5">/</span>
                                    <span className="text-destructive">{failRate}%</span>
                                </span>
                                <span className="tabular-nums font-medium text-foreground/80">
                                    <DollarSign className="mr-0.5 inline-block size-3 text-primary/60" strokeWidth={1.5} />
                                    {stats.total_cost.formatted.value}
                                    {stats.total_cost.formatted.unit ? <span className="ml-0.5 text-[0.6rem] text-muted-foreground">{stats.total_cost.formatted.unit}</span> : null}
                                </span>
                            </div>
                        </div>

                        {/* Toggle - vertically centered across both lines */}
                        <Switch
                            checked={channel.enabled}
                            onCheckedChange={handleEnableChange}
                            disabled={enableChannel.isPending}
                            onClick={(e) => e.stopPropagation()}
                            onKeyDown={(e) => e.stopPropagation()}
                            className="shrink-0 self-center"
                        />
                    </div>
                </MorphingDialogTrigger>

                <MorphingDialogContainer>
                    <MorphingDialogContent className="relative flex max-h-[calc(100dvh-2rem)] min-h-0 w-[min(100vw-1rem,56rem)] max-w-full flex-col overflow-hidden rounded-xl border border-border bg-card px-4 pt-4 pb-[calc(1rem+env(safe-area-inset-bottom,0px))] text-card-foreground md:px-6 md:py-5">
                        <CardContent channel={channel} stats={stats} />
                    </MorphingDialogContent>
                </MorphingDialogContainer>
            </MorphingDialog>
        );
    }

    // ── Grid / List layout ────────────────────────────────────────────────
    return (
        <MorphingDialog>
            <MorphingDialogTrigger
                className={`group relative flex w-full rounded-xl border border-border bg-card p-4 text-card-foreground transition-all duration-200 hover:border-border/80 hover:shadow-md hover:bg-muted/20 active:scale-[0.995] md:hover:-translate-y-0.5 md:active:translate-y-0 ${isListLayout ? 'min-h-[12rem]' : 'min-h-[18rem]'}`}
            >
                <div className="relative flex w-full flex-col gap-3 sm:gap-4">
                    <header className="flex items-start justify-between gap-2 sm:gap-3">
                        <div className="min-w-0 flex-1 space-y-2 sm:space-y-3">
                            <div className="flex flex-wrap items-center gap-1.5 sm:gap-2">
                                <span className={`h-1.5 w-1.5 sm:h-2 sm:w-2 rounded-full ${channel.enabled ? 'bg-emerald-500' : 'bg-destructive'}`} />
                                <Badge variant="secondary" className="rounded-md border border-border bg-muted px-1.5 sm:px-2 py-0.5 text-[0.65rem] sm:text-xs font-medium">
                                    #{channel.id}
                                </Badge>
                                <Badge variant="secondary" className="rounded-md border border-border bg-muted px-1.5 sm:px-2 py-0.5 text-[0.65rem] sm:text-xs font-medium">
                                    {enabledKeyCount}/{channel.keys.length}
                                </Badge>
                            </div>
                            <div className="flex items-center gap-2">
                                <span
                                    className="inline-flex h-7 w-7 sm:h-8 sm:w-8 shrink-0 items-center justify-center rounded-md border border-dashed border-border/60 bg-muted/40 text-muted-foreground"
                                    aria-hidden="true"
                                >
                                    <GripVertical className="size-3.5 sm:size-4" />
                                </span>
                                <Tooltip side="top" sideOffset={10} align="center">
                                    <TooltipTrigger asChild>
                                        <h3 className="max-w-full truncate text-base sm:text-lg font-semibold tracking-tight">{channel.name}</h3>
                                    </TooltipTrigger>
                                    <TooltipContent key={channel.name}>{channel.name}</TooltipContent>
                                </Tooltip>
                            </div>
                        </div>
                        <div className="flex items-center gap-1.5 sm:gap-2">
                            <Switch
                                checked={channel.enabled}
                                onCheckedChange={handleEnableChange}
                                disabled={enableChannel.isPending}
                                onClick={(e) => e.stopPropagation()}
                                onKeyDown={(e) => e.stopPropagation()}
                            />
                        </div>
                    </header>

                    <div className={`grid gap-3 ${isListLayout ? 'lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1.8fr)]' : 'grid-cols-1'}`}>
                            <div className="relative flex min-h-[6.5rem] flex-col justify-between overflow-hidden rounded-lg border border-border bg-card p-3.5">
                                <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
                                    <Globe className="size-3.5 text-primary/60" strokeWidth={1.5} />
                                    {tSections('baseUrls')}
                                </div>
                                <p className="font-mono text-sm leading-6 text-foreground/80 line-clamp-2 break-all">
                                    {firstBaseUrl || '—'}
                                </p>
                            </div>

                            {isListLayout ? (
                                <dl className="grid grid-cols-2 gap-2 sm:gap-3 lg:grid-cols-4">
                                    <div className="rounded-lg border border-border bg-card p-2.5 sm:p-3">
                                        <dt className="mb-1.5 flex items-center gap-1.5 text-[0.65rem] sm:text-xs text-muted-foreground">
                                            <MessageSquare className="size-3 sm:size-3.5 text-primary/60" strokeWidth={1.5} />
                                            {t('requestCount')}
                                        </dt>
                                        <dd className="text-sm sm:text-base font-semibold tabular-nums">
                                            {stats.request_count.formatted.value}
                                            <span className="ml-0.5 text-[0.65rem] sm:text-[0.7rem] text-muted-foreground">{stats.request_count.formatted.unit}</span>
                                        </dd>
                                    </div>
                                    <div className="rounded-lg border border-border bg-card p-2.5 sm:p-3">
                                        <dt className="mb-1.5 flex items-center gap-1.5 text-[0.65rem] sm:text-xs text-muted-foreground">
                                            <Layers className="size-3 sm:size-3.5 text-primary/60" strokeWidth={1.5} />
                                            {tForm('model')}
                                        </dt>
                                        <dd className="text-sm sm:text-base font-semibold tabular-nums">{modelCount}</dd>
                                    </div>
                                    <div className="rounded-lg border border-border bg-card p-2.5 sm:p-3">
                                        <dt className="mb-1.5 flex items-center gap-1.5 text-[0.65rem] sm:text-xs text-muted-foreground">
                                            <CheckCircle2 className="size-3 sm:size-3.5 text-emerald-500" strokeWidth={1.5} />
                                            {tMetrics('successRequests')}
                                        </dt>
                                        <dd className="text-sm sm:text-base font-semibold tabular-nums">
                                            {successRequests.value}
                                            {successRequests.unit ? (
                                                <span className="ml-0.5 text-[0.65rem] sm:text-[0.7rem] text-muted-foreground">{successRequests.unit}</span>
                                            ) : null}
                                        </dd>
                                    </div>
                                    <div className="rounded-lg border border-border bg-card p-2.5 sm:p-3">
                                        <dt className="mb-1.5 flex items-center gap-1.5 text-[0.65rem] sm:text-xs text-muted-foreground">
                                            <DollarSign className="size-3 sm:size-3.5 text-primary/60" strokeWidth={1.5} />
                                            {t('totalCost')}
                                        </dt>
                                        <dd className="text-sm sm:text-base font-semibold tabular-nums">
                                            {stats.total_cost.formatted.value}
                                            <span className="ml-0.5 text-[0.65rem] sm:text-[0.7rem] text-muted-foreground">{stats.total_cost.formatted.unit}</span>
                                        </dd>
                                    </div>
                                </dl>
                            ) : (
                                <dl className="grid grid-cols-2 gap-2 sm:gap-3">
                                    <div className="rounded-lg border border-border bg-card p-2.5 sm:p-3">
                                        <dt className="mb-1.5 flex items-center gap-1.5 text-[0.65rem] sm:text-xs text-muted-foreground">
                                            <MessageSquare className="size-3 sm:size-3.5 text-primary/60" strokeWidth={1.5} />
                                            {t('requestCount')}
                                        </dt>
                                        <dd className="text-sm sm:text-base font-semibold tabular-nums">
                                            {stats.request_count.formatted.value}
                                            <span className="ml-0.5 text-[0.65rem] sm:text-[0.7rem] text-muted-foreground">{stats.request_count.formatted.unit}</span>
                                        </dd>
                                    </div>
                                    <div className="rounded-lg border border-border bg-card p-2.5 sm:p-3">
                                        <dt className="mb-1.5 flex items-center gap-1.5 text-[0.65rem] sm:text-xs text-muted-foreground">
                                            <DollarSign className="size-3 sm:size-3.5 text-primary/60" strokeWidth={1.5} />
                                            {t('totalCost')}
                                        </dt>
                                        <dd className="text-sm sm:text-base font-semibold tabular-nums">
                                            {stats.total_cost.formatted.value}
                                            <span className="ml-0.5 text-[0.65rem] sm:text-[0.7rem] text-muted-foreground">{stats.total_cost.formatted.unit}</span>
                                        </dd>
                                    </div>
                                    <div className="rounded-lg border border-border bg-card p-2.5 sm:p-3">
                                        <dt className="mb-1.5 flex items-center gap-1.5 text-[0.65rem] sm:text-xs text-muted-foreground">
                                            <Key className="size-3 sm:size-3.5 text-primary/60" strokeWidth={1.5} />
                                            {tSections('keys')}
                                        </dt>
                                        <dd className="text-sm sm:text-base font-semibold tabular-nums">{enabledKeyCount}/{channel.keys.length}</dd>
                                    </div>
                                    <div className="rounded-lg border border-border bg-card p-2.5 sm:p-3">
                                        <dt className="mb-1.5 flex items-center gap-1.5 text-[0.65rem] sm:text-xs text-muted-foreground">
                                            <Layers className="size-3 sm:size-3.5 text-primary/60" strokeWidth={1.5} />
                                            {tForm('model')}
                                        </dt>
                                        <dd className="text-sm sm:text-base font-semibold tabular-nums">{modelCount}</dd>
                                    </div>
                                    <div className="rounded-lg border border-border bg-card p-2.5 sm:p-3">
                                        <dt className="mb-1.5 flex items-center gap-1.5 text-[0.65rem] sm:text-xs text-muted-foreground">
                                            <CheckCircle2 className="size-3 sm:size-3.5 text-emerald-500" strokeWidth={1.5} />
                                            {tMetrics('successRequests')}
                                        </dt>
                                        <dd className="text-sm sm:text-base font-semibold tabular-nums">
                                            {successRequests.value}
                                            {successRequests.unit ? (
                                                <span className="ml-0.5 text-[0.65rem] sm:text-[0.7rem] text-muted-foreground">{successRequests.unit}</span>
                                            ) : null}
                                        </dd>
                                    </div>
                                    <div className="rounded-lg border border-border bg-card p-2.5 sm:p-3">
                                        <dt className="mb-1.5 flex items-center gap-1.5 text-[0.65rem] sm:text-xs text-muted-foreground">
                                            <XCircle className="size-3 sm:size-3.5 text-destructive" strokeWidth={1.5} />
                                            {tMetrics('failedRequests')}
                                        </dt>
                                        <dd className="text-sm sm:text-base font-semibold tabular-nums">
                                            {failedRequests.value}
                                            {failedRequests.unit ? (
                                                <span className="ml-0.5 text-[0.65rem] sm:text-[0.7rem] text-muted-foreground">{failedRequests.unit}</span>
                                            ) : null}
                                        </dd>
                                    </div>
                                </dl>
                            )}
                        </div>
                </div>
            </MorphingDialogTrigger>

            <MorphingDialogContainer>
                <MorphingDialogContent className="relative flex max-h-[calc(100dvh-2rem)] min-h-0 w-[min(100vw-1rem,56rem)] max-w-full flex-col overflow-hidden rounded-xl border border-border bg-card px-4 pt-4 pb-[calc(1rem+env(safe-area-inset-bottom,0px))] text-card-foreground md:px-6 md:py-5">
                    <CardContent channel={channel} stats={stats} />
                </MorphingDialogContent>
            </MorphingDialogContainer>
        </MorphingDialog>
    );
}
