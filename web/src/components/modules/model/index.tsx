'use client';

import { useMemo, useState } from 'react';
import { useModelMarket, useUpdateModelPrice } from '@/api/endpoints/model';
import { useTranslations } from 'next-intl';
import { ModelItem } from './Item';
import { ModelMarketSummary } from './MarketSummary';
import { useSearchStore, useToolbarViewOptionsStore } from '@/components/modules/toolbar';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';
import { sortModelMarketItems } from './sort';
import { CapabilitiesPanel } from './CapabilitiesPanel';
import { cn } from '@/lib/utils';

export function Model() {
    const t = useTranslations('model');
    const { data: market } = useModelMarket();
    const updateModelPrice = useUpdateModelPrice();
    const pageKey = 'model' as const;
    const searchTerm = useSearchStore((s) => s.getSearchTerm(pageKey));
    const layout = useToolbarViewOptionsStore((s) => s.getLayout(pageKey));
    const filter = useToolbarViewOptionsStore((s) => s.modelFilter);
    const modelSortMode = useToolbarViewOptionsStore((s) => s.modelSortMode);

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

    const summary = market?.summary ?? {
        model_count: 0,
        coverage_count: 0,
        unique_channel_count: 0,
        average_latency_ms: 0,
        last_update_time: '',
    };

    const [viewMode, setViewMode] = useState<'market' | 'capabilities'>('market');

    return (
        <section className="relative flex h-full min-h-0 flex-col gap-4 overflow-y-auto overscroll-contain rounded-t-xl pb-24 md:pb-4" aria-label={pageKey}>
            {/* View mode toggle */}
            <div className="flex items-center gap-1 self-start rounded-lg border border-border/35 bg-card p-1">
                <button
                    type="button"
                    onClick={() => setViewMode('market')}
                    className={cn(
                        'rounded-md px-4 py-2 text-sm font-medium transition-colors',
                        viewMode === 'market'
                            ? 'bg-primary text-primary-foreground shadow-sm'
                            : 'text-muted-foreground hover:text-foreground',
                    )}
                >
                    Market
                </button>
                <button
                    type="button"
                    onClick={() => setViewMode('capabilities')}
                    className={cn(
                        'rounded-md px-4 py-2 text-sm font-medium transition-colors',
                        viewMode === 'capabilities'
                            ? 'bg-primary text-primary-foreground shadow-sm'
                            : 'text-muted-foreground hover:text-foreground',
                    )}
                >
                    Capabilities
                </button>
            </div>

            {viewMode === 'market' ? (
                <>
                    <ModelMarketSummary
                summary={summary}
                onRefresh={() => updateModelPrice.mutate()}
                isRefreshing={updateModelPrice.isPending}
            />

            {visibleModels.length > 0 ? (
                <section className="relative flex min-h-0 flex-1 flex-col rounded-xl border border-border/35 bg-card p-3 text-card-foreground md:p-4">
                    <div className="relative min-h-0 flex-1">
                        <VirtualizedGrid
                            items={visibleModels}
                            layout={layout}
                            columns={{ default: 1, sm: 2, md: 2, lg: 3 }}
                            estimateItemHeight={228}
                            getItemKey={(model) => `model-${model.name}`}
                            renderItem={(model) => <ModelItem model={model} layout={layout} />}
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
