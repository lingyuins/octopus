# Channel Management

### 📡 Channel Management

Channels are the basic configuration units for connecting to LLM providers.

**Channel Templates:**

The UI provides built-in channel templates for quick creation: OpenAI, Anthropic, Gemini, DeepSeek, OpenRouter, SiliconFlow, Volcengine, and MiMo (the OpenAI template creates an OpenAI Responses type channel).

**Base URL Guide:**

The program automatically appends API paths based on channel type. You only need to provide the base URL:

| Channel Type | Auto-appended Path | Base URL | Full Request URL Example |
|--------------|-------------------|----------|--------------------------|
| OpenAI Chat | `/chat/completions` | `https://api.openai.com/v1` | `https://api.openai.com/v1/chat/completions` |
| OpenAI Responses | `/responses` | `https://api.openai.com/v1` | `https://api.openai.com/v1/responses` |
| OpenAI Embeddings | `/embeddings` | `https://api.openai.com/v1` | `https://api.openai.com/v1/embeddings` |
| OpenAI Images | `/images/generations`, `/images/edits`, `/images/variations` | `https://api.openai.com/v1` | `https://api.openai.com/v1/images/generations` |
| Anthropic | `/messages` | `https://api.anthropic.com/v1` | `https://api.anthropic.com/v1/messages` |
| Gemini | `/models/:model:generateContent` | `https://generativelanguage.googleapis.com/v1beta` | `https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent` |
| Volcengine | `/responses` | `https://ark.cn-beijing.volces.com/api/v3` | `https://ark.cn-beijing.volces.com/api/v3/responses` |
| MiMo Chat | `/chat/completions` | `https://api.xiaomimimo.com/v1` | `https://api.xiaomimimo.com/v1/chat/completions` |

> 💡 **Tip**: Base URLs now support `Auto detect` and `Custom`. `Auto detect` appends the version suffix based on the channel type, while `Custom` keeps the URL exactly as you entered it.

**Proxy Mode:**

Each channel can configure a proxy mode:

| Mode | Description |
|------|-------------|
| `direct` | No proxy, connect directly |
| `system` | Use system proxy settings |
| `pool` | Select from the named proxy pool |
| `inherit` | Inherit proxy from the parent site or account |

**Request Rewrite Profiles:**

Per-channel request rewriting for upstream compatibility:

| Profile | Description |
|---------|-------------|
| `preserve` | No body rewrite — forward as-is |
| `openai_chat_compat` | Strip incompatible fields for standard OpenAI Chat format |
| `codex` | Codex-specific header shaping and tool/system-message strategy |

**Parameter Override:**

Each channel supports a `param_override` JSON configuration that injects or overrides specific parameters in outbound requests to the upstream provider, enabling per-channel parameter customization without modifying the client request.

Header and message strategies:

| Strategy | Options | Description |
|----------|---------|-------------|
| Header Profile | `none`, `codex` | Codex-specific header shaping |
| Tool Role | `keep`, `stringify_to_user` | How to handle tool role messages |
| System Message | `keep`, `merge` | How to handle system messages |

**Skip Model Availability Test:**

Each channel has a `skip_model_test` toggle (issue #98). When enabled, the channel is excluded from group/model availability probes — useful for upstreams that deduct quota or ban accounts on low-byte probe requests. Skipped channels are recorded as a non-passing attempt in the test log with a clear reason instead of sending a real request upstream.

**Key Selection Strategy:**

When a channel has multiple keys, the global `key_selection_strategy` setting controls which key is picked per request:

| Strategy | Description |
|----------|-------------|
| `cost` | Default. Prefer the lowest-cost key |
| `availability` | Prefer keys with a higher availability score (error-type weighted, time-based lazy recovery) |
| `speed` | Prefer keys with higher recent TPS (tokens per second) throughput (issue #140) |
| `priority` | Prefer keys with a higher per-key priority ordering |

**Scheduled Key Availability Patrol:**

A background job (issue #142) periodically probes all channel keys at the interval configured by `key_health_check_interval` (minutes). Failed keys trigger a notification and grey out the affected channel so degraded upstreams are visible at a glance.

---

| [← Home](../Home.md) | [← Previous](03-Admin-Roles.md) | [Next →](05-Groups.md) |