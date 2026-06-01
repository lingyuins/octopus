# Memory Leak Troubleshooting

> **Applies to:** v1.3.x and above
> **Use case:** Long-running Octopus in Docker Desktop (WSL2) with TUN-mode proxy

## Problem

Octopus container's memory usage grows continuously over time, eventually causing the container to hang and become unresponsive. Only restarting Docker Desktop releases the memory.

## Root Cause

The scheduled sync task in `internal/task/sync.go` periodically sends HEAD / GET requests to every configured channel to sync model lists and measure latency.

When some channels are unreachable (TUN interception, DNS failure, upstream API blocked), the following happens:

1. These sync requests keep failing
2. Octopus does NOT stop on failure — Go goroutines keep retrying
3. Failed request buffers / goroutines are not reclaimed by the WSL2 kernel
4. RSS climbs steadily, eventually OOMing the container or host

## Reproduction Conditions

- Docker Desktop on Windows (WSL2 backend)
- Host uses a TUN-mode proxy (FlyShadow, Clash Meta for Windows, etc.)
- Container egress does NOT go through `host.docker.internal:<port>` style proxy
- Octopus has unstable / unreachable external channels

## Diagnosis

### 1. Observe Memory

```bash
docker stats octopus --no-stream
```

Under normal conditions, Octopus should use ~100 MB. If it climbs past 1 GB, something is wrong.

### 2. Check Sync Task Warnings

```bash
docker logs octopus --tail 200 | grep -E "WARN|ERROR"
```

Common warning patterns:

```
WARN  task/sync.go:43      failed to fetch models for channel XXX: EOF
WARN  helper/channel.go:47 failed to get url delay (channel=N): dial tcp: lookup XXX: no such host
```

## Temporary Mitigation

### Option A: Disable Unreachable Channels

In WebUI, set unreachable channels to `enabled = false`. The scheduled sync task will skip disabled channels.

If the WebUI does not show certain channels (e.g. cross-version import from bestruirui/octopus where field formats are incompatible), edit SQLite directly:

```bash
docker stop octopus
docker cp octopus:/app/data/data.db ./data.db
sqlite3 ./data.db "UPDATE channels SET enabled = 0 WHERE id IN (1, 2, 3, ...);"
docker cp ./data.db octopus:/app/data/data.db
docker run --rm -v octopus-data:/app/data alpine chown -R 1000:1000 /app/data
docker start octopus
```

### Option B: Cap Container Memory

```bash
docker run -d --name octopus \
  --restart unless-stopped \
  --memory 2g --memory-reservation 512m \
  -p 8080:8080 \
  -v octopus-data:/app/data \
  -e OCTOPUS_AUTH_JWT_SECRET="your-secret" \
  -e HTTP_PROXY="http://host.docker.internal:6785" \
  -e HTTPS_PROXY="http://host.docker.internal:6785" \
  -e NO_PROXY="localhost,127.0.0.1,host.docker.internal" \
  lingyuins/octopus:latest
```

## Long-Term Improvements

1. **Circuit breaker** in `internal/task/sync.go`: auto-pause sync for a channel after N consecutive failures for M minutes
2. **WebUI "show all channels" toggle** to help clean up incompatible data from old-version imports
3. **Prometheus / metrics endpoint** exposing per-channel failure count and last successful sync time

## Verification

In the reporter's environment (Windows 11 + Docker Desktop + WSL2 + FlyShadow TUN), disabling 13 unreachable channels brought container memory back to a steady ~100 MB and eliminated sync task warnings.
