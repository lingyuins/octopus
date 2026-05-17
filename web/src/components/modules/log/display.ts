import type { ChannelAttempt, RelayLog, RelayLogDetail } from '../../../api/endpoints/log.ts';
import { inferCapabilities, type CapabilityType } from '../group/capabilities.ts';

const capabilityEndpointMap: Record<Exclude<CapabilityType, 'chat' | 'moderation'>, string> = {
    embeddings: 'embeddings',
    rerank: 'rerank',
    image_generation: 'image_generation',
    audio_speech: 'audio_speech',
    audio_transcription: 'audio_transcription',
    video_generation: 'video_generation',
    music_generation: 'music_generation',
    search: 'search',
};

function firstNonEmpty(...values: Array<string | null | undefined>) {
    for (const value of values) {
        const trimmed = value?.trim();
        if (trimmed) return trimmed;
    }
    return '';
}

function lastAttemptValue(
    attempts: ChannelAttempt[] | undefined,
    pick: (attempt: ChannelAttempt) => string | undefined,
) {
    if (!attempts?.length) return '';
    for (let index = attempts.length - 1; index >= 0; index -= 1) {
        const value = pick(attempts[index])?.trim();
        if (value) return value;
    }
    return '';
}

function firstNonZero(...values: Array<number | null | undefined>) {
    for (const value of values) {
        if (typeof value === 'number' && value > 0) return value;
    }
    return 0;
}

function lastAttemptChannelId(attempts: ChannelAttempt[] | undefined) {
    if (!attempts?.length) return 0;
    for (let index = attempts.length - 1; index >= 0; index -= 1) {
        const value = attempts[index]?.channel_id;
        if (typeof value === 'number' && value > 0) return value;
    }
    return 0;
}

function inferEndpointTypeFromModels(modelNames: string[]) {
    const normalizedNames = modelNames
        .map((name) => name.trim().toLowerCase())
        .filter(Boolean);

    if (normalizedNames.some((name) => name.includes('deepseek'))) {
        return 'deepseek';
    }
    if (normalizedNames.some((name) => name.includes('mimo'))) {
        return 'mimo';
    }

    for (const modelName of normalizedNames) {
        const capability = inferCapabilities(modelName).find((item) => item !== 'chat');
        if (!capability) continue;
        if (capability === 'moderation') return 'moderations';
        return capabilityEndpointMap[capability];
    }

    return normalizedNames.length > 0 ? 'chat' : '';
}

export function resolveLogDisplayFields(
    log: RelayLog,
    detail?: RelayLogDetail | null,
    channelNameById?: ReadonlyMap<number, string>,
) {
    const mergedAttempts = detail?.attempts?.length ? detail.attempts : log.attempts;

    const requestModelName = firstNonEmpty(detail?.request_model_name, log.request_model_name);
    const actualModelName = firstNonEmpty(
        detail?.actual_model_name,
        log.actual_model_name,
        lastAttemptValue(mergedAttempts, (attempt) => attempt.model_name),
        requestModelName,
    );
    const endpointType = firstNonEmpty(
        detail?.endpoint_type,
        log.endpoint_type,
        inferEndpointTypeFromModels([
            actualModelName,
            requestModelName,
            lastAttemptValue(mergedAttempts, (attempt) => attempt.model_name),
        ]),
    );
    const channelId = firstNonZero(detail?.channel, log.channel, lastAttemptChannelId(mergedAttempts));
    const channelName = firstNonEmpty(
        detail?.channel_name,
        log.channel_name,
        lastAttemptValue(mergedAttempts, (attempt) => attempt.channel_name),
        channelId > 0 ? channelNameById?.get(channelId) : '',
        channelId > 0 ? `Channel #${channelId}` : '',
    );

    return {
        requestAPIKeyName: firstNonEmpty(detail?.request_api_key_name, log.request_api_key_name),
        requestModelName,
        actualModelName,
        endpointType,
        channelId,
        channelName,
        semanticCacheHit: detail?.semantic_cache_hit ?? log.semantic_cache_hit ?? false,
        cacheReadTokens: detail?.cache_read_tokens ?? log.cache_read_tokens ?? 0,
    };
}
