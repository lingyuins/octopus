import { create } from 'zustand';
import type { ToolbarSearchablePage } from './view-options-store';

interface SearchState {
    searchTerms: Partial<Record<ToolbarSearchablePage, string>>;
    getSearchTerm: (page: ToolbarSearchablePage) => string;
    setSearchTerm: (page: ToolbarSearchablePage, term: string) => void;
}

export const useSearchStore = create<SearchState>((set, get) => ({
    searchTerms: {},
    getSearchTerm: (page) => get().searchTerms[page] || '',
    setSearchTerm: (page, term) => set((state) => ({
        searchTerms: { ...state.searchTerms, [page]: term }
    })),
}));
