'use client';

import { useEffect, useId, useRef, useState } from 'react';
import { GripVertical, X, Trash2, Waves, Dot, CircleAlert, CircleCheck, Loader2 } from 'lucide-react';
import {
    DragDropContext,
    Draggable,
    Droppable,
    type DraggableProvided,
    type DropResult,
} from '@hello-pangea/dnd';
import { motion, AnimatePresence } from 'motion/react';
import { cn } from '@/lib/utils';
import { getModelIcon } from '@/lib/model-icons';
import type { LLMChannel } from '@/api/endpoints/model';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/animate-ui/components/animate/tooltip';
import { useTranslations } from 'next-intl';

export interface SelectedMember extends LLMChannel {
    id: string;
    item_id?: number;
    weight?: number;
}

export type MemberAvailabilityStatus = 'idle' | 'testing' | 'available' | 'unavailable';

export interface MemberAvailabilityMeta {
    status: MemberAvailabilityStatus;
    message?: string;
}

function reorderList<T>(list: T[], startIndex: number, endIndex: number): T[] {
    const result = [...list];
    const [removed] = result.splice(startIndex, 1);
    result.splice(endIndex, 0, removed);
    return result;
}

type MemberItemDnd = {
    innerRef: DraggableProvided['innerRef'];
    draggableProps: DraggableProvided['draggableProps'];
    dragHandleProps: DraggableProvided['dragHandleProps'];
};

function MemberItem({
    member,
    onRemove,
    onWeightChange,
    isRemoving,
    index,
    showWeight = false,
    showConfirmDelete = true,
    layoutScope,
    dnd,
    isDragging,
    availability,
}: {
    member: SelectedMember;
    onRemove: (id: string) => void;
    onWeightChange?: (id: string, weight: number) => void;
    isRemoving?: boolean;
    index: number;
    showWeight?: boolean;
    showConfirmDelete?: boolean;
    layoutScope?: string;
    dnd: MemberItemDnd;
    isDragging: boolean;
    availability?: MemberAvailabilityMeta;
}) {
    const { Avatar: ModelAvatar } = getModelIcon(member.name);
    const t = useTranslations('group');
    const [confirmDelete, setConfirmDelete] = useState(false);
    const isDisabled = member.enabled === false;
    const availabilityStatus = availability?.status ?? 'idle';

    return (
        <div
            // DnD libraries provide imperative refs/props; the hook lint rule (`react-hooks/refs`)
            // flags this pattern, but it's safe and required for correct drag behavior.
            // eslint-disable-next-line react-hooks/refs
            ref={dnd.innerRef}
            // eslint-disable-next-line react-hooks/refs
            {...dnd.draggableProps}
            className={cn('rounded-lg grid transition-[grid-template-rows] duration-200', isRemoving ? 'grid-rows-[0fr]' : 'grid-rows-[1fr]')}
            // eslint-disable-next-line react-hooks/refs
            style={{
                /* eslint-disable-next-line react-hooks/refs */
                ...(dnd.draggableProps?.style ?? {}),
                ...(isDragging ? { zIndex: 50, boxShadow: '0 8px 32px rgba(0,0,0,0.15)' } : null),
            }}
        >
            <div className={cn(
                'group/item relative flex items-center gap-2 overflow-hidden rounded-lg border border-border/30 bg-card px-3 py-2.5 select-none transition-[opacity,transform,border-color,box-shadow,background-color] duration-200',
                isRemoving && 'opacity-0',
                isDisabled && 'opacity-60 grayscale',
                availabilityStatus === 'unavailable' && 'border-destructive/40 bg-destructive/5',
                !isRemoving && !isDragging && 'hover:-translate-y-0.5 hover:border-primary/16 hover:bg-card',
                isDragging && 'border-primary/30 bg-card'
            )}>
                <span className={cn(
                    'relative grid size-7 shrink-0 place-items-center rounded-md text-xs font-bold',
                    isDisabled ? 'bg-muted text-muted-foreground' : 'bg-primary/10 text-primary'
                )}>
                    {index + 1}
                </span>

                <div
                    className={cn(
                        'relative rounded-md p-1 touch-none transition-colors',
                        isDisabled
                            ? 'cursor-grab active:cursor-grabbing hover:bg-muted/60'
                            : 'cursor-grab active:cursor-grabbing hover:bg-primary/8'
                    )}
                    // eslint-disable-next-line react-hooks/refs
                    {...dnd.dragHandleProps}
                >
                    <GripVertical className="size-3.5 text-muted-foreground" />
                </div>

                <span className={cn('relative', isDisabled && 'opacity-70')}>
                    <ModelAvatar size={18} />
                </span>

                <div className="relative flex min-w-0 flex-1 flex-col">
                    <div className="flex min-w-0 items-center gap-2">
                        <Tooltip side="top" sideOffset={10} align="start">
                            <TooltipTrigger className={cn(
                                'min-w-0 text-sm font-medium truncate leading-tight',
                                isDisabled && 'text-muted-foreground',
                                availabilityStatus === 'unavailable' && 'text-destructive'
                            )}>
                                {member.name}
                            </TooltipTrigger>
                            <TooltipContent key={member.name}>{member.name}</TooltipContent>
                        </Tooltip>
                        {availabilityStatus === 'testing' ? <Loader2 className="size-3.5 shrink-0 animate-spin text-muted-foreground" /> : null}
                        {availabilityStatus === 'available' ? <CircleCheck className="size-3.5 shrink-0 text-emerald-500" /> : null}
                        {availabilityStatus === 'unavailable' ? (
                            <Tooltip side="top" sideOffset={10} align="center">
                                <TooltipTrigger asChild>
                                    <span className="inline-flex shrink-0 items-center gap-1 rounded-full border border-destructive/25 bg-destructive/10 px-1.5 py-0.5 text-[10px] font-medium text-destructive">
                                        <CircleAlert className="size-3" />
                                        {t('detail.availability.unavailableBadge')}
                                    </span>
                                </TooltipTrigger>
                                <TooltipContent>{availability?.message || t('detail.availability.unavailableBadge')}</TooltipContent>
                            </Tooltip>
                        ) : null}
                    </div>
                    <span className="inline-flex items-center gap-1 truncate text-[10px] leading-tight text-muted-foreground">
                        <Dot className="size-3 opacity-70" />
                        {member.channel_name}
                    </span>
                </div>

                {showWeight && (
                    <input
                        type="number"
                        min={1}
                        value={member.weight ?? 1}
                        onChange={(e) => onWeightChange?.(member.id, Math.max(1, parseInt(e.target.value) || 1))}
                        className={cn(
                            'h-7 w-14 rounded-md border border-border/35 bg-card text-center text-xs shadow-sm focus:outline-none focus:ring-1 focus:ring-primary',
                            isDisabled && 'text-muted-foreground'
                        )}
                    />
                )}

                {(!showConfirmDelete || !confirmDelete) && (
                    <motion.button
                        layoutId={`delete-btn-member-${layoutScope ?? 'default'}-${member.id}`}
                        type="button"
                        onClick={() => showConfirmDelete ? setConfirmDelete(true) : onRemove(member.id)}
                        className="relative rounded-md p-1.5 transition-colors hover:bg-destructive/10 hover:text-destructive"
                        initial={false}
                        animate={{ opacity: 1, x: 0 }}
                        transition={{ duration: 0.15 }}
                        style={{ pointerEvents: 'auto' }}
                    >
                        <X className="size-3" />
                    </motion.button>
                )}

                <AnimatePresence>
                    {showConfirmDelete && confirmDelete && (
                        <motion.div
                            layoutId={`delete-btn-member-${layoutScope ?? 'default'}-${member.id}`}
                            className="absolute inset-0 flex items-center justify-center gap-2 rounded-lg bg-destructive p-1.5"
                            transition={{ type: 'spring', stiffness: 400, damping: 30 }}
                        >
                            <button
                                type="button"
                                onClick={() => setConfirmDelete(false)}
                                className="flex h-6 w-6 items-center justify-center rounded-md bg-destructive-foreground/20 text-destructive-foreground transition-all hover:bg-destructive-foreground/30 active:scale-95"
                            >
                                <X className="h-3 w-3" />
                            </button>
                            <button
                                type="button"
                                onClick={() => onRemove(member.id)}
                                className="flex-1 h-6 flex items-center justify-center gap-1.5 rounded-md bg-destructive-foreground text-destructive text-xs font-semibold transition-all hover:bg-destructive-foreground/90 active:scale-[0.98]"
                            >
                                <Trash2 className="h-3 w-3" />
                            </button>
                        </motion.div>
                    )}
                </AnimatePresence>
            </div>
        </div>
    );
}

export interface MemberListProps {
    members: SelectedMember[];
    onReorder: (members: SelectedMember[]) => void;
    onRemove: (id: string) => void;
    onWeightChange?: (id: string, weight: number) => void;
    /**
     * When true, auto-scroll the list to bottom when a *new visible* member appears
     * (i.e. a new member id is added). Useful in "editor" flows. Defaults to true.
     */
    autoScrollOnAdd?: boolean;
    onDragStart?: () => void;
    /**
     * Called only when a drop results in a different order (i.e. commit reorder).
     * Useful for persisting the new order.
     */
    onDrop?: (members: SelectedMember[]) => void;
    /**
     * Called whenever a drag ends (including cancel / same-index drop).
     * Useful for lifecycle cleanup (e.g. clearing "isDragging" flags).
     */
    onDragFinish?: () => void;
    removingIds?: Set<string>;
    showWeight?: boolean;
    /**
     * When true, show a confirmation overlay before removing an item.
     * When false, clicking the delete button removes the item immediately.
     * Defaults to true.
     */
    showConfirmDelete?: boolean;
    layoutScope?: string;
    availabilityById?: Record<string, MemberAvailabilityMeta>;
}

export function MemberList({
    members,
    onReorder,
    onRemove,
    onWeightChange,
    autoScrollOnAdd = true,
    onDragStart,
    onDrop,
    onDragFinish,
    removingIds = new Set(),
    showWeight = false,
    showConfirmDelete = true,
    layoutScope: externalLayoutScope,
    availabilityById = {},
}: MemberListProps) {
    const internalLayoutScope = useId();
    const layoutScope = externalLayoutScope ?? internalLayoutScope;

    const scrollContainerRef = useRef<HTMLDivElement | null>(null);
    const prevMemberCountRef = useRef<number>(0);
    const hasMountedRef = useRef(false);

    const visibleCount = members.filter((m) => !removingIds.has(m.id)).length;
    const isEmpty = visibleCount === 0;
    const t = useTranslations('group');

    useEffect(() => {
        // Skip the initial mount so we don't auto-scroll on first render / initial data load.
        if (!hasMountedRef.current) {
            hasMountedRef.current = true;
            prevMemberCountRef.current = members.length;
            return;
        }

        if (!autoScrollOnAdd) {
            prevMemberCountRef.current = members.length;
            return;
        }

        const hasNewMember = members.length > prevMemberCountRef.current;

        // Auto-scroll only when member count increases (i.e. added; not reorder / not "unhide").
        if (hasNewMember) {
            // Wait a tick for DOM/placeholder/layout to settle.
            requestAnimationFrame(() => {
                const el = scrollContainerRef.current;
                if (!el) return;
                el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' });
            });
        }

        prevMemberCountRef.current = members.length;
    }, [members.length, autoScrollOnAdd]);

    const handleDragEnd = (result: DropResult) => {
        try {
            const { destination, source } = result;
            if (!destination) return;
            if (destination.index === source.index) return;

            const next = reorderList(members, source.index, destination.index);
            onReorder(next);
            onDrop?.(next);
        } finally {
            // Ensure drag lifecycle always finishes, even when drop is canceled.
            onDragFinish?.();
        }
    };

    return (
        <div className="relative h-full min-h-0">
            <div
                className={cn(
                    'absolute inset-0 flex flex-col items-center justify-center gap-3 text-muted-foreground',
                    'transition-opacity duration-200 ease-out',
                    isEmpty ? 'opacity-100' : 'opacity-0 pointer-events-none'
                )}
            >
                <div className="grid size-16 place-items-center rounded-lg border border-border/25 bg-card">
                    <Waves className="size-7 opacity-60" />
                </div>
                <div className="space-y-1 text-center">
                    <div className="text-sm font-medium text-foreground">{t('card.empty')}</div>
                    <div className="text-xs text-muted-foreground">{t('form.addItem')}</div>
                </div>
            </div>

            <div
                className={cn(
                    'h-full min-h-0 overflow-y-auto transition-opacity duration-200',
                    isEmpty ? 'opacity-0' : 'opacity-100'
                )}
                ref={scrollContainerRef}
            >
                <DragDropContext
                    onDragStart={() => onDragStart?.()}
                    onDragEnd={handleDragEnd}
                >
                    <Droppable droppableId={`members-${layoutScope}`}>
                        {(droppableProvided) => (
                            <div
                                ref={droppableProvided.innerRef}
                                {...droppableProvided.droppableProps}
                                className="flex flex-col space-y-2 p-2.5"
                            >
                                {members.map((member, index) => (
                                    <Draggable
                                        key={member.id}
                                        draggableId={member.id}
                                        index={index}
                                        isDragDisabled={removingIds.has(member.id)}
                                    >
                                        {(draggableProvided, snapshot) => (
                                            <MemberItem
                                                member={member}
                                                onRemove={onRemove}
                                                onWeightChange={onWeightChange}
                                                isRemoving={removingIds.has(member.id)}
                                                index={index}
                                                showWeight={showWeight}
                                                showConfirmDelete={showConfirmDelete}
                                                layoutScope={layoutScope}
                                                dnd={{
                                                    innerRef: draggableProvided.innerRef,
                                                    draggableProps: draggableProvided.draggableProps,
                                                    dragHandleProps: draggableProvided.dragHandleProps,
                                                }}
                                                isDragging={snapshot.isDragging}
                                                availability={availabilityById[member.id]}
                                            />
                                        )}
                                    </Draggable>
                                ))}
                                {droppableProvided.placeholder}
                            </div>
                        )}
                    </Droppable>
                </DragDropContext>
            </div>
        </div>
    );
}
