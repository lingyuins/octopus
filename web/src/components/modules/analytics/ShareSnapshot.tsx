'use client';

import { useRef, useState } from 'react';
import { toPng } from 'html-to-image';
import { Share2, Download, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { toast } from '@/components/common/Toast';
import { useTranslations } from 'next-intl';

interface ShareSnapshotProps {
  data: {
    title: string;
    subtitle?: string;
    stats: Array<{
      label: string;
      value: string | number;
      change?: string;
      trend?: 'up' | 'down' | 'neutral';
    }>;
    timestamp: string;
  };
}

export function ShareSnapshot({ data }: ShareSnapshotProps) {
  const t = useTranslations('analytics.share');
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const snapshotRef = useRef<HTMLDivElement>(null);

  const handleExport = async () => {
    if (!snapshotRef.current) return;

    setLoading(true);
    try {
      const dataUrl = await toPng(snapshotRef.current, {
        backgroundColor: '#ffffff',
        pixelRatio: 2,
        quality: 0.95,
      });

      // Create download link
      const link = document.createElement('a');
      link.download = `analytics-${data.title.toLowerCase().replace(/\s+/g, '-')}-${Date.now()}.png`;
      link.href = dataUrl;
      link.click();

      toast.success(t('exportSuccess'));
      setOpen(false);
    } catch (error) {
      console.error('Failed to export snapshot:', error);
      toast.error(t('exportError'));
    } finally {
      setLoading(false);
    }
  };

  const handleCopyToClipboard = async () => {
    if (!snapshotRef.current) return;

    setLoading(true);
    try {
      const dataUrl = await toPng(snapshotRef.current, {
        backgroundColor: '#ffffff',
        pixelRatio: 2,
        quality: 0.95,
      });

      // Convert to blob and copy to clipboard
      const response = await fetch(dataUrl);
      const blob = await response.blob();
      await navigator.clipboard.write([
        new ClipboardItem({ 'image/png': blob }),
      ]);

      toast.success(t('copySuccess'));
    } catch (error) {
      console.error('Failed to copy to clipboard:', error);
      toast.error(t('copyError'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Button
        variant="outline"
        size="sm"
        onClick={() => setOpen(true)}
        className="gap-2"
      >
        <Share2 className="h-4 w-4" />
        {t('button')}
      </Button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Share2 className="h-5 w-5" />
              {t('title')}
            </DialogTitle>
          </DialogHeader>

          <div className="space-y-4">
            {/* Snapshot Preview */}
            <div className="border rounded-lg overflow-hidden">
              <div
                ref={snapshotRef}
                className="p-6 bg-gradient-to-br from-primary/5 to-primary/10 space-y-6"
              >
                {/* Header */}
                <div className="space-y-1">
                  <h2 className="text-2xl font-bold">{data.title}</h2>
                  {data.subtitle && (
                    <p className="text-sm text-muted-foreground">{data.subtitle}</p>
                  )}
                  <p className="text-xs text-muted-foreground">{data.timestamp}</p>
                </div>

                {/* Stats Grid */}
                <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
                  {data.stats.map((stat, index) => (
                    <div
                      key={index}
                      className="bg-white dark:bg-gray-900 rounded-lg p-4 shadow-sm border border-border"
                    >
                      <div className="text-xs text-muted-foreground mb-1">
                        {stat.label}
                      </div>
                      <div className="text-2xl font-bold">{stat.value}</div>
                      {stat.change && (
                        <div
                          className={`text-xs mt-1 flex items-center gap-1 ${
                            stat.trend === 'up'
                              ? 'text-green-600'
                              : stat.trend === 'down'
                              ? 'text-red-600'
                              : 'text-muted-foreground'
                          }`}
                        >
                          {stat.change}
                        </div>
                      )}
                    </div>
                  ))}
                </div>

                {/* Footer */}
                <div className="text-xs text-muted-foreground text-center pt-4 border-t">
                  Generated by Octopus Analytics
                </div>
              </div>
            </div>

            {/* Actions */}
            <div className="flex gap-2">
              <Button
                onClick={handleExport}
                disabled={loading}
                className="flex-1 gap-2"
              >
                <Download className="h-4 w-4" />
                {t('download')}
              </Button>
              <Button
                onClick={handleCopyToClipboard}
                disabled={loading}
                variant="outline"
                className="flex-1 gap-2"
              >
                <Share2 className="h-4 w-4" />
                {t('copy')}
              </Button>
              <Button
                onClick={() => setOpen(false)}
                variant="ghost"
                disabled={loading}
                size="icon"
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
