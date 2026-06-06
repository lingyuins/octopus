'use client';

import { motion } from 'motion/react';
import { Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';

interface LoadingStateProps {
    message?: string;
}

export function LoadingState({ message }: LoadingStateProps) {
    const t = useTranslations('common.loadingState');

    return (
        <div className="flex min-h-[14rem] sm:min-h-[18rem] items-center justify-center px-4">
            <motion.div
                initial={{ opacity: 0, y: 12 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.4, ease: [0.22, 1, 0.36, 1] }}
                className="flex flex-col items-center gap-2.5 sm:gap-3"
            >
                <div className="grid size-12 sm:size-14 place-items-center rounded-lg border border-border bg-card">
                    <Loader2 className="size-5 sm:size-6 animate-spin text-primary" />
                </div>
                <span className="text-xs sm:text-sm text-muted-foreground">{message ?? t('message')}</span>
            </motion.div>
        </div>
    );
}
