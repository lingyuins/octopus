'use client';

import { useState } from 'react';
import {
    useCredentialList,
    useDeleteCredential,
    useRunVerificationForProfile,
    useGenerateCLIExport,
    type APICredentialProfile,
    CLI_TOOLS,
} from '@/api/endpoints/api-credential';
import { LoadingState } from '@/components/common/LoadingState';
import { ErrorState } from '@/components/common/ErrorState';
import { KeySquare, Plus, Trash2, Pencil, ShieldCheck, Download, CircleDot } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/common/Toast';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { useTranslations } from 'next-intl';
import { CredentialDialog } from '../credential/CredentialDialog';

function healthColor(status: string): string {
    switch (status) {
        case 'healthy': return 'text-green-500';
        case 'error': return 'text-red-500';
        default: return 'text-muted-foreground';
    }
}

function CredentialCard({ profile, onEdit, onDelete, onVerify, onExport }: {
    profile: APICredentialProfile;
    onEdit: (p: APICredentialProfile) => void;
    onDelete: (id: number) => void;
    onVerify: (id: number) => void;
    onExport: (p: APICredentialProfile) => void;
}) {
    const t = useTranslations('credential');

    return (
        <div className="rounded-lg border bg-card text-card-foreground shadow-sm p-4 flex flex-col gap-3">
            <div className="flex items-start justify-between gap-2">
                <div className="flex items-center gap-2 min-w-0">
                    <CircleDot className={cn('h-3 w-3 shrink-0', healthColor(profile.health_status))} />
                    <h3 className="font-medium truncate">{profile.name}</h3>
                </div>
                <div className="flex items-center gap-0.5 sm:gap-1 shrink-0">
                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onVerify(profile.id)} title={t('verify')}>
                        <ShieldCheck className="h-3.5 w-3.5" />
                    </Button>
                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onExport(profile)} title={t('export')}>
                        <Download className="h-3.5 w-3.5" />
                    </Button>
                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onEdit(profile)}>
                        <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button variant="ghost" size="icon" className="h-7 w-7 text-destructive" onClick={() => onDelete(profile.id)}>
                        <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                </div>
            </div>
            <div className="text-xs text-muted-foreground space-y-1 min-w-0">
                <Badge variant="outline" className="text-xs font-normal">{profile.api_type}</Badge>
                <div className="truncate break-all">{profile.base_url}</div>
                {profile.last_verified_at && (
                    <div className="truncate">{t('lastVerified')}: {new Date(profile.last_verified_at).toLocaleString()}</div>
                )}
            </div>
        </div>
    );
}

export function CredentialPanel() {
    const { data: profiles, isLoading, isError, refetch } = useCredentialList();
    const deleteProfile = useDeleteCredential();
    const runVerify = useRunVerificationForProfile();
    const generateExport = useGenerateCLIExport();
    const [dialogOpen, setDialogOpen] = useState(false);
    const [editing, setEditing] = useState<APICredentialProfile | null>(null);
    const t = useTranslations('credential');

    const handleDelete = (id: number) => {
        if (!confirm(t('confirmDelete'))) return;
        deleteProfile.mutate(id, {
            onSuccess: () => toast.success(t('deleted')),
            onError: (err) => toast.error(err.message),
        });
    };

    const handleVerify = (id: number) => {
        runVerify.mutate({ id }, {
            onSuccess: (results) => {
                const allOK = results.every(r => r.success);
                if (allOK) {
                    toast.success(t('verifySuccess'));
                } else {
                    const failed = results.filter(r => !r.success).map(r => `${r.probe}: ${r.error}`).join('; ');
                    toast.error(`${t('verifyFailed')}: ${failed}`);
                }
            },
            onError: (err) => toast.error(err.message),
        });
    };

    const handleExport = (profile: APICredentialProfile) => {
        const tool = CLI_TOOLS[0];
        generateExport.mutate(
            { base_url: profile.base_url, api_key: profile.api_key, api_type: profile.api_type, tool },
            {
                onSuccess: (result) => {
                    navigator.clipboard.writeText(result.content).then(() => {
                        toast.success(`${t('exportCopied')} (${result.tool})`);
                    });
                },
                onError: (err) => toast.error(err.message),
            },
        );
    };

    const handleEdit = (p: APICredentialProfile) => {
        setEditing(p);
        setDialogOpen(true);
    };

    const handleCreate = () => {
        setEditing(null);
        setDialogOpen(true);
    };

    if (isLoading) return <LoadingState />;
    if (isError) return <ErrorState onRetry={refetch} />;

    return (
        <div className="flex flex-col gap-4">
            <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2 min-w-0">
                    <KeySquare className="h-5 w-5 shrink-0" />
                    <h2 className="text-lg font-semibold truncate">{t('title')}</h2>
                    <Badge variant="secondary" className="shrink-0">{profiles?.length ?? 0}</Badge>
                </div>
                <Button size="sm" onClick={handleCreate} className="shrink-0">
                    <Plus className="h-4 w-4 sm:mr-1" />
                    <span className="hidden sm:inline">{t('add')}</span>
                </Button>
            </div>

            {profiles && profiles.length > 0 ? (
                <div className="grid gap-3 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
                    {profiles.map((p) => (
                        <CredentialCard
                            key={p.id}
                            profile={p}
                            onEdit={handleEdit}
                            onDelete={handleDelete}
                            onVerify={handleVerify}
                            onExport={handleExport}
                        />
                    ))}
                </div>
            ) : (
                <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
                    <KeySquare className="h-12 w-12 mb-3 opacity-50" />
                    <p>{t('empty')}</p>
                    <Button variant="outline" size="sm" className="mt-3" onClick={handleCreate}>
                        <Plus className="h-4 w-4 mr-1" />
                        {t('add')}
                    </Button>
                </div>
            )}

            <CredentialDialog
                open={dialogOpen}
                onOpenChange={setDialogOpen}
                editing={editing}
            />
        </div>
    );
}
