'use client';

import { useMemo, useState } from 'react';
import { Link2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import type { Channel } from '@/api/endpoints/channel';
import { useAPIKeyList } from '@/api/endpoints/apikey';
import { useGroupList } from '@/api/endpoints/group';
import { CopyIconButton } from '@/components/common/CopyButton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';
import {
    buildCCSwitchProviderLink,
    CC_SWITCH_APPS,
    maskCCSwitchSecret,
    normalizeCCSwitchEndpoint,
    type CCSwitchApp,
} from './ccswitch';

export function CCSwitchProviderLink({
    channel,
    publicApiBaseUrl,
    sectionClassName,
}: {
    channel: Channel;
    publicApiBaseUrl: string;
    sectionClassName: string;
}) {
    const t = useTranslations('channel.detail');
    const { data: groups = [] } = useGroupList();
    const { data: apiKeys = [] } = useAPIKeyList();
    const [selectedApp, setSelectedApp] = useState<CCSwitchApp>('claude');
    const [selectedGroupID, setSelectedGroupID] = useState('');
    const [selectedAPIKeyID, setSelectedAPIKeyID] = useState('');

    const availableAPIKeys = useMemo(
        () => apiKeys.filter((key) => key.enabled && key.api_key.trim()),
        [apiKeys],
    );
    const selectableGroups = useMemo(
        () => groups.filter((group) => typeof group.id === 'number'),
        [groups],
    );

    const resolvedGroupID = useMemo(() => {
        if (selectableGroups.length === 0) return '';
        if (selectableGroups.some((group) => String(group.id) === selectedGroupID)) return selectedGroupID;
        const defaultGroup = selectableGroups.find((group) => group.id === channel.group_id) ?? selectableGroups[0];
        return String(defaultGroup.id);
    }, [channel.group_id, selectableGroups, selectedGroupID]);

    const resolvedAPIKeyID = useMemo(() => {
        if (availableAPIKeys.length === 0) return '';
        if (availableAPIKeys.some((key) => String(key.id) === selectedAPIKeyID)) return selectedAPIKeyID;
        return String(availableAPIKeys[0].id);
    }, [availableAPIKeys, selectedAPIKeyID]);

    const selectedGroup = useMemo(
        () => selectableGroups.find((group) => String(group.id) === resolvedGroupID),
        [selectableGroups, resolvedGroupID],
    );
    const selectedAPIKey = useMemo(
        () => availableAPIKeys.find((key) => String(key.id) === resolvedAPIKeyID),
        [availableAPIKeys, resolvedAPIKeyID],
    );

    const endpoint = normalizeCCSwitchEndpoint(publicApiBaseUrl);
    const generatedLink = useMemo(() => {
        if (!endpoint || !selectedGroup || !selectedAPIKey) return '';

        const notes = [
            `Octopus channel: ${channel.name}`,
            selectedGroup.endpoint_type ? `endpoint: ${selectedGroup.endpoint_type}` : '',
            selectedGroup.items?.length ? `${selectedGroup.items.length} routes` : '',
        ].filter(Boolean).join(', ');

        return buildCCSwitchProviderLink({
            app: selectedApp,
            endpoint,
            apiKey: selectedAPIKey.api_key,
            name: selectedGroup.name,
            model: selectedGroup.name,
            notes,
        });
    }, [channel.name, endpoint, selectedAPIKey, selectedApp, selectedGroup]);

    const missingReason = !endpoint
        ? t('ccSwitch.missingPublicApiBaseUrl')
        : selectableGroups.length === 0
            ? t('ccSwitch.missingGroup')
            : availableAPIKeys.length === 0
                ? t('ccSwitch.missingAPIKey')
                : '';

    return (
        <section className={sectionClassName}>
            <h4 className="mb-3 flex items-center gap-2 text-xs font-semibold text-muted-foreground">
                <Link2 className="size-3.5" />
                {t('sections.ccSwitch')}
            </h4>
            <div className="space-y-4">
                <p className="text-sm text-muted-foreground">{t('ccSwitch.description')}</p>

                <div className="grid gap-3 lg:grid-cols-[1fr_1fr]">
                    <div className="space-y-1.5">
                        <div className="text-xs font-medium text-muted-foreground">{t('ccSwitch.app')}</div>
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
                        <div className="text-xs font-medium text-muted-foreground">{t('ccSwitch.group')}</div>
                        <Select value={resolvedGroupID} onValueChange={setSelectedGroupID} disabled={selectableGroups.length === 0}>
                            <SelectTrigger className="w-full">
                                <SelectValue placeholder={t('ccSwitch.selectGroup')} />
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

                    <div className="space-y-1.5 lg:col-span-2">
                        <div className="text-xs font-medium text-muted-foreground">{t('ccSwitch.apiKey')}</div>
                        <Select
                            value={resolvedAPIKeyID}
                            onValueChange={setSelectedAPIKeyID}
                            disabled={availableAPIKeys.length === 0}
                        >
                            <SelectTrigger className="w-full">
                                <SelectValue placeholder={t('ccSwitch.selectAPIKey')} />
                            </SelectTrigger>
                            <SelectContent>
                                {availableAPIKeys.map((key) => (
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
                        <dt className="mb-1 text-xs text-muted-foreground">{t('ccSwitch.endpoint')}</dt>
                        <dd className="break-all font-mono text-xs sm:text-sm">{endpoint || '-'}</dd>
                    </div>
                    <div className="rounded-lg border border-border/25 bg-card p-3 shadow-sm">
                        <dt className="mb-1 text-xs text-muted-foreground">{t('ccSwitch.model')}</dt>
                        <dd className="break-all font-mono text-xs sm:text-sm">{selectedGroup?.name || '-'}</dd>
                    </div>
                </dl>

                {generatedLink ? (
                    <div className="flex items-start gap-2 rounded-lg border border-border/25 bg-card p-3 shadow-sm">
                        <code className="min-w-0 flex-1 break-all font-mono text-xs leading-5 sm:text-sm">{generatedLink}</code>
                        <CopyIconButton
                            text={generatedLink}
                            className="shrink-0 rounded-lg p-2 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                            copyIconClassName="size-4"
                            checkIconClassName="size-4 text-emerald-500"
                        />
                    </div>
                ) : (
                    <div className="rounded-lg border border-dashed border-border/30 bg-card p-3 text-sm text-muted-foreground shadow-sm">
                        {missingReason}
                    </div>
                )}
            </div>
        </section>
    );
}
