import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';

/**
 * 创建隔离的 QueryClient 用于 hook 测试。
 * 关闭 retry 避免测试中自动重试干扰断言。
 */
export function createTestQueryClient() {
    return new QueryClient({
        defaultOptions: {
            queries: {
                retry: false,
                gcTime: 0,
            },
            mutations: {
                retry: false,
            },
        },
    });
}

/**
 * 极简 QueryClientProvider wrapper，避免全局状态串扰。
 * 仅提供 queryClient context，不引入其他 provider。
 */
export function TestQueryProvider({
    client,
    children,
}: {
    client: QueryClient;
    children: React.ReactNode;
}) {
    return React.createElement(QueryClientProvider, { client }, children);
}
