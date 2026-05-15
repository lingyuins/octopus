import {
    AutoGroupType,
    ChannelType,
    RequestRewriteProfile,
    SystemMessageStrategy,
    ToolRoleStrategy,
    type Channel,
    type RequestRewriteConfig,
    useFetchModel,
    useTestChannel,
    type TestChannelSummary,
} from '@/api/endpoints/channel';
import { channelTemplates } from './templates';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import {
    MorphingDialog,
    MorphingDialogClose,
    MorphingDialogContainer,
    MorphingDialogContent,
    MorphingDialogDescription,
    MorphingDialogTitle,
    MorphingDialogTrigger,
    useMorphingDialog,
} from '@/components/ui/morphing-dialog';
import { toast } from '@/components/common/Toast';
import { useTranslations } from 'next-intl';
import { useEffect, useMemo, useRef, useState } from 'react';
import { RefreshCw, X, Plus, FlaskConical, CheckCircle2, AlertTriangle, Sparkles, Orbit, Layers3, KeyRound, Cable, Search, Check, ListFilter } from 'lucide-react';
import { useIsMobile } from '@/hooks/use-mobile';

export interface ChannelKeyFormItem {
    id?: number;
    enabled: boolean;
    channel_key: string;
    status_code?: number;
    last_use_time_stamp?: number;
    total_cost?: number;
    remark?: string;
}

export interface ChannelFormData {
    name: string;
    type: ChannelType;
    base_urls: Channel['base_urls'];
    custom_header: Channel['custom_header'];
    channel_proxy: string;
    param_override: string;
    request_rewrite: RequestRewriteConfig;
    keys: ChannelKeyFormItem[];
    model: string;
    custom_model: string;
    enabled: boolean;
    proxy: boolean;
    auto_sync: boolean;
    auto_group: AutoGroupType;
    match_regex: string;
}

export function createDefaultRequestRewriteFormData(): RequestRewriteConfig {
    return {
        enabled: false,
        profile: RequestRewriteProfile.OpenAIChatCompat,
        tool_role_strategy: ToolRoleStrategy.Keep,
        system_message_strategy: SystemMessageStrategy.Keep,
    };
}

export function normalizeRequestRewriteFormData(config?: RequestRewriteConfig | null): RequestRewriteConfig {
    return {
        enabled: config?.enabled ?? false,
        profile: config?.profile ?? RequestRewriteProfile.OpenAIChatCompat,
        tool_role_strategy: config?.tool_role_strategy ?? ToolRoleStrategy.Keep,
        system_message_strategy: config?.system_message_strategy ?? SystemMessageStrategy.Keep,
    };
}

export function isRequestRewriteSupportedChannelType(channelType: ChannelType): boolean {
    return channelType === ChannelType.OpenAIChat || channelType === ChannelType.Mimo;
}

export function getEffectiveRequestRewriteFormData(channelType: ChannelType, config?: RequestRewriteConfig | null): RequestRewriteConfig {
    const normalized = normalizeRequestRewriteFormData(config);
    if (isRequestRewriteSupportedChannelType(channelType)) {
        return normalized;
    }

    return {
        ...normalized,
        enabled: false,
    };
}

export interface ChannelFormProps {
    formData: ChannelFormData;
    onFormDataChange: (data: ChannelFormData) => void;
    onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
    isPending: boolean;
    submitText: string;
    pendingText: string;
    onCancel?: () => void;
    cancelText?: string;
    idPrefix?: string;
    showTemplatePicker?: boolean;
    onShowTemplatePicker?: () => void;
}

import {
    Accordion,
    AccordionContent,
    AccordionItem,
    AccordionTrigger,
} from "@/components/ui/accordion";

function SectionHeader({
    icon: Icon,
    title,
    hint,
}: {
    icon: typeof Sparkles;
    title: string;
    hint?: string;
}) {
    return (
        <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="space-y-2">
                <div className="inline-flex items-center gap-2 rounded-full border border-primary/12 bg-card px-3 py-1 text-[0.68rem] font-semibold text-primary">
                    <Icon className="size-3.5" />
                    {title}
                </div>
                {hint ? <p className="text-xs leading-5 text-muted-foreground">{hint}</p> : null}
            </div>
        </div>
    );
}

interface ModelPickerDialogPanelProps {
    models: string[];
    selectedModels: string[];
    isLoading: boolean;
    onApply: (models: string[]) => void;
}

function ModelPickerDialogPanel({ models, selectedModels, isLoading, onApply }: ModelPickerDialogPanelProps) {
    const t = useTranslations('channel.form.modelPicker');
    const { setIsOpen } = useMorphingDialog();
    const [searchTerm, setSearchTerm] = useState('');
    const [draftSelected, setDraftSelected] = useState<string[]>(selectedModels);

    useEffect(() => {
        setDraftSelected(selectedModels);
    }, [selectedModels]);

    const normalizedSearch = searchTerm.trim().toLowerCase();
    const filteredModels = normalizedSearch
        ? models.filter((model) => model.toLowerCase().includes(normalizedSearch))
        : models;
    const selectedSet = new Set(draftSelected);
    const allFilteredSelected = filteredModels.length > 0 && filteredModels.every((model) => selectedSet.has(model));

    const toggleModel = (model: string) => {
        setDraftSelected((current) =>
            current.includes(model)
                ? current.filter((item) => item !== model)
                : [...current, model]
        );
    };

    const handleSelectFiltered = () => {
        if (filteredModels.length === 0) return;

        setDraftSelected((current) => {
            const currentSet = new Set(current);
            if (filteredModels.every((model) => currentSet.has(model))) {
                return current.filter((model) => !filteredModels.includes(model));
            }
            return Array.from(new Set([...current, ...filteredModels]));
        });
    };

    const handleApply = () => {
        onApply(draftSelected);
        setIsOpen(false);
    };

    return (
        <div className="relative flex h-full min-h-0 flex-col overflow-hidden rounded-xl border border-border/35 bg-card text-card-foreground shadow-md">
            <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_16%_14%,color-mix(in_oklch,var(--primary)_18%,transparent)_0%,transparent_30%),linear-gradient(180deg,color-mix(in_oklch,white_18%,transparent),transparent_28%)]" />
            <MorphingDialogTitle className="shrink-0">
                <header className="relative flex items-center justify-between gap-4 border-b border-border/20 px-5 py-4 md:px-6">
                    <div className="min-w-0 space-y-2">
                        <div className="flex items-center gap-2">
                            <span className="h-2.5 w-10 rounded-full bg-primary/18 shadow-sm" />
                            <span className="h-2.5 w-20 rounded-full bg-card shadow-inner" />
                        </div>
                        <div className="space-y-1">
                            <h3 className="truncate text-lg font-semibold tracking-tight text-card-foreground md:text-xl">
                                {t('title')}
                            </h3>
                            <p className="text-xs text-muted-foreground">
                                {t('description', { count: models.length })}
                            </p>
                        </div>
                    </div>
                    <MorphingDialogClose className="relative right-0 top-0" />
                </header>
            </MorphingDialogTitle>

            <MorphingDialogDescription disableLayoutAnimation className="relative flex min-h-0 flex-1 flex-col gap-4 px-4 py-4 md:px-6">
                <div className="relative shrink-0">
                    <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        value={searchTerm}
                        onChange={(event) => setSearchTerm(event.target.value)}
                        placeholder={t('searchPlaceholder')}
                        className="h-11 rounded-lg pl-9"
                    />
                </div>

                <div className="flex shrink-0 flex-wrap items-center justify-between gap-2">
                    <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                        <Badge variant="secondary" className="rounded-full">
                            {t('selectedCount', { count: draftSelected.length })}
                        </Badge>
                        <span>{t('filteredCount', { count: filteredModels.length })}</span>
                    </div>
                    <div className="flex items-center gap-2">
                        <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={() => setDraftSelected([])}
                            disabled={draftSelected.length === 0}
                            className="h-8 rounded-lg px-2 text-xs text-muted-foreground hover:text-foreground"
                        >
                            {t('clear')}
                        </Button>
                        <Button
                            type="button"
                            variant="secondary"
                            size="sm"
                            onClick={handleSelectFiltered}
                            disabled={filteredModels.length === 0}
                            className="h-8 rounded-lg px-2 text-xs"
                        >
                            <ListFilter className="size-3.5" />
                            {allFilteredSelected ? t('unselectFiltered') : t('selectFiltered')}
                        </Button>
                    </div>
                </div>

                <div className="min-h-0 flex-1 overflow-y-auto rounded-lg border border-border/25 bg-card p-2 shadow-sm">
                    {isLoading ? (
                        <div className="flex h-40 items-center justify-center gap-2 text-sm text-muted-foreground">
                            <RefreshCw className="size-4 animate-spin" />
                            {t('loading')}
                        </div>
                    ) : filteredModels.length > 0 ? (
                        <div className="grid gap-2 sm:grid-cols-2">
                            {filteredModels.map((model) => {
                                const selected = selectedSet.has(model);
                                return (
                                    <button
                                        key={model}
                                        type="button"
                                        onClick={() => toggleModel(model)}
                                        className={`flex min-w-0 items-center gap-3 rounded-lg border px-3 py-2.5 text-left text-sm transition-colors ${
                                            selected
                                                ? 'border-primary/30 bg-primary/10 text-foreground'
                                                : 'border-border/25 bg-background/40 text-muted-foreground hover:border-border/60 hover:text-foreground'
                                        }`}
                                    >
                                        <span className={`flex size-5 shrink-0 items-center justify-center rounded-md border ${
                                            selected ? 'border-primary bg-primary text-primary-foreground' : 'border-border bg-card'
                                        }`}>
                                            {selected ? <Check className="size-3.5" /> : null}
                                        </span>
                                        <span className="min-w-0 flex-1 truncate font-mono" title={model}>
                                            {model}
                                        </span>
                                    </button>
                                );
                            })}
                        </div>
                    ) : (
                        <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">
                            {models.length === 0 ? t('empty') : t('noSearchResult')}
                        </div>
                    )}
                </div>

                <div className="flex shrink-0 flex-col gap-2 border-t border-border/20 pt-4 sm:flex-row">
                    <Button
                        type="button"
                        variant="secondary"
                        onClick={() => setIsOpen(false)}
                        className="h-11 rounded-lg sm:flex-1"
                    >
                        {t('cancel')}
                    </Button>
                    <Button
                        type="button"
                        onClick={handleApply}
                        className="h-11 rounded-lg sm:flex-1"
                    >
                        {t('apply', { count: draftSelected.length })}
                    </Button>
                </div>
            </MorphingDialogDescription>
        </div>
    );
}

export function TemplatePickerGrid({
    compact,
    onApplyTemplate,
}: {
    compact: boolean;
    onApplyTemplate: (templateKey: string) => void;
}) {
    const t = useTranslations('channel.form');

    return (
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
            {channelTemplates.slice(0, compact ? 4 : channelTemplates.length).map((template) => (
                <Button
                    key={template.key}
                    type="button"
                    variant="outline"
                    onClick={() => onApplyTemplate(template.key)}
                    className="h-auto min-h-20 flex-col items-start gap-1 rounded-lg border-border/30 bg-card px-3.5 py-3 text-left whitespace-normal hover:bg-card md:min-h-24 md:rounded-lg md:px-4"
                >
                    <span className="text-sm font-semibold">{template.name}</span>
                    <span className="text-xs text-muted-foreground">{t(template.descriptionKey)}</span>
                </Button>
            ))}
        </div>
    );
}

export function ChannelForm({
    formData,
    onFormDataChange,
    onSubmit,
    isPending,
    submitText,
    pendingText,
    onCancel,
    cancelText,
    idPrefix = 'channel',
    showTemplatePicker = true,
    onShowTemplatePicker,
}: ChannelFormProps) {
    const t = useTranslations('channel.form');
    const isCompactViewport = useIsMobile();
    const requestRewriteSupported = isRequestRewriteSupportedChannelType(formData.type);
    const sectionClassName = 'space-y-4 rounded-lg bg-card/70 p-4 md:p-5';
    const labelClassName = 'text-sm font-medium text-card-foreground';
    const fieldGroupClassName = 'space-y-2';

    // Ensure the form always shows at least 1 row for base_urls / keys / custom_header.
    // This avoids "empty list" UI and also keeps URL + APIKEY layout consistent.
    useEffect(() => {
        if (!formData.base_urls || formData.base_urls.length === 0) {
            onFormDataChange({ ...formData, base_urls: [{ url: '', delay: 0, suffix_mode: 'auto' }] });
            return;
        }
        if (!formData.keys || formData.keys.length === 0) {
            onFormDataChange({ ...formData, keys: [{ enabled: true, channel_key: '' }] });
            return;
        }
        if (!formData.custom_header || formData.custom_header.length === 0) {
            onFormDataChange({ ...formData, custom_header: [{ header_key: '', header_value: '' }] });
        }
    }, [formData, onFormDataChange]);

    const autoModels = formData.model
        ? formData.model.split(',').map((m) => m.trim()).filter(Boolean)
        : [];
    const customModels = formData.custom_model
        ? formData.custom_model.split(',').map((m) => m.trim()).filter(Boolean)
        : [];
    const [inputValue, setInputValue] = useState('');
    const [fetchedModels, setFetchedModels] = useState<string[]>([]);
    const inputRef = useRef<HTMLInputElement>(null);

    const fetchModel = useFetchModel();
    const testChannel = useTestChannel();
    const [testSummary, setTestSummary] = useState<TestChannelSummary | null>(null);

    const effectiveKey =
        formData.keys.find((k) => k.enabled && k.channel_key.trim())?.channel_key.trim() || '';

    const updateModels = (nextAuto: string[], nextCustom: string[]) => {
        const model = nextAuto.join(',');
        const custom_model = nextCustom.join(',');
        if (formData.model === model && formData.custom_model === custom_model) return;
        onFormDataChange({ ...formData, model, custom_model });
    };

    const applyFetchedModelSelection = (selectedModels: string[]) => {
        const customModelSet = new Set(customModels);
        const nextAutoModels = Array.from(new Set(selectedModels)).filter((model) => !customModelSet.has(model));
        updateModels(nextAutoModels, customModels);
        toast.success(t('modelPicker.applySuccess', { count: nextAutoModels.length }));
    };

    const normalizeFetchedModels = (data: unknown): string[] => {
        if (!Array.isArray(data)) return [];

        return Array.from(new Set(
            data
                .map((item) => {
                    if (typeof item === 'string') return item.trim();
                    if (!item || typeof item !== 'object') return '';

                    const candidate =
                        ('id' in item && typeof item.id === 'string' && item.id) ||
                        ('name' in item && typeof item.name === 'string' && item.name) ||
                        ('display_name' in item && typeof item.display_name === 'string' && item.display_name) ||
                        ('displayName' in item && typeof item.displayName === 'string' && item.displayName) ||
                        '';

                    return candidate.trim();
                })
                .filter(Boolean)
        ));
    };

    const normalizedHeaders = useMemo(() =>
        (formData.custom_header ?? [])
            .map((h) => ({ header_key: h.header_key.trim(), header_value: h.header_value }))
            .filter((h) => h.header_key && h.header_value !== ''),
        [formData.custom_header]
    );

    const buildTestPayload = () => ({
        type: formData.type,
        base_urls: (formData.base_urls ?? []).filter((u) => u.url.trim()).map((u) => ({
            url: u.url.trim(),
            delay: Number(u.delay || 0),
            suffix_mode: u.suffix_mode && u.suffix_mode !== 'auto' ? u.suffix_mode : undefined,
        })),
        keys: formData.keys
            .filter((k) => k.channel_key.trim())
            .map((k) => ({ enabled: k.enabled, channel_key: k.channel_key.trim(), remark: k.remark ?? '' })),
        proxy: formData.proxy,
        channel_proxy: formData.channel_proxy?.trim() || null,
        match_regex: formData.match_regex.trim() || null,
        custom_header: normalizedHeaders,
        model: formData.model,
        custom_model: formData.custom_model,
        name: formData.name,
        enabled: formData.enabled,
        auto_sync: formData.auto_sync,
        auto_group: formData.auto_group,
        param_override: formData.param_override.trim() || null,
    });

    const handleTestChannel = () => {
        setTestSummary(null);
        testChannel.mutate(buildTestPayload(), {
            onSuccess: (data) => {
                setTestSummary(data);
                toast.success(data.passed ? t('test.success') : t('test.partialSuccess'));
            },
            onError: (error) => {
                const errorMessage = error instanceof Error ? error.message : String(error);
                toast.error(t('test.failed'), { description: errorMessage });
            },
        });
    };

    const handleRefreshModels = async () => {
        if (!formData.base_urls?.[0]?.url || !effectiveKey) return;
        setFetchedModels([]);
        fetchModel.mutate(
            {
                type: formData.type,
                base_urls: formData.base_urls,
                keys: formData.keys
                    .filter((k) => k.channel_key.trim())
                    .map((k) => ({ enabled: k.enabled, channel_key: k.channel_key.trim() })),
                proxy: formData.proxy,
                channel_proxy: formData.channel_proxy?.trim() || null,
                match_regex: formData.match_regex.trim() || null,
                custom_header: normalizedHeaders,
            },
            {
                onSuccess: (data) => {
                    const normalizedModels = normalizeFetchedModels(data);
                    if (normalizedModels.length > 0) {
                        setFetchedModels(normalizedModels);
                        toast.success(t('modelRefreshSuccess', { count: normalizedModels.length }));
                    } else {
                        setFetchedModels([]);
                        toast.warning(t('modelRefreshEmpty'));
                    }
                },
                onError: (error) => {
                    setFetchedModels([]);
                    const errorMessage = error instanceof Error ? error.message : String(error);
                    toast.error(t('modelRefreshFailed'), { description: errorMessage });
                },
            }
        );
    };

    const handleAddModel = (model: string) => {
        const trimmedModel = model.trim();
        if (trimmedModel && !customModels.includes(trimmedModel) && !autoModels.includes(trimmedModel)) {
            updateModels(autoModels, [...customModels, trimmedModel]);
        }
        setInputValue('');
    };

    const handleRemoveAutoModel = (model: string) => {
        updateModels(autoModels.filter(m => m !== model), customModels);
    };

    const handleRemoveCustomModel = (model: string) => {
        updateModels(autoModels, customModels.filter(m => m !== model));
    };

    const handleInputKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            if (inputValue.trim()) handleAddModel(inputValue);
        }
    };

    const handleAddKey = () => {
        onFormDataChange({
            ...formData,
            keys: [...formData.keys, { enabled: true, channel_key: '' }],
        });
    };

    const handleUpdateKey = (idx: number, patch: Partial<ChannelKeyFormItem>) => {
        const next = formData.keys.map((k, i) => (i === idx ? { ...k, ...patch } : k));
        onFormDataChange({ ...formData, keys: next });
    };

    const handleRemoveKey = (idx: number) => {
        const curr = formData.keys ?? [];
        if (curr.length <= 1) return;
        const next = curr.filter((_, i) => i !== idx);
        onFormDataChange({ ...formData, keys: next });
    };

    const handleAddBaseUrl = () => {
        onFormDataChange({
            ...formData,
            base_urls: [...(formData.base_urls ?? []), { url: '', delay: 0, suffix_mode: 'auto' }],
        });
    };

    const handleUpdateBaseUrl = (idx: number, patch: Partial<Channel['base_urls'][number]>) => {
        const next = (formData.base_urls ?? []).map((u, i) => (i === idx ? { ...u, ...patch } : u));
        onFormDataChange({ ...formData, base_urls: next });
    };

    const handleRemoveBaseUrl = (idx: number) => {
        const curr = formData.base_urls ?? [];
        if (curr.length <= 1) return;
        onFormDataChange({ ...formData, base_urls: curr.filter((_, i) => i !== idx) });
    };

    const handleAddHeader = () => {
        onFormDataChange({
            ...formData,
            custom_header: [...(formData.custom_header ?? []), { header_key: '', header_value: '' }],
        });
    };

    const handleUpdateHeader = (idx: number, patch: Partial<Channel['custom_header'][number]>) => {
        const next = (formData.custom_header ?? []).map((h, i) => (i === idx ? { ...h, ...patch } : h));
        onFormDataChange({ ...formData, custom_header: next });
    };

    const handleRemoveHeader = (idx: number) => {
        const curr = formData.custom_header ?? [];
        if (curr.length <= 1) return;
        onFormDataChange({ ...formData, custom_header: curr.filter((_, i) => i !== idx) });
    };

    const handleApplyTemplate = (templateKey: string) => {
        const template = channelTemplates.find((item) => item.key === templateKey);
        if (!template) return;
        onFormDataChange(template.apply(formData));
        setTestSummary(null);
    };

    return (
        <form onSubmit={onSubmit} className="flex h-full min-h-0 flex-col">
            <div className="flex-1 min-h-0 space-y-4 overflow-y-auto pb-2">
            {showTemplatePicker ? (
                <section className={sectionClassName}>
                    <SectionHeader icon={Sparkles} title={t('template.label')} hint={t('template.hint')} />
                    <TemplatePickerGrid
                        compact={isCompactViewport}
                        onApplyTemplate={handleApplyTemplate}
                    />
                </section>
            ) : (
                <div className="flex justify-end">
                    <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={onShowTemplatePicker}
                        className="h-8 rounded-lg text-xs"
                    >
                        <Sparkles className="size-3.5" />
                        {t('template.open')}
                    </Button>
                </div>
            )}

            <section className={sectionClassName}>
                <SectionHeader icon={Orbit} title={t('name')} hint={t('type')} />
                <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                    <div className={fieldGroupClassName}>
                        <label htmlFor={`${idPrefix}-name`} className={labelClassName}>
                        {t('name')}
                        </label>
                        <Input
                            className="rounded-lg"
                            id={`${idPrefix}-name`}
                            type="text"
                            value={formData.name}
                            onChange={(event) => onFormDataChange({ ...formData, name: event.target.value })}
                            required
                        />
                    </div>

                    <div className={fieldGroupClassName}>
                        <label htmlFor={`${idPrefix}-type`} className={labelClassName}>
                        {t('type')}
                        </label>
                        <Select
                            value={String(formData.type)}
                            onValueChange={(value) => onFormDataChange({ ...formData, type: Number(value) as ChannelType })}
                        >
                            <SelectTrigger id={`${idPrefix}-type`} className="w-full rounded-lg border border-border px-4 py-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent className="rounded-lg">
                                <SelectItem className="rounded-xl" value={String(ChannelType.OpenAIChat)}>{t('typeOpenAIChat')}</SelectItem>
                                <SelectItem className="rounded-xl" value={String(ChannelType.OpenAIResponse)}>{t('typeOpenAIResponse')}</SelectItem>
                                <SelectItem className="rounded-xl" value={String(ChannelType.Anthropic)}>{t('typeAnthropic')}</SelectItem>
                                <SelectItem className="rounded-xl" value={String(ChannelType.Gemini)}>{t('typeGemini')}</SelectItem>
                                <SelectItem className="rounded-xl" value={String(ChannelType.Volcengine)}>{t('typeVolcengine')}</SelectItem>
                                <SelectItem className="rounded-xl" value={String(ChannelType.OpenAIEmbedding)}>{t('typeOpenAIEmbedding')}</SelectItem>
                                <SelectItem className="rounded-xl" value={String(ChannelType.Mimo)}>{t('typeMimo')}</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                </div>
            </section>

            <section className={sectionClassName}>
                <SectionHeader icon={Cable} title={t('baseUrls')} hint={t('baseUrlHint')} />
                <div className="flex items-center justify-between">
                    <label className={labelClassName}>
                        {t('baseUrls')} {formData.base_urls.length > 0 ? `(${formData.base_urls.length})` : ''}
                    </label>
                    <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={handleAddBaseUrl}
                        className="h-6 px-2 text-xs text-muted-foreground/70 hover:text-muted-foreground hover:bg-transparent"
                    >
                        <Plus className="h-3 w-3 mr-1" />
                        {t('add')}
                    </Button>
                </div>
                <div className="space-y-2">
                    {(formData.base_urls ?? []).map((u, idx) => (
                        <div key={`baseurl-${idx}`} className="flex items-center gap-2 rounded-lg border border-border/25 bg-card p-2">
                            <Input
                                id={`${idPrefix}-base-${idx}`}
                                type="url"
                                value={u.url}
                                onChange={(e) => handleUpdateBaseUrl(idx, { url: e.target.value })}
                                placeholder={t('baseUrlUrl')}
                                required={idx === 0}
                                className="flex-1 rounded-lg"
                            />
                            <Select
                                value={u.suffix_mode || 'auto'}
                                onValueChange={(value) => handleUpdateBaseUrl(idx, { suffix_mode: value as Channel['base_urls'][number]['suffix_mode'] })}
                            >
                                <SelectTrigger className="h-10 w-44 shrink-0 rounded-lg">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent className="rounded-lg">
                                    <SelectItem className="rounded-xl" value="auto">{t('baseUrlSuffixAuto')}</SelectItem>
                                    <SelectItem className="rounded-xl" value="openai_compat">{t('baseUrlSuffixOpenAICompat')}</SelectItem>
                                    <SelectItem className="rounded-xl" value="anthropic">{t('baseUrlSuffixAnthropic')}</SelectItem>
                                    <SelectItem className="rounded-xl" value="gemini">{t('baseUrlSuffixGemini')}</SelectItem>
                                    <SelectItem className="rounded-xl" value="volcengine">{t('baseUrlSuffixVolcengine')}</SelectItem>
                                    <SelectItem className="rounded-xl" value="custom">{t('baseUrlSuffixCustom')}</SelectItem>
                                </SelectContent>
                            </Select>
                            <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                onClick={() => handleRemoveBaseUrl(idx)}
                                disabled={(formData.base_urls ?? []).length <= 1}
                                className="h-8 w-8 p-0 rounded-xl text-muted-foreground hover:text-destructive disabled:opacity-40 hover:bg-transparent"
                                title={t('remove')}
                            >
                                <X className="h-4 w-4" />
                            </Button>
                        </div>
                    ))}
                </div>
            </section>

            <section className={sectionClassName}>
                <SectionHeader icon={KeyRound} title={t('apiKey')} hint={t('remark')} />
                <div className="flex items-center justify-between">
                    <label className={labelClassName}>
                        {t('apiKey')} {formData.keys.length > 0 ? `(${formData.keys.length})` : ''}
                    </label>
                    <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={handleAddKey}
                        className="h-6 px-2 text-xs text-muted-foreground/70 hover:text-muted-foreground hover:bg-transparent"
                    >
                        <Plus className="h-3 w-3 mr-1" />
                        {t('add')}
                    </Button>
                </div>
                <div className="space-y-2">
                    {(formData.keys ?? []).map((k, idx) => (
                        <div key={k.id ?? `new-${idx}`} className="grid gap-2 rounded-lg border border-border/25 bg-card p-2 md:grid-cols-[minmax(0,1fr)_10rem_auto_auto] md:items-center">
                            <Input
                                type="text"
                                value={k.channel_key}
                                onChange={(e) => handleUpdateKey(idx, { channel_key: e.target.value })}
                                placeholder={t('apiKey')}
                                required={idx === 0}
                                className="rounded-lg"
                            />
                            <Input
                                type="text"
                                value={k.remark ?? ''}
                                onChange={(e) => handleUpdateKey(idx, { remark: e.target.value })}
                                placeholder={t('remark')}
                                className="rounded-lg md:w-40"
                            />
                            <label className="flex items-center gap-2 rounded-lg border border-border/20 bg-card px-3 py-2 text-sm text-card-foreground">
                                <Switch
                                    checked={k.enabled}
                                    onCheckedChange={(checked) => handleUpdateKey(idx, { enabled: checked })}
                                />
                                <span>{t('enabled')}</span>
                            </label>
                            <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                onClick={() => handleRemoveKey(idx)}
                                disabled={(formData.keys ?? []).length <= 1}
                                className="h-8 w-8 p-0 rounded-xl text-muted-foreground hover:text-destructive hover:bg-transparent disabled:opacity-40"
                                title={t('remove')}
                            >
                                <X className="h-4 w-4" />
                            </Button>
                        </div>
                    ))}
                </div>
            </section>

            <section className={sectionClassName}>
                <SectionHeader icon={Layers3} title={t('model')} hint={t('modelSelected')} />
                <div className="flex items-center justify-between gap-2">
                    <label className={labelClassName}>{t('model')}</label>
                    <div className="flex items-center gap-2">
                        <MorphingDialog onOpen={handleRefreshModels}>
                            <MorphingDialogTrigger
                                disabled={!formData.base_urls?.[0]?.url || !effectiveKey || fetchModel.isPending}
                                className="inline-flex h-6 items-center justify-center gap-1.5 rounded-md px-2 text-xs font-medium text-muted-foreground/50 transition-colors hover:bg-transparent hover:text-muted-foreground"
                            >
                                <RefreshCw className={`h-3 w-3 ${fetchModel.isPending ? 'animate-spin' : ''}`} />
                                {t('modelRefresh')}
                            </MorphingDialogTrigger>
                            <MorphingDialogContainer>
                                <MorphingDialogContent className="h-[calc(100dvh-2rem)] w-[min(100vw-2rem,54rem)] max-w-full rounded-xl border border-border/35 bg-card p-0 md:h-[min(44rem,calc(100dvh-3rem))]">
                                    <ModelPickerDialogPanel
                                        models={fetchedModels}
                                        selectedModels={autoModels}
                                        isLoading={fetchModel.isPending}
                                        onApply={applyFetchedModelSelection}
                                    />
                                </MorphingDialogContent>
                            </MorphingDialogContainer>
                        </MorphingDialog>
                        <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={handleTestChannel}
                            disabled={testChannel.isPending || !(formData.base_urls?.some((u) => u.url.trim()) && formData.keys?.some((k) => k.channel_key.trim()))}
                            className="h-6 px-2 text-xs text-muted-foreground/50 hover:text-muted-foreground hover:bg-transparent"
                        >
                            {testChannel.isPending ? (
                                <RefreshCw className="h-3 w-3 mr-1 animate-spin" />
                            ) : (
                                <FlaskConical className="h-3 w-3 mr-1" />
                            )}
                            {t('test.button')}
                        </Button>
                    </div>
                </div>
                <input type="hidden" value={formData.model} required />

                    {testSummary && (
                    <div className="space-y-2 rounded-lg border border-border/25 bg-card p-3">
                        <div className="flex items-center justify-between gap-2">
                            <div className="flex items-center gap-2 text-sm font-medium text-card-foreground">
                                {testSummary.passed ? (
                                    <CheckCircle2 className="h-4 w-4 text-green-500" />
                                ) : (
                                    <AlertTriangle className="h-4 w-4 text-orange-500" />
                                )}
                                <span>{testSummary.passed ? t('test.success') : t('test.partialSuccess')}</span>
                            </div>
                            <Badge variant="secondary">{testSummary.results.length} {t('test.results')}</Badge>
                        </div>
                        <div className="space-y-2 max-h-48 overflow-y-auto">
                            {testSummary.results.map((result, idx) => (
                                <div key={`${result.base_url}-${result.key_masked}-${idx}`} className="rounded-lg border border-border/30 bg-card p-2.5 text-xs space-y-1">
<div className="flex flex-wrap items-center justify-between gap-2">
                                        <span className="font-mono truncate">{result.base_url}</span>
                                        <div className="flex items-center gap-1 shrink-0">
                                            <Badge variant="secondary">{result.key_masked || '-'}</Badge>
                                            <Badge variant="secondary">{result.status_code}</Badge>
                                        </div>
                                    </div>
                                    <div className="flex items-center justify-between gap-2 text-muted-foreground">
                                        <span>{result.key_remark || t('test.noRemark')}</span>
                                        <span>{result.latency_ms}ms · {result.passed ? t('test.pass') : t('test.fail')}</span>
                                    </div>
                                    {result.message && <p className="break-all text-muted-foreground">{result.message}</p>}
                                </div>
                            ))}
                        </div>
                        <p className="text-xs text-muted-foreground">{t('test.hint')}</p>
                    </div>
                )}

                <div className="relative">
                    <Input
                        ref={inputRef}
                        id={`${idPrefix}-model-custom`}
                        type="text"
                        value={inputValue}
                        onChange={(e) => setInputValue(e.target.value)}
                        onKeyDown={handleInputKeyDown}
                        placeholder={t('modelCustomPlaceholder')}
                        className="rounded-lg pr-10"
                    />
                    {inputValue.trim() && !customModels.includes(inputValue.trim()) && !autoModels.includes(inputValue.trim()) && (
                        <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={() => handleAddModel(inputValue)}
                            className="absolute rounded-lg right-1 top-1/2 -translate-y-1/2 h-7 w-7 p-0 text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
                            title={t('modelAdd')}
                        >
                            <Plus className="size-4" />
                        </Button>
                    )}
                </div>

                <div className="space-y-2">
                    <div className="flex items-center justify-between">
                        <label className="text-xs font-medium text-card-foreground">
                            {t('modelSelected')} {(autoModels.length + customModels.length) > 0 && `(${autoModels.length + customModels.length})`}
                        </label>
                        {(autoModels.length + customModels.length) > 0 && (
                            <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                onClick={() => {
                                    updateModels([], []);
                                }}
                                className="h-6 px-2 text-xs text-muted-foreground/50 hover:text-muted-foreground hover:bg-transparent"
                            >
                                {t('modelClearAll')}
                            </Button>
                        )}
                    </div>
                    <div className="max-h-40 min-h-12 overflow-y-auto rounded-lg border border-border/25 bg-card p-2.5 shadow-sm">
                        {(autoModels.length + customModels.length) > 0 ? (
                            <div className="flex flex-wrap gap-1.5">
                                {autoModels.map((model) => (
                                    <Badge key={model} variant="secondary" className="bg-muted hover:bg-muted/80">
                                        {model}
                                        <button
                                            type="button"
                                            onClick={() => handleRemoveAutoModel(model)}
                                            className="ml-1 rounded-sm opacity-70 hover:opacity-100 focus:outline-none focus:ring-1 focus:ring-ring"
                                        >
                                            <X className="h-3 w-3" />
                                        </button>
                                    </Badge>
                                ))}
                                {customModels.map((model) => (
                                    <Badge key={model} className="bg-primary hover:bg-primary/90">
                                        {model}
                                        <button
                                            type="button"
                                            onClick={() => handleRemoveCustomModel(model)}
                                            className="ml-1 rounded-sm opacity-70 hover:opacity-100 focus:outline-none focus:ring-1 focus:ring-ring"
                                        >
                                            <X className="h-3 w-3" />
                                        </button>
                                    </Badge>
                                ))}
                            </div>
                        ) : (
                            <div className="flex items-center justify-center h-8 text-xs text-muted-foreground">
                                {t('modelNoSelected')}
                            </div>
                        )}
                    </div>
                </div>
            </section>

            <Accordion type="single" collapsible className="w-full">
                <AccordionItem value="advanced" className="border-none">
                    <AccordionTrigger className="rounded-lg bg-card/70 px-4 py-4 text-sm font-medium text-card-foreground transition-colors hover:bg-card hover:no-underline">
                        <span className="flex items-center gap-2">
                            <span className="h-2 w-2 rounded-full bg-primary/70" />
                            {t('advanced')}
                        </span>
                    </AccordionTrigger>
                    <AccordionContent className="pt-4">
                        <div className="space-y-4">
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div className={fieldGroupClassName}>
                                <label htmlFor={`${idPrefix}-auto-group`} className={labelClassName}>
                                    {t('autoGroup')}
                                </label>
                                <Select
                                    value={String(formData.auto_group)}
                                    onValueChange={(value) => onFormDataChange({ ...formData, auto_group: Number(value) as AutoGroupType })}
                                >
                                    <SelectTrigger id={`${idPrefix}-auto-group`} className="w-full rounded-lg border border-border px-4 py-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent className="rounded-lg">
                                        <SelectItem className="rounded-xl" value={String(AutoGroupType.None)}>{t('autoGroupNone')}</SelectItem>
                                        <SelectItem className="rounded-xl" value={String(AutoGroupType.Fuzzy)}>{t('autoGroupFuzzy')}</SelectItem>
                                        <SelectItem className="rounded-xl" value={String(AutoGroupType.Exact)}>{t('autoGroupExact')}</SelectItem>
                                        <SelectItem className="rounded-xl" value={String(AutoGroupType.Regex)}>{t('autoGroupRegex')}</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>

                            <div className={fieldGroupClassName}>
                                <label htmlFor={`${idPrefix}-channel-proxy`} className={labelClassName}>
                                    {t('channelProxy')}
                                </label>
                                <Input
                                    id={`${idPrefix}-channel-proxy`}
                                    type="text"
                                    value={formData.channel_proxy}
                                    onChange={(e) => onFormDataChange({ ...formData, channel_proxy: e.target.value })}
                                    placeholder={t('channelProxyPlaceholder')}
                                    className="rounded-lg"
                                />
                            </div>
                        </div>

                        <div className={fieldGroupClassName}>
                            <div className="flex items-center justify-between">
                                <label className={labelClassName}>
                                    {t('customHeader')} {formData.custom_header.length > 0 ? `(${formData.custom_header.length})` : ''}
                                </label>
                                <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    onClick={handleAddHeader}
                                    className="h-6 px-2 text-xs text-muted-foreground/70 hover:text-muted-foreground hover:bg-transparent"
                                >
                                    <Plus className="h-3 w-3 mr-1" />
                                    {t('customHeaderAdd')}
                                </Button>
                            </div>
                            <div className="space-y-2">
                                {(formData.custom_header ?? []).map((h, idx) => (
                                    <div key={`hdr-${idx}`} className="grid gap-2 rounded-lg border border-border/25 bg-card p-2 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] md:items-center">
                                        <Input
                                            type="text"
                                            value={h.header_key}
                                            onChange={(e) => handleUpdateHeader(idx, { header_key: e.target.value })}
                                            placeholder={t('customHeaderKey')}
                                            className="rounded-lg"
                                        />
                                        <Input
                                            type="text"
                                            value={h.header_value}
                                            onChange={(e) => handleUpdateHeader(idx, { header_value: e.target.value })}
                                            placeholder={t('customHeaderValue')}
                                            className="rounded-lg"
                                        />
                                        <Button
                                            type="button"
                                            variant="ghost"
                                            size="sm"
                                            onClick={() => handleRemoveHeader(idx)}
                                            disabled={(formData.custom_header ?? []).length <= 1}
                                            className="h-8 w-8 p-0 rounded-xl text-muted-foreground hover:text-destructive hover:bg-transparent disabled:opacity-40"
                                            title={t('remove')}
                                        >
                                            <X className="h-4 w-4" />
                                        </Button>
                                    </div>
                                ))}
                            </div>
                        </div>

                        <div className={fieldGroupClassName}>
                            <label htmlFor={`${idPrefix}-match-regex`} className={labelClassName}>
                                {t('matchRegex')}
                            </label>
                            <Input
                                id={`${idPrefix}-match-regex`}
                                type="text"
                                value={formData.match_regex}
                                onChange={(e) => onFormDataChange({ ...formData, match_regex: e.target.value })}
                                placeholder={t('matchRegexPlaceholder')}
                                className="rounded-lg"
                            />
                        </div>

                        <div className={fieldGroupClassName}>
                            <label htmlFor={`${idPrefix}-param-override`} className={labelClassName}>
                                {t('paramOverride')}
                            </label>
                            <textarea
                                id={`${idPrefix}-param-override`}
                                value={formData.param_override}
                                onChange={(e) => onFormDataChange({ ...formData, param_override: e.target.value })}
                                placeholder={t('paramOverridePlaceholder')}
                                className="min-h-28 w-full rounded-lg border border-border/35 bg-card px-3 py-2 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                            />
                        </div>

                        <div className="space-y-4 pt-2 border-t border-border/20">
                            <div className="flex flex-wrap items-center justify-between gap-3">
                                <div>
                                    <p className="text-sm font-medium text-card-foreground">{t('requestRewrite')}</p>
                                    <p className="text-xs text-muted-foreground">{t('requestRewriteHint')}</p>
                                </div>
                                <label className="flex items-center gap-2 cursor-pointer">
                                    <Switch
                                        checked={requestRewriteSupported && formData.request_rewrite.enabled}
                                        onCheckedChange={(checked) => onFormDataChange({
                                            ...formData,
                                            request_rewrite: {
                                                ...formData.request_rewrite,
                                                enabled: checked,
                                            },
                                        })}
                                        disabled={!requestRewriteSupported}
                                    />
                                    <span className="text-sm text-card-foreground">{t('requestRewriteEnabled')}</span>
                                </label>
                            </div>

                            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                                <div className={fieldGroupClassName}>
                                    <label htmlFor={`${idPrefix}-request-rewrite-profile`} className={labelClassName}>
                                        {t('requestRewriteProfile')}
                                    </label>
                                    <Select
                                        value={formData.request_rewrite.profile ?? RequestRewriteProfile.OpenAIChatCompat}
                                        onValueChange={(value) => onFormDataChange({
                                            ...formData,
                                            request_rewrite: {
                                                ...formData.request_rewrite,
                                                profile: value as RequestRewriteProfile,
                                            },
                                        })}
                                        disabled={!requestRewriteSupported || !formData.request_rewrite.enabled}
                                    >
                                        <SelectTrigger id={`${idPrefix}-request-rewrite-profile`} className="w-full rounded-lg border border-border px-4 py-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent className="rounded-lg">
                                            <SelectItem className="rounded-xl" value={RequestRewriteProfile.OpenAIChatCompat}>{t('requestRewriteProfileOpenAIChatCompat')}</SelectItem>
                                        </SelectContent>
                                    </Select>
                                </div>

                                <div className={fieldGroupClassName}>
                                    <label htmlFor={`${idPrefix}-request-rewrite-tool-role`} className={labelClassName}>
                                        {t('requestRewriteToolRoleStrategy')}
                                    </label>
                                    <Select
                                        value={formData.request_rewrite.tool_role_strategy ?? ToolRoleStrategy.Keep}
                                        onValueChange={(value) => onFormDataChange({
                                            ...formData,
                                            request_rewrite: {
                                                ...formData.request_rewrite,
                                                tool_role_strategy: value as ToolRoleStrategy,
                                            },
                                        })}
                                        disabled={!requestRewriteSupported || !formData.request_rewrite.enabled}
                                    >
                                        <SelectTrigger id={`${idPrefix}-request-rewrite-tool-role`} className="w-full rounded-lg border border-border px-4 py-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent className="rounded-lg">
                                            <SelectItem className="rounded-xl" value={ToolRoleStrategy.Keep}>{t('requestRewriteStrategyKeep')}</SelectItem>
                                            <SelectItem className="rounded-xl" value={ToolRoleStrategy.StringifyToUser}>{t('requestRewriteStrategyStringifyToUser')}</SelectItem>
                                        </SelectContent>
                                    </Select>
                                </div>

                                <div className={fieldGroupClassName}>
                                    <label htmlFor={`${idPrefix}-request-rewrite-system`} className={labelClassName}>
                                        {t('requestRewriteSystemMessageStrategy')}
                                    </label>
                                    <Select
                                        value={formData.request_rewrite.system_message_strategy ?? SystemMessageStrategy.Keep}
                                        onValueChange={(value) => onFormDataChange({
                                            ...formData,
                                            request_rewrite: {
                                                ...formData.request_rewrite,
                                                system_message_strategy: value as SystemMessageStrategy,
                                            },
                                        })}
                                        disabled={!requestRewriteSupported || !formData.request_rewrite.enabled}
                                    >
                                        <SelectTrigger id={`${idPrefix}-request-rewrite-system`} className="w-full rounded-lg border border-border px-4 py-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent className="rounded-lg">
                                            <SelectItem className="rounded-xl" value={SystemMessageStrategy.Keep}>{t('requestRewriteStrategyKeep')}</SelectItem>
                                            <SelectItem className="rounded-xl" value={SystemMessageStrategy.Merge}>{t('requestRewriteStrategyMerge')}</SelectItem>
                                        </SelectContent>
                                    </Select>
                                </div>
                            </div>
                        </div>
                        </div>
                    </AccordionContent>
                </AccordionItem>
            </Accordion>
            </div>

            <section className={`${sectionClassName} mt-4 flex shrink-0 flex-col gap-4 border-t border-border/20 pt-4 md:flex-row md:items-center md:justify-between`}>
                <label className="flex items-center gap-2 cursor-pointer">
                    <Switch
                        checked={formData.enabled}
                        onCheckedChange={(checked) => onFormDataChange({ ...formData, enabled: checked })}
                    />
                    <span className="text-sm font-medium text-card-foreground">{t('enabled')}</span>
                </label>
                <div className="flex items-center gap-6">
                    <label className="flex items-center gap-2 cursor-pointer">
                        <Switch
                            checked={formData.proxy}
                            onCheckedChange={(checked) => onFormDataChange({ ...formData, proxy: checked })}
                        />
                        <span className="text-sm text-card-foreground">{t('proxy')}</span>
                    </label>
                    <label className="flex items-center gap-2 cursor-pointer">
                        <Switch
                            checked={formData.auto_sync}
                            onCheckedChange={(checked) => onFormDataChange({ ...formData, auto_sync: checked })}
                        />
                        <span className="text-sm text-card-foreground">{t('autoSync')}</span>
                    </label>
                </div>
            </section>

            <div className={`shrink-0 flex flex-col gap-3 pt-4 ${onCancel ? 'sm:flex-row' : ''}`}>
                {onCancel && cancelText && (
                    <Button
                        type="button"
                        variant="secondary"
                        onClick={onCancel}
                        className="h-12 w-full rounded-lg sm:flex-1"
                    >
                        {cancelText}
                    </Button>
                )}
                <Button
                    type="submit"
                    disabled={isPending}
                    className="h-12 w-full rounded-lg sm:flex-1"
                >
                    {isPending ? pendingText : submitText}
                </Button>
            </div>
        </form>
    );
}
