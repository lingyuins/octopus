# Relay Endpoints, Proxy Pool, Account Pools & Model Mapping

### 🌐 Public Relay Endpoints

The public relay API supports both OpenAI-style and Anthropic-style clients:

- OpenAI-style clients: `Authorization: Bearer sk-octopus-...`
- Anthropic-style clients: `x-api-key: sk-octopus-...`

| Category | Paths | Notes |
|----------|-------|-------|
| OpenAI-compatible LLM | `/v1/chat/completions`, `/v1/responses`, `/v1/embeddings`, `/v1/models` | JSON request / response |
| Anthropic-compatible LLM | `/v1/messages` | Anthropic-style request / response |
| JSON media / utility | `/v1/images/generations`, `/v1/audio/speech`, `/v1/videos/generations`, `/v1/music/generations`, `/v1/search`, `/v1/rerank`, `/v1/moderations` | Uses the same group / retry / circuit-breaker pipeline |
| Multipart media | `/v1/images/edits`, `/v1/images/variations`, `/v1/audio/transcriptions` | Multipart upload forwarding |

JSON media endpoints can also proxy upstream SSE streams when the provider supports `stream=true`.

Semantic cache is currently evaluated for non-streaming and streaming OpenAI Chat and OpenAI Responses text requests (streaming cache hits replay from the SSE session buffer). Anthropic, embeddings, and media / utility requests bypass the cache and continue through the normal relay flow.

**Log module: two views**

The Log module has two switchable views:

- **Relay logs** — one record per forwarded request, with candidates, retries, and cost details (see below)
- **Error logs** — collects backend panics and errors (with request method / path, client IP, and user agent) and frontend exceptions (uncaught errors / unhandled rejections, with page URL and stack). Entries are stored in the main database and retained up to an internal cap, with the oldest half cleaned up periodically; the view also offers one-click clearing

**Relay log details:**

The Log module opens a two-panel detail view: Group / Request Body tabs on the left and the response JSON on the right, with usage, timing, and cost in the footer. Candidate rows show model logos, channel/key names, and recorded success / failure / skip / circuit-break statuses. Expand a row to inspect individual retries and their messages. Cooldown seconds are shown only when explicitly recorded in the log; they are not a live countdown. Current items from an exact-name group supplement recorded candidates and are marked as unrecorded. Deleted, renamed, or mapped groups may only show the recorded candidates. Narrow screens stack the panels vertically.

**Zen Direct Model Routing:**

Requests with model name prefixed `zen/<model>` bypass group model mapping and route directly to the upstream model. Octopus performs smart channel-type detection based on the model name (e.g., Claude → Anthropic, Gemini → Gemini, GPT → OpenAI).

**Response ID Affinity:**

For the OpenAI Responses API, follow-up requests referencing the same response ID are automatically routed to the same upstream channel to maintain conversation continuity.

**Model Mapping:**

Global model name rewriting rules are applied in the relay pipeline before group resolution. Rules support exact, wildcard (glob), and regex matching with priority ordering and optional group scope.

---

### 🌍 Proxy Pool

A shared proxy configuration pool accessible from the app shell toolbar:

- Named proxy configurations with URL, scheme (SOCKS5 / HTTP / HTTPS), enable/disable, and remarks
- 4 proxy modes: `direct`, `system`, `pool`, `inherit`
- Proxy connectivity testing against a configurable test URL
- **Reference tree** showing which sites, site accounts, managed channels, and channels use each proxy
- Jump-to-reference navigation that deep-links to the referencing entity
- Deletion protection when a proxy has active references

---

### 👥 Account Pools

A top-level module for managing upstream accounts (Anthropic, OpenAI, Gemini, Grok, Volcengine, custom) that relay requests can be scheduled through:

- Pool list with keyword search; the create/edit dialog covers name, description, scheduling strategy, default concurrency, cooldown, and enabled state; deleting a pool asks for confirmation
- The pool detail view shows summary stats and filters accounts by keyword, platform, and status — rendered as a table on desktop and cards on mobile
- Accounts support per-platform credential types (OAuth / API key / cookie / upstream / setup token); supported platforms can authorize an account directly via the OAuth login flow
- Selected accounts support batch token refresh, error clearing, and connectivity tests; individual accounts can be paused from scheduling temporarily
- Account credentials can be imported in bulk or exported to a file — the export dialog warns that the output contains live credentials

---

### 🔁 Model Mapping

Global model name rewriting rules applied in the relay pipeline before group resolution:

- **Match types**: Exact, Wildcard (glob), and Regex
- **Target model**: the rewritten model name
- **Priority ordering**: rules are evaluated in priority order
- **Group scope**: optionally apply only to a specific group
- **Enable/disable toggle** per rule

---

| [← Home](../Home.md) | [← Previous](06-Model-Market.md) | [Next →](08-Analytics.md) |