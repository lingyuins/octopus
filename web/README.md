# Octopus Web Console

This directory contains the management UI for Octopus.

## Stack

- Next.js 16
- React 19
- TypeScript
- Tailwind CSS 4
- TanStack Query
- Zustand 5
- Radix UI
- `next-intl`

The app uses App Router as the shell entrypoint, but the actual screen switching inside the console is handled client-side in `src/components/app.tsx`.

## Commands

Install dependencies:

```bash
pnpm install
```

Run the frontend against a local backend:

```bash
NEXT_PUBLIC_API_BASE_URL="http://127.0.0.1:8080" pnpm dev
```

Lint:

```bash
pnpm lint
```

Tests (i18n key parity + unit tests; unit tests use an explicit file list registered in `package.json`, so new `*.test.ts` files must be appended there manually):

```bash
pnpm test:i18n
pnpm test:unit
```

Full check (lint + tests + static export build):

```bash
pnpm check
```

Build the static export used by the embedded management UI:

```bash
NEXT_PUBLIC_APP_VERSION="$(git describe --tags --always 2>/dev/null || printf 'dev')" pnpm build
```

## Environment Variables

- `NEXT_PUBLIC_API_BASE_URL`: Optional API base URL. Defaults to relative requests against the current origin.
- `NEXT_PUBLIC_APP_VERSION`: Version string shown in the UI. For release builds, set this to the current git tag or commit. When unset, `next.config.ts` falls back to reading the `Version` from `../internal/conf/version.go` (run the build from this `web/` directory for that fallback to resolve).

## Lockfiles

This directory keeps its own `pnpm-lock.yaml`. CI (`quality.yml`) and the top-level `Dockerfile` install with `pnpm install --frozen-lockfile` against it, so dependency changes must update this lockfile (a root-level `pnpm-lock.yaml` also exists for the dev toolchain at the repo root; the frontend dependency tree of record is the one in `web/`).

## Output and Embedding

`pnpm build` produces a static export in `out/`.

The Go server embeds these files from `../static/out/` (`static/static.go`, `//go:embed all:out`). Note that a fresh checkout has no `static/out/` directory at all — create it before any Go build, otherwise `go build` / `go test` / `go run` fail on the embed directive (CI does exactly this with `mkdir -p static/out && touch static/out/.keep`).

A typical local embed flow is:

```bash
pnpm install
NEXT_PUBLIC_APP_VERSION="$(git describe --tags --always 2>/dev/null || printf 'dev')" pnpm build
cd ..
# Replace the previously generated embed directory (static/out is gitignored build
# output; web/out is kept as-is, so the export does not need to be re-run)
rm -rf static/out
mkdir -p static/out
cp -r web/out/. static/out/
# go:embed fails on an empty directory; if Next.js exported an empty _not-found, add a placeholder
if [ -d web/out/_not-found ] && [ -z "$(ls -A web/out/_not-found)" ]; then
  touch static/out/_not-found/.keep
fi
```

For local development you can skip the copy entirely: with `OCTOPUS_DEBUG=true`, the Go server serves `web/out` or `static/out` straight from disk (whichever has an `index.html`), so no embedding step is needed.

The top-level `Dockerfile` already builds this frontend and copies the export into `static/out` during image build, so release images contain the matching frontend automatically.

## Key Directories

- `src/components/app.tsx`: Main application shell with login mode switching (user credentials / API key)
- `src/components/modules/home/*`: Runtime/version overview, hero summary, trend chart (multi-metric overlay: cost/count/tokens/success-rate), GitHub-style activity heatmap, ranking panel, and analytics overview cards
- `src/components/modules/remote-site/*`: Hub module — tab-based interface: Sites (multi-account cards with inline balance / sync / check-in status, archive/restore, batch edit, bulk import), Site Channels (SiteChannelSection), Automation (SettingSiteAutomation), Balance (plan balance charts), and TokenPlan (token plan monitoring). Check-in/announcement/redemption/usage/credential panels are inlined into the Site cards or accessed via the API
- `src/components/modules/site/*`: Site management module — the multi-account site card grid rendered inside the Hub "Sites" tab. Includes `Site` (main grid + site/account dialogs), `CheckinPanel` (summary strip), `AccountEditDialog`, `SiteEditDialog`, `checkin-status`, `site-message`, and `ui-store`. Supports pin / archive / restore, batch enable/disable/delete, and bulk import from AllAPIHub / MetAPI
- `src/components/modules/site-channel/*`: Site Channels section — dedicated view for managing channels associated with remote sites (projected channel bindings)
- `src/components/modules/credential/*`: Shared `CredentialDialog` component reused by the Hub CredentialPanel
- `src/components/modules/model-mapping/*`: Model name mapping rules UI (exact/wildcard/regex pattern-based name rewriting with priority and group scope). Not registered as a top-level nav route; backed by `/api/v1/model-mapping`
- `src/components/modules/proxy-pool/*`: Proxy configuration pool management — CRUD, connectivity testing, reference tree tracking, and jump-to-reference navigation. Accessible from the app shell toolbar
- `src/components/modules/channel/*`: Channel configuration with built-in quick-create templates (OpenAI — creates an OpenAI Responses channel, Anthropic, Gemini, DeepSeek, OpenRouter, SiliconFlow, Volcengine, MiMo), key management, sync, latency, model declarations, proxy mode (direct/system/pool/inherit), request rewrite profiles (`preserve` / `openai_chat_compat` / `codex` with header, tool-role, and system-message strategies), `param_override` JSON for per-channel parameter injection, and a per-channel `skip_model_test` toggle (issue #98) that excludes the channel from group/model availability probes
- `src/components/modules/pool/*`: Account Pool management — pool list with keyword search and create/edit dialogs (name, description, scheduling strategy, default concurrency, cooldown, enabled), a pool detail view with account search plus platform/status filters (desktop table, mobile cards), OAuth account authorization, batch operations, import/export dialogs, and delete confirmations; the credential export dialog warns that the output contains live credentials
- `src/components/modules/group/*`: Route groups, balancing strategy configuration, zashboard-style collapsible group list, group testing, AI route generation (full-table from the route page button; single-group append from the edit dialog), AI route progress dialog, group thinking mode (`auto` / `off` / `on`, persisted on create/update), endpoint provider configuration, outbound format (`chat_only` / `responses_only` disable cross-format fallback), a Maintenance dropdown button (Purge Unavailable Models, Delete All Route Groups), and CC Switch deep link generator (Claude Code, Codex, Gemini, OpenCode, OpenClaw)
- `src/components/modules/model/*`: Model Market UI with toolbar views (Market cards, Available Endpoints migrated from the API key page, Price Categories with peak/off-peak billing), a toolbar summary strip (full-dataset metrics + price refresh), virtualized responsive cards, multi-dimension filter (search + capability + provider + priced/free + normalized-name dedupe) with live result count and one-click reset, strict price-validation edit / delete dialogs, endpoint groups merged by conversation family sharing the toolbar search, and price-category / peak-schedule rule dialogs with client-side validation
- `src/components/modules/analytics/*`: Tabs for Channel × Model (default), Usage Breakdown, Route Health, Latency, Evaluation, and Cache, with usage-distribution share chart, latency distribution histogram, provider prompt cache analytics, and share snapshot (PNG export / clipboard copy via html-to-image)
- `src/components/modules/log/*`: Dual views — a relay request list/detail view with Group / Request Body tabs beside the response viewer, model/channel candidate rows, expandable attempt diagnostics, and a usage/cost footer, plus an `ErrorLogView` for backend-recorded errors. Candidate statuses and cooldown seconds come from recorded attempts; current exact-name group items supplement the list as unrecorded entries, not historical or live health snapshots.
- `src/components/modules/notification/*`: Unified notification center — top-level route aggregating system events, alert firings, and plan notifications. Groups: Messages (inbox / archived), Alerts (rules / history via `AlertSections`), Delivery (channels / policies / preferences), and Reports (schedules / history). Renders notification text via `notif-text.ts` and streams updates over SSE
- `src/components/modules/plan-provider/*`: Plan provider monitoring section — tracks upstream subscription quota/usage (Codex, MiMo, StepFun, SenseNova, and balance-type providers) and manages auto-created forwarding channels
- `src/components/modules/report/*`: Usage report scheduling (`ReportScheduleManager`) and report history (`ReportHistoryList`), rendered inside the Notification reports group
- `src/components/modules/alert/*`: Alert rule, notification-channel, and history UI. No longer a top-level route — its `AlertSections` component is embedded inside the Notification module's Alerts and Delivery groups
- `src/components/modules/ops/*`: Tabs for Telemetry, Quota, Health, Maintenance, System, and Audit. Telemetry includes hero metrics, P95 latency, throughput RPS, database health, session/quota activity, semantic cache snapshot, provider health table (sortable columns + mini bar charts), and provider prompt cache analytics. Maintenance tab consolidates Retry / Circuit Breaker / Response Filter settings (moved out of Settings page)
- `src/components/modules/apikey/*`: API key creation, allowlists, expiry, max-cost, RPM/TPM, per-model quota, and IP/CIDR allowlist controls
- `src/components/modules/apikey-dashboard/*`: API key dashboard views — request stats, token usage, cost, quota, expiration countdown, and supported models when authenticated via API key
- `src/components/modules/setting/*`: Settings cards — Info (version, self-update, version mismatch detection), Appearance (theme, locale, nav order + visibility), AI Route, Auto Strategy, Account (timezone, session), Semantic Cache, Log, System (CORS allowlist, proxy, stats interval), LLM Sync, Backup (export/import/live migration), Redis (optional cache backend, restart to apply), WebDAV Backup, WebAuthn/Passkey, Normalize (model-name normalization rules: router prefixes, functional suffixes, explicit variant→canonical mappings, with an offline AI-assisted workflow), Pool (account-pool scheduling tuning), and Proxy Pool (opens the shared proxy-pool dialog instead of an inline panel). The authoritative card list lives in `SETTING_ITEM_DEFS` in `src/components/modules/setting/index.tsx`. Supports drag-and-drop card reordering with localStorage persistence. Note: Retry/CircuitBreaker/ResponseFilter were moved to `ops/Maintenance.tsx`; SiteAutomation is rendered inside the Hub Automation tab; PurgeUnavailableModels/RouteGroupDanger moved to the Group Maintenance button
- `src/components/modules/user/*`: Management-console user and role administration
- `src/components/modules/navbar/*`: Top-level navigation state and persisted nav-order/visibility helpers
- `src/components/modules/toolbar/*`: Shared toolbar with per-page search, layout toggle (grid/list), sort options, and context-dependent filters
- `src/components/modules/login/*`: Login form supporting both username/password and API-key authentication modes behind a tabbed interface; both the login screen and user management support WebAuthn/Passkey registration and authentication
- `src/components/modules/logo/*`: Shared animated Octopus logo component used by login and first-run screens
- `src/components/modules/first-run-setup.tsx`: Bootstrap wizard for creating the initial admin account when no admin exists, with animated particle background
- `src/api/`: API client and endpoint hooks (TanStack Query)
- `src/route/config.tsx`: UI route registration (lazy-loaded top-level modules: home, hub, channel, pool, group, model, analytics, log, notification, ops, apikey, setting, user)
- `src/stores/`: Zustand state stores
- `src/lib/`: Utilities, i18n, logger, time zone helpers, service worker management
- `public/locale/`: Localized text resources (en, zh_hans, zh_hant)

## Notes

Cross-cutting constraints and conventions (module structure lives in Key Directories above):

- Top-level routes stay mounted when you switch pages (keep-alive rendering in `src/route/content-loader.tsx`): components are not unmounted, so `useEffect` cleanups do not fire on tab switches. Gate polling queries to the active module/view instead of relying on unmount.
- Group mode labels and endpoint type display values are shared with backend behavior and should be updated together when adding new strategies or capabilities.
- API types in `src/api/` are hand-maintained — there is no OpenAPI/code generation, so backend DTO changes must be mirrored manually.
- i18n keys must be added to all three locale files (`public/locale/en.json`, `zh_hans.json`, `zh_hant.json`); `pnpm test:i18n` enforces key parity in both directions.
- User-visible time formatting in new code goes through `@/lib/time` (`Intl.DateTimeFormat` + the user-configurable time zone from `src/stores/setting.ts`, independent from the server-side stats timezone). Legacy components may still use local formatting, so coverage across the UI is partial.
- The build is a static export (`output: "export"`, no SSR, `images.unoptimized`); non-dev builds use a relative `assetPrefix` so the Go server can host the UI under any prefix.
- The `Model Market` views are backed by `/api/v1/model/market`, `/api/v1/model/capabilities`, and `/api/v1/model/price-category/*` + `/api/v1/model/price-schedule/*`. The market summary-strip metrics come from the market endpoint's full-dataset `summary` and do not change with search or filters. Scheduled price refresh lives in the Settings `LLM Sync` card.
- The `analytics` overview query remains available, but the primary UI summary cards for those metrics live on the Home page.
- Ops `Audit` only covers selected management write routes, not public relay traffic.
- Semantic cache settings are split into configured state and runtime-enabled state: enabling the switch alone is not enough; the embedding base URL and embedding model must also be configured before runtime metrics turn green.
- Top-level page order and visibility are edited inside the `Appearance` card, persisted through the `nav_order` and `nav_visible` settings, and normalized against `DEFAULT_NAV_ORDER`, so missing routes are appended automatically and unknown routes are dropped.
- Group `thinking_mode` forced modes (`off` / `on`) require upstream model support; raw/passthrough channels leave the body unchanged and skip the policy.
- Viewer accounts see masked domains (`***`) for Hub-related management data across sites, remote sites, credentials, channels, and URL settings.
- The AI Route setting is the default target group for the single-group compatibility flow, not the target for full-table generation.
- When backend API surfaces or top-level modules change, update both `web/README.md` and the root README files so the embedded-console docs remain consistent.
