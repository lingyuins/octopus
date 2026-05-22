# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
