<div align="center">

<img src="web/public/logo.svg" alt="Octopus Logo" width="120" height="120">

### Octopus

**A Simple, Beautiful, and Elegant LLM API Aggregation & Load Balancing Service for Individuals**

 English | [简体中文](README_zh.md) | [Changelog](CHANGELOG.md)

</div>


## ✨ Features

- 🔀 **Multi-Channel Aggregation** - Connect multiple LLM provider channels with unified management
- 🔑 **Multi-Key Support** - Support multiple API keys for a single channel
- ⚡ **Smart Selection** - Multiple endpoints per channel, smart selection of the endpoint with the shortest delay
- ⚖️ **Load Balancing** - Support round robin, random, failover, weighted, and auto strategies
- 🤖 **Auto Strategy** - Explore candidates first, then prefer higher in-window success rate automatically
- 🧠 **AI Routing, Auto Grouping & Conditional Groups** - Generate the full routing table from the route page, fill a single group from the edit dialog, and gate groups with JSON conditions
- 🔄 **Protocol Conversion** - Seamless conversion between OpenAI Chat / OpenAI Responses / OpenAI Embeddings / Anthropic API formats
- 🌐 **Multi-Provider Support** - Built-in support for OpenAI-compatible, Anthropic, Cloudflare, Gemini, Volcengine, MiMo, Codex, and passthrough channels
- 🛰️ **Media & Utility Relay** - Relay OpenAI Images, audio, video, search, rerank, and moderation endpoints through the same group / retry / circuit-breaker infrastructure
- 🧾 **API Key Governance** - Supported-model allowlists, expiry, max-cost caps, RPM / TPM limits, per-model quotas, and IP / CIDR allowlists
- 🔐 **Role-Based Admin Access** - Built-in `admin`, `editor`, and `viewer` roles with server-side permission enforcement
- 🔑 **WebAuthn / Passkey Login** — Passwordless login and registration via WebAuthn/Passkey with configurable RP settings
- 🚨 **Alerts & Notification Center** - Unified notification center aggregating system events, alert firings, and plan notifications with SSE streaming; alert rules for error rate (with scope + sliding window), cost threshold, quota exceeded, and channel down, delivered via webhook, Gotify, email, Telegram, Feishu, DingTalk, WeCom, and ntfy
- 📦 **Plan Provider Monitoring** - Track upstream subscription quota/usage (Codex, MiMo, StepFun, SenseNova, and balance-type providers like DeepSeek / Kimi / OpenRouter) and auto-create dedicated forwarding channels under the "Plan" channel group
- 📅 **Usage Reports** - Schedule daily / weekly / monthly usage reports delivered through notification channels
- 💎 **Model Market** - Unified model catalog with pricing, channel coverage, enabled key counts, latency, and success metrics, multi-dimension filtering with normalized dedupe, available-endpoint and price-category views, plus create / edit / delete / refresh price workflows
- 🔃 **Model Sync** - Automatic synchronization of available model lists with channels
- 📊 **Analytics & Evaluation** - Overview, provider / model / API key utilization, route health, latency distribution, semantic-cache evaluation, provider prompt-cache analytics, and live entry points for group testing / AI routing
- 🛠️ **Ops & Audit** - Telemetry, quota, health, system, and audit dashboards for daily operations, plus a management-write audit trail
- 🧠 **Semantic Cache** - Embedding-backed semantic cache for non-streaming and streaming OpenAI Chat / OpenAI Responses text requests, with runtime status and effectiveness metrics
- 🧭 **Configurable Navigation** - Persist top-level console page order and visibility in settings and reuse it across browsers
- 💾 **Runtime State Persistence** - Persist auto strategy windows and circuit breaker state to the database
- 🔗 **Site Management** - Manage upstream relay platforms (New-API, One-API, One-Hub, Sub2API, etc.) with multi-account support, projected channels, auto-sync, and auto-checkin
- 🌍 **Proxy Pool** - Named proxy configurations with direct / system / pool / inherit modes and reference tracking across sites, accounts, and channels
- 🔁 **Model Mapping** - Global model name rewriting rules with exact, wildcard, and regex matching, priority ordering, and optional group scope
- ☁️ **WebDAV Cloud Backup** - Automated cloud backup via WebDAV with configurable schedule, remote file management, and one-click restore
- 🔑 **API Credential Profiles** - Reusable Base URL + API Key pairs with health verification probes and CLI config export
- 📤 **CLI Config Export** - Generate configuration snippets for Claude Code, Codex, Gemini CLI, Cherry Studio, and Kilo Code
- 🎨 **Elegant UI** - Clean and beautiful web management panel with dark mode, activity heatmap, share snapshot, and responsive mobile layout
- 🗄️ **Multi-Database Support** - Support for SQLite, MySQL, PostgreSQL with live migration between database types


## 🚀 Quick Start

### 🐳 Docker

The examples below use `latest`. For a pinned version, use the image tag recorded in `docker-compose.yml` after that image has been published. Tag pushes start the release workflow; wait for it to finish before pulling the pinned image.

Run directly:

```bash
docker run -d --name octopus \
  --restart unless-stopped \
  -p 8080:8080 \
  -v octopus-data:/app/data \
  -e OCTOPUS_AUTH_JWT_SECRET="replace-with-a-long-random-secret" \
  lingyuins/octopus:latest
```

Recommended on Windows Docker Desktop:

```powershell
docker run -d --name octopus `
  --restart unless-stopped `
  -p 8080:8080 `
  -v octopus-data:/app/data `
  -e OCTOPUS_AUTH_JWT_SECRET="replace-with-a-long-random-secret" `
  lingyuins/octopus:latest
```

Or use docker compose:

```yaml
services:
  octopus:
    image: lingyuins/octopus:latest
    container_name: octopus
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - octopus-data:/app/data
    environment:
      OCTOPUS_AUTH_JWT_SECRET: "replace-with-a-long-random-secret"

volumes:
  octopus-data:
```

Then run:

```bash
docker compose up -d
```

Note: The official image runs as the non-root user `octopus` with UID/GID `1000`. The examples above use a Docker named volume for `/app/data` to avoid most host-permission issues, especially on Linux bind mounts and Windows Docker Desktop. If you intentionally bind-mount a host directory to `/app/data` (for example `./data:/app/data`), make sure that directory is writable by UID/GID `1000`, otherwise startup will fail with `permission denied` when creating `config.json` or `data.db`.

The official Docker image rebuilds the frontend during image build and embeds the latest exported UI into the Go binary, so the container includes the matching management UI for that release.

> **🕐 Timezone:** The image defaults to `Asia/Shanghai`. When running with `docker run` (not compose), pass `-e TZ=Asia/Shanghai` or your target IANA timezone (e.g. `-e TZ=America/Los_Angeles`). The server's log timestamps, statistics day boundaries, and frontend time display all depend on the container's timezone setting.

If you are upgrading from an older web build and still see stale frontend errors in the browser, clear the site data / service worker cache once after upgrading so the latest embedded assets are loaded.

> **🔗 Behind a reverse proxy:** By default Octopus does not trust any proxy, so `c.ClientIP()` returns the direct TCP address. Behind Docker bridge networking or a reverse proxy this is the gateway (e.g. `172.17.0.1`), not the real client — relay logs, login rate-limiting, and API-key IP allowlists all see the gateway IP. Set `OCTOPUS_SERVER_TRUSTED_PROXIES` to the proxy's CIDR (e.g. `172.17.0.0/16` for the default Docker bridge) to resolve the real client IP from `X-Forwarded-For`.


### 📦 Download from Release

Download the binary for your platform from [Releases](https://github.com/lingyuins/octopus/releases), then run:

```bash
./octopus start
```

### 🛠️ Build from Source

**Requirements:**
- Go 1.25.0
- Node.js 20+
- pnpm

```bash
# Clone the repository
git clone https://github.com/lingyuins/octopus.git
cd octopus
# Required before any Go build: the go:embed directive in static/static.go expects this
# directory to exist (it is gitignored, so fresh checkouts do not have it)
mkdir -p static/out && touch static/out/.keep
# Optional: bootstrap the initial admin via environment variables
export OCTOPUS_INITIAL_ADMIN_USERNAME="admin"
export OCTOPUS_INITIAL_ADMIN_PASSWORD="change-this-password-long"
# Required: set at least ONE of the two secrets below. Without security.encryption_key
# or a persistent auth.jwt_secret, startup is refused (deriving the AES key from an
# ephemeral secret would make stored encrypted data unrecoverable after a restart).
export OCTOPUS_AUTH_JWT_SECRET="replace-with-a-long-random-secret"
export OCTOPUS_SECURITY_ENCRYPTION_KEY="replace-with-another-long-random-secret"
# Start the backend service (API-only mode until frontend assets are embedded)
go run main.go start
```

> The `static/out/` step is mandatory for source builds: without that directory every `go build` / `go test` / `go run` fails on the embed directive. Once the directory exists (even with just `.keep`), the backend compiles and serves the API; the management UI becomes available after you place the built frontend assets under `static/out/` as shown below.

Release builds (`bash scripts/build.sh release`) install frontend dependencies with `--frozen-lockfile` and stop if any platform build or artifact preparation fails. Run `bash scripts/build_test.sh` to check release failure handling without building binaries.

**Build frontend assets for the embedded management UI**

```bash
cd web && pnpm install && NEXT_PUBLIC_APP_VERSION="$(git describe --tags --always 2>/dev/null || printf 'dev')" pnpm build && cd ..
# Replace the previously generated embed directory (static/out is gitignored build
# output; web/out is kept as-is, so the export does not need to be re-run)
rm -rf static/out
mkdir -p static/out
cp -r web/out/. static/out/
# go:embed fails on an empty directory; if Next.js exported an empty _not-found, add a placeholder
if [ -d web/out/_not-found ] && [ -z "$(ls -A web/out/_not-found)" ]; then
  touch static/out/_not-found/.keep
fi
# Start the backend service with embedded UI assets available in the repository
go run main.go start
```

**Development Mode**

```bash
cd web && pnpm install && NEXT_PUBLIC_API_BASE_URL="http://127.0.0.1:8080" NEXT_PUBLIC_APP_VERSION="$(git describe --tags --always 2>/dev/null || printf 'dev')" pnpm dev
## Open a new terminal, optionally set initial admin credentials for automatic bootstrap
export OCTOPUS_INITIAL_ADMIN_USERNAME="admin"
export OCTOPUS_INITIAL_ADMIN_PASSWORD="change-this-password-long"
## Required: set at least ONE of the two secrets below (see the Quick Start notes)
export OCTOPUS_AUTH_JWT_SECRET="replace-with-a-long-random-secret"
export OCTOPUS_SECURITY_ENCRYPTION_KEY="replace-with-another-long-random-secret"
## Start the backend service
go run main.go start
## Access the frontend at
http://localhost:3000
```

> **Dev CORS note:** the dev frontend (`http://localhost:3000`) calls the backend on `8080` cross-origin. The CORS allowlist is empty by default (= deny all origins), so if the browser blocks API requests, add `http://localhost:3000` (or `*`) in `Settings → System → CORS`.

### 🔐 Initial Admin Setup

On first launch, you can initialize the admin account in either of these ways:

- Provide `OCTOPUS_INITIAL_ADMIN_USERNAME` and `OCTOPUS_INITIAL_ADMIN_PASSWORD` to bootstrap automatically at startup
- Or open the Web UI on first visit and create the initial admin account through the guided setup wizard

> ⚠️ **Security Notice**: The initial admin password must be at least 12 characters long.
>
> ⚠️ **Security Notice**: Configure a persistent `auth.jwt_secret` or `security.encryption_key`; without either, startup is refused to prevent loss of encrypted credentials. If only the encryption key is configured, the JWT secret remains temporary and existing login tokens become invalid after a restart.

### 👥 Admin Roles

The management API and embedded Web UI use three built-in roles:

- `admin`: full access, including user management
- `editor`: operational write access for channels, groups, settings, API keys, logs, alerts, and AI routing
- `viewer`: read-only access to operational data

Role checks are enforced on the server side, using the currently stored role rather than trusting only the JWT claim.

### 📝 Configuration File

The configuration file is located at `data/config.json` by default and is automatically generated on first startup.

**Complete Configuration Example:**

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080
  },
  "database": {
    "type": "sqlite",
    "path": "data/data.db"
  },
  "log": {
    "level": "info"
  },
  "auth": {
    "jwt_secret": "replace-with-a-long-random-secret"
  },
  "security": {
    "encryption_key": "replace-with-another-long-random-secret"
  }
}
```

Most operational knobs are not stored in `config.json`. Retry policy, circuit breaker thresholds, auto-strategy tuning, relay log retention, public API base URL, AI-route service settings, semantic-cache switches, WebDAV backup, proxy pool, and model mapping rules are managed at runtime from the Settings page / management API and stored in the database.

**Configuration Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `server.host` | Listen address | `0.0.0.0` |
| `server.port` | Server port | `8080` |
| `server.trusted_proxies` | Comma-separated trusted reverse-proxy CIDRs/IPs for resolving real client IP from `X-Forwarded-For`. Empty = trust none (safe default; `c.ClientIP()` returns the direct TCP address). `*` = trust all (dev only; XFF spoofing risk). | empty |
| `database.type` | Database type | `sqlite` |
| `database.path` | Database connection string | `data/data.db` |
| `database.sqlite.cache_size` | SQLite `PRAGMA cache_size` (negative = KB, e.g. `-20000` ≈ 20 MB; positive = pages). Only used when `database.type` is `sqlite`. | `-20000` (≈ 20 MB) |
| `database.sqlite.mmap_size` | SQLite `PRAGMA mmap_size` in bytes. `0` disables mmap (safe default for low-memory hosts). | `0` (disabled) |
| `log.level` | Log level | `info` |
| `auth.jwt_secret` | JWT signing secret | empty (ephemeral secret generated at startup if unset) |
| `security.encryption_key` | Encryption key for sensitive stored data (credential profiles, site passwords, etc.) | empty (falls back to a persistent JWT secret; startup is refused if neither is configured) |
| `relay.max_json_body_bytes` | Maximum JSON request body size | `67108864` (64 MB) |
| `relay.max_multipart_body_bytes` | Maximum multipart request body size | `67108864` (64 MB) |

> 💡 **Tip**: Set `OCTOPUS_AUTH_JWT_SECRET` or `auth.jwt_secret` before running Octopus in production so login tokens stay valid across restarts.

**Database Configuration:**

Three database types are supported:

| Type | `database.type` | `database.path` Format |
|------|-----------------|-----------------------|
| SQLite | `sqlite` | `data/data.db` |
| MySQL | `mysql` | `user:password@tcp(host:port)/dbname` |
| PostgreSQL | `postgres` | `postgresql://user:password@host:port/dbname?sslmode=disable` |

**MySQL Configuration Example:**

```json
{
  "database": {
    "type": "mysql",
    "path": "root:password@tcp(127.0.0.1:3306)/octopus"
  }
}
```

**PostgreSQL Configuration Example:**

```json
{
  "database": {
    "type": "postgres",
    "path": "postgresql://user:password@localhost:5432/octopus?sslmode=disable"
  }
}
```

> 💡 **Tip**: MySQL and PostgreSQL require manual database creation. The application will automatically create the table structure.

**SQLite Tuning (Low-Memory Environments):**

For SQLite deployments, two per-connection PRAGMAs are configurable via `database.sqlite.*` (issue #97: low-memory hosts saw sustained disk IO because the cache was too small and mmap thrashing). Both default to memory-safe values: mmap disabled, cache ≈ 20 MB.

| Option | Description | Default |
|--------|-------------|---------|
| `database.sqlite.cache_size` | `PRAGMA cache_size`. Negative = KB (e.g. `-20000` ≈ 20 MB); positive = pages (4 KB each). `0` falls back to the built-in default. | `-20000` |
| `database.sqlite.mmap_size` | `PRAGMA mmap_size` in bytes. `0` disables mmap (recommended on hosts where RAM < DB size). Set a positive value on memory-rich hosts with a large DB to cut `read` syscalls. | `0` |

Recommended config for a ~1.6 GB RAM host with a few-hundred-MB DB (mmap off, modest cache):

```json
{
  "database": {
    "type": "sqlite",
    "path": "data/data.db",
    "sqlite": {
      "cache_size": -20000,
      "mmap_size": 0
    }
  }
}
```

### 🌐 Environment Variables

All configuration options can be overridden via environment variables using the format `OCTOPUS_` + configuration path (joined with `_`):

| Environment Variable | Configuration Option |
|---------------------|---------------------|
| `OCTOPUS_SERVER_PORT` | `server.port` |
| `OCTOPUS_SERVER_HOST` | `server.host` |
| `OCTOPUS_SERVER_TRUSTED_PROXIES` | `server.trusted_proxies` |
| `OCTOPUS_DATABASE_TYPE` | `database.type` |
| `OCTOPUS_DATABASE_PATH` | `database.path` |
| `OCTOPUS_DATABASE_SQLITE_CACHE_SIZE` | `database.sqlite.cache_size` (SQLite page cache; negative = KB, e.g. `-20000` ≈ 20MB) |
| `OCTOPUS_DATABASE_SQLITE_MMAP_SIZE` | `database.sqlite.mmap_size` (SQLite mmap size in bytes; `0` disables mmap — safe default for low-RAM hosts) |
| `OCTOPUS_DATA_DIR` | Default directory for `config.json` and the SQLite DB when `database.path` is not explicitly set |
| `OCTOPUS_LOG_LEVEL` | `log.level` |
| `OCTOPUS_AUTH_JWT_SECRET` | `auth.jwt_secret` |
| `OCTOPUS_SECURITY_ENCRYPTION_KEY` | `security.encryption_key` |
| `OCTOPUS_INITIAL_ADMIN_USERNAME` | Bootstrap the initial admin username at startup |
| `OCTOPUS_INITIAL_ADMIN_PASSWORD` | Bootstrap the initial admin password at startup |
| `OCTOPUS_GITHUB_PAT` | For rate limiting when getting the latest version (optional) |
| `OCTOPUS_RELAY_MAX_SSE_EVENT_SIZE` | Maximum SSE event size (optional) |

## 📸 Screenshots

> Note: The screenshots below show the core console surfaces. Current builds keep the same visual system and navigation, with `Model` presented as `Model Market` and additional `Analytics` / `Ops` entries in the sidebar.

### 🖥️ Desktop

<div align="center">
<table>
<tr>
<td align="center"><b>Dashboard</b></td>
<td align="center"><b>Channel Management</b></td>
<td align="center"><b>Group Management</b></td>
</tr>
<tr>
<td><img src="web/public/screenshot/desktop-home.png" alt="Dashboard" width="400"></td>
<td><img src="web/public/screenshot/desktop-channel.png" alt="Channel" width="400"></td>
<td><img src="web/public/screenshot/desktop-group.png" alt="Group" width="400"></td>
</tr>
<tr>
<td align="center"><b>Model Market</b></td>
<td align="center"><b>Logs</b></td>
<td align="center"><b>Settings</b></td>
</tr>
<tr>
<td><img src="web/public/screenshot/desktop-price.png" alt="Model Market" width="400"></td>
<td><img src="web/public/screenshot/desktop-log.png" alt="Logs" width="400"></td>
<td><img src="web/public/screenshot/desktop-setting.png" alt="Settings" width="400"></td>
</tr>
</table>
</div>

### 📱 Mobile

<div align="center">
<table>
<tr>
<td align="center"><b>Home</b></td>
<td align="center"><b>Channel</b></td>
<td align="center"><b>Group</b></td>
<td align="center"><b>Model Market</b></td>
<td align="center"><b>Logs</b></td>
<td align="center"><b>Settings</b></td>
</tr>
<tr>
<td><img src="web/public/screenshot/mobile-home.png" alt="Mobile Home" width="140"></td>
<td><img src="web/public/screenshot/mobile-channel.png" alt="Mobile Channel" width="140"></td>
<td><img src="web/public/screenshot/mobile-group.png" alt="Mobile Group" width="140"></td>
<td><img src="web/public/screenshot/mobile-price.png" alt="Mobile Model Market" width="140"></td>
<td><img src="web/public/screenshot/mobile-log.png" alt="Mobile Logs" width="140"></td>
<td><img src="web/public/screenshot/mobile-setting.png" alt="Mobile Settings" width="140"></td>
</tr>
</table>
</div>


## 📖 Documentation

> 📚 **Wiki**: This documentation is also available as a structured, navigable [wiki](wiki/Home.md) (English | [简体中文](wiki/Home_zh.md)).

### 🧭 Management Console Modules

The embedded management UI currently ships with these top-level modules:

| Module | What it covers |
|--------|----------------|
| Home | Version, runtime status, high-level summaries, trend chart, activity heatmap, and ranking panel |
| Hub | Upstream relay platform management with tabs for Sites (multi-account cards with inline balance / sync / check-in status, archive/restore, batch edit, and bulk import from AllAPIHub / MetAPI), Site Channels (projected channel bindings), Automation (auto-sync and auto-checkin intervals), Balance (plan balance charts), and TokenPlan (token plan monitoring) |
| Channel | Upstream provider configuration, keys, headers, sync, latency probing, proxy mode, and request rewrite profiles |
| Account Pool | Account pools for relay scheduling: searchable pool list with create/edit (name, description, strategy, default concurrency, cooldown, enabled) and delete confirmation; detail view with account keyword / platform / status filters, OAuth account authorization, batch operations, and credential import/export |
| Group | Model routing, load-balancing strategies, sticky sessions, group test, AI route generation, group thinking mode (auto / off / on), endpoint provider, zashboard-style collapsible group list, and CC Switch deep link |
| Model Market | Model catalog with market / available endpoints / price categories views, custom pricing, channel coverage, enabled key counts, latency, success metrics, multi-dimension filters with normalized dedupe, and fallback pricing plus peak/off-peak billing rules |
| Analytics | Channel × Model (default), Usage Breakdown, Route Health, Latency distribution, Evaluation, Cache (semantic + provider prompt cache), and share snapshot |
| Log | Dual views: Relay request history with Group / Request Body tabs, model/channel candidate statuses alongside the response, expandable attempt diagnostics, token usage, and cost records; plus an Error Log view for backend-recorded errors |
| Notification | Unified notification center with groups for Messages (inbox / archived), Alerts (rules / history), Delivery (channels / policies / preferences), and Reports (schedules / history). Alert rules, notification channels (webhook, Gotify, email, Telegram, Feishu, DingTalk, WeCom, ntfy), and usage report scheduling all live here |
| Ops | Telemetry (hero metrics, P95 latency, provider health, prompt-cache analytics), Quota, Health, Maintenance (retry / circuit breaker / response filter), System, and Audit trail |
| APIKey | API key create, edit, delete, supported-model allowlists, expiry, max-cost caps, RPM / TPM quotas, IP allowlists, and per-model quotas |
| Setting | Version/update info, appearance and nav preferences (order + visibility), AI route services, semantic cache, log retention, system options (public API base URL, proxy, CORS), LLM/price sync cadence, database backup + live migration, Redis cache backend, WebDAV backup, WebAuthn/Passkey, model-name normalization, account pool scheduling, a proxy pool entry, and dangerous operations |
| User | Admin user management and roles |

Additionally, the following features are accessible from the app shell toolbar or within other modules:

| Feature | What it covers |
|---------|----------------|
| Proxy Pool | Named proxy configurations CRUD, connectivity testing, and reference tree tracking |
| Model Mapping | Global model name rewriting rules with exact / wildcard / regex matching, priority, and group scope |
| API Credential Profiles | Reusable Base URL + API Key pairs with health verification and CLI export |

### 📡 Channel Management

Channels are the basic configuration units for connecting to LLM providers.

**Channel Templates:**

The UI provides built-in channel templates for quick creation (defined in `web/src/components/modules/channel/templates.ts`): OpenAI (creates an OpenAI Responses channel), Anthropic, Gemini, DeepSeek, OpenRouter, SiliconFlow, Volcengine, and MiMo.

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

**Zen Direct Model Routing:**

Requests with model name prefixed `zen/<model>` bypass group model mapping and route directly to the upstream model. Octopus performs smart channel-type detection based on the model name (e.g., Claude → Anthropic, Gemini → Gemini, GPT → OpenAI).

**Response ID Affinity:**

For the OpenAI Responses API, follow-up requests referencing the same response ID are automatically routed to the same upstream channel to maintain conversation continuity.

**Model Mapping:**

Global model name rewriting rules are applied in the relay pipeline before group resolution. Rules support exact, wildcard (glob), and regex matching with priority ordering and optional group scope.

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

### 📁 Group Management

Groups aggregate multiple channels into a unified external model name.

**Core Concepts:**

- **Group name** is the model name exposed by the program
- When calling the API, set the `model` parameter to the group name
- **First Token Timeout**: unit in seconds, only effective for streaming responses, `0` means no limit
- **Session Keep Time**: unit in seconds, keeps using the same channel for the same API key + model within the configured session window, `0` means disabled
- **Condition (JSON)**: optional AND rules currently evaluated in the main LLM relay path; the built-in request context currently includes `model`, `api_key_id`, and `hour`
- **Endpoint Provider**: provider-aware request rewriting that adapts requests for upstream compatibility per endpoint type. Chat providers (`openai`, `deepseek`, `mimo`, `siliconflow`, `newapi`) strip incompatible reasoning fields; music providers (`newapi`, `minimax`) rewrite the request body and path; video provider (`agnes`) rewrites the upstream path; audio speech provider (`mimo`) converts the request format and path
- **Outbound Format**: controls cross-format adapter fallback. `""` (auto), `chat`, and `responses` set the Chat/Responses attempt order; `chat_only` and `responses_only` disable the fallback entirely — useful for upstreams (e.g. public-welfare relays) that reject the other format with 400/404
- **Thinking Mode**: configure `thinking_mode` in the group editor's advanced settings. `auto` (default) preserves client/upstream behavior; `off` / `on` override client reasoning controls even when the client sends none. Adapters use provider-specific fields for DeepSeek/MiMo, OpenAI Chat/Responses (including Codex), Anthropic, Gemini, and Volcengine. For a DeepSeek/MiMo-compatible proxy with a generic URL, set the corresponding API type or endpoint provider. This requires upstream model support; raw/passthrough formats leave the original body unchanged and skip the policy. It does not add a FIM or `/v1/completions` endpoint.
- **Key Cooldown**: rate-limit cooldown is tracked per `(keyID, model)` (issue #94), so a single model's 429 no longer blocks the same key's other models. Per-channel retry count can be set to `0` (try once, then move to the next channel); the max-total-attempts quota counts only real upstream forwards (cooldown/circuit-breaker skips do not consume it)

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
- Existing groups with the same name only receive missing route items; existing groups are not cleared or replaced
- Clicking **AI Fill Current Group** in the edit dialog sends all models to AI and appends only the matched route items to that group
- The setting previously named AI route target group now acts as the default target group for the single-group compatibility flow only
- AI route tasks are persistent with heartbeat, progress tracking, batch management, and interruption recovery

**CC Switch Integration:**

The group toolbar includes a CC Switch deep link generator that creates provider import links for target apps: Claude Code, Codex, Gemini, OpenCode, and OpenClaw. For Claude Code, it supports mapping Haiku / Sonnet / Opus models to specific route groups.

> 💡 **Example**: Create a group named `gpt-4o`, add multiple providers' GPT-4o channels to it, then access all channels via a unified `model: gpt-4o`.

---

### 💎 Model Market & Pricing

The `Model` route (Model Market) provides switchable toolbar views: **Market** (pricing and coverage cards), **Available Endpoints** (endpoint → model grouping), and **Price Categories** (fallback pricing rules plus peak/off-peak billing).

**Market view data merged on each card:**

- Custom or synced pricing from the LLM price catalog (input / output / cache read / cache write)
- Channel coverage and enabled key counts from channel-model relationships
- Average latency, success rate, and success / failure request counts from recorded model stats
- A peak-billing badge when the model is covered by a peak/off-peak billing schedule (directory price is the peak price; the idle price is discounted)

**Multi-dimension filtering (Market view):**

- Name search shared with the toolbar search box
- Capability chips (chat, embeddings, rerank, …) — conversation-style endpoints count toward the chat capability
- Vendor chips inferred from the model name
- Pricing filter: all / priced / free
- Normalized-name dedupe: merges naming variants of the same base model (e.g. `kimi-k2.5`, `moonshotai/kimi-k2.5`, `dmxapi-kimi-k2.5-cc`). Rules come from the Settings `Normalize` card (router prefixes, functional suffixes, explicit variant→canonical mappings), and dedupe can be turned on by default via a dedicated setting
- The collapsible filter bar shows the live result count (visible / total), the number of active filters, and a one-click reset that clears search, capability, provider, pricing, and dedupe

**Summary metrics:**

The toolbar summary strip shows four metrics plus the last price-update time and a refresh-prices button. The numbers come from the market endpoint's summary over the **full** dataset — they do **not** change with search or filters (only the card list is filtered). When the default-dedupe setting is on, the market endpoint itself aggregates naming variants server-side, so cards and summary counts already show merged entries:

| Metric | Meaning |
|--------|---------|
| Models | Total models aggregated in the market |
| Channel Coverage | Total channel-to-model coverage entries across the market |
| Unique Channels | Distinct channels across all models |
| Average Latency | Request-weighted average latency across the market |

**Cards and dialogs:**

- Responsive virtualized cards (grid / list / compact layouts on desktop, single-column list on mobile), sorted by success rate or request count
- Expand a card for price pairs (input / cache-read and output / cache-write, with optional CNY conversion), runtime metrics, and per-channel rows (enabled state and enabled key count); channel tags fold into a `+N` chip when there are too many
- Standard edit dialog with strict price validation (non-negative decimals; invalid fields are highlighted inline instead of being silently zeroed) and a delete confirmation dialog

**Available Endpoints view:**

Aggregated from valid route groups and inverted into endpoint → model groups. Conversation-family endpoints (chat / deepseek / mimo / responses / messages / auto) are merged into a single "Chat" group shown first; models with no declared endpoint fall under the auto (`*`) group. This view shares the toolbar search box with the market view — a search matches endpoint names/labels or model names, forces all groups expanded, and reveals every matched chip. Each group card is collapsible with a model count, vendor-icon chips, and a "show more" control for large model sets.

**Price Categories view:**

- Fallback pricing by rule for models without an exact price; rules are matched in sort order and the first hit wins
- Rule table: name, rule type (exact / prefix / contains), rule value, the four prices, sort order, and an enabled badge
- Create / edit dialog with validation: name and rule value required, prices must be finite non-negative numbers, sort order must be an integer. Errors appear after the first submit attempt and clear as the input is fixed
- Peak/Off-peak Billing section (currently for DeepSeek): off-peak multiplier, weekend off-peak switch, and two Beijing-time windows (an empty window is closed; both windows closed means all-day off-peak; overlapping windows are allowed with a hint)
- Rule changes invalidate the market price cache; this view has no search box

**Query gating:**

Views stay mounted when you switch pages (keep-alive), so every query is gated: the market and capabilities queries only run while the Model module is active and the matching view is selected, and switching views stops the query for the view you left.

**Data Sources:**

- The system periodically syncs model pricing data from [models.dev](https://github.com/sst/models.dev)
- When creating or syncing channels, if a model is not yet in the local catalog, Octopus automatically creates a local model-price record so the price can still be maintained manually
- Manual creation of models that exist in models.dev is also supported for custom pricing

**Price Priority:**

| Priority | Source | Description |
|:--------:|--------|-------------|
| 🥇 High | This Page | Prices set by user in the model market page |
| 🥈 Low | models.dev | Auto-synced default prices |

> 💡 **Tip**: To override a model's default price, simply set a custom price for it in the model market page.

**Operational actions preserved on the page:**

- Create a custom model price record
- Edit input / output / cache prices for an existing model
- Delete a custom model entry
- Refresh upstream pricing from the toolbar summary strip
- Keep the scheduled price refresh policy in the Settings `LLM Sync` card

---

### 📈 Analytics

The Analytics module is a read-oriented operations view. The default tab is **Channel × Model** so the most-watched data shows first:

| Tab | What it shows |
|-----|---------------|
| Channel × Model | Channel×model usage matrix, usage-distribution share chart (top-N by model / channel×model, cost/count/tokens metrics) |
| Usage Breakdown | Provider, model, and API key breakdowns for the selected time range (renamed from "Utilization" with a no-billing hint when cost data is empty) |
| Route Health | Health score, enabled / disabled item counts, and recent failure pressure for each group |
| Latency | Request latency metrics (Avg, P50, P95, P99), first-token-user-time (FTUT) metrics, and latency distribution histogram |
| Evaluation | Group readiness, AI route progress, group test progress, and semantic-cache effectiveness |
| Cache | Semantic cache effectiveness and provider-side prompt-cache analytics (cache rate, reuse ratio, estimated cost savings per provider) |

**Time ranges:** `1d`, `7d`, `30d`, `90d`, `ytd`, and `all`

The overview metrics API still exists as `/api/v1/analytics/overview`, but the primary UI entry point for those summary cards is now the Home page. Home also carries an independent `7d / 30d / 90d` overview-range switch, plus a daily hero summary, trend chart, GitHub-style activity heatmap, and ranking panel.

The Evaluation tab is intentionally lightweight: it acts as an entry point into group testing, AI routing, and semantic-cache tuning instead of duplicating those full workflows.

**Share Snapshot:**

The Analytics page includes a Share button that generates a visual PNG snapshot of the current analytics state, which can be downloaded or copied to the clipboard. The snapshot includes key stats (requests, tokens, cost, providers, cache hit rate) and a timestamp.

---

### 🛠️ Ops

The Ops module focuses on runtime posture and operational diagnostics:

| Tab | What it shows |
|-----|---------------|
| Telemetry | Hero metrics (uptime, total requests, avg latency, error rate, active connections, memory usage), P95 latency, throughput RPS, database health, session & quota activity, semantic cache snapshot, provider health table (sortable columns + mini bar charts) |
| Quota | API key limit posture across RPM, TPM, max-cost, and per-model quota settings, merged with total tokens + success rate + "view key detail" jump |
| Health | Database reachability, cache readiness, task-runtime sanity, recent error count, and failing groups (with jump to Analytics → Route Health) |
| Maintenance | Actionable runtime tuning: Retry, Circuit Breaker, and Response Filter settings consolidated in one tab (moved out of the Settings page) |
| System | Build metadata, database type, public API base URL, proxy, retention intervals, AI route mode, and AI route services |
| Audit | Paginated audit history for management-side write operations |

**Provider Prompt Cache Analytics:**

The Telemetry tab includes provider-side prompt cache monitoring, tracking upstream provider prompt caching effectiveness: cache rate, cache reuse ratio, cache read / write tokens, estimated cost savings per channel, and a 24-hour cache trend chart. This is separate from the semantic cache.

**Audit scope:**

- Covers selected management write routes such as channel / group / model / setting / API key / alert / user mutations, AI route generation, log clearing, price refresh, import, and self-update
- Does not record public `/v1/...` relay traffic

---

### ⚙️ Settings

Global system configuration.

**Statistics Save Interval (minutes):**

Since the program handles numerous statistics, writing to the database on every request would impact read/write performance. The program uses this strategy:

- Statistics are first stored in **memory**
- Periodically **batch-written** to the database at the configured interval
- Relay balancer runtime state uses the same periodic persistence pattern

**Runtime State Persistence:**

- Auto strategy windows are loaded from the database on startup
- Circuit breaker state is loaded from the database on startup
- Both are saved periodically using the same interval as statistics persistence
- Both are also saved during graceful shutdown

**Key settings cards in the current UI** (the definitive list lives in `web/src/components/modules/setting/index.tsx`):

| Card | Purpose |
|------|---------|
| Info | Current version, latest release lookup, cache-mismatch detection, and in-place self-update entry with version mismatch notification |
| Appearance | Theme, locale, alert language, drag-and-drop top-level navigation order, and per-page visibility toggles |
| AI Route | Default compatibility group, timeout, parallelism, and service-pool configuration |
| Auto Strategy | Auto strategy tuning (minimum samples, time window, sliding window size, latency weight) |
| Account | Login-session/account preferences and application timezone selection |
| Semantic Cache | Enablement, TTL, similarity threshold, max entries, embedding base URL / API key / model / timeout |
| Log | Retention (time-based and count-based) and log level |
| System | Public API base URL, proxy URL, CORS allowlist (tag-style management), and stats persistence interval |
| LLM Sync | Upstream model synchronization and price refresh cadence |
| Backup | Database export, import, and live database migration between SQLite / MySQL / PostgreSQL with connection testing and per-table row count results |
| Redis | Optional Redis cache backend configuration: connection settings, test connection, and save (restart to apply). Unloads stats, runtime state, rate-limit/cooldown, and channel-delay probing to Redis for low-memory hosts and multi-instance scaling |
| WebDAV Backup | WebDAV cloud backup configuration: connection settings, auto-backup interval, max backups retention, manual trigger, remote file listing, restore, and delete |
| WebAuthn / Passkey | RP ID, RP name, allowed origins configuration |
| Normalize | Model-name normalization rules: router prefixes, functional suffixes, and explicit variant→canonical mappings (runtime-configurable, with an offline AI-assisted normalization workflow) |
| Pool | Account-pool scheduling tuning: health patrol, layered scheduling, sticky-session escape, weight reset, and related keys |
| Proxy Pool | Opens the shared proxy-pool management dialog (no inline panel) |

> **Note:** The following settings have been relocated to more relevant modules (issue #87):
> - **Retry / Circuit Breaker / Response Filter** → `Ops → Maintenance` tab
> - **Site Automation** → `Hub → Automation` tab
> - **Purge Unavailable Models / Delete All Route Groups** → `Group` page "Maintenance" dropdown button

**Semantic Cache Scope:**

- Applies to non-streaming and streaming OpenAI Chat and OpenAI Responses text requests
- Streaming cache hits replay from the SSE session buffer with stable stream-session recovery
- Namespaces cache entries by `api_key_id + endpoint_family + requested_model`
- If the embedding client is not fully configured, or embedding lookup / store fails, Octopus bypasses the cache and relays the request normally
- Runtime state and effectiveness are visible in both `Analytics -> Evaluation` and `Ops -> Telemetry`
- Cache entries are preserved across unchanged runtime config refreshes

**Database Live Migration:**

The Backup settings card includes a live database migration feature beyond simple export/import:

- Target database types: SQLite, MySQL, PostgreSQL
- Connection testing before migration
- Optional inclusion of logs and stats in migration
- Migration result display with per-table row counts
- Post-migration restart reminder (the backend continues using the old database until restart)

**Dangerous Operation in Settings:**

- The Settings page provides **Delete All Route Groups**
- The action requires a second confirmation before execution
- It deletes all groups and group items, then resets the default target group for single-group AI routing to `0` to avoid dangling references

**Settings Card Order:**

The Settings page supports drag-and-drop reordering of its card sections, with order persisted to local storage. A "Reset to Default" button restores the original order.

> ⚠️ **Important**: When exiting the program, use proper shutdown methods (like `Ctrl+C` or sending `SIGTERM` signal) to ensure in-memory statistics are correctly written to the database. **Do NOT use `kill -9` or other forced termination methods**, as this may result in statistics data loss.

---

### 🔗 Site Management (Hub → Sites)

The Hub module's **Sites** tab manages upstream relay platforms as a first-class entity. Sites represent platforms like New-API, One-API, One-Hub, Done-Hub, Sub2API, OpenAI, Claude, Gemini, and SAPI. Each site renders as a multi-account card with inline balance, sync status, and check-in status, replacing the previous multi-tab Hub layout.

**Features:**

- Multi-account support per site with username/password, access_token, or api_key credentials
- Auto-sync of channels, tokens, and models at configurable intervals
- Auto-checkin with configurable intervals and random time windows (per-account toggle)
- Inline per-account actions: sync, check-in, enable/disable, edit, delete
- Site-level actions: pin, archive / restore, batch edit (enable / disable / delete), import
- **Projected channels**: automatically creates local Octopus channels from site account groups with per-group key management, model routing, and history tracking (viewable in the **Site Channels** tab)
- Route type inference per model (openai_chat, openai_response, anthropic, gemini, volcengine, embedding)
- Manual model add / delete and route type override
- Bulk import from AllAPIHub and MetAPI formats (file upload or paste JSON)
- Proxy pool integration with per-site, per-account, and per-channel proxy selection
- Archive sites (keeps accounts/keys/models, takes projected channels offline) and restore from the archived list
- **Automation** tab consolidates auto-sync and auto-checkin interval configuration

---

### 🌍 Proxy Pool

A shared proxy configuration pool accessible from the app shell toolbar:

- Named proxy configurations with URL, scheme (SOCKS5 / HTTP / HTTPS), enable/disable, and remarks
- Proxy modes: `direct`, `system`, `pool`, `inherit`
- Proxy connectivity testing against a configurable test URL
- **Reference tree** showing which sites, site accounts, managed channels, and channels use each proxy
- Jump-to-reference navigation that deep-links to the referencing entity
- Deletion protection when a proxy has active references

---

### 🔁 Model Mapping

Global model name rewriting rules applied in the relay pipeline before group resolution:

- **Match types**: Exact, Wildcard (glob), and Regex
- **Target model**: the rewritten model name
- **Priority ordering**: rules are evaluated in priority order
- **Group scope**: optionally apply only to a specific group
- **Enable/disable toggle** per rule

---

### ☁️ WebDAV Cloud Backup

Automated cloud backup via WebDAV with full lifecycle management:

- Configurable base URL, credentials, remote path, auto-backup interval (default 6 hours), and max backups retention
- Connection testing before enabling
- Manual backup trigger
- Remote backup file listing with size info
- One-click restore from any remote backup
- Delete remote backups
- Included in the Settings page as a dedicated card

---

### 🔑 API Credential Profiles & CLI Export

Reusable API credential profiles store Base URL + API Key pairs for quick access:

- Health verification probes: `text_gen`, `models_list`, `tool_calling`, `structured_output`
- Health status tracking per credential
- Encryption at rest via `security.encryption_key`
- Tags and notes for organization

**CLI Config Export:**

Generate ready-to-use configuration snippets for client tools:

| Tool | Format |
|------|--------|
| Claude Code | Environment variables for `~/.claude/settings.json` |
| Codex | Environment variables for `~/.codex/auth.json` and `config.toml` |
| Gemini CLI | Environment variables |
| Cherry Studio | JSON provider import configuration |
| Kilo Code | JSON settings block |

---

### 🚨 Notification Center & Alerts

The Notification module is a unified center aggregating system events, alert firings, and plan-provider notifications with severity levels, read/archive state, filtering, and SSE streaming for real-time delivery. Alert rules monitor system health and trigger notifications:

**Alert rule types:** Error rate (with configurable scope — per-channel / per-group / global — and sliding-window evaluation), cost threshold, quota exceeded, channel down, and account-pool account errors (`pool_account_error`, scoped to an account).

**Notification channels:**

| Channel | Configuration |
|---------|--------------|
| Webhook | URL, method, headers |
| Gotify | Server URL, app token |
| Email | SMTP settings, recipients |
| Telegram | Bot token, chat ID |
| Feishu | Webhook key |
| DingTalk | Robot access token, optional HMAC-SHA256 signing secret |
| WeCom | Group robot key |
| ntfy | Topic URL, optional access token |

Alert state and history are tracked per rule, with configurable evaluation intervals.

**Usage Reports:** Schedule daily / weekly / monthly usage reports delivered through the configured notification channels, with report history tracking.

---

## 🔌 Client Integration

### OpenAI SDK

```python
from openai import OpenAI
import os

client = OpenAI(   
    base_url="http://127.0.0.1:8080/v1",   
    api_key="sk-octopus-P48ROljwJmWBYVARjwQM8Nkiezlg7WOrXXOWDYY8TI5p9Mzg", 
)
completion = client.chat.completions.create(
    model="octopus-openai",  # Use the correct group name
    messages = [
        {"role": "user", "content": "Hello"},
    ],
)
print(completion.choices[0].message.content)
```

### Claude Code

Edit `~/.claude/settings.json`

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:8080",
    "ANTHROPIC_AUTH_TOKEN": "sk-octopus-P48ROljwJmWBYVARjwQM8Nkiezlg7WOrXXOWDYY8TI5p9Mzg",
    "API_TIMEOUT_MS": "3000000",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "ANTHROPIC_MODEL": "octopus-sonnet-4-5",
    "ANTHROPIC_SMALL_FAST_MODEL": "octopus-haiku-4-5",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "octopus-sonnet-4-5",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "octopus-sonnet-4-5",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "octopus-haiku-4-5"
  }
}
```

### Codex

Edit `~/.codex/config.toml`

```toml
model = "octopus-codex" # Use the correct group name

model_provider = "octopus"

[model_providers.octopus]
name = "octopus"
base_url = "http://127.0.0.1:8080/v1"
```

Edit `~/.codex/auth.json`

```json
{
  "OPENAI_API_KEY": "sk-octopus-P48ROljwJmWBYVARjwQM8Nkiezlg7WOrXXOWDYY8TI5p9Mzg"
}
```

### CLI Config Export

For other clients (Gemini CLI, Cherry Studio, Kilo Code), use the built-in **CLI Export** feature from the API Credential Profiles panel in the management console to generate ready-to-use configuration snippets.

---

## 🏗️ Architecture

Octopus follows a clean layered architecture in Go:

```
cmd/                    # Entry points (Cobra CLI)
internal/
├── apperror/           # Application error types (machine code + HTTP status + i18n params)
├── conf/               # Configuration loading & build metadata
├── client/             # Outbound HTTP client pool (timeout buckets, proxy, SSRF guards)
├── db/                 # Database connection & migrations (SQLite/MySQL/PostgreSQL)
│   └── migrate/        # Versioned schema migrations (numbering follows the files in this directory)
├── model/              # Domain types (Channel, Group, APIKey, User, Site, ProxyConfiguration, ModelMapping, …)
├── op/                 # Business logic operations split by domain
│   ├── airoute/        # AI route generation, progress tracking, service pool, and compatibility helpers
│   ├── alert/          # Alert rule evaluation and notification dispatch
│   ├── analytics/      # Dashboard, utilization, route-health, evaluation, and latency queries
│   ├── apikey/         # API key CRUD and validation
│   ├── audit/          # Audit log persistence
│   ├── backup/         # Database export/import, WebDAV cloud backup scheduler
│   ├── cacheusage/     # Cache usage tracking
│   ├── channel/        # Channel CRUD, sync, grouping, keys, managed channel projection, and base URL helpers
│   ├── credential/     # API credential profile management with encryption
│   ├── dbmigration/    # Live database migration between SQLite/MySQL/PostgreSQL
│   ├── errorlog/       # Backend error log persistence and retention
│   ├── group/          # Route-group CRUD, auto-grouping, group items, tests, and cache-backed lookups
│   ├── llm/            # LLM price catalog operations
│   ├── modelmapping/   # Model mapping rule management
│   ├── modelnormalize/ # Model-name normalization rules
│   ├── navorder/       # Navigation order and visibility persistence
│   ├── notification/   # Notification center (messages, SSE stream, preferences)
│   ├── ops/            # Ops dashboard data aggregation (telemetry, quota, health)
│   ├── pool/           # Account pool management for relay scheduling
│   ├── ratelimitstore/ # RPM/TPM rate limit state
│   ├── relaylog/       # Relay log persistence with async flush worker
│   ├── remotesite/     # Remote Hub site operations (balance, checkin, announcements, usage, tokens, redemption)
│   ├── report/         # Usage report scheduling and delivery
│   ├── setting/        # Settings CRUD and validation
│   ├── stats/          # Request statistics aggregation, cache, and site-model backfill
│   ├── user/           # User management and authentication
│   └── webauthn/       # WebAuthn / Passkey registration and authentication
├── relay/              # Core relay pipeline
│   ├── balancer/       # Load balancing strategies (RoundRobin, Random, Failover, Weighted, Auto)
│   ├── condition/      # Request condition evaluation
│   └── poolscheduler/  # Account pool scheduling for pool-backed channels
├── server/             # HTTP layer (Gin)
│   ├── auth/           # JWT auth & permissions
│   ├── handlers/       # Route handlers (one per resource)
│   ├── middleware/     # Auth, RBAC, CORS, rate-limit, audit, security, IP allowlist, …
│   ├── resp/           # Response envelope helpers
│   └── router/         # Route registration system
├── task/               # Background periodic jobs
├── transformer/        # Protocol adapters
│   ├── inbound/        # Client→Internal (OpenAI, Anthropic)
│   ├── outbound/       # Internal→Upstream (OpenAI, Anthropic, Cloudflare, Gemini, Volcengine, MiMo, Codex, Passthrough)
│   ├── rewrite/        # Request normalization with configurable profiles
│   └── model/          # Shared transformer types & interfaces
├── hub/                # Remote site adapter interface, registry, HTTP client, and platform-specific adapters
├── planprovider/       # Upstream subscription plan monitoring (Codex, MiMo, StepFun, SenseNova, balance-type providers)
├── poolhealthcheck/    # Periodic account-pool health patrol
├── pooltokenrefresh/   # Account-pool OAuth token refresh with backoff
├── poolscheduledtest/  # Scheduled account-pool connectivity tests and recovery
├── store/              # Optional cache/state backend (KVStore, RateLimitStore, StatsStore, RuntimeStateStore, ChannelDelayStore): memory + Redis
├── helper/             # Cross-cutting helpers (AI route, channel/group probes, price, notify)
├── price/              # LLM price catalog (models.dev sync)
├── pkg/                # Third-party OAuth flow helpers for pool accounts (Anthropic / OpenAI / Gemini CLI / xAI)
├── update/             # Self-update mechanism
├── utils/              # Utilities (cache, ratelimit, semantic_cache, tokenizer, crypto, …)
├── site/               # Thin facade forwarding to sitesync
└── sitesync/           # Site sync, projection, and check-in implementation
```

**Relay data flow:**

```
Client Request
    ↓
Model Mapping (global name rewriting)
    ↓
inbound.TransformRequest (raw → internal format)
    ↓
outbound.TransformRequest (internal → upstream format)
    ↓
http.Do (forward to upstream provider)
    ↓
outbound.TransformResponse (upstream response → internal format)
    ↓
inbound.TransformResponse (internal → client format)
    ↓
Client Response
```

For streaming, the same pipeline processes each SSE event through `TransformStream`.

**Hub adapters:**

The Hub remote site management uses an adapter-based architecture. Adapter packages register dedicated handlers for their site types in `init()` (see `internal/hub/` and `internal/hub/README.md` for the current list), and the registry falls back to the `common` (One API / New API family) adapter for any site type without a dedicated one:

| Adapter | Site Type(s) |
|---------|-----------|
| `common` | `new-api`, `unknown` registered directly; `veloera`, `done-hub`, `one-hub`, `anyrouter` served via the registry fallback (One API / New API family) |
| `octopus` | `octopus` (self-aware adapter) |
| `aihubmix` | `aihubmix` |
| `axonhub` | `axonhub` |
| `claudecodehub` | `claude-code-hub` |
| `sub2api` | `sub2api` |
| `sapi` | `sapi` (user account/password login with token caching) |

The `ldoh` package provides public site discovery (not an adapter).

Each adapter implements the `SiteAdapter` interface covering user info, check-in, models, pricing, tokens, channels, announcements, status, redemption, and usage logs.

**Frontend (Next.js 16 App Router):**

```
web/src/
├── api/               # API client & endpoint hooks (TanStack Query)
├── app/               # Next.js App Router pages
├── components/
│   ├── modules/       # Domain modules (channel, group, apikey, remote-site, site, pool, proxy-pool, model-mapping, credential, …)
│   ├── ui/            # UI primitives (Radix-based)
│   ├── common/        # Shared components
│   ├── nature/        # Animated backgrounds & effects
│   └── animate-ui/    # Animated UI components
├── hooks/             # Custom hooks
├── lib/               # Utilities, i18n, logger, time zone helpers
├── provider/          # React context providers
├── route/             # Lazy-loaded route config
└── stores/            # Zustand client state
```

## 🕐 Timezone Architecture

Octopus involves three independent timezone layers:

| Layer | Controlled By | Affects |
|-------|--------------|---------|
| **Container timezone** | `ENV TZ` / `-e TZ=` | Server log timestamps, `time.Now()` return value |
| **Stats timezone** | Admin UI → `stats_timezone` (IANA name, e.g. `Asia/Shanghai`) | Which date hourly/daily statistics roll into |
| **Frontend display timezone** | Admin UI → user preference | How all timestamps appear on pages |

The three layers are independent: the container timezone affects the server runtime, the stats timezone affects data aggregation, and the frontend timezone only changes how users see time text.

## 🔐 Security

- **JWT Authentication**: Management API uses JWT tokens with configurable expiry. Login rate limiting protects against brute-force attacks (configurable window and max failed attempts).
- **Role-Based Access Control**: Server-side RBAC with `admin`, `editor`, `viewer` roles, reloaded from DB each request.
- **API Key Security**: API keys (`sk-octopus-...`) support model allowlists, IP/CIDR allowlists, expiry, max-cost caps, RPM/TPM limits, and per-model quotas.
- **Encryption at Rest**: Sensitive stored data (credential profiles, site passwords) is encrypted via AES-256-GCM using `security.encryption_key`.
- **CORS Management**: Tag-style CORS allowlist manager with `*` for all, specific domains, or deny-all (empty).
- **Viewer Domain Masking**: Hub-related management data masks domains for viewer accounts across sites, remote sites, credentials, channels, and URL settings.

## 🤝 Acknowledgments

- 🙏 [looplj/axonhub](https://github.com/looplj/axonhub) - The LLM API adaptation module in this project is directly derived from this repository
- 📊 [sst/models.dev](https://github.com/sst/models.dev) - AI model database providing model pricing data
- 💡 [qixing-jk/all-api-hub](https://github.com/qixing-jk/all-api-hub) - The Hub concept and feature design inspiration
- 🛠️ [Hureru/octopus](https://github.com/Hureru/octopus) - The original Hub implementation
- 🏊 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) - The account-pool OAuth flows (Anthropic / OpenAI / Gemini CLI / xAI) and account-management UX are ported from this project

