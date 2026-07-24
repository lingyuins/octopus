# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### 🚀 Features
- **Log**: show effective outbound reasoning effort and upstream reasoning tokens on log cards (hidden when empty; field toggles default on).
- **Log**: when official reasoning tokens are absent, fall back to thinking text character count (UTF-8 runes) and display as "思考 XXt" / "思考 XX字".
- **Backup (issue #158)**: 数据导入性能优化——大表分批插入（每批 1000 行），避免单次超大事务超时/内存溢出；导入大小上限从 64 MB 提升至 256 MB，前端导入页增加大小提示。

### 🐛 Bug Fixes
- **Group/Relay**: Anthropic 入站请求现在会正确应用分组出站格式（`messages` / `messages_only` 等）；此前 `isLLMRequestFormat` 漏判 Anthropic Messages，导致强制退回渠道原生 OpenAI Chat。
- **Passthrough**: 原始透传会把分组解析后的上游模型名写回请求体 `model` 字段，不再把分组名原样发给上游。
- **Relay**: 上游 400 类客户端错误（如 `context_length_exceeded`）不再被 adapter 回退链路改写成换渠道，也不再吞成管理端 502；原样把上游状态码与错误体回给下游，便于 omp 等客户端识别溢出并自动压缩上下文。
- **Transformer**: Anthropic streaming now attaches usage on `message_delta` chunks so relay logs keep input/output tokens when `message_stop` is missing.
- **Transformer**: preserve OpenAI `reasoning_effort` values `minimal`/`xhigh`/`max` instead of collapsing them to `high`; Anthropic/Gemini budget mapping now covers `xhigh`/`max`.

## [v2.4.0] - 2026-07-15

### 🚀 Features
- **Plan Provider (issue #141)**: ChatGPT Codex plan monitoring — tracks Codex subscription usage/quota and auto-creates dedicated forwarding channels with a new `codex` outbound adapter (`internal/transformer/outbound/codex/`). Plan channels are grouped under the "Plan" channel group.
- **Analytics (issue #145)**: home usage overview now supports switching between "active models only" and "all models" scopes.

## [v2.3.9] - 2026-07-15

### 🚀 Features
- **Balancer (issue #140)**: new `speed` key selection strategy — prefers keys with higher TPS (tokens per second) throughput based on recent relay performance data.
- **Channel (issue #142)**: scheduled key availability patrol — periodically probes all keys, sends failure notifications, and greys out channels with unhealthy keys.
- **Channel (issue #144)**: batch set channel group — toggle "Batch Group" mode on the channel list, select multiple channels, and move them to a target group in one operation (`POST /api/v1/channel/batch-group`).

### 🐛 Bug Fixes
- **Channel (issue #143)**: new channel default type now matches the first item in the frontend dropdown.
- **Web**: trend chart China-mode cost alignment with usage overview; removed redundant currency conversion.

## [v2.3.8] - 2026-07-13

### 🐛 Bug Fixes
- **Store (issue #135)**: Redis backend now gracefully degrades to in-memory when unreachable at startup, with background reconnection — prevents startup failure on transient Redis outages.
- **Channel**: deleting a channel group now auto-migrates its channels to the default group instead of orphaning them.
- **Group**: group deletion failure now logs the error and ensures settings consistency.
- **Web**: fixed China-mode cost display showing "1.87¥万" misalignment; optimized home heatmap and ranking panel layout ratios for large screens.

## [v2.3.7] - 2026-07-11

### 🚀 Features
- **Timezone**: stats timezone switched from fixed offsets to IANA timezone names, with frontend timezone picker wired to backend reporting.
- **Notification**: notification titles/bodies now render as i18n keys in the user's UI language instead of being baked at creation time.

### 🛠 Optimizations
- **Balancer (issue #133)**: Auto strategy scoring now senses circuit-breaker state — fully tripped channels get `score = -Inf` so they sort last during candidate selection, avoiding the appearance of conflicting with circuit breaking.

### 🐛 Bug Fixes
- **Report**: fixed daily report data errors and switched to plain-text formatting.
- **Notification**: fixed `notif-text` translation function type incompatibility; import `TranslationValues` from `next-intl`.
- **Stats**: fixed timezone test non-portability on UTC runners.

## [v2.3.6] - 2026-07-09

### 🚀 Features
- **Group**: group page now displays groups aggregated by category, with collapsible category sections.

## [v2.3.5] - 2026-07-08

### 🚀 Features
- **Plan Provider (issue #132)**: MiMo `passToken` auto-refresh — automatically refreshes `serviceToken` when the browser-cookie-based credential expires.
- **Site Management**: support skipping model list sync for sites where model enumeration is expensive or unnecessary.

### 🐛 Bug Fixes
- **DB**: fixed startup crash `FOREIGN KEY constraint failed (787)` after upgrading from older versions.

## [v2.3.4] - 2026-07-07

### 🚀 Features
- **Group**: added per-group route model view showing which models are routable through each group.
- **Notification**: support deleting notification preferences.

### 🐛 Bug Fixes
- **Group**: allow different endpoint types to use same-named route groups (previously blocked by unique constraint).
- **Stats**: fixed daily stats empty error on freshly initialized databases.

## [v2.3.3] - 2026-07-07

### 🚀 Features
- **Alert**: error-rate alert rules now support configurable scope (per-channel, per-group, or global) and sliding-window evaluation instead of fixed intervals.

### 🐛 Bug Fixes
- **Stats**: fixed daily stats cache not refreshing after period rollover.

## [v2.3.2] - 2026-07-07

### 🚀 Features
- **Group**: added group category access control — categories can restrict which user roles see the groups within them.
- **Plan Provider**: MiMo Token Plan monitoring via browser cookie — tracks MiMo subscription quota/usage and auto-creates forwarding channels.

### 🐛 Bug Fixes
- **Plan Provider**: fixed MiMo plan monitoring credential handling and quota statistics.

## [v2.3.1] - 2026-07-06

### 🚀 Features
- **Notification**: alerts merged into the notification center — alert firings now appear as notification items alongside system events, with unified read/archive/filter UX.
- **UI**: header icons hidden on narrow screens to reduce clutter.

## [v2.3.0] - 2026-07-06

### 🚀 Features
- **Notification Center**: new unified notification center (`internal/op/notification/`) — aggregates system events, alert firings, and plan-provider notifications with severity levels, read/archive state, filtering, and SSE streaming for real-time delivery.
- **Alert / Report**: usage report scheduling — configure daily/weekly/monthly usage reports delivered via notification channels (`internal/op/report/`).

### 🐛 Bug Fixes
- **Notification**: normalized list filter parameters.
- **Plan Provider**: plan channels now correctly use the "Plan" channel group.
- **Model**: normalized market dedupe aggregation.
- **Analytics**: daily usage breakdown stats now persist correctly.
- **Report**: aligned schedule API contract.
- **UI**: setting order stacked under navigation preferences in appearance layout.

## [v2.2.9-fix] - 2026-07-06

### 🛠 Optimizations
- **Channel**: reversed key priority direction (lower number = higher priority) and show priority input only when the priority strategy is active.

### 🐛 Bug Fixes
- **i18n**: added missing `channel.detail.priority` translation.

## [v2.2.9] - 2026-07-05

### 🚀 Features
- **Channel**: key priority selection — per-key priority ordering for failover/weighted strategies, with a new `priority` key selection strategy.
- **Channel**: per-key model detection endpoint (`POST /api/v1/channel/fetch-models-per-key`) — discover which models each key can access individually.
- **Group**: raw outbound passthrough mode — forward requests to upstream without any protocol transformation via the new `passthrough` outbound adapter.

### 🐛 Bug Fixes
- **Docker**: fixed compose file permission issues.

## [v2.2.8-fix2] - 2026-07-05

### 🐛 Bug Fixes
- **Deps**: removed redundant `@radix-ui/react-*` direct dependencies, fixing Accordion double-Context issues.

## [v2.2.8-fix] - 2026-07-05

### 🐛 Bug Fixes
- **Setting**: added missing Hub/Analytics/Ops sub-tab SettingKey definitions.

### 🛠 Optimizations
- **Deps**: upgraded recharts to v3, react-day-picker to v10; regenerated shadcn/ui components.

## [v2.2.8] - 2026-07-03

### 🚀 Features
- **Plan Provider**: SenseNova and StepFun plan usage monitoring integrated into the TokenPlan framework.
- **Setting**: sub-tab order and visibility configuration for Hub/Analytics/Ops modules.

### 🐛 Bug Fixes
- **Plan Provider**: plan-monitoring channels now correctly grouped under the "Plan" channel group.
- **Model**: fixed available-endpoints view scrolling; normalized market dedupe aggregation.
- **Channel**: enlarged channel group management dialog.
- **Home**: unified trend/period button heights; mobile stats cards use 2×2 grid layout.

## [v2.2.7] - 2026-07-02

### 🚀 Features
- **Channel**: disposable channel support — channels can be created with an expiry time; expired channels are auto-deleted with notification.
- **Plan Provider**: new `internal/planprovider/` module — TokenPlan quota/usage monitoring for upstream subscription plans (MiMo, StepFun, SenseNova), with auto-creation of forwarding channels from monitored plans.
- **Site Management**: added quota/TokenPlan monitoring module to site management.

### 🐛 Bug Fixes
- **Audit**: added plan-provider write routes to the audit exemption allowlist.
- **Model**: fixed model market stats disappearing; fixed multiple planprovider defects.
- **Channel**: widened channel detail and creation dialogs.

## [v2.2.6] - 2026-07-02

### 🚀 Features
- **Home (issue #125)**: the home stats panel now has a manual refresh button (refetches all stats/analytics queries and replays the count-up animation) and a configurable auto-refresh interval selector (5s / 10s / 15s / 30s / 60s / off, default 30s). The preference is persisted to localStorage. Stats hooks (`useStatsToday`/`Daily`/`Hourly`/`Total`/`APIKey`/`Channel`) and `useAnalyticsOverview` gained an optional `refetchIntervalMs` parameter (backward compatible).
- **Group**: outbound format gained `messages` and `messages_only` modes for upstream gateways that accept both OpenAI and Anthropic Messages formats.

## [v2.2.5-fix] - 2026-06-30

### 🛠 Optimizations
- **Memory (issue #124)**: `stats.modelCache` now periodically purges idle entries (1h threshold, tracked via a `modelLastActivity` sync.Map; dirty entries skip purge to avoid losing unflushed increments); the per-channel-proxy `*http.Client` is now cached by timeout bucket to eliminate per-request transport/connection-pool churn, with explicit connection-pool caps on the cloned default transport. Fixes the `key_availability` / HTTP-client memory growth.

## [v2.2.5] - 2026-06-30

### 🚀 Features
- **Cache (issue #123)**: optional Redis backend (`internal/store`) for stats persistence, runtime state, rate-limit/cooldown, failure-hint cache, and channel-delay probing — unloads runtime data to Redis for low-memory hosts and multi-instance horizontal scaling. Zero-breaking when Redis is not configured. Stats switched to Redis native incremental semantics (`HINCRBY` + Lua max), crash-safe. Circuit-breaker / auto-strategy dual-write to Redis for multi-instance sharing. A Redis cache-backend config card (test connection / save, restart to apply) was added to Settings.
- **Group (issue #122)**: group-level per-forward timeout `attempt_time_out` (default 0 = disabled) covers the entire forward (HTTP request + response read), both streaming and non-streaming — previously you could only wait for the HTTP client timeout (600s) or the global upstream timeout before failover triggered.
- **Group (issue #119)**: group-test card now distinguishes all-failed vs. partial-failed (new `last_test_all_failed` field); only all-failed cards get the full grey-out, partial-failed stays normal.
- **Log**: log cards gained TPS (output_tokens ÷ use_time, tk/s) and cache-hit-rate (cache_read_tokens ÷ (input+output) × 100%) metrics, each toggleable in the "show fields" panel.

### 🐛 Bug Fixes
- **Analytics (issue #121)**: the Channel × Model view no longer drops intermediate retry-failed channels. Root cause was a SQLite write-queue race — `relay_logs` async flush truncated the in-memory cache before `relay_log_attempts` landed via `EnqueueWrite`, so the analytics query could neither JOIN the attempts table nor read the cache. Fixed by ensuring attempts are flushed before truncating the cache.

## [v2.2.4] - 2026-06-28

### 🚀 Features
- **Log (issue #117)**: logs now support filtering by one or more specific models (exact, case-insensitive match on `request_model_name` / `actual_model_name`), with a multi-select popover in the filter bar. Coexists with the existing fuzzy search via OR.
- **Analytics (issue #114)**: ranking lists (Usage Breakdown cards + Channel × Model) now support sorting by success rate, request count, or cost, with ascending/descending toggle — pure frontend sort, no backend changes.
- **Home**: the home stats chart gained an "All" time range option.
- **Analytics**: share snapshot now supports selecting more data sections with custom checkboxes.

### 🐛 Bug Fixes
- **Backup (issue #118)**: after migrating from SQLite to MySQL/Postgres, the old SQLite connection is now closed and the `.db` / `.db-wal` / `.db-shm` files are deleted, preventing the process from getting stuck in D state (IO blocked) due to continued reads on the stale SQLite file.
- **Ops**: fixed the audit-detail dialog being clipped when zoomed.

## [v2.2.3] - 2026-06-27

### 🚀 Features
- **Group (issue #113)**: group-test-failed channels now get a greyed-out card marker so degraded members are visible at a glance.
- **Setting**: trusted-proxy CIDR segments are now configurable from the Settings UI page (previously config-file / env only).

### 🐛 Bug Fixes
- **Backup (issue #112)**: cross-DB migration import now temporarily disables the target session's foreign-key checks, fixing `MySQL Error 1452` / Postgres `23503` / SQLite `787` caused by orphaned child-table rows (the source DB — especially SQLite with historically-off FK enforcement — may carry stale children whose parent was deleted).
- **Backup**: export now includes the proxy field and redacts it for the `viewer` role, matching the other masked domain fields.
- **Backup**: full restore now preserves the existing admin account and migrates site-management data instead of dropping it.

## [v2.2.2-fix] - 2026-06-26

### 🐛 Bug Fixes
- **Backup**: `deleteAll` now quotes table names with dialect-aware identifiers, fixing MySQL reserved-word errors (e.g. `usage`) during full restore.
- **DB**: migration `AddColumn` switched to the GORM Migrator API to avoid MySQL reserved-word errors on column adds.
- **CI**: release workflow now explicitly sets `make_latest` so a `-fix` build becomes the GitHub `latest` release.

## [v2.2.2] - 2026-06-26

### 🚀 Features
- **Setting**: database-migration form split into structured fields (type / host / port / db / user / pass) with live DSN auto-generation, replacing the raw DSN textarea.

### 🐛 Bug Fixes
- **Log**: optimized relay-log read/write performance (DB-side `EXISTS` subqueries, composite indexes `idx_relay_logs_channel_time` / `idx_relay_logs_apikey_time`, migration `019`) and added a `relay_log_content_enabled` toggle to strip large request/response bodies from log writes; `SemanticCacheHit` and `CacheReadTokens` are still derived without the large fields.
- **DB**: accept `mysql://` URL-format DSNs and auto-wrap host:port in `tcp(...)` for the MySQL driver.
- **Model**: added `size` to indexed string fields to avoid MySQL `BLOB/TEXT` index errors under `utf8mb4`.
- **WebAuthn**: pre-checks RP ID against the request domain and classifies `credential manager` errors; explicitly sets `UserVerification=preferred` to fix Android passkey failures.
- **Audit**: added `DELETE /api/v1/log/clear-contents` to the audit exemption allowlist.

## [v2.2.1] - 2026-06-25

### 🚀 Features
- **Relay**: new key-availability selection strategy — scores each `(channelID, keyID, model)` key by error type (401/403 = −100, 429 = −15, 5xx/timeout = −10, network = −5; success = +5; time-based lazy recovery), falling back to lowest-cost when a key's score ≤ 0. In-memory only, not persisted.
- **Relay**: empty model output now triggers automatic retry — both the streaming and non-streaming paths.
- **APIKey**: keys can now cap per-model Token usage, and the total consumed is shown on the log page.

### 🐛 Bug Fixes
- **Server**: added a `trusted_proxies` config option (CIDR list; `*` = trust all) restoring real client IPs behind a reverse proxy, after `C-01` disabled trusted proxies by default. Docs updated.
- **APIKey**: corrected the `b` (billion) token-shorthand multiplier.

### 🛠 Optimizations
- **Analytics**: rewrote the channel×model legacy aggregation branch from full-row in-memory merge (LEFT JOIN anti-join + per-row Go-layer merge of `[]RelayLog` with JSON attempt blobs, no `LIMIT`, no DB-side `GROUP BY`) to DB-side `GROUP BY` aggregation, eliminating the memory explosion / disk-read spike on large ranges; added a toggleable query cache.
- **Analytics/ratelimitstore**: purged the unbounded global `requestBuckets` / `tokenBuckets` maps (keyed by `apiKeyID:modelName`, model name is client-controlled) via periodic `PurgeStaleBuckets`; `RemoveAPIKeyBuckets` runs on key deletion. Replaced `loadLatencyDistribution`'s full-table `.Find()` + in-memory sort percentile with a single DB-side `COUNT`/`SUM`/`CASE WHEN` aggregation + linear bucket interpolation.
- **Frontend**: removed `refetchOnMount:'always'` from analytics/ops/channel/group endpoints so the global 60s `staleTime` applies; rewrote the content loader to keep-alive non-active routes (`display:none` + `inert`) instead of unmounting on tab switch, fixing slow re-loads, loading flicker, and long-session memory crashes. WebAuthn passkey registration now guards on `window.isSecureContext` and classifies `SecurityError`/`NotAllowedError` with an amber banner on insecure origins.

## [v2.2.0] - 2026-06-24

### 🚀 Features
- **Channel (issue #98)**: per-channel "skip model availability test" toggle (`skip_model_test`) so channels that penalize low-byte probe requests (deduct quota / ban) can opt out of group/model availability probes. Migration `016` adds the column idempotently across SQLite/MySQL/Postgres.
- **Relay (issue #94)**: key cooldown moved from per-key to per-`(keyID, model)` granularity so a single model's 429 no longer drags down the same key's other models. Expired cooldowns are purged by the relay-log periodic task. The `model` package receives the cooldown query via an init-time injection (breaking the `model → balancer` import cycle).
- **Relay (issue #95)**: per-channel retry count can now be set to `0` (try once, then move to the next channel). Route-level retry count is exposed on the Ops Maintenance panel, and the max-total-attempts check now counts only real upstream forwards (cooldown/circuit-breaker skips no longer consume the quota). Relay-log attempt badges distinguish skipped vs. real-failed attempts and show a separate "forwarded" count.
- **Relay (issue #93)**: error logs now append the raw upstream error response body (status + body) to the decision summary, so 429s etc. can be distinguished by cause (resource exhaustion vs. RPM limit) instead of a generic "rate limited" message. Applied to both LLM and media relay paths.
- **Group**: outbound format gained `chat_only` and `responses_only` modes that disable cross-format adapter fallback entirely — useful for upstreams (e.g. public-welfare relays) that reject the other format with 400/404.
- **Model normalization**: model-name normalization rules (router prefixes, functional suffixes, explicit variant→canonical mappings) are now runtime-configurable via DB settings and a new Settings "Normalize" card, with an offline AI-assisted normalization workflow (export channel model variants → analyze → upload rules). A default-dedupe toggle and AI analysis prompt were added.
- **Web**: revamped model market UI — rename, collapsible filter, toolbar toggles; group edit dialog moved the availability-test area to a right-side panel; small-screen overview cards compacted to a 2×2 grid.

### 🐛 Bug Fixes
- **Issue #97** (low-memory SQLite IO): the SQLite per-connection PRAGMAs (`cache_size`, `mmap_size`, `synchronous`, `foreign_keys`, `auto_vacuum`, `journal_mode`) were previously emitted as `_cache_size=...`-style DSN keys, which the `glebarez/go-sqlite` driver silently ignores (it only honors `_pragma`/`_txlock`/`_time_format`/`vfs`). As a result every PRAGMA stayed at the SQLite default — `cache_size` ≈ 2 MB, `synchronous=FULL` — so on memory-constrained hosts (e.g. 1.6 GB) the page cache was far smaller than the database and idle IO stayed high. PRAGMAs are now emitted as `_pragma=xxx(yyy)`, so they actually take effect, and two are exposed via `config.json`: `database.sqlite.cache_size` (default `-20000` ≈ 20 MB) and `database.sqlite.mmap_size` (default `268435456` ≈ 256 MB).
- **Security hardening (2C-01)**: refuse to start without a persistent encryption key — `cmd/start.go` now fails fast when `security.encryption_key` is unset instead of silently falling back, preventing data loss on restart.
- **Security hardening**: disabled trusted proxies by default to prevent IP spoofing (C-01); protected `adminCache` with an RWMutex (C-03); constrained SQLite migration paths to the data directory (2C-03); validated WebDAV backup filenames and `base_url` against path traversal / SSRF (2C-02, C-05); limited site import payload size to prevent memory exhaustion (C-07); applied rate limiting to the WebAuthn login/begin endpoint (C-18); returned an error from `GenerateAPIKey` on rand failure (C-16).
- **Relay**: guarded against nil inbound adapter to prevent panic (C-13); wrote response body to singleflight shared callers on cache miss (4C-01); corrected unicode byte offset in the response filter (H-04); protected trend snapshots with an RWMutex (C-14); circuit breaker no longer resets the cooldown timer on failure in the Open state (C-04).
- **Gemini**: used function name instead of tool call id in `FunctionResponse` (C-12).
- **Hub/sapi**: test initialization now configures the encryption key, fixing "encryption key not configured" (268ad5a).
- **Analytics (issue #87, #101, #103)**: channel×model now aggregates historical logs to fix pre-deployment data disappearing; retry-failed channels now display and duplicate titles are removed; usage-distribution-by-model empty data and top-five-only share bars fixed. Log cleanup now also deletes `relay_log_attempts` rows to prevent orphan rows from making stats vanish.
- **Notify/alert**: Feishu error detection now uses OR semantics, and deleted channels no longer trigger "channel down" alerts (H-24, H-27).
- **Sitesync**: removed variable shadowing in `createSub2APIToken` (3M-01).
- **Docs**: clarified that the semantic cache is exact-match, not semantic (H-42); synced README to the latest module structure.
- **Web**: normalized `data:null` to `undefined` to prevent destructuring defaults from failing; home chart metric selection restricted to single and switched to `ComposedChart` so multi-metric Line curves render; added an entry button for the archived-sites dialog; i18n `outboundFormat` option keys corrected to camelCase; added missing `ops.quota.fields/actions` keys and guarded empty runtime status; fixed `modelFilter.capability` namespace title; fixed `log.card.requestTypeLabels` returning an empty key for unlabeled endpoints.
- **WebDAV**: immediate backup no longer depends on the auto-backup switch; fixed empty-list `data:null` crash and multiple cloud-backup issues (#91).
- **Hub**: narrowed token-cache lock granularity and deduplicated logins via `singleflight`.

## [v2.1.6] - 2026-06-18

### 🚀 Features
- **Analytics (issue #87)**: reorganize data presentation across the management UI.
  - Analytics default tab switched to "Channel × Model"; usage-distribution share chart added (top-N by model / channel×model, cost/count/tokens metrics).
  - Quota page merged with API key detail (added total tokens + success rate + "view key detail" jump).
  - Group list cards show health badges; ops Health → analytics route-health jump.
  - "Available endpoints" view migrated from API key page to model market.
  - Ops center gains a "Maintenance" tab (circuit breaker / retry / response filter); setting page slimmed from 18 → 12 entries (site automation → hub automation tab; purge-unavailable / route-group-danger → group maintenance dropdown).
  - Provider health table: sortable columns + mini bar charts.
  - Model market: multi-dim filter (capability + provider + normalized-name dedupe, e.g. `kimi-k2.5` / `moonshotai/kimi-k2.5` / `dmxapi-kimi-k2.5-cc` merge).
  - Utilization renamed to "Usage Breakdown" with no-billing hint.
  - Home chart: multi-metric overlay (multi-select cost/count/tokens/success-rate on one chart).

### 🐛 Bug Fixes
- **Issue #90**: test-availability logs no longer show input/output/cost as "unknown" — `sendGroupProbeRequest` now returns the parsed `InternalLLMResponse`, and `recordTestLog` populates `InputTokens`/`OutputTokens`/`CacheReadTokens`/`Cost` using the same `price.GetLLMPrice` calculation as the relay pipeline.
- **CI**: `TestAllManagementWriteRoutesAreAudited` fixed by adding the read-only `POST /api/v1/channel/test-model` probe route to the audit exemption list.

## [v2.0.5] - 2026-06

### 🚀 Features
- Add Agnes video generation type support with provider-specific path rewrite (`/v1/videos`).
- Add MiMo TTS provider support for audio speech — converts OpenAI TTS requests to MiMo Chat Completions format and extracts base64 audio from JSON responses.
- Display adapter type and request type in frontend log detail and tooltips.
- Improve relay log readability: adapter type names, fallback path logging, and semantic cache hit indicators.

### 🐛 Bug Fixes
- Adapter fallback now always prefers Response adapter first to leverage upstream prompt caching.
- Add Chat adapter fallback for Responses API requests that previously failed with `convert_request_failed`.
- Fix circuit breaker integer overflow, sticky session memory leak, and streaming protocol violations.
- Fix Responses channel migration: circuit breaker false-positive trigger and unclosed stream sessions.
- Preserve relay log channel info on inflight request reuse.
- Restore chat fallback for response channels.
- Fix streaming disconnect falsely reported as success, missing media relay condition evaluation, and context inconsistency.
- Fix last-channel info loss when all media relay attempts fail with `ScopeAbortAll`.
- Hub adapters: replace `http.DefaultClient` with 30s timeout client and clean up token caches.
- Server handlers: add missing permission middleware, restructure error handling, and add WebDAV timeout.
- Transformer modules: fix nil-panic in function_call_output, content_block_stop ordering, and response.completed event ordering.
- Semantic cache: fix global cache pointer race condition with RWMutex.
- Rate limiter: fix division by zero in ResetAt.
- Task shutdown: wait for in-flight tasks with WaitGroup.

### ⚠️ Upgrade Notes
- Agnes video generation and MiMo TTS require the group's Endpoint Provider to be set to `agnes` or `mimo` respectively. Standard video/audio_speech endpoints continue to work without changes.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v2.0.4...v2.0.5

---

## [v2.0.4] - 2026-06

### 🐛 Bug Fixes
- Fix small-screen bottom navigation content overlap.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v2.0.3...v2.0.4

---

## [v2.0.3] - 2026-06

### 🚀 Features
- Add zashboard-style collapsible group list view for the group management page.
- Inject channel `param_override` into outbound relay requests for per-channel parameter customization.

### 🐛 Bug Fixes
- Write stream responses to semantic cache for streaming SSE replay support.
- Prune expired semantic-cache entries in `Stats()` for accurate size reporting.
- Cancel upstream request on client stream disconnect to avoid wasted resources.
- Stop injecting default `max_completion_tokens` for reasoning models in the outbound transformer.
- Fix group edit dialog sizing and horizontal overflow issues.
- Fix group edit dialog and site management overview display issues.
- Improve mobile API key form layout.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v2.0.2...v2.0.3

---

## [v2.0.2] - 2026-06

### 🚀 Features
- Update Hub module workflows for stream-session resilience and viewer-safe management surfaces.
- Hub-related management data now masks domains for viewer accounts across sites, remote sites, credentials, channels, and URL settings.

### 🐛 Bug Fixes
- Enable semantic cache for streaming requests, including SSE cache-hit replay and stable stream-session recovery without explicit `conversation_id`.
- Preserve semantic-cache entries across unchanged runtime config refreshes.

### ⚠️ Upgrade Notes
- The Hub module has been updated. For security and consistency, please re-enter Hub site domains/Base URLs and related credentials if you need to edit or refresh them after upgrading.
- Viewer accounts will see masked domains (`***`) and should ask an admin/editor to re-enter or confirm Hub connection details when needed.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v2.0.1...v2.0.2

---

## [v2.0.0] - 2026-05

### 🚀 Features
- Hub navigation overhaul: merge five standalone modules (Announcement, Check-in, Redemption, Usage History, Credential) into Hub as tab panels, reducing top-level navigation from 18 to 13 items.
- Hub tab interface with six tabs: Sites, Check-in, Announcement, Redemption, Usage, and Credential.

### 🐛 Bug Fixes
- Fix FetchTokens pagination in common Hub adapter — tokens beyond the first page of 100 are now correctly retrieved.
- Add 13 missing StatsMetrics fields (latency percentiles, FTUT metrics, histogram counts) to all stats formatting functions to resolve TypeScript compilation errors.

### 🛠 Optimizations/Refactor
- Remove orphaned TokenManager frontend component and remote-site-token API hooks (dead code after Hub navigation merge).
- Remove 12 orphaned i18n keys across all three locales (en, zh_hans, zh_hant).
- Bump version to v2.0.0 (version.go, package.json, docker-compose.yml).

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.8...v2.0.0

---

## [v1.9.8] - 2026-05

### 🚀 Features
- Refine custom base URL suffix handling for upstream compatibility.

### 🐛 Bug Fixes
- Restore scrolling in the channel detail dialog overlay.
- Preserve custom OpenAI version root endpoints when saving upstream URLs.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.7...v1.9.8

## [v1.9.7] - 2026-05

### 🐛 Bug Fixes
- Clarify missing prompt cache trend usage signals.
- Show unknown usage when upstream usage data is missing.
- Restore scrolling in morphing dialog overlays.
- Preserve explicit OpenAI upstream endpoints.
- Remove update package size limit for release downloads.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.6...v1.9.7

---
## [v1.9.6] - 2026-05

### 🚀 Features
- Align management-side API contracts and tighten backend input boundaries.

### 🐛 Bug Fixes
- Keep edit and delete actions visible at the bottom of the mobile dialog.
- Preserve SuffixMode in ChannelBaseUrlDelayUpdate.
- Fix telemetry cache stats and deep-link generation in the ops center.
- Fix cache token metric unit loss in the analytics center.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.5...v1.9.6

---
## [v1.9.5] - 2026-05

### 🚀 Features
- Include circuit breaker runtime state in backup export and restore workflows.

### 🐛 Bug Fixes
- Allow API key IP allowlists to match client addresses that include ports.
- Allow the request origin in CSP `connect-src` for separated management console deployments.
- Fix telemetry latency percentile calculations so empty samples do not skew p95.
- Fix provider prompt cache token metric unit formatting in the ops dashboard.

### 🛠 Optimizations/Refactor
- Add OCI image metadata labels for Docker image version, revision, and build time.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.4...v1.9.5

---

## [v1.9.4] - 2026-05

### 🚀 Features
- Complete backup/restore overhaul.
- Add client IP logging and API key IP restriction.
- Expand endpoint provider rewrites and improve relay hardening.
- Add CCswitch deep link generator to group toolbar.

### 🐛 Bug Fixes
- Cap relay logs and audit logs backup export at 500k rows to prevent OOM.
- Hide availability test for non-conversation endpoint types.
- Resolve frontend-backend type mismatches.
- Add labels and contextual hints to alert rule form fields.
- Fix telemetry runtime metrics imports and wire session, sticky, quota alert, and quota monitor stats into the ops dashboard.
- Auto-detect git commit and build time for local builds.
- Include semantic cache hits and Anthropic cache writes in provider prompt cache trends.
- Remove redundant token unit suffix formatting in the UI.

### 🛠 Optimizations/Refactor
- Cap model picker height for better channel browsing ergonomics.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.3...v1.9.4

---

## [v1.9.3] - 2026-05

### 🚀 Features
- Support endpoint-provider specific rewrites for group relay and media passthrough.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.2...v1.9.3

---

## [v1.9.2] - 2026-05

### 🚀 Features
- Add model capabilities API and endpoint-aware model discovery.
- Add Model Market and Capabilities dual-view workflow in the management UI.
- Add endpoint capability aggregation and validation for group and relay model listing.

### 🐛 Bug Fixes
- Filter out groups without valid items or enabled channels when listing models.
- Narrow `*` group items by endpoint capability during media relay to avoid invalid upstream routing.

### 🛠 Optimizations/Refactor
- Refine model market summary trigger, capabilities filtering, and locale coverage.
- Add clearer logs for model-not-found relay failures.
- Refresh README model discovery and capabilities documentation.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.1...v1.9.2

---

## [v1.9.1] - 2026-05

### 🚀 Features
- Add architecture-focused Markdown refreshes for the current management console, relay pipeline, and embedded UI workflow.

### 🐛 Bug Fixes
- Correct hourly statistics keying and analytics time boundaries.
- Fix setting-cache backward compatibility while refreshing architecture documentation.

### 🛠 Optimizations/Refactor
- Split `internal/op/*` business logic into domain packages for AI routing, analytics, API keys, audit, backup, cache usage, channel, group, LLM metadata, nav order, ops, rate-limit state, relay logs, settings, stats, and users.
- Ignore local development log files generated during backend and frontend verification.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.0...v1.9.1

---

## [v1.8.9] - 2025-04

### 🚀 Features
- Switch channel page to current-group view (@henryz78)

### 🐛 Bug Fixes
- Improve mimo reasoning budget handling (@lingyuins)
- Support removing failed models after group test (@lingyuins)
- Optimize group availability checks and channel group list (@lingyuins)
- Preserve renamed default group label (@henryz78)
- Avoid stretched group action button on mobile (@henryz78)
- Restore group dialog shell and cache trend layout (@henryz78)
- Refine channel dialog and cache trend view (@henryz78)
- Refine channel group labels and summary layout (@henryz78)
- Improve responsive stats cards (@henryz78)
- Avoid empty state card overflow on mobile (@henryz78)
- Improve mobile page scroll layout (@henryz78)

### 🛠 Optimizations/Refactor
- Stop tracking superpowers specs (@lingyuins)

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.8.8...v1.8.9

---

## [v1.8.7] - 2025-04

### 🚀 Features
- Add management groups (@henryz78)
- Move sync actions out of settings (@henryz78)
- Refine dashboard ranking and cache layout (@Lingyu)

### 🐛 Bug Fixes
- Sync prices after manual refresh (@henryz78)
- Align mobile preset picker actions (@henryz78)
- Improve error messages and mobile channel dialog (@henryz78)

### 🛠 Optimizations/Refactor
- Merge page sync actions pull request (@LingyuIns)

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.8.6...v1.8.7

---

> **Note:** Earlier releases (v1.8.6 and below) are not recorded in this changelog.
> See the [GitHub Releases](https://github.com/lingyuins/octopus/releases) for the full history.

