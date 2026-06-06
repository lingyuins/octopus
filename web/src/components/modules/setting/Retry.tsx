'use client';

import { useEffect, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { RotateCcw } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { useSettingList, useSetSetting } from '@/api/endpoints/setting';
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
        </div>
    );
}
