'use client';

import { Check, Loader, Trash2, X } from 'lucide-react';
import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';
import { Input } from '@/components/ui/input';

type EditValues = {
    input: string;
    output: string;
    cache_read: string;
    cache_write: string;
};

type ModelDeleteOverlayProps = {
    layoutId: string;
    isPending: boolean;
    onCancel: () => void;
    onConfirm: () => void;
};

export function ModelDeleteOverlay({
    layoutId,
    isPending,
    onCancel,
    onConfirm,
}: ModelDeleteOverlayProps) {
    const t = useTranslations('model.overlay');
    return (
        <motion.div
            layoutId={layoutId}
            className="absolute inset-0 flex flex-col items-center justify-center gap-2 rounded-lg border border-destructive/20 bg-destructive p-3 sm:flex-row sm:gap-3 sm:p-4"
            transition={{ type: 'spring', stiffness: 400, damping: 30 }}
        >
            <button
                type="button"
                onClick={onCancel}
                className="flex h-9 w-full items-center justify-center gap-1.5 rounded-lg border border-white/15 bg-destructive-foreground/16 px-3 text-sm font-medium text-destructive-foreground transition-all hover:bg-destructive-foreground/24 active:scale-[0.98] sm:h-10 sm:w-auto sm:px-4"
            >
                <X className="size-4" />
                {t('cancel')}
            </button>
            <button
                type="button"
                onClick={onConfirm}
                disabled={isPending}
                className="flex h-9 w-full items-center justify-center gap-1.5 rounded-lg bg-destructive-foreground px-3 text-sm font-medium text-destructive transition-all hover:bg-destructive-foreground/92 active:scale-[0.98] sm:h-10 sm:w-auto sm:px-4 disabled:cursor-not-allowed disabled:opacity-50"
            >
                {isPending ? (
                    <Loader className="size-4 animate-spin" />
                ) : (
                    <Trash2 className="size-4" />
                )}
                {isPending ? t('deleting') : t('confirmDelete')}
            </button>
        </motion.div>
    );
}

type ModelEditOverlayProps = {
    layoutId: string;
    modelName: string;
    brandColor: string;
    editValues: EditValues;
    isPending: boolean;
    onChange: (next: EditValues) => void;
    onCancel: () => void;
    onSave: () => void;
};

export function ModelEditOverlay({
    layoutId,
    modelName,
    brandColor,
    editValues,
    isPending,
    onChange,
    onCancel,
    onSave,
}: ModelEditOverlayProps) {
    const t = useTranslations('model.overlay');
    return (
        <motion.div
            layoutId={layoutId}
            className="absolute inset-x-0 top-0 z-20 flex flex-col overflow-hidden rounded-xl border border-border/35 bg-card p-3 text-card-foreground sm:p-5"
            transition={{ type: 'spring', stiffness: 400, damping: 30 }}
        >
            <div className="relative">
                <div className="mb-2 inline-flex items-center rounded-full border border-primary/12 bg-card px-2.5 py-0.5 text-[0.62rem] font-semibold text-primary sm:mb-3 sm:px-3 sm:py-1 sm:text-[0.68rem]">
                    {t('save')}
                </div>
                <h3 className="mb-3 line-clamp-1 text-sm font-semibold text-card-foreground sm:mb-4 sm:text-base">
                    {modelName}
                </h3>

                <div className="grid grid-cols-2 gap-1.5 sm:gap-2">
                    <label className="grid gap-1 text-[0.68rem] text-muted-foreground sm:gap-1.5 sm:text-xs">
                        {t('input')}
                        <Input
                            type="number"
                            step="any"
                            value={editValues.input}
                            onChange={(e) => onChange({ ...editValues, input: e.target.value })}
                            className="h-9 rounded-lg border-border/25 bg-card text-xs sm:h-10 sm:text-sm"
                        />
                    </label>
                    <label className="grid gap-1 text-[0.68rem] text-muted-foreground sm:gap-1.5 sm:text-xs">
                        {t('output')}
                        <Input
                            type="number"
                            step="any"
                            value={editValues.output}
                            onChange={(e) => onChange({ ...editValues, output: e.target.value })}
                            className="h-9 rounded-lg border-border/25 bg-card text-xs sm:h-10 sm:text-sm"
                        />
                    </label>
                    <label className="grid gap-1 text-[0.68rem] text-muted-foreground sm:gap-1.5 sm:text-xs">
                        {t('cacheRead')}
                        <Input
                            type="number"
                            step="any"
                            value={editValues.cache_read}
                            onChange={(e) => onChange({ ...editValues, cache_read: e.target.value })}
                            className="h-9 rounded-lg border-border/25 bg-card text-xs sm:h-10 sm:text-sm"
                        />
                    </label>
                    <label className="grid gap-1 text-[0.68rem] text-muted-foreground sm:gap-1.5 sm:text-xs">
                        {t('cacheWrite')}
                        <Input
                            type="number"
                            step="any"
                            value={editValues.cache_write}
                            onChange={(e) => onChange({ ...editValues, cache_write: e.target.value })}
                            className="h-9 rounded-lg border-border/25 bg-card text-xs sm:h-10 sm:text-sm"
                        />
                    </label>
                </div>

                <div className="mt-3 flex gap-2 sm:mt-4">
                    <button
                        type="button"
                        onClick={onCancel}
                        disabled={isPending}
                        className="flex h-9 flex-1 items-center justify-center gap-1.5 rounded-lg border border-border/25 bg-card text-xs font-medium text-muted-foreground transition-all hover:bg-card active:scale-[0.98] sm:h-10 sm:text-sm disabled:opacity-50"
                    >
                        <X className="size-3.5 sm:size-4" />
                        {t('cancel')}
                    </button>
                    <button
                        type="button"
                        onClick={onSave}
                        disabled={isPending}
                        className="flex h-9 flex-1 items-center justify-center gap-1.5 rounded-lg text-xs font-medium transition-all active:scale-[0.98] sm:h-10 sm:text-sm disabled:opacity-50"
                        style={{ backgroundColor: brandColor, color: '#fff' }}
                    >
                        {isPending ? <Loader className="size-3.5 animate-spin sm:size-4" /> : <Check className="size-3.5 sm:size-4" />}
                        {t('save')}
                    </button>
                </div>
            </div>
        </motion.div>
    );
}
