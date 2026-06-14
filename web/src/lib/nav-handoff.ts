'use client';

import { create } from 'zustand';
import type { LogFilter } from '@/api/endpoints/log';

interface NavHandoffState {
    // 待消费的日志筛选条件。其它模块（如分析/分组健康）写入后，日志模块挂载时读取并清空，
    // 实现"点击失败渠道 → 跳转日志并预填筛选"。
    pendingLogFilter: LogFilter | null;
    setPendingLogFilter: (filter: LogFilter | null) => void;
    consumePendingLogFilter: () => LogFilter | null;
}

export const useNavHandoff = create<NavHandoffState>((set, get) => ({
    pendingLogFilter: null,
    setPendingLogFilter: (filter) => set({ pendingLogFilter: filter }),
    consumePendingLogFilter: () => {
        const filter = get().pendingLogFilter;
        if (filter) {
            set({ pendingLogFilter: null });
        }
        return filter;
    },
}));
