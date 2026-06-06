import { formatCount, formatMoney, formatTime } from '../../lib/utils.ts';
import type { StatsAPIKey, StatsAPIKeyFormatted } from './stats';

/**
 * API Key Stats 响应（包含 stats 和 info）
 */
export interface APIKeyStatsResponse<TInfo> {
    stats: StatsAPIKey;
    info: TInfo;
}

export interface APIKeyStatsResponseFormatted<TInfo> {
    stats: StatsAPIKeyFormatted;
    info: TInfo;
}

export function formatAPIKeyStatsResponse<TInfo>(
    data: APIKeyStatsResponse<TInfo>,
): APIKeyStatsResponseFormatted<TInfo> {
    return {
        stats: {
            api_key_id: data.stats.api_key_id,
            input_token: formatCount(data.stats.input_token),
            output_token: formatCount(data.stats.output_token),
            total_token: formatCount(data.stats.input_token + data.stats.output_token),
            input_cost: formatMoney(data.stats.input_cost),
            output_cost: formatMoney(data.stats.output_cost),
            total_cost: formatMoney(data.stats.input_cost + data.stats.output_cost),
            wait_time: formatTime(data.stats.wait_time),
            request_success: formatCount(data.stats.request_success),
            request_failed: formatCount(data.stats.request_failed),
            latency_p50: formatTime(data.stats.latency_p50),
            latency_p95: formatTime(data.stats.latency_p95),
            latency_p99: formatTime(data.stats.latency_p99),
            ftut_avg: formatTime(data.stats.ftut_avg),
            ftut_p50: formatTime(data.stats.ftut_p50),
            ftut_p95: formatTime(data.stats.ftut_p95),
            ftut_p99: formatTime(data.stats.ftut_p99),
            histogram_lt_100: formatCount(data.stats.histogram_lt_100),
            histogram_100_500: formatCount(data.stats.histogram_100_500),
            histogram_500_1k: formatCount(data.stats.histogram_500_1k),
            histogram_1k_5k: formatCount(data.stats.histogram_1k_5k),
            histogram_gt_5k: formatCount(data.stats.histogram_gt_5k),
            request_count: formatCount(data.stats.request_success + data.stats.request_failed),
        },
        info: data.info,
    };
}
