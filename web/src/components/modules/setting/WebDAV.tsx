'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Cloud, Download, Loader2, RefreshCw, Trash2, Upload } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { toast } from '@/components/common/Toast';
import {
    useWebDAVConfig,
    useUpdateWebDAVConfig,
    useTestWebDAVConnection,
    useTriggerWebDAVBackup,
    useWebDAVFiles,
    useRestoreWebDAVBackup,
    useDeleteWebDAVBackup,
    type WebDAVConfig,
} from '@/api/endpoints/webdav';

export function SettingWebDAV() {
    const t = useTranslations('setting');
    const { data: config, isLoading } = useWebDAVConfig();
    const updateConfig = useUpdateWebDAVConfig();
    const testConnection = useTestWebDAVConnection();
    const triggerBackup = useTriggerWebDAVBackup();
    const { data: files = [] } = useWebDAVFiles();
    const restoreBackup = useRestoreWebDAVBackup();
    const deleteBackup = useDeleteWebDAVBackup();

    const [enabled, setEnabled] = useState(false);
    const [baseURL, setBaseURL] = useState('');
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [remotePath, setRemotePath] = useState('/octopus-backup/');
    const [intervalHours, setIntervalHours] = useState(6);
    const [includeStats, setIncludeStats] = useState(true);
    const [includeLogs, setIncludeLogs] = useState(false);
    const [maxBackups, setMaxBackups] = useState(10);

    useEffect(() => {
        if (!config) return;
        setEnabled(config.enabled);
        setBaseURL(config.base_url);
        setUsername(config.username);
        setPassword(config.password || '');
        setRemotePath(config.remote_path || '/octopus-backup/');
        setIntervalHours(config.interval_hours || 6);
        setIncludeStats(config.include_stats);
        setIncludeLogs(config.include_logs);
        setMaxBackups(config.max_backups || 10);
    }, [config]);

    const onSave = async () => {
        try {
            const payload: WebDAVConfig = {
                enabled,
                base_url: baseURL,
                username,
                password,
                remote_path: remotePath,
                interval_hours: intervalHours,
                include_stats: includeStats,
                include_logs: includeLogs,
                max_backups: maxBackups,
            };
            await updateConfig.mutateAsync(payload);
            toast.success(t('webdav.saved'));
        } catch (e) {
            toast.error(e instanceof Error ? e.message : t('webdav.saveFailed'));
        }
    };

    const onTest = async () => {
        try {
            await testConnection.mutateAsync({
                base_url: baseURL,
                username,
                password,
            });
            toast.success(t('webdav.testSuccess'));
        } catch (e) {
            toast.error(e instanceof Error ? e.message : t('webdav.testFailed'));
        }
    };

    const onBackup = async () => {
        try {
            await triggerBackup.mutateAsync();
            toast.success(t('webdav.backupStarted'));
        } catch (e) {
            toast.error(e instanceof Error ? e.message : t('webdav.backupFailed'));
        }
    };

    const onRestore = async (filename: string) => {
        if (!window.confirm(t('webdav.restoreConfirm', { filename }))) return;
        try {
            await restoreBackup.mutateAsync(filename);
            toast.success(t('webdav.restoreSuccess'));
        } catch (e) {
            toast.error(e instanceof Error ? e.message : t('webdav.restoreFailed'));
        }
    };

    const onDelete = async (filename: string) => {
        if (!window.confirm(t('webdav.deleteConfirm', { filename }))) return;
        try {
            await deleteBackup.mutateAsync(filename);
            toast.success(t('webdav.deleteSuccess'));
        } catch (e) {
            toast.error(e instanceof Error ? e.message : t('webdav.deleteFailed'));
        }
    };

    if (isLoading) {
        return (
            <div className="rounded-xl border-border/35 bg-card p-6 text-card-foreground shadow-md">
                <div className="flex items-center justify-center py-8">
                    <Loader2 className="size-5 animate-spin text-muted-foreground" />
                </div>
            </div>
        );
    }

    return (
        <div className="rounded-xl border-border/35 bg-card p-6 space-y-5 text-card-foreground shadow-md">
            <h2 className="text-lg font-bold text-card-foreground flex items-center gap-2">
                <Cloud className="h-5 w-5" />
                {t('webdav.title')}
            </h2>

            {/* Enable toggle */}
            <div className="flex items-center justify-between gap-4">
                <div>
                    <div className="text-sm font-medium text-card-foreground">{t('webdav.enabled')}</div>
                    <div className="text-xs text-muted-foreground">{t('webdav.enabledHint')}</div>
                </div>
                <Switch checked={enabled} onCheckedChange={setEnabled} />
            </div>

            {/* Connection fields */}
            <div className="space-y-3">
                <div>
                    <label className="text-xs font-medium text-muted-foreground">{t('webdav.baseUrl')}</label>
                    <Input
                        value={baseURL}
                        onChange={(e) => setBaseURL(e.target.value)}
                        placeholder="https://dav.example.com"
                        className="rounded-xl mt-1"
                    />
                </div>
                <div className="grid grid-cols-2 gap-3">
                    <div>
                        <label className="text-xs font-medium text-muted-foreground">{t('webdav.username')}</label>
                        <Input
                            value={username}
                            onChange={(e) => setUsername(e.target.value)}
                            placeholder={t('webdav.username')}
                            className="rounded-xl mt-1"
                        />
                    </div>
                    <div>
                        <label className="text-xs font-medium text-muted-foreground">{t('webdav.password')}</label>
                        <Input
                            type="password"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            placeholder="••••••"
                            className="rounded-xl mt-1"
                        />
                    </div>
                </div>
                <div>
                    <label className="text-xs font-medium text-muted-foreground">{t('webdav.remotePath')}</label>
                    <Input
                        value={remotePath}
                        onChange={(e) => setRemotePath(e.target.value)}
                        placeholder="/octopus-backup/"
                        className="rounded-xl mt-1"
                    />
                </div>
            </div>

            {/* Options */}
            <div className="space-y-3 rounded-lg border border-border/30 p-3">
                <div className="flex items-center justify-between gap-4">
                    <div className="text-sm text-muted-foreground">{t('webdav.includeStats')}</div>
                    <Switch checked={includeStats} onCheckedChange={setIncludeStats} />
                </div>
                <div className="flex items-center justify-between gap-4">
                    <div className="text-sm text-muted-foreground">{t('webdav.includeLogs')}</div>
                    <Switch checked={includeLogs} onCheckedChange={setIncludeLogs} />
                </div>
                <div className="grid grid-cols-2 gap-3">
                    <div>
                        <label className="text-xs font-medium text-muted-foreground">{t('webdav.intervalHours')}</label>
                        <Input
                            type="number"
                            min={1}
                            max={168}
                            value={intervalHours}
                            onChange={(e) => setIntervalHours(parseInt(e.target.value) || 6)}
                            className="rounded-xl mt-1"
                        />
                    </div>
                    <div>
                        <label className="text-xs font-medium text-muted-foreground">{t('webdav.maxBackups')}</label>
                        <Input
                            type="number"
                            min={1}
                            max={100}
                            value={maxBackups}
                            onChange={(e) => setMaxBackups(parseInt(e.target.value) || 10)}
                            className="rounded-xl mt-1"
                        />
                    </div>
                </div>
            </div>

            {/* Actions */}
            <div className="flex flex-col gap-2 sm:flex-row">
                <Button
                    variant="default"
                    className="rounded-xl flex-1"
                    onClick={onSave}
                    disabled={updateConfig.isPending}
                >
                    {updateConfig.isPending ? <Loader2 className="size-4 animate-spin" /> : <Cloud className="size-4" />}
                    {t('webdav.save')}
                </Button>
                <Button
                    variant="outline"
                    className="rounded-xl"
                    onClick={onTest}
                    disabled={testConnection.isPending}
                >
                    {testConnection.isPending ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
                    {t('webdav.test')}
                </Button>
                <Button
                    variant="outline"
                    className="rounded-xl"
                    onClick={onBackup}
                    disabled={triggerBackup.isPending || !enabled}
                >
                    {triggerBackup.isPending ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
                    {t('webdav.backupNow')}
                </Button>
            </div>

            {/* Remote backup list */}
            {files.length > 0 && (
                <>
                    <div className="h-px bg-border/50" />
                    <div className="space-y-2">
                        <div className="text-sm font-semibold text-card-foreground">{t('webdav.remoteBackups')}</div>
                        <div className="space-y-1 max-h-48 overflow-y-auto">
                            {files.map((file) => (
                                <div
                                    key={file.path}
                                    className="flex items-center justify-between gap-2 rounded-lg border border-border/20 px-3 py-2"
                                >
                                    <div className="min-w-0 flex-1">
                                        <div className="truncate text-xs font-medium">{file.name}</div>
                                        <div className="text-[10px] text-muted-foreground">
                                            {file.size > 0 ? `${(file.size / 1024).toFixed(1)} KB` : ''}
                                        </div>
                                    </div>
                                    <div className="flex gap-1 shrink-0">
                                        <Button
                                            variant="ghost"
                                            size="icon-sm"
                                            onClick={() => onRestore(file.name)}
                                            disabled={restoreBackup.isPending}
                                            title={t('webdav.restore')}
                                        >
                                            <Download className="size-3.5" />
                                        </Button>
                                        <Button
                                            variant="ghost"
                                            size="icon-sm"
                                            onClick={() => onDelete(file.name)}
                                            disabled={deleteBackup.isPending}
                                            title={t('webdav.delete')}
                                        >
                                            <Trash2 className="size-3.5 text-destructive" />
                                        </Button>
                                    </div>
                                </div>
                            ))}
                        </div>
                    </div>
                </>
            )}
        </div>
    );
}
