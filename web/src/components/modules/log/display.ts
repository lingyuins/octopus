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

function isStreamRequest(requestContent?: string | null) {
    if (!requestContent) return false;
    try {
        const parsed = JSON.parse(requestContent) as { stream?: unknown };
        return parsed.stream === true;
    } catch {
        return false;
    }
}

function inferRequestTypeKey(endpointType: string, modelNames: string[], requestContent?: string | null) {
    const normalizedEndpoint = endpointType.trim().toLowerCase();
    const normalizedNames = modelNames
        .map((name) => name.trim().toLowerCase())
        .filter(Boolean);
    const streaming = isStreamRequest(requestContent);

    if (normalizedEndpoint === 'embeddings') return 'embedding';
    if (normalizedEndpoint === 'responses') return 'responses';
    if (normalizedEndpoint === 'messages') return 'anthropicMessages';
    if (normalizedEndpoint === 'gemini') return 'gemini';
    if (normalizedEndpoint === 'mimo' || normalizedNames.some((name) => name.includes('mimo'))) return 'mimoChat';
    if (normalizedNames.some((name) => name.includes('gemini'))) return 'gemini';
    if (normalizedNames.some((name) => name.includes('claude'))) return 'anthropicMessages';
    if (normalizedNames.some((name) => name.includes('doubao') || name.includes('volcengine') || name.includes('ark'))) return 'volcengine';
    if (normalizedEndpoint === 'deepseek' || normalizedEndpoint === 'chat') {
        return streaming ? 'streamingChat' : 'chat';
    }
    return normalizedEndpoint ? normalizedEndpoint : (streaming ? 'streamingChat' : 'chat');
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
    const requestContent = firstNonEmpty(detail?.request_content, '');
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
        channelId > 0 ? 'channel_fallback' : '',
    );

    return {
        requestAPIKeyName: firstNonEmpty(detail?.request_api_key_name, log.request_api_key_name),
        requestModelName,
        actualModelName,
        endpointType,
        requestTypeKey: inferRequestTypeKey(endpointType, [actualModelName, requestModelName], requestContent),
        channelId,
        channelName,
        semanticCacheHit: detail?.semantic_cache_hit ?? log.semantic_cache_hit ?? false,
        cacheReadTokens: detail?.cache_read_tokens ?? log.cache_read_tokens ?? 0,
    };
}




