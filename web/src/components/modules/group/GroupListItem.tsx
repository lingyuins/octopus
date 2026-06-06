'use client';

import { useState, useMemo, useCallback, useEffect, useRef } from 'react';
import {
    Trash2,
    X,
    Pencil,
    Activity,
    Loader2,
    CircleCheck,
    CircleX,
    Waves,
    TestTubeDiagonal,
    ChevronDown,
    AlertTriangle,
} from 'lucide-react';
import { motion, AnimatePresence } from 'motion/react';
import {
    type Group,
    useDeleteGroup,
    useUpdateGroup,
    useTestGroup,
    useTestDraftGroup,
    useGroupTestProgress,
    type GroupTestResult,
    GroupMode,
    type GroupUpdateRequest,
} from '@/api/endpoints/group';
import { useModelChannelList } from '@/api/endpoints/model';
import { useTranslations } from 'next-intl';
import { cn } from '@/lib/utils';
import { toast } from '@/components/common/Toast';
import { CopyIconButton } from '@/components/common/CopyButton';
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/animate-ui/components/animate/tooltip';
import type { MemberAvailabilityMeta, SelectedMember } from './ItemList';
import { MemberList } from './ItemList';
import { GroupEditor, type GroupEditorValues } from './Editor';
import { AIRouteButton } from './AIRouteButton';
import {
    buildChannelNameByModelKey,
    modelChannelKey,
    MODE_LABELS,
    endpointTypeLabelKey,
    normalizeEndpointType,
    supportsGroupTest,
} from './utils';
import { getModelIcon } from '@/lib/model-icons';
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
import { Progress } from '@/components/ui/progress';

// ---------------------------------------------------------------------------
// EditDialogContent (duplicated from Card.tsx to keep this component self-contained)
// ---------------------------------------------------------------------------

interface EditDialogContentProps {
    group: Group;
    editMembers: SelectedMember[];
    isSubmitting: boolean;
    onSubmit: (values: GroupEditorValues, onDone?: () => void) => void;
    onTestAvailability: () => void;
    onRemoveFailedModels: () => void;
    isTestingAvailability: boolean;
    canRemoveFailedModels: boolean;
    testProgressCompleted: number;
    testProgressTotal: number;
    testProgressValue: number;
    testResults: GroupTestResult[];
    availabilityByMemberId: Record<string, MemberAvailabilityMeta>;
    availabilitySummary?: {
        unavailableCount: number;
        availableCount: number;
        allAvailable: boolean;
        fullyMatched: boolean;
    };
}

function EditDialogContent({
    group,
    editMembers,
    isSubmitting,
    onSubmit,
    onTestAvailability,
    onRemoveFailedModels,
    isTestingAvailability,
    canRemoveFailedModels,
    testProgressCompleted,
    testProgressTotal,
    testProgressValue,
    testResults,
    availabilityByMemberId,
    availabilitySummary,
}: EditDialogContentProps) {
    const { setIsOpen } = useMorphingDialog();
    const t = useTranslations('group');
    return (
        <div className="relative flex h-full min-h-0 w-full max-w-full flex-col">
            <MorphingDialogTitle className="shrink-0">
                <header className="relative mb-4 flex items-start justify-between gap-4">
                    <div className="space-y-3">
                        <div className="inline-flex items-center gap-2 rounded-full border border-primary/15 bg-card px-3 py-1 text-[0.68rem] font-semibold text-primary">
                            <Waves className="size-3.5" />
                            {t('detail.actions.edit')}
                        </div>
                        <div className="space-y-1">
                            <h2 className="text-2xl font-bold text-card-foreground">
                                {t('detail.actions.edit')}
                            </h2>
                            <p className="text-sm text-muted-foreground">{group.name}</p>
                        </div>
                    </div>
                    <div className="flex items-center gap-2">
                        {group.id && supportsGroupTest(group.endpoint_type) ? (
                            <AIRouteButton
                                scope="group"
                                groupId={group.id}
                                variant="default"
                                className="h-10 rounded-lg px-3"
                                onSuccess={() => setIsOpen(false)}
                            />
                        ) : null}
                        <MorphingDialogClose className="relative right-0 top-0" />
                    </div>
                </header>
            </MorphingDialogTitle>
            <MorphingDialogDescription className="flex flex-1 min-h-0 flex-col gap-4 overflow-hidden pr-1">
                {group.id ? (
                    <section className="shrink-0 rounded-lg border border-border/25 bg-card p-4">
                        <div className="flex flex-wrap items-start justify-between gap-3">
                            <div className="space-y-1">
                                <h3 className="text-sm font-semibold text-foreground">
                                    {t('detail.availability.title')}
                                </h3>
                                <p className="text-xs text-muted-foreground">
                                    {t('detail.availability.description')}
                                </p>
                            </div>
                            <div className="flex flex-wrap items-center gap-2">
                                {canRemoveFailedModels ? (
                                    <button
                                        type="button"
                                        onClick={onRemoveFailedModels}
                                        className="inline-flex h-9 items-center gap-2 rounded-lg border border-destructive/20 bg-destructive/8 px-3 text-sm font-medium text-destructive transition-colors hover:bg-destructive/12"
                                    >
                                        <Trash2 className="size-4" />
                                        {t('detail.actions.removeFailedModels')}
                                    </button>
                                ) : null}
                                <button
                                    type="button"
                                    onClick={onTestAvailability}
                                    disabled={isTestingAvailability || !group.id}
                                    className={cn(
                                        'inline-flex h-9 items-center gap-2 rounded-lg border px-3 text-sm font-medium transition-colors',
                                        isTestingAvailability
                                            ? 'cursor-not-allowed border-border/25 bg-muted text-muted-foreground'
                                            : 'border-primary/20 bg-primary text-primary-foreground hover:opacity-90',
                                    )}
                                >
                                    {isTestingAvailability ? (
                                        <Loader2 className="size-4 animate-spin" />
                                    ) : (
                                        <TestTubeDiagonal className="size-4" />
                                    )}
                                    {t('detail.availability.testAll')}
                                </button>
                            </div>
                        </div>
                        {isTestingAvailability || availabilitySummary ? (
                            <div className="mt-4 space-y-3">
                                <Progress value={testProgressValue} className="h-2" />
                                <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                                    <span>
                                        {t('card.testProgressCount', {
                                            completed: testProgressCompleted,
                                            total: testProgressTotal || editMembers.length,
                                        })}
                                    </span>
                                    {availabilitySummary?.fullyMatched ? (
                                        <span
                                            className={cn(
                                                'inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-medium',
                                                availabilitySummary.allAvailable
                                                    ? 'border-emerald-500/20 bg-emerald-500/10 text-emerald-600'
                                                    : 'border-destructive/20 bg-destructive/10 text-destructive',
                                            )}
                                        >
                                            {availabilitySummary.allAvailable
                                                ? t('toast.testAllPassed')
                                                : t('toast.testPartialFailed')}
                                        </span>
                                    ) : null}
                                    {availabilitySummary?.fullyMatched ? (
                                        <span className="text-destructive">
                                            {t('detail.availability.unavailableCount', {
                                                count: availabilitySummary.unavailableCount,
                                            })}
                                        </span>
                                    ) : null}
                                </div>
                                <div className="rounded-lg border border-border/20 bg-background/40">
                                    <MemberList
                                        members={editMembers}
                                        onReorder={() => undefined}
                                        onRemove={() => undefined}
                                        autoScrollOnAdd={false}
                                        showConfirmDelete={false}
                                        showWeight={
                                            (MODE_LABELS[group.mode] ? group.mode : GroupMode.Auto) ===
                                                GroupMode.Weighted ||
                                            (MODE_LABELS[group.mode] ? group.mode : GroupMode.Auto) ===
                                                GroupMode.Auto
                                        }
                                        availabilityById={availabilityByMemberId}
                                    />
                                </div>
                                {testResults.length > 0 ? (
                                    <div className="rounded-lg border border-border/20 bg-background/50 p-3">
                                        <div className="mb-2 text-xs font-medium text-foreground">
                                            {t('detail.availability.resultTitle')}
                                        </div>
                                        <div className="space-y-2">
                                            {testResults.map((result) => (
                                                <div
                                                    key={
                                                        result.client_id ||
                                                        `${result.item_id}-${result.channel_id}-${result.model_name}`
                                                    }
                                                    className={cn(
                                                        'rounded-md border px-3 py-2 text-xs',
                                                        result.passed
                                                            ? 'border-emerald-500/15 bg-emerald-500/5'
                                                            : 'border-destructive/15 bg-destructive/5',
                                                    )}
                                                >
                                                    <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                                                        <span className="font-medium text-foreground">
                                                            {result.model_name}
                                                        </span>
                                                        <span className="text-muted-foreground">
                                                            @ {result.channel_name}
                                                        </span>
                                                        <span
                                                            className={cn(
                                                                result.passed
                                                                    ? 'text-emerald-600'
                                                                    : 'text-destructive',
                                                            )}
                                                        >
                                                            {result.passed
                                                                ? t(
                                                                      'detail.availability.resultPassed',
                                                                  )
                                                                : t(
                                                                      'detail.availability.resultFailed',
                                                                  )}
                                                        </span>
                                                        {result.status_code > 0 ? (
                                                            <span className="text-muted-foreground">
                                                                HTTP {result.status_code}
                                                            </span>
                                                        ) : null}
                                                        <span className="text-muted-foreground">
                                                            {t(
                                                                'detail.availability.resultAttempts',
                                                                { count: result.attempts },
                                                            )}
                                                        </span>
                                                    </div>
                                                    {result.message ? (
                                                        <div className="mt-1 break-all text-muted-foreground">
                                                            {result.message}
                                                        </div>
                                                    ) : null}
                                                    {!result.passed && result.response_text ? (
                                                        <div className="mt-1 break-all text-muted-foreground/90">
                                                            {result.response_text}
                                                        </div>
                                                    ) : null}
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                ) : null}
                            </div>
                        ) : null}
                    </section>
                ) : null}

                <GroupEditor
                    key={`edit-group-${group.id}`}
                    className="flex-1 min-h-0"
                    initial={{
                        name: group.name,
                        endpoint_type: normalizeEndpointType(group.endpoint_type),
                        endpoint_provider: group.endpoint_provider ?? '',
                        match_regex: group.match_regex ?? '',
                        condition: group.condition ?? '',
                        mode: group.mode,
                        first_token_time_out: group.first_token_time_out ?? 0,
                        session_keep_time: group.session_keep_time ?? 0,
                        members: editMembers,
                    }}
                    submitText={t('detail.actions.save')}
                    submittingText={t('create.submitting')}
                    isSubmitting={isSubmitting}
                    onCancel={() => setIsOpen(false)}
                    onSubmit={(v) => onSubmit(v, () => setIsOpen(false))}
                />
            </MorphingDialogDescription>
        </div>
    );
}

// ---------------------------------------------------------------------------
// GroupListItem
// ---------------------------------------------------------------------------

export function GroupListItem({ group }: { group: Group }) {
    const t = useTranslations('group');
    const updateGroup = useUpdateGroup();
    const deleteGroup = useDeleteGroup();
    const testGroup = useTestGroup();
    const testDraftGroup = useTestDraftGroup();
    const { data: modelChannels = [] } = useModelChannelList();

    const [expanded, setExpanded] = useState(false);
    const [confirmDelete, setConfirmDelete] = useState(false);

    // ---- Channel maps ----
    const channelNameByKey = useMemo(
        () => buildChannelNameByModelKey(modelChannels),
        [modelChannels],
    );
    const enabledByKey = useMemo(() => {
        const map = new Map<string, boolean>();
        modelChannels.forEach((mc) => {
            map.set(modelChannelKey(mc.channel_id, mc.name), mc.enabled);
        });
        return map;
    }, [modelChannels]);

    // ---- Members ----
    const displayMembers = useMemo((): SelectedMember[] =>
        [...(group.items || [])]
            .sort((a, b) => a.priority - b.priority)
            .map((item) => ({
                id: modelChannelKey(item.channel_id, item.model_name),
                name: item.model_name,
                enabled:
                    enabledByKey.get(
                        modelChannelKey(item.channel_id, item.model_name),
                    ) ?? true,
                channel_id: item.channel_id,
                channel_name:
                    channelNameByKey.get(
                        modelChannelKey(item.channel_id, item.model_name),
                    ) ??
                    t('aiRoute.progress.channelFallbackName', {
                        id: item.channel_id,
                    }),
                item_id: item.id,
                weight: item.weight,
            })),
        [group.items, channelNameByKey, enabledByKey, t],
    );

    const [members, setMembers] = useState<SelectedMember[]>(displayMembers);
    const [isDragging, setIsDragging] = useState(false);
    const [currentTestId, setCurrentTestId] = useState<string | null>(null);
    const weightTimerRef = useRef<NodeJS.Timeout | null>(null);
    const membersRef = useRef<SelectedMember[]>([]);
    const lastDisplayMembersRef = useRef(displayMembers);
    const handledTestCompletionRef = useRef<string | null>(null);
    const testProgressQuery = useGroupTestProgress(currentTestId);
    const testProgress = testProgressQuery.data;

    useEffect(() => {
        membersRef.current = members;
    }, [members]);

    useEffect(() => {
        if (isDragging || lastDisplayMembersRef.current === displayMembers) return;
        lastDisplayMembersRef.current = displayMembers;
        const frameId = window.requestAnimationFrame(() => {
            setMembers(displayMembers);
        });
        return () => window.cancelAnimationFrame(frameId);
    }, [displayMembers, isDragging]);

    useEffect(() => {
        return () => {
            if (weightTimerRef.current) clearTimeout(weightTimerRef.current);
        };
    }, []);

    // ---- Toast helpers ----
    const onSuccess = useCallback(() => toast.success(t('toast.updated')), [t]);
    const onError = useCallback(
        (error: Error) =>
            toast.error(t('toast.updateFailed'), { description: error.message }),
        [t],
    );
    const getRemovableSuggestion = useCallback(
        (results: GroupTestResult[]) =>
            results
                .filter((r) => !r.passed)
                .map((r) => `${r.model_name} @ ${r.channel_name}`),
        [],
    );

    // ---- Test completion handling ----
    useEffect(() => {
        if (
            !testProgress?.done ||
            !testProgress.id ||
            handledTestCompletionRef.current === testProgress.id
        )
            return;

        handledTestCompletionRef.current = testProgress.id;

        if (testProgress.message) {
            toast.error(t('toast.testRequestFailed'), {
                description: testProgress.message,
            });
            return;
        }

        const failedResults = (testProgress.results ?? []).filter(
            (r) => !r.passed,
        );
        if (failedResults.length === 0) {
            toast.success(t('toast.testAllPassed'));
            return;
        }

        const removable = getRemovableSuggestion(failedResults);
        toast.warning(t('toast.testPartialFailed'), {
            description:
                removable.length > 0
                    ? `${t('toast.removableSuggestion')}: ${removable.join(', ')}`
                    : t('toast.testPartialFailedDescription'),
            duration: 8000,
        });
    }, [getRemovableSuggestion, t, testProgress]);

    // ---- Test handlers ----
    const handleTestDraftGroup = useCallback(() => {
        if (members.length === 0) return;
        setCurrentTestId(null);
        handledTestCompletionRef.current = null;
        testDraftGroup.mutate(
            {
                endpoint_type: normalizeEndpointType(group.endpoint_type),
                endpoint_provider: group.endpoint_provider ?? '',
                items: members.map((member) => ({
                    client_id: member.id,
                    channel_id: member.channel_id,
                    model_name: member.name,
                })),
            },
            {
                onSuccess: (progress) => setCurrentTestId(progress.id),
                onError: (error: Error) =>
                    toast.error(t('toast.testRequestFailed'), {
                        description: error.message,
                    }),
            },
        );
    }, [
        group.endpoint_provider,
        group.endpoint_type,
        members,
        t,
        testDraftGroup,
    ]);

    const handleTestGroup = useCallback(() => {
        if (!group.id) return;
        setCurrentTestId(null);
        handledTestCompletionRef.current = null;
        testGroup.mutate(group.id, {
            onSuccess: (progress) => setCurrentTestId(progress.id),
            onError: (error: Error) =>
                toast.error(t('toast.testRequestFailed'), {
                    description: error.message,
                }),
        });
    }, [group.id, t, testGroup]);

    // ---- Mode switching ----
    const isUpdatingMode = (() => {
        if (!updateGroup.isPending) return false;
        const v = updateGroup.variables;
        if (typeof v !== 'object' || v === null) return false;
        return (
            'mode' in v && typeof (v as { mode?: unknown }).mode === 'number'
        );
    })();

    const resolvedMode = MODE_LABELS[group.mode] ? group.mode : GroupMode.Auto;

    // ---- Priority map ----
    const priorityByItemId = useMemo(() => {
        const map = new Map<number, number>();
        (group.items || []).forEach((item) => {
            if (item.id !== undefined) map.set(item.id, item.priority);
        });
        return map;
    }, [group.items]);

    // ---- Drag handlers ----
    const handleDragStart = useCallback(() => setIsDragging(true), []);
    const handleDragFinish = useCallback(() => setIsDragging(false), []);

    const handleDropReorder = useCallback(
        (nextMembers: SelectedMember[]) => {
            const itemsToUpdate = nextMembers
                .map((m, i) => ({ member: m, newPriority: i + 1 }))
                .filter(({ member, newPriority }) => {
                    if (!member.item_id) return false;
                    const origPriority = priorityByItemId.get(member.item_id);
                    return (
                        origPriority !== undefined &&
                        origPriority !== newPriority
                    );
                })
                .map(({ member, newPriority }) => ({
                    id: member.item_id!,
                    priority: newPriority,
                    weight: member.weight ?? 1,
                }));
            if (itemsToUpdate.length > 0)
                updateGroup.mutate(
                    { id: group.id!, items_to_update: itemsToUpdate },
                    { onSuccess, onError },
                );
        },
        [group.id, priorityByItemId, updateGroup, onSuccess, onError],
    );

    // ---- Remove / Weight ----
    const handleRemoveMember = useCallback(
        (id: string) => {
            const member = members.find((m) => m.id === id);
            if (member?.item_id !== undefined)
                updateGroup.mutate(
                    { id: group.id!, items_to_delete: [member.item_id] },
                    { onSuccess, onError },
                );
        },
        [members, group.id, updateGroup, onSuccess, onError],
    );

    const handleWeightChange = useCallback(
        (id: string, weight: number) => {
            setMembers((prev) =>
                prev.map((m) => (m.id === id ? { ...m, weight } : m)),
            );
            if (weightTimerRef.current) clearTimeout(weightTimerRef.current);
            weightTimerRef.current = setTimeout(() => {
                const member = membersRef.current.find((m) => m.id === id);
                if (!member?.item_id) return;
                const priority = priorityByItemId.get(member.item_id);
                if (!priority) return;
                updateGroup.mutate(
                    {
                        id: group.id!,
                        items_to_update: [{ id: member.item_id, priority, weight }],
                    },
                    { onSuccess, onError },
                );
            }, 500);
        },
        [group.id, priorityByItemId, updateGroup, onSuccess, onError],
    );

    // ---- Edit submit ----
    const handleSubmitEdit = useCallback(
        (values: GroupEditorValues, onDone?: () => void) => {
            if (!group.id) return;

            const originalItems = [...(group.items || [])].sort(
                (a, b) => a.priority - b.priority,
            );
            const originalById = new Map<
                number,
                { priority: number; weight: number }
            >();
            const originalIds = new Set<number>();
            originalItems.forEach((it) => {
                if (typeof it.id === 'number') {
                    originalIds.add(it.id);
                    originalById.set(it.id, {
                        priority: it.priority,
                        weight: it.weight,
                    });
                }
            });

            const newIds = new Set<number>();
            values.members.forEach((m) => {
                if (typeof m.item_id === 'number') newIds.add(m.item_id);
            });

            const items_to_delete = Array.from(originalIds).filter(
                (id) => !newIds.has(id),
            );

            const items_to_add = values.members
                .map((m, idx) => ({ m, priority: idx + 1 }))
                .filter(({ m }) => typeof m.item_id !== 'number')
                .map(({ m, priority }) => ({
                    channel_id: m.channel_id,
                    model_name: m.name,
                    priority,
                    weight: m.weight ?? 1,
                }));

            const items_to_update = values.members
                .map((m, idx) => ({ m, priority: idx + 1 }))
                .filter(({ m }) => typeof m.item_id === 'number')
                .map(({ m, priority }) => {
                    const id = m.item_id!;
                    const orig = originalById.get(id);
                    const weight = m.weight ?? 1;
                    if (!orig) return null;
                    if (orig.priority === priority && orig.weight === weight)
                        return null;
                    return { id, priority, weight };
                })
                .filter(
                    (x): x is { id: number; priority: number; weight: number } =>
                        x !== null,
                );

            const payload: GroupUpdateRequest = { id: group.id };
            const nextName = values.name.trim();
            const nextEndpointType = normalizeEndpointType(
                values.endpoint_type,
            );
            const nextRegex = (values.match_regex ?? '').trim();
            const nextEndpointProvider = (
                values.endpoint_provider ?? ''
            )
                .trim()
                .toLowerCase();
            const nextCondition = values.condition.trim();
            const nextFirstTokenTimeOut =
                values.first_token_time_out ?? 0;
            const nextSessionKeepTime =
                values.session_keep_time ?? 0;

            if (nextName && nextName !== group.name)
                payload.name = nextName;
            if (
                nextEndpointType !==
                normalizeEndpointType(group.endpoint_type)
            )
                payload.endpoint_type = nextEndpointType;
            if (
                nextEndpointProvider !==
                (group.endpoint_provider ?? '').trim().toLowerCase()
            )
                payload.endpoint_provider = nextEndpointProvider;
            if (values.mode !== group.mode) payload.mode = values.mode;
            if (nextRegex !== (group.match_regex ?? ''))
                payload.match_regex = nextRegex;
            if (nextCondition !== (group.condition ?? ''))
                payload.condition = nextCondition;
            if (
                nextFirstTokenTimeOut !==
                (group.first_token_time_out ?? 0)
            )
                payload.first_token_time_out = nextFirstTokenTimeOut;
            if (
                nextSessionKeepTime !==
                (group.session_keep_time ?? 0)
            )
                payload.session_keep_time = nextSessionKeepTime;
            if (items_to_add.length) payload.items_to_add = items_to_add;
            if (items_to_update.length)
                payload.items_to_update = items_to_update;
            if (items_to_delete.length)
                payload.items_to_delete = items_to_delete;

            if (Object.keys(payload).length === 1) {
                onDone?.();
                return;
            }

            updateGroup.mutate(payload, {
                onSuccess: () => {
                    onSuccess();
                    onDone?.();
                },
                onError,
            });
        },
        [
            group.condition,
            group.endpoint_provider,
            group.endpoint_type,
            group.first_token_time_out,
            group.session_keep_time,
            group.id,
            group.items,
            group.match_regex,
            group.mode,
            group.name,
            onSuccess,
            onError,
            updateGroup,
        ],
    );

    // ---- Remove failed models ----
    const handleRemoveFailedMembers = useCallback(() => {
        if (!group.id) return;

        const failedResults = (testProgress?.results ?? []).filter(
            (r) => !r.passed,
        );
        const failedItemIds = new Set(
            failedResults
                .map((r) => r.item_id)
                .filter(
                    (itemId): itemId is number => typeof itemId === 'number',
                ),
        );
        const failedClientIds = new Set(
            failedResults
                .map((r) => r.client_id)
                .filter(
                    (clientId): clientId is string =>
                        typeof clientId === 'string' && clientId.length > 0,
                ),
        );
        const failedFallbackKeys = new Set(
            failedResults.map((r) =>
                modelChannelKey(r.channel_id, r.model_name),
            ),
        );

        if (
            failedItemIds.size === 0 &&
            failedClientIds.size === 0 &&
            failedFallbackKeys.size === 0
        )
            return;

        const nextMembers = members.filter((member) => {
            if (
                failedClientIds.has(member.id) ||
                failedFallbackKeys.has(member.id)
            )
                return false;
            if (typeof member.item_id !== 'number') return true;
            return !failedItemIds.has(member.item_id);
        });

        if (nextMembers.length === members.length) return;

        const values: GroupEditorValues = {
            name: group.name,
            endpoint_type: normalizeEndpointType(group.endpoint_type),
            endpoint_provider: group.endpoint_provider ?? '',
            match_regex: group.match_regex ?? '',
            condition: group.condition ?? '',
            mode: group.mode,
            first_token_time_out: group.first_token_time_out ?? 0,
            session_keep_time: group.session_keep_time ?? 0,
            members: nextMembers,
        };

        setMembers(nextMembers);
        handleSubmitEdit(values, () => {
            setCurrentTestId(null);
            handledTestCompletionRef.current = null;
            toast.success(t('toast.removedFailedModels'));
        });
    }, [
        group.condition,
        group.endpoint_provider,
        group.endpoint_type,
        group.first_token_time_out,
        group.id,
        group.match_regex,
        group.mode,
        group.name,
        group.session_keep_time,
        handleSubmitEdit,
        members,
        t,
        testProgress?.results,
    ]);

    // ---- Test progress derived state ----
    const failedTestResults = useMemo(
        () =>
            testProgress?.done
                ? (testProgress.results ?? []).filter((r) => !r.passed)
                : [],
        [testProgress],
    );

    const resultByItemId = useMemo(() => {
        const map = new Map<number, GroupTestResult>();
        (testProgress?.results ?? []).forEach((r) =>
            map.set(r.item_id, r),
        );
        return map;
    }, [testProgress]);

    const isTesting =
        testDraftGroup.isPending ||
        (!!currentTestId && !testProgress?.done);
    const completedCount = testProgress?.completed ?? 0;
    const totalCount = testProgress?.total ?? group.items?.length ?? 0;
    const progressValue =
        totalCount > 0 ? (completedCount / totalCount) * 100 : 0;

    const availabilityByMemberId = useMemo(() => {
        const map: Record<string, MemberAvailabilityMeta> = {};
        const activeResults = testProgress?.results ?? [];
        const resultByClientId = new Map<string, GroupTestResult>();
        const resultByFallbackKey = new Map<string, GroupTestResult>();

        activeResults.forEach((r) => {
            if (r.client_id) resultByClientId.set(r.client_id, r);
            resultByFallbackKey.set(
                modelChannelKey(r.channel_id, r.model_name),
                r,
            );
        });

        members.forEach((member) => {
            const matched =
                resultByClientId.get(member.id) ??
                resultByFallbackKey.get(member.id);
            if (matched) {
                map[member.id] = {
                    status: matched.passed ? 'available' : 'unavailable',
                    message:
                        matched.message || matched.response_text || undefined,
                };
                return;
            }
            map[member.id] = { status: isTesting ? 'testing' : 'idle' };
        });

        return map;
    }, [isTesting, members, testProgress?.results]);

    const availabilitySummary = useMemo(() => {
        if (!testProgress?.done) return undefined;
        const results = testProgress.results ?? [];
        const resultKeys = new Set(
            results.map((r) =>
                modelChannelKey(r.channel_id, r.model_name),
            ),
        );
        const savedMembers = members.filter(
            (m) => typeof m.item_id === 'number',
        );
        const fullyMatched =
            savedMembers.length > 0 &&
            savedMembers.every((m) => resultKeys.has(m.id));
        return {
            unavailableCount: results.filter((r) => !r.passed).length,
            availableCount: results.filter((r) => r.passed).length,
            allAvailable:
                results.length > 0 && results.every((r) => r.passed),
            fullyMatched,
        };
    }, [members, testProgress]);

    const canRemoveFailedModels = Boolean(
        availabilitySummary?.fullyMatched &&
            availabilitySummary.unavailableCount > 0 &&
            !isTesting &&
            !updateGroup.isPending,
    );

    // ---- Derived display values for collapsed state ----
    const firstModelName =
        displayMembers.length > 0 ? displayMembers[0].name : '';
    const memberCount = displayMembers.length;
    const modeLabel = t(`mode.${MODE_LABELS[resolvedMode]}`);

    const FirstAvatar = useMemo(() => {
        if (!firstModelName) return null;
        const { Avatar } = getModelIcon(firstModelName);
        return Avatar;
    }, [firstModelName]);

    // ---- Status indicator logic ----
    const testDone = testProgress?.done;
    const totalResults = testProgress?.results ?? [];
    const passedCount = totalResults.filter((r) => r.passed).length;
    const failCount = totalResults.length - passedCount;

    // ---- Member list adaptive height ----
    const needsScroll = members.length >= 4;

    return (
        <article className="group relative overflow-hidden rounded-xl border border-border bg-card text-card-foreground transition-shadow hover:shadow-sm">
            {/* ===== Collapsed row (always visible) ===== */}
            <div
                className="flex cursor-pointer items-center gap-3 px-4 py-3 md:py-4"
                onClick={() => setExpanded((prev) => !prev)}
            >
                {/* Left: icon + text */}
                <div className="flex min-w-0 flex-1 items-center gap-3">
                    {/* Model icon */}
                    <span className="flex size-9 shrink-0 items-center justify-center rounded-full border border-border/40 bg-muted/30">
                        {FirstAvatar ? (
                            <FirstAvatar size={18} />
                        ) : (
                            <Waves className="size-4 text-muted-foreground" />
                        )}
                    </span>

                    {/* Text block */}
                    <div className="min-w-0 flex-1">
                        <h3 className="truncate text-sm font-semibold text-card-foreground md:text-base">
                            {group.name}
                        </h3>
                        <p className="line-clamp-2 text-xs text-muted-foreground">
                            {modeLabel}
                            {firstModelName ? ` · ${firstModelName}` : ''}
                            {' · '}
                            {t('card.modelCount', { count: memberCount })}
                        </p>
                    </div>
                </div>

                {/* Right: status + chevron */}
                <div className="flex shrink-0 items-center gap-2">
                    {/* Status indicator */}
                    <div
                        onClick={(e) => e.stopPropagation()}
                        className="flex items-center"
                    >
                        {!testDone && !isTesting && (
                            <Tooltip side="top" sideOffset={8} align="center">
                                <TooltipTrigger asChild>
                                    <button
                                        type="button"
                                        onClick={handleTestGroup}
                                        disabled={!group.id}
                                        className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
                                    >
                                        <Activity className="size-4" />
                                    </button>
                                </TooltipTrigger>
                                <TooltipContent>
                                    {t('detail.actions.testAvailability')}
                                </TooltipContent>
                            </Tooltip>
                        )}
                        {isTesting && (
                            <Loader2 className="size-4 animate-spin text-muted-foreground" />
                        )}
                        {testDone && !isTesting && totalResults.length > 0 && failCount === 0 && (
                            <CircleCheck className="size-4 text-emerald-500" />
                        )}
                        {testDone &&
                            !isTesting &&
                            totalResults.length > 0 &&
                            failCount > 0 &&
                            passedCount > 0 && (
                                <Tooltip
                                    side="top"
                                    sideOffset={8}
                                    align="center"
                                >
                                    <TooltipTrigger asChild>
                                        <span className="inline-flex items-center gap-1">
                                            <AlertTriangle className="size-4 text-amber-500" />
                                            <span className="text-[10px] font-semibold text-amber-600 dark:text-amber-400">
                                                {failCount}
                                            </span>
                                        </span>
                                    </TooltipTrigger>
                                    <TooltipContent>
                                        {t('toast.testPartialFailed')}
                                    </TooltipContent>
                                </Tooltip>
                            )}
                        {testDone &&
                            !isTesting &&
                            totalResults.length > 0 &&
                            failCount > 0 &&
                            passedCount === 0 && (
                                <CircleX className="size-4 text-destructive" />
                            )}
                    </div>

                    {/* Chevron */}
                    <motion.div
                        animate={{ rotate: expanded ? 180 : 0 }}
                        transition={{ duration: 0.2, ease: 'easeInOut' }}
                    >
                        <ChevronDown className="size-4 text-muted-foreground" />
                    </motion.div>
                </div>
            </div>

            {/* ===== Expanded content ===== */}
            <AnimatePresence initial={false}>
                {expanded && (
                    <motion.div
                        key="expanded-content"
                        initial={{ height: 0, opacity: 0 }}
                        animate={{ height: 'auto', opacity: 1 }}
                        exit={{ height: 0, opacity: 0 }}
                        transition={{
                            height: { duration: 0.25, ease: 'easeInOut' },
                            opacity: { duration: 0.2 },
                        }}
                        className="overflow-hidden"
                    >
                        <div className="space-y-3 border-t border-border/40 px-4 pb-4 pt-3">
                            {/* --- Mode switcher --- */}
                            <div className="grid grid-cols-3 gap-1.5 sm:grid-cols-5">
                                {(
                                    [
                                        GroupMode.RoundRobin,
                                        GroupMode.Random,
                                        GroupMode.Failover,
                                        GroupMode.Weighted,
                                        GroupMode.Auto,
                                    ] as const
                                ).map((m) => (
                                    <button
                                        key={m}
                                        type="button"
                                        aria-disabled={
                                            isUpdatingMode || !group.id
                                        }
                                        onClick={() => {
                                            if (isUpdatingMode || !group.id)
                                                return;
                                            if (m === group.mode) return;
                                            updateGroup.mutate(
                                                { id: group.id!, mode: m },
                                                { onSuccess, onError },
                                            );
                                        }}
                                        className={cn(
                                            'rounded-md px-2 py-1.5 text-[10px] font-medium transition-[transform,border-color,background-color] duration-300 md:px-3 md:py-2 md:text-xs',
                                            resolvedMode === m
                                                ? 'border border-primary/20 bg-primary text-primary-foreground'
                                                : 'border border-border/25 bg-card text-foreground hover:-translate-y-0.5 hover:border-primary/16 hover:bg-card',
                                            !group.id &&
                                                'cursor-not-allowed opacity-50',
                                        )}
                                    >
                                        {t(`mode.${MODE_LABELS[m]}`)}
                                    </button>
                                ))}
                            </div>

                            {/* --- Member list --- */}
                            <section
                                className={cn(
                                    'relative overflow-hidden rounded-lg border border-border/25 bg-card transition-[height] duration-200',
                                    !needsScroll && 'h-auto',
                                    members.length === 0 && !needsScroll && 'min-h-[5rem]',
                                )}
                                style={
                                    needsScroll
                                        ? { maxHeight: '248px' }
                                        : undefined
                                }
                            >
                                <div
                                    className={cn(
                                        needsScroll &&
                                            'max-h-[248px] overflow-y-auto',
                                    )}
                                >
                                    <MemberList
                                        members={members}
                                        onReorder={setMembers}
                                        onRemove={handleRemoveMember}
                                        onWeightChange={handleWeightChange}
                                        onDragStart={handleDragStart}
                                        onDrop={handleDropReorder}
                                        onDragFinish={handleDragFinish}
                                        autoScrollOnAdd={false}
                                        showWeight={
                                            resolvedMode ===
                                                GroupMode.Weighted ||
                                            resolvedMode === GroupMode.Auto
                                        }
                                        layoutScope={`list-item-${group.id ?? 'unknown'}`}
                                    />
                                </div>
                            </section>

                            {/* --- Bottom action bar --- */}
                            <div className="flex items-center gap-1 border-t border-border/25 pt-2">
                                {/* Edit */}
                                <MorphingDialog>
                                    <MorphingDialogTrigger className="rounded-md p-2 text-muted-foreground transition-colors hover:bg-card hover:text-foreground">
                                        <Tooltip
                                            side="top"
                                            sideOffset={10}
                                            align="center"
                                        >
                                            <TooltipTrigger asChild>
                                                <Pencil className="size-4" />
                                            </TooltipTrigger>
                                            <TooltipContent>
                                                {t('detail.actions.edit')}
                                            </TooltipContent>
                                        </Tooltip>
                                    </MorphingDialogTrigger>

                                    <MorphingDialogContainer>
                                        <MorphingDialogContent className="max-h-[calc(100dvh-1rem)] sm:max-h-[calc(100dvh-3rem)] max-w-full sm:max-w-full relative flex h-[calc(100dvh-1rem)] w-[min(100vw-1rem,92rem)] flex-col overflow-hidden rounded-xl border border-border/35 bg-card px-4 py-4 text-card-foreground shadow-md md:h-[calc(100dvh-3rem)] md:w-[min(100vw-2rem,92rem)] md:px-6">
                                            <EditDialogContent
                                                group={group}
                                                editMembers={members}
                                                isSubmitting={
                                                    updateGroup.isPending
                                                }
                                                onSubmit={handleSubmitEdit}
                                                onTestAvailability={
                                                    handleTestDraftGroup
                                                }
                                                onRemoveFailedModels={
                                                    handleRemoveFailedMembers
                                                }
                                                isTestingAvailability={
                                                    isTesting
                                                }
                                                canRemoveFailedModels={
                                                    canRemoveFailedModels
                                                }
                                                testProgressCompleted={
                                                    completedCount
                                                }
                                                testProgressTotal={totalCount}
                                                testProgressValue={
                                                    progressValue
                                                }
                                                testResults={
                                                    testProgress?.results ?? []
                                                }
                                                availabilityByMemberId={
                                                    availabilityByMemberId
                                                }
                                                availabilitySummary={
                                                    availabilitySummary
                                                }
                                            />
                                        </MorphingDialogContent>
                                    </MorphingDialogContainer>
                                </MorphingDialog>

                                {/* Copy name */}
                                <Tooltip
                                    side="top"
                                    sideOffset={10}
                                    align="center"
                                >
                                    <TooltipTrigger>
                                        <CopyIconButton
                                            text={group.name}
                                            className="rounded-md p-2 text-muted-foreground transition-colors hover:bg-card hover:text-foreground"
                                            copyIconClassName="size-4"
                                            checkIconClassName="size-4 text-primary"
                                        />
                                    </TooltipTrigger>
                                    <TooltipContent>
                                        {t('detail.actions.copyName')}
                                    </TooltipContent>
                                </Tooltip>

                                {/* Delete */}
                                <div className="relative ml-auto">
                                    {!confirmDelete && (
                                        <Tooltip
                                            side="top"
                                            sideOffset={10}
                                            align="center"
                                        >
                                            <TooltipTrigger>
                                                <motion.button
                                                    layoutId={`delete-btn-list-group-${group.id}`}
                                                    type="button"
                                                    onClick={() =>
                                                        setConfirmDelete(true)
                                                    }
                                                    className="rounded-md p-2 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
                                                >
                                                    <Trash2 className="size-4" />
                                                </motion.button>
                                            </TooltipTrigger>
                                            <TooltipContent>
                                                {t('detail.actions.delete')}
                                            </TooltipContent>
                                        </Tooltip>
                                    )}

                                    <AnimatePresence>
                                        {confirmDelete && (
                                            <motion.div
                                                layoutId={`delete-btn-list-group-${group.id}`}
                                                className="absolute inset-y-0 right-0 flex items-center gap-2 rounded-lg bg-destructive px-2"
                                                transition={{
                                                    type: 'spring',
                                                    stiffness: 400,
                                                    damping: 30,
                                                }}
                                            >
                                                <button
                                                    type="button"
                                                    onClick={() =>
                                                        setConfirmDelete(false)
                                                    }
                                                    className="flex h-7 w-7 items-center justify-center rounded-lg bg-destructive-foreground/20 text-destructive-foreground transition-all hover:bg-destructive-foreground/30 active:scale-95"
                                                >
                                                    <X className="size-4" />
                                                </button>
                                                <button
                                                    type="button"
                                                    onClick={() =>
                                                        group.id &&
                                                        deleteGroup.mutate(
                                                            group.id,
                                                            {
                                                                onSuccess: () =>
                                                                    toast.success(
                                                                        t(
                                                                            'toast.deleted',
                                                                        ),
                                                                    ),
                                                            },
                                                        )
                                                    }
                                                    disabled={
                                                        deleteGroup.isPending
                                                    }
                                                    className="flex h-7 items-center gap-2 rounded-lg bg-destructive-foreground px-3 text-sm font-semibold text-destructive transition-all hover:bg-destructive-foreground/90 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50"
                                                >
                                                    <Trash2 className="size-3.5" />
                                                    {t(
                                                        'detail.actions.confirmDelete',
                                                    )}
                                                </button>
                                            </motion.div>
                                        )}
                                    </AnimatePresence>
                                </div>
                            </div>
                        </div>
                    </motion.div>
                )}
            </AnimatePresence>
        </article>
    );
}
