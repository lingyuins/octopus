'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTheme } from 'next-themes';
import { SettingOrder } from './SettingOrder';
import { useTranslations } from 'next-intl';
import { Bell, Clock3, GripVertical, Languages, ListOrdered, Monitor, Moon, RotateCcw, Sun } from 'lucide-react';
import {
    DragDropContext,
    Draggable,
    Droppable,
    type DraggableProvided,
    type DropResult,
} from '@hello-pangea/dnd';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import {
    DEFAULT_NAV_ORDER,
    isFixedVisibleNavItem,
    MIN_VISIBLE_NAV_ITEMS,
    useNavStore,
    type NavItem,
} from '@/components/modules/navbar';
import { serializeNavOrder, serializeNavVisible } from '@/components/modules/navbar';
import { useSettingStore, type Locale } from '@/stores/setting';
import { SettingKey, useSetSetting, useSettingList } from '@/api/endpoints/setting';
import { toast } from '@/components/common/Toast';

type AlertNotifyLanguage = Locale;
const TIME_ZONE_OPTIONS = [
    'Asia/Shanghai',
    'Asia/Tokyo',
    'Asia/Singapore',
    'UTC',
    'Europe/London',
    'Europe/Berlin',
    'America/New_York',
    'America/Chicago',
    'America/Denver',
    'America/Los_Angeles',
] as const;

function normalizeAlertNotifyLanguage(value: string | null | undefined): AlertNotifyLanguage {
    switch (value) {
        case 'zh-Hans':
        case 'zh-Hant':
        case 'en':
            return value;
        default:
            return 'en';
    }
}

function reorderList<T>(list: readonly T[], startIndex: number, endIndex: number): T[] {
    const result = [...list];
    const [removed] = result.splice(startIndex, 1);
    result.splice(endIndex, 0, removed);
    return result;
}

function NavigationPreferences() {
    const t = useTranslations('setting');
    const navT = useTranslations('navbar');
    const setSetting = useSetSetting();
    const orderedItems = useNavStore((state) => state.orderedItems);
    const visibleItems = useNavStore((state) => state.visibleItems);
    const setOrderedItems = useNavStore((state) => state.setOrderedItems);
    const setItemVisible = useNavStore((state) => state.setItemVisible);
    const resetPreferences = useNavStore((state) => state.resetPreferences);
    const visibleItemSet = useMemo(() => new Set(visibleItems), [visibleItems]);
    const visibleCount = visibleItems.length;

    const persistNavOrder = useCallback((items: readonly NavItem[], onSuccess?: () => void) => {
        setSetting.mutate(
            {
                key: SettingKey.NavOrder,
                value: serializeNavOrder(items),
            },
            {
                onSuccess,
                onError: () => {
                    toast.error(t('saveFailed'));
                },
            }
        );
    }, [setSetting, t]);

    const persistNavVisible = useCallback((items: readonly NavItem[], onSuccess?: () => void) => {
        setSetting.mutate(
            {
                key: SettingKey.NavVisible,
                value: serializeNavVisible(items),
            },
            {
                onSuccess,
                onError: () => {
                    toast.error(t('saveFailed'));
                },
            }
        );
    }, [setSetting, t]);

    const handleDragEnd = useCallback((result: DropResult) => {
        const { destination, source } = result;
        if (!destination || destination.index === source.index) {
            return;
        }

        const nextOrder = reorderList(orderedItems, source.index, destination.index);
        setOrderedItems(nextOrder);
        persistNavOrder(nextOrder);
    }, [orderedItems, persistNavOrder, setOrderedItems]);

    const handleVisibleChange = useCallback((item: NavItem, checked: boolean) => {
        if (!checked && isFixedVisibleNavItem(item)) {
            return;
        }
        if (!checked && visibleItemSet.has(item) && visibleCount <= MIN_VISIBLE_NAV_ITEMS) {
            toast.error(t('navOrder.minimumVisibleError', { count: MIN_VISIBLE_NAV_ITEMS }));
            return;
        }

        const nextVisible = checked
            ? Array.from(new Set([...visibleItems, item]))
            : visibleItems.filter((visibleItem) => visibleItem !== item);
        setItemVisible(item, checked);
        persistNavVisible(nextVisible);
    }, [persistNavVisible, setItemVisible, t, visibleCount, visibleItemSet, visibleItems]);

    const handleReset = useCallback(() => {
        resetPreferences();
        persistNavOrder(DEFAULT_NAV_ORDER, () => {
            toast.success(t('navOrder.resetSuccess'));
        });
        persistNavVisible(DEFAULT_NAV_ORDER);
    }, [persistNavOrder, persistNavVisible, resetPreferences, t]);

    return (
        <div className="space-y-4 rounded-lg border-border/30 bg-card p-4 shadow-sm ">
            <div className="flex items-start justify-between gap-3">
                <div className="space-y-1">
                    <div className="flex items-center gap-2">
                        <ListOrdered className="size-4 text-muted-foreground" />
                        <h3 className="text-sm font-semibold text-foreground">{t('navOrder.title')}</h3>
                    </div>
                    <p className="text-xs leading-5 text-muted-foreground">{t('navOrder.description')}</p>
                </div>

                <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleReset}
                    className="shrink-0 rounded-xl"
                >
                    <RotateCcw className="mr-1.5 size-3.5" />
                    {t('navOrder.reset')}
                </Button>
            </div>

            <div className="rounded-lg border border-border/30 bg-card p-1.5 shadow-sm ">
                <DragDropContext onDragEnd={handleDragEnd}>
                    <Droppable droppableId="setting-nav-order">
                        {(droppableProvided) => (
                            <div
                                ref={droppableProvided.innerRef}
                                {...droppableProvided.droppableProps}
                                className="max-h-[24rem] space-y-2 overflow-y-auto p-2 pr-3"
                            >
                                {orderedItems.map((item, index) => {
                                    const isVisible = visibleItemSet.has(item);
                                    const isFixed = isFixedVisibleNavItem(item);
                                    const disableToggle = isFixed || (isVisible && visibleCount <= MIN_VISIBLE_NAV_ITEMS);

                                    return (
                                        <Draggable key={item} draggableId={item} index={index}>
                                            {(draggableProvided, snapshot) => (
                                                <div
                                                    ref={draggableProvided.innerRef}
                                                    {...draggableProvided.draggableProps}
                                                    className={cn(
                                                        'flex items-center justify-between gap-3 rounded-lg border-border/30 bg-card px-3 py-3 shadow-sm transition-[transform,border-color,box-shadow]',
                                                        snapshot.isDragging && 'border-primary/40 shadow-md'
                                                    )}
                                                    style={draggableProvided.draggableProps.style}
                                                >
                                                    <div className="flex min-w-0 items-center gap-3">
                                                        <span className="grid size-7 shrink-0 place-items-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
                                                            {index + 1}
                                                        </span>
                                                        <div
                                                            className="rounded-lg p-1 text-muted-foreground"
                                                            {...(draggableProvided.dragHandleProps as DraggableProvided['dragHandleProps'])}
                                                        >
                                                            <GripVertical className="size-4" />
                                                        </div>
                                                        <div className="min-w-0">
                                                            <div className="truncate text-sm font-medium text-foreground">
                                                                {navT(item)}
                                                            </div>
                                                            <div className="text-xs text-muted-foreground">
                                                                {isVisible ? t('navOrder.visible') : t('navOrder.hidden')}
                                                            </div>
                                                        </div>
                                                    </div>

                                                    <div className="flex shrink-0 items-center gap-2">
                                                        {isFixed && (
                                                            <span className="rounded-full border border-border/60 bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
                                                                {t('navOrder.fixed')}
                                                            </span>
                                                        )}
                                                        <Switch
                                                            checked={isVisible}
                                                            onCheckedChange={(checked) => handleVisibleChange(item, checked)}
                                                            disabled={disableToggle}
                                                            aria-label={t('navOrder.toggleAriaLabel', { page: navT(item) })}
                                                        />
                                                    </div>
                                                </div>
                                            )}
                                        </Draggable>
                                    );
                                })}
                                {droppableProvided.placeholder}
                            </div>
                        )}
                    </Droppable>
                </DragDropContext>
            </div>

            <p className="text-xs leading-5 text-muted-foreground">
                {t('navOrder.minimumVisibleHint', { count: MIN_VISIBLE_NAV_ITEMS })}
            </p>
        </div>
    );
}

export function SettingAppearance() {
    const t = useTranslations('setting');
    const { theme, setTheme } = useTheme();
    const { locale, setLocale, timeZone, setTimeZone } = useSettingStore();
    const { data: settings } = useSettingList();
    const setSetting = useSetSetting();
    const [alertNotifyLanguage, setAlertNotifyLanguage] = useState<AlertNotifyLanguage>('en');
    const initialAlertNotifyLanguage = useRef<AlertNotifyLanguage>('en');

    useEffect(() => {
        if (!settings) return;
        const alertNotifyLanguageSetting = settings.find((item) => item.key === SettingKey.AlertNotifyLanguage);
        if (!alertNotifyLanguageSetting) return;

        const nextValue = normalizeAlertNotifyLanguage(alertNotifyLanguageSetting.value);
        queueMicrotask(() => setAlertNotifyLanguage(nextValue));
        initialAlertNotifyLanguage.current = nextValue;
    }, [settings]);

    const handleAlertNotifyLanguageChange = (value: string) => {
        const nextValue = normalizeAlertNotifyLanguage(value);
        setAlertNotifyLanguage(nextValue);

        setSetting.mutate(
            { key: SettingKey.AlertNotifyLanguage, value: nextValue },
            {
                onSuccess: () => {
                    toast.success(t('saved'));
                    initialAlertNotifyLanguage.current = nextValue;
                },
                onError: () => {
                    setAlertNotifyLanguage(initialAlertNotifyLanguage.current);
                    toast.error(t('saveFailed'));
                },
            }
        );
    };

    return (
        <div className="relative overflow-visible rounded-xl border-border/35 bg-card p-4 sm:p-6 text-card-foreground shadow-none ">
            <div className="space-y-5">
                <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                    <div className="space-y-1.5">
                        <h2 className="flex items-center gap-2 text-lg font-bold text-card-foreground">
                            <Sun className="h-5 w-5" />
                            {t('appearance')}
                        </h2>
                        <p className="text-sm text-muted-foreground">{t('navOrder.description')}</p>
                    </div>
                    <div className="w-fit rounded-full border-border/25 bg-card px-3 py-1.5 text-xs font-medium text-muted-foreground shadow-sm">
                        Octopus
                    </div>
                </div>

                <div className="grid gap-4">
                    <div className="flex flex-col gap-4 rounded-lg border border-primary/20 bg-gradient-to-br from-primary/5 to-transparent p-4 shadow-sm md:flex-row md:items-center md:justify-between">
                        <div className="flex items-center gap-3">
                            {theme === 'dark' ? (
                                <div className="grid size-9 shrink-0 place-items-center rounded-lg bg-primary/12">
                                    <Moon className="h-5 w-5 text-primary" />
                                </div>
                            ) : (
                                <div className="grid size-9 shrink-0 place-items-center rounded-lg bg-primary/12">
                                    <Sun className="h-5 w-5 text-primary" />
                                </div>
                            )}
                            <div className="space-y-0.5">
                                <span className="text-sm font-semibold text-card-foreground">{t('theme.label')}</span>
                                <p className="text-xs text-muted-foreground">{theme === 'dark' ? t('theme.dark') : theme === 'light' ? t('theme.light') : t('theme.system')}</p>
                            </div>
                        </div>
                        <Select value={theme} onValueChange={setTheme}>
                            <SelectTrigger className="w-full rounded-lg md:w-44">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent className="rounded-lg">
                                <SelectItem value="light" className="rounded-xl">
                                    <Sun className="size-4" />
                                    {t('theme.light')}
                                </SelectItem>
                                <SelectItem value="dark" className="rounded-xl">
                                    <Moon className="size-4" />
                                    {t('theme.dark')}
                                </SelectItem>
                                <SelectItem value="system" className="rounded-xl">
                                    <Monitor className="size-4" />
                                    {t('theme.system')}
                                </SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    <div className="grid gap-4 lg:grid-cols-2">
                        <div className="flex flex-col gap-4 rounded-lg border-border/30 bg-card p-4 shadow-sm">
                            <div className="flex items-center gap-3">
                                <Languages className="h-5 w-5 text-muted-foreground" />
                                <span className="text-sm font-medium">{t('language.label')}</span>
                            </div>
                            <Select value={locale} onValueChange={(value) => setLocale(value as Locale)}>
                                <SelectTrigger className="w-full rounded-lg">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent className="rounded-lg">
                                    <SelectItem value="zh-Hans" className="rounded-xl">{t('language.zh_hans')}</SelectItem>
                                    <SelectItem value="zh-Hant" className="rounded-xl">{t('language.zh_hant')}</SelectItem>
                                    <SelectItem value="en" className="rounded-xl">{t('language.en')}</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>

                        <div className="flex flex-col gap-4 rounded-lg border-border/30 bg-card p-4 shadow-sm">
                            <div className="flex items-center gap-3">
                                <Clock3 className="h-5 w-5 text-muted-foreground" />
                                <span className="text-sm font-medium">{t('timeZone.label')}</span>
                            </div>
                            <Select value={timeZone} onValueChange={setTimeZone}>
                                <SelectTrigger className="w-full rounded-lg">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent className="rounded-lg">
                                    {TIME_ZONE_OPTIONS.map((option) => (
                                        <SelectItem key={option} value={option} className="rounded-xl">
                                            {t(`timeZone.options.${option}`)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                            <p className="text-xs leading-5 text-muted-foreground">{t('timeZone.description')}</p>
                        </div>

                        <div className="flex flex-col gap-4 rounded-lg border-border/30 bg-card p-4 shadow-sm">
                            <div className="flex items-center gap-3">
                                <Bell className="h-5 w-5 text-muted-foreground" />
                                <span className="text-sm font-medium">{t('alertLanguage.label')}</span>
                            </div>
                            <Select value={alertNotifyLanguage} onValueChange={handleAlertNotifyLanguageChange}>
                                <SelectTrigger className="w-full rounded-lg">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent className="rounded-lg">
                                    <SelectItem value="zh-Hans" className="rounded-xl">{t('alertLanguage.zh_hans')}</SelectItem>
                                    <SelectItem value="zh-Hant" className="rounded-xl">{t('alertLanguage.zh_hant')}</SelectItem>
                                    <SelectItem value="en" className="rounded-xl">{t('alertLanguage.en')}</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                    </div>

                    <NavigationPreferences />
                    <SettingOrder />
                </div>
            </div>
        </div>
    );
}
