'use client';

import { useEffect, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { RotateCcw } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { SettingKey, useSettingList, useSetSetting } from '@/api/endpoints/setting';
import { toast } from '@/components/common/Toast';
import { RETRY_FIELDS } from './runtime-settings';

export function SettingRetry() {
    const t = useTranslations('setting');
    const { data: settings } = useSettingList();
    const setSetting = useSetSetting();

    const [values, setValues] = useState<Record<string, string>>({});
    const initialValues = useRef<Record<string, string>>({});

    useEffect(() => {
        if (!settings) return;

        const nextValues = RETRY_FIELDS.reduce<Record<string, string>>((acc, field) => {
            acc[field.key] = settings.find((item) => item.key === field.key)?.value ?? '';
            return acc;
        }, {});
        nextValues[SettingKey.KeySelectionStrategy] = settings.find((item) => item.key === SettingKey.KeySelectionStrategy)?.value ?? 'cost';
        nextValues[SettingKey.RetryEmptyOutput] = settings.find((item) => item.key === SettingKey.RetryEmptyOutput)?.value ?? 'true';
        nextValues[SettingKey.ReasoningBufferStrategy] = settings.find((item) => item.key === SettingKey.ReasoningBufferStrategy)?.value ?? 'buffer';
        nextValues[SettingKey.RelayLogQueueDropPolicy] = settings.find((item) => item.key === SettingKey.RelayLogQueueDropPolicy)?.value ?? 'oldest';
        nextValues[SettingKey.StreamSessionReplayEnabled] = settings.find((item) => item.key === SettingKey.StreamSessionReplayEnabled)?.value ?? 'true';
        nextValues[SettingKey.KeyHealthCheckEnabled] = settings.find((item) => item.key === SettingKey.KeyHealthCheckEnabled)?.value ?? 'false';
        nextValues[SettingKey.KeyHealthCheckInterval] = settings.find((item) => item.key === SettingKey.KeyHealthCheckInterval)?.value ?? '30';
        nextValues[SettingKey.KeyHealthCheckFailThreshold] = settings.find((item) => item.key === SettingKey.KeyHealthCheckFailThreshold)?.value ?? '3';
        nextValues[SettingKey.KeyHealthCheckNotifyEnabled] = settings.find((item) => item.key === SettingKey.KeyHealthCheckNotifyEnabled)?.value ?? 'true';
        nextValues[SettingKey.KeyHealthCheckRecoveryNotify] = settings.find((item) => item.key === SettingKey.KeyHealthCheckRecoveryNotify)?.value ?? 'true';
        nextValues[SettingKey.KeyHealthCheckNotifyCooldown] = settings.find((item) => item.key === SettingKey.KeyHealthCheckNotifyCooldown)?.value ?? '300';
        nextValues[SettingKey.RateLimitHoldEnabled] = settings.find((item) => item.key === SettingKey.RateLimitHoldEnabled)?.value ?? 'false';
        queueMicrotask(() => setValues(nextValues));
        initialValues.current = nextValues;
    }, [settings]);

    const handleSave = (key: string) => {
        const value = values[key] ?? '';
        if (value === initialValues.current[key]) return;

        setSetting.mutate(
            { key, value },
            {
                onSuccess: () => {
                    toast.success(t('saved'));
                    initialValues.current = {
                        ...initialValues.current,
                        [key]: value,
                    };
                }
            }
        );
    };

    return (
        <div className="space-y-5 rounded-xl border-border/35 bg-card p-6 text-card-foreground shadow-md">
            <h2 className="flex items-center gap-2 text-lg font-bold text-card-foreground">
                <RotateCcw className="h-5 w-5" />
                {t('retry.title')}
            </h2>

            <div className="space-y-4">
                {RETRY_FIELDS.map((field) => (
                    <div
                        key={field.key}
                        className="flex min-w-0 flex-col gap-3 rounded-lg border-border/30 bg-card p-4 shadow-sm md:flex-row md:items-center md:justify-between"
                    >
                        <div className="min-w-0 flex flex-col gap-1">
                            <span className="text-sm font-medium">{t(field.labelKey)}</span>
                            {field.hintKey ? (
                                <span className="text-xs text-muted-foreground">{t(field.hintKey)}</span>
                            ) : null}
                        </div>
                        <Input
                            type="number"
                            min={field.min}
                            max={field.max}
                            value={values[field.key] ?? ''}
                            onChange={(e) => setValues((prev) => ({ ...prev, [field.key]: e.target.value }))}
                            onBlur={() => handleSave(field.key)}
                            placeholder={t(field.placeholderKey)}
                            className="w-full rounded-xl md:w-48"
                        />
                    </div>
                ))}
            </div>

            <div className="flex min-w-0 flex-col gap-3 rounded-lg border-border/30 bg-card p-4 shadow-sm md:flex-row md:items-center md:justify-between">
                <div className="min-w-0 flex flex-col gap-1">
                    <span className="text-sm font-medium">{t('retry.emptyOutput.label')}</span>
                    <span className="text-xs text-muted-foreground">{t('retry.emptyOutput.hint')}</span>
                </div>
                <Switch
                    checked={values[SettingKey.RetryEmptyOutput] === 'true'}
                    onCheckedChange={(checked) => {
                        const value = checked ? 'true' : 'false';
                        setValues((prev) => ({ ...prev, [SettingKey.RetryEmptyOutput]: value }));
                        setSetting.mutate(
                            { key: SettingKey.RetryEmptyOutput, value },
                            {
                                onSuccess: () => {
                                    toast.success(t('saved'));
                                    initialValues.current = {
                                        ...initialValues.current,
                                        [SettingKey.RetryEmptyOutput]: value,
                                    };
                                },
                            },
                        );
                    }}
                />
            </div>
            <div className="space-y-4 rounded-lg border-border/30 bg-card p-4 shadow-sm">
                <div className="flex min-w-0 flex-col gap-3 md:flex-row md:items-center md:justify-between">
                    <div className="min-w-0 flex flex-col gap-1">
                        <span className="text-sm font-medium">{t('retry.rateLimitHold.label')}</span>
                        <span className="text-xs text-muted-foreground">{t('retry.rateLimitHold.hint')}</span>
                    </div>
                    <Switch
                        checked={values[SettingKey.RateLimitHoldEnabled] === 'true'}
                        onCheckedChange={(checked) => {
                            const value = checked ? 'true' : 'false';
                            setValues((prev) => ({ ...prev, [SettingKey.RateLimitHoldEnabled]: value }));
                            setSetting.mutate(
                                { key: SettingKey.RateLimitHoldEnabled, value },
                                {
                                    onSuccess: () => {
                                        toast.success(t('saved'));
                                        initialValues.current = {
                                            ...initialValues.current,
                                            [SettingKey.RateLimitHoldEnabled]: value,
                                        };
                                    },
                                },
                            );
                        }}
                    />
                </div>
                {values[SettingKey.RateLimitHoldEnabled] === 'true' ? (
                    <div className="text-xs text-muted-foreground">{t('retry.rateLimitHold.enabledDetail')}</div>
                ) : null}
            </div>
            <div className="flex min-w-0 flex-col gap-3 rounded-lg border-border/30 bg-card p-4 shadow-sm md:flex-row md:items-center md:justify-between">
                <div className="min-w-0 flex flex-col gap-1">
                    <span className="text-sm font-medium">{t('retry.keySelectionStrategy.label')}</span>
                    <span className="text-xs text-muted-foreground">{t('retry.keySelectionStrategy.hint')}</span>
                </div>
                <Select
                    value={values[SettingKey.KeySelectionStrategy] || 'cost'}
                    onValueChange={(value) => {
                        setValues((prev) => ({ ...prev, [SettingKey.KeySelectionStrategy]: value }));
                        setSetting.mutate(
                            { key: SettingKey.KeySelectionStrategy, value },
                            {
                                onSuccess: () => {
                                    toast.success(t('saved'));
                                    initialValues.current = {
                                        ...initialValues.current,
                                        [SettingKey.KeySelectionStrategy]: value,
                                    };
                                },
                            },
                        );
                    }}
                >
                    <SelectTrigger className="w-full rounded-xl md:w-48">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent className="rounded-xl">
                        <SelectItem className="rounded-lg" value="cost">{t('retry.keySelectionStrategy.cost')}</SelectItem>
                        <SelectItem className="rounded-lg" value="availability">{t('retry.keySelectionStrategy.availability')}</SelectItem>
                        <SelectItem className="rounded-lg" value="speed">{t('retry.keySelectionStrategy.speed')}</SelectItem>
                        <SelectItem className="rounded-lg" value="priority">{t('retry.keySelectionStrategy.priority')}</SelectItem>
                    </SelectContent>
                </Select>
            </div>

            <div className="flex min-w-0 flex-col gap-3 rounded-lg border-border/30 bg-card p-4 shadow-sm md:flex-row md:items-center md:justify-between">
                <div className="min-w-0 flex flex-col gap-1">
                    <span className="text-sm font-medium">{t('retry.reasoningBufferStrategy.label')}</span>
                    <span className="text-xs text-muted-foreground">{t('retry.reasoningBufferStrategy.hint')}</span>
                </div>
                <Select
                    value={values[SettingKey.ReasoningBufferStrategy] || 'buffer'}
                    onValueChange={(value) => {
                        setValues((prev) => ({ ...prev, [SettingKey.ReasoningBufferStrategy]: value }));
                        setSetting.mutate(
                            { key: SettingKey.ReasoningBufferStrategy, value },
                            {
                                onSuccess: () => {
                                    toast.success(t('saved'));
                                    initialValues.current = {
                                        ...initialValues.current,
                                        [SettingKey.ReasoningBufferStrategy]: value,
                                    };
                                },
                            },
                        );
                    }}
                >
                    <SelectTrigger className="w-full rounded-xl md:w-48">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent className="rounded-xl">
                        <SelectItem className="rounded-lg" value="buffer">{t('retry.reasoningBufferStrategy.buffer')}</SelectItem>
                        <SelectItem className="rounded-lg" value="immediate">{t('retry.reasoningBufferStrategy.immediate')}</SelectItem>
                    </SelectContent>
                </Select>
            </div>
            {/* 高 QPS 内存优化（日志队列丢弃策略 + 流重连功能开关） */}
            <div className="flex min-w-0 flex-col gap-3 rounded-lg border-border/30 bg-card p-4 shadow-sm md:flex-row md:items-center md:justify-between">
                <div className="min-w-0 flex flex-col gap-1">
                    <span className="text-sm font-medium">{t('retry.logQueueDropPolicy.label')}</span>
                    <span className="text-xs text-muted-foreground">{t('retry.logQueueDropPolicy.hint')}</span>
                </div>
                <Select
                    value={values[SettingKey.RelayLogQueueDropPolicy] || 'oldest'}
                    onValueChange={(value) => {
                        setValues((prev) => ({ ...prev, [SettingKey.RelayLogQueueDropPolicy]: value }));
                        setSetting.mutate(
                            { key: SettingKey.RelayLogQueueDropPolicy, value },
                            {
                                onSuccess: () => {
                                    toast.success(t('saved'));
                                    initialValues.current = {
                                        ...initialValues.current,
                                        [SettingKey.RelayLogQueueDropPolicy]: value,
                                    };
                                },
                            },
                        );
                    }}
                >
                    <SelectTrigger className="w-full rounded-xl md:w-48">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent className="rounded-xl">
                        <SelectItem className="rounded-lg" value="disabled">{t('retry.logQueueDropPolicy.disabled')}</SelectItem>
                        <SelectItem className="rounded-lg" value="oldest">{t('retry.logQueueDropPolicy.oldest')}</SelectItem>
                        <SelectItem className="rounded-lg" value="newest">{t('retry.logQueueDropPolicy.newest')}</SelectItem>
                    </SelectContent>
                </Select>
            </div>
            <div className="flex min-w-0 flex-col gap-3 rounded-lg border-border/30 bg-card p-4 shadow-sm md:flex-row md:items-center md:justify-between">
                <div className="min-w-0 flex flex-col gap-1">
                    <span className="text-sm font-medium">{t('retry.streamReplay.label')}</span>
                    <span className="text-xs text-muted-foreground">{t('retry.streamReplay.hint')}</span>
                </div>
                <Switch
                    checked={values[SettingKey.StreamSessionReplayEnabled] === 'true'}
                    onCheckedChange={(checked) => {
                        const value = checked ? 'true' : 'false';
                        setValues((prev) => ({ ...prev, [SettingKey.StreamSessionReplayEnabled]: value }));
                        setSetting.mutate(
                            { key: SettingKey.StreamSessionReplayEnabled, value },
                            {
                                onSuccess: () => {
                                    toast.success(t('saved'));
                                    initialValues.current = {
                                        ...initialValues.current,
                                        [SettingKey.StreamSessionReplayEnabled]: value,
                                    };
                                },
                            },
                        );
                    }}
                />
            </div>
            {/* 定时 Key 可用性巡检（issue #142） */}
            <div className="space-y-4 rounded-lg border-border/30 bg-card p-4 shadow-sm">
                <div className="flex min-w-0 flex-col gap-3 md:flex-row md:items-center md:justify-between">
                    <div className="min-w-0 flex flex-col gap-1">
                        <span className="text-sm font-medium">{t('retry.keyHealth.label')}</span>
                        <span className="text-xs text-muted-foreground">{t('retry.keyHealth.hint')}</span>
                    </div>
                    <Switch
                        checked={values[SettingKey.KeyHealthCheckEnabled] === 'true'}
                        onCheckedChange={(checked) => {
                            const value = checked ? 'true' : 'false';
                            setValues((prev) => ({ ...prev, [SettingKey.KeyHealthCheckEnabled]: value }));
                            setSetting.mutate(
                                { key: SettingKey.KeyHealthCheckEnabled, value },
                                {
                                    onSuccess: () => {
                                        toast.success(t('saved'));
                                        initialValues.current = { ...initialValues.current, [SettingKey.KeyHealthCheckEnabled]: value };
                                    },
                                },
                            );
                        }}
                    />
                </div>
                {values[SettingKey.KeyHealthCheckEnabled] === 'true' ? (
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
                        <div className="flex flex-col gap-1">
                            <label className="text-xs text-muted-foreground">{t('retry.keyHealth.interval')}</label>
                            <Input
                                type="number"
                                min={1}
                                value={values[SettingKey.KeyHealthCheckInterval] ?? '30'}
                                onChange={(e) => setValues((prev) => ({ ...prev, [SettingKey.KeyHealthCheckInterval]: e.target.value }))}
                                onBlur={() => handleSave(SettingKey.KeyHealthCheckInterval)}
                                className="h-9 rounded-lg"
                            />
                        </div>
                        <div className="flex flex-col gap-1">
                            <label className="text-xs text-muted-foreground">{t('retry.keyHealth.failThreshold')}</label>
                            <Input
                                type="number"
                                min={1}
                                value={values[SettingKey.KeyHealthCheckFailThreshold] ?? '3'}
                                onChange={(e) => setValues((prev) => ({ ...prev, [SettingKey.KeyHealthCheckFailThreshold]: e.target.value }))}
                                onBlur={() => handleSave(SettingKey.KeyHealthCheckFailThreshold)}
                                className="h-9 rounded-lg"
                            />
                        </div>
                        <div className="flex flex-col gap-1">
                            <label className="text-xs text-muted-foreground">{t('retry.keyHealth.notifyCooldown')}</label>
                            <Input
                                type="number"
                                min={1}
                                value={values[SettingKey.KeyHealthCheckNotifyCooldown] ?? '300'}
                                onChange={(e) => setValues((prev) => ({ ...prev, [SettingKey.KeyHealthCheckNotifyCooldown]: e.target.value }))}
                                onBlur={() => handleSave(SettingKey.KeyHealthCheckNotifyCooldown)}
                                className="h-9 rounded-lg"
                            />
                        </div>
                        <div className="flex flex-col gap-2">
                            <label className="flex items-center gap-2 text-xs text-muted-foreground">
                                <Switch
                                    checked={values[SettingKey.KeyHealthCheckNotifyEnabled] !== 'false'}
                                    onCheckedChange={(checked) => {
                                        const value = checked ? 'true' : 'false';
                                        setValues((prev) => ({ ...prev, [SettingKey.KeyHealthCheckNotifyEnabled]: value }));
                                        setSetting.mutate({ key: SettingKey.KeyHealthCheckNotifyEnabled, value }, { onSuccess: () => { toast.success(t('saved')); initialValues.current = { ...initialValues.current, [SettingKey.KeyHealthCheckNotifyEnabled]: value }; } });
                                    }}
                                />
                                {t('retry.keyHealth.notifyEnabled')}
                            </label>
                            <label className="flex items-center gap-2 text-xs text-muted-foreground">
                                <Switch
                                    checked={values[SettingKey.KeyHealthCheckRecoveryNotify] !== 'false'}
                                    onCheckedChange={(checked) => {
                                        const value = checked ? 'true' : 'false';
                                        setValues((prev) => ({ ...prev, [SettingKey.KeyHealthCheckRecoveryNotify]: value }));
                                        setSetting.mutate({ key: SettingKey.KeyHealthCheckRecoveryNotify, value }, { onSuccess: () => { toast.success(t('saved')); initialValues.current = { ...initialValues.current, [SettingKey.KeyHealthCheckRecoveryNotify]: value }; } });
                                    }}
                                />
                                {t('retry.keyHealth.recoveryNotify')}
                            </label>
                        </div>
                    </div>
                ) : null}
            </div>
        </div>
    );
}
