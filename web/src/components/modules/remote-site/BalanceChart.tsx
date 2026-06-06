'use client';

import { useMemo } from 'react';
import { useBalanceChart, useBalancePrediction } from '@/api/endpoints/balance-history';
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid, ReferenceLine } from 'recharts';
import { useTranslations } from 'next-intl';

interface BalanceChartProps {
    siteId: number;
    days?: number;
}

export function BalanceChart({ siteId, days = 30 }: BalanceChartProps) {
    const t = useTranslations('hub');
    const startDate = useMemo(() => {
        const d = new Date();
        d.setDate(d.getDate() - days);
        return d.toISOString().slice(0, 10);
    }, [days]);

    const { data: chartData, isLoading } = useBalanceChart(siteId, startDate);
    const { data: prediction } = useBalancePrediction(siteId);

    // Merge historical + prediction data
    const mergedData = useMemo(() => {
        if (!chartData || chartData.length === 0) return [];

        const historical = chartData.map((p) => ({
            day_key: p.day_key,
            quota: p.quota,
            predicted: undefined as number | undefined,
        }));

        if (prediction?.trend_points && prediction.trend_points.length > 0) {
            // Add a bridge point (last historical point repeated as prediction start)
            const lastHistorical = historical[historical.length - 1];
            const predicted = prediction.trend_points.map((p) => ({
                day_key: p.day_key,
                quota: undefined as number | undefined,
                predicted: p.quota,
            }));
            // Bridge: add the last historical point with prediction start
            const bridge = {
                day_key: lastHistorical.day_key,
                quota: lastHistorical.quota,
                predicted: lastHistorical.quota,
            };
            // Replace last historical with bridge
            historical[historical.length - 1] = bridge;
            return [...historical, ...predicted];
        }

        return historical;
    }, [chartData, prediction]);

    if (isLoading) {
        return <div className="h-40 sm:h-48 flex items-center justify-center text-muted-foreground text-sm">{t('chart.loading')}</div>;
    }

    if (!mergedData || mergedData.length === 0) {
        return <div className="h-40 sm:h-48 flex items-center justify-center text-muted-foreground text-sm">{t('chart.noData')}</div>;
    }

    return (
        <div className="space-y-3">
            <div className="h-48 sm:h-56 md:h-64 w-full">
            <ResponsiveContainer width="100%" height="100%">
                <LineChart data={mergedData} margin={{ top: 5, right: 10, left: 10, bottom: 5 }}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                    <XAxis
                        dataKey="day_key"
                        tick={{ fontSize: 11 }}
                        tickFormatter={(v: string) => v.slice(5)}
                        className="text-muted-foreground"
                    />
                    <YAxis
                        tick={{ fontSize: 11 }}
                        className="text-muted-foreground"
                        width={50}
                    />
                    <Tooltip
                        contentStyle={{
                            fontSize: 12,
                            borderRadius: 8,
                            border: '1px solid hsl(var(--border))',
                            background: 'hsl(var(--card))',
                        }}
                        formatter={(value: number, name: string) => {
                            if (value === undefined || value === null) return ['-', name];
                            return [value.toFixed(2), name === 'predicted' ? t('chart.predicted') : t('chart.quota')];
                        }}
                        labelFormatter={(label: string) => label}
                    />
                    <Line
                        type="monotone"
                        dataKey="quota"
                        stroke="hsl(var(--primary))"
                        strokeWidth={2}
                        dot={false}
                        activeDot={{ r: 4 }}
                        connectNulls={false}
                    />
                    <Line
                        type="monotone"
                        dataKey="predicted"
                        stroke="hsl(var(--chart-4, 45 96% 54%))"
                        strokeWidth={2}
                        strokeDasharray="6 4"
                        dot={false}
                        activeDot={{ r: 4 }}
                        connectNulls={false}
                    />
                    {prediction && prediction.estimated_zero_at && (
                        <ReferenceLine
                            x={prediction.estimated_zero_at.slice(5)}
                            stroke="hsl(var(--destructive))"
                            strokeDasharray="3 3"
                            strokeOpacity={0.6}
                        />
                    )}
                </LineChart>
            </ResponsiveContainer>
            </div>

            {/* Prediction info cards */}
            {prediction && (
                <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                    <div className="rounded-lg border border-border/30 bg-muted/30 p-2 text-center">
                        <div className="text-[10px] text-muted-foreground">{t('chart.dailyBurn')}</div>
                        <div className="text-sm font-bold tabular-nums">{prediction.daily_burn_rate.toFixed(1)}</div>
                    </div>
                    <div className="rounded-lg border border-border/30 bg-muted/30 p-2 text-center">
                        <div className="text-[10px] text-muted-foreground">{t('chart.daysRemaining')}</div>
                        <div className="text-sm font-bold tabular-nums">
                            {prediction.days_remaining < 0 ? '∞' : prediction.days_remaining}
                        </div>
                    </div>
                    <div className="rounded-lg border border-border/30 bg-muted/30 p-2 text-center">
                        <div className="text-[10px] text-muted-foreground">{t('chart.sevenDayAvg')}</div>
                        <div className="text-sm font-bold tabular-nums">{prediction.seven_day_avg_burn.toFixed(1)}</div>
                    </div>
                    <div className="rounded-lg border border-border/30 bg-muted/30 p-2 text-center">
                        <div className="text-[10px] text-muted-foreground">{t('chart.estimatedZero')}</div>
                        <div className="text-sm font-bold tabular-nums">
                            {prediction.estimated_zero_at ? prediction.estimated_zero_at.slice(5) : '—'}
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
