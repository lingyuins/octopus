'use client';

import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { AlertCircle, Loader2, Waves, Orbit } from 'lucide-react';
import { AnimatedNumber } from '@/components/common/AnimatedNumber';
import { cn } from '@/lib/utils';
import { resolveRuntimeI18nMessage } from '@/lib/i18n-runtime';
import { formatUnixSeconds } from '@/lib/time';

export function formatPercent(value: number | undefined) {
    const raw = value ?? 0;
    return {
        raw,
        formatted: {
            value: raw.toFixed(2),
            unit: '%',
        },
    };
}

export function formatUnixTime(value: number | undefined) {
    if (!value) return '';
    return formatUnixSeconds(value, {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
    });
}

export function getErrorMessage(error: unknown) {
    if (error && typeof error === 'object' && 'message' in error && typeof error.message === 'string') {
        return error.message;
    }
    return resolveRuntimeI18nMessage('errors.unexpectedError', undefined, 'Unexpected error');
}

export function QueryState({
    loading,
    error,
    empty,
    emptyLabel,
    children,
}: {
    loading: boolean;
    error: unknown;
    empty: boolean;
    emptyLabel: string;
    children: ReactNode;
}) {
    if (loading) {
        return (
            <div className="flex min-h-44 flex-col items-center justify-center gap-4 rounded-lg border border-border/30 bg-card px-4 py-8 text-sm text-muted-foreground shadow-sm ">
                <div className="grid size-14 place-items-center rounded-lg border border-border/25 bg-card shadow-sm">
                    <Loader2 className="h-6 w-6 animate-spin opacity-70" />
                </div>
                <span>{emptyLabel}</span>
            </div>
        );
    }

    if (error) {
        return (
            <div className="flex min-h-44 flex-col items-center justify-center gap-4 rounded-lg border border-destructive/20 bg-destructive/6 px-4 py-8 text-sm text-destructive">
                <div className="grid size-14 place-items-center rounded-lg border border-destructive/20 bg-destructive/10">
                    <AlertCircle className="h-6 w-6 opacity-80" />
                </div>
                <span>{getErrorMessage(error)}</span>
            </div>
        );
    }

    if (empty) {
        return (
            <div className="flex min-h-44 flex-col items-center justify-center gap-4 rounded-lg border border-dashed border-border/30 bg-card px-4 py-8 text-sm text-muted-foreground">
                <div className="grid size-14 place-items-center rounded-lg border border-border/25 bg-card">
                    <Waves className="h-6 w-6 opacity-70" />
                </div>
                <span>{emptyLabel}</span>
            </div>
        );
    }

    return <>{children}</>;
}

export function MetricCard({
    title,
    value,
    unit,
    icon: Icon,
    helper,
    accentClassName,
}: {
    title: string;
    value: string | number;
    unit?: string;
    icon: LucideIcon;
    helper?: string;
    accentClassName?: string;
}) {
    return (
        <article className="group relative min-w-0 overflow-hidden rounded-lg border border-border/30 bg-card p-3 transition-[transform,border-color] duration-300 hover:-translate-y-0.5 hover:border-primary/18 md:p-4">
            <div className="flex items-start justify-between gap-3">
                <div className="relative min-w-0">
                    <div className="text-[11px] leading-tight text-muted-foreground md:text-xs">{title}</div>
                    <div className="mt-1.5 flex min-w-0 items-baseline gap-1 md:mt-2">
                        <span className="text-[1.35rem] font-semibold leading-none md:text-2xl">
                            <AnimatedNumber value={value} />
                        </span>
                        {unit ? <span className="text-xs text-muted-foreground md:text-sm">{unit}</span> : null}
                    </div>
                    {helper ? <p className="mt-1.5 break-words text-[11px] leading-tight text-muted-foreground md:mt-2 md:text-xs">{helper}</p> : null}
                </div>
                <div
                    className={cn(
                        'relative flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary md:h-10 md:w-10',
                        accentClassName,
                    )}
                >
                    <Icon className="h-4 w-4" />
                </div>
            </div>
        </article>
    );
}

export function StatusBadge({
    label,
    tone,
}: {
    label: string;
    tone: 'success' | 'warning' | 'danger' | 'neutral';
}) {
    const toneClassName = {
        success: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
        warning: 'bg-amber-500/10 text-amber-700 dark:text-amber-300',
        danger: 'bg-destructive/10 text-destructive',
        neutral: 'bg-muted text-muted-foreground',
    }[tone];

    return (
        <span className={cn('inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium', toneClassName)}>
            {label}
        </span>
    );
}

export function ObservatorySection({
    eyebrow,
    title,
    description,
    icon: Icon,
    children,
    actions,
}: {
    eyebrow?: string;
    title: string;
    description?: string;
    icon: LucideIcon;
    children: ReactNode;
    actions?: ReactNode;
}) {
    return (
        <section className="relative overflow-hidden rounded-xl border border-border/35 bg-card p-5 text-card-foreground md:p-6">
            <div className="relative space-y-5">
                <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div className="flex min-w-0 items-start gap-4">
                        <div className="grid size-12 shrink-0 place-items-center rounded-lg border border-border/30 bg-card text-primary">
                            <Icon className="h-5 w-5" />
                        </div>
                        <div className="min-w-0 space-y-2">
                            {eyebrow ? (
                                <div className="inline-flex items-center gap-2 rounded-full border border-primary/12 bg-card px-3 py-1 text-[0.68rem] font-semibold text-primary">
                                    <Orbit className="h-3.5 w-3.5" />
                                    {eyebrow}
                                </div>
                            ) : null}
                            <div className="space-y-1">
                                <h3 className="text-lg font-semibold tracking-tight">{title}</h3>
                                {description ? (
                                    <p className="max-w-3xl text-sm leading-6 text-muted-foreground">{description}</p>
                                ) : null}
                            </div>
                        </div>
                    </div>
                    {actions ? <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div> : null}
                </div>
                {children}
            </div>
        </section>
    );
}
