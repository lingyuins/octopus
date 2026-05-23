'use client';

import { useMemo, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Database, Download, Upload, AlertTriangle, Loader2, Check, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { toast } from '@/components/common/Toast';
import { useExportDB, useImportDB } from '@/api/endpoints/setting';
import { Progress } from '@/components/ui/progress';
import { cn } from '@/lib/utils';

type ImportMode = 'incremental' | 'full';

export function SettingBackup() {
    const t = useTranslations('setting');

    const exportDB = useExportDB();
    const importDB = useImportDB();

    const [includeLogs, setIncludeLogs] = useState(false);
    const [includeStats, setIncludeStats] = useState(false);
    const [importMode, setImportMode] = useState<ImportMode>('incremental');

    const [file, setFile] = useState<File | null>(null);
    const fileInputRef = useRef<HTMLInputElement | null>(null);

    const rowsAffected = importDB.data?.rows_affected ?? null;
    const importProgress = importDB.data?.progress ?? [];
    const totalSteps = importProgress.length;
    const completedSteps = importProgress.filter(s => s.ok).length;
    const failedSteps = importProgress.filter(s => !s.ok).length;
    const progressValue = totalSteps > 0 ? Math.round((completedSteps / totalSteps) * (importDB.isPending ? 50 : 100)) : 0;

    const rowsAffectedList = useMemo(() => {
        if (!rowsAffected) return [];
        return Object.entries(rowsAffected)
            .sort(([a], [b]) => a.localeCompare(b))
            .map(([k, v]) => ({ table: k, count: v }));
    }, [rowsAffected]);

    const onPickFile = (f: File | null) => {
        setFile(f);
    };

    const onImport = async () => {
        if (!file) {
            toast.error(t('backup.import.noFile'));
            return;
        }
        try {
            await importDB.mutateAsync({ file, mode: importMode });
            toast.success(t('backup.import.success'));
            if (fileInputRef.current) fileInputRef.current.value = '';
            setFile(null);
        } catch (e) {
            toast.error(e instanceof Error ? e.message : t('backup.import.failed'));
        }
    };

    const onExport = async () => {
        try {
            await exportDB.mutateAsync({ include_logs: includeLogs, include_stats: includeStats });
            toast.success(t('backup.export.success'));
        } catch (e) {
            toast.error(e instanceof Error ? e.message : t('backup.export.failed'));
        }
    };

    return (
        <div className="rounded-xl border-border/35 bg-card p-6 space-y-5 text-card-foreground shadow-md ">
            <h2 className="text-lg font-bold text-card-foreground flex items-center gap-2">
                <Database className="h-5 w-5" />
                {t('backup.title')}
            </h2>

            {/* 导出 */}
            <div className="space-y-3 rounded-lg border-border/30 bg-card p-4 shadow-sm">
                <div className="text-sm font-semibold text-card-foreground">{t('backup.export.title')}</div>

                <div className="flex items-center justify-between gap-4">
                    <div className="text-sm text-muted-foreground">{t('backup.export.includeLogs')}</div>
                    <Switch checked={includeLogs} onCheckedChange={setIncludeLogs} />
                </div>

                <div className="flex items-center justify-between gap-4">
                    <div className="text-sm text-muted-foreground">{t('backup.export.includeStats')}</div>
                    <Switch checked={includeStats} onCheckedChange={setIncludeStats} />
                </div>

                <Button
                    type="button"
                    variant="outline"
                    className="w-full rounded-xl"
                    onClick={onExport}
                    disabled={exportDB.isPending}
                >
                    <Download className="size-4" />
                    {exportDB.isPending ? t('backup.export.exporting') : t('backup.export.button')}
                </Button>
            </div>

            <div className="h-px bg-border/50" />

            {/* 导入 */}
            <div className="space-y-3 rounded-lg border-border/30 bg-card p-4 shadow-sm">
                <div className="text-sm font-semibold text-card-foreground">{t('backup.import.title')}</div>

                <div className="flex items-center justify-between gap-4">
                    <div className="text-sm text-muted-foreground">{t('backup.import.mode.label')}</div>
                    <div className="flex gap-1 rounded-lg border border-border/30 bg-muted/30 p-0.5">
                        <button
                            type="button"
                            onClick={() => setImportMode('incremental')}
                            className={cn(
                                'rounded-md px-3 py-1.5 text-xs font-medium transition-colors',
                                importMode === 'incremental' ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
                            )}
                        >
                            {t('backup.import.mode.incremental')}
                        </button>
                        <button
                            type="button"
                            onClick={() => setImportMode('full')}
                            className={cn(
                                'rounded-md px-3 py-1.5 text-xs font-medium transition-colors',
                                importMode === 'full' ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
                            )}
                        >
                            {t('backup.import.mode.full')}
                        </button>
                    </div>
                </div>

                {importMode === 'full' && (
                    <div className="flex items-start gap-2 rounded-lg border border-amber-500/20 bg-amber-500/8 p-3 text-xs text-amber-700 dark:text-amber-300">
                        <AlertTriangle className="size-4 shrink-0 mt-0.5" />
                        <span>{t('backup.import.mode.fullWarning')}</span>
                    </div>
                )}

                <Input
                    ref={fileInputRef}
                    type="file"
                    accept="application/json,.json"
                    onChange={(e) => onPickFile(e.target.files?.[0] ?? null)}
                    className="rounded-xl"
                />

                <Button
                    type="button"
                    variant="destructive"
                    className="w-full rounded-xl"
                    onClick={onImport}
                    disabled={importDB.isPending}
                >
                    {importDB.isPending ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
                    {importDB.isPending ? t('backup.import.importing') : t('backup.import.button')}
                </Button>

                {(importDB.isPending || importProgress.length > 0) && (
                    <div className="space-y-2 pt-2">
                        <Progress value={progressValue} className="h-1.5" />
                        {importProgress.length > 0 && (
                            <div className="space-y-1 max-h-48 overflow-y-auto">
                                {importProgress.map((step, i) => (
                                    <div key={i} className={cn(
                                        'flex items-center gap-2 text-xs rounded-md px-2 py-1',
                                        step.ok ? 'text-emerald-600 dark:text-emerald-400' : 'text-destructive bg-destructive/5'
                                    )}>
                                        {step.ok ? <Check className="size-3.5 shrink-0" /> : <X className="size-3.5 shrink-0" />}
                                        <span className="tabular-nums w-10 shrink-0 text-muted-foreground">{step.mode}</span>
                                        <span className="truncate flex-1">{step.table}</span>
                                        <span className="tabular-nums shrink-0 text-muted-foreground">{step.rows_affected}</span>
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>
                )}

                {rowsAffectedList.length > 0 && (
                    <div className="mt-2 space-y-1">
                        <div className="text-xs font-semibold text-card-foreground">{t('backup.import.result')}</div>
                        <div className="grid grid-cols-2 gap-1 text-xs text-muted-foreground">
                            {rowsAffectedList.map((it) => (
                                <div key={it.table} className="flex justify-between gap-2">
                                    <span className="truncate">{it.table}</span>
                                    <span className="tabular-nums">{it.count}</span>
                                </div>
                            ))}
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}


