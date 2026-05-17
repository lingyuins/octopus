'use client';

import { PageWrapper } from '@/components/common/PageWrapper';
import { SettingAppearance } from './Appearance';
import { SettingSystem } from './System';
import { SettingAccount } from './Account';
import { SettingInfo } from './Info';
import { SettingLLMSync } from './LLMSync';
import { SettingLog } from './Log';
import { SettingBackup } from './Backup';
import { SettingCircuitBreaker } from './CircuitBreaker';
import { SettingRetry } from './Retry';
import { SettingAutoStrategy } from './AutoStrategy';
import { SettingAIRoute } from './AIRoute';
import { SettingSemanticCache } from './SemanticCache';
import { SettingRouteGroupDanger } from './RouteGroupDanger';

export function Setting() {
    const cards = [
        { id: 'setting-appearance', node: <SettingAppearance /> },
        { id: 'setting-ai-route', node: <SettingAIRoute /> },
        { id: 'setting-auto-strategy', node: <SettingAutoStrategy /> },
        { id: 'setting-account', node: <SettingAccount /> },
        { id: 'setting-semantic-cache', node: <SettingSemanticCache /> },
        { id: 'setting-retry', node: <SettingRetry /> },
        { id: 'setting-log', node: <SettingLog /> },
        { id: 'setting-info', node: <SettingInfo /> },
        { id: 'setting-system', node: <SettingSystem /> },
        { id: 'setting-llmsync', node: <SettingLLMSync /> },
        { id: 'setting-circuit-breaker', node: <SettingCircuitBreaker /> },
        { id: 'setting-backup', node: <SettingBackup /> },
        { id: 'setting-route-group-danger', node: <SettingRouteGroupDanger /> },
    ];

    return (
        <div className="h-full min-h-0 overflow-y-auto overscroll-contain rounded-t-xl">
            <PageWrapper className="pb-24 md:pb-6">
                <div className="columns-1 gap-5 lg:columns-2 xl:columns-3 [column-fill:balance]">
                    {cards.map((card) => (
                        <div key={card.id} className="mb-5 min-w-0 break-inside-avoid">
                            {card.node}
                        </div>
                    ))}
                </div>
            </PageWrapper>
        </div>
    );
}
