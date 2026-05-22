'use client';

import { useMemo, useState } from 'react';
import { useModelCapabilities, type ModelCapability } from '@/api/endpoints/model';
import { useTranslations } from 'next-intl';
import { LoadingState } from '@/components/common/LoadingState';
import { ErrorState } from '@/components/common/ErrorState';
import { Search } from 'lucide-react';
import { Badge } from '@/components/ui/badge';

function CapabilityBadge({ endpoint }: { endpoint: string }) {
    if (endpoint === '*') {
        return <Badge variant="secondary" className="font-mono text-[10px]">*</Badge>;
    }
    return <Badge variant="outline" className="font-mono text-[10px]">{endpoint}</Badge>;
}

function CapabilityRow({ cap }: { cap: ModelCapability }) {
    return (
        <tr className="border-b border-border/30 transition-colors hover:bg-muted/40">
            <td className="px-4 py-2.5 text-sm font-medium">{cap.name}</td>
            <td className="px-4 py-2.5">
                <div className="flex flex-wrap gap-1">
                    {cap.endpoints.map((ep) => (
                        <CapabilityBadge key={ep} endpoint={ep} />
                    ))}
                </div>
            </td>
            <td className="px-4 py-2.5 text-center">
                {cap.conversation ? (
                    <span className="inline-flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400">
                        <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500" />
                        Yes
                    </span>
                ) : (
                    <span className="text-xs text-muted-foreground">—</span>
                )}
            </td>
            <td className="px-4 py-2.5 text-center">
                {cap.available ? (
                    <span className="inline-flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400">
                        <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500" />
                        Active
                    </span>
                ) : (
                    <span className="inline-flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400">
                        <span className="inline-block h-1.5 w-1.5 rounded-full bg-amber-500" />
                        Down
                    </span>
                )}
            </td>
        </tr>
    );
}

export function CapabilitiesPanel() {
    const t = useTranslations('model');
    const { data: capabilities, isLoading, error, refetch } = useModelCapabilities();
    const [search, setSearch] = useState('');

    const filtered = useMemo(() => {
        if (!capabilities) return [];
        const term = search.toLowerCase().trim();
        if (!term) return capabilities;
        return capabilities.filter((c) => c.name.toLowerCase().includes(term));
    }, [capabilities, search]);

    if (isLoading) {
        return (
            <section className="rounded-xl border border-border/35 bg-card p-3 text-card-foreground md:p-4">
                <LoadingState />
            </section>
        );
    }

    if (error) {
        return (
            <section className="rounded-xl border border-border/35 bg-card p-3 text-card-foreground md:p-4">
                <ErrorState message={error.message} onRetry={() => refetch()} />
            </section>
        );
    }

    const convCount = filtered.filter((c) => c.conversation).length;
    const nonConvCount = filtered.length - convCount;

    return (
        <section className="relative flex h-full min-h-0 flex-col gap-4">
            {/* Summary strip */}
            <div className="flex flex-wrap items-center gap-4 rounded-xl border border-border/35 bg-card px-4 py-3 text-sm text-muted-foreground">
                <span>Total: <strong className="text-foreground">{filtered.length}</strong></span>
                <span>Conversation: <strong className="text-foreground">{convCount}</strong></span>
                <span>Non-conversation: <strong className="text-foreground">{nonConvCount}</strong></span>
            </div>

            {/* Search + table */}
            <div className="flex min-h-0 flex-1 flex-col rounded-xl border border-border/35 bg-card">
                <div className="flex items-center gap-2 border-b border-border/30 px-4 py-2.5">
                    <Search className="h-4 w-4 text-muted-foreground" />
                    <input
                        type="text"
                        placeholder="Filter by model name…"
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                    />
                </div>

                <div className="min-h-0 flex-1 overflow-auto">
                    <table className="w-full text-left text-sm">
                        <thead className="sticky top-0 z-10 border-b border-border/30 bg-muted/50 text-xs uppercase text-muted-foreground">
                            <tr>
                                <th className="px-4 py-2.5 font-medium">Model</th>
                                <th className="px-4 py-2.5 font-medium">Endpoints</th>
                                <th className="px-4 py-2.5 text-center font-medium">Conversation</th>
                                <th className="px-4 py-2.5 text-center font-medium">Status</th>
                            </tr>
                        </thead>
                        <tbody>
                            {filtered.length > 0 ? (
                                filtered.map((cap) => (
                                    <CapabilityRow key={cap.name} cap={cap} />
                                ))
                            ) : (
                                <tr>
                                    <td colSpan={4} className="px-4 py-12 text-center text-muted-foreground">
                                        {search ? 'No models match your filter.' : 'No capabilities found.'}
                                    </td>
                                </tr>
                            )}
                        </tbody>
                    </table>
                </div>
            </div>

            <p className="px-1 text-xs text-muted-foreground">
                * group items are narrowed at relay time for non-conversation endpoints.
                A model appearing here may still return 404 for an endpoint if its items don&apos;t actually support it.
            </p>
        </section>
    );
}
