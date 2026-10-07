<div align="center">

<img src="../web/public/logo.svg" alt="Octopus Logo" width="120" height="120">

### Octopus Wiki

**A Simple, Beautiful, and Elegant LLM API Aggregation & Load Balancing Service for Individuals**

[README](../README.md) | [Changelog](../CHANGELOG.md) | [简体中文](Home_zh.md)

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
- 👥 **Account Pools** - Pool upstream accounts (OAuth / API key / cookie credentials) with dedicated scheduling, health probes, token auto-refresh, and scheduled connectivity tests
- 📜 **Relay & Error Logs** - Dual log views: relay logs with per-candidate retries, and error logs collecting backend panics plus frontend exceptions
- 🧾 **API Key Governance** - Supported-model allowlists, expiry, max-cost caps, RPM / TPM limits, per-model quotas, and IP / CIDR allowlists
- 🔐 **Role-Based Admin Access** - Built-in `admin`, `editor`, and `viewer` roles with server-side permission enforcement
- 🔑 **WebAuthn / Passkey Login** — Passwordless login and registration via WebAuthn/Passkey with configurable RP settings
- 🚨 **Alerts & Notification Center** - Unified notification center aggregating system events, alert firings, and plan notifications with SSE streaming
- 📦 **Plan Provider Monitoring** - Track upstream subscription quota/usage and auto-create dedicated forwarding channels
- 📅 **Usage Reports** - Schedule daily / weekly / monthly usage reports delivered through notification channels
- 💎 **Model Market** - Unified model catalog with pricing, channel coverage, enabled key counts, latency, and success metrics
- 🔃 **Model Sync** - Automatic synchronization of available model lists with channels
- 📊 **Analytics & Evaluation** - Overview, provider / model / API key utilization, route health, latency distribution, semantic-cache evaluation
- 🛠️ **Ops & Audit** - Telemetry, quota, health, system, and audit dashboards for daily operations
- 🧠 **Semantic Cache** - Embedding-backed semantic cache for non-streaming and streaming OpenAI Chat / Responses text requests
- 🧭 **Configurable Navigation** - Persist top-level console page order and visibility in settings
- 💾 **Runtime State Persistence** - Persist auto strategy windows and circuit breaker state to the database
- 🔗 **Site Management** - Manage upstream relay platforms with multi-account support, projected channels, auto-sync, and auto-checkin
- 🌍 **Proxy Pool** - Named proxy configurations with direct / system / pool / inherit modes and reference tracking
- 🔁 **Model Mapping** - Global model name rewriting rules with exact, wildcard, and regex matching
- ☁️ **WebDAV Cloud Backup** - Automated cloud backup via WebDAV with configurable schedule and one-click restore
- 🔑 **API Credential Profiles** - Reusable Base URL + API Key pairs with health verification probes and CLI config export
- 📤 **CLI Config Export** - Generate configuration snippets for Claude Code, Codex, Gemini CLI, Cherry Studio, and Kilo Code
- 🎨 **Elegant UI** - Clean and beautiful web management panel with dark mode, activity heatmap, share snapshot, and responsive mobile layout
- 🗄️ **Multi-Database Support** - Support for SQLite, MySQL, PostgreSQL with live migration between database types

---

## 📚 Documentation Index

| # | Page | What it covers |
|---|------|----------------|
| 01 | [Installation](en/01-Installation.md) | Docker / Release / source build, initial admin setup |
| 02 | [Configuration](en/02-Configuration.md) | config.json, environment variables, SQLite tuning, database types |
| 03 | [Admin Roles](en/03-Admin-Roles.md) | admin / editor / viewer roles, WebAuthn/Passkey |
| 04 | [Channels](en/04-Channels.md) | Channel templates, base URLs, proxy mode, request rewrite, param override, key strategy |
| 05 | [Groups](en/05-Groups.md) | Group management, load balancing, model discovery & capabilities |
| 06 | [Model Market](en/06-Model-Market.md) | Model catalog, pricing, coverage, capabilities dual-view |
| 07 | [Relay Endpoints](en/07-Relay-Endpoints.md) | Public relay API, Zen routing, model mapping, proxy pool, account pools |
| 08 | [Analytics](en/08-Analytics.md) | Channel×Model, usage breakdown, route health, latency, evaluation, cache |
| 09 | [Ops](en/09-Ops.md) | Telemetry, quota, health, maintenance, system, audit |
| 10 | [Settings](en/10-Settings.md) | Settings cards, semantic cache, DB migration, dangerous ops |
| 11 | [Hub & Sites](en/11-Hub-Sites.md) | Site management, WebDAV backup, API credentials, CLI export, notifications |
| 12 | [Client Integration](en/12-Client-Integration.md) | OpenAI SDK, Claude Code, Codex, CLI export |
| 13 | [Architecture](en/13-Architecture.md) | Layered architecture, relay data flow, hub adapters, timezone, security |

---

## 📸 Screenshots

> Captured from v2.6.3-fix on 2026-10-07 using an isolated instance with synthetic demo data. Desktop viewport: 1440 × 960; mobile viewport: 430 × 932. No production accounts or credentials are shown.

### 🖥️ Desktop

<div align="center">
<table>
<tr>
<td align="center"><b>Dashboard</b></td>
<td align="center"><b>Channel Management</b></td>
<td align="center"><b>Group Management</b></td>
</tr>
<tr>
<td><img src="../web/public/screenshot/desktop-home.png" alt="Dashboard" width="400"></td>
<td><img src="../web/public/screenshot/desktop-channel.png" alt="Channel" width="400"></td>
<td><img src="../web/public/screenshot/desktop-group.png" alt="Group" width="400"></td>
</tr>
<tr>
<td align="center"><b>Model Market</b></td>
<td align="center"><b>Logs</b></td>
<td align="center"><b>Settings</b></td>
</tr>
<tr>
<td><img src="../web/public/screenshot/desktop-price.png" alt="Model Market" width="400"></td>
<td><img src="../web/public/screenshot/desktop-log.png" alt="Logs" width="400"></td>
<td><img src="../web/public/screenshot/desktop-setting.png" alt="Settings" width="400"></td>
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
<td><img src="../web/public/screenshot/mobile-home.png" alt="Mobile Home" width="140"></td>
<td><img src="../web/public/screenshot/mobile-channel.png" alt="Mobile Channel" width="140"></td>
<td><img src="../web/public/screenshot/mobile-group.png" alt="Mobile Group" width="140"></td>
<td><img src="../web/public/screenshot/mobile-price.png" alt="Mobile Model Market" width="140"></td>
<td><img src="../web/public/screenshot/mobile-log.png" alt="Mobile Logs" width="140"></td>
<td><img src="../web/public/screenshot/mobile-setting.png" alt="Mobile Settings" width="140"></td>
</tr>
</table>
</div>

---

| → Next | [Installation](en/01-Installation.md) |
|--------|---------------------------------------|
