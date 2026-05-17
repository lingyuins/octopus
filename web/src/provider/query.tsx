'use client';

import { QueryClient, QueryClientProvider, QueryCache, MutationCache } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from '@/components/common/Toast';

function getErrorMessage(error: unknown, fallback = 'An error occurred') {
    if (error instanceof Error && error.message) {
        return error.message;
    }
    if (typeof error === 'object' && error !== null && 'message' in error && typeof error.message === 'string') {
        return error.message;
    }
    return fallback;
}

export default function QueryProvider({ children }: { children: React.ReactNode }) {
    const [queryClient] = useState(
        () =>
            new QueryClient({
                defaultOptions: {
                    queries: {
                        staleTime: 60 * 1000,
                        refetchOnWindowFocus: false,
                        retry: (failureCount, error) => {
                            // Only retry on network errors, not 4xx client errors
                            if (error instanceof Error && 'code' in error) {
                                const code = (error as { code: number }).code;
                                if (code >= 400 && code < 500) return false;
                            }
                            return failureCount < 2;
                        },
                        retryDelay: (attemptIndex) => Math.min(1000 * 2 ** attemptIndex, 30000),
                    },
                    mutations: {
                        retry: false,
                    },
                },
                queryCache: new QueryCache({
                    onError: (error, query) => {
                        if (query.meta?.skipGlobalErrorHandler) return;
                        const message = getErrorMessage(error);
                        toast.error(message);
                    },
                }),
                mutationCache: new MutationCache({
                    onError: (error, _variables, _context, mutation) => {
                        if (mutation.meta?.skipGlobalErrorHandler) return;
                        const message = getErrorMessage(error);
                        toast.error(message);
                    },
                }),
            })
    );

    return (
        <QueryClientProvider client={queryClient}>
            {children}
        </QueryClientProvider>
    );
}
