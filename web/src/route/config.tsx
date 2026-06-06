import { lazyWithPreload } from './lazy-with-preload';
import { lazy, ComponentType } from 'react';
import type { LucideIcon } from 'lucide-react';
import { Home, Radio, Sparkles, FolderTree, Settings, Logs, Users, Bell, BarChart3, Wrench, KeyRound, Globe } from 'lucide-react';
import { DEFAULT_NAV_ORDER } from '@/components/modules/navbar';

export type LazyComponent = ReturnType<typeof lazy> & {
    preload: () => Promise<{ default: ComponentType<Record<string, never>> }>
};

export const ROUTE_IDS = DEFAULT_NAV_ORDER;
export type RouteId = (typeof ROUTE_IDS)[number];

export interface RouteConfig {
    id: RouteId;
    icon: LucideIcon;
    component: LazyComponent;
}

const Home_Module = lazyWithPreload(() => import('@/components/modules/home').then(m => ({ default: m.Home })));
const Channel_Module = lazyWithPreload(() => import('@/components/modules/channel').then(m => ({ default: m.Channel })));
const Model_Module = lazyWithPreload(() => import('@/components/modules/model').then(m => ({ default: m.Model })));
const Group_Module = lazyWithPreload(() => import('@/components/modules/group').then(m => ({ default: m.Group })));
const Analytics_Module = lazyWithPreload(() => import('@/components/modules/analytics').then(m => ({ default: m.Analytics })));
const Log_Module = lazyWithPreload(() => import('@/components/modules/log').then(m => ({ default: m.Log })));
const APIKey_Module = lazyWithPreload(() => import('@/components/modules/apikey').then(m => ({ default: m.APIKeyPage })));
const Setting_Module = lazyWithPreload(() => import('@/components/modules/setting').then(m => ({ default: m.Setting })));
const User_Module = lazyWithPreload(() => import('@/components/modules/user').then(m => ({ default: m.User })));
const Alert_Module = lazyWithPreload(() => import('@/components/modules/alert').then(m => ({ default: m.Alert })));
const Ops_Module = lazyWithPreload(() => import('@/components/modules/ops').then(m => ({ default: m.Ops })));
const Hub_Module = lazyWithPreload(() => import('@/components/modules/remote-site').then(m => ({ default: m.RemoteSite })));

export const ROUTES: RouteConfig[] = [
    { id: 'home', icon: Home, component: Home_Module },
    { id: 'hub', icon: Globe, component: Hub_Module },
    { id: 'channel', icon: Radio, component: Channel_Module },
    { id: 'group', icon: FolderTree, component: Group_Module },
    { id: 'model', icon: Sparkles, component: Model_Module },
    { id: 'analytics', icon: BarChart3, component: Analytics_Module },
    { id: 'log', icon: Logs, component: Log_Module },
    { id: 'alert', icon: Bell, component: Alert_Module },
    { id: 'ops', icon: Wrench, component: Ops_Module },
    { id: 'apikey', icon: KeyRound, component: APIKey_Module },
    { id: 'setting', icon: Settings, component: Setting_Module },
    { id: 'user', icon: Users, component: User_Module },
];

export const CONTENT_MAP = ROUTES.reduce((acc, route) => {
    acc[route.id] = route.component;
    return acc;
}, {} as Record<RouteId, LazyComponent>);
