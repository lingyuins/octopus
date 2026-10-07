# Hub, Sites, Backup, Credentials & Notifications

## 🔗 Site Management (Hub → Sites)

The Hub module's **Sites** tab manages upstream relay platforms as a first-class entity. Sites represent platforms like New-API, One-API, One-Hub, Done-Hub, Sub2API, AnyRouter, OpenAI, Claude, and Gemini. Each site renders as a multi-account card with inline balance, sync status, and check-in status, replacing the previous multi-tab Hub layout.

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

## ☁️ WebDAV Cloud Backup

Automated cloud backup via WebDAV with full lifecycle management:

- Configurable base URL, credentials, remote path, auto-backup interval (default 6 hours), and max backups retention
- Connection testing before enabling
- Manual backup trigger
- Remote backup file listing with size info
- One-click restore from any remote backup
- Delete remote backups
- Included in the Settings page as a dedicated card

**What a backup contains — handle with care:**

The backup is a JSON dump of the database, same format as the Settings → Backup export. It always includes the core tables (channels, keys, groups, API keys, settings, alerts, notifications, runtime states), the Site tables, and the Hub tables `remote_sites`, `balance_snapshots`, `check_in_records`, `api_credential_profiles`, `site_announcements`, and `remote_site_tokens`. Relay logs and audit logs (the "include logs" option) and statistics (the "include stats" option) are only exported when selected. Account pool tables and Hub usage-history records (`remote_usage_records`) are not part of the dump.

> ⚠️ **Confidentiality**: In the current dump format (v2), sensitive fields — channel keys, API keys, site access tokens / passwords, credential API keys — are exported in **plaintext** (so a backup restores correctly on another instance; import re-encrypts them with the target instance's key). Anyone holding the backup file holds live credentials: store it encrypted / access-restricted, and never commit or share it.

---

## 🔑 API Credential Profiles & CLI Export

Reusable API credential profiles store Base URL + API Key pairs for quick access:

- Health verification probes: `text_gen`, `models_list`, `tool_calling`, `structured_output`
- Health status tracking per credential
- Encryption at rest via `security.encryption_key`
- Tags and notes for organization

**CLI Config Export:**

Generate ready-to-use configuration snippets for the following client tools:

| Tool | Format |
|------|--------|
| Claude Code | Environment variables for `~/.claude/settings.json` |
| Codex | Environment variables for `~/.codex/auth.json` and `config.toml` |
| Gemini CLI | Environment variables |
| Cherry Studio | JSON provider import configuration |
| Kilo Code | JSON settings block |

---

## 🚨 Notification Center & Alerts

The Notification module is a unified center aggregating system events, alert firings, and plan-provider notifications with severity levels, read/archive state, filtering, and SSE streaming for real-time delivery. Alert rules monitor system health and trigger notifications:

**Alert rule types:** Error rate (with configurable scope — per-channel / per-group / global — and sliding-window evaluation), cost threshold, quota exceeded, channel down, and pool account error (fires while any — or the scoped — pool account stays in the error state).

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

| [← Home](../Home.md) | [← Previous](10-Settings.md) | [Next →](12-Client-Integration.md) |