'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslations } from 'next-intl'
import { Calendar, RefreshCw, Filter } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { toast } from 'sonner'
import { useRemoteSiteList } from '@/api/endpoints/remote-site'
import {
  queryUsageHistory,
  queryUsageSummary,
  getUsageModels,
  syncUsageHistory,
  syncAllUsageHistory,
} from '@/api/endpoints/usage-history'

export function UsageHistoryPanel() {
  const t = useTranslations('usage_history')
  const queryClient = useQueryClient()

  const [siteId, setSiteId] = useState<string>('')
  const [dayFrom, setDayFrom] = useState('')
  const [dayTo, setDayTo] = useState('')
  const [modelName, setModelName] = useState('')
  const [tokenName, setTokenName] = useState('')

  const { data: sites = [] } = useRemoteSiteList()

  const { data: history } = useQuery({
    queryKey: ['usage-history', siteId, dayFrom, dayTo, modelName, tokenName],
    queryFn: () =>
      queryUsageHistory({
        site_id: siteId ? Number(siteId) : undefined,
        day_from: dayFrom || undefined,
        day_to: dayTo || undefined,
        model_name: modelName || undefined,
        token_name: tokenName || undefined,
        limit: 100,
      }),
    enabled: !!siteId,
  })

  const { data: summary } = useQuery({
    queryKey: ['usage-summary', siteId, dayFrom, dayTo],
    queryFn: () =>
      queryUsageSummary({
        site_id: siteId ? Number(siteId) : undefined,
        day_from: dayFrom || undefined,
        day_to: dayTo || undefined,
      }),
    enabled: !!siteId,
  })

  const { data: models = [] } = useQuery({
    queryKey: ['usage-models', siteId],
    queryFn: () => getUsageModels(Number(siteId)),
    enabled: !!siteId,
  })

  const syncMutation = useMutation({
    mutationFn: () => syncUsageHistory(Number(siteId)),
    onSuccess: (data) => {
      toast.success(t('sync_success', { count: data.inserted }))
      queryClient.invalidateQueries({ queryKey: ['usage-history'] })
      queryClient.invalidateQueries({ queryKey: ['usage-summary'] })
    },
    onError: () => {
      toast.error(t('sync_error'))
    },
  })

  const syncAllMutation = useMutation({
    mutationFn: syncAllUsageHistory,
    onSuccess: (data) => {
      toast.success(t('sync_all_success', { count: data.inserted }))
      queryClient.invalidateQueries({ queryKey: ['usage-history'] })
      queryClient.invalidateQueries({ queryKey: ['usage-summary'] })
    },
    onError: () => {
      toast.error(t('sync_error'))
    },
  })

  const totalTokens = summary?.reduce((acc, s) => acc + s.total_tokens, 0) || 0
  const totalQuota = summary?.reduce((acc, s) => acc + s.quota_consumed, 0) || 0

  return (
    <div className="py-4 sm:py-6 space-y-4 sm:space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-2xl sm:text-3xl font-bold">{t('title')}</h1>
          <p className="text-sm sm:text-base text-muted-foreground mt-1">{t('description')}</p>
        </div>
        <div className="flex gap-2 shrink-0">
          {siteId && (
            <Button
              onClick={() => syncMutation.mutate()}
              disabled={syncMutation.isPending}
              variant="outline"
            >
              <RefreshCw className={`mr-2 h-4 w-4 ${syncMutation.isPending ? 'animate-spin' : ''}`} />
              {t('sync')}
            </Button>
          )}
          <Button
            onClick={() => syncAllMutation.mutate()}
            disabled={syncAllMutation.isPending}
          >
            <RefreshCw className={`mr-2 h-4 w-4 ${syncAllMutation.isPending ? 'animate-spin' : ''}`} />
            {t('sync_all')}
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Filter className="h-5 w-5" />
            {t('filters')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-3 sm:gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-5">
            <div>
              <label className="text-sm font-medium">{t('site')}</label>
              <Select value={siteId} onValueChange={setSiteId}>
                <SelectTrigger>
                  <SelectValue placeholder={t('select_site')} />
                </SelectTrigger>
                <SelectContent>
                  {sites.map((site) => (
                    <SelectItem key={site.id} value={String(site.id)}>
                      {site.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div>
              <label className="text-sm font-medium">{t('day_from')}</label>
              <Input
                type="date"
                value={dayFrom}
                onChange={(e) => setDayFrom(e.target.value)}
              />
            </div>

            <div>
              <label className="text-sm font-medium">{t('day_to')}</label>
              <Input
                type="date"
                value={dayTo}
                onChange={(e) => setDayTo(e.target.value)}
              />
            </div>

            <div>
              <label className="text-sm font-medium">{t('model')}</label>
              <Select value={modelName} onValueChange={setModelName}>
                <SelectTrigger>
                  <SelectValue placeholder={t('all_models')} />
                </SelectTrigger>
                <SelectContent>
                  {models.map((model) => (
                    <SelectItem key={model} value={model}>
                      {model}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div>
              <label className="text-sm font-medium">{t('token')}</label>
              <Input
                value={tokenName}
                onChange={(e) => setTokenName(e.target.value)}
                placeholder={t('all_tokens')}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {siteId && (
        <>
          <div className="grid gap-3 sm:gap-4 grid-cols-1 sm:grid-cols-3">
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>{t('total_requests')}</CardDescription>
                <CardTitle className="text-3xl">
                  {history?.total || 0}
                </CardTitle>
              </CardHeader>
            </Card>

            <Card>
              <CardHeader className="pb-2">
                <CardDescription>{t('total_tokens')}</CardDescription>
                <CardTitle className="text-3xl">
                  {totalTokens.toLocaleString()}
                </CardTitle>
              </CardHeader>
            </Card>

            <Card>
              <CardHeader className="pb-2">
                <CardDescription>{t('total_quota')}</CardDescription>
                <CardTitle className="text-3xl">
                  ${totalQuota.toFixed(4)}
                </CardTitle>
              </CardHeader>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Calendar className="h-5 w-5" />
                {t('history')}
              </CardTitle>
              <CardDescription>{t('history_description')}</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="overflow-x-auto -mx-6 px-6">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('date')}</TableHead>
                    <TableHead>{t('model')}</TableHead>
                    <TableHead>{t('token')}</TableHead>
                    <TableHead className="text-right">{t('requests')}</TableHead>
                    <TableHead className="text-right">{t('prompt_tokens')}</TableHead>
                    <TableHead className="text-right">{t('completion_tokens')}</TableHead>
                    <TableHead className="text-right">{t('total_tokens')}</TableHead>
                    <TableHead className="text-right">{t('quota')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {history?.records.map((record) => (
                    <TableRow key={record.id}>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <Badge variant="outline">{record.day_key}</Badge>
                          <span className="text-xs text-muted-foreground">
                            {record.hour}:00
                          </span>
                        </div>
                      </TableCell>
                      <TableCell className="font-medium">{record.model_name}</TableCell>
                      <TableCell>{record.token_name || '-'}</TableCell>
                      <TableCell className="text-right">{record.request_count}</TableCell>
                      <TableCell className="text-right">{record.prompt_tokens.toLocaleString()}</TableCell>
                      <TableCell className="text-right">{record.completion_tokens.toLocaleString()}</TableCell>
                      <TableCell className="text-right">{record.total_tokens.toLocaleString()}</TableCell>
                      <TableCell className="text-right">${record.quota_consumed.toFixed(4)}</TableCell>
                    </TableRow>
                  ))}
                  {(!history?.records || history.records.length === 0) && (
                    <TableRow>
                      <TableCell colSpan={8} className="text-center text-muted-foreground py-8">
                        {t('no_data')}
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
              </div>
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}
