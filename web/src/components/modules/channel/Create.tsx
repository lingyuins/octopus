import { useState } from 'react';
import { AnimatePresence, motion } from 'motion/react';
import {
    MorphingDialogClose,
    MorphingDialogTitle,
    MorphingDialogDescription,
    useMorphingDialog,
} from '@/components/ui/morphing-dialog';
import { Button } from '@/components/ui/button';
import {
    AutoGroupType,
    ChannelType,
    useCreateChannel,
} from '@/api/endpoints/channel';
import { Sparkles, X } from 'lucide-react';
import { useIsMobile } from '@/hooks/use-mobile';
import { useTranslations } from 'next-intl';
import {
    ChannelForm,
    TemplatePickerGrid,
    createDefaultRequestRewriteFormData,
    getEffectiveRequestRewriteFormData,
    type ChannelFormData,
} from './Form';
import { channelTemplates } from './templates';

export function CreateDialogContent() {
    const { setIsOpen } = useMorphingDialog();
    const createChannel = useCreateChannel();
    const isCompactViewport = useIsMobile();
    const [showPresetPicker, setShowPresetPicker] = useState(true);
    const [formData, setFormData] = useState<ChannelFormData>({
        name: '',
        type: ChannelType.OpenAIChat,
        base_urls: [{ url: '', delay: 0, suffix_mode: 'auto' }],
        custom_header: [],
        channel_proxy: '',
        param_override: '',
        request_rewrite: createDefaultRequestRewriteFormData(),
        keys: [{ enabled: true, channel_key: '', remark: '' }],
        model: '',
        custom_model: '',
        auto_sync: false,
        auto_group: AutoGroupType.None,
        enabled: true,
        proxy: false,
        match_regex: '',
    });
    const t = useTranslations('channel.create');
    const tForm = useTranslations('channel.form');

    const resetFormData = () => {
        setFormData({
            name: '',
            type: ChannelType.OpenAIChat,
            base_urls: [{ url: '', delay: 0, suffix_mode: 'auto' }],
            custom_header: [],
            channel_proxy: '',
            param_override: '',
            request_rewrite: createDefaultRequestRewriteFormData(),
            keys: [{ enabled: true, channel_key: '', remark: '' }],
            model: '',
            custom_model: '',
            auto_sync: false,
            auto_group: AutoGroupType.None,
            enabled: true,
            proxy: false,
            match_regex: '',
        });
        setShowPresetPicker(true);
    };

    const handleApplyTemplate = (templateKey: string) => {
        const template = channelTemplates.find((item) => item.key === templateKey);
        if (!template) return;
        setFormData((current) => template.apply(current));
        setShowPresetPicker(false);
    };

    const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        const normalizedBaseUrls = (formData.base_urls ?? []).filter((u) => u.url.trim()).map((u) => ({
            url: u.url.trim(),
            delay: Number(u.delay || 0),
            suffix_mode: u.suffix_mode && u.suffix_mode !== 'auto' ? u.suffix_mode : undefined,
        }));
        const normalizedKeys = formData.keys
            .filter((k) => k.channel_key.trim())
            .map((k) => ({ enabled: k.enabled, channel_key: k.channel_key, remark: k.remark ?? '' }));
        const normalizedHeaders = (formData.custom_header ?? [])
            .map((h) => ({ header_key: h.header_key.trim(), header_value: h.header_value }))
            .filter((h) => h.header_key && h.header_value !== '');

        const channelProxy = formData.channel_proxy.trim();
        const paramOverride = formData.param_override.trim();
        const requestRewrite = getEffectiveRequestRewriteFormData(formData.type, formData.request_rewrite);
        createChannel.mutate(
            {
                name: formData.name,
                type: formData.type,
                enabled: formData.enabled,
                base_urls: normalizedBaseUrls,
                keys: normalizedKeys,
                model: formData.model,
                custom_model: formData.custom_model,
                proxy: formData.proxy,
                auto_sync: formData.auto_sync,
                auto_group: formData.auto_group,
                custom_header: normalizedHeaders,
                channel_proxy: channelProxy,
                param_override: paramOverride,
                request_rewrite: requestRewrite.enabled ? requestRewrite : undefined,
                match_regex: formData.match_regex.trim(),
            },
            {
                onSuccess: () => {
                    resetFormData();
                    setIsOpen(false);
                }
            });
    };

    return (
        <div className="relative flex h-full w-full min-h-0 flex-col overflow-hidden rounded-xl border border-border/35 bg-card text-card-foreground shadow-md">
            <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_18%_14%,color-mix(in_oklch,var(--primary)_24%,transparent)_0%,transparent_32%),radial-gradient(circle_at_82%_16%,color-mix(in_oklch,var(--primary)_14%,transparent)_0%,transparent_24%),linear-gradient(180deg,color-mix(in_oklch,white_20%,transparent),transparent_26%,color-mix(in_oklch,var(--primary)_10%,transparent))]" />
            <MorphingDialogTitle className="shrink-0">
                <header className="relative flex items-center justify-between border-b border-border/20 px-5 py-4 md:px-6 md:py-5">
                    <div className="space-y-2">
                        <div className="flex items-center gap-2">
                            <span className="h-2.5 w-10 rounded-full bg-primary/18 shadow-sm" />
                            <span className="h-2.5 w-20 rounded-full bg-card shadow-inner" />
                        </div>
                        <h2 className="text-xl font-semibold tracking-tight text-card-foreground md:text-2xl">{t('dialogTitle')}</h2>
                    </div>
                    {showPresetPicker ? (
                        <Button
                            type="button"
                            variant="outline"
                            size="icon"
                            onClick={() => setShowPresetPicker(false)}
                            aria-label={tForm('template.skip')}
                            className="h-9 w-9 rounded-md border-border bg-card opacity-80 transition-all duration-150 hover:bg-muted hover:opacity-100"
                        >
                            <X className="size-5" />
                        </Button>
                    ) : (
                        <MorphingDialogClose
                            className="relative right-0 top-0"
                            variants={{
                                initial: { opacity: 0, scale: 0.8 },
                                animate: { opacity: 1, scale: 1 },
                                exit: { opacity: 0, scale: 0.8 }
                            }}
                        />
                    )}
                </header>
            </MorphingDialogTitle>
            <MorphingDialogDescription disableLayoutAnimation className="relative flex-1 min-h-0 overflow-hidden px-4 py-4 md:px-6 md:py-5">
                <AnimatePresence mode="wait" initial={false}>
                {showPresetPicker ? (
                    <motion.div
                        key="preset-picker"
                        initial={{ opacity: 0, scale: 0.98, y: 6 }}
                        animate={{ opacity: 1, scale: 1, y: 0 }}
                        exit={{ opacity: 0, scale: 0.98, y: -4 }}
                        transition={{ duration: 0.16, ease: 'easeOut' }}
                        className="flex h-full min-h-0 flex-col gap-4 overflow-y-auto"
                    >
                        <div className="rounded-lg bg-card/70 p-4 md:p-5">
                            <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
                                <div className="space-y-2">
                                    <div className="inline-flex items-center gap-2 rounded-full border border-primary/12 bg-card px-3 py-1 text-[0.68rem] font-semibold text-primary">
                                        <Sparkles className="size-3.5" />
                                        {tForm('template.label')}
                                    </div>
                                    <p className="text-xs leading-5 text-muted-foreground">{tForm('template.pickerHint')}</p>
                                </div>
                                <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    onClick={() => setShowPresetPicker(false)}
                                    className="h-8 rounded-lg text-xs text-muted-foreground transition-[color,background-color,border-color,box-shadow,opacity,transform] duration-100 ease-out active:scale-[0.98]"
                                >
                                    {tForm('template.skip')}
                                </Button>
                            </div>
                            <TemplatePickerGrid compact={isCompactViewport} onApplyTemplate={handleApplyTemplate} />
                        </div>
                    </motion.div>
                ) : (
                    <motion.div
                        key="manual-form"
                        initial={{ opacity: 0, scale: 0.98, y: 6 }}
                        animate={{ opacity: 1, scale: 1, y: 0 }}
                        exit={{ opacity: 0, scale: 0.98, y: -4 }}
                        transition={{ duration: 0.16, ease: 'easeOut' }}
                        className="h-full min-h-0"
                    >
                        <ChannelForm
                            formData={formData}
                            onFormDataChange={setFormData}
                            onSubmit={handleSubmit}
                            isPending={createChannel.isPending}
                            submitText={t('submit')}
                            pendingText={t('submitting')}
                            idPrefix="new-channel"
                            showTemplatePicker={false}
                            onShowTemplatePicker={() => setShowPresetPicker(true)}
                        />
                    </motion.div>
                )}
                </AnimatePresence>
            </MorphingDialogDescription>
        </div>
    );
}
