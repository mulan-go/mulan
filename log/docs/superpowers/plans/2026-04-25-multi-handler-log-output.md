# Multi Handler Log Output Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add configurable local console and server JSON logging with independent output levels.

**Architecture:** Keep `NewWithConfig` as the public entry point and move multi-output core construction into a focused helper. Preserve existing `Config.Level`, `Config.JSON`, and adaptor DSNs while adding mode-aware defaults and per-console level configuration.

**Tech Stack:** Go, zap, zapcore, existing file/http writer adaptors.

---

### Task 1: Behavior Tests

**Files:**
- Modify: `log_test.go`

- [ ] Add tests that redirect stdout and assert local console output contains colorized levels when JSON is disabled.
- [ ] Add tests that assert JSON mode emits parseable JSON.
- [ ] Add tests that file adaptors inherit the global level when no DSN level is present.
- [ ] Add tests that file adaptors can override the global level through `?level=debug`.
- [ ] Run `go test ./...` and confirm the new tests fail before implementation.

### Task 2: Multi Output Builder

**Files:**
- Modify: `config.go`
- Modify: `dsn.go`
- Modify: `log.go`

- [ ] Add `Mode`, `ConsoleLevel`, and `Format` fields to `Config`.
- [ ] Add helpers to resolve mode defaults, encoder format, and per-output levels.
- [ ] Track whether a DSN level was explicitly configured so adaptor cores inherit the global level by default.
- [ ] Replace inline core assembly in `NewWithConfig` with a small multi-handler builder that owns cores and closers.
- [ ] Run `go test ./...` and fix failures.

### Task 3: Documentation

**Files:**
- Modify: `README.md`

- [ ] Document local debug configuration.
- [ ] Document server JSON configuration.
- [ ] Document independent console and adaptor level behavior.
- [ ] Run `go test ./...`.
