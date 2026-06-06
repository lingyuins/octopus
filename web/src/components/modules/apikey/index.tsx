'use client';

import { PageWrapper } from '@/components/common/PageWrapper';
import { APIKeyPagePanel } from '@/components/modules/setting/APIKey';

export function APIKeyPage() {
    return (
        <div className="h-full min-h-0 overflow-y-auto overscroll-contain rounded-t-xl">
            <PageWrapper className="pb-3 md:pb-6">
                <APIKeyPagePanel />
            </PageWrapper>
        </div>
    );
}
