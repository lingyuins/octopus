# Configuration

### 📝 Configuration File

The configuration file is located at `data/config.json` by default and is automatically generated on first startup.

**Minimal Configuration Example:**

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
| `server.trusted_proxies` | Comma-separated trusted reverse-proxy CIDRs/IPs for resolving real client IP from `X-Forwarded-For`. Empty = trust none (safe default; `c.ClientIP()` returns the direct TCP address). `*` = trust all (dev only; XFF spoofing risk). Applied once at startup — restart to apply changes; a DB setting of the same name overrides config/env when present. | empty |
| `database.type` | Database type | `sqlite` |
| `database.path` | Database connection string | `data/data.db` |
| `database.sqlite.cache_size` | SQLite `PRAGMA cache_size` (negative = KB, e.g. `-20000` ≈ 20 MB; positive = pages). Only used when `database.type` is `sqlite`. | `-20000` (≈ 20 MB) |
| `database.sqlite.mmap_size` | SQLite `PRAGMA mmap_size` in bytes. `0` disables mmap (safe default for low-memory hosts). | `0` (disabled) |
| `log.level` | Log level | `info` |
| `auth.jwt_secret` | JWT signing secret | empty (ephemeral secret generated at startup if unset) |
| `security.encryption_key` | Encryption key for sensitive stored data (credential profiles, site passwords, etc.) | empty (falls back to `auth.jwt_secret`; if that secret is ephemeral, startup is refused) |
| `database.log_type` | Optional dedicated log database type (SQLite / MySQL / PostgreSQL). Empty = relay logs are stored in the main database | empty |
| `database.log_path` | Log database connection string (used together with `database.log_type`) | empty |
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
| `OCTOPUS_DEBUG` | Enable debug mode: when `true`, the backend serves frontend assets from disk (`web/out` or `static/out`, whichever contains `index.html`) instead of the embedded UI | unset |

---

| [← Home](../Home.md) | [← Previous](01-Installation.md) | [Next →](03-Admin-Roles.md) |