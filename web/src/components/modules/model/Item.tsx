'use client';

import { memo, useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import { Pencil, Trash2, ArrowDownToLine, ArrowUpFromLine, ChevronDown, CircleCheckBig, Gauge, KeyRound, Orbit, RadioTower, Waves } from 'lucide-react';
import { motion, AnimatePresence } from 'motion/react';
import { useTranslations } from 'next-intl';
import { useUpdateModel, useDeleteModel, type ModelMarketItem } from '@/api/endpoints/model';
import { getModelIcon } from '@/lib/model-icons';
import { toast } from '@/components/common/Toast';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/animate-ui/components/animate/tooltip';
import { ModelDeleteOverlay, ModelEditOverlay } from './ItemOverlays';
import { cn } from '@/lib/utils';
import { createPortal } from 'react-dom';
import { CopyIconButton } from '@/components/common/CopyButton';
import { formatAverageLatency, type LatencyUnitMode } from './latency-format';
import { useSettingStore } from '@/stores/setting';

interface ModelItemProps {
    model: ModelMarketItem;
    layout?: 'grid' | 'list' | 'compact';
    latencyUnit?: LatencyUnitMode;
}

export const ModelItem = memo(function ModelItem({ model, layout = 'grid', latencyUnit = 'auto' }: ModelItemProps) {
    const t = useTranslations('model');
    const { chinaMode, exchangeRate } = useSettingStore();
    const isListLayout = layout === 'list';
    const [isEditOpen, setIsEditOpen] = useState(false);
    const [confirmDelete, setConfirmDelete] = useState(false);
    const [isExpanded, setIsExpanded] = useState(false);
    const [overlayRect, setOverlayRect] = useState<{ top: number; left: number; width: number } | null>(null);
    const instanceId = useId();
    const editLayoutId = `edit-btn-${model.name}-${instanceId}`;
    const deleteLayoutId = `delete-btn-${model.name}-${instanceId}`;
    const cardRef = useRef<HTMLElement | null>(null);
    const editButtonRef = useRef<HTMLButtonElement | null>(null);
    const editOverlayRef = useRef<HTMLDivElement | null>(null);
    const [editValues, setEditValues] = useState(() => ({
        input: model.input.toString(),
        output: model.output.toString(),
        cache_read: model.cache_read.toString(),
        cache_write: model.cache_write.toString(),
    }));

    const updateModel = useUpdateModel();
    const deleteModel = useDeleteModel();

    const { Avatar: ModelAvatar, color: brandColor, label: providerLabel } = useMemo(() => getModelIcon(model.name), [model.name]);
    const requestCount = model.request_success + model.request_failed;
    const visibleChannelTags = useMemo(() => model.channels.slice(0, isListLayout ? 4 : 3), [isListLayout, model.channels]);
    const hiddenChannelTagCount = Math.max(0, model.channels.length - visibleChannelTags.length);
    const successRateLabel = requestCount > 0 ? `${(model.success_rate * 100).toFixed(2)}%` : '—';
    const latencyLabel = formatAverageLatency(model.average_latency_ms, requestCount, latencyUnit);
    const specimenMetricClassName = cn(
        'inline-flex items-center gap-1.5 rounded-lg border border-border/25 bg-card px-2 py-1.5 text-xs sm:gap-2 sm:px-3 sm:py-2 sm:text-sm',
        isListLayout ? 'min-w-[10rem] flex-1' : '',
    );

    const updateOverlayRect = useCallback(() => {
        const card = cardRef.current;
        if (!card) return;
        const rect = card.getBoundingClientRect();
        setOverlayRect((prev) => {
            if (prev && prev.top === rect.top && prev.left === rect.left && prev.width === rect.width) {
                return prev;
            }
            return { top: rect.top, left: rect.left, width: rect.width };
        });
    }, []);

    const closeEdit = useCallback(() => {
        setIsEditOpen(false);
    }, []);

    const handleEditClick = () => {
        setConfirmDelete(false);
        setEditValues({
            input: model.input.toString(),
            output: model.output.toString(),
            cache_read: model.cache_read.toString(),
            cache_write: model.cache_write.toString(),
        });
        // Ensure first open already has anchor geometry so layout animation can run.
        updateOverlayRect();
        setIsEditOpen(true);
    };

    const handleCancelEdit = () => {
        closeEdit();
    };

    const handleSaveEdit = () => {
        updateModel.mutate({
            name: model.name,
            input: parseFloat(editValues.input) || 0,
            output: parseFloat(editValues.output) || 0,
            cache_read: parseFloat(editValues.cache_read) || 0,
            cache_write: parseFloat(editValues.cache_write) || 0,
        }, {
            onSuccess: () => {
                closeEdit();
                toast.success(t('toast.updated'));
            },
            onError: (error) => {
                toast.error(t('toast.updateFailed'), { description: error.message });
            }
        });
    };

    const handleDeleteClick = () => {
        closeEdit();
        setConfirmDelete(true);
    };
    const handleCancelDelete = () => setConfirmDelete(false);
    const handleConfirmDelete = () => {
        deleteModel.mutate(model.name, {
            onSuccess: () => {
                setConfirmDelete(false);
                toast.success(t('toast.deleted'));
            },
            onError: (error) => {
                setConfirmDelete(false);
                toast.error(t('toast.deleteFailed'), { description: error.message });
            }
        });
    };

    useEffect(() => {
        if (!isEditOpen) return;

        const handlePointerDown = (event: PointerEvent) => {
            const target = event.target as Node | null;
            if (!target) return;
            if (editOverlayRef.current?.contains(target)) return;
            if (editButtonRef.current?.contains(target)) return;
            closeEdit();
        };

        const handleKeyDown = (event: KeyboardEvent) => {
            if (event.key === 'Escape') closeEdit();
        };

        updateOverlayRect();
        window.addEventListener('resize', updateOverlayRect);
        window.addEventListener('scroll', updateOverlayRect, true);
        document.addEventListener('pointerdown', handlePointerDown);
        document.addEventListener('keydown', handleKeyDown);

        return () => {
            window.removeEventListener('resize', updateOverlayRect);
            window.removeEventListener('scroll', updateOverlayRect, true);
            document.removeEventListener('pointerdown', handlePointerDown);
            document.removeEventListener('keydown', handleKeyDown);
        };
    }, [isEditOpen, updateOverlayRect, closeEdit]);

    const shouldRenderEditPortal = isEditOpen || overlayRect !== null;

    return (
        <article
            ref={cardRef}
            className={cn(
                'group relative overflow-hidden rounded-xl border border-border/35 bg-card p-3 text-card-foreground transition-[border-color] duration-300 hover:border-primary/18 sm:p-4 md:hover:-translate-y-0.5 md:p-5',
                (isEditOpen || confirmDelete) && 'z-50'
            )}
        >
            <div className="relative flex flex-col gap-3 sm:gap-4">
                <div className="flex items-start gap-3 sm:gap-4">
                    <div className="grid h-12 w-12 shrink-0 place-items-center rounded-lg border border-border/25 bg-card sm:h-16 sm:w-16">
                        <div className="[&>svg]:!h-9 [&>svg]:!w-9 sm:[&>svg]:!h-12 sm:[&>svg]:!w-12">
                            <ModelAvatar size={48} />
                        </div>
                    </div>

                    <div className="min-w-0 flex-1 space-y-2 sm:space-y-3">
                        <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-start sm:justify-between sm:gap-3">
                            <div className="min-w-0 space-y-1.5 sm:space-y-2">
                                <div className="inline-flex items-center gap-1.5 rounded-full border border-primary/12 bg-card px-2.5 py-0.5 text-[0.62rem] font-semibold text-primary sm:gap-2 sm:px-3 sm:py-1 sm:text-[0.68rem]">
                                    <Orbit className="size-3 sm:size-3.5" />
                                    {providerLabel}
                                </div>
                                <Tooltip side="top" sideOffset={10} align="start">
                                    <TooltipTrigger className="block max-w-full truncate text-left text-base font-semibold leading-tight text-card-foreground sm:text-lg">
                                        {model.name}
                                    </TooltipTrigger>
                                    <TooltipContent key={model.name}>{model.name}</TooltipContent>
                                </Tooltip>
                                <div className="flex flex-wrap items-center gap-1.5 text-sm text-muted-foreground sm:gap-2">
                                    <span className="inline-flex items-center gap-1.5 rounded-full border border-border/25 bg-card px-2.5 py-0.5 text-[0.68rem] sm:gap-2 sm:px-3 sm:py-1 sm:text-xs">
                                        <Waves className="size-3 text-primary sm:size-3.5" />
                                        {t('card.requests')}: {requestCount.toLocaleString()}
                                    </span>
                                </div>
                            </div>

                            <div
                                className={cn(
                                    'flex shrink-0 items-center gap-1.5 sm:gap-2',
                                    (isEditOpen || confirmDelete) && 'invisible pointer-events-none'
                                )}
                            >
                                <CopyIconButton
                                    text={model.name}
                                    className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-border/25 bg-card text-muted-foreground transition-colors hover:bg-card hover:text-foreground sm:h-10 sm:w-10"
                                    copyIconClassName="size-3.5 sm:size-4"
                                    checkIconClassName="size-3.5 sm:size-4"
                                />
                                <button
                                    type="button"
                                    onClick={() => setIsExpanded((prev) => !prev)}
                                    aria-label={isExpanded ? t('card.collapse') : t('card.expand')}
                                    aria-expanded={isExpanded}
                                    className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-border/25 bg-card text-muted-foreground transition-colors hover:bg-card hover:text-foreground sm:h-10 sm:w-10"
                                    title={isExpanded ? t('card.collapse') : t('card.expand')}
                                >
                                    <ChevronDown className={cn('size-3.5 transition-transform sm:size-4', isExpanded && 'rotate-180')} />
                                </button>
                                <motion.button
                                    ref={editButtonRef}
                                    layoutId={editLayoutId}
                                    type="button"
                                    onClick={handleEditClick}
                                    aria-label={t('card.edit')}
                                    disabled={isEditOpen || confirmDelete}
                                    className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-border/25 bg-card text-muted-foreground transition-colors hover:bg-card hover:text-foreground sm:h-10 sm:w-10 disabled:opacity-50"
                                    title={t('card.edit')}
                                >
                                    <Pencil className="size-3.5 sm:size-4" />
                                </motion.button>
                                <motion.button
                                    layoutId={deleteLayoutId}
                                    type="button"
                                    onClick={handleDeleteClick}
                                    aria-label={t('card.delete')}
                                    disabled={isEditOpen || confirmDelete}
                                    className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-destructive/15 bg-destructive/8 text-destructive transition-colors hover:bg-destructive hover:text-destructive-foreground sm:h-10 sm:w-10 disabled:opacity-50"
                                    title={t('card.delete')}
                                >
                                    <Trash2 className="size-3.5 sm:size-4" />
                                </motion.button>
                            </div>
                        </div>

                        <div className={cn('grid gap-1.5 text-sm text-muted-foreground sm:gap-2', isListLayout ? 'grid-cols-2 xl:grid-cols-4' : 'grid-cols-2')}>
                            <div className={specimenMetricClassName}>
                                <RadioTower className="size-3.5 sm:size-4" style={{ color: brandColor }} />
                                <span className="truncate">{t('card.channels')}</span>
                                <span className="ml-auto tabular-nums text-foreground">{model.channel_count}</span>
                            </div>
                            <div className={specimenMetricClassName}>
                                <KeyRound className="size-3.5 sm:size-4" style={{ color: brandColor }} />
                                <span className="truncate">{t('card.keys')}</span>
                                <span className="ml-auto tabular-nums text-foreground">{model.enabled_key_count}</span>
                            </div>
                            <div className={specimenMetricClassName}>
                                <Gauge className="size-3.5 sm:size-4" style={{ color: brandColor }} />
                                <span className="truncate">{t('card.averageLatency')}</span>
                                <span className="ml-auto tabular-nums text-foreground">{latencyLabel}</span>
                            </div>
                            <div className={specimenMetricClassName}>
                                <CircleCheckBig className="size-3.5 sm:size-4" style={{ color: brandColor }} />
                                <span className="truncate">{t('card.successRate')}</span>
                                <span className="ml-auto tabular-nums text-foreground">{successRateLabel}</span>
                            </div>
                        </div>

                        <div className="flex flex-wrap gap-1.5 sm:gap-2">
                            <span className="rounded-full border border-primary/12 bg-card px-2 py-0.5 text-[0.68rem] font-medium text-foreground sm:px-2.5 sm:py-1 sm:text-xs">
                                {providerLabel}
                            </span>
                            {visibleChannelTags.map((channel) => (
                                <span
                                    key={`${model.name}-${channel.channel_id}`}
                                    className="rounded-full border border-border/25 bg-card px-2 py-0.5 text-[0.68rem] text-muted-foreground sm:px-2.5 sm:py-1 sm:text-xs"
                                >
                                    {channel.channel_name}
                                </span>
                            ))}
                            {hiddenChannelTagCount > 0 ? (
                                <span className="rounded-full border border-border/25 bg-card px-2 py-0.5 text-[0.68rem] text-muted-foreground sm:px-2.5 sm:py-1 sm:text-xs">
                                    +{hiddenChannelTagCount}
                                </span>
                            ) : null}
                        </div>
                    </div>
                </div>

                <AnimatePresence initial={false}>
                    {isExpanded ? (
                        <motion.div
                            initial={{ height: 0, opacity: 0 }}
                            animate={{ height: 'auto', opacity: 1 }}
                            exit={{ height: 0, opacity: 0 }}
                            transition={{ duration: 0.24 }}
                            className="overflow-hidden"
                        >
                            <div className="space-y-4 border-t border-border/20 pt-4">
                                <div className={cn('grid gap-3', isListLayout ? 'xl:grid-cols-3' : 'grid-cols-1 md:grid-cols-2')}>
                                    <div className="rounded-lg border border-border/25 bg-card p-3 sm:p-4">
                                        <h4 className="text-sm font-medium text-foreground">{t('detail.pricing')}</h4>
                                        <div className="mt-3 space-y-2 text-sm text-muted-foreground">
                                            <div className="flex items-center justify-between gap-2 rounded-lg bg-card px-2.5 py-2 sm:gap-3 sm:px-3">
                                                <span className="inline-flex shrink-0 items-center gap-1.5">
                                                    <ArrowDownToLine className="size-3.5" style={{ color: brandColor }} />
                                                    {t('card.inputCache')}
                                                </span>
                                                <span className="min-w-0 truncate text-right tabular-nums text-foreground">
                                                    {chinaMode
                                                        ? `${(model.input * exchangeRate).toFixed(2)}/${(model.cache_read * exchangeRate).toFixed(2)}¥`
                                                        : `${model.input.toFixed(2)}/${model.cache_read.toFixed(2)}$`
                                                    }
                                                </span>
                                            </div>
                                            <div className="flex items-center justify-between gap-2 rounded-lg bg-card px-2.5 py-2 sm:gap-3 sm:px-3">
                                                <span className="inline-flex shrink-0 items-center gap-1.5">
                                                    <ArrowUpFromLine className="size-3.5" style={{ color: brandColor }} />
                                                    {t('card.outputCache')}
                                                </span>
                                                <span className="min-w-0 truncate text-right tabular-nums text-foreground">
                                                    {chinaMode
                                                        ? `${(model.output * exchangeRate).toFixed(2)}/${(model.cache_write * exchangeRate).toFixed(2)}¥`
                                                        : `${model.output.toFixed(2)}/${model.cache_write.toFixed(2)}$`
                                                    }
                                                </span>
                                            </div>
                                        </div>
                                    </div>

                                    <div className="rounded-lg border border-border/25 bg-card p-3 sm:p-4">
                                        <h4 className="text-sm font-medium text-foreground">{t('detail.runtime')}</h4>
                                        <div className="mt-3 grid grid-cols-2 gap-2 text-sm text-muted-foreground">
                                            <div className="rounded-lg bg-card px-2.5 py-2 sm:px-3">
                                                <div className="text-xs sm:text-sm">{t('detail.requestSuccess')}</div>
                                                <div className="mt-1 tabular-nums text-sm font-medium text-foreground sm:text-base">{model.request_success.toLocaleString()}</div>
                                            </div>
                                            <div className="rounded-lg bg-card px-2.5 py-2 sm:px-3">
                                                <div className="text-xs sm:text-sm">{t('detail.requestFailed')}</div>
                                                <div className="mt-1 tabular-nums text-sm font-medium text-foreground sm:text-base">{model.request_failed.toLocaleString()}</div>
                                            </div>
                                            <div className="rounded-lg bg-card px-2.5 py-2 sm:px-3">
                                                <div className="text-xs sm:text-sm">{t('card.averageLatency')}</div>
                                                <div className="mt-1 tabular-nums text-sm font-medium text-foreground sm:text-base">{latencyLabel}</div>
                                            </div>
                                            <div className="rounded-lg bg-card px-2.5 py-2 sm:px-3">
                                                <div className="text-xs sm:text-sm">{t('card.successRate')}</div>
                                                <div className="mt-1 tabular-nums text-sm font-medium text-foreground sm:text-base">{successRateLabel}</div>
                                            </div>
                                        </div>
                                    </div>
                                </div>

                                <div className="rounded-lg border border-border/25 bg-card p-3 sm:p-4">
                                    <div className="flex flex-wrap items-center justify-between gap-2 sm:gap-3">
                                        <h4 className="text-sm font-medium text-foreground">{t('detail.channels')}</h4>
                                        <span className="text-xs text-muted-foreground">{model.channels.length} {t('card.channels')}</span>
                                    </div>
                                    <div className="mt-3 grid gap-2">
                                        {model.channels.length === 0 ? (
                                            <div className="rounded-lg bg-card px-3 py-2 text-sm text-muted-foreground">
                                                {t('detail.noChannels')}
                                            </div>
                                        ) : (
                                            model.channels.map((channel) => (
                                                <div key={`${model.name}-detail-${channel.channel_id}`} className="flex flex-col gap-2 rounded-lg bg-card px-3 py-2.5 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between sm:gap-3 sm:py-3">
                                                    <div className="min-w-0">
                                                        <div className="truncate text-sm font-medium text-foreground">{channel.channel_name}</div>
                                                        <div className="mt-0.5 text-xs text-muted-foreground">ID {channel.channel_id}</div>
                                                    </div>
                                                    <div className="flex flex-wrap items-center gap-1.5 text-xs sm:gap-2">
                                                        <span className={cn(
                                                            'rounded-full border px-2 py-0.5 sm:px-2.5 sm:py-1',
                                                            channel.enabled
                                                                ? 'border-emerald-500/20 bg-emerald-500/10 text-emerald-700'
                                                                : 'border-border/25 bg-card text-muted-foreground'
                                                        )}>
                                                            {channel.enabled ? t('detail.enabled') : t('detail.disabled')}
                                                        </span>
                                                        <span className="rounded-full border border-border/25 bg-card px-2 py-0.5 text-muted-foreground sm:px-2.5 sm:py-1">
                                                            {channel.enabled_key_count} {t('detail.keyCount')}
                                                        </span>
                                                    </div>
                                                </div>
                                            ))
                                        )}
                                    </div>
                                </div>
                            </div>
                        </motion.div>
                    ) : null}
                </AnimatePresence>
            </div>
            <AnimatePresence>
                {confirmDelete && (
                    <ModelDeleteOverlay
                        layoutId={deleteLayoutId}
                        isPending={deleteModel.isPending}
                        onCancel={handleCancelDelete}
                        onConfirm={handleConfirmDelete}
                    />
                )}
            </AnimatePresence>

            {shouldRenderEditPortal && typeof document !== 'undefined'
                ? createPortal(
                    <AnimatePresence onExitComplete={() => setOverlayRect(null)}>
                        {isEditOpen && overlayRect && (
                            <div
                                ref={editOverlayRef}
                                className="fixed z-[90]"
                                style={{
                                    top: `${overlayRect.top}px`,
                                    left: `${overlayRect.left}px`,
                                    width: `${overlayRect.width}px`,
                                }}
                            >
                                <div className="relative">
                                    <ModelEditOverlay
                                        layoutId={editLayoutId}
                                        modelName={model.name}
                                        brandColor={brandColor}
                                        editValues={editValues}
                                        isPending={updateModel.isPending}
                                        onChange={setEditValues}
                                        onCancel={handleCancelEdit}
                                        onSave={handleSaveEdit}
                                    />
                                </div>
                            </div>
                        )}
                    </AnimatePresence>,
                    document.body
                )
                : null}
        </article>
    );
});
