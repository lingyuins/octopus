'use client';

import { motion } from 'motion/react';
import { AlertTriangle, RotateCcw } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';

interface ErrorStateProps {
    title?: string;
    message?: string;
    onRetry?: () => void;
}

export function ErrorState({
    title,
    message,
    onRetry,
}: ErrorStateProps) {
    const t = useTranslations('common.errorState');

    return (
        <div className="flex min-h-[14rem] sm:min-h-[18rem] items-center justify-center px-4">
            <motion.div
                initial={{ opacity: 0, y: 12 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.4, ease: [0.22, 1, 0.36, 1] }}
                className="flex flex-col items-center gap-3 sm:gap-4 text-center"
            >
                <div className="grid size-12 sm:size-14 place-items-center rounded-lg border border-destructive/20 bg-destructive/10">
                    <AlertTriangle className="size-5 sm:size-6 text-destructive" />
                </div>
                <div className="space-y-1">
                    <p className="text-sm sm:text-base font-medium">{title ?? t('title')}</p>
                    <p className="max-w-[16rem] sm:max-w-xs text-xs sm:text-sm text-muted-foreground">{message ?? t('message')}</p>
                </div>
                {onRetry && (
                    <Button
                        onClick={onRetry}
                        variant="outline"
                        size="sm"
                        className="gap-2 rounded-xl sm:size-auto"
                    >
                        <RotateCcw className="size-4" />
                        {t('retry')}
                    </Button>
                )}
            </motion.div>
        </div>
    );
}
