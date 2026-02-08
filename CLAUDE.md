# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Pulse is a Go CLI tool that recursively scans directories for Git repositories and displays a status summary (branch, clean/dirty, ahead/behind, ghost detection, recent commits).

## Commands

```bash
just build          # compile to ./pulse
just run [ARGS]     # go run ./cmd/pulse with args
just test           # go test ./...
just bench          # go test -bench=. -benchmem ./...
just vet            # go vet ./...
just check          # lint + test
just tidy           # go mod tidy
```

Benchmarks use env vars `PULSE_BENCH_REPO` and `PULSE_BENCH_PATH` for target paths.

## Architecture

```
cmd/pulse/main.go       → CLI entry point (flag parsing, wiring, rendering)
pkg/pulse/pulse.go      → Public API: pulse.Run(ctx, config) → *ScanResult
internal/core/          → Core business logic (no I/O formatting)
  types.go              → Data contracts: ScanConfig, RepoStatus, ScanResult
  scanner.go            → Directory traversal using godirwalk
  analyzer.go           → Git repo analysis (go-git + shelling out for worktree status)
  pool.go               → Channel-based goroutine worker pool
internal/cli/           → Presentation layer
  flags.go              → CLI flag definitions
  render.go             → Table/JSON output, timing visualization
internal/tracing/       → OpenTelemetry performance tracing (in-memory collector)
```

**Data flow**: CLI flags → `ScanConfig` → `Scanner.Scan()` → Worker Pool → `Analyzer` → `ScanResult` → Renderer

**Key design rule**: `internal/core/` never does formatting or I/O — it returns pure structs. This enables swapping frontends (HTTP API, TUI) without touching core logic.

## Technical Decisions

- **Hybrid git**: Uses `go-git` for most operations but shells out to `git status --porcelain` for worktree status (10-20x faster).
- **godirwalk** for directory traversal (faster than `filepath.Walk`).
- **OpenTelemetry** for internal performance tracing, rendered as waterfall timelines with `--time`.
- Scanner skips `node_modules`, `.Trash`, `vendor` and stops descending into found repos.
