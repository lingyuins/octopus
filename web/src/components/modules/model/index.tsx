'use client';

import { useMemo, useState } from 'react';
import { useModelMarket } from '@/api/endpoints/model';
import { useTranslations } from 'next-intl';
import { ModelItem } from './Item';
import { useSearchStore, useToolbarViewOptionsStore } from '@/components/modules/toolbar';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';
import { sortModelMarketItems } from './sort';
import { CapabilitiesPanel } from './CapabilitiesPanel';
import { cn } from '@/lib/utils';

export function Model() {
    const t = useTranslations('model');
    const { data: market } = useModelMarket();
    const pageKey = 'model' as const;
    const searchTerm = useSearchStore((s) => s.getSearchTerm(pageKey));
    const layout = useToolbarViewOptionsStore((s) => s.getLayout(pageKey));
    const filter = useToolbarViewOptionsStore((s) => s.modelFilter);
    const modelSortMode = useToolbarViewOptionsStore((s) => s.modelSortMode);
    const modelLatencyUnit = useToolbarViewOptionsStore((s) => s.modelLatencyUnit);

    const sortedModels = useMemo(() => {
        const items = market?.items ?? [];
        return sortModelMarketItems(items, modelSortMode);
    }, [market, modelSortMode]);
    const hasAnyModel = (market?.items.length ?? 0) > 0;

    const visibleModels = useMemo(() => {
        const term = searchTerm.toLowerCase().trim();
        const byName = !term ? sortedModels : sortedModels.filter((m) => m.name.toLowerCase().includes(term));
        const hasPricing = (model: (typeof byName)[number]) =>
            model.input + model.output + model.cache_read + model.cache_write > 0;

        if (filter === 'priced') {
            return byName.filter(hasPricing);
        }
        if (filter === 'free') {
            return byName.filter((m) => !hasPricing(m));
        }

        return byName;
    }, [sortedModels, searchTerm, filter]);

    const [viewMode, setViewMode] = useState<'market' | 'capabilities'>('market');

    return (
        <section className="relative flex h-full min-h-0 flex-col gap-3 overflow-y-auto overscroll-contain rounded-t-xl pb-3 sm:gap-4 sm:pb-4 md:pb-4" aria-label={pageKey}>
            {/* View mode toggle */}
            <div className="flex items-center gap-0.5 self-start rounded-lg border border-border/35 bg-card p-0.5 sm:gap-1 sm:p-1">
                <button
                    type="button"
                    onClick={() => setViewMode('market')}
                    className={cn(
                        'rounded-md px-3 py-1.5 text-xs font-medium transition-colors sm:px-4 sm:py-2 sm:text-sm',
                        viewMode === 'market'
                            ? 'bg-primary text-primary-foreground shadow-sm'
                            : 'text-muted-foreground hover:text-foreground',
                    )}
                >
                    {t('tabs.market')}
                </button>
                <button
                    type="button"
                    onClick={() => setViewMode('capabilities')}
                    className={cn(
                        'rounded-md px-3 py-1.5 text-xs font-medium transition-colors sm:px-4 sm:py-2 sm:text-sm',
                        viewMode === 'capabilities'
                            ? 'bg-primary text-primary-foreground shadow-sm'
                            : 'text-muted-foreground hover:text-foreground',
                    )}
                >
                    {t('tabs.capabilities')}
                </button>
            </div>

            {viewMode === 'market' ? (
                <>
                    {visibleModels.length > 0 ? (
                <section className="relative flex min-h-0 flex-1 flex-col rounded-xl border border-border/35 bg-card p-3 text-card-foreground md:p-4">
                    <div className="relative min-h-0 flex-1">
                        <VirtualizedGrid
                            items={visibleModels}
                            layout={layout}
                            columns={{ default: 1, sm: 2, md: 2, lg: 3 }}
                            estimateItemHeight={228}
                            getItemKey={(model) => `model-${model.name}`}
                            renderItem={(model) => <ModelItem model={model} layout={layout} latencyUnit={modelLatencyUnit} />}
                            bottomPaddingClassName="pb-3 md:pb-4"
                        />
                    </div>
                </section>
            ) : (
                <section className="rounded-xl border border-border/35 bg-card p-3 text-card-foreground md:p-4">
                    <div className="relative flex min-h-[18rem] items-center justify-center overflow-hidden rounded-xl border border-dashed border-border/35 bg-card py-6">
                        <div className="relative flex flex-col items-center gap-4 px-6 text-center">
                            <div className="flex items-end gap-3">
                                <span className="h-24 w-16 rounded-lg border border-border/30 bg-card" />
                                <span className="h-28 w-20 rounded-xl border border-primary/18 bg-card" />
                                <span className="h-20 w-14 rounded-lg border border-border/30 bg-card" />
                            </div>
                            <p className="text-sm text-muted-foreground">
                                {hasAnyModel ? t('empty') : t('emptyAll')}
                            </p>
                        </div>
                    </div>
                </section>
            )}
                </>
            ) : (
                <CapabilitiesPanel />
            )}
        </section>
    );
}

