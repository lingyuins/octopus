'use client';

import { useStatsDaily, type StatsDailyFormatted } from '@/api/endpoints/stats';
import { useMemo, useRef, useLayoutEffect, useState, useCallback } from 'react';
import { createPortal } from 'react-dom';
import { useTranslations } from 'next-intl';
import { Fragment } from 'react';
import dayjs from 'dayjs';
import { Flame } from 'lucide-react';

interface StatsDailyData {
    dateStr: string;
    isFuture: boolean;
    formatted: StatsDailyFormatted | null;
}

const ACTIVITY_LEVELS = [
    { min: 5000, level: 4 },
    { min: 2000, level: 3 },
    { min: 1000, level: 2 },
    { min: 1, level: 1 }
];

function getActivityLevel(value: number): number {
    if (value === 0) return 0;
    return ACTIVITY_LEVELS.find(level => value >= level.min)?.level || 1;
}

function formatCompactDate(dateStr: string): string {
    if (/^\d{8}$/.test(dateStr)) {
        return `${dateStr.slice(0, 4)}-${dateStr.slice(4, 6)}-${dateStr.slice(6, 8)}`;
    }
    return dateStr;
}

export function Activity() {
    const { data: statsDailyFormatted, isLoading } = useStatsDaily();
    const scrollRef = useRef<HTMLDivElement>(null);
    const t = useTranslations('home.activity');

    const [tooltip, setTooltip] = useState<{ day: StatsDailyData; x: number; y: number; visible: boolean } | null>(null);

    const days = useMemo(() => {
        if (!statsDailyFormatted) return [];
        const formattedMap = new Map(statsDailyFormatted.map(stat => [stat.date, stat]));

        // `today` 使用 dayjs() 默认获取浏览器本地日期。
        // 日历网格仅用于展示活动热力图，不参与统计聚合；
        // 与后端统计时区的对齐在 P2 统计模型升级专项中处理。
        const today = dayjs();
        const startDate = today.subtract(today.day() + 53 * 7, 'day');

        const result: StatsDailyData[] = [];

        for (let i = 0; i < 54 * 7; i++) {
            const currentDate = startDate.add(i, 'day');
            const dateStr = currentDate.format('YYYYMMDD');

            result.push({
                dateStr,
                isFuture: currentDate.isAfter(today, 'day'),
                formatted: formattedMap.get(dateStr) || null
            });
        }

        return result;
    }, [statsDailyFormatted]);

    const [maskImage, setMaskImage] = useState('none');

    const checkScroll = useCallback(() => {
        if (!scrollRef.current) return;
        const { scrollLeft, scrollWidth, clientWidth } = scrollRef.current;
        const isStart = scrollLeft <= 1;
        const isEnd = Math.abs(scrollWidth - clientWidth - scrollLeft) <= 1;

        if (isStart && isEnd) {
            setMaskImage('none');
        } else if (isStart) {
            setMaskImage('linear-gradient(to left, transparent, rgba(0,0,0,0) 10px, black 40px)');
        } else if (isEnd) {
            setMaskImage('linear-gradient(to right, transparent, rgba(0,0,0,0) 10px,black 40px)');
        } else {
            setMaskImage('linear-gradient(to right, transparent, rgba(0,0,0,0) 10px, black 40px, black calc(100% - 40px),  rgba(0,0,0,0) calc(100% - 10px), transparent)');
        }
    }, []);

    useLayoutEffect(() => {
        const scrollToRight = () => {
            if (scrollRef.current) {
                scrollRef.current.scrollLeft = scrollRef.current.scrollWidth;
                checkScroll();
            }
        };
        scrollToRight();
        window.addEventListener('resize', scrollToRight);
        return () => window.removeEventListener('resize', scrollToRight);
    }, [days, isLoading, checkScroll]);

    return (
        <div className="relative flex h-full flex-col rounded-xl border border-border bg-card text-card-foreground">
            <div className="px-4 pt-4 md:px-5 md:pt-5">
                <div className="mb-3 inline-flex items-center gap-2 rounded-md border border-primary/12 bg-card px-2.5 py-1 text-xs font-medium text-primary">
                    <Flame className="h-3.5 w-3.5" strokeWidth={1.5} />
                    <span>{t('title')}</span>
                </div>
                <p className="text-sm leading-6 text-muted-foreground">{t('description')}</p>
            </div>
            <div
                ref={scrollRef}
                onScroll={checkScroll}
                className="relative flex-1 overflow-x-auto px-4 pb-5 pt-4 md:px-5 md:pb-5"
                style={{ maskImage, WebkitMaskImage: maskImage }}
            >
                <div className="w-fit rounded-lg border border-border bg-card p-3.5">
                    <div className="grid gap-1"
                        style={{
                            gridTemplateColumns: 'repeat(54, 1rem)',
                            gridTemplateRows: 'repeat(7, 1rem)',
                            gridAutoFlow: 'column'
                        }}
                    >
                        {days.map((day) => {
                            if (day.isFuture) {
                                return <div key={day.dateStr} />;
                            }

                            const level = getActivityLevel(day.formatted?.request_count.raw ?? 0);
                            const tooltipDateLabel = formatCompactDate(day.dateStr);
                            const ariaLabel = day.formatted
                                ? [
                                      tooltipDateLabel,
                                      `${t('requestCount')} ${day.formatted.request_count.formatted.value}${day.formatted.request_count.formatted.unit}`,
                                      `${t('waitTime')} ${day.formatted.wait_time.formatted.value}${day.formatted.wait_time.formatted.unit}`,
                                      `${t('totalToken')} ${day.formatted.total_token.formatted.value}${day.formatted.total_token.formatted.unit}`,
                                      `${t('totalCost')} ${day.formatted.total_cost.formatted.value}${day.formatted.total_cost.formatted.unit}`,
                                  ].join('，')
                                : `${tooltipDateLabel}，${t('noData')}`;

                            return (
                                <button
                                    key={day.dateStr}
                                    type="button"
                                    aria-label={ariaLabel}
                                    className="cursor-pointer rounded-sm ring-1 ring-border/20 transition-all duration-150 hover:scale-150 hover:ring-primary/30 focus-visible:scale-150 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-primary/50"
                                    onMouseEnter={(e) => {
                                        const rect = e.currentTarget.getBoundingClientRect();
                                        setTooltip({ day, x: rect.left + rect.width / 2, y: rect.top, visible: true });
                                    }}
                                    onMouseLeave={() => setTooltip(prev => prev ? { ...prev, visible: false } : null)}
                                    onFocus={(e) => {
                                        const rect = e.currentTarget.getBoundingClientRect();
                                        setTooltip({ day, x: rect.left + rect.width / 2, y: rect.top, visible: true });
                                    }}
                                    onBlur={() => setTooltip(prev => prev ? { ...prev, visible: false } : null)}
                                    style={{ backgroundColor: level === 0 ? 'var(--muted)' : `color-mix(in oklch, var(--primary) ${level * 25}%, var(--muted))` }}
                                />
                            );
                        })}
                    </div>
                </div>
            </div>
            {tooltip && typeof document !== 'undefined' && createPortal(
                (() => {
                    const isLeft = tooltip.x < 200;
                    const isRight = tooltip.x > window.innerWidth - 200;
                    const isTop = tooltip.y < window.innerHeight / 2;
                    const tooltipDateLabel = formatCompactDate(tooltip.day.dateStr);

                    let transform = 'translate(-50%, 15%)';
                    if (!isTop && !isLeft && !isRight) {
                        transform = 'translate(-50%, -105%)';
                    } else if (isTop && isLeft) {
                        transform = 'translate(10%, 15%)';
                    } else if (isTop && isRight) {
                        transform = 'translate(-110%, 15%)';
                    } else if (!isTop && isLeft) {
                        transform = 'translate(10%, -105%)';
                    } else if (!isTop && isRight) {
                        transform = 'translate(-110%, -105%)';
                    }

                    return (
                        <div
                            className={`fixed z-50 w-fit min-w-max rounded-lg border border-border bg-card p-3 text-sm text-foreground transition-opacity duration-300 pointer-events-none ${tooltip.visible ? 'opacity-100' : 'opacity-0'}`}
                            style={{
                                left: tooltip.x,
                                top: tooltip.y,
                                transform
                            }}
                        >
                            <div className="space-y-2">
                                <p className="font-semibold text-foreground">{tooltipDateLabel}</p>
                                {tooltip.day.formatted ? (
                                    <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 items-center text-muted-foreground">
                                        {[
                                            { labelKey: 'requestCount', ...tooltip.day.formatted.request_count },
                                            { labelKey: 'waitTime', ...tooltip.day.formatted.wait_time },
                                            { labelKey: 'totalToken', ...tooltip.day.formatted.total_token },
                                            { labelKey: 'totalCost', ...tooltip.day.formatted.total_cost },
                                        ].map((item, index) => (
                                            <Fragment key={index}>
                                                <span className="wrap-break-word">{t(item.labelKey)}</span>
                                                <span className="text-foreground font-medium text-right">{item.formatted.value}{item.formatted.unit}</span>
                                            </Fragment>
                                        ))}
                                    </div>
                                ) : (
                                    <p className="text-muted-foreground">{t('noData')}</p>
                                )}
                            </div>
                        </div>
                    );
                })(),
                document.body
            )}
        </div>
    );
}
