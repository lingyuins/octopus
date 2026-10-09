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
    useCreateChannel,
} from '@/api/endpoints/channel';
import { ArrowLeft, Sparkles, X } from 'lucide-react';
import { useTranslations } from 'next-intl';
import {
    ChannelForm,
    TemplatePickerGrid,
    createDefaultRequestRewriteFormData,
    getEffectiveRequestRewriteFormData,
    type ChannelFormData,
} from './Form';
import { channelTemplates } from './templates';
import { DEFAULT_CHANNEL_TYPE } from './type-options';
import { toast } from '@/components/common/Toast';

export function CreateDialogContent() {
    const { setIsOpen } = useMorphingDialog();
    const createChannel = useCreateChannel();
    const [showPresetPicker, setShowPresetPicker] = useState(false);
    const [formData, setFormData] = useState<ChannelFormData>({
        name: '',
        group_id: 0,
        type: DEFAULT_CHANNEL_TYPE,
        base_urls: [{ url: '', delay: 0, suffix_mode: 'auto' }],
        custom_header: [],
        channel_proxy: '',
        param_override: '',
        request_rewrite: createDefaultRequestRewriteFormData(),
        keys: [{ enabled: true, channel_key: '', priority: 0, remark: '' }],
        model: '',
        custom_model: '',
        auto_sync: false,
        auto_sync_key_models: false,
        auto_group: AutoGroupType.None,
        skip_model_test: false,
        disposable: false,
        expire_at: '',
        notif_channel_id: null,
        key_selection_strategy: '',
        enabled: true,
        proxy_mode: 'direct',
        proxy_config_id: null,
        match_regex: '',
        pool_id: 0,
    });
    const t = useTranslations('channel.create');
    const tForm = useTranslations('channel.form');
    const tProxy = useTranslations('proxyPool');

    const handleApplyTemplate = (templateKey: string) => {
        const template = channelTemplates.find((item) => item.key === templateKey);
        if (!template) return;
        setFormData((current) => template.apply(current));
        setShowPresetPicker(false);
    };

    const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        // pool 模式必须选择代理配置（或填写自定义渠道代理地址），与后端校验一致
        if (formData.proxy_mode === 'pool' && !formData.proxy_config_id && !formData.channel_proxy.trim()) {
            toast.error(tProxy('selectRequired'));
            return;
        }
        const normalizedBaseUrls = (formData.base_urls ?? []).filter((u) => u.url.trim()).map((u) => ({
            url: u.url.trim(),
            delay: Number(u.delay || 0),
            suffix_mode: u.suffix_mode && u.suffix_mode !== 'auto' ? u.suffix_mode : undefined,
        }));
        const normalizedKeys = formData.keys.map((k) => ({
            enabled: k.enabled,
            channel_key: k.channel_key.trim(),
            priority: Number(k.priority ?? 0),
            remark: k.remark ?? '',
            supported_models: k.supported_models ?? '',
        }));
        const normalizedHeaders = (formData.custom_header ?? [])
            .map((h) => ({ header_key: h.header_key.trim(), header_value: h.header_value }))
            .filter((h) => h.header_key && h.header_value !== '');

        const channelProxy = formData.channel_proxy.trim();
        const paramOverride = formData.param_override.trim();
        const requestRewrite = getEffectiveRequestRewriteFormData(formData.type, formData.request_rewrite);
        createChannel.mutate(
            {
                name: formData.name,
                group_id: formData.group_id || undefined,
                type: formData.type,
                enabled: formData.enabled,
                base_urls: normalizedBaseUrls,
                keys: normalizedKeys,
                model: formData.model,
                custom_model: formData.custom_model,
                proxy_mode: formData.proxy_mode,
                proxy_config_id: formData.proxy_mode === 'pool' ? formData.proxy_config_id : null,
                proxy: formData.proxy_mode !== 'direct',
                auto_sync: formData.auto_sync,
                auto_sync_key_models: formData.auto_sync_key_models,
                auto_group: formData.auto_group,
                key_selection_strategy: formData.key_selection_strategy,
                skip_model_test: formData.skip_model_test,
                disposable: formData.disposable,
                // datetime-local 返回无时区的 "YYYY-MM-DDTHH:mm"，浏览器按本地时区解释。
                // 转 ISO 字符串（带 Z 时区）发给后端，避免 Go 按解析无时区字符串为 UTC 导致时区偏移。
                expire_at: formData.disposable && formData.expire_at ? new Date(formData.expire_at).toISOString() : undefined,
                notif_channel_id: formData.disposable && formData.notif_channel_id != null ? formData.notif_channel_id : undefined,
                custom_header: normalizedHeaders,
                channel_proxy: channelProxy,
                param_override: paramOverride,
                request_rewrite: requestRewrite.enabled ? requestRewrite : undefined,
                match_regex: formData.match_regex.trim(),
                pool_id: formData.pool_id || undefined,
            },
            {
                onSuccess: () => {
                    setIsOpen(false);
                }
            });
    };

    return (
        <div className="flex h-full w-full min-h-0 flex-col overflow-hidden bg-card text-card-foreground">
            <MorphingDialogTitle className="shrink-0">
                <header className="flex min-h-16 items-center justify-between gap-3 border-b border-border px-4 py-3 sm:px-6">
                    <h2 className="min-w-0 text-lg font-semibold">{t('dialogTitle')}</h2>
                    <div className="flex shrink-0 items-center gap-2">
                    <Button
                        type="button"
                        variant="outline"
                        onClick={() => setShowPresetPicker(!showPresetPicker)}
                        className="h-10 rounded-lg px-3 text-xs sm:text-sm"
                    >
                        {showPresetPicker ? <ArrowLeft className="size-4" /> : <Sparkles className="size-4" />}
                        <span>{showPresetPicker ? tForm('template.skip') : tForm('template.open')}</span>
                    </Button>
                    {createChannel.isPending ? (
                        <Button
                            type="button"
                            variant="outline"
                            size="icon"
                            disabled
                            aria-label={tForm('modelPicker.cancel')}
                            className="h-9 w-9 rounded-md border-border bg-card opacity-80 transition-all duration-150 hover:bg-muted hover:opacity-100"
                        >
                            <X className="size-5" />
                        </Button>
                    ) : (
                        <MorphingDialogClose
                            className="relative inset-auto size-10 shrink-0 p-2 sm:inset-auto sm:size-10 sm:p-2"
                            variants={{
                                initial: { opacity: 0, scale: 0.8 },
                                animate: { opacity: 1, scale: 1 },
                                exit: { opacity: 0, scale: 0.8 }
                            }}
                        />
                    )}
                    </div>
                </header>
            </MorphingDialogTitle>
            <MorphingDialogDescription disableLayoutAnimation className="flex min-h-0 flex-1 flex-col overflow-hidden">
                <AnimatePresence mode="wait" initial={false}>
                {showPresetPicker ? (
                    <motion.div
                        key="preset-picker"
                        initial={{ opacity: 0, scale: 0.98, y: 6 }}
                        animate={{ opacity: 1, scale: 1, y: 0 }}
                        exit={{ opacity: 0, scale: 0.98, y: -4 }}
                        transition={{ duration: 0.16, ease: 'easeOut' }}
                        className="flex h-full min-h-0 flex-col gap-4 overflow-y-auto overscroll-contain px-4 py-4 sm:px-6"
                    >
                        <div>
                            <div className="mb-4 space-y-2">
                                <div className="flex items-center justify-between gap-3">
                                    <div className="inline-flex min-w-0 items-center gap-2 rounded-full border border-primary/12 bg-card px-3 py-1 text-[0.68rem] font-semibold text-primary">
                                        <Sparkles className="size-3.5" />
                                        {tForm('template.label')}
                                    </div>
                                    <Button
                                        type="button"
                                        variant="ghost"
                                        size="sm"
                                        onClick={() => setShowPresetPicker(false)}
                                        className="h-8 shrink-0 rounded-lg text-xs text-muted-foreground transition-[color,background-color,border-color,box-shadow,opacity,transform] duration-100 ease-out active:scale-[0.98]"
                                    >
                                        {tForm('template.skip')}
                                    </Button>
                                </div>
                                <p className="text-xs leading-5 text-muted-foreground">{tForm('template.pickerHint')}</p>
                            </div>
                            <TemplatePickerGrid onApplyTemplate={handleApplyTemplate} />
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
                            layout="create"
                            onCancel={() => setIsOpen(false)}
                            cancelText={tForm('modelPicker.cancel')}
                        />
                    </motion.div>
                )}
                </AnimatePresence>
            </MorphingDialogDescription>
        </div>
    );
}
