# Octopus Web Console

This directory contains the management UI for Octopus.

## Stack

- Next.js 16
- React 19
- TypeScript
- Tailwind CSS 4
- TanStack Query
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

Build the static export used by the embedded management UI:

```bash
NEXT_PUBLIC_APP_VERSION="$(git describe --tags --always 2>/dev/null || printf 'dev')" pnpm build
```

## Environment Variables

- `NEXT_PUBLIC_API_BASE_URL`: Optional API base URL. Defaults to relative requests against the current origin.
- `NEXT_PUBLIC_APP_VERSION`: Version string shown in the UI. For release builds, set this to the current git tag or commit.

## Output and Embedding

`pnpm build` produces a static export in `out/`.

The Go server embeds these files from `../static/out/`. A typical local embed flow is:

```bash
pnpm install
NEXT_PUBLIC_APP_VERSION="$(git describe --tags --always 2>/dev/null || printf 'dev')" pnpm build
cd ..
mkdir -p static/out
cp -r web/out/* static/out/
```

If `static/out/_not-found/` exists but is empty, add `.keep` before running `go build` or `go run main.go start`.

The top-level `Dockerfile` already builds this frontend and copies the export into `static/out` during image build, so release images contain the matching frontend automatically.

## Key Directories

- `src/components/app.tsx`: Main application shell with login mode switching (user credentials / API key)
- `src/components/modules/home/*`: Runtime/version overview, hero summary, trend chart, GitHub-style activity heatmap, ranking panel, and analytics overview cards
- `src/components/modules/remote-site/*`: Hub module — tab-based interface with panels: Sites (SitesPanel with BalanceChart and prediction), Check-in (CheckInPanel), Announcement (AnnouncementPanel), Redemption (RedemptionPanel), Usage History (UsageHistoryPanel), Credential (CredentialPanel), and Site Channels. The formerly standalone announcement, checkin, redemption, usage-history, and credential modules have been merged here as tab panels
- `src/components/modules/site/*`: Site management module — upstream relay platform management with multi-account support, projected channels, auto-sync, and auto-checkin
- `src/components/modules/site-channel/*`: Site Channels section — dedicated view for managing channels associated with remote sites
- `src/components/modules/credential/*`: Shared `CredentialDialog` component reused by the Hub CredentialPanel
- `src/components/modules/model-mapping/*`: Model name mapping rules UI (exact/wildcard/regex pattern-based name rewriting with priority and group scope). Not registered as a top-level nav route; backed by `/api/v1/model-mapping`
- `src/components/modules/proxy-pool/*`: Proxy configuration pool management — CRUD, connectivity testing, reference tree tracking, and jump-to-reference navigation. Accessible from the app shell toolbar
- `src/components/modules/channel/*`: Channel configuration with 9 built-in templates (OpenAI, OpenAI Responses, Anthropic, Gemini, DeepSeek, OpenRouter, SiliconFlow, Volcengine, MiMo), key management, sync, latency, model declarations, proxy mode, and request rewrite profiles
- `src/components/modules/group/*`: Route groups, balancing strategy configuration, zashboard-style collapsible group list, group testing, single-group AI route append flow, AI route progress dialog, endpoint provider configuration, and CC Switch deep link generator (Claude Code, Codex, Gemini, OpenCode, OpenClaw)
- `src/components/modules/model/*`: Model Market UI with dual-tab interface (Market and Capabilities), including summary strip, virtualized cards, price-edit actions, and capabilities panel with endpoint support and status badges
- `src/components/modules/analytics/*`: Five tabs (Cache, Utilization, Route Health, Evaluation, Latency) with latency distribution histogram, provider prompt cache analytics, and share snapshot (PNG export / clipboard copy via html-to-image)
- `src/components/modules/log/*`: Relay request list/detail views, usage/cost display, and error diagnostics
- `src/components/modules/alert/*`: Alert rules, notification channels (webhook, Gotify, email, Telegram, Feishu, DingTalk, WeCom, ntfy), state records, and history views
- `src/components/modules/ops/*`: Five tabs (Telemetry, Quota, Health, System, Audit). Telemetry includes hero metrics, P95 latency, throughput RPS, database health, session/quota activity, semantic cache snapshot, provider health table, and provider prompt cache analytics
- `src/components/modules/apikey/*`: API key creation, allowlists, expiry, max-cost, RPM/TPM, per-model quota, and IP/CIDR allowlist controls
- `src/components/modules/apikey-dashboard/*`: API key dashboard views — request stats, token usage, cost, quota, expiration countdown, and supported models when authenticated via API key
- `src/components/modules/setting/*`: 14+ settings cards including Info (version, self-update, version mismatch detection), Appearance (theme, locale, nav order + visibility), System (CORS allowlist, proxy, stats interval), Account (timezone, session), Semantic Cache, AI Route, API Key defaults, Retry, Auto Strategy, Circuit Breaker, Log/Price/Sync, Site Automation, WebDAV Backup, Backup (export/import/live migration), and Route Group Danger. Supports drag-and-drop card reordering with localStorage persistence
- `src/components/modules/user/*`: Management-console user and role administration
- `src/components/modules/navbar/*`: Top-level navigation state and persisted nav-order/visibility helpers
- `src/components/modules/toolbar/*`: Shared toolbar with per-page search, layout toggle (grid/list), sort options, and context-dependent filters
- `src/components/modules/login/*`: Login form supporting both username/password and API-key authentication modes behind a tabbed interface
- `src/components/modules/logo/*`: Shared animated Octopus logo component used by login and first-run screens
- `src/components/modules/first-run-setup.tsx`: Bootstrap wizard for creating the initial admin account when no admin exists, with animated particle background
- `src/api/`: API client and endpoint hooks (TanStack Query)
- `src/route/config.tsx`: UI route registration (lazy-loaded top-level modules: home, hub, channel, group, model, analytics, log, alert, ops, apikey, setting, user)
- `src/stores/`: Zustand state stores
- `src/lib/`: Utilities, i18n, logger, time zone helpers (10 time zones), service worker management
- `public/locale/`: Localized text resources (en, zh_hans, zh_hant)

## Notes

- The settings module includes dangerous operations such as deleting all route groups; UI confirmation is required before executing them.
- The settings module supports drag-and-drop reordering of its 14+ card sections, with order persisted to localStorage. A "Reset to Default" button restores the original order.
- Group mode labels and endpoint type display values are shared with backend behavior and should be updated together when adding new strategies or capabilities.
- AI routing has two entry points: the route page button generates the full routing table, while the group edit dialog button appends matched items into the current group only.
- The settings field for AI routing is now the default target group for the single-group compatibility flow, not the target for full-table generation.
- The `Model` route is now a `Model Market` view with dual tabs (Market and Capabilities) backed by `/api/v1/model/market`; it merges pricing, coverage, enabled-key counts, latency, and success metrics while preserving price-management actions. The Capabilities tab shows per-model endpoint support, conversation flags, and availability status.
- `Analytics` is organized into five tabs: `Cache` (semantic cache + provider prompt cache), `Utilization`, `Route Health`, `Evaluation`, and `Latency` (with P50/P95/P99 metrics and distribution histogram). The overview query remains available, but its primary UI summary cards now live on Home.
- `Ops` is organized into five tabs: `Telemetry` (hero metrics, P95, provider health, prompt cache analytics), `Quota`, `Health`, `System`, and `Audit`. Audit only covers selected management write routes, not public relay traffic.
- Semantic cache settings are split into configured state and runtime-enabled state. Enabling the switch alone is not enough; the embedding base URL and embedding model also need to be configured before runtime metrics turn green.
- Top-level page order and visibility are edited inside the `Appearance` card, persisted through the `nav_order` and `nav_visible` settings, and normalized against `DEFAULT_NAV_ORDER`, so missing routes are appended automatically and unknown routes are dropped.
- The Hub (`remote-site`) module consolidates all remote-site management into a single tab-based page. The five formerly standalone modules — announcement, checkin, redemption, usage-history, and credential — no longer exist as separate top-level routes; their UI now lives as tab panels inside `remote-site/`. A shared `CredentialDialog` component remains under `credential/` for reuse by the CredentialPanel.
- The `model-mapping` module provides pattern-based model name rewriting rules but is not exposed as a top-level navigation route. It is accessible from the app shell toolbar.
- The `proxy-pool` module provides named proxy configuration management with CRUD, connectivity testing, and reference tracking. It is accessible from the app shell toolbar.
- The login screen supports two authentication modes (user credentials and API key) behind a tabbed interface, and the first-run bootstrap wizard appears automatically when no admin account exists.
- The API key dashboard shows a dedicated view when authenticated via API key (instead of username/password), displaying request stats, token usage, cost, quota, and expiration info.
- The Settings `Backup` card includes a live database migration feature beyond simple export/import, with connection testing, per-table row counts, and post-migration restart reminder.
- The Settings `WebDAV` card provides automated cloud backup via WebDAV with configurable schedule, remote file management, and one-click restore.
- The Settings `Site Automation` card configures auto-sync and auto-checkin intervals for remote sites.
- The `channel` module includes per-channel proxy mode (direct/system/pool/inherit), request rewrite profiles (preserve/openai_chat_compat with codex header, tool role, and system message strategies), and param_override JSON for per-channel parameter injection into outbound requests.
- The `group` module includes endpoint provider configuration (openai/deepseek/mimo/siliconflow/newapi) for stripping incompatible reasoning fields, and supports a zashboard-style collapsible group list view.
- The Home page includes a GitHub-style activity heatmap visualizing request activity across the past year.
- The Analytics page includes a Share button that generates a PNG snapshot of the current analytics state for download or clipboard copy.
- User-configurable time zone (10 zones) affects all date/time display in the UI, independent from server-side stats timezone.
- Viewer accounts see masked domains (`***`) for Hub-related management data across sites, remote sites, credentials, channels, and URL settings.
- When backend API surfaces or top-level modules change, update both `web/README.md` and the root README files so the embedded-console docs remain consistent.
