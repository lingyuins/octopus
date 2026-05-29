'use client';

import { useCallback, useState, useMemo } from 'react';
import { useTranslations } from 'next-intl';
import { GripVertical, ListOrdered, RotateCcw } from 'lucide-react';
import {
    DragDropContext,
    Draggable,
    Droppable,
    type DraggableProvided,
    type DropResult,
} from '@hello-pangea/dnd';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { toast } from '@/components/common/Toast';

export type SettingItemId =
    | 'appearance'
    | 'ai-route'
    | 'auto-strategy'
    | 'account'
    | 'semantic-cache'
    | 'retry'
    | 'log'
    | 'info'
    | 'system'
    | 'llmsync'
    | 'circuit-breaker'
    | 'backup'
    | 'route-group-danger';

export const DEFAULT_SETTING_ORDER: SettingItemId[] = [
    'appearance',
    'ai-route',
    'auto-strategy',
    'account',
    'semantic-cache',
    'retry',
    'log',
    'info',
    'system',
    'llmsync',
    'circuit-breaker',
    'backup',
    'route-group-danger',
];

export const SETTING_ORDER_STORAGE_KEY = 'octopus-setting-order';

function loadStoredOrder(): SettingItemId[] {
    try {
        const raw = localStorage.getItem(SETTING_ORDER_STORAGE_KEY);
        if (!raw) return [...DEFAULT_SETTING_ORDER];
        const parsed = JSON.parse(raw);
        if (!Array.isArray(parsed)) return [...DEFAULT_SETTING_ORDER];
        const filtered = parsed.filter((id): id is SettingItemId =>
            DEFAULT_SETTING_ORDER.includes(id as SettingItemId)
        );
        const missing = DEFAULT_SETTING_ORDER.filter((id) => !filtered.includes(id));
        return [...filtered, ...missing];
    } catch {
        return [...DEFAULT_SETTING_ORDER];
    }
}

function persistOrder(items: readonly SettingItemId[]) {
    localStorage.setItem(SETTING_ORDER_STORAGE_KEY, JSON.stringify(items));
}

function reorderList<T>(list: readonly T[], startIndex: number, endIndex: number): T[] {
    const result = [...list];
    const [removed] = result.splice(startIndex, 1);
    result.splice(endIndex, 0, removed);
    return result;
}

export function useSettingOrder() {
    const [order, setOrder] = useState<SettingItemId[]>(loadStoredOrder);

    const handleDragEnd = useCallback((result: DropResult) => {
        const { destination, source } = result;
        if (!destination || destination.index === source.index) return;
        setOrder((prev) => {
            const next = reorderList(prev, source.index, destination.index);
            persistOrder(next);
            return next;
        });
    }, []);

    const reset = useCallback(() => {
        const def = [...DEFAULT_SETTING_ORDER];
        setOrder(def);
        persistOrder(def);
    }, []);

    return { order, handleDragEnd, reset };
}

export function SettingOrder() {
    const t = useTranslations('setting');
    const { order, handleDragEnd, reset } = useSettingOrder();
    const settingT = useTranslations('setting');

    const titleByKey = useMemo(() => {
        const map: Record<SettingItemId, string> = {
            appearance: settingT('appearance'),
            'ai-route': settingT('aiRoute.title'),
            'auto-strategy': settingT('autoStrategy.title'),
            account: settingT('account.title'),
            'semantic-cache': settingT('semanticCache.title'),
            retry: settingT('retry.title'),
            log: settingT('log.title'),
            info: settingT('info.title'),
            system: settingT('system'),
            llmsync: settingT('llmSync.title'),
            'circuit-breaker': settingT('circuitBreaker.title'),
            backup: settingT('backup.title'),
            'route-group-danger': settingT('routeGroups.title'),
        };
        return map;
    }, [settingT]);

    return (
        <div className="space-y-4 rounded-lg border-border/30 bg-card p-4 shadow-sm">
            <div className="flex items-start justify-between gap-3">
                <div className="space-y-1">
                    <div className="flex items-center gap-2">
                        <ListOrdered className="size-4 text-muted-foreground" />
                        <h3 className="text-sm font-semibold text-foreground">{t('settingOrder.title')}</h3>
                    </div>
                    <p className="text-xs leading-5 text-muted-foreground">{t('settingOrder.description')}</p>
                </div>

                <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => {
                        reset();
                        toast.success(t('settingOrder.resetSuccess'));
                    }}
                    className="shrink-0 rounded-xl"
                >
                    <RotateCcw className="mr-1.5 size-3.5" />
                    {t('settingOrder.reset')}
                </Button>
            </div>

            <div className="rounded-lg border border-border/30 bg-card p-1.5 shadow-sm">
                <DragDropContext onDragEnd={handleDragEnd}>
                    <Droppable droppableId="setting-item-order">
                        {(droppableProvided) => (
                            <div
                                ref={droppableProvided.innerRef}
                                {...droppableProvided.droppableProps}
                                className="max-h-[24rem] space-y-2 overflow-y-auto p-2 pr-3"
                            >
                                {order.map((id, index) => (
                                    <Draggable key={id} draggableId={id} index={index}>
                                        {(draggableProvided, snapshot) => (
                                            <div
                                                ref={draggableProvided.innerRef}
                                                {...draggableProvided.draggableProps}
                                                className={cn(
                                                    'flex items-center gap-3 rounded-lg border-border/30 bg-card px-3 py-3 shadow-sm transition-[transform,border-color,box-shadow]',
                                                    snapshot.isDragging && 'border-primary/40 shadow-md'
                                                )}
                                                style={draggableProvided.draggableProps.style}
                                            >
                                                <span className="grid size-7 shrink-0 place-items-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
                                                    {index + 1}
                                                </span>
                                                <div
                                                    className="rounded-lg p-1 text-muted-foreground"
                                                    {...(draggableProvided.dragHandleProps as DraggableProvided['dragHandleProps'])}
                                                >
                                                    <GripVertical className="size-4" />
                                                </div>
                                                <span className="truncate text-sm font-medium text-foreground">
                                                    {titleByKey[id]}
                                                </span>
                                            </div>
                                        )}
                                    </Draggable>
                                ))}
                                {droppableProvided.placeholder}
                            </div>
                        )}
                    </Droppable>
                </DragDropContext>
            </div>
        </div>
    );
}
