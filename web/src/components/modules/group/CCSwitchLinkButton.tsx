'use client';

import { useCallback, useMemo, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Check, Copy, ExternalLink, Link } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useGroupList } from '@/api/endpoints/group';
import { cn } from '@/lib/utils';
import {
    MorphingDialog,
    MorphingDialogTrigger,
    MorphingDialogContainer,
    MorphingDialogContent,
    MorphingDialogTitle,
    MorphingDialogDescription,
    MorphingDialogClose,
} from '@/components/ui/morphing-dialog';

const CC_APPS = [
    { value: 'claude', label: 'Claude Code' },
    { value: 'codex', label: 'Codex' },
    { value: 'gemini', label: 'Gemini' },
    { value: 'opencode', label: 'OpenCode' },
    { value: 'openclaw', label: 'OpenClaw' },
] as const;

export function CCSwitchLinkButton({ className }: { className?: string }) {
    const t = useTranslations('group');
    const { data: groups = [] } = useGroupList();

    const [selectedGroupId, setSelectedGroupId] = useState<number | ''>('');
    const [selectedApp, setSelectedApp] = useState<string>(CC_APPS[0].value);
    const [apiKey, setApiKey] = useState('');
    const [generatedLink, setGeneratedLink] = useState('');
    const [copied, setCopied] = useState(false);

    const selectedGroup = useMemo(
        () => groups.find((g) => g.id === selectedGroupId),
        [groups, selectedGroupId]
    );

    const generateLink = useCallback(() => {
        if (!selectedGroup || !apiKey.trim()) return;

        const params = new URLSearchParams();
        params.set('resource', 'provider');
        params.set('app', selectedApp);
        params.set('name', selectedGroup.name);
        params.set('apiKey', apiKey.trim());

        // Use the Octopus server as the endpoint
        const baseUrl = typeof window !== 'undefined'
            ? `${window.location.protocol}//${window.location.host}`
            : '';
        params.set('endpoint', `${baseUrl}/v1`);

        // Use the group name as the default model
        params.set('model', selectedGroup.name);

        const notesParts: string[] = [];
        if (selectedGroup.endpoint_type) {
            notesParts.push(`endpoint: ${selectedGroup.endpoint_type}`);
        }
        if (selectedGroup.items?.length) {
            notesParts.push(`${selectedGroup.items.length} routes`);
        }
        if (notesParts.length > 0) {
            params.set('notes', `Octopus route: ${notesParts.join(', ')}`);
        }

        setGeneratedLink(`ccswitch://v1/import?${params.toString()}`);
    }, [selectedGroup, selectedApp, apiKey]);

    const copyLink = useCallback(async () => {
        if (!generatedLink) return;
        try {
            await navigator.clipboard.writeText(generatedLink);
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
        } catch {
            // Fallback: select text
        }
    }, [generatedLink]);

    return (
        <MorphingDialog>
            <MorphingDialogTrigger
                className={cn(
                    'inline-flex items-center gap-2 rounded-lg border border-border bg-card px-3.5 text-sm font-medium text-muted-foreground transition-colors hover:border-primary/20 hover:text-foreground h-11',
                    className,
                )}
            >
                <Link className="size-4" />
                <span className="hidden sm:inline">CCswitch</span>
            </MorphingDialogTrigger>

            <MorphingDialogContainer>
                <MorphingDialogContent className="w-[min(100vw-1rem,28rem)] max-w-full bg-card text-card-foreground px-5 py-5 rounded-xl max-h-[calc(100dvh-1rem)] flex flex-col overflow-hidden">
                    <MorphingDialogTitle className="shrink-0">
                        <h2 className="text-lg font-bold text-card-foreground flex items-center gap-2">
                            <ExternalLink className="size-5" />
                            {t('ccswitch.title')}
                        </h2>
                        <p className="mt-1 text-sm text-muted-foreground">
                            {t('ccswitch.description')}
                        </p>
                    </MorphingDialogTitle>
                    <MorphingDialogDescription className="flex-1 min-h-0 overflow-y-auto space-y-4 pr-1 pt-4">
                        {/* Group selector */}
                        <div className="space-y-1.5">
                            <label className="text-xs font-medium text-muted-foreground">
                                {t('ccswitch.groupLabel')}
                            </label>
                            <select
                                value={selectedGroupId}
                                onChange={(e) => setSelectedGroupId(e.target.value ? Number(e.target.value) : '')}
                                className="h-10 w-full rounded-lg border border-border/40 bg-card px-3 text-sm outline-none focus-visible:border-ring focus-visible:ring-4 focus-visible:ring-ring/20"
                            >
                                <option value="">{t('ccswitch.selectGroup')}</option>
                                {groups.map((g) => (
                                    <option key={g.id} value={g.id}>
                                        {g.name}
                                    </option>
                                ))}
                            </select>
                        </div>

                        {/* App selector */}
                        <div className="space-y-1.5">
                            <label className="text-xs font-medium text-muted-foreground">
                                {t('ccswitch.appLabel')}
                            </label>
                            <div className="flex flex-wrap gap-1.5">
                                {CC_APPS.map((app) => (
                                    <button
                                        key={app.value}
                                        type="button"
                                        onClick={() => setSelectedApp(app.value)}
                                        className={cn(
                                            'rounded-md px-3 py-1.5 text-xs font-medium transition-colors',
                                            selectedApp === app.value
                                                ? 'bg-primary text-primary-foreground'
                                                : 'border border-border/30 bg-card text-muted-foreground hover:text-foreground'
                                        )}
                                    >
                                        {app.label}
                                    </button>
                                ))}
                            </div>
                        </div>

                        {/* API Key */}
                        <div className="space-y-1.5">
                            <label className="text-xs font-medium text-muted-foreground">
                                {t('ccswitch.apiKeyLabel')}
                            </label>
                            <input
                                type="text"
                                value={apiKey}
                                onChange={(e) => setApiKey(e.target.value)}
                                placeholder="sk-octopus-..."
                                className="h-10 w-full rounded-lg border border-border/40 bg-card px-3 text-sm outline-none font-mono placeholder:text-muted-foreground/50 focus-visible:border-ring focus-visible:ring-4 focus-visible:ring-ring/20"
                            />
                            <p className="text-[11px] text-muted-foreground/70">
                                {t('ccswitch.apiKeyHint')}
                            </p>
                        </div>

                        {/* Generate button */}
                        <Button
                            type="button"
                            disabled={!selectedGroup || !apiKey.trim()}
                            onClick={generateLink}
                            className="w-full rounded-xl"
                        >
                            <ExternalLink className="size-4" />
                            {t('ccswitch.generate')}
                        </Button>

                        {/* Generated link */}
                        {generatedLink && (
                            <div className="space-y-2 rounded-lg border border-border/30 bg-muted/30 p-3">
                                <div className="text-xs font-medium text-muted-foreground">
                                    {t('ccswitch.generatedLink')}
                                </div>
                                <div className="rounded-md border border-border/30 bg-card p-2.5">
                                    <p className="text-xs font-mono break-all text-foreground leading-relaxed">
                                        {generatedLink}
                                    </p>
                                </div>
                                <Button
                                    type="button"
                                    variant="outline"
                                    size="sm"
                                    onClick={copyLink}
                                    className="w-full rounded-lg"
                                >
                                    {copied ? (
                                        <>
                                            <Check className="size-3.5" />
                                            {t('ccswitch.copied')}
                                        </>
                                    ) : (
                                        <>
                                            <Copy className="size-3.5" />
                                            {t('ccswitch.copy')}
                                        </>
                                    )}
                                </Button>
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
