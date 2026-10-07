# Settings

## ⚙️ Settings

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

**Key settings cards in the current UI:**

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
| Backup | Database export, import, and live database migration between SQLite / MySQL / PostgreSQL with connection testing and per-table row count results. The exported dump contains sensitive fields (channel keys, API keys, site tokens) in plaintext — keep backup files confidential |
| Redis | Optional Redis cache backend configuration: connection settings, test connection, and save (restart to apply). Unloads stats, runtime state, rate-limit/cooldown, and channel-delay probing to Redis for low-memory hosts and multi-instance scaling |
| WebDAV Backup | WebDAV cloud backup configuration: connection settings, auto-backup interval, max backups retention, manual trigger, remote file listing, restore, and delete |
| WebAuthn / Passkey | RP ID, RP name, allowed origins configuration |
| Normalize | Model-name normalization rules: router prefixes, functional suffixes, and explicit variant→canonical mappings (runtime-configurable, with an offline AI-assisted normalization workflow) |
| Pool | Account-pool scheduling and health tuning: probe interval, failure threshold, minimum priority, unschedulable rules, sticky escape, weight reset / quota |
| Proxy Pool | Not an inline panel — opens the shared proxy pool dialog |

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

| [← Home](../Home.md) | [← Previous](09-Ops.md) | [Next →](11-Hub-Sites.md) |