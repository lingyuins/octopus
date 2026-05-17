'use client';

import { useEffect, useState, useRef } from 'react';
import { useTranslations } from 'next-intl';
import { RefreshCw, Clock, DollarSign } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { useSettingList, useSetSetting, SettingKey } from '@/api/endpoints/setting';
import { toast } from '@/components/common/Toast';

export function SettingLLMSync() {
    const t = useTranslations('setting');
    const { data: settings } = useSettingList();
    const setSetting = useSetSetting();

    const [syncInterval, setSyncInterval] = useState('');
    const [updateInterval, setUpdateInterval] = useState('');
    const initialSyncInterval = useRef('');
    const initialUpdateInterval = useRef('');

    useEffect(() => {
        if (settings) {
            const syncIntervalSetting = settings.find(s => s.key === SettingKey.SyncLLMInterval);
            if (syncIntervalSetting) {
                queueMicrotask(() => setSyncInterval(syncIntervalSetting.value));
                initialSyncInterval.current = syncIntervalSetting.value;
            }

            const updateIntervalSetting = settings.find(s => s.key === SettingKey.ModelInfoUpdateInterval);
            if (updateIntervalSetting) {
                queueMicrotask(() => setUpdateInterval(updateIntervalSetting.value));
                initialUpdateInterval.current = updateIntervalSetting.value;
            }
        }
    }, [settings]);

    const handleSave = (key: string, value: string, initialValue: string) => {
        if (value === initialValue) return;

        setSetting.mutate({ key, value }, {
            onSuccess: () => {
                toast.success(t('saved'));
                if (key === SettingKey.SyncLLMInterval) {
                    initialSyncInterval.current = value;
                }
                if (key === SettingKey.ModelInfoUpdateInterval) {
                    initialUpdateInterval.current = value;
                }
            }
        });
    };

    return (
        <div className="rounded-xl border-border/35 bg-card p-6 space-y-5 text-card-foreground shadow-md ">
            <h2 className="text-lg font-bold text-card-foreground flex items-center gap-2">
                <RefreshCw className="h-5 w-5" />
                {t('llmSync.title')}
            </h2>

            <div className="flex flex-col gap-3 rounded-lg border-border/30 bg-card p-4 shadow-sm md:flex-row md:items-center md:justify-between">
                <div className="flex items-center gap-3">
                    <Clock className="h-5 w-5 text-muted-foreground" />
                    <span className="text-sm font-medium">{t('llmSync.syncInterval.label')}</span>
                </div>
                <Input
                    type="number"
                    value={syncInterval}
                    onChange={(e) => setSyncInterval(e.target.value)}
                    onBlur={() => handleSave(SettingKey.SyncLLMInterval, syncInterval, initialSyncInterval.current)}
                    placeholder={t('llmSync.syncInterval.placeholder')}
                    className="w-48 rounded-xl"
                />
            </div>

            <div className="flex flex-col gap-3 rounded-lg border-border/30 bg-card p-4 shadow-sm md:flex-row md:items-center md:justify-between">
                <div className="flex items-center gap-3">
                    <DollarSign className="h-5 w-5 text-muted-foreground" />
                    <span className="text-sm font-medium">{t('llmPrice.updateInterval.label')}</span>
                </div>
                <Input
                    type="number"
                    value={updateInterval}
                    onChange={(e) => setUpdateInterval(e.target.value)}
                    onBlur={() => handleSave(SettingKey.ModelInfoUpdateInterval, updateInterval, initialUpdateInterval.current)}
                    placeholder={t('llmPrice.updateInterval.placeholder')}
                    className="w-48 rounded-xl"
                />
            </div>
        </div>
    );
}

