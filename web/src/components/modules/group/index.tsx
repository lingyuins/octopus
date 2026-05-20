'use client';

import { useTranslations } from 'next-intl';
import { GroupCard } from './Card';
import { AutoGroupButton } from './AutoGroupButton';
import { AIRouteButton } from './AIRouteButton';
import { useGroupList } from '@/api/endpoints/group';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';
import {
    MorphingDialog,
    MorphingDialogContainer,
    MorphingDialogContent,
    MorphingDialogTrigger,
} from '@/components/ui/morphing-dialog';
import { matchesGroupEndpointFilter } from './utils';
import type { GroupEndpointFilter } from './utils';
import { CreateDialogContent } from './Create';
import { buttonVariants } from '@/components/ui/button';
import { useSearchableList, useGroupFilter } from '@/hooks/use-searchable-list';
import { LoadingState } from '@/components/common/LoadingState';
import { ErrorState } from '@/components/common/ErrorState';

export function Group() {
    const t = useTranslations('group');
    const { data: groups, isLoading, isError, refetch } = useGroupList();
    const pageKey = 'group' as const;
    const filter = useGroupFilter();

    const { visibleItems: visibleGroups } = useSearchableList({
        data: groups,
        pageKey,
        filter,
        filterPredicate: (item, f) => {
            if (f === 'with-members') return (item.items?.length || 0) > 0;
            if (f === 'empty') return (item.items?.length || 0) === 0;
            if (f !== 'all') {
                return matchesGroupEndpointFilter(
                    f as GroupEndpointFilter,
                    item.endpoint_type,
                    (item.items || []).map((i) => (i as { model_name: string }).model_name),
                );
            }
            return true;
        },
    });

    if (isLoading) {
        return (
            <div className="flex h-full min-h-0 flex-col overflow-y-auto overscroll-contain rounded-t-xl pb-24 md:pb-4">
                <section className="relative min-h-0 flex-1">
                    <LoadingState />
                </section>
            </div>
        );
    }

    if (isError) {
        return (
            <div className="flex h-full min-h-0 flex-col overflow-y-auto overscroll-contain rounded-t-xl pb-24 md:pb-4">
                <section className="relative min-h-0 flex-1">
                    <ErrorState onRetry={() => refetch()} />
                </section>
            </div>
        );
    }

    if (groups && groups.length === 0) {
        return (
            <div className="flex h-full items-center justify-center overflow-y-auto overscroll-contain rounded-t-xl px-4 py-6 pb-24 md:pb-6">
                <section className="relative w-full max-w-5xl rounded-xl border border-border bg-card p-5 text-card-foreground md:p-7">
                    <div className="relative flex flex-col gap-5 rounded-xl border border-border bg-card p-5 md:p-6">
                        <div className="space-y-3">
                            <div className="inline-flex w-fit items-center gap-2 rounded-md border border-border bg-card px-2.5 py-1 text-xs font-medium text-muted-foreground">
                                {t('card.empty')}
                            </div>
                            <div className="space-y-2">
                                <h2 className="max-w-xl text-2xl font-semibold tracking-tight text-foreground md:text-3xl">
                                    {t('emptyState.title')}
                                </h2>
                                <p className="max-w-2xl text-sm leading-6 text-muted-foreground">
                                    {t('emptyState.description')}
                                </p>
                            </div>
                        </div>

                        <div className="flex flex-col gap-3">
                            <AutoGroupButton variant="default" className="h-11 rounded-lg justify-start px-4" />
                            <AIRouteButton variant="default" className="h-11 rounded-lg justify-start px-4" />
                            <MorphingDialog>
                                <MorphingDialogTrigger className={buttonVariants({ variant: 'outline', className: 'h-11 min-w-36 justify-start rounded-lg border-border bg-card px-4 hover:bg-muted' })}>
                                    {t('create.submit')}
                                </MorphingDialogTrigger>
                                <MorphingDialogContainer>
                                    <MorphingDialogContent className="h-[calc(100dvh-2.5rem)] w-[min(100vw-2rem,92rem)] max-w-full flex-col overflow-hidden rounded-xl border border-border bg-card px-4 pt-4 pb-[calc(env(safe-area-inset-bottom)+1rem)] text-card-foreground md:h-[calc(100dvh-3rem)] md:px-6 md:py-5">
                                        <CreateDialogContent />
                                    </MorphingDialogContent>
                                </MorphingDialogContainer>
                            </MorphingDialog>
                        </div>
                    </div>
                </section>
            </div>
        );
    }

    return (
        <div className="flex h-full min-h-0 flex-col overflow-y-auto overscroll-contain rounded-t-xl pb-24 md:pb-4">
            <section className="relative min-h-0 flex-1">
                <VirtualizedGrid
                    items={visibleGroups}
                    columns={{ default: 1, sm: 2, md: 2, lg: 3 }}
                    estimateItemHeight={620}
                    getItemKey={(group, index) => group.id ?? `group-${index}`}
                    renderItem={(group) => <GroupCard group={group} />}
                />
            </section>
        </div>
    );
}
