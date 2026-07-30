# AGENTS.md

## Project

Fork of **logrus** — a structured logger for Go. Module path: `github.com/bnulwh/logrus`. Maintenance-mode; no new features, only bug fixes and backwards-compatible changes.

## Commands

- **Test:** `go test -race -v ./...`
- **Lint:** `golangci-lint run ./...` (config in `.golangci.yml`; linters: megacheck, govet; tests excluded from lint)
- **Cross-build:** `cd ci && go run mage.go -v -w ../ crossBuild`
- **Benchmarks:** `go test -bench=.*` (caller tracing overhead: `go test -bench=.*CallerTracing`)

CI runs: lint → cross-build → test (in that order).

## Architecture

Single Go package at repo root (`package logrus`). No subpackages except:
- `hooks/test` — test hook utilities (`NewNullLogger`, `NewLocal`, `NewGlobal`)
- `hooks/writer` — io.Writer hook
- `hooks/syslog` — syslog hook (build-tag gated for non-Windows)
- `internal/testutils` — shared test helpers using dot-import of logrus + testify/require

Core files:
- `logger.go` — `Logger` struct, the main type
- `entry.go` — `Entry` struct (per-log-event object)
- `logrus.go` — types: `Level`, `Fields`, `StdLogger`, `FieldLogger`
- `exported.go` — package-level convenience functions delegating to the standard logger (`std`)
- `formatter.go` / `text_formatter.go` / `json_formatter.go` / `simple_formatter.go` — formatters
- `hooks.go` — `Hook` interface and `LevelHooks`
- `alt_exit.go` — `RegisterExitHandler` / exit handler machinery
- `lfs_hook.go` / `lfs_exported.go` — local filesystem log hook with `ConfigLocalFileSystemLogger` (uses `file-rotatelogs`)
- `writer.go` — `Logger.Writer()` returning `io.Writer`
- `terminal_check_*.go` — build-tag-split TTY detection per OS
- `buffer_pool.go` — sync.Pool for buffers

## Conventions

- Import path must be lowercase: `github.com/bnulwh/logrus` (case-sensitivity is a known issue).
- Uses `vendor/` directory (not just go.mod).
- `ci/` is a separate Go module for mage tasks; runs from `ci/` with `-w ../`.
- Tests use `stretchr/testify` (assert + require).
- `internal/testutils` dot-imports logrus for test brevity.
- `hooks/test` is the preferred way to assert log output in tests (`test.NewNullLogger`).
