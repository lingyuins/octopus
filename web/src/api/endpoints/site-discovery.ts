import { useMutation } from '@tanstack/react-query';
import { apiClient } from '../client';

export interface DiscoveredSite {
    name: string;
    base_url: string;
    type: string;
    status: string;
}

export function useDiscoverSites() {
    return useMutation({
        mutationFn: () => apiClient.get<DiscoveredSite[]>('/api/v1/site-discovery/discover'),
    });
}
