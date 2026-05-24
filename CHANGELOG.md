# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v1.9.6] - 2026-05

### 🚀 Features
- Align management-side API contracts and tighten backend input boundaries.

### 🐛 Bug Fixes
- Keep edit and delete actions visible at the bottom of the mobile dialog.
- Preserve SuffixMode in ChannelBaseUrlDelayUpdate.
- Fix telemetry cache stats and deep-link generation in the ops center.
- Fix cache token metric unit loss in the analytics center.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.5...v1.9.6

---
## [v1.9.5] - 2026-05

### 🚀 Features
- Include circuit breaker runtime state in backup export and restore workflows.

### 🐛 Bug Fixes
- Allow API key IP allowlists to match client addresses that include ports.
- Allow the request origin in CSP `connect-src` for separated management console deployments.
- Fix telemetry latency percentile calculations so empty samples do not skew p95.
- Fix provider prompt cache token metric unit formatting in the ops dashboard.

### 🛠 Optimizations/Refactor
- Add OCI image metadata labels for Docker image version, revision, and build time.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.4...v1.9.5

---

## [v1.9.4] - 2026-05

### 🚀 Features
- Complete backup/restore overhaul.
- Add client IP logging and API key IP restriction.
- Expand endpoint provider rewrites and improve relay hardening.
- Add CCswitch deep link generator to group toolbar.

### 🐛 Bug Fixes
- Cap relay logs and audit logs backup export at 500k rows to prevent OOM.
- Hide availability test for non-conversation endpoint types.
- Resolve frontend-backend type mismatches.
- Add labels and contextual hints to alert rule form fields.
- Fix telemetry runtime metrics imports and wire session, sticky, quota alert, and quota monitor stats into the ops dashboard.
- Auto-detect git commit and build time for local builds.
- Include semantic cache hits and Anthropic cache writes in provider prompt cache trends.
- Remove redundant token unit suffix formatting in the UI.

### 🛠 Optimizations/Refactor
- Cap model picker height for better channel browsing ergonomics.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.3...v1.9.4

---

## [v1.9.3] - 2026-07

### 🚀 Features
- Support endpoint-provider specific rewrites for group relay and media passthrough.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.2...v1.9.3

---

## [v1.9.2] - 2026-05

### 🚀 Features
- Add model capabilities API and endpoint-aware model discovery.
- Add Model Market and Capabilities dual-view workflow in the management UI.
- Add endpoint capability aggregation and validation for group and relay model listing.

### 🐛 Bug Fixes
- Filter out groups without valid items or enabled channels when listing models.
- Narrow `*` group items by endpoint capability during media relay to avoid invalid upstream routing.

### 🛠 Optimizations/Refactor
- Refine model market summary trigger, capabilities filtering, and locale coverage.
- Add clearer logs for model-not-found relay failures.
- Refresh README model discovery and capabilities documentation.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.1...v1.9.2

---

## [v1.9.1] - 2026-05

### 🚀 Features
- Add architecture-focused Markdown refreshes for the current management console, relay pipeline, and embedded UI workflow.

### 🐛 Bug Fixes
- Correct hourly statistics keying and analytics time boundaries.
- Fix setting-cache backward compatibility while refreshing architecture documentation.

### 🛠 Optimizations/Refactor
- Split `internal/op/*` business logic into domain packages for AI routing, analytics, API keys, audit, backup, cache usage, channel, group, LLM metadata, nav order, ops, rate-limit state, relay logs, settings, stats, and users.
- Ignore local development log files generated during backend and frontend verification.

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.9.0...v1.9.1

---

## [v1.8.9] - 2025-04

### 🚀 Features
- Switch channel page to current-group view (@henryz78)

### 🐛 Bug Fixes
- Improve mimo reasoning budget handling (@lingyuins)
- Support removing failed models after group test (@lingyuins)
- Optimize group availability checks and channel group list (@lingyuins)
- Preserve renamed default group label (@henryz78)
- Avoid stretched group action button on mobile (@henryz78)
- Restore group dialog shell and cache trend layout (@henryz78)
- Refine channel dialog and cache trend view (@henryz78)
- Refine channel group labels and summary layout (@henryz78)
- Improve responsive stats cards (@henryz78)
- Avoid empty state card overflow on mobile (@henryz78)
- Improve mobile page scroll layout (@henryz78)

### 🛠 Optimizations/Refactor
- Stop tracking superpowers specs (@lingyuins)

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.8.8...v1.8.9

---

## [v1.8.7] - 2025-04

### 🚀 Features
- Add management groups (@henryz78)
- Move sync actions out of settings (@henryz78)
- Refine dashboard ranking and cache layout (@Lingyu)

### 🐛 Bug Fixes
- Sync prices after manual refresh (@henryz78)
- Align mobile preset picker actions (@henryz78)
- Improve error messages and mobile channel dialog (@henryz78)

### 🛠 Optimizations/Refactor
- Merge page sync actions pull request (@LingyuIns)

**Full Changelog:** https://github.com/lingyuins/octopus/compare/v1.8.6...v1.8.7

---

> **Note:** Earlier releases (v1.8.6 and below) are not recorded in this changelog.
> See the [GitHub Releases](https://github.com/lingyuins/octopus/releases) for the full history.
