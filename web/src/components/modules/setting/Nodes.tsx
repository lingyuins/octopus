'use client';

import { useEffect, useId, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { useNodeAction, useNodeList, useTestNode, type LocalNode, type NodeAction } from '@/api/endpoints/node';
import type { NodeProbeResult } from '@/api/endpoints/node-probe';
import { useSettingList, useSetSetting, SettingKey } from '@/api/endpoints/setting';
import { NodeProbeFeedback } from '@/components/common/NodeProbeFeedback';
import { API_BASE_URL } from '@/api/client';
import { useAuthStore } from '@/api/endpoints/user';
import { useNavStore } from '@/components/modules/navbar/nav-store';
import type { ApiError } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { toast } from '@/components/common/Toast';
import { formatDateTime } from '@/lib/time';
import { writeClipboardText } from '@/lib/clipboard';
import { buildNodeCloudURL, buildNodeConfig } from './node-config';

function NodeTestResult({ result }: { result: NodeProbeResult | { error: string } }) {
    const t = useTranslations('node.test');
    return <div role={'error' in result ? 'alert' : 'status'} className="space-y-2 rounded-lg border p-3">
        {'error' in result ? <p className="break-words text-sm text-destructive">{t('failed')}: {result.error}</p> : <>
            <p className={result.passed ? 'text-sm font-medium' : 'text-sm font-medium text-destructive'}>{t(result.passed ? 'success' : 'failed')}</p>
            <NodeProbeFeedback code={result.code} latencyMS={result.latency_ms} modelCount={result.model_count} />
        </>}
    </div>;
}

export function SettingNodes() {
    const t = useTranslations('node');
    const active = useNavStore((state) => state.activeItem === 'setting');
    const authenticated = useAuthStore((state) => state.isAuthenticated);
    const { data, isLoading, isError, isFetching, refetch } = useNodeList(active && authenticated);
    const test = useTestNode();
    const [testResults, setTestResults] = useState<Record<number, NodeProbeResult | { error: string }>>({});
    const testGeneration = useRef(0);
    const canWrite = data?.can_write === true;
    const { data: settings } = useSettingList();
    const setSetting = useSetSetting();
    const [streamQueueFrames, setStreamQueueFrames] = useState('');
    const [consumerWaitTimeout, setConsumerWaitTimeout] = useState('');
    const initStreamQueueFrames = useRef('');
    const initConsumerWaitTimeout = useRef('');
    useEffect(() => {
        if (!settings) return;
        const get = (k: string) => settings.find((s) => s.key === k)?.value;
        const frames = get(SettingKey.NodeTransportStreamQueueFrames);
        const wait = get(SettingKey.NodeTransportConsumerWaitTimeout);
        queueMicrotask(() => {
            if (frames !== undefined) { setStreamQueueFrames(frames); initStreamQueueFrames.current = frames; }
            if (wait !== undefined) { setConsumerWaitTimeout(wait); initConsumerWaitTimeout.current = wait; }
        });
    }, [settings]);
    const saveValue = (key: string, value: string, initial: string) => {
        if (value === initial) return;
        setSetting.mutate({ key, value }, {
            onSuccess: () => {
                toast.success(t('saved'));
                if (key === SettingKey.NodeTransportStreamQueueFrames) initStreamQueueFrames.current = value;
                else if (key === SettingKey.NodeTransportConsumerWaitTimeout) initConsumerWaitTimeout.current = value;
            },
        });
    };
    const [name, setName] = useState('');
    const [secret, setSecret] = useState<{ node: LocalNode; token: string } | null>(null);
    const [confirmation, setConfirmation] = useState<NodeAction | null>(null);
    const confirmationId = useId();
    const confirmationRef = useRef<HTMLDivElement>(null);
    useEffect(() => {
        if (confirmation) {
            confirmationRef.current?.scrollIntoView({ block: 'nearest' });
            confirmationRef.current?.focus();
        }
    }, [confirmation]);
    const action = useNodeAction((node, token) => {
        if (useNavStore.getState().activeItem === 'setting' && useAuthStore.getState().isAuthenticated) setSecret({ node, token });
    });
    useEffect(() => {
        const generation = testGeneration;
        if (!active || !authenticated || !canWrite) { setSecret(null); setTestResults({}); }
        return () => { generation.current++; };
    }, [active, authenticated, canWrite]);
    const run = (input: NodeAction) => {
        if (!canWrite) return;
        setSecret(null);
        testGeneration.current++;
        setTestResults({});
        action.mutate(input, {
            onSuccess: () => { setName(''); toast.success(t('saved')); },
            onError: (error) => {
                const key = (error as ApiError).message_key;
                toast.error(key === 'node.errors.referenced' ? t('errors.referenced') : key === 'node.errors.notFound' ? t('errors.notFound') : t('errors.operationFailed'));
            },
        });
    };
    const runTest = (id: number) => {
        if (!canWrite || test.isPending) return;
        const generation = ++testGeneration.current;
        setTestResults((current) => { const next = { ...current }; delete next[id]; return next; });
        test.mutate(id, {
            onSuccess: (result) => {
                if (testGeneration.current !== generation || useNavStore.getState().activeItem !== 'setting' || !useAuthStore.getState().isAuthenticated) return;
                setTestResults((current) => ({ ...current, [id]: result }));
                if (result.passed) toast.success(t('test.success'));
                else toast.error(t('test.failed'));
            },
            onError: (error) => {
                if (testGeneration.current === generation && useNavStore.getState().activeItem === 'setting' && useAuthStore.getState().isAuthenticated) {
                    setTestResults((current) => ({ ...current, [id]: { error: error.message } }));
                }
            },
        });
    };
    const copy = async (value: string) => {
        try {
            await writeClipboardText(value);
            toast.success(t('copied'));
        } catch {
            toast.error(t('errors.copyFailed'));
        }
    };
    const config = secret && canWrite
        ? buildNodeConfig(secret.token, new URL(API_BASE_URL.replace(/\/?$/, '/'), window.location.href).href.replace(/\/$/, ''))
        : '';
    const cloudURL = secret && canWrite
        ? buildNodeCloudURL(new URL(API_BASE_URL.replace(/\/?$/, '/'), window.location.href).href.replace(/\/$/, ''))
        : '';

    return <section className="space-y-4 p-4 sm:p-6">
        <div className="flex items-center justify-between gap-2">
            <h2 className="text-lg font-semibold">{t('title')}</h2>
            <Button type="button" variant="outline" disabled={isFetching} onClick={() => void refetch()}>{t('refresh')}</Button>
        </div>
        <p className="text-sm text-muted-foreground">{t('description')}</p>
        <p className="text-sm text-muted-foreground">{t('restartHint')}</p>
        {!canWrite && <p className="text-sm text-muted-foreground">{t('readOnly')}</p>}
        {isLoading && <p role="status">{t('loading')}</p>}
        {isError && <div role="alert"><p>{t('errors.loadFailed')}</p><Button variant="outline" onClick={() => void refetch()}>{t('refresh')}</Button></div>}
        {canWrite && <form className="flex flex-wrap gap-2" onSubmit={(event) => { event.preventDefault(); setConfirmation({ action: 'create', name: name.trim() }); }}>
            <Input aria-label={t('name')} placeholder={t('name')} value={name} maxLength={100} onChange={(event) => setName(event.target.value)} required className="min-w-0 flex-1" />
            <Button type="submit" disabled={action.isPending || confirmation !== null || !name.trim()}>{t('create')}</Button>
        </form>}
        {secret && canWrite && <div className="space-y-3 rounded-xl border border-amber-500 p-4">
            <h3 className="font-semibold">{t('tokenTitle', { name: secret.node.name })}</h3>
            <p className="text-sm">{t('tokenWarning')}</p>
            <pre className="overflow-x-auto whitespace-pre-wrap break-all rounded bg-muted p-3 select-all">{secret.token}</pre>
            <Button variant="outline" onClick={() => void copy(secret.token)}>{t('copyToken')}</Button>
            <h4 className="font-medium">{t('cloudURLTitle')}</h4>
            <p className="text-sm text-muted-foreground">{t('cloudURLHint')}</p>
            <pre className="overflow-x-auto whitespace-pre-wrap break-all rounded bg-muted p-3 select-all">{cloudURL}</pre>
            <Button variant="outline" onClick={() => void copy(cloudURL)}>{t('copyCloudURL')}</Button>
            <h4 className="font-medium">{t('configTitle')}</h4>
            <p className="text-sm text-muted-foreground">{t('configHint')}</p>
            <pre className="max-h-80 overflow-auto rounded bg-muted p-3 text-xs">{config}</pre>
            <div className="flex flex-wrap gap-2">
                <Button variant="outline" onClick={() => void copy(config)}>{t('copyConfig')}</Button>
                <Button onClick={() => setSecret(null)}>{t('dismissToken')}</Button>
            </div>
        </div>}
        {data?.nodes.length === 0 && <p className="text-sm text-muted-foreground">{t('empty')}</p>}
        {canWrite && <div className="space-y-3 rounded-xl border border-border/50 p-4">
            <h3 className="font-semibold">{t('transport.title')}</h3>
            <p className="text-xs text-muted-foreground">{t('transport.description')}</p>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                    <label className="text-xs text-muted-foreground">{t('transport.streamQueueFrames')}</label>
                    <Input
                        type="number" min={1} max={4096} value={streamQueueFrames}
                        onChange={(e) => setStreamQueueFrames(e.target.value)}
                        onBlur={() => saveValue(SettingKey.NodeTransportStreamQueueFrames, streamQueueFrames, initStreamQueueFrames.current)}
                        placeholder="8"
                    />
                    <p className="mt-1 text-xs text-muted-foreground">{t('transport.streamQueueFramesHint')}</p>
                </div>
                <div>
                    <label className="text-xs text-muted-foreground">{t('transport.consumerWaitTimeout')}</label>
                    <Input
                        type="number" min={0} max={300} value={consumerWaitTimeout}
                        onChange={(e) => setConsumerWaitTimeout(e.target.value)}
                        onBlur={() => saveValue(SettingKey.NodeTransportConsumerWaitTimeout, consumerWaitTimeout, initConsumerWaitTimeout.current)}
                        placeholder="0"
                    />
                    <p className="mt-1 text-xs text-muted-foreground">{t('transport.consumerWaitTimeoutHint')}</p>
                </div>
            </div>
            <p className="text-xs text-amber-600 dark:text-amber-500">{t('transport.restartNotice')}</p>
        </div>}
        <div className="space-y-3">{data?.nodes.map((node) => <article key={node.id} className="space-y-3 rounded-xl border p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
                <h3 className="font-semibold break-all">{node.name}</h3>
                <span className="text-sm">{t(node.enabled ? 'enabled' : 'disabled')} · {t(node.online ? 'online' : 'offline')}</span>
            </div>
            <dl className="grid gap-2 text-sm sm:grid-cols-2">
                <div><dt className="text-muted-foreground">{t('version')}</dt><dd>{node.connector_version || '-'}</dd></div>
                <div><dt className="text-muted-foreground">{t('concurrency')}</dt><dd>{node.in_flight} / {node.max_concurrency}</dd></div>
                <div><dt className="text-muted-foreground">{t('lastSeen')}</dt><dd>{formatDateTime(node.last_seen_at)}</dd></div>
                <div><dt className="text-muted-foreground">{t('updatedAt')}</dt><dd>{formatDateTime(node.updated_at)}</dd></div>
            </dl>
            {node.last_error && <p className="break-all text-sm text-destructive">{t('lastError')}: {node.last_error}</p>}
            {testResults[node.id] && <NodeTestResult result={testResults[node.id]} />}
            {canWrite && <div className="flex flex-wrap gap-2">
                <Button variant="outline" type="button" disabled={test.isPending || action.isPending || confirmation !== null} onClick={() => runTest(node.id)}>{t(test.isPending && test.variables === node.id ? 'test.running' : 'test.button')}</Button>
                <Button variant="outline" type="button" disabled={action.isPending || confirmation !== null} onClick={() => setConfirmation({ action: 'update', id: node.id, enabled: !node.enabled })}>{t(node.enabled ? 'disable' : 'enable')}</Button>
                <Button variant="outline" type="button" disabled={action.isPending || confirmation !== null} onClick={() => setConfirmation({ action: 'rotate', id: node.id })}>{t('rotate')}</Button>
                <Button variant="destructive" type="button" disabled={action.isPending || confirmation !== null} onClick={() => setConfirmation({ action: 'delete', id: node.id })}>{t('delete')}</Button>
            </div>}
        </article>)}</div>
        {confirmation && <div
            ref={confirmationRef}
            role="alertdialog"
            aria-labelledby={`${confirmationId}-title`}
            aria-describedby={`${confirmationId}-description`}
            tabIndex={-1}
            className="space-y-3 rounded-xl border border-primary p-4 focus:outline-none focus:ring-2 focus:ring-ring"
            onKeyDown={(event) => {
                if (event.key === 'Escape') {
                    event.preventDefault();
                    event.stopPropagation();
                    setConfirmation(null);
                }
            }}
        >
            <h3 id={`${confirmationId}-title`} className="font-semibold">{t('confirmTitle')}</h3>
            <p id={`${confirmationId}-description`} className="text-sm text-muted-foreground">{t(confirmation.action === 'delete' ? 'confirmDelete' : confirmation.action === 'rotate' ? 'confirmRotate' : confirmation.action === 'create' ? 'confirmCreate' : 'confirmToggle')}</p>
            <div className="flex flex-wrap gap-2">
                <Button type="button" variant="outline" onClick={(event) => { event.stopPropagation(); setConfirmation(null); }}>{t('cancel')}</Button>
                <Button type="button" disabled={action.isPending || !canWrite} onClick={(event) => { event.stopPropagation(); run(confirmation); setConfirmation(null); }}>{t('confirm')}</Button>
            </div>
        </div>}
    </section>;
}
