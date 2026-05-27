'use client';

import { useMemo, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Database, Download, Upload, AlertTriangle, Loader2, Check, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { toast } from '@/components/common/Toast';
import { useExportDB, useImportDB, useMigrateDatabase, useTestDatabaseConnection } from '@/api/endpoints/setting';
import { Progress } from '@/components/ui/progress';
import { cn } from '@/lib/utils';

type ImportMode = 'incremental' | 'full';

export function SettingBackup() {
    const t = useTranslations('setting');

    const exportDB = useExportDB();
    const importDB = useImportDB();
    const testDatabase = useTestDatabaseConnection();
    const migrateDatabase = useMigrateDatabase();

    const [includeLogs, setIncludeLogs] = useState(false);
    const [includeStats, setIncludeStats] = useState(false);
    const [importMode, setImportMode] = useState<ImportMode>('incremental');
    const [targetType, setTargetType] = useState<'sqlite' | 'mysql' | 'postgres'>('sqlite');
    const [targetPath, setTargetPath] = useState('data/data-next.db');
    const [migrateLogs, setMigrateLogs] = useState(false);
    const [migrateStats, setMigrateStats] = useState(false);

    const [file, setFile] = useState<File | null>(null);
    const fileInputRef = useRef<HTMLInputElement | null>(null);

    const rowsAffected = importDB.data?.rows_affected ?? null;
    const importProgress = importDB.data?.progress ?? [];
    const totalSteps = importProgress.length;
    const completedSteps = importProgress.filter(s => s.ok).length;
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

    const migrationPayload = {
        type: targetType,
        path: targetPath,
        include_logs: migrateLogs,
        include_stats: migrateStats,
    };

    const onTestDatabase = async () => {
        if (!targetPath.trim()) {
            toast.error(t('backup.migration.targetRequired')); 
            return;
        }
        try {
            await testDatabase.mutateAsync(migrationPayload);
            toast.success(t('backup.migration.testSuccess')); 
        } catch (e) {
            toast.error(e instanceof Error ? e.message : t('backup.migration.testFailed')); 
        }
    };

    const onMigrateDatabase = async () => {
        if (!targetPath.trim()) {
            toast.error('Please enter target database path / DSN');
            return;
        }
        if (!window.confirm(t('backup.migration.confirm'))) {
            return;
        }
        try {
            await migrateDatabase.mutateAsync(migrationPayload);
            toast.success(t('backup.migration.success')); 
        } catch (e) {
            toast.error(e instanceof Error ? e.message : t('backup.migration.failed')); 
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
                    <div>
                        <div className="text-sm text-muted-foreground">{t('backup.export.includeLogs')}</div>
                        {includeLogs && <div className="text-[11px] text-muted-foreground/70">{t('backup.export.includeLogsHint')}</div>}
                    </div>
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

            <div className="space-y-3 rounded-lg border border-amber-500/20 bg-card p-4 shadow-sm">
                <div className="flex items-start gap-2">
                    <AlertTriangle className="mt-0.5 size-4 shrink-0 text-amber-600" />
                    <div>
                        <div className="text-sm font-semibold text-card-foreground">{t('backup.migration.title')}</div>
                        <div className="mt-1 text-xs leading-5 text-muted-foreground">
                            {t('backup.migration.description')}
                        </div>
                    </div>
                </div>

                <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                    <select
                        value={targetType}
                        onChange={(e) => setTargetType(e.target.value as 'sqlite' | 'mysql' | 'postgres')}
                        className="h-10 rounded-xl border border-input bg-background px-3 text-sm"
                    >
                        <option value="sqlite">SQLite</option>
                        <option value="mysql">MySQL</option>
                        <option value="postgres">PostgreSQL</option>
                    </select>
                    <Input
                        className="md:col-span-2 rounded-xl"
                        value={targetPath}
                        onChange={(e) => setTargetPath(e.target.value)}
                        placeholder={targetType === 'sqlite' ? 'data/data-next.db' : 'user:pass@tcp(host:3306)/octopus'}
                    />
                </div>

                <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                    <div className="flex items-center justify-between gap-4 rounded-lg border border-border/30 p-3">
                        <div>
                            <div className="text-sm text-muted-foreground">{t('backup.migration.migrateLogs')}</div>
                            <div className="text-[11px] text-muted-foreground/70">{t('backup.migration.migrateLogsHint')}</div>
                        </div>
                        <Switch checked={migrateLogs} onCheckedChange={setMigrateLogs} />
                    </div>
                    <div className="flex items-center justify-between gap-4 rounded-lg border border-border/30 p-3">
                        <div className="text-sm text-muted-foreground">{t('backup.migration.migrateStats')}</div>
                        <Switch checked={migrateStats} onCheckedChange={setMigrateStats} />
                    </div>
                </div>

                <div className="flex flex-col gap-2 sm:flex-row">
                    <Button type="button" variant="outline" className="rounded-xl" onClick={onTestDatabase} disabled={testDatabase.isPending || migrateDatabase.isPending}>
                        {testDatabase.isPending ? <Loader2 className="size-4 animate-spin" /> : <Check className="size-4" />}
                        {t('backup.migration.testButton')}
                    </Button>
                    <Button type="button" variant="destructive" className="rounded-xl" onClick={onMigrateDatabase} disabled={migrateDatabase.isPending || testDatabase.isPending}>
                        {migrateDatabase.isPending ? <Loader2 className="size-4 animate-spin" /> : <Database className="size-4" />}
                        {migrateDatabase.isPending ? t('backup.migration.migrating') : t('backup.migration.button')}
                    </Button>
                </div>

                {migrateDatabase.data ? (
                    <div className="rounded-lg border border-emerald-500/20 bg-emerald-500/8 p-3 text-xs text-emerald-700 dark:text-emerald-300">
                        {t('backup.migration.successDetail', { type: migrateDatabase.data.type, path: migrateDatabase.data.path })}
                    </div>
                ) : null}
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
