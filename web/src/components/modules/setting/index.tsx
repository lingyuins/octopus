'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import {
    Sun, User, Database, Cog, Shield, RotateCcw, Bell, Route, Zap,
    ScrollText, Globe, Server, KeyRound, AlertTriangle, ChevronsUpDown,
} from 'lucide-react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { SettingAppearance } from './Appearance';
import { SettingAccount } from './Account';
import { SettingBackup } from './Backup';
import { SettingSystem } from './System';
import { SettingInfo } from './Info';
import { SettingLLMSync } from './LLMSync';
import { SettingLog } from './Log';
import { SettingCircuitBreaker } from './CircuitBreaker';
import { SettingRetry } from './Retry';
import { SettingAutoStrategy } from './AutoStrategy';
import { SettingAIRoute } from './AIRoute';
import { SettingSemanticCache } from './SemanticCache';
import { SettingRouteGroupDanger } from './RouteGroupDanger';

type SettingItem = {
    id: string;
    icon: React.ReactNode;
    titleKey: string;
    component: React.ReactNode;
};

const SETTING_ITEMS: SettingItem[] = [
    { id: 'appearance',        icon: <Sun className="h-5 w-5" />,              titleKey: 'appearance',           component: <SettingAppearance /> },
    { id: 'ai-route',          icon: <Zap className="h-5 w-5" />,               titleKey: 'aiRoute.title',        component: <SettingAIRoute /> },
    { id: 'auto-strategy',     icon: <Cog className="h-5 w-5" />,               titleKey: 'autoStrategy.title',   component: <SettingAutoStrategy /> },
    { id: 'account',           icon: <User className="h-5 w-5" />,              titleKey: 'account.title',         component: <SettingAccount /> },
    { id: 'semantic-cache',    icon: <Database className="h-5 w-5" />,          titleKey: 'semanticCache.title',  component: <SettingSemanticCache /> },
    { id: 'retry',             icon: <RotateCcw className="h-5 w-5" />,         titleKey: 'retry.title',          component: <SettingRetry /> },
    { id: 'log',               icon: <ScrollText className="h-5 w-5" />,        titleKey: 'log.title',            component: <SettingLog /> },
    { id: 'info',              icon: <AlertTriangle className="h-5 w-5" />,     titleKey: 'info.title',           component: <SettingInfo /> },
    { id: 'system',            icon: <Server className="h-5 w-5" />,             titleKey: 'system',               component: <SettingSystem /> },
    { id: 'llmsync',           icon: <KeyRound className="h-5 w-5" />,          titleKey: 'llmSync.title',        component: <SettingLLMSync /> },
    { id: 'circuit-breaker',   icon: <Shield className="h-5 w-5" />,            titleKey: 'circuitBreaker.title', component: <SettingCircuitBreaker /> },
    { id: 'backup',            icon: <Database className="h-5 w-5" />,          titleKey: 'backup.title',         component: <SettingBackup /> },
    { id: 'route-group-danger',icon: <AlertTriangle className="h-5 w-5" />,     titleKey: 'routeGroups.title',    component: <SettingRouteGroupDanger /> },
];

export function Setting() {
    const t = useTranslations('setting');
    const [openId, setOpenId] = useState<string | null>(null);
    const activeItem = SETTING_ITEMS.find((item) => item.id === openId);

    return (
        <div className="h-full min-h-0 overflow-y-auto overscroll-contain rounded-t-xl">
            <div className="pb-24 md:pb-6 px-4 md:px-6 pt-4">
                <div className="space-y-2 max-w-2xl mx-auto">
                    {SETTING_ITEMS.map((item) => (
                        <button
                            key={item.id}
                            type="button"
                            onClick={() => setOpenId(item.id)}
                            className="w-full flex items-center gap-3 rounded-xl border border-border/35 bg-card px-4 py-3.5 text-left shadow-sm transition-colors hover:bg-accent/40 active:bg-accent/60"
                        >
                            <span className="shrink-0 text-muted-foreground">{item.icon}</span>
                            <span className="flex-1 text-sm font-semibold text-card-foreground truncate">
                                {t(item.titleKey)}
                            </span>
                            <ChevronsUpDown className="h-4 w-4 shrink-0 text-muted-foreground" />
                        </button>
                    ))}
                </div>
            </div>

            <Dialog open={openId !== null} onOpenChange={(open) => { if (!open) setOpenId(null); }}>
                <DialogContent className="w-[min(95vw,720px)] max-h-[90vh] overflow-y-auto p-0 gap-0 sm:rounded-2xl">
                    {activeItem && (
                        <>
                            <DialogHeader className="px-6 pt-5 pb-3 border-b border-border/30">
                                <DialogTitle className="flex items-center gap-2 text-base font-bold">
                                    {activeItem.icon}
                                    {t(activeItem.titleKey)}
                                </DialogTitle>
                            </DialogHeader>
                            <div className="px-6 py-5">
                                {activeItem.component}
                            </div>
                        </>
                    )}
                </DialogContent>
            </Dialog>
        </div>
    );
}
