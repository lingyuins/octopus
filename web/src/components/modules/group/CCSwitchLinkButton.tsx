'use client';

import { useMemo, useState } from 'react';
import { ExternalLink, Link } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useAPIKeyList } from '@/api/endpoints/apikey';
import { useGroupList } from '@/api/endpoints/group';
import { SettingKey, useSettingList } from '@/api/endpoints/setting';
import { CopyIconButton } from '@/components/common/CopyButton';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
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
    const [nameEdited, setNameEdited] = useState(false);
    const [customName, setCustomName] = useState('');
    const [haikuModel, setHaikuModel] = useState('');
    const [sonnetModel, setSonnetModel] = useState('');
    const [opusModel, setOpusModel] = useState('');

    const publicBaseUrl = settings.find((item) => item.key === SettingKey.PublicAPIBaseURL)?.value ?? '';
    const homepage = publicBaseUrl.trim().replace(/\/+$/, '');
    const endpoint = normalizeCCSwitchEndpoint(publicBaseUrl);

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

    const groupOptions = useMemo(
        () => selectableGroups.map((group) => ({ value: group.name, label: group.name })),
        [selectableGroups],
    );
    const validGroupNames = useMemo(() => new Set(groupOptions.map((option) => option.value)), [groupOptions]);

    const selectedGroupName = selectedGroup?.name ?? '';
    const autoName = selectedGroupName ? `octopus_${selectedApp}_${selectedGroupName}` : '';
    const effectiveName = nameEdited ? customName : autoName;
    const effectiveHaikuModel = validGroupNames.has(haikuModel) ? haikuModel : '';
    const effectiveSonnetModel = validGroupNames.has(sonnetModel) ? sonnetModel : '';
    const effectiveOpusModel = validGroupNames.has(opusModel) ? opusModel : '';

    const generatedLink = useMemo(() => {
        if (!endpoint || !selectedGroup || !selectedApiKey || !effectiveName.trim()) return '';

        const notes = [
            'Octopus route group',
            selectedGroup.endpoint_type ? `endpoint: ${selectedGroup.endpoint_type}` : '',
            selectedGroup.items?.length ? `${selectedGroup.items.length} routes` : '',
        ].filter(Boolean).join(', ');

        return buildCCSwitchProviderLink({
            app: selectedApp,
            endpoint,
            homepage,
            apiKey: selectedApiKey.api_key,
            name: effectiveName,
            model: selectedGroup.name,
            haikuModel: effectiveHaikuModel,
            sonnetModel: effectiveSonnetModel,
            opusModel: effectiveOpusModel,
            notes,
        });
    }, [effectiveHaikuModel, effectiveName, effectiveOpusModel, effectiveSonnetModel, endpoint, homepage, selectedApiKey, selectedApp, selectedGroup]);

    const missingReason = !endpoint
        ? t('ccswitch.missingPublicApiBaseUrl')
        : selectableGroups.length === 0
            ? t('ccswitch.missingGroup')
            : availableApiKeys.length === 0
                ? t('ccswitch.missingAPIKey')
                : '';

    const handleImport = () => {
        if (!generatedLink) return;
        window.open(generatedLink, '_blank');
    };

    return (
        <MorphingDialog>
            <MorphingDialogTrigger
                className={cn(
                    'inline-flex h-11 items-center gap-2 rounded-lg border border-border bg-card px-3.5 text-sm font-medium text-muted-foreground transition-colors hover:border-primary/20 hover:text-foreground',
                    className,
                )}
            >
                <Link className="size-4" />
                <span className="hidden sm:inline">CC Switch</span>
            </MorphingDialogTrigger>

            <MorphingDialogContainer>
                <MorphingDialogContent className="flex max-h-[calc(100dvh-1rem)] w-[min(100vw-1rem,40rem)] max-w-full flex-col overflow-hidden rounded-xl bg-card px-5 py-5 text-card-foreground">
                    <MorphingDialogTitle className="shrink-0">
                        <h2 className="flex items-center gap-2 text-lg font-bold text-card-foreground">
                            <ExternalLink className="size-5" />
                            {t('ccswitch.title')}
                        </h2>
                    </MorphingDialogTitle>

                    <MorphingDialogDescription className="mt-3 min-h-0 flex-1 overflow-y-auto pr-1">
                        <div className="space-y-4">
                            <p className="text-sm leading-6 text-muted-foreground">
                                {t('ccswitch.description')}
                            </p>

                            <div className="space-y-1.5">
                                <label className="text-xs font-medium text-muted-foreground">
                                    {t('ccswitch.appLabel')}
                                </label>
                                <div className="grid grid-cols-2 gap-2 sm:grid-cols-5">
                                    {CC_SWITCH_APPS.map((app) => (
                                        <button
                                            key={app.value}
                                            type="button"
                                            onClick={() => setSelectedApp(app.value)}
                                            className={cn(
                                                'rounded-lg border px-3 py-2 text-xs font-medium transition-colors',
                                                selectedApp === app.value
                                                    ? 'border-primary/25 bg-primary text-primary-foreground'
                                                    : 'border-border/25 bg-card text-foreground hover:border-primary/20 hover:text-foreground',
                                            )}
                                        >
                                            {app.label}
                                        </button>
                                    ))}
                                </div>
                            </div>

                            <div className="grid gap-4 sm:grid-cols-2">
                                <div className="space-y-1.5">
                                    <label className="text-xs font-medium text-muted-foreground">
                                        {t('ccswitch.groupLabel')}
                                    </label>
                                    <Select
                                        value={resolvedGroupId}
                                        onValueChange={setSelectedGroupId}
                                        disabled={selectableGroups.length === 0}
                                    >
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

                                <div className="space-y-1.5">
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

                            <div className="space-y-1.5">
                                <label className="text-xs font-medium text-muted-foreground">
                                    {t('ccswitch.nameLabel')}
                                </label>
                                <Input
                                    value={effectiveName}
                                    onChange={(event) => {
                                        setNameEdited(true);
                                        setCustomName(event.target.value);
                                    }}
                                    placeholder={t('ccswitch.namePlaceholder')}
                                />
                            </div>

                            {selectedApp === 'claude' ? (
                                <div className="grid gap-4 sm:grid-cols-3">
                                    <div className="space-y-1.5">
                                        <label className="text-xs font-medium text-muted-foreground">{t('ccswitch.haikuModelLabel')}</label>
                                        <Select value={effectiveHaikuModel} onValueChange={setHaikuModel}>
                                            <SelectTrigger className="w-full">
                                                <SelectValue placeholder={t('ccswitch.selectModel')} />
                                            </SelectTrigger>
                                            <SelectContent>
                                                {groupOptions.map((group) => (
                                                    <SelectItem key={`haiku-${group.value}`} value={group.value}>
                                                        {group.label}
                                                    </SelectItem>
                                                ))}
                                            </SelectContent>
                                        </Select>
                                    </div>
                                    <div className="space-y-1.5">
                                        <label className="text-xs font-medium text-muted-foreground">{t('ccswitch.sonnetModelLabel')}</label>
                                        <Select value={effectiveSonnetModel} onValueChange={setSonnetModel}>
                                            <SelectTrigger className="w-full">
                                                <SelectValue placeholder={t('ccswitch.selectModel')} />
                                            </SelectTrigger>
                                            <SelectContent>
                                                {groupOptions.map((group) => (
                                                    <SelectItem key={`sonnet-${group.value}`} value={group.value}>
                                                        {group.label}
                                                    </SelectItem>
                                                ))}
                                            </SelectContent>
                                        </Select>
                                    </div>
                                    <div className="space-y-1.5">
                                        <label className="text-xs font-medium text-muted-foreground">{t('ccswitch.opusModelLabel')}</label>
                                        <Select value={effectiveOpusModel} onValueChange={setOpusModel}>
                                            <SelectTrigger className="w-full">
                                                <SelectValue placeholder={t('ccswitch.selectModel')} />
                                            </SelectTrigger>
                                            <SelectContent>
                                                {groupOptions.map((group) => (
                                                    <SelectItem key={`opus-${group.value}`} value={group.value}>
                                                        {group.label}
                                                    </SelectItem>
                                                ))}
                                            </SelectContent>
                                        </Select>
                                    </div>
                                </div>
                            ) : null}

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
                                <div className="space-y-3 rounded-lg border border-border/30 bg-muted/30 p-3">
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
                                    <Button type="button" onClick={handleImport} className="w-full rounded-lg gap-2">
                                        <ExternalLink className="size-4" />
                                        {t('ccswitch.import')}
                                    </Button>
                                </div>
                            ) : (
                                <div className="rounded-lg border border-dashed border-border/30 bg-card p-3 text-sm text-muted-foreground shadow-sm">
                                    {missingReason}
                                </div>
                            )}
                        </div>
                    </MorphingDialogDescription>

                    <div className="mt-4 flex shrink-0 justify-end">
                        <MorphingDialogClose className="relative right-0 top-0 inline-flex h-9 items-center justify-center rounded-lg border border-transparent bg-secondary px-4 py-2 text-sm font-medium text-secondary-foreground opacity-100 transition-[color,background-color,border-color,box-shadow,opacity,transform] duration-100 ease-out hover:bg-secondary/82 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/40 focus-visible:outline-none active:scale-[0.98]">
                            {t('detail.actions.cancel')}
                        </MorphingDialogClose>
                    </div>
                </MorphingDialogContent>
            </MorphingDialogContainer>
        </MorphingDialog>
    );
}
