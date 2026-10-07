# Architecture, Timezone & Security

## 🏗️ Architecture

Octopus follows a clean layered architecture in Go:

```
cmd/                    # Entry points (Cobra CLI)
internal/
├── conf/               # Configuration loading & build metadata
├── client/             # HTTP client utilities
├── db/                 # Database connection & migrations (SQLite/MySQL/PostgreSQL)
│   └── migrate/        # Versioned schema migrations (numbering grows over time — see the directory for the current set)
├── model/              # Domain types (Channel, Group, APIKey, User, Site, ProxyConfiguration, ModelMapping, …)
├── apperror/           # Application error types (machine code + HTTP status + i18n params)
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
│   ├── errorlog/       # Error log persistence and retention cleanup (backend panics + frontend exceptions)
│   ├── group/          # Route-group CRUD, auto-grouping, group items, tests, and cache-backed lookups
│   ├── llm/            # LLM price catalog operations
│   ├── modelmapping/   # Model mapping rule management
│   ├── modelnormalize/ # Model-name normalization rules
│   ├── navorder/       # Navigation order and visibility persistence
│   ├── notification/   # Notification center (messages, SSE stream, preferences)
│   ├── ops/            # Ops dashboard data aggregation (telemetry, quota, health)
│   ├── pool/           # Account pool CRUD, credentials, quota, unschedulable rules, and account testing
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
│   └── poolscheduler/  # Account pool scheduling for relay requests
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
├── pkg/                # Third-party OAuth flow helpers for account pool accounts (Anthropic / OpenAI / Gemini CLI / xAI)
├── poolhealthcheck/    # Periodic health probes for account pool accounts
├── pooltokenrefresh/   # Automatic OAuth token refresh for account pool accounts (with backoff)
├── poolscheduledtest/  # Scheduled connectivity tests for account pool accounts
├── store/              # Optional cache/state backend (KVStore, RateLimitStore, StatsStore, RuntimeStateStore): memory + Redis
├── helper/             # Cross-cutting helpers (AI route, channel/group probes, price, notify)
├── price/              # LLM price catalog (models.dev sync)
├── update/             # Self-update mechanism
├── utils/              # Utilities (cache, ratelimit, semantic_cache, tokenizer, crypto, …)
└── sitesync/           # Site sync, projection, and check-in implementation (the `site/` package is a thin facade over it)
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

The Hub remote site management uses an adapter-based architecture. Adapter packages cover the site types below; any site type without a dedicated adapter falls back to the `common` adapter (the One API / New API family fallback):

| Adapter | Site Type(s) |
|---------|-----------|
| `common` | `new-api`, `veloera`, `done-hub`, `one-hub`, `anyrouter`, `unknown` (One API / New API family fallback) |
| `octopus` | `octopus` (self-aware adapter) |
| `aihubmix` | `aihubmix` |
| `axonhub` | `axonhub` |
| `claudecodehub` | `claude-code-hub` |
| `sub2api` | `sub2api` |
| `sapi` | `sapi` (user account/password login with token caching) |

The `ldoh` package provides public site discovery (not an adapter).

Each adapter implements the [`SiteAdapter` interface](../../internal/hub/adapter.go) covering user info, check-in, models, pricing, tokens, channels, announcements, status, redemption, and usage logs.

**Frontend (Next.js 16 App Router):**

```
web/src/
├── api/               # API client & endpoint hooks (TanStack Query)
├── app/               # Next.js App Router pages
├── components/
│   ├── modules/       # Domain modules (channel, group, apikey, pool, remote-site, site, proxy-pool, model-mapping, credential, …)
│   ├── ui/            # UI primitives (Radix-based)
│   ├── common/        # Shared components
│   ├── nature/        # Animated backgrounds & effects
│   └── animate-ui/    # Animated UI components
├── hooks/             # Custom hooks
├── lib/               # Utilities, i18n, logger, time zone helpers
├── provider/          # React context providers
├── route/             # Lazy-loaded route config
├── stores/            # Zustand client state
└── test-utils/        # Test helpers (fetch mock, query client, timers)
```

## 🕐 Timezone Architecture

Octopus involves three independent timezone layers:

| Layer | Controlled By | Affects |
|-------|--------------|---------|
| **Container timezone** | `ENV TZ` / `-e TZ=` | Server log timestamps, `time.Now()` return value |
| **Stats timezone** | Admin UI → `stats_timezone` (IANA name, e.g. `Asia/Shanghai`) | Which date hourly/daily statistics roll into |
| **Frontend display timezone** | Admin UI → user preference (preset IANA time-zone list) | How timestamps appear on pages |

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

---

| [← Home](../Home.md) | [← Previous](12-Client-Integration.md) | |