'use client';

import { useState, useMemo, useCallback, useEffect, useRef } from 'react';
import { Trash2, X, Pencil, Activity, Loader2, CircleCheck, CircleX, Clock3, Layers, Waves, Orbit, TestTubeDiagonal } from 'lucide-react';
import { motion, AnimatePresence } from 'motion/react';
import { type Group, useDeleteGroup, useUpdateGroup, useTestGroup, useTestDraftGroup, useGroupTestProgress, type GroupTestResult } from '@/api/endpoints/group';
import { useModelChannelList } from '@/api/endpoints/model';
import { useTranslations } from 'next-intl';
import { cn } from '@/lib/utils';
import { toast } from '@/components/common/Toast';
import { CopyIconButton } from '@/components/common/CopyButton';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/animate-ui/components/animate/tooltip';
import type { MemberAvailabilityMeta, SelectedMember } from './ItemList';
import { MemberList } from './ItemList';
import { GroupEditor, type GroupEditorValues } from './Editor';
import { AIRouteButton } from './AIRouteButton';
import { buildChannelNameByModelKey, modelChannelKey, MODE_LABELS, inferGroupCapabilities, CAPABILITY_LABEL_KEYS, CAPABILITY_COLORS, endpointTypeLabelKey, normalizeEndpointType, supportsGroupTest } from './utils';
import { GroupMode, type GroupUpdateRequest } from '@/api/endpoints/group';
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
        <>
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
            <MorphingDialogDescription className="flex-1 min-h-0 overflow-y-auto">
                <div className="space-y-4 pr-1">
                    {group.id ? (
                        <section className="rounded-lg border border-border/25 bg-card p-4">
                            <div className="flex flex-wrap items-start justify-between gap-3">
                                <div className="space-y-1">
                                    <h3 className="text-sm font-semibold text-foreground">{t('detail.availability.title')}</h3>
                                    <p className="text-xs text-muted-foreground">{t('detail.availability.description')}</p>
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
                                                : 'border-primary/20 bg-primary text-primary-foreground hover:opacity-90'
                                        )}
                                    >
                                        {isTestingAvailability ? <Loader2 className="size-4 animate-spin" /> : <TestTubeDiagonal className="size-4" />}
                                        {t('detail.availability.testAll')}
                                    </button>
                                </div>
                            </div>
                            {(isTestingAvailability || availabilitySummary) ? (
                                <div className="mt-4 space-y-3">
                                    <Progress value={testProgressValue} className="h-2" />
                                    <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                                        <span>{t('card.testProgressCount', { completed: testProgressCompleted, total: testProgressTotal || editMembers.length })}</span>
                                        {availabilitySummary?.fullyMatched ? (
                                            <span className={cn(
                                                'inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-medium',
                                                availabilitySummary.allAvailable
                                                    ? 'border-emerald-500/20 bg-emerald-500/10 text-emerald-600'
                                                    : 'border-destructive/20 bg-destructive/10 text-destructive'
                                            )}>
                                                {availabilitySummary.allAvailable ? t('toast.testAllPassed') : t('toast.testPartialFailed')}
                                            </span>
                                        ) : null}
                                        {availabilitySummary?.fullyMatched ? (
                                            <span className="text-destructive">
                                                {t('detail.availability.unavailableCount', { count: availabilitySummary.unavailableCount })}
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
                                            showWeight={(MODE_LABELS[group.mode] ? group.mode : GroupMode.Auto) === GroupMode.Weighted || (MODE_LABELS[group.mode] ? group.mode : GroupMode.Auto) === GroupMode.Auto}
                                            availabilityById={availabilityByMemberId}
                                        />
                                    </div>
                                    {testResults.length > 0 ? (
                                        <div className="rounded-lg border border-border/20 bg-background/50 p-3">
                                            <div className="mb-2 text-xs font-medium text-foreground">{t('detail.availability.resultTitle')}</div>
                                            <div className="space-y-2">
                                                {testResults.map((result) => (
                                                    <div
                                                        key={result.client_id || `${result.item_id}-${result.channel_id}-${result.model_name}`}
                                                        className={cn(
                                                            'rounded-md border px-3 py-2 text-xs',
                                                            result.passed
                                                                ? 'border-emerald-500/15 bg-emerald-500/5'
                                                                : 'border-destructive/15 bg-destructive/5'
                                                        )}
                                                    >
                                                        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                                                            <span className="font-medium text-foreground">{result.model_name}</span>
                                                            <span className="text-muted-foreground">@ {result.channel_name}</span>
                                                            <span className={cn(result.passed ? 'text-emerald-600' : 'text-destructive')}>
                                                                {result.passed ? t('detail.availability.resultPassed') : t('detail.availability.resultFailed')}
                                                            </span>
                                                            {result.status_code > 0 ? (
                                                                <span className="text-muted-foreground">HTTP {result.status_code}</span>
                                                            ) : null}
                                                            <span className="text-muted-foreground">{t('detail.availability.resultAttempts', { count: result.attempts })}</span>
                                                        </div>
                                                        {result.message ? (
                                                            <div className="mt-1 break-all text-muted-foreground">{result.message}</div>
                                                        ) : null}
                                                        {!result.passed && result.response_text ? (
                                                            <div className="mt-1 break-all text-muted-foreground/90">{result.response_text}</div>
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
                </div>
            </MorphingDialogDescription>
        </>
    );
}

export function GroupCard({ group }: { group: Group }) {
    const t = useTranslations('group');
    const updateGroup = useUpdateGroup();
    const deleteGroup = useDeleteGroup();
    const testGroup = useTestGroup();
    const testDraftGroup = useTestDraftGroup();
    const { data: modelChannels = [] } = useModelChannelList();

    const channelNameByKey = useMemo(() => buildChannelNameByModelKey(modelChannels), [modelChannels]);
    const enabledByKey = useMemo(() => {
        const map = new Map<string, boolean>();
        modelChannels.forEach((mc) => {
            map.set(modelChannelKey(mc.channel_id, mc.name), mc.enabled);
        });
        return map;
    }, [modelChannels]);

    const displayMembers = useMemo((): SelectedMember[] =>
        [...(group.items || [])]
            .sort((a, b) => a.priority - b.priority)
            .map((item) => ({
                id: modelChannelKey(item.channel_id, item.model_name),
                name: item.model_name,
                enabled: enabledByKey.get(modelChannelKey(item.channel_id, item.model_name)) ?? true,
                channel_id: item.channel_id,
                channel_name: channelNameByKey.get(modelChannelKey(item.channel_id, item.model_name)) ?? t('aiRoute.progress.channelFallbackName', { id: item.channel_id }),
                item_id: item.id,
                weight: item.weight,
            })),
        [group.items, channelNameByKey, enabledByKey, t]
    );

    const [confirmDelete, setConfirmDelete] = useState(false);
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
        if (isDragging || lastDisplayMembersRef.current === displayMembers) {
            return;
        }

        lastDisplayMembersRef.current = displayMembers;
        const frameId = window.requestAnimationFrame(() => {
            setMembers(displayMembers);
        });

        return () => window.cancelAnimationFrame(frameId);
    }, [displayMembers, isDragging]);

    useEffect(() => {
        return () => { if (weightTimerRef.current) clearTimeout(weightTimerRef.current); };
    }, []);

    const onSuccess = useCallback(() => toast.success(t('toast.updated')), [t]);
    const onError = useCallback((error: Error) => toast.error(t('toast.updateFailed'), { description: error.message }), [t]);
    const getRemovableSuggestion = useCallback((results: GroupTestResult[]) => results
        .filter((result) => !result.passed)
        .map((result) => `${result.model_name} @ ${result.channel_name}`), []);

    useEffect(() => {
        if (!testProgress?.done || !testProgress.id || handledTestCompletionRef.current === testProgress.id) {
            return;
        }

        handledTestCompletionRef.current = testProgress.id;

        if (testProgress.message) {
            toast.error(t('toast.testRequestFailed'), { description: testProgress.message });
            return;
        }

        const failedResults = (testProgress.results ?? []).filter((result) => !result.passed);
        if (failedResults.length === 0) {
            toast.success(t('toast.testAllPassed'));
            return;
        }

        const removable = getRemovableSuggestion(failedResults);
        toast.warning(t('toast.testPartialFailed'), {
            description: removable.length > 0
                ? `${t('toast.removableSuggestion')}: ${removable.join(', ')}`
                : t('toast.testPartialFailedDescription'),
            duration: 8000,
        });
    }, [getRemovableSuggestion, t, testProgress]);

    const handleTestDraftGroup = useCallback(() => {
        if (members.length === 0) return;
        setCurrentTestId(null);
        handledTestCompletionRef.current = null;
        testDraftGroup.mutate({
            endpoint_type: normalizeEndpointType(group.endpoint_type),
            endpoint_provider: group.endpoint_provider ?? '',
            items: members.map((member) => ({
                client_id: member.id,
                channel_id: member.channel_id,
                model_name: member.name,
            })),
        }, {
            onSuccess: (progress) => {
                setCurrentTestId(progress.id);
            },
            onError: (error: Error) => {
                toast.error(t('toast.testRequestFailed'), { description: error.message });
            },
        });
    }, [group.endpoint_provider, group.endpoint_type, members, t, testDraftGroup]);

    const handleTestGroup = useCallback(() => {
        if (!group.id) return;
        setCurrentTestId(null);
        handledTestCompletionRef.current = null;
        testGroup.mutate(group.id, {
            onSuccess: (progress) => {
                setCurrentTestId(progress.id);
            },
            onError: (error: Error) => {
                toast.error(t('toast.testRequestFailed'), { description: error.message });
            },
        });
    }, [group.id, t, testGroup]);

    // Avoid UI flicker: drag-reorder also uses the same mutation, so only "mode switch" should lock mode buttons.
    const isUpdatingMode = (() => {
        if (!updateGroup.isPending) return false;
        const v = updateGroup.variables;
        if (typeof v !== 'object' || v === null) return false;
        return 'mode' in v && typeof (v as { mode?: unknown }).mode === 'number';
    })();

    const priorityByItemId = useMemo(() => {
        const map = new Map<number, number>();
        (group.items || []).forEach((item) => {
            if (item.id !== undefined) map.set(item.id, item.priority);
        });
        return map;
    }, [group.items]);

    const handleDragStart = useCallback(() => { setIsDragging(true); }, []);
    const handleDragFinish = useCallback(() => { setIsDragging(false); }, []);

    const handleDropReorder = useCallback((nextMembers: SelectedMember[]) => {
        const itemsToUpdate = nextMembers
            .map((m, i) => ({ member: m, newPriority: i + 1 }))
            .filter(({ member, newPriority }) => {
                if (!member.item_id) return false;
                const origPriority = priorityByItemId.get(member.item_id);
                return origPriority !== undefined && origPriority !== newPriority;
            })
            .map(({ member, newPriority }) => ({ id: member.item_id!, priority: newPriority, weight: member.weight ?? 1 }));
        if (itemsToUpdate.length > 0) updateGroup.mutate({ id: group.id!, items_to_update: itemsToUpdate }, { onSuccess, onError });
    }, [group.id, priorityByItemId, updateGroup, onSuccess, onError]);

    const handleRemoveMember = useCallback((id: string) => {
        const member = members.find((m) => m.id === id);
        if (member?.item_id !== undefined) updateGroup.mutate({ id: group.id!, items_to_delete: [member.item_id] }, { onSuccess, onError });
    }, [members, group.id, updateGroup, onSuccess, onError]);

    const handleWeightChange = useCallback((id: string, weight: number) => {
        setMembers((prev) => prev.map((m) => m.id === id ? { ...m, weight } : m));
        if (weightTimerRef.current) clearTimeout(weightTimerRef.current);
        weightTimerRef.current = setTimeout(() => {
            const member = membersRef.current.find((m) => m.id === id);
            if (!member?.item_id) return;
            const priority = priorityByItemId.get(member.item_id);
            if (!priority) return;
            updateGroup.mutate(
                { id: group.id!, items_to_update: [{ id: member.item_id, priority, weight }] },
                { onSuccess, onError }
            );
        }, 500);
    }, [group.id, priorityByItemId, updateGroup, onSuccess, onError]);

    const handleSubmitEdit = useCallback((values: GroupEditorValues, onDone?: () => void) => {
        if (!group.id) return;

        const originalItems = [...(group.items || [])].sort((a, b) => a.priority - b.priority);
        const originalById = new Map<number, { priority: number; weight: number }>();
        const originalIds = new Set<number>();
        originalItems.forEach((it) => {
            if (typeof it.id === 'number') {
                originalIds.add(it.id);
                originalById.set(it.id, { priority: it.priority, weight: it.weight });
            }
        });

        const newIds = new Set<number>();
        values.members.forEach((m) => { if (typeof m.item_id === 'number') newIds.add(m.item_id); });

        const items_to_delete = Array.from(originalIds).filter((id) => !newIds.has(id));

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
                if (orig.priority === priority && orig.weight === weight) return null;
                return { id, priority, weight };
            })
            .filter((x): x is { id: number; priority: number; weight: number } => x !== null);

        const payload: GroupUpdateRequest = { id: group.id };
        const nextName = values.name.trim();
        const nextEndpointType = normalizeEndpointType(values.endpoint_type);
        const nextRegex = (values.match_regex ?? '').trim();
        const nextEndpointProvider = (values.endpoint_provider ?? '').trim().toLowerCase();
        const nextCondition = values.condition.trim();
        const nextFirstTokenTimeOut = values.first_token_time_out ?? 0;
        const nextSessionKeepTime = values.session_keep_time ?? 0;

        if (nextName && nextName !== group.name) payload.name = nextName;
        if (nextEndpointType !== normalizeEndpointType(group.endpoint_type)) payload.endpoint_type = nextEndpointType;
        if (nextEndpointProvider !== ((group.endpoint_provider ?? '').trim().toLowerCase())) payload.endpoint_provider = nextEndpointProvider;
        if (values.mode !== group.mode) payload.mode = values.mode;
        if (nextRegex !== (group.match_regex ?? '')) payload.match_regex = nextRegex;
        if (nextCondition !== (group.condition ?? '')) payload.condition = nextCondition;
        if (nextFirstTokenTimeOut !== (group.first_token_time_out ?? 0)) payload.first_token_time_out = nextFirstTokenTimeOut;
        if (nextSessionKeepTime !== (group.session_keep_time ?? 0)) payload.session_keep_time = nextSessionKeepTime;
        if (items_to_add.length) payload.items_to_add = items_to_add;
        if (items_to_update.length) payload.items_to_update = items_to_update;
        if (items_to_delete.length) payload.items_to_delete = items_to_delete;

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
    }, [group.condition, group.endpoint_provider, group.endpoint_type, group.first_token_time_out, group.session_keep_time, group.id, group.items, group.match_regex, group.mode, group.name, onSuccess, onError, updateGroup]);

    const resolvedMode = MODE_LABELS[group.mode] ? group.mode : GroupMode.Auto;

    const handleRemoveFailedMembers = useCallback(() => {
        if (!group.id) return;

        const failedResults = (testProgress?.results ?? []).filter((result) => !result.passed);
        const failedItemIds = new Set(
            failedResults
                .map((result) => result.item_id)
                .filter((itemId): itemId is number => typeof itemId === 'number')
        );
        const failedClientIds = new Set(
            failedResults
                .map((result) => result.client_id)
                .filter((clientId): clientId is string => typeof clientId === 'string' && clientId.length > 0)
        );
        const failedFallbackKeys = new Set(
            failedResults.map((result) => modelChannelKey(result.channel_id, result.model_name))
        );

        if (failedItemIds.size === 0 && failedClientIds.size === 0 && failedFallbackKeys.size === 0) {
            return;
        }

        const nextMembers = members.filter((member) => {
            if (failedClientIds.has(member.id) || failedFallbackKeys.has(member.id)) {
                return false;
            }
            if (typeof member.item_id !== 'number') {
                return true;
            }
            return !failedItemIds.has(member.item_id);
        });

        if (nextMembers.length === members.length) {
            return;
        }

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
    }, [group.condition, group.endpoint_provider, group.endpoint_type, group.first_token_time_out, group.id, group.match_regex, group.mode, group.name, group.session_keep_time, handleSubmitEdit, members, t, testProgress?.results]);

    const failedTestResults = useMemo(
        () => (testProgress?.done ? (testProgress.results ?? []).filter((result) => !result.passed) : []),
        [testProgress]
    );

    const resultByItemId = useMemo(() => {
        const map = new Map<number, GroupTestResult>();
        const activeResults = testProgress?.results ?? [];
        activeResults.forEach((result) => {
            map.set(result.item_id, result);
        });
        return map;
    }, [testProgress]);

    const isTesting = testDraftGroup.isPending || (!!currentTestId && !testProgress?.done);
    const completedCount = testProgress?.completed ?? 0;
    const totalCount = testProgress?.total ?? group.items?.length ?? 0;
    const progressValue = totalCount > 0 ? (completedCount / totalCount) * 100 : 0;
    const availabilityByMemberId = useMemo(() => {
        const map: Record<string, MemberAvailabilityMeta> = {};
        const activeResults = testProgress?.results ?? [];
        const resultByClientId = new Map<string, GroupTestResult>();
        const resultByFallbackKey = new Map<string, GroupTestResult>();

        activeResults.forEach((result) => {
            if (result.client_id) {
                resultByClientId.set(result.client_id, result);
            }
            resultByFallbackKey.set(modelChannelKey(result.channel_id, result.model_name), result);
        });

        members.forEach((member) => {
            const matched = resultByClientId.get(member.id) ?? resultByFallbackKey.get(member.id);
            if (matched) {
                map[member.id] = {
                    status: matched.passed ? 'available' : 'unavailable',
                    message: matched.message || matched.response_text || undefined,
                };
                return;
            }
            map[member.id] = { status: isTesting ? 'testing' : 'idle' };
        });

        return map;
    }, [isTesting, members, testProgress?.results]);


    const availabilitySummary = useMemo(() => {
        if (!testProgress?.done) {
            return undefined;
        }
        const results = testProgress.results ?? [];
        const resultKeys = new Set(results.map((result) => modelChannelKey(result.channel_id, result.model_name)));
        const savedMembers = members.filter((member) => typeof member.item_id === 'number');
        const fullyMatched = savedMembers.length > 0 && savedMembers.every((member) => resultKeys.has(member.id));
        return {
            unavailableCount: results.filter((result) => !result.passed).length,
            availableCount: results.filter((result) => result.passed).length,
            allAvailable: results.length > 0 && results.every((result) => result.passed),
            fullyMatched,
        };
    }, [members, testProgress]);

    const canRemoveFailedModels = Boolean(availabilitySummary?.fullyMatched && availabilitySummary.unavailableCount > 0 && !isTesting && !updateGroup.isPending);

    const maxVisibleMembers = 6;
    const memberRowHeightRem = 3.25;
    const memberRowGapRem = 0.5;
    const memberListPaddingRem = 1.25;
    const memberListEmptyHeightRem = 10;
    const visibleMemberCount = Math.min(Math.max(members.length, 1), maxVisibleMembers);
    // Keep the card compact for short groups and turn the list into an inner scroller after 6 rows.
    const memberListHeightRem = members.length === 0
        ? memberListEmptyHeightRem
        : memberListPaddingRem + (visibleMemberCount * memberRowHeightRem) + ((visibleMemberCount - 1) * memberRowGapRem);

    return (
        <article className="group relative flex flex-col rounded-xl border border-border bg-card p-4 text-card-foreground">
            <header className="relative mb-4 rounded-lg border border-border bg-card px-4 py-4">
                <div className="flex items-start justify-between gap-3">
                <div className="relative mr-2 min-w-0 flex-1 group/title">
                    <div className="mb-2 inline-flex items-center gap-2 rounded-full border border-primary/10 bg-card px-2.5 py-1 text-[0.64rem] font-semibold text-primary">
                        <Orbit className="size-3.5" />
                        {t('card.endpointType', {
                            value: t(endpointTypeLabelKey(group.endpoint_type) ?? 'form.endpointType.options.all'),
                        })}
                    </div>
                    <Tooltip side="top" sideOffset={10} align="center">
                        <TooltipTrigger asChild>
                            <h3 className="truncate text-xl font-bold tracking-tight">{group.name}</h3>
                        </TooltipTrigger>
                        <TooltipContent key={group.name}>{group.name}</TooltipContent>
                    </Tooltip>
                </div>

                <div className="flex shrink-0 items-center gap-1">
                    <MorphingDialog>
                        <MorphingDialogTrigger className="rounded-md p-2 text-muted-foreground transition-colors hover:bg-card hover:text-foreground">
                            <Tooltip side="top" sideOffset={10} align="center">
                                <TooltipTrigger asChild>
                                    <Pencil className="size-4" />
                                </TooltipTrigger>
                                <TooltipContent>{t('detail.actions.edit')}</TooltipContent>
                            </Tooltip>
                        </MorphingDialogTrigger>

                        <MorphingDialogContainer>
                            <MorphingDialogContent className="relative flex h-[calc(100dvh-2rem)] w-[min(100vw-2rem,92rem)] max-w-full flex-col overflow-hidden rounded-xl border border-border/35 bg-card px-4 py-4 text-card-foreground shadow-md  md:h-[calc(100dvh-3rem)] md:px-6">
                                <EditDialogContent
                                    group={group}
                                    editMembers={members}
                                    isSubmitting={updateGroup.isPending}
                                    onSubmit={handleSubmitEdit}
                                    onTestAvailability={handleTestDraftGroup}
                                    onRemoveFailedModels={handleRemoveFailedMembers}
                                    isTestingAvailability={isTesting}
                                    canRemoveFailedModels={canRemoveFailedModels}
                                    testProgressCompleted={completedCount}
                                    testProgressTotal={totalCount}
                                    testProgressValue={progressValue}
                                    testResults={testProgress?.results ?? []}
                                    availabilityByMemberId={availabilityByMemberId}
                                    availabilitySummary={availabilitySummary}
                                />
                            </MorphingDialogContent>
                        </MorphingDialogContainer>
                    </MorphingDialog>

                    {supportsGroupTest(group.endpoint_type) ? (
                    <Tooltip side="top" sideOffset={10} align="center">
                        <TooltipTrigger asChild>
                            <button
                                type="button"
                                onClick={handleTestGroup}
                                disabled={isTesting || !group.id}
                                className="rounded-md p-2 text-muted-foreground transition-colors hover:bg-card hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
                            >
                                {isTesting ? <Loader2 className="size-4 animate-spin" /> : <Activity className="size-4" />}
                            </button>
                        </TooltipTrigger>
                        <TooltipContent>{t('detail.actions.testAvailability')}</TooltipContent>
                    </Tooltip>
                    ) : null}

                    <Tooltip side="top" sideOffset={10} align="center">
                        <TooltipTrigger>
                            <CopyIconButton
                                text={group.name}
                                className="rounded-md p-2 text-muted-foreground transition-colors hover:bg-card hover:text-foreground"
                                copyIconClassName="size-4"
                                checkIconClassName="size-4 text-primary"
                            />
                        </TooltipTrigger>
                        <TooltipContent>{t('detail.actions.copyName')}</TooltipContent>
                    </Tooltip>
                    {!confirmDelete && (
                        <Tooltip side="top" sideOffset={10} align="center">
                            <TooltipTrigger>
                                <motion.button layoutId={`delete-btn-group-${group.id}`} type="button" onClick={() => setConfirmDelete(true)} className="rounded-md p-2 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive">
                                    <Trash2 className="size-4" />
                                </motion.button>
                            </TooltipTrigger>
                            <TooltipContent>{t('detail.actions.delete')}</TooltipContent>
                        </Tooltip>
                    )}
                </div>

                <AnimatePresence>
                    {confirmDelete && (
                        <motion.div layoutId={`delete-btn-group-${group.id}`} className="absolute inset-0 flex items-center justify-center gap-2 rounded-lg bg-destructive p-2" transition={{ type: 'spring', stiffness: 400, damping: 30 }}>
                            <button type="button" onClick={() => setConfirmDelete(false)} className="flex h-7 w-7 items-center justify-center rounded-lg bg-destructive-foreground/20 text-destructive-foreground transition-all hover:bg-destructive-foreground/30 active:scale-95">
                                <X className="size-4" />
                            </button>
                            <button type="button" onClick={() => group.id && deleteGroup.mutate(group.id, { onSuccess: () => toast.success(t('toast.deleted')) })} disabled={deleteGroup.isPending} className="flex-1 h-7 flex items-center justify-center gap-2 rounded-lg bg-destructive-foreground text-destructive text-sm font-semibold transition-all hover:bg-destructive-foreground/90 active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed">
                                <Trash2 className="size-3.5" />
                                {t('detail.actions.confirmDelete')}
                            </button>
                        </motion.div>
                    )}
                </AnimatePresence>
                </div>

                {(() => {
                    const modelNames = (group.items || []).map((item) => item.model_name);
                    const capabilities = inferGroupCapabilities(modelNames);
                    const modelCount = modelNames.length;
                    return (
                        <div className="mt-4 flex flex-wrap items-center gap-2">
                            {capabilities.map((cap) => (
                                <span
                                    key={cap}
                                    className={cn(
                                        'inline-flex items-center rounded-full px-2.5 py-1 text-[10px] font-medium',
                                        CAPABILITY_COLORS[cap]
                                    )}
                                >
                                    {t(CAPABILITY_LABEL_KEYS[cap])}
                                </span>
                            ))}
                            <span className="ml-auto inline-flex items-center gap-1 rounded-full border border-border/20 bg-card px-2.5 py-1 text-[10px] text-muted-foreground">
                                <Layers className="size-3" />
                                {t('card.modelCount', { count: modelCount })}
                            </span>
                        </div>
                    );
                })()}
            </header>

            <section className="relative mb-4 rounded-lg border border-border/25 bg-card p-3">
                <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-border/25 bg-card px-2.5 py-1 text-[0.64rem] font-semibold text-muted-foreground">
                    <Waves className="size-3.5" />
                    {t(`mode.${MODE_LABELS[resolvedMode]}`)}
                </div>
                <div className="grid grid-cols-2 gap-2 md:grid-cols-5">
                {([GroupMode.RoundRobin, GroupMode.Random, GroupMode.Failover, GroupMode.Weighted, GroupMode.Auto] as const).map((m) => (
                    <button
                        key={m}
                        type="button"
                        aria-disabled={isUpdatingMode || !group.id}
                        onClick={() => {
                            if (isUpdatingMode || !group.id) return;
                            if (m === group.mode) return;
                            updateGroup.mutate({ id: group.id!, mode: m }, { onSuccess, onError });
                        }}
                        className={cn(
                            'rounded-md px-3 py-2 text-xs font-medium transition-[transform,border-color,background-color] duration-300',
                            resolvedMode === m
                                ? 'border border-primary/20 bg-primary text-primary-foreground'
                                : 'border border-border/25 bg-card text-foreground hover:-translate-y-0.5 hover:border-primary/16 hover:bg-card',
                            // Keep visuals stable (no opacity/disabled flicker) while still preventing double-submit via onClick guard.
                            (!group.id) && 'cursor-not-allowed opacity-50'
                        )}
                    >
                        {t(`mode.${MODE_LABELS[m]}`)}
                    </button>
                ))}
                </div>
            </section>

            <section
                className="relative overflow-hidden rounded-lg border border-border/25 bg-card transition-[height] duration-200"
                style={{ height: `${memberListHeightRem}rem` }}
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
                    showWeight={resolvedMode === GroupMode.Weighted || resolvedMode === GroupMode.Auto}
                    layoutScope={`card-${group.id ?? 'unknown'}`}
                />
            </section>

            {(isTesting || resultByItemId.size > 0) && (
                <section className="mt-4 rounded-lg border border-border/25 bg-card p-4">
                    <div className="flex items-center justify-between gap-3">
                        <div className="space-y-1">
                            <div className="inline-flex items-center gap-2 rounded-full border border-border/25 bg-card px-2.5 py-1 text-[0.64rem] font-semibold text-muted-foreground">
                                <TestTubeDiagonal className="size-3.5" />
                                {t('card.testProgressTitle')}
                            </div>
                            <p className="text-xs text-muted-foreground">
                                {t('card.testProgressCount', { completed: completedCount, total: totalCount })}
                            </p>
                        </div>
                        {isTesting && <Loader2 className="size-4 animate-spin text-muted-foreground" />}
                    </div>
                    <Progress value={progressValue} className="mt-3 h-2" />
                    <ul className="mt-3 space-y-2">
                        {displayMembers.map((member) => {
                            const result = member.item_id !== undefined ? resultByItemId.get(member.item_id) : undefined;
                            const status = !result ? 'pending' : result.passed ? 'passed' : 'failed';

                            return (
                                <li key={`test-status-${member.id}`} className="flex items-center justify-between gap-3 rounded-lg border border-border/25 bg-card px-3 py-2.5">
                                    <div className="min-w-0">
                                        <div className="truncate text-sm font-medium text-foreground">{member.name}</div>
                                        <div className="truncate text-xs text-muted-foreground">{member.channel_name}</div>
                                    </div>
                                    <div className="flex items-center gap-2 text-xs">
                                        {status === 'pending' && (
                                            <>
                                                <Clock3 className="size-3.5 text-muted-foreground" />
                                                <span className="text-muted-foreground">{t('card.testPending')}</span>
                                            </>
                                        )}
                                        {status === 'passed' && (
                                            <>
                                                <CircleCheck className="size-3.5 text-emerald-500" />
                                                <span className="text-emerald-600 dark:text-emerald-400">{t('card.testPassed')}</span>
                                            </>
                                        )}
                                        {status === 'failed' && (
                                            <>
                                                <CircleX className="size-3.5 text-destructive" />
                                                <span className="max-w-48 truncate text-destructive" title={result?.message || t('card.testUnknownError')}>
                                                    {result?.message || t('card.testUnknownError')}
                                                </span>
                                            </>
                                        )}
                                    </div>
                                </li>
                            );
                        })}
                    </ul>
                </section>
            )}

            {failedTestResults.length > 0 && (
                <section className="mt-4 rounded-lg border border-amber-500/28 bg-amber-500/6 p-4">
                    <div className="flex items-start justify-between gap-3">
                        <div className="text-sm font-medium text-amber-700 dark:text-amber-300">
                            {t('card.testFailedTitle')}
                        </div>
                        <button
                            type="button"
                            onClick={handleRemoveFailedMembers}
                            disabled={updateGroup.isPending}
                            className="shrink-0 rounded-lg bg-amber-500/15 px-2.5 py-1 text-xs font-medium text-amber-700 transition-colors hover:bg-amber-500/25 disabled:cursor-not-allowed disabled:opacity-50 dark:text-amber-300"
                        >
                            {t('detail.actions.removeFailedModels')}
                        </button>
                    </div>
                    <ul className="mt-2 space-y-1 text-xs text-muted-foreground">
                        {failedTestResults.map((result) => (
                                <li key={`${result.item_id}-${result.channel_id}-${result.model_name}`}>
                                    {result.model_name} @ {result.channel_name} · {t('card.testAttempts', { count: result.attempts })} · {result.message || t('card.testUnknownError')}
                                </li>
                            ))}
                    </ul>
                    <p className="mt-2 text-xs text-amber-700/90 dark:text-amber-300/90">
                        {t('card.testRemovableHint')}
                    </p>
                </section>
            )}
        </article >
    );
}
