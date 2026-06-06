import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';

export interface SiteAnnouncement {
    id: number;
    remote_site_id: number;
    content: string;
    fetched_at: string;
}

const ANNOUNCEMENT_KEYS = {
    all: ['announcements'] as const,
    list: () => [...ANNOUNCEMENT_KEYS.all, 'list'] as const,
    bySite: (siteId: number) => [...ANNOUNCEMENT_KEYS.all, 'site', siteId] as const,
};

export function useAnnouncementList() {
    return useQuery({
        queryKey: ANNOUNCEMENT_KEYS.list(),
        queryFn: () => apiClient.get<SiteAnnouncement[]>('/api/v1/announcement/list'),
    });
}

export function useAnnouncementsBySite(siteId: number, enabled = true) {
    return useQuery({
        queryKey: ANNOUNCEMENT_KEYS.bySite(siteId),
        queryFn: () => apiClient.get<SiteAnnouncement[]>(`/api/v1/announcement/list/${siteId}`),
        enabled: enabled && siteId > 0,
    });
}

export function useRefreshAnnouncement() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (siteId: number) =>
            apiClient.post<null>(`/api/v1/announcement/refresh/${siteId}`),
        onSuccess: () => qc.invalidateQueries({ queryKey: ANNOUNCEMENT_KEYS.all }),
    });
}

export function useRefreshAllAnnouncements() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: () =>
            apiClient.post<{ refreshed: number }>('/api/v1/announcement/refresh-all'),
        onSuccess: () => qc.invalidateQueries({ queryKey: ANNOUNCEMENT_KEYS.all }),
    });
}
