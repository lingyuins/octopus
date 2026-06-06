'use client';

import { useEffect } from 'react';
import { useTranslations } from 'next-intl';
import { Plus } from 'lucide-react';
import { PageWrapper } from '@/components/common/PageWrapper';
import { Button } from '@/components/ui/button';
import { Tabs, TabsContents, TabsContent, TabsList, TabsTrigger } from '@/components/animate-ui/components/animate/tabs';
import { Site } from '@/components/modules/site';
import { SiteChannelSection } from '@/components/modules/site-channel';
import { useHubTabStore, type HubTab } from './hub-tab-store';
import { useSiteUIStore } from '@/components/modules/site/ui-store';

export function RemoteSite() {
    const t = useTranslations('hub');
    const { activeTab, setActiveTab } = useHubTabStore();
    const requestOpenCreateDialog = useSiteUIStore((state) => state.requestOpenCreateDialog);

    return (
        <PageWrapper className="h-full min-h-0 overflow-y-auto overscroll-contain space-y-4 sm:space-y-6 rounded-t-xl pb-3 md:pb-4">
            <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as HubTab)}>
                <section className="rounded-xl border border-border bg-card p-3 sm:p-5 text-card-foreground">
                    <div className="flex items-center justify-between gap-3">
                        <div className="overflow-x-auto -mx-1 px-1 scrollbar-none min-w-0">
                            <TabsList className="w-max min-w-full xl:min-w-0">
                                <TabsTrigger value="sites">{t('tabs.sites')}</TabsTrigger>
                                <TabsTrigger value="site-channels">{t('tabs.siteChannels')}</TabsTrigger>
                            </TabsList>
                        </div>
                        {activeTab === 'sites' && (
                            <Button
                                size="sm"
                                className="shrink-0 rounded-xl"
                                onClick={requestOpenCreateDialog}
                            >
                                <Plus className="size-4" />
                                <span className="hidden sm:inline">{t('addSite')}</span>
                            </Button>
                        )}
                    </div>
                </section>

                <TabsContents>
                    <TabsContent value="sites">
                        <Site />
                    </TabsContent>
                    <TabsContent value="site-channels">
                        {activeTab === 'site-channels' ? <SiteChannelSection /> : <div />}
                    </TabsContent>
                </TabsContents>
            </Tabs>
        </PageWrapper>
    );
}
