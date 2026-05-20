'use client';

import { useEffect, useMemo, useState } from 'react';
import {
    type ChannelGroup,
    useChannelGroupList,
    useChannelList,
    useCreateChannelGroup,
    useDeleteChannelGroup,
    useUpdateChannelGroup,
} from '@/api/endpoints/channel';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
    MorphingDialog,
    MorphingDialogClose,
    MorphingDialogContainer,
    MorphingDialogContent,
    MorphingDialogTrigger,
} from '@/components/ui/morphing-dialog';
import { toast } from '@/components/common/Toast';
import { useTranslations } from 'next-intl';
import { Check, FolderTree, Pencil, Plus, Trash2, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useToolbarViewOptionsStore } from '@/components/modules/toolbar/view-options-store';

type ChannelGroupManagerPanelProps = {
    groups: ChannelGroup[];
    channelCountByGroup: Map<number, number>;
    selectedGroupId: number | null;
    isLoading: boolean;
    isError: boolean;
    onSelectGroup: (groupId: number) => void;
    className?: string;
};

export function getChannelGroupDisplayName(group: ChannelGroup, defaultName: string) {
    return group.is_default ? defaultName : group.name;
}

export function ChannelGroupManagerPanel({
    groups,
    channelCountByGroup,
    selectedGroupId,
    isLoading,
    isError,
    onSelectGroup,
    className,
}: ChannelGroupManagerPanelProps) {
    const t = useTranslations('channel.groupManager');
    const defaultName = t('defaultName');
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
        <section className={cn('rounded-lg border border-border/30 bg-card/70 p-4 md:p-5', className)}>
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
                        const isSelected = selectedGroupId === group.id;
                        const displayName = getChannelGroupDisplayName(group, defaultName);

                        return (
                            <div
                                key={group.id}
                                className={cn(
                                    'rounded-lg border border-border/25 bg-card p-3',
                                    isSelected && 'border-primary/35 bg-primary/5'
                                )}
                            >
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
                                            <span className="truncate text-sm font-medium text-card-foreground">{displayName}</span>
                                            {group.is_default ? (
                                                <Badge variant="secondary" className="rounded-full">
                                                    {t('defaultBadge')}
                                                </Badge>
                                            ) : null}
                                            <Badge variant="secondary" className="rounded-full">
                                                {t('count', { count: channelCount })}
                                            </Badge>
                                        </div>
                                        <div className="flex flex-wrap items-center gap-2">
                                            <Button
                                                type="button"
                                                variant={isSelected ? 'secondary' : 'ghost'}
                                                size="sm"
                                                onClick={() => onSelectGroup(group.id)}
                                                disabled={isSelected}
                                                className="h-8 rounded-lg"
                                            >
                                                <Check className="size-4" />
                                                {isSelected ? t('current') : t('select')}
                                            </Button>
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

export function ChannelGroupManagerDialog({ className }: { className?: string }) {
    const t = useTranslations('channel.groupManager');
    const defaultName = t('defaultName');
    const { data: channelsData = [] } = useChannelList();
    const { data: channelGroupsData = [], isLoading, isError } = useChannelGroupList();
    const selectedGroupId = useToolbarViewOptionsStore((s) => s.selectedChannelGroupId);
    const setSelectedGroupId = useToolbarViewOptionsStore((s) => s.setSelectedChannelGroupId);

    const groups = useMemo<ChannelGroup[]>(() => {
        if (channelGroupsData.length > 0) {
            return [...channelGroupsData].sort((a, b) => {
                if (a.is_default !== b.is_default) {
                    return a.is_default ? -1 : 1;
                }
                if (a.created_at !== b.created_at) {
                    return a.created_at - b.created_at;
                }
                return a.id - b.id;
            });
        }

        const fallbackIDs = Array.from(new Set(channelsData.map((item) => item.raw.group_id))).filter((id) => id > 0);
        return fallbackIDs.map((id, index) => ({
            id,
            name: index === 0 ? t('fallbackName') : t('fallbackNameWithID', { id }),
            is_default: index === 0,
            created_at: index,
            updated_at: index,
        }));
    }, [channelGroupsData, channelsData, t]);

    const channelCountByGroup = useMemo(() => {
        const counts = new Map<number, number>();
        for (const item of channelsData) {
            counts.set(item.raw.group_id, (counts.get(item.raw.group_id) ?? 0) + 1);
        }
        return counts;
    }, [channelsData]);

    const activeGroup = useMemo(() => {
        if (groups.length === 0) {
            return null;
        }
        return groups.find((group) => group.id === selectedGroupId) ?? groups[0];
    }, [groups, selectedGroupId]);

    useEffect(() => {
        if (activeGroup && selectedGroupId !== activeGroup.id) {
            setSelectedGroupId(activeGroup.id);
        }
    }, [activeGroup, selectedGroupId, setSelectedGroupId]);

    return (
        <MorphingDialog>
            <MorphingDialogTrigger ariaLabel={t('title')} className={className}>
                <FolderTree className="size-4 transition-colors duration-300" />
                <span className="max-w-32 truncate sm:max-w-44">
                    {activeGroup ? getChannelGroupDisplayName(activeGroup, defaultName) : t('title')}
                </span>
            </MorphingDialogTrigger>
            <MorphingDialogContainer>
                <MorphingDialogContent className="w-[min(100vw-1rem,42rem)] max-w-full rounded-xl border border-border bg-card p-3 text-card-foreground shadow-lg md:w-[min(100vw-3rem,48rem)] md:p-4">
                    <MorphingDialogClose className="right-3 top-3 text-muted-foreground hover:text-foreground md:right-4 md:top-4" />
                    <div className="max-h-[calc(100dvh-2rem)] overflow-y-auto md:max-h-[calc(100dvh-3rem)]">
                        <ChannelGroupManagerPanel
                            groups={groups}
                            channelCountByGroup={channelCountByGroup}
                            selectedGroupId={activeGroup?.id ?? null}
                            isLoading={isLoading}
                            isError={isError}
                            onSelectGroup={setSelectedGroupId}
                            className="border-0 bg-transparent p-0"
                        />
                    </div>
                </MorphingDialogContent>
            </MorphingDialogContainer>
        </MorphingDialog>
    );
}
