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
            request_count: formatCount(data.stats.request_success + data.stats.request_failed),
        },
        info: data.info,
    };
}
