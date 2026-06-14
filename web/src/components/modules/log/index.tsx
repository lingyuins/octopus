'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useChannelList } from '@/api/endpoints/channel';
import { useAPIKeyList } from '@/api/endpoints/apikey';
import { useLogs, type LogFilter } from '@/api/endpoints/log';
import { LogCard } from './Item';
import { Loader2, Search, X } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';
import { useNavHandoff } from '@/lib/nav-handoff';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { ENDPOINT_TYPE_OPTIONS } from '@/components/modules/group/utils';

const EMPTY_FILTER: LogFilter = {};

/**
 * 日志筛选栏
 */
function LogFilterBar({
    filter,
    onChange,
}: {
    filter: LogFilter;
    onChange: (f: LogFilter) => void;
}) {
    const t = useTranslations('log.filter');
    const tGroup = useTranslations('group');
    const { data: channels = [] } = useChannelList();
    const { data: apiKeys = [] } = useAPIKeyList();

    const hasFilter = !!(
        filter.model ||
        filter.channel_id != null ||
        filter.api_key_id != null ||
        filter.endpoint_type ||
        filter.status
    );

    const handleClear = useCallback(() => {
        onChange(EMPTY_FILTER);
    }, [onChange]);

    // 受控输入框文本：与 filter.model 单向同步。用户输入只更新本地缓冲并防抖回写
    // filter；当 filter.model 被外部重置（如「清除」按钮）时再同步回输入框，
    // 避免非受控 defaultValue 在清除后仍残留旧文字。
    const [modelInput, setModelInput] = useState(filter.model ?? '');
    useEffect(() => {
        setModelInput(filter.model ?? '');
    }, [filter.model]);

    // Debounce model input
    const modelTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
    const handleModelInput = useCallback(
        (e: React.ChangeEvent<HTMLInputElement>) => {
            const val = e.target.value;
            setModelInput(val);
            if (modelTimer.current) clearTimeout(modelTimer.current);
            modelTimer.current = setTimeout(() => {
                const trimmed = val.trim() || undefined;
                onChange({ ...filter, model: trimmed });
            }, 400);
        },
        [filter, onChange],
    );

    return (
        <div className="flex flex-wrap items-center gap-2 rounded-xl border border-border/35 bg-card px-3 py-2">
            <div className="flex items-center gap-1.5 min-w-0">
                <Search className="size-3.5 shrink-0 text-muted-foreground" />
                <input
                    value={modelInput}
                    onChange={handleModelInput}
                    placeholder={t('modelPlaceholder')}
                    className="h-7 min-w-0 w-28 rounded-md border border-border/50 bg-background px-2 text-xs outline-none placeholder:text-muted-foreground/50 focus:border-primary/30 focus:ring-1 focus:ring-primary/20"
                />
            </div>

            <Select
                value={filter.channel_id != null ? String(filter.channel_id) : ''}
                onValueChange={(v) => {
                    const next = { ...filter };
                    if (v && v !== '' && v !== '__all__') {
                        next.channel_id = Number(v);
                    } else {
                        delete next.channel_id;
                    }
                    onChange(next);
                }}
            >
                <SelectTrigger size="sm" className="h-7 text-xs min-w-[7rem]">
                    <SelectValue placeholder={t('allChannels')} />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="__all__">{t('allChannels')}</SelectItem>
                    {channels.map((ch) => (
                        <SelectItem key={ch.raw.id} value={String(ch.raw.id)}>
                            {ch.raw.name}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>

            <Select
                value={filter.api_key_id != null ? String(filter.api_key_id) : ''}
                onValueChange={(v) => {
                    const next = { ...filter };
                    if (v && v !== '' && v !== '__all__') {
                        next.api_key_id = Number(v);
                    } else {
                        delete next.api_key_id;
                    }
                    onChange(next);
                }}
            >
                <SelectTrigger size="sm" className="h-7 text-xs min-w-[7rem]">
                    <SelectValue placeholder={t('allKeys')} />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="__all__">{t('allKeys')}</SelectItem>
                    {apiKeys.map((key) => (
                        <SelectItem key={key.id} value={String(key.id)}>
                            {key.name}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>

            <Select
                value={filter.endpoint_type ?? ''}
                onValueChange={(v) => {
                    const next = { ...filter };
                    if (v && v !== '' && v !== '__all__') {
                        next.endpoint_type = v;
                    } else {
                        delete next.endpoint_type;
                    }
                    onChange(next);
                }}
            >
                <SelectTrigger size="sm" className="h-7 text-xs min-w-[7rem]">
                    <SelectValue placeholder={t('allTypes')} />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="__all__">{t('allTypes')}</SelectItem>
                    {ENDPOINT_TYPE_OPTIONS.filter((o) => o.value !== '*').map((opt) => (
                        <SelectItem key={opt.value} value={opt.value}>
                            {tGroup(opt.labelKey)}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>

            <Select
                value={filter.status ?? ''}
                onValueChange={(v) => {
                    const next = { ...filter };
                    if (v && v !== '' && v !== '__all__') {
                        next.status = v as 'success' | 'error';
                    } else {
                        delete next.status;
                    }
                    onChange(next);
                }}
            >
                <SelectTrigger size="sm" className="h-7 text-xs min-w-[6rem]">
                    <SelectValue placeholder={t('allStatuses')} />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="__all__">{t('allStatuses')}</SelectItem>
                    <SelectItem value="success">{t('statusSuccess')}</SelectItem>
                    <SelectItem value="error">{t('statusError')}</SelectItem>
                </SelectContent>
            </Select>

            {hasFilter && (
                <button
                    type="button"
                    onClick={handleClear}
                    className="flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
                >
                    <X className="size-3" />
                    {t('clear')}
                </button>
            )}
        </div>
    );
}

/**
 * 日志页面组件
 * - 初始加载 pageSize 条历史日志
 * - SSE 实时推送新日志
 * - 滚动自动加载更多
 * - 筛选（模型、渠道、密钥、端点类型、状态）
 */
export function Log() {
    const t = useTranslations('log');
    const [filter, setFilter] = useState<LogFilter>(EMPTY_FILTER);
    const { logs, hasMore, isLoading, isLoadingMore, loadMore } = useLogs({ filter });
    const { data: channels = [] } = useChannelList();

    // 消费来自其它模块（分析/分组健康）的待处理筛选，实现"点击失败渠道 → 跳转日志并预填"。
    const pendingLogFilter = useNavHandoff((s) => s.pendingLogFilter);
    const consumePendingLogFilter = useNavHandoff((s) => s.consumePendingLogFilter);
    useEffect(() => {
        const pending = consumePendingLogFilter();
        if (pending) {
            setFilter(pending);
        }
    }, [pendingLogFilter, consumePendingLogFilter]);

    const channelNameById = useMemo(() => {
        const map = new Map<number, string>();
        for (const item of channels) {
            map.set(item.raw.id, item.raw.name);
        }
        return map;
    }, [channels]);

    const canLoadMore = hasMore && !isLoading && !isLoadingMore && logs.length > 0;
    const handleReachEnd = useCallback(() => {
        if (!canLoadMore) return;
        void loadMore();
    }, [canLoadMore, loadMore]);

    const footer = useMemo(() => {
        if (hasMore && (isLoading || isLoadingMore)) {
            return (
                <div className="flex justify-center py-6">
                    <div className="flex items-center gap-2 rounded-full border border-border/50 bg-card/80 px-4 py-2 shadow-sm backdrop-blur">
                        <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                        <span className="text-xs text-muted-foreground">{t('list.loadingMore')}</span>
                    </div>
                </div>
            );
        }
        if (!hasMore && logs.length > 0) {
            return (
                <div className="flex justify-center py-6">
                    <span className="text-xs text-muted-foreground/60">{t('list.noMore')}</span>
                </div>
            );
        }
        return null;
    }, [hasMore, isLoading, isLoadingMore, logs.length, t]);

    return (
        <div className="flex h-full min-h-0 flex-col gap-3 overflow-hidden rounded-t-xl pt-2 md:pt-0">
            <LogFilterBar filter={filter} onChange={setFilter} />
            {isLoading && logs.length === 0 ? (
                <div className="flex min-h-[18rem] items-center justify-center rounded-xl border border-border/35 bg-card">
                    <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                </div>
            ) : logs.length === 0 ? (
                <div className="flex min-h-[18rem] items-center justify-center rounded-xl border border-dashed border-border/35 bg-card px-6 py-6 text-center">
                    <p className="text-sm text-muted-foreground">{t('list.empty')}</p>
                </div>
            ) : (
                <div className="min-h-0 flex-1">
                    <VirtualizedGrid
                        items={logs}
                        layout="list"
                        columns={{ default: 1 }}
                        estimateItemHeight={180}
                        overscan={8}
                        getItemKey={(log) => `log-${log.id}`}
                        renderItem={(log) => <LogCard log={log} channelNameById={channelNameById} />}
                        footer={footer}
                        onReachEnd={handleReachEnd}
                        reachEndEnabled={canLoadMore}
                        reachEndOffset={2}
                        bottomPaddingClassName="pb-16 md:pb-4"
                    />
                </div>
            )}
        </div>
    );
}
