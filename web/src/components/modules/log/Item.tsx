'use client';

import { useMemo, useState, useEffect } from 'react';
import { Clock, Cpu, Zap, AlertCircle, ArrowDownToLine, ArrowUpFromLine, DollarSign, ArrowRight, ArrowDown, Send, MessageSquare, Loader2, RotateCw, ChevronDown, ChevronUp, Pin, KeyRound } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { motion, AnimatePresence } from 'motion/react';
import JsonView from '@uiw/react-json-view';
import { githubDarkTheme } from '@uiw/react-json-view/githubDark';
import { githubLightTheme } from '@uiw/react-json-view/githubLight';
import { useTheme } from 'next-themes';
import { type RelayLog, type ChannelAttempt, useLogDetail } from '@/api/endpoints/log';
import { getModelIcon } from '@/lib/model-icons';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { endpointTypeLabelKey } from '@/components/modules/group/utils';
import { resolveLogDisplayFields } from './display';
import { CopyIconButton } from '@/components/common/CopyButton';
import {
    MorphingDialog,
    MorphingDialogTrigger,
    MorphingDialogContainer,
    MorphingDialogContent,
    MorphingDialogClose,
    MorphingDialogTitle,
    MorphingDialogDescription,
    useMorphingDialog,
} from '@/components/ui/morphing-dialog';
import { Tooltip, TooltipContent, TooltipTrigger, TooltipProvider } from '@/components/animate-ui/components/animate/tooltip';

function formatTime(timestamp: number): string {
    const date = new Date(timestamp * 1000);
    return date.toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
    });
}

function formatDuration(ms: number): string {
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
}

interface RetryBadgeWithTooltipProps {
    channelName: string;
    brandColor: string;
    attempts: ChannelAttempt[];
    channelNameById?: ReadonlyMap<number, string>;
}

function RetryBadgeWithTooltip({ channelName, brandColor, attempts, channelNameById }: RetryBadgeWithTooltipProps) {
    const t = useTranslations('log.card');

    return (
        <Tooltip>
            <TooltipTrigger asChild>
                <Badge
                    variant="secondary"
                    className="shrink-0 text-xs px-1.5 py-0 cursor-help"
                    style={{ backgroundColor: `${brandColor}15`, color: brandColor }}
                >
                    <RotateCw className="size-3 mr-1 opacity-80" />
                    {channelName}
                </Badge>
            </TooltipTrigger>
            <TooltipContent className="border bg-card p-2 min-w-[280px] max-w-[calc(100vw-2rem)] rounded-xl flex flex-col gap-1">
                {attempts.map((attempt, idx) => (
                    <div key={idx} className="flex flex-col w-full">
                        <div className="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted/50 transition-colors">
                            <Badge
                                className={cn(
                                    "h-5 shrink-0 px-1.5 text-[10px] font-bold uppercase shadow-none border-0",
                                    attempt.status === 'success'
                                        ? "bg-primary/15 text-primary"
                                        : "bg-destructive/15 text-destructive"
                                )}
                            >
                                {attempt.status === 'success' ? t('success') : t('failed')}
                            </Badge>
                            <div className="flex min-w-0 flex-col flex-1">
                                <span className="truncate text-xs font-semibold text-foreground">
                                    {attempt.channel_name?.trim() || channelNameById?.get(attempt.channel_id) || `Channel #${attempt.channel_id}`}
                                </span>
                                <span className="text-[10px] text-muted-foreground">
                                    {attempt.model_name} • {formatDuration(attempt.duration)}
                                </span>
                            </div>
                        </div>
                        {
                            idx < attempts.length - 1 && (
                                <div className="flex justify-center py-0.5">
                                    <ArrowDown className="size-3 text-muted-foreground/30" />
                                </div>
                            )
                        }
                    </div>
                ))}
            </TooltipContent>
        </Tooltip >
    );
}

function DeferredJsonContent({ content, fallbackText }: { content: string | undefined; fallbackText: string }) {
    const { resolvedTheme } = useTheme();
    const { isOpen } = useMorphingDialog();
    const [shouldRender, setShouldRender] = useState(false);

    const parsed = useMemo(() => {
        if (!content) return { isJson: false, data: null };
        try {
            return { isJson: true, data: JSON.parse(content) };
        } catch {
            return { isJson: false, data: content };
        }
    }, [content]);

    useEffect(() => {
        if (isOpen) {
            const timer = setTimeout(() => setShouldRender(true), 300);
            return () => clearTimeout(timer);
        }
    }, [isOpen]);

    if (!isOpen) {
        if (shouldRender) setShouldRender(false);
        return null;
    }

    if (!content) {
        return (
            <div className="h-full min-h-0 overflow-auto overscroll-contain">
                <pre className="p-4 text-xs text-muted-foreground whitespace-pre-wrap wrap-break-word leading-relaxed">
                    {fallbackText}
                </pre>
            </div>
        );
    }

    return (
        <AnimatePresence mode="wait">
            {!shouldRender ? (
                <motion.div
                    key="loading"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.15 }}
                    className="flex h-full min-h-0 items-center justify-center overflow-auto overscroll-contain p-4"
                >
                    <Loader2 className="h-5 w-5 text-muted-foreground animate-spin" />
                </motion.div>
            ) : parsed.isJson ? (
                <motion.div
                    key="json"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className="h-full min-h-0 overflow-auto overscroll-contain p-4"
                >
                    <JsonView
                        value={parsed.data as object}
                        style={{
                            ...(resolvedTheme === 'dark' ? githubDarkTheme : githubLightTheme),
                            fontSize: '12px',
                            fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
                            backgroundColor: 'transparent',
                        }}
                        displayDataTypes={false}
                        displayObjectSize={false}
                        collapsed={false}
                    />
                </motion.div>
            ) : (
                <motion.pre
                    key="text"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className="h-full min-h-0 overflow-auto overscroll-contain p-4 text-xs text-muted-foreground whitespace-pre-wrap wrap-break-word font-mono leading-relaxed"
                >
                    {content}
                </motion.pre>
            )}
        </AnimatePresence>
    );
}

export function LogCard({ log, channelNameById }: { log: RelayLog; channelNameById?: ReadonlyMap<number, string> }) {
    const t = useTranslations('log.card');
    const tGroup = useTranslations('group');
    const { detail, isLoading: isDetailLoading, fetchDetail, reset: resetDetail } = useLogDetail();
    const hasError = !!log.error;
    const hasMultipleAttempts = log.attempts && log.attempts.length > 1;
    const [isDiagnosticExpanded, setIsDiagnosticExpanded] = useState(false);
    const displayFields = useMemo(() => resolveLogDisplayFields(log, detail, channelNameById), [channelNameById, detail, log]);
    const { Avatar: ModelAvatar, color: brandColor } = useMemo(
        () => getModelIcon(displayFields.actualModelName),
        [displayFields.actualModelName]
    );
    const requestAPIKeyName = displayFields.requestAPIKeyName;
    const cacheReadTokens = displayFields.cacheReadTokens;
    const semanticCacheHit = displayFields.semanticCacheHit;
    const effectiveInputTokens = Math.max(0, log.input_tokens - cacheReadTokens);
    const inputLabel = cacheReadTokens > 0 ? t('realInput') : t('input');
    const displayChannelName = displayFields.channelName || '-';
    const displayEndpointType = useMemo(() => {
        const rawEndpointType = displayFields.endpointType;
        if (!rawEndpointType) return '-';

        const labelKey = endpointTypeLabelKey(rawEndpointType);
        return labelKey ? tGroup(labelKey) : rawEndpointType;
    }, [displayFields.endpointType, tGroup]);
    const displayActualModelName = displayFields.actualModelName || '-';
    const displayRequestModelName = displayFields.requestModelName || log.request_model_name;

    const requestContent = detail?.request_content;
    const responseContent = detail?.response_content;

    return (
        <TooltipProvider>
            <MorphingDialog onOpen={() => fetchDetail(log.id)} onClose={resetDetail}>
                <MorphingDialogTrigger
                    className={cn(
                        "rounded-xl border bg-card w-full text-left",
                        hasError ? "border-destructive/40" : "border-border",
                    )}
                >
                    <div className={cn("p-4 grid grid-cols-[auto_1fr] gap-4", hasError ? "items-start" : "items-center")}>
                        <ModelAvatar size={40} />
                        <div className="min-w-0 flex flex-col gap-3">
                            <div className="flex min-w-0 flex-wrap items-center gap-2 text-sm md:flex-nowrap">
                                <span className="min-w-0 max-w-full font-semibold text-card-foreground truncate md:max-w-[32%]" title={displayRequestModelName}>
                                    {displayRequestModelName}
                                </span>
                                <ArrowRight className="size-3.5 shrink-0 text-muted-foreground/50" />
                                <Badge
                                    variant="secondary"
                                    className="max-w-full shrink-0 text-xs px-1.5 py-0"
                                    style={{ backgroundColor: `${brandColor}15`, color: brandColor }}
                                    title={displayEndpointType}
                                >
                                    <span className="block max-w-[10rem] truncate">{displayEndpointType}</span>
                                </Badge>
                                <ArrowRight className="size-3.5 shrink-0 text-muted-foreground/50" />
                                {hasMultipleAttempts ? (
                                    <RetryBadgeWithTooltip
                                        channelName={displayChannelName}
                                        brandColor={brandColor}
                                        attempts={log.attempts!}
                                        channelNameById={channelNameById}
                                    />
                                ) : (
                                    <Badge
                                        variant="secondary"
                                        className="max-w-full shrink-0 text-xs px-1.5 py-0"
                                        style={{ backgroundColor: `${brandColor}15`, color: brandColor }}
                                        title={displayChannelName}
                                    >
                                        <span className="block max-w-[18rem] truncate">{displayChannelName}</span>
                                    </Badge>
                                )}
                                <span className="min-w-0 text-muted-foreground truncate md:flex-1" title={displayActualModelName}>
                                    {displayActualModelName}
                                </span>
                                {log.attempts?.some(a => a.sticky) && (
                                    <Pin className="size-3.5 shrink-0 text-amber-500" />
                                )}
                            </div>
                            <div className="grid grid-cols-2 md:grid-cols-7 gap-x-4 gap-y-2 text-xs tabular-nums text-muted-foreground">
                                <div className="flex items-center gap-1.5">
                                    <Clock className="size-3.5 shrink-0" style={{ color: brandColor }} />
                                    <span>{formatTime(log.time)}</span>
                                </div>
                                {requestAPIKeyName && (
                                    <div className="flex items-center gap-1.5">
                                        <KeyRound className="size-3.5 shrink-0 text-orange-500" />
                                        <span className="truncate" title={requestAPIKeyName}>
                                            {requestAPIKeyName}
                                        </span>
                                    </div>
                                )}
                                <div className="flex items-center gap-1.5">
                                    <Zap className="size-3.5 shrink-0 text-amber-500" />
                                    <span>{t('firstToken')} {formatDuration(log.ftut)}</span>
                                </div>
                                <div className="flex items-center gap-1.5">
                                    <Cpu className="size-3.5 shrink-0 text-blue-500" />
                                    <span>{t('totalTime')} {formatDuration(log.use_time)}</span>
                                </div>
                                <div className="flex items-center gap-1.5">
                                    <ArrowDownToLine className="size-3.5 shrink-0 text-green-500" />
                                    <span>{inputLabel} {effectiveInputTokens.toLocaleString()}</span>
                                </div>
                                {semanticCacheHit && (
                                    <div className="flex items-center gap-1.5">
                                        <ArrowDownToLine className="size-3.5 shrink-0 text-cyan-500" />
                                        <span>{t('semanticCacheHit')}</span>
                                    </div>
                                )}
                                {cacheReadTokens > 0 && (
                                    <div className="flex items-center gap-1.5">
                                        <ArrowDownToLine className="size-3.5 shrink-0 text-teal-500" />
                                        <span>{t('cacheHit')} {cacheReadTokens.toLocaleString()}</span>
                                    </div>
                                )}
                                <div className="flex items-center gap-1.5">
                                    <ArrowUpFromLine className="size-3.5 shrink-0 text-purple-500" />
                                    <span>{t('output')} {log.output_tokens.toLocaleString()}</span>
                                </div>
                                <div className="flex items-center gap-1.5">
                                    <DollarSign className="size-3.5 shrink-0 text-emerald-500" />
                                    <span className="font-medium text-emerald-600 dark:text-emerald-400">
                                        {t('cost')} {Number(log.cost).toFixed(6)}
                                    </span>
                                </div>
                            </div>
                            {hasError && (
                                <div className="p-2.5 rounded-xl bg-destructive/10 border border-destructive/20 overflow-hidden">
                                    <p className="text-xs text-destructive line-clamp-2">{log.error}</p>
                                </div>
                            )}
                        </div>
                    </div>
                </MorphingDialogTrigger>

                <MorphingDialogContainer>
                    <MorphingDialogContent className="relative flex max-h-[calc(100dvh-2rem)] min-h-0 w-[calc(100vw-2rem)] flex-col overflow-hidden rounded-xl bg-card px-6 py-4 text-card-foreground md:w-[95vw] md:max-w-7xl">
                        <MorphingDialogClose className="top-4 right-5 text-muted-foreground hover:text-foreground transition-colors" />
                        <MorphingDialogTitle className="flex items-center gap-2 mb-3 text-sm">
                            <ModelAvatar size={28} />
                            <span className="font-semibold text-card-foreground">{displayRequestModelName}</span>
                            <ArrowRight className="size-3.5 text-muted-foreground/50" />
                            <Badge
                                variant="secondary"
                                className="max-w-full shrink-0 text-xs px-1.5 py-0"
                                style={{ backgroundColor: `${brandColor}15`, color: brandColor }}
                                title={displayEndpointType}
                            >
                                <span className="block max-w-[10rem] truncate">{displayEndpointType}</span>
                            </Badge>
                            <ArrowRight className="size-3.5 text-muted-foreground/50" />
                            {hasMultipleAttempts ? (
                                <RetryBadgeWithTooltip
                                    channelName={displayChannelName}
                                    brandColor={brandColor}
                                    attempts={log.attempts!}
                                    channelNameById={channelNameById}
                                />
                            ) : (
                                <Badge
                                    variant="secondary"
                                    className="max-w-full text-xs px-1.5 py-0"
                                    style={{ backgroundColor: `${brandColor}15`, color: brandColor }}
                                    title={displayChannelName}
                                >
                                    <span className="block max-w-[18rem] truncate">{displayChannelName}</span>
                                </Badge>
                            )}
                            <span className="min-w-0 flex-1 truncate text-muted-foreground" title={displayActualModelName}>{displayActualModelName}</span>
                            {log.attempts?.some(a => a.sticky) && (
                                <Pin className="size-3.5 shrink-0 text-amber-500" />
                            )}
                        </MorphingDialogTitle>

                        <MorphingDialogDescription className="flex min-h-0 flex-1 overflow-hidden">
                            <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-hidden">
                                {(hasError || hasMultipleAttempts) && (
                                    <div className={cn(
                                        "flex-initial min-h-0 flex flex-col rounded-2xl border overflow-hidden max-h-[40%]",
                                        hasError
                                            ? "bg-destructive/5 border-destructive/20"
                                            : "bg-secondary/30 border-border/50"
                                    )}>
                                        <div
                                            className={cn(
                                                "flex items-center gap-2 px-3 py-2.5 shrink-0 cursor-pointer select-none hover:bg-muted/50 transition-colors",
                                                hasError && "hover:bg-destructive/10"
                                            )}
                                            onClick={() => setIsDiagnosticExpanded(!isDiagnosticExpanded)}
                                        >
                                            {hasError ? (
                                                <AlertCircle className="size-4 text-destructive" />
                                            ) : (
                                                <RotateCw className="size-4 text-muted-foreground" />
                                            )}
                                            <span className={cn(
                                                "text-sm font-medium",
                                                hasError ? "text-destructive" : "text-secondary-foreground"
                                            )}>
                                                {hasError ? t('errorInfo') : t('retryDetails')}
                                            </span>
                                            <div className="ml-auto flex items-center gap-2">
                                                {hasMultipleAttempts && (
                                                    <Badge
                                                        variant="outline"
                                                        className={cn(
                                                            "text-xs border-0",
                                                            hasError
                                                                ? "bg-destructive/10 text-destructive"
                                                                : "bg-secondary text-secondary-foreground"
                                                        )}
                                                    >
                                                        {log.total_attempts || log.attempts!.length} {t('attempts')}
                                                    </Badge>
                                                )}
                                                {isDiagnosticExpanded ? (
                                                    <ChevronUp className="size-4 text-muted-foreground" />
                                                ) : (
                                                    <ChevronDown className="size-4 text-muted-foreground" />
                                                )}
                                            </div>
                                        </div>

                                        <AnimatePresence initial={false}>
                                            {isDiagnosticExpanded && (
                                                <motion.div
                                                    initial={{ height: 0, opacity: 0 }}
                                                    animate={{ height: "auto", opacity: 1 }}
                                                    exit={{ height: 0, opacity: 0 }}
                                                    transition={{ duration: 0.2, ease: "easeInOut" }}
                                                    className="overflow-hidden flex flex-col min-h-0"
                                                >
                                                    <div className="flex-1 overflow-auto p-2.5 md:p-3 flex flex-col gap-4">
                                                        {hasError && (
                                                            <div className="relative pl-1">
                                                                <div className="absolute right-0 top-0">
                                                                    <CopyIconButton
                                                                        text={log.error ?? ''}
                                                                        className="p-1 rounded-md text-destructive/60 hover:text-destructive hover:bg-destructive/10 transition-colors"
                                                                        copyIconClassName="size-4"
                                                                        checkIconClassName="size-4"
                                                                    />
                                                                </div>
                                                                <p className="text-sm text-destructive whitespace-pre-wrap wrap-break-word pr-8 leading-relaxed">
                                                                    {log.error}
                                                                </p>
                                                            </div>
                                                        )}

                                                        {hasMultipleAttempts && (
                                                            <div className="flex flex-col gap-2">
                                                                {log.attempts!.map((attempt, idx) => (
                                                                    <div
                                                                        key={idx}
                                                                        className={cn(
                                                                            "text-xs p-2.5 rounded-xl border transition-colors flex flex-col gap-2",
                                                                            attempt.status === 'success'
                                                                                ? "bg-primary/5 border-primary/20 hover:bg-primary/10"
                                                                                : "bg-destructive/5 border-destructive/20 hover:bg-destructive/10"
                                                                        )}
                                                                    >
                                                                        <div className="flex items-center gap-2">
                                                                            <span className="font-semibold text-foreground">
                                                                                {attempt.channel_name?.trim() || channelNameById?.get(attempt.channel_id) || `Channel #${attempt.channel_id}`}
                                                                            </span>
                                                                            <span className="text-muted-foreground">
                                                                                ({attempt.model_name})
                                                                            </span>
                                                                            <span className="ml-auto text-muted-foreground tabular-nums font-mono">
                                                                                {formatDuration(attempt.duration)}
                                                                            </span>
                                                                        </div>
                                                                        {attempt.msg && (
                                                                            <div className="text-destructive/90 pl-2 border-l-2 border-destructive/30 text-[11px] leading-relaxed">
                                                                                {attempt.msg}
                                                                            </div>
                                                                        )}
                                                                    </div>
                                                                ))}
                                                            </div>
                                                        )}
                                                    </div>
                                                </motion.div>
                                            )}
                                        </AnimatePresence>
                                    </div>
                                )}
                                <div className="min-h-0 flex-1 overflow-hidden pb-1">
                                    <div className="grid h-full min-h-0 grid-cols-1 gap-4 md:grid-cols-2">
                                        <div className="flex min-h-0 flex-col rounded-2xl border border-border bg-muted/30 overflow-hidden">
                                            <div className="flex items-center gap-2 px-3 md:px-4 py-2.5 md:py-3 border-b border-border bg-muted/50 shrink-0">
                                                <Send className="size-4 text-green-500" />
                                                <span className="text-sm font-medium text-card-foreground">{t('requestContent')}</span>
                                                <Badge variant="secondary" className="ml-auto text-xs">
                                                    {log.input_tokens.toLocaleString()} {t('tokens')}
                                                </Badge>
                                            </div>
                                            <div className="min-h-0 flex-1 overflow-hidden">
                                                {isDetailLoading ? (
                                                    <div className="p-4 flex items-center justify-center h-full">
                                                        <Loader2 className="h-5 w-5 text-muted-foreground animate-spin" />
                                                    </div>
                                                ) : (
                                                    <DeferredJsonContent content={requestContent} fallbackText={t('noRequestContent')} />
                                                )}
                                            </div>
                                        </div>
                                        <div className="flex min-h-0 flex-col rounded-2xl border border-border bg-muted/30 overflow-hidden">
                                            <div className="flex items-center gap-2 px-3 md:px-4 py-2.5 md:py-3 border-b border-border bg-muted/50 shrink-0">
                                                <MessageSquare className="size-4 text-purple-500" />
                                                <span className="text-sm font-medium text-card-foreground">{t('responseContent')}</span>
                                                <Badge variant="secondary" className="ml-auto text-xs">
                                                    {log.output_tokens.toLocaleString()} {t('tokens')}
                                                </Badge>
                                            </div>
                                            <div className="min-h-0 flex-1 overflow-hidden">
                                                {isDetailLoading ? (
                                                    <div className="p-4 flex items-center justify-center h-full">
                                                        <Loader2 className="h-5 w-5 text-muted-foreground animate-spin" />
                                                    </div>
                                                ) : (
                                                    <DeferredJsonContent content={responseContent} fallbackText={t('noResponseContent')} />
                                                )}
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </MorphingDialogDescription>

                        <div className="shrink-0 border-t border-border/50 pt-3 text-xs text-muted-foreground">
                            <div className="flex flex-wrap items-center gap-3 md:gap-4">
                            <div className="flex items-center gap-1.5">
                                <Clock className="size-3.5" style={{ color: brandColor }} />
                                <span className="tabular-nums">{formatTime(log.time)}</span>
                            </div>
                            {requestAPIKeyName && (
                                <div className="flex min-w-0 items-center gap-1.5">
                                    <KeyRound className="size-3.5 shrink-0 text-orange-500" />
                                    <span className="truncate" title={requestAPIKeyName}>
                                        {requestAPIKeyName}
                                    </span>
                                </div>
                            )}
                            <div className="flex items-center gap-1.5">
                                <Zap className="size-3.5 text-amber-500" />
                                <span>{t('firstTokenTime')}: {formatDuration(log.ftut)}</span>
                            </div>
                            <div className="flex items-center gap-1.5">
                                <Cpu className="size-3.5 text-blue-500" />
                                <span>{t('totalTime')}: {formatDuration(log.use_time)}</span>
                            </div>
                            {cacheReadTokens > 0 && (
                                <div className="flex items-center gap-1.5">
                                    <ArrowDownToLine className="size-3.5 text-teal-500" />
                                    <span>{t('cacheHit')}: {cacheReadTokens.toLocaleString()}</span>
                                </div>
                            )}
                            {semanticCacheHit && (
                                <div className="flex items-center gap-1.5">
                                    <ArrowDownToLine className="size-3.5 text-cyan-500" />
                                    <span>{t('semanticCacheHit')}</span>
                                </div>
                            )}
                            <div className="flex items-center gap-1.5">
                                <DollarSign className="size-3.5 text-emerald-500" />
                                <span className="font-medium text-emerald-600 dark:text-emerald-400">
                                    {t('cost')}: {Number(log.cost).toFixed(6)}
                                </span>
                            </div>
                            </div>
                        </div>
                    </MorphingDialogContent>
                </MorphingDialogContainer>
            </MorphingDialog>
        </TooltipProvider>
    );
}
