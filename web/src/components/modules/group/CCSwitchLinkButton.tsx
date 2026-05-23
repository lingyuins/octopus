'use client';

import { useMemo, useState } from 'react';
import { ExternalLink, Link } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useAPIKeyList } from '@/api/endpoints/apikey';
import { useGroupList } from '@/api/endpoints/group';
import { SettingKey, useSettingList } from '@/api/endpoints/setting';
import { CopyIconButton } from '@/components/common/CopyButton';
import { Button } from '@/components/ui/button';
import {
    MorphingDialog,
    MorphingDialogClose,
    MorphingDialogContainer,
    MorphingDialogContent,
    MorphingDialogDescription,
    MorphingDialogTitle,
    MorphingDialogTrigger,
} from '@/components/ui/morphing-dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';
import {
    buildCCSwitchProviderLink,
    CC_SWITCH_APPS,
    maskCCSwitchSecret,
    normalizeCCSwitchEndpoint,
    type CCSwitchApp,
} from '../channel/ccswitch';

export function CCSwitchLinkButton({ className }: { className?: string }) {
    const t = useTranslations('group');
    const { data: groups = [] } = useGroupList();
    const { data: apiKeys = [] } = useAPIKeyList();
    const { data: settings = [] } = useSettingList();
    const [selectedGroupId, setSelectedGroupId] = useState('');
    const [selectedApp, setSelectedApp] = useState<CCSwitchApp>('claude');
    const [selectedApiKeyId, setSelectedApiKeyId] = useState('');

    const endpoint = normalizeCCSwitchEndpoint(
        settings.find((item) => item.key === SettingKey.PublicAPIBaseURL)?.value ?? '',
    );

    const availableApiKeys = useMemo(
        () => apiKeys.filter((key) => key.enabled && key.api_key.trim()),
        [apiKeys],
    );
    const selectableGroups = useMemo(
        () => groups.filter((group) => typeof group.id === 'number'),
        [groups],
    );

    const resolvedGroupId = useMemo(() => {
        if (selectableGroups.length === 0) return '';
        if (selectableGroups.some((group) => String(group.id) === selectedGroupId)) return selectedGroupId;
        return String(selectableGroups[0].id);
    }, [selectableGroups, selectedGroupId]);

    const resolvedApiKeyId = useMemo(() => {
        if (availableApiKeys.length === 0) return '';
        if (availableApiKeys.some((key) => String(key.id) === selectedApiKeyId)) return selectedApiKeyId;
        return String(availableApiKeys[0].id);
    }, [availableApiKeys, selectedApiKeyId]);

    const selectedGroup = useMemo(
        () => selectableGroups.find((group) => String(group.id) === resolvedGroupId),
        [selectableGroups, resolvedGroupId],
    );
    const selectedApiKey = useMemo(
        () => availableApiKeys.find((key) => String(key.id) === resolvedApiKeyId),
        [availableApiKeys, resolvedApiKeyId],
    );

    const generatedLink = useMemo(() => {
        if (!endpoint || !selectedGroup || !selectedApiKey) return '';

        const notes = [
            'Octopus route group',
            selectedGroup.endpoint_type ? `endpoint: ${selectedGroup.endpoint_type}` : '',
            selectedGroup.items?.length ? `${selectedGroup.items.length} routes` : '',
        ].filter(Boolean).join(', ');

        return buildCCSwitchProviderLink({
            app: selectedApp,
            endpoint,
            apiKey: selectedApiKey.api_key,
            name: selectedGroup.name,
            model: selectedGroup.name,
            notes,
        });
    }, [endpoint, selectedApiKey, selectedApp, selectedGroup]);

    const missingReason = !endpoint
        ? t('ccswitch.missingPublicApiBaseUrl')
        : selectableGroups.length === 0
            ? t('ccswitch.missingGroup')
            : availableApiKeys.length === 0
                ? t('ccswitch.missingAPIKey')
                : '';

    return (
        <MorphingDialog>
            <MorphingDialogTrigger
                className={cn(
                    'inline-flex h-11 items-center gap-2 rounded-lg border border-border bg-card px-3.5 text-sm font-medium text-muted-foreground transition-colors hover:border-primary/20 hover:text-foreground',
                    className,
                )}
            >
                <Link className="size-4" />
                <span className="hidden sm:inline">CCswitch</span>
            </MorphingDialogTrigger>

            <MorphingDialogContainer>
                <MorphingDialogContent className="flex max-h-[calc(100dvh-1rem)] w-[min(100vw-1rem,34rem)] max-w-full flex-col overflow-hidden rounded-xl bg-card px-5 py-5 text-card-foreground">
                    <MorphingDialogTitle className="shrink-0">
                        <h2 className="flex items-center gap-2 text-lg font-bold text-card-foreground">
                            <ExternalLink className="size-5" />
                            {t('ccswitch.title')}
                        </h2>
                        <p className="mt-1 text-sm text-muted-foreground">
                            {t('ccswitch.description')}
                        </p>
                    </MorphingDialogTitle>

                    <MorphingDialogDescription className="min-h-0 flex-1 space-y-4 overflow-y-auto pr-1 pt-4">
                        <div className="grid gap-3 sm:grid-cols-2">
                            <div className="space-y-1.5">
                                <label className="text-xs font-medium text-muted-foreground">
                                    {t('ccswitch.appLabel')}
                                </label>
                                <div className="flex flex-wrap gap-1.5">
                                    {CC_SWITCH_APPS.map((app) => (
                                        <button
                                            key={app.value}
                                            type="button"
                                            onClick={() => setSelectedApp(app.value)}
                                            className={cn(
                                                'rounded-lg border px-2.5 py-1.5 text-xs font-medium transition-colors',
                                                selectedApp === app.value
                                                    ? 'border-primary/30 bg-primary/10 text-primary'
                                                    : 'border-border/40 bg-card text-muted-foreground hover:text-foreground',
                                            )}
                                        >
                                            {app.label}
                                        </button>
                                    ))}
                                </div>
                            </div>

                            <div className="space-y-1.5">
                                <label className="text-xs font-medium text-muted-foreground">
                                    {t('ccswitch.groupLabel')}
                                </label>
                                <Select value={resolvedGroupId} onValueChange={setSelectedGroupId} disabled={selectableGroups.length === 0}>
                                    <SelectTrigger className="w-full">
                                        <SelectValue placeholder={t('ccswitch.selectGroup')} />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {selectableGroups.map((group) => (
                                            <SelectItem key={group.id} value={String(group.id)}>
                                                {group.name}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>

                            <div className="space-y-1.5 sm:col-span-2">
                                <label className="text-xs font-medium text-muted-foreground">
                                    {t('ccswitch.apiKeyLabel')}
                                </label>
                                <Select
                                    value={resolvedApiKeyId}
                                    onValueChange={setSelectedApiKeyId}
                                    disabled={availableApiKeys.length === 0}
                                >
                                    <SelectTrigger className="w-full">
                                        <SelectValue placeholder={t('ccswitch.selectAPIKey')} />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {availableApiKeys.map((key) => (
                                            <SelectItem key={key.id} value={String(key.id)}>
                                                {key.name || `Key ${key.id}`} - {maskCCSwitchSecret(key.api_key)}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>
                        </div>

                        <dl className="grid gap-3 sm:grid-cols-2">
                            <div className="rounded-lg border border-border/25 bg-card p-3 shadow-sm">
                                <dt className="mb-1 text-xs text-muted-foreground">{t('ccswitch.endpoint')}</dt>
                                <dd className="break-all font-mono text-xs sm:text-sm">{endpoint || '-'}</dd>
                            </div>
                            <div className="rounded-lg border border-border/25 bg-card p-3 shadow-sm">
                                <dt className="mb-1 text-xs text-muted-foreground">{t('ccswitch.model')}</dt>
                                <dd className="break-all font-mono text-xs sm:text-sm">{selectedGroup?.name || '-'}</dd>
                            </div>
                        </dl>

                        {generatedLink ? (
                            <div className="space-y-2 rounded-lg border border-border/30 bg-muted/30 p-3">
                                <div className="text-xs font-medium text-muted-foreground">
                                    {t('ccswitch.generatedLink')}
                                </div>
                                <div className="flex items-start gap-2 rounded-md border border-border/30 bg-card p-2.5">
                                    <p className="min-w-0 flex-1 break-all font-mono text-xs leading-relaxed text-foreground">
                                        {generatedLink}
                                    </p>
                                    <CopyIconButton
                                        text={generatedLink}
                                        className="shrink-0 rounded-lg p-2 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                                        copyIconClassName="size-4"
                                        checkIconClassName="size-4 text-emerald-500"
                                    />
                                </div>
                            </div>
                        ) : (
                            <div className="rounded-lg border border-dashed border-border/30 bg-card p-3 text-sm text-muted-foreground shadow-sm">
                                {missingReason}
                            </div>
                        )}
                    </MorphingDialogDescription>

                    <div className="mt-4 shrink-0">
                        <MorphingDialogClose className="w-full">
                            <Button variant="secondary" className="w-full rounded-lg">
                                {t('detail.actions.cancel')}
                            </Button>
                        </MorphingDialogClose>
                    </div>
                </MorphingDialogContent>
            </MorphingDialogContainer>
        </MorphingDialog>
    );
}
