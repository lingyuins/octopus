# Groups & Model Discovery

### 📁 Group Management

Groups aggregate multiple channels into a unified external model name.

**Core Concepts:**

- **Group name** is the model name exposed by the program
- When calling the API, set the `model` parameter to the group name
- **First Token Timeout**: seconds, streaming only, `0` disables it; media streams use the first response data byte. Member connectivity probes are not full client-route acceptance tests covering authorization, conditions, caching, and retries.
- **Session Keep Time**: unit in seconds, keeps using the same channel for the same API key + model within the configured session window, `0` means disabled
- **Condition (JSON)**: AND rules apply to text and media requests, including stream recovery. Context supports `model`, `api_key_id`, `api_key_name`, `content_type`, UTC `hour`, and `header_*`. Management writes validate fields and operators.
- **Endpoint Provider**: provider-aware request rewriting that adapts requests for upstream compatibility per endpoint type. Chat providers (`openai`, `deepseek`, `mimo`, `siliconflow`, `newapi`) strip incompatible reasoning fields; music providers (`newapi`, `minimax`) rewrite the request body and path; video provider (`agnes`) rewrites the upstream path; audio speech provider (`mimo`) converts the request format and path
- **Outbound Format**: controls cross-format adapter fallback. `""` (auto) sniffs the inbound protocol: Anthropic Messages inbound tries Anthropic first, other inbound formats try Chat Completions first, then falls back along Chat → Responses; `chat` and `responses` only set the Chat/Responses attempt order without inbound sniffing; `messages` tries Anthropic → Chat → Responses and `messages_only` uses Anthropic only. Cross-format fallback applies to route-level failures such as 404/5xx or network errors, not client/key-scoped errors. Use the `*_only` variants for upstreams (e.g. public-welfare relays) that reject the other format.
- **Thinking Mode**: the advanced group editor setting `thinking_mode` accepts `auto` (default), `off`, or `on`. Forced modes override client reasoning parameters, including when none are provided. DeepSeek/MiMo use `thinking.type`; OpenAI Chat/Responses (including Codex) use reasoning effort; Anthropic, Gemini, and Volcengine use their own controls. For a generic DeepSeek/MiMo proxy URL, select the matching API type or endpoint provider. Model support is required; raw/passthrough formats skip this policy. This is not a FIM or `/v1/completions` endpoint.
- **Key Cooldown**: rate-limit cooldown is tracked per `(keyID, model)` (issue #94), so a single model's 429 no longer blocks the same key's other models. Per-channel retry count can be set to `0` (try once, then move to the next channel); the max-total-attempts budget includes upstream attempts and cooldown/circuit skips in the current round; client input errors (400/422) do not cool down keys

**Load Balancing Modes:**

| Mode | Description |
|------|-------------|
| 🔄 **Round Robin** | Cycles through channels sequentially for each request |
| 🎲 **Random** | Randomly selects an available channel for each request |
| 🛡️ **Failover** | Prioritizes high-priority channels, switches to lower priority only on failure |
| ⚖️ **Weighted** | Orders candidates by weight from high to low, then tries them in that order |
| 🤖 **Auto** | Explores under-sampled candidates first, then prefers the candidate with the best success rate inside the configured window |

**Auto Strategy Defaults:**

- **Minimum samples**: `10`
- **Time window**: `300` seconds
- **Sliding window size**: `100` records per channel-model pair
- **Latency weight**: `30`
- Before a candidate reaches the minimum sample count, Octopus prioritizes exploration
- After candidates are explored, Octopus sorts by success rate, then uses sample count, weight, priority, and latency tuning as tie-breakers
- Auto-strategy windows are restored from the database at startup and saved periodically plus on graceful shutdown

**AI Routing Behavior:**

- Clicking **AI Route** on the route page sends all models to AI and generates the full routing table in batch
- Same-name groups synchronize their route items to the generated result; category conflicts or unreliable tier detection are reported without changing the original group or its items
- Clicking **AI Fill Current Group** in the edit dialog sends all models to AI and synchronizes the current group's items to the matched route
- Category strategies can preserve existing categories, classify by model vendor, or split free/paid routes by channel tier. Manual categories are preserved, and conflicting routes cannot write channels across tiers
- Single-group AI routing starts from the target group and requires its explicit group ID; there is no default target group setting
- AI route tasks are persistent with heartbeat, progress tracking, batch management, and interruption recovery

**CC Switch Integration:**

The group toolbar includes a CC Switch deep link generator that creates provider import links for 5 target apps: Claude Code, Codex, Gemini, OpenCode, and OpenClaw. For Claude Code, it supports mapping Haiku / Sonnet / Opus models to specific route groups.

> 💡 **Example**: Create a group named `gpt-4o`, add multiple providers' GPT-4o channels to it, then access all channels via a unified `model: gpt-4o`.

---

### 🔍 Model Discovery & Capabilities

Octopus exposes multiple levels of model visibility:

#### `/v1/models` — Flat Compatible Model List

Returns all model names that have at least one enabled channel. Compatible with OpenAI SDKs.

This is the broadest view — if a model appears here, Octopus has a channel that *declares* it.

#### `/v1/models?endpoint=<type>` — Endpoint-Filtered List

Narrows the list to models whose **declared endpoint type** matches the filter:

- `?endpoint=chat` — conversation models (chat / responses / messages / deepseek / mimo)
- `?endpoint=embeddings` — embedding models
- `?endpoint=image_generation` — image models
- `?endpoint=music_generation` — music models
- … and so on for `audio_speech`, `audio_transcription`, `video_generation`, `search`, `rerank`, `moderations`

When `endpoint` is omitted or set to `*`, all models are returned.

> Boundaries between some endpoints are not absolute. Models from the **conversation family** (`chat`, `responses`, `messages`, `deepseek`, `mimo`) are visible to one another through the `endpoint` filter because Octopus can bridge these formats transparently.

#### `GET /api/v1/model/capabilities` — Declared Capability Table (Management API)

A management-only endpoint that returns the **aggregated capability view** of every routable model:

```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "name": "gpt-4o",
      "endpoints": ["chat"],
      "conversation": true,
      "available": true
    },
    {
      "name": "music-2.6",
      "endpoints": ["music_generation"],
      "conversation": false,
      "available": true
    }
  ]
}
```

| Field | Meaning |
|-------|---------|
| `name` | Model name as exposed to clients |
| `endpoints` | Endpoint types the model declares (deduplicated, sorted) |
| `conversation` | Whether the model belongs to the conversation family |
| `available` | Whether the model has at least one enabled channel |

This is the **declared** capability — what your `Group` configuration says. The actual routable capability may be narrower; see `*` group behaviour below.

#### `*` Group Semantics

A group with endpoint type `*` (EndpointTypeAll) is a **universal pass**: it can be selected by any endpoint type, including `chat`, `embeddings`, `image_generation`, etc.

However, **universal selection does not mean every item in the group actually supports the endpoint**. For non-conversation endpoints (image / video / music / audio / search / rerank / moderation), the relay layer now filters `*` group items before the balancer:

- Only items whose channel type or model name hint at support for the requested endpoint are kept.
- If no items survive filtering, the request returns `404 model not found` instead of blindly trying incompatible channels.
- Conversation endpoints (`chat`, `responses`, `messages`, `deepseek`, `mimo`) are **not** affected by this filtering.

> **Tip:** When you see a model in `/v1/models` or `/api/v1/model/capabilities` but it still returns `model not found` for a specific endpoint, check whether the `*` group's items actually support that endpoint — the relay narrowing may have filtered them all out.

---

| [← Home](../Home.md) | [← Previous](04-Channels.md) | [Next →](06-Model-Market.md) |