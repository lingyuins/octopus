'use client';

import { useMemo } from 'react';
import { useSearchStore, useToolbarViewOptionsStore, normalizeGroupFilterValue } from '@/components/modules/toolbar';
import type { ToolbarCreatedSortablePage, ChannelFilter, GroupFilter } from '@/components/modules/toolbar/view-options-store';

type SortableItem = {
    id: number;
    name: string;
};

type ChannelItem = SortableItem & {
    enabled: boolean;
};

type GroupItem = SortableItem & {
    items?: { length: number }[] | unknown[];
    endpoint_type: string;
};

type FilterPredicate<T> = (item: T, filter: string) => boolean;

interface UseSearchableListOptions<T> {
    data: T[] | undefined;
    pageKey: ToolbarCreatedSortablePage;
    filter?: string;
    filterPredicate?: FilterPredicate<T>;
    getItemId?: (item: T) => number;
    getItemName?: (item: T) => string;
}

function getSortComparator<T>(
    sortField: 'name' | 'created',
    sortOrder: 'asc' | 'desc',
    getItemId: (item: T) => number,
    getItemName: (item: T) => string,
) {
    return (a: T, b: T) => {
        const diff = sortField === 'name'
            ? getItemName(a).localeCompare(getItemName(b))
            : getItemId(a) - getItemId(b);
        return sortOrder === 'asc' ? diff : -diff;
    };
}

export function useSearchableList<T>({
    data,
    pageKey,
    filter,
    filterPredicate,
    getItemId,
    getItemName,
}: UseSearchableListOptions<T>) {
    const searchTerm = useSearchStore((s) => s.getSearchTerm(pageKey));
    const sortField = useToolbarViewOptionsStore((s) => s.getSortField(pageKey) as 'name' | 'created');
    const sortOrder = useToolbarViewOptionsStore((s) => s.getSortOrder(pageKey));
    const resolvedGetItemId = useMemo(
        () => getItemId ?? ((item: T) => (item as SortableItem).id),
        [getItemId]
    );
    const resolvedGetItemName = useMemo(
        () => getItemName ?? ((item: T) => (item as SortableItem).name),
        [getItemName]
    );

    const sortedItems = useMemo(() => {
        if (!data) return [];
        return [...data].sort(getSortComparator(sortField, sortOrder, resolvedGetItemId, resolvedGetItemName));
    }, [data, sortField, sortOrder, resolvedGetItemId, resolvedGetItemName]);

    const visibleItems = useMemo(() => {
        const term = searchTerm.toLowerCase().trim();
        const byName = !term ? sortedItems : sortedItems.filter((item) => resolvedGetItemName(item).toLowerCase().includes(term));

        if (filter && filterPredicate) {
            return byName.filter((item) => filterPredicate(item, filter));
        }

        return byName;
    }, [sortedItems, searchTerm, filter, filterPredicate, resolvedGetItemName]);

    return { visibleItems, sortedItems, searchTerm, sortField, sortOrder };
}

export function useChannelFilter() {
    return useToolbarViewOptionsStore((s) => s.channelFilter);
}

export function useGroupFilter() {
    return useToolbarViewOptionsStore((s) => normalizeGroupFilterValue(s.groupFilter));
}

export function createChannelFilterPredicate(filter: ChannelFilter) {
    return (item: ChannelItem) => {
        if (filter === 'enabled') return item.enabled;
        if (filter === 'disabled') return !item.enabled;
        return true;
    };
}

export function createGroupFilterPredicate(filter: GroupFilter) {
    return (item: GroupItem) => {
        if (filter === 'with-members') return (item.items?.length || 0) > 0;
        if (filter === 'empty') return (item.items?.length || 0) === 0;
        return true;
    };
}
