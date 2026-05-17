'use client';

import type { ReactNode } from 'react';
import { Activity, ArrowRight, Orbit, Radar, Route } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useAnalyticsEvaluationRuntime } from '@/api/endpoints/analytics';
import { useNavStore } from '@/components/modules/navbar';
import { Button } from '@/components/ui/button';
import { ObservatorySection, StatusBadge } from './shared';

function getStatusTone(status?: string) {
    switch (status) {
        case 'completed':
            return 'success' as const;
        case 'unavailable':
            return 'warning' as const;
        case 'failed':
        case 'timeout':
            return 'danger' as const;
        case 'running':
            return 'warning' as const;
        default:
            return 'neutral' as const;
    }
}

function SummaryStat({
    label,
    value,
}: {
    label: string;
    value: string;
}) {
    return (
        <div className="rounded-lg border border-border/25 bg-card p-3 shadow-sm">
            <div className="text-xs text-muted-foreground">{label}</div>
            <div className="mt-2 text-sm font-semibold">{value}</div>
        </div>
    );
}

function EntryCard({
    icon: Icon,
    title,
    description,
    hint,
    status,
    action,
}: {
    icon: typeof Activity;
    title: string;
    description: string;
    hint: string;
    status?: { label: string; tone: 'success' | 'warning' | 'danger' | 'neutral' };
    action: ReactNode;
}) {
    return (
        <article className="rounded-lg border border-border/30 bg-card p-4">
            <div className="flex items-start justify-between gap-3">
                <div className="grid h-10 w-10 shrink-0 place-items-center rounded-lg border border-border/25 bg-card text-primary">
                    <Icon className="h-4 w-4" />
                </div>
                {status ? <StatusBadge label={status.label} tone={status.tone} /> : null}
            </div>
            <div className="mt-4 space-y-2">
                <h4 className="text-sm font-semibold">{title}</h4>
                <p className="text-sm leading-6 text-muted-foreground">{description}</p>
                <div className="rounded-lg border border-border/20 bg-card px-3 py-2 text-sm text-muted-foreground">
                    {hint}
                </div>
            </div>
            <div className="mt-4">{action}</div>
        </article>
    );
}

export function Evaluation() {
    const t = useTranslations('analytics');
    const { setActiveItem } = useNavStore();
    const sectionDescription = t('evaluation.description');
    const runtime = useAnalyticsEvaluationRuntime();
    const aiRoute = runtime.aiRouteProgress;
    const groupTest = runtime.groupTestProgress;
    const passedCount = (groupTest?.results ?? []).filter((result) => result.passed).length;
    const failedCount = (groupTest?.results ?? []).filter((result) => !result.passed).length;
    const hasAiRouteUnavailable = Boolean(runtime.aiRouteTask && runtime.aiRouteError && !aiRoute);
    const hasGroupTestUnavailable = Boolean(runtime.groupTestTask && runtime.groupTestError && !groupTest);
    const groupTestHasFailures = failedCount > 0 || Boolean(groupTest?.message);
    const aiRouteStatus = hasAiRouteUnavailable ? 'unavailable' : (aiRoute?.status ?? 'idle');
    const aiRouteStep = aiRoute?.current_step ?? 'idle';
    const groupTestStatus = groupTest
        ? (groupTest.done ? (groupTestHasFailures ? 'failed' : 'completed') : 'running')
        : hasGroupTestUnavailable
            ? 'unavailable'
            : 'idle';
    const groupTestResultLabel = !groupTest
        ? t('evaluation.summary.empty')
        : !groupTest.done
            ? t('evaluation.runtime.status.running')
            : groupTestHasFailures
                ? t('evaluation.summary.partialFailed')
                : t('evaluation.summary.allPassed');
    const statusButtonClassName = 'rounded-lg border-border/25 bg-card text-foreground shadow-sm hover:bg-card hover:text-foreground';

    return (
        <ObservatorySection
            eyebrow={t('evaluation.title')}
            title={t('evaluation.title')}
            description={sectionDescription}
            icon={Radar}
        >
            <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
                <EntryCard
                    icon={Activity}
                    title={t('evaluation.availability.title')}
                    description={t('evaluation.availability.description')}
                    hint={runtime.isLoading
                        ? t('states.loading')
                        : runtime.hasGroups
                            ? t('evaluation.availability.hint', { count: runtime.groupCount })
                            : t('evaluation.availability.empty')}
                    action={
                        <Button className={statusButtonClassName} onClick={() => setActiveItem('group')}>
                            {t('evaluation.actions.openGroupTest')}
                            <ArrowRight className="size-4" />
                        </Button>
                    }
                />
                <EntryCard
                    icon={Route}
                    title={t('evaluation.aiRoute.title')}
                    description={t('evaluation.aiRoute.description')}
                    hint={aiRoute
                        ? t('evaluation.aiRoute.hint', { step: t(`evaluation.runtime.step.${aiRouteStep}`) })
                        : hasAiRouteUnavailable
                            ? t('evaluation.aiRoute.unavailable')
                            : t('evaluation.aiRoute.empty')}
                    status={{
                        label: t(`evaluation.runtime.status.${aiRouteStatus}`),
                        tone: getStatusTone(aiRouteStatus),
                    }}
                    action={
                        <Button className={statusButtonClassName} onClick={() => setActiveItem('group')}>
                            {t('evaluation.actions.openAIRoute')}
                            <ArrowRight className="size-4" />
                        </Button>
                    }
                />
            </div>

            <div className="mt-4 space-y-4">
                <div className="rounded-lg border border-dashed border-border/30 bg-card p-4">
                    <div className="mb-2 inline-flex items-center gap-2 rounded-full border border-primary/10 bg-card px-3 py-1 text-[0.68rem] font-semibold text-primary">
                        <Orbit className="h-3.5 w-3.5" />
                        {t('evaluation.summary.title')}
                    </div>
                    <p className="mt-1 text-sm leading-6 text-muted-foreground">{t('evaluation.summary.description')}</p>
                </div>

                <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
                    <article className="rounded-lg border border-border/30 bg-card p-4">
                        <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
                            <Route className="h-4 w-4" />
                        </div>
                        <div className="mt-4 flex items-center justify-between gap-3">
                            <h4 className="text-sm font-semibold">{t('evaluation.summary.aiRoute')}</h4>
                            <StatusBadge
                                label={t(`evaluation.runtime.status.${aiRouteStatus}`)}
                                tone={getStatusTone(aiRouteStatus)}
                            />
                        </div>
                        {aiRoute ? (
                            <div className="mt-4 grid grid-cols-2 gap-3">
                                <SummaryStat
                                    label={t('evaluation.summary.status')}
                                    value={t(`evaluation.runtime.step.${aiRouteStep}`)}
                                />
                                <SummaryStat
                                    label={t('evaluation.summary.progress')}
                                    value={`${aiRoute.completed_batches} / ${aiRoute.total_batches}`}
                                />
                                <SummaryStat
                                    label={t('evaluation.summary.groups')}
                                    value={String(aiRoute.result?.group_count ?? 0)}
                                />
                                <SummaryStat
                                    label={t('evaluation.summary.routes')}
                                    value={`${aiRoute.result?.route_count ?? 0} / ${aiRoute.result?.item_count ?? 0}`}
                                />
                            </div>
                        ) : (
                            <div className="mt-4 rounded-lg border border-border/20 bg-card px-4 py-3 text-sm leading-6 text-muted-foreground">
                                {t('evaluation.aiRoute.empty')}
                            </div>
                        )}
                    </article>

                    <article className="rounded-lg border border-border/30 bg-card p-4">
                        <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
                            <Activity className="h-4 w-4" />
                        </div>
                        <div className="mt-4 flex items-center justify-between gap-3">
                            <h4 className="text-sm font-semibold">{t('evaluation.summary.groupTest')}</h4>
                            <StatusBadge
                                label={t(`evaluation.runtime.status.${groupTestStatus}`)}
                                tone={getStatusTone(groupTestStatus)}
                            />
                        </div>
                        {groupTest ? (
                            <>
                                <div className="mt-4 grid grid-cols-2 gap-3">
                                    <SummaryStat
                                        label={t('evaluation.summary.progress')}
                                        value={`${groupTest.completed} / ${groupTest.total}`}
                                    />
                                    <SummaryStat
                                        label={t('evaluation.summary.result')}
                                        value={groupTestResultLabel}
                                    />
                                    <SummaryStat
                                        label={t('evaluation.summary.passed')}
                                        value={String(passedCount)}
                                    />
                                    <SummaryStat
                                        label={t('evaluation.summary.failed')}
                                        value={String(failedCount)}
                                    />
                                </div>
                                {groupTest.message ? (
                                    <p className="mt-3 text-sm leading-6 text-destructive">{groupTest.message}</p>
                                ) : null}
                            </>
                        ) : (
                            <div className="mt-4 rounded-lg border border-border/20 bg-card px-4 py-3 text-sm leading-6 text-muted-foreground">
                                {hasGroupTestUnavailable
                                    ? t('evaluation.summary.unavailable')
                                    : t('evaluation.summary.empty')}
                            </div>
                        )}
                    </article>
                </div>
            </div>
        </ObservatorySection>
    );
}
