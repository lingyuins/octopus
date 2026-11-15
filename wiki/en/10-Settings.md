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

**Key settings cards in the current UI** (the authoritative list is `SETTING_ITEM_DEFS` in `web/src/components/modules/setting/index.tsx`):

| Card | Purpose |
|------|---------|
| Info | Current version, latest release lookup, cache-mismatch detection, and in-place self-update entry with version mismatch notification |
| Appearance | Theme, locale, alert language, drag-and-drop top-level navigation order, and per-page visibility toggles |
| AI Route | Primary analysis service, timeout, parallelism, models per batch, and service-pool configuration; model suggestions save on main-service selection/blur, while service-pool edits require explicit save |
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
| Local Nodes | Online status, connector version, in-flight/limit and last seen; admin-only create, availability, token rotation and deletion |

**Local node setup:**

1. Open `Settings → Local Nodes` as an admin and create a node. Editors and viewers can read status, but cannot manage nodes or change channel bindings.
2. Copy the one-time token/configuration before closing the panel. Tokens are not stored in browser storage or query caches; rotate if lost. Rotation invalidates the old credential. Disable/rotate/delete require confirmation; delete is rejected while channels reference the node.
3. Install the independent `octopus-to-magpie-connect` community plugin in Magpie. Fill in its cloud WSS URL, exact model allowlist, node token, Magpie API key and local port (default 3425). The host is fixed to `127.0.0.1`. Generated configuration uses `ws://127.0.0.1` for local HTTP deployments and retains WSS for remote deployments. Page credentials are saved locally in plain text; environment variables or an external private file are alternatives. The split configuration reference must not be loaded as a whole.
   The per-request body limit is the smaller of the cloud setting and the Connector's advertised handshake limit. Current Connectors advertise 16 MiB; legacy peers that omit the field retain their original 4 MiB limit. Larger requests require both a Connector upgrade and a matching cloud node-transport setting.
4. Confirm **Request connection**, then **fully quit and restart Magpie**; refreshing the page is not enough. No separate Connector process or Magpie source modification is needed. The Bun host remains lazy and may need a normal plugin-using operation to start. Reloads can interrupt requests without replay.
5. Click **Test connection** on the node card. The admin-only `POST /api/v1/nodes/:id/test` requests only `/v1/models` through the existing tunnel, with a short timeout and bounded response. It reports offline/disabled/busy nodes, gateway connectivity, authentication and model-list problems without inference or arbitrary URL access.
6. Select the node in a channel's connection selector. URL/proxy/key inputs are hidden, account-pool/proxy combinations are disallowed, and the server generates internal placeholders. Discovery and tests use `node_id`, not a Magpie credential. Node channel tests show detailed diagnostics and never automatically fall back to paid model calls.


> **Note:** The following settings have been relocated to more relevant modules (issue #87):
> - **Retry / Circuit Breaker / Response Filter** → `Ops → Maintenance` tab
> - **Site Automation** → `Hub → Automation` tab
> - **Purge Unavailable Models / Delete All Route Groups** → `Group` page "Maintenance" dropdown button

**Semantic Cache Scope:**

- Applies to non-streaming and streaming OpenAI Chat and OpenAI Responses text requests
- Streaming cache hits replay from the SSE session buffer with stable stream-session recovery
- Only text requests without tools, multimodal content, or stateful continuation references are cacheable; model, output parameters, and routing/settings generations are isolated exactly
- TTL expires entries after their last access; configuration changes can invalidate old entries
- Identical non-streaming requests are coalesced only while the cache is runtime-enabled, using an exact request fingerprint; shared/cache results do not add physical upstream calls or cost
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
- It deletes all groups and group items without changing other settings

**Settings Card Order:**

The Settings page supports drag-and-drop reordering of its card sections, with order persisted to local storage. A "Reset to Default" button restores the original order.

> ⚠️ **Important**: When exiting the program, use proper shutdown methods (like `Ctrl+C` or sending `SIGTERM` signal) to ensure in-memory statistics are correctly written to the database. **Do NOT use `kill -9` or other forced termination methods**, as this may result in statistics data loss.

---

| [← Home](../Home.md) | [← Previous](09-Ops.md) | [Next →](11-Hub-Sites.md) |