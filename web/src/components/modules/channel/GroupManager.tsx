'use client';

import { useState } from 'react';
import {
    type ChannelGroup,
    useCreateChannelGroup,
    useDeleteChannelGroup,
    useUpdateChannelGroup,
} from '@/api/endpoints/channel';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { toast } from '@/components/common/Toast';
import { useTranslations } from 'next-intl';
import { Check, FolderTree, Pencil, Plus, Trash2, X } from 'lucide-react';

type ChannelGroupManagerProps = {
    groups: ChannelGroup[];
    channelCountByGroup: Map<number, number>;
    isLoading: boolean;
    isError: boolean;
};

export function ChannelGroupManager({
    groups,
    channelCountByGroup,
    isLoading,
    isError,
}: ChannelGroupManagerProps) {
    const t = useTranslations('channel.groupManager');
    const createChannelGroup = useCreateChannelGroup();
    const updateChannelGroup = useUpdateChannelGroup();
    const deleteChannelGroup = useDeleteChannelGroup();

    const [isCreating, setIsCreating] = useState(false);
    const [newGroupName, setNewGroupName] = useState('');
    const [editingGroupID, setEditingGroupID] = useState<number | null>(null);
    const [editingGroupName, setEditingGroupName] = useState('');

    const handleCreate = () => {
        const name = newGroupName.trim();
        if (!name) {
            return;
        }
        createChannelGroup.mutate(
            { name },
            {
                onSuccess: () => {
                    toast.success(t('createSuccess'));
                    setNewGroupName('');
                    setIsCreating(false);
                },
                onError: (error) => {
                    toast.error(error.message);
                },
            }
        );
    };

    const handleSaveRename = (groupID: number) => {
        const name = editingGroupName.trim();
        if (!name) {
            return;
        }
        updateChannelGroup.mutate(
            { id: groupID, name },
            {
                onSuccess: () => {
                    toast.success(t('renameSuccess'));
                    setEditingGroupID(null);
                    setEditingGroupName('');
                },
                onError: (error) => {
                    toast.error(error.message);
                },
            }
        );
    };

    const handleDelete = (group: ChannelGroup) => {
        if (!window.confirm(t('deleteConfirm', { name: group.name }))) {
            return;
        }
        deleteChannelGroup.mutate(group.id, {
            onSuccess: () => {
                toast.success(t('deleteSuccess'));
            },
            onError: (error) => {
                toast.error(error.message);
            },
        });
    };

    return (
        <section className="rounded-lg border border-border/30 bg-card/70 p-4 md:p-5">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="space-y-1">
                    <div className="inline-flex items-center gap-2 rounded-full border border-primary/12 bg-card px-3 py-1 text-[0.68rem] font-semibold text-primary">
                        <FolderTree className="size-3.5" />
                        {t('title')}
                    </div>
                    <p className="text-xs leading-5 text-muted-foreground">{t('hint')}</p>
                </div>
                {!isCreating ? (
                    <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => setIsCreating(true)}
                        className="h-9 rounded-lg"
                    >
                        <Plus className="size-4" />
                        {t('create')}
                    </Button>
                ) : null}
            </div>

            {isCreating ? (
                <div className="mt-4 flex flex-col gap-2 rounded-lg border border-border/25 bg-card p-3 sm:flex-row">
                    <Input
                        value={newGroupName}
                        onChange={(event) => setNewGroupName(event.target.value)}
                        placeholder={t('createPlaceholder')}
                        className="rounded-lg"
                    />
                    <div className="flex items-center gap-2">
                        <Button
                            type="button"
                            size="sm"
                            onClick={handleCreate}
                            disabled={createChannelGroup.isPending || !newGroupName.trim()}
                            className="h-9 rounded-lg"
                        >
                            <Check className="size-4" />
                            {t('save')}
                        </Button>
                        <Button
                            type="button"
                            variant="secondary"
                            size="sm"
                            onClick={() => {
                                setIsCreating(false);
                                setNewGroupName('');
                            }}
                            className="h-9 rounded-lg"
                        >
                            <X className="size-4" />
                            {t('cancel')}
                        </Button>
                    </div>
                </div>
            ) : null}

            <div className="mt-4 space-y-2">
                {isLoading ? (
                    <div className="rounded-lg border border-dashed border-border/30 bg-card p-4 text-sm text-muted-foreground">
                        {t('loading')}
                    </div>
                ) : isError ? (
                    <div className="rounded-lg border border-dashed border-border/30 bg-card p-4 text-sm text-muted-foreground">
                        {t('loadFailed')}
                    </div>
                ) : groups.length === 0 ? (
                    <div className="rounded-lg border border-dashed border-border/30 bg-card p-4 text-sm text-muted-foreground">
                        {t('empty')}
                    </div>
                ) : (
                    groups.map((group) => {
                        const channelCount = channelCountByGroup.get(group.id) ?? 0;
                        const isEditing = editingGroupID === group.id;

                        return (
                            <div key={group.id} className="rounded-lg border border-border/25 bg-card p-3">
                                {isEditing ? (
                                    <div className="flex flex-col gap-2 sm:flex-row">
                                        <Input
                                            value={editingGroupName}
                                            onChange={(event) => setEditingGroupName(event.target.value)}
                                            className="rounded-lg"
                                        />
                                        <div className="flex items-center gap-2">
                                            <Button
                                                type="button"
                                                size="sm"
                                                onClick={() => handleSaveRename(group.id)}
                                                disabled={updateChannelGroup.isPending || !editingGroupName.trim()}
                                                className="h-9 rounded-lg"
                                            >
                                                <Check className="size-4" />
                                                {t('save')}
                                            </Button>
                                            <Button
                                                type="button"
                                                variant="secondary"
                                                size="sm"
                                                onClick={() => {
                                                    setEditingGroupID(null);
                                                    setEditingGroupName('');
                                                }}
                                                className="h-9 rounded-lg"
                                            >
                                                <X className="size-4" />
                                                {t('cancel')}
                                            </Button>
                                        </div>
                                    </div>
                                ) : (
                                    <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                                        <div className="flex min-w-0 flex-wrap items-center gap-2">
                                            <span className="truncate text-sm font-medium text-card-foreground">{group.name}</span>
                                            {group.is_default ? (
                                                <Badge variant="secondary" className="rounded-full">
                                                    {t('defaultBadge')}
                                                </Badge>
                                            ) : null}
                                            <Badge variant="secondary" className="rounded-full">
                                                {t('count', { count: channelCount })}
                                            </Badge>
                                        </div>
                                        <div className="flex items-center gap-2">
                                            <Button
                                                type="button"
                                                variant="ghost"
                                                size="sm"
                                                onClick={() => {
                                                    setEditingGroupID(group.id);
                                                    setEditingGroupName(group.name);
                                                }}
                                                className="h-8 rounded-lg"
                                            >
                                                <Pencil className="size-4" />
                                                {t('rename')}
                                            </Button>
                                            {!group.is_default ? (
                                                <Button
                                                    type="button"
                                                    variant="ghost"
                                                    size="sm"
                                                    onClick={() => handleDelete(group)}
                                                    disabled={deleteChannelGroup.isPending}
                                                    className="h-8 rounded-lg text-destructive hover:text-destructive"
                                                >
                                                    <Trash2 className="size-4" />
                                                    {t('delete')}
                                                </Button>
                                            ) : null}
                                        </div>
                                    </div>
                                )}
                            </div>
                        );
                    })
                )}
            </div>
        </section>
    );
}
