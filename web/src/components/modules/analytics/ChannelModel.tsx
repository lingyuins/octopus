'use client';

import { useState } from 'react';
import { Boxes } from 'lucide-react';
import { useTranslations } from 'next-intl';
import {
    useAnalyticsChannelModel,
    type AnalyticsRange,
    type AnalyticsChannelModelItem,
} from '@/api/endpoints/analytics';
import { useGroupList } from '@/api/endpoints/group';
import { ObservatorySection, QueryState, formatPercent } from './shared';
import { formatCount, formatMoney, cn } from '@/lib/utils';
import { useNavStore } from '@/components/modules/navbar/nav-store';
import { useNavHandoff } from '@/lib/nav-handoff';

function successRateClass(rate: number) {
    if (rate < 50) return 'text-destructive';
    if (rate < 90) return 'text-amber-600 dark:text-amber-400';
    if (rate >= 99.99) return 'text-accent';
    return 'text-foreground';
}

function ChannelModelRow({
    item,
    onViewLogs,
}: {
    item: AnalyticsChannelModelItem;
    onViewLogs: (item: AnalyticsChannelModelItem) => void;
}) {
    const t = useTranslations('analytics');
    const failed = item.request_count - Math.round((item.request_count * item.success_rate) / 100);

    return (
        <button
            type="button"
            onClick={() => onViewLogs(item)}
            className="flex w-full items-center justify-between gap-3 rounded-lg border border-border/25 bg-card px-3 py-3 text-left shadow-sm transition-colors hover:border-primary/25"
        >
            <div className="min-w-0">
                <div className="flex items-center gap-2">
                    <span className="truncate text-sm font-medium">{item.channel_name || `#${item.channel_id}`}</span>
                    {!item.enabled && (
                        <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                            {t('channelModel.disabled')}
                        </span>
                    )}
                </div>
                <div className="mt-0.5 truncate text-xs text-muted-foreground">{item.model_name}</div>
                <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                    <span>
                        {formatCount(item.request_count).formatted.value}
                        {formatCount(item.request_count).formatted.unit} {t('channelModel.requests')}
                    </span>
                    {failed > 0 && (
                        <span className="text-destructive">
                            {formatCount(failed).formatted.value}
                            {formatCount(failed).formatted.unit} {t('channelModel.failed')}
                        </span>
                    )}
                </div>
            </div>
            <div className="flex shrink-0 flex-col items-end gap-1">
                <span className={cn('text-lg font-semibold', successRateClass(item.success_rate))}>
                    {formatPercent(item.success_rate).formatted.value}%
                </span>
                <span className="text-xs text-muted-foreground">
                    {formatMoney(item.total_cost).formatted.value}
                    {formatMoney(item.total_cost).formatted.unit}
                </span>
            </div>
        </button>
    );
}

export function ChannelModel({ range }: { range: AnalyticsRange }) {
    const t = useTranslations('analytics');
    const { data: groups = [] } = useGroupList();
    const [groupId, setGroupId] = useState<number | undefined>(undefined);
    const { data, isLoading, error } = useAnalyticsChannelModel(range, groupId);

    const setActiveItem = useNavStore((s) => s.setActiveItem);
    const setPendingLogFilter = useNavHandoff((s) => s.setPendingLogFilter);

    const onViewLogs = (item: AnalyticsChannelModelItem) => {
        // 跳转日志，预填渠道 + 模型 + 开启尝试穿透，定位该渠道该模型的失败请求。
        setPendingLogFilter({
            channel_id: item.channel_id,
            model: item.model_name,
            include_attempts: true,
        });
        setActiveItem('log');
    };

    return (
        <ObservatorySection
            eyebrow={t('cards.channelModel.title')}
            title={t('cards.channelModel.title')}
            description={t('channelModel.description')}
            icon={Boxes}
            actions={
                <div className="flex items-center gap-2">
                    <label className="text-xs text-muted-foreground">{t('channelModel.scopeAll')}</label>
                    <select
                        value={groupId ?? ''}
                        onChange={(e) => {
                            const v = e.target.value;
                            setGroupId(v === '' ? undefined : Number(v));
                        }}
                        className="h-7 rounded-md border border-border/50 bg-background px-2 text-xs outline-none focus:border-primary/30"
                    >
                        <option value="">{t('channelModel.scopeAll')}</option>
                        {groups.map((g) => (
                            <option key={g.id} value={g.id}>
                                {g.name}
                            </option>
                        ))}
                    </select>
                </div>
            }
        >
            <QueryState
                loading={isLoading}
                error={error}
                empty={!data || data.length === 0}
                emptyLabel={isLoading ? t('states.loading') : t('channelModel.empty')}
            >
                <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
                    {(data ?? []).map((item) => (
                        <ChannelModelRow
                            key={`${item.channel_id}-${item.model_name}`}
                            item={item}
                            onViewLogs={onViewLogs}
                        />
                    ))}
                </div>
            </QueryState>
        </ObservatorySection>
    );
}
