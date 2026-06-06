'use client';

import { useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/common/Toast';
import { useRedeemCodes, useRedeemAllSites, useRedemptionHistory } from '@/api/endpoints/redemption';
import { useRemoteSiteList } from '@/api/endpoints/remote-site';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { CheckCircle2, XCircle, Clock, AlertCircle, Gift, Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';

export function RedemptionPanel() {
  const t = useTranslations('redemption');
  const [codes, setCodes] = useState('');
  const [selectedSiteId, setSelectedSiteId] = useState<string>('');

  const { data: sites = [] } = useRemoteSiteList();
  const redeemCodes = useRedeemCodes();
  const redeemAllSites = useRedeemAllSites();
  const { data: history = [] } = useRedemptionHistory(
    selectedSiteId ? parseInt(selectedSiteId) : 0
  );

  const handleRedeem = () => {
    const codeList = codes.split('\n').map(c => c.trim()).filter(c => c.length > 0);

    if (codeList.length === 0) {
      toast.error(t('errors.noCodes'));
      return;
    }

    if (!selectedSiteId) {
      toast.error(t('errors.noSite'));
      return;
    }

    redeemCodes.mutate(
      {
        site_id: parseInt(selectedSiteId),
        codes: codeList,
      },
      {
        onSuccess: (data) => {
          toast.success(
            t('success.description', {
              success: data.success_count,
              total: data.total_codes,
            })
          );
          setCodes('');
        },
        onError: (error) => {
          toast.error(error.message);
        },
      }
    );
  };

  const handleRedeemAll = () => {
    const codeList = codes.split('\n').map(c => c.trim()).filter(c => c.length > 0);

    if (codeList.length === 0) {
      toast.error(t('errors.noCodes'));
      return;
    }

    redeemAllSites.mutate(
      { codes: codeList },
      {
        onSuccess: (data) => {
          const successCount = data.filter(r => r.status === 'success').length;
          toast.success(t('success.allSitesDesc', { count: successCount }));
          setCodes('');
        },
        onError: (error) => {
          toast.error(error.message);
        },
      }
    );
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'success':
        return <CheckCircle2 className="h-4 w-4 text-green-500" />;
      case 'already_used':
        return <Clock className="h-4 w-4 text-yellow-500" />;
      case 'invalid':
        return <AlertCircle className="h-4 w-4 text-orange-500" />;
      case 'failed':
      default:
        return <XCircle className="h-4 w-4 text-red-500" />;
    }
  };

  const getStatusBadge = (status: string) => {
    const variant = status === 'success' ? 'default' :
                    status === 'already_used' ? 'secondary' :
                    status === 'invalid' ? 'destructive' : 'outline';
    return (
      <Badge variant={variant as 'default' | 'secondary' | 'destructive' | 'outline'}>
        {t(`status.${status}`)}
      </Badge>
    );
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Gift className="h-5 w-5" />
            {t('title')}
          </CardTitle>
          <CardDescription>{t('description')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t('labels.codes')}</label>
            <textarea
              placeholder={t('placeholders.codes')}
              value={codes}
              onChange={(e) => setCodes(e.target.value)}
              rows={6}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            />
            <p className="text-xs text-muted-foreground">
              {t('hints.codes')}
            </p>
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">{t('labels.site')}</label>
            <Select value={selectedSiteId} onValueChange={setSelectedSiteId}>
              <SelectTrigger>
                <SelectValue placeholder={t('placeholders.site')} />
              </SelectTrigger>
              <SelectContent>
                {sites.map((site) => (
                  <SelectItem key={site.id} value={site.id.toString()}>
                    {site.name} ({site.base_url})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col sm:flex-row gap-2">
            <Button
              onClick={handleRedeem}
              disabled={redeemCodes.isPending || redeemAllSites.isPending}
              className="flex-1 sm:flex-none"
            >
              {redeemCodes.isPending ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  {t('buttons.redeeming')}
                </>
              ) : (
                t('buttons.redeem')
              )}
            </Button>
            <Button
              variant="outline"
              onClick={handleRedeemAll}
              disabled={redeemCodes.isPending || redeemAllSites.isPending}
              className="flex-1 sm:flex-none"
            >
              {redeemAllSites.isPending ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  {t('buttons.redeeming')}
                </>
              ) : (
                t('buttons.redeemAll')
              )}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('history.title')}</CardTitle>
          <CardDescription>
            {selectedSiteId
              ? t('history.description')
              : t('history.selectSite')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {selectedSiteId ? (
            <div className="max-h-[400px] overflow-y-auto space-y-2">
              {history.length === 0 ? (
                <p className="text-center text-muted-foreground py-8">
                  {t('history.empty')}
                </p>
              ) : (
                history.map((record) => (
                  <div
                    key={record.id}
                    className="flex items-center justify-between p-3 border rounded-lg"
                  >
                    <div className="flex items-center gap-3">
                      {getStatusIcon(record.status)}
                      <div>
                        <div className="font-mono text-sm">{record.code}</div>
                        <div className="text-xs text-muted-foreground">
                          {new Date(record.executed_at).toLocaleString()}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      {record.quota_awarded > 0 && (
                        <span className="text-sm font-medium text-green-600">
                          +{record.quota_awarded}
                        </span>
                      )}
                      {getStatusBadge(record.status)}
                    </div>
                  </div>
                ))
              )}
            </div>
          ) : (
            <div className="flex items-center justify-center h-[200px] text-muted-foreground">
              {t('history.selectSite')}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
