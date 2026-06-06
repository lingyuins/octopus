'use client';

import { useEffect, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { ShieldAlert, ListFilter, AlertTriangle, MessageSquareWarning, Ban, Replace } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { SettingKey, useSetSetting, useSettingList } from '@/api/endpoints/setting';
import { toast } from '@/components/common/Toast';

const defaultSettingValues: Record<string, string> = {
    [SettingKey.ResponseFilterEnabled]: 'false',
    [SettingKey.ResponseFilterKeywords]: '[]',
    [SettingKey.ResponseFilterAction]: 'block',
    [SettingKey.ResponseFilterErrorMessage]: 'The response contains blocked keywords and has been intercepted.',
};

export function SettingResponseFilter() {
    const t = useTranslations('setting');
    const { data: settings } = useSettingList();
    const setSetting = useSetSetting();

    const [enabled, setEnabled] = useState(false);
    const [keywords, setKeywords] = useState<string[]>([]);
    const [newKeyword, setNewKeyword] = useState('');
    const [action, setAction] = useState<'block' | 'replace'>('block');
    const [errorMessage, setErrorMessage] = useState(defaultSettingValues[SettingKey.ResponseFilterErrorMessage]);

    const intendedValuesRef = useRef<Record<string, string>>({ ...defaultSettingValues });
    const loadedKeysRef = useRef<Set<string>>(new Set());
    const hasLocalIntentRef = useRef<Record<string, boolean>>({});
    const inFlightValuesRef = useRef<Record<string, string | undefined>>({});

    useEffect(() => {
        if (!settings) return;

        const nextValues: Record<string, string> = {
            [SettingKey.ResponseFilterEnabled]:
                settings.find((item) => item.key === SettingKey.ResponseFilterEnabled)?.value || 'false',
            [SettingKey.ResponseFilterKeywords]:
                settings.find((item) => item.key === SettingKey.ResponseFilterKeywords)?.value || '[]',
            [SettingKey.ResponseFilterAction]:
                settings.find((item) => item.key === SettingKey.ResponseFilterAction)?.value || 'block',
            [SettingKey.ResponseFilterErrorMessage]:
                settings.find((item) => item.key === SettingKey.ResponseFilterErrorMessage)?.value || defaultSettingValues[SettingKey.ResponseFilterErrorMessage],
        };

        const shouldApplyServerValue = (key: string, nextValue: string) => {
            if (!loadedKeysRef.current.has(key)) {
                loadedKeysRef.current.add(key);
                return true;
            }
            if (hasLocalIntentRef.current[key] && intendedValuesRef.current[key] !== nextValue) {
                return false;
            }
            hasLocalIntentRef.current[key] = false;
            return true;
        };

        queueMicrotask(() => {
            if (shouldApplyServerValue(SettingKey.ResponseFilterEnabled, nextValues[SettingKey.ResponseFilterEnabled])) {
                intendedValuesRef.current[SettingKey.ResponseFilterEnabled] = nextValues[SettingKey.ResponseFilterEnabled];
                setEnabled(nextValues[SettingKey.ResponseFilterEnabled] === 'true');
            }
            if (shouldApplyServerValue(SettingKey.ResponseFilterKeywords, nextValues[SettingKey.ResponseFilterKeywords])) {
                intendedValuesRef.current[SettingKey.ResponseFilterKeywords] = nextValues[SettingKey.ResponseFilterKeywords];
                try { setKeywords(JSON.parse(nextValues[SettingKey.ResponseFilterKeywords])); } catch { setKeywords([]); }
            }
            if (shouldApplyServerValue(SettingKey.ResponseFilterAction, nextValues[SettingKey.ResponseFilterAction])) {
                intendedValuesRef.current[SettingKey.ResponseFilterAction] = nextValues[SettingKey.ResponseFilterAction];
                setAction(nextValues[SettingKey.ResponseFilterAction] as 'block' | 'replace');
            }
            if (shouldApplyServerValue(SettingKey.ResponseFilterErrorMessage, nextValues[SettingKey.ResponseFilterErrorMessage])) {
                intendedValuesRef.current[SettingKey.ResponseFilterErrorMessage] = nextValues[SettingKey.ResponseFilterErrorMessage];
                setErrorMessage(nextValues[SettingKey.ResponseFilterErrorMessage]);
            }
        });
    }, [settings]);

    const flushSettingSave = (key: string) => {
        if (inFlightValuesRef.current[key] !== undefined || !hasLocalIntentRef.current[key]) return;

        const value = intendedValuesRef.current[key];
        inFlightValuesRef.current[key] = value;

        setSetting.mutate(
            { key, value },
            {
                onSuccess: () => { toast.success(t('saved')); },
                onError: () => {
                    if (intendedValuesRef.current[key] === value) {
                        hasLocalIntentRef.current[key] = false;
                    }
                },
                onSettled: () => {
                    delete inFlightValuesRef.current[key];
                    if (hasLocalIntentRef.current[key] && intendedValuesRef.current[key] !== value) {
                        flushSettingSave(key);
                    }
                },
            },
        );
    };

    const saveTextSetting = (key: string, value: string) => {
        if (value === intendedValuesRef.current[key]) return;
        intendedValuesRef.current[key] = value;
        hasLocalIntentRef.current[key] = true;
        flushSettingSave(key);
    };

    const saveBooleanSetting = (checked: boolean) => {
        const value = checked ? 'true' : 'false';
        setEnabled(checked);
        if (value === intendedValuesRef.current[SettingKey.ResponseFilterEnabled]) return;
        intendedValuesRef.current[SettingKey.ResponseFilterEnabled] = value;
        hasLocalIntentRef.current[SettingKey.ResponseFilterEnabled] = true;
        flushSettingSave(SettingKey.ResponseFilterEnabled);
    };

    const saveKeywords = (nextKeywords: string[]) => {
        setKeywords(nextKeywords);
        const value = JSON.stringify(nextKeywords);
        saveTextSetting(SettingKey.ResponseFilterKeywords, value);
    };

    const saveAction = (nextAction: 'block' | 'replace') => {
        setAction(nextAction);
        saveTextSetting(SettingKey.ResponseFilterAction, nextAction);
    };

    const handleAddKeyword = () => {
        const kw = newKeyword.trim();
        if (!kw) return;
        if (keywords.includes(kw)) {
            toast.error(t('responseFilter.keywords.duplicate'));
            return;
        }
        saveKeywords([...keywords, kw]);
        setNewKeyword('');
    };

    const handleRemoveKeyword = (index: number) => {
        saveKeywords(keywords.filter((_, i) => i !== index));
    };

    const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            handleAddKeyword();
        }
    };

    return (
        <div className="min-w-0 space-y-5 rounded-xl border border-border/35 bg-card p-6 text-card-foreground">
            <div className="space-y-1">
                <h2 className="flex items-center gap-2 text-lg font-bold text-card-foreground">
                    <ShieldAlert className="h-5 w-5" />
                    {t('responseFilter.title')}
                </h2>
                <p className="text-sm text-muted-foreground">
                    {t('responseFilter.description')}
                </p>
            </div>

            {/* 启用开关 */}
            <div className="flex min-w-0 flex-col gap-3 rounded-lg border border-border/30 bg-card p-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0 flex items-center gap-3">
                    <ShieldAlert className="h-5 w-5 text-muted-foreground" />
                    <span className="text-sm font-medium">{t('responseFilter.enabled.label')}</span>
                </div>
                <Switch checked={enabled} onCheckedChange={saveBooleanSetting} />
            </div>

            {/* 拦截动作 */}
            <div className="space-y-3 rounded-lg border border-border/30 bg-card p-4">
                <div className="flex items-center gap-3">
                    <AlertTriangle className="h-5 w-5 text-muted-foreground" />
                    <span className="text-sm font-medium">{t('responseFilter.action.label')}</span>
                </div>
                <div className="flex gap-2 pl-8">
                    <Button
                        type="button"
                        variant={action === 'block' ? 'default' : 'outline'}
                        size="sm"
                        className="rounded-xl"
                        onClick={() => saveAction('block')}
                    >
                        <Ban className="mr-1.5 h-4 w-4" />
                        {t('responseFilter.action.block')}
                    </Button>
                    <Button
                        type="button"
                        variant={action === 'replace' ? 'default' : 'outline'}
                        size="sm"
                        className="rounded-xl"
                        onClick={() => saveAction('replace')}
                    >
                        <Replace className="mr-1.5 h-4 w-4" />
                        {t('responseFilter.action.replace')}
                    </Button>
                </div>
                <p className="pl-8 text-xs text-muted-foreground">
                    {action === 'block'
                        ? t('responseFilter.action.blockHint')
                        : t('responseFilter.action.replaceHint')}
                </p>
            </div>

            {/* 关键词列表 */}
            <div className="space-y-3 rounded-lg border border-border/30 bg-card p-4">
                <div className="flex items-center gap-3">
                    <ListFilter className="h-5 w-5 text-muted-foreground" />
                    <span className="text-sm font-medium">{t('responseFilter.keywords.label')}</span>
                    <Badge variant="secondary" className="text-xs">{keywords.length}</Badge>
                </div>

                <div className="flex gap-2 pl-8">
                    <Input
                        value={newKeyword}
                        onChange={(e) => setNewKeyword(e.target.value)}
                        onKeyDown={handleKeyDown}
                        placeholder={t('responseFilter.keywords.placeholder')}
                        className="flex-1 rounded-xl"
                    />
                    <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="shrink-0 rounded-xl"
                        onClick={handleAddKeyword}
                        disabled={!newKeyword.trim()}
                    >
                        {t('responseFilter.keywords.add')}
                    </Button>
                </div>

                {keywords.length > 0 && (
                    <div className="flex flex-wrap gap-2 pl-8">
                        {keywords.map((kw, index) => (
                            <Badge
                                key={index}
                                variant="outline"
                                className="cursor-pointer gap-1.5 rounded-lg px-2.5 py-1 text-xs transition-colors hover:bg-destructive/10 hover:text-destructive hover:border-destructive/30"
                                onClick={() => handleRemoveKeyword(index)}
                                title={t('responseFilter.keywords.removeHint')}
                            >
                                {kw}
                                <span className="text-muted-foreground">×</span>
                            </Badge>
                        ))}
                    </div>
                )}

                {keywords.length === 0 && (
                    <p className="pl-8 text-xs text-muted-foreground">
                        {t('responseFilter.keywords.empty')}
                    </p>
                )}
            </div>

            {/* 错误信息（仅 block 模式） */}
            {action === 'block' && (
                <div className="space-y-3 rounded-lg border border-border/30 bg-card p-4">
                    <div className="flex items-center gap-3">
                        <MessageSquareWarning className="h-5 w-5 text-muted-foreground" />
                        <span className="text-sm font-medium">{t('responseFilter.errorMessage.label')}</span>
                    </div>
                    <Input
                        value={errorMessage}
                        onChange={(e) => setErrorMessage(e.target.value)}
                        onBlur={() => saveTextSetting(SettingKey.ResponseFilterErrorMessage, errorMessage)}
                        placeholder={t('responseFilter.errorMessage.placeholder')}
                        className="ml-8 w-[calc(100%-2rem)] rounded-xl"
                    />
                    <p className="pl-8 text-xs text-muted-foreground">
                        {t('responseFilter.errorMessage.hint')}
                    </p>
                </div>
            )}
        </div>
    );
}
