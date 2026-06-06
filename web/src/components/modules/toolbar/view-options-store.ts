import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type ToolbarLayout = 'grid' | 'list';
export type ToolbarSortOrder = 'asc' | 'desc';
export type ToolbarSortField = 'name' | 'created';
export type SiteToolbarSortField = 'default' | 'name' | 'created' | 'balance';
export type ToolbarCreatedSortablePage = 'channel' | 'group';
export type ToolbarSortablePage = ToolbarCreatedSortablePage | 'site';
export const TOOLBAR_PAGES = ['channel', 'group', 'model'] as const;
export type ToolbarPage = (typeof TOOLBAR_PAGES)[number];
export type ToolbarSearchablePage = ToolbarPage | 'site';
export type ChannelFilter = 'all' | 'enabled' | 'disabled';
export type GroupFilter = 'all' | 'with-members' | 'empty' | 'chat' | 'deepseek' | 'mimo' | 'responses' | 'messages' | 'embeddings' | 'rerank' | 'moderations' | 'image_generation' | 'audio_speech' | 'audio_transcription' | 'video_generation' | 'music_generation' | 'search';
export type ModelFilter = 'all' | 'priced' | 'free';
export type ModelSortMode = 'success-rate' | 'request-count';
export type ModelLatencyUnit = 'auto' | 'ms' | 's' | 'h';

export function normalizeGroupFilterValue(value?: string | null): GroupFilter {
    switch (value) {
        case 'moderation':
            return 'moderations';
        case 'responses':
        case 'messages':
            return 'chat';
        case 'all':
        case 'with-members':
        case 'empty':
        case 'chat':
        case 'deepseek':
        case 'mimo':
        case 'embeddings':
        case 'rerank':
        case 'moderations':
        case 'image_generation':
        case 'audio_speech':
        case 'audio_transcription':
        case 'video_generation':
        case 'music_generation':
        case 'search':
            return value;
        default:
            return 'all';
    }
}

function normalizePersistedToolbarState(
    state?: Partial<ToolbarViewOptionsState> | null,
): Partial<ToolbarViewOptionsState> {
    if (!state) {
        return {};
    }

    return {
        ...state,
        groupFilter: normalizeGroupFilterValue(state.groupFilter),
    };
}

interface ToolbarViewOptionsState {
    layouts: Partial<Record<ToolbarPage, ToolbarLayout>>;
    sortFields: Partial<Record<ToolbarCreatedSortablePage, ToolbarSortField> & Record<'site', SiteToolbarSortField>>;
    sortOrders: Partial<Record<ToolbarSearchablePage, ToolbarSortOrder>>;
    channelFilter: ChannelFilter;
    selectedChannelGroupId: number | null;
    groupFilter: GroupFilter;
    modelFilter: ModelFilter;
    modelSortMode: ModelSortMode;
    modelLatencyUnit: ModelLatencyUnit;

    getLayout: (item: ToolbarPage) => ToolbarLayout;
    setLayout: (item: ToolbarPage, value: ToolbarLayout) => void;

    getSortField: (item: ToolbarSortablePage) => ToolbarSortField | SiteToolbarSortField;
    setSortConfig: (
        item: ToolbarSortablePage,
        field: ToolbarSortField | SiteToolbarSortField,
        order: ToolbarSortOrder
    ) => void;

    getSortOrder: (item: ToolbarSearchablePage) => ToolbarSortOrder;
    setSortOrder: (item: ToolbarSearchablePage, value: ToolbarSortOrder) => void;

    setChannelFilter: (value: ChannelFilter) => void;
    setSelectedChannelGroupId: (value: number | null) => void;
    setGroupFilter: (value: GroupFilter) => void;
    setModelFilter: (value: ModelFilter) => void;
    setModelSortMode: (value: ModelSortMode) => void;
    setModelLatencyUnit: (value: ModelLatencyUnit) => void;
}

export const useToolbarViewOptionsStore = create<ToolbarViewOptionsState>()(
    persist(
        (set, get) => ({
            layouts: {},
            sortFields: {},
            sortOrders: {},
            channelFilter: 'all',
            selectedChannelGroupId: null,
            groupFilter: 'all',
            modelFilter: 'all',
            modelSortMode: 'success-rate',
            modelLatencyUnit: 'auto',

            getLayout: (item) => get().layouts[item] || 'grid',
            setLayout: (item, value) => {
                set((state) => ({ layouts: { ...state.layouts, [item]: value } }));
            },

            getSortField: (item) => get().sortFields[item] || 'name',
            setSortConfig: (item, field, order) => {
                set((state) => ({
                    sortFields: { ...state.sortFields, [item]: field },
                    sortOrders: { ...state.sortOrders, [item]: order },
                }));
            },

            getSortOrder: (item) => (get().sortOrders[item] === 'desc' ? 'desc' : 'asc'),
            setSortOrder: (item, value) => {
                set((state) => ({ sortOrders: { ...state.sortOrders, [item]: value } }));
            },

            setChannelFilter: (value) => set({ channelFilter: value }),
            setSelectedChannelGroupId: (value) => set({ selectedChannelGroupId: value }),
            setGroupFilter: (value) => set({ groupFilter: normalizeGroupFilterValue(value) }),
            setModelFilter: (value) => set({ modelFilter: value }),
            setModelSortMode: (value) => set({ modelSortMode: value }),
            setModelLatencyUnit: (value) => set({ modelLatencyUnit: value }),
        }),
        {
            name: 'toolbar-view-options-storage',
            partialize: (state) => ({
                layouts: state.layouts,
                sortFields: state.sortFields,
                sortOrders: state.sortOrders,
                channelFilter: state.channelFilter,
                selectedChannelGroupId: state.selectedChannelGroupId,
                groupFilter: state.groupFilter,
                modelFilter: state.modelFilter,
                modelSortMode: state.modelSortMode,
                modelLatencyUnit: state.modelLatencyUnit,
            }),
            merge: (persistedState, currentState) => ({
                ...currentState,
                ...normalizePersistedToolbarState(persistedState as Partial<ToolbarViewOptionsState> | null),
            }),
        }
    )
);
