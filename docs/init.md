# Pulse - CLI Git Activity Scanner

## Context
A Go CLI tool that recursively scans directories for Git repos and summarizes their status. The key architectural requirement is **clean separation between core logic and rendering**, so the rendering layer can be swapped out later (e.g., OpenTUI + SolidJS frontend consuming JSON from the Go core).

## Project Structure

```
pulse/
├── go.mod
├── justfile                       # Task runner (just) for common operations
├── CLAUDE.md                      # AI assistant guidance
├── cmd/
│   └── pulse/
│       └── main.go                # CLI entry point (wiring only)
├── internal/
│   ├── core/
│   │   ├── types.go               # Core data structures (the contract)
│   │   ├── scanner.go             # Directory traversal + orchestration
│   │   ├── analyzer.go            # Git repo analysis (go-git + git CLI hybrid)
│   │   ├── pool.go                # Channel-based goroutine worker pool
│   │   ├── analyzer_bench_test.go # Benchmarks for analyzer operations
│   │   └── scanner_bench_test.go  # Benchmarks for scanning operations
│   ├── cli/
│   │   ├── render.go              # CLI table + JSON + detail + timing output
│   │   └── flags.go               # CLI flag parsing
│   └── tracing/
│       └── tracing.go             # OpenTelemetry in-memory tracing
├── pkg/
│   └── pulse/
│       └── pulse.go               # Public API: pulse.Run(ctx, config) → *ScanResult
└── docs/
    └── init.md                    # This file
```

**Separation principle**: `internal/core` returns pure Go structs, never does formatting. `internal/cli` is a thin presentation layer. `pkg/pulse` is the public API any consumer (CLI, HTTP server, future frontend) calls.

## Core Data Structures (`internal/core/types.go`)

- `ScanConfig` — root path, max depth, detail mode flag, ghost threshold, worker count
- `RepoStatus` — name, path, branch, clean/dirty, last commit time, unpushed/unpulled counts, ghost flag, recent commits (detail mode), lines changed stats (detail mode)
- `Commit` — hash, author, message, timestamp
- `LinesChanged` — added, removed, period
- `ScanResult` — list of RepoStatus, total count, daily commit tally, scan duration, errors
- `ScanError` — path + message for non-fatal errors during scanning

## Dependencies

- `github.com/go-git/go-git/v5` — pure Go git for branch, commits, remote status
- `github.com/karrick/godirwalk` — fast directory traversal
- `github.com/olekukonko/tablewriter` — CLI table formatting
- `github.com/fatih/color` — status coloring (red dirty, green clean)
- `go.opentelemetry.io/otel` — performance tracing (in-memory collector)

## Technical Decisions

- **Hybrid git**: Uses `go-git` for most operations but shells out to `git status --porcelain` for worktree status (10-20x faster than go-git's `Worktree.Status()`). Trade-off: requires git binary installed.
- **godirwalk** for directory traversal (faster than `filepath.Walk`). Scanner skips `node_modules`, `.Trash`, `vendor` and stops descending into found repos.
- **OpenTelemetry** for internal performance tracing, rendered as waterfall timelines with `--time`.
- **Results sorted** by last commit time (most recently active first) via `SortByLastActive`.

## Data Flow

```
CLI:        flags → ScanConfig → pulse.Run(ctx, config) → *ScanResult → Render()
Future API: HTTP JSON body → ScanConfig → pulse.Run(ctx, config) → *ScanResult → json.Encode()
Future TUI: UI state → ScanConfig → pulse.Run(ctx, config) → *ScanResult → OpenTUI render
```

The core never imports presentation packages. The core never calls fmt.Print. It just returns structs.

## CLI Flags

| Flag       | Default   | Description                          |
|------------|-----------|--------------------------------------|
| `--path`   | `.`       | Root directory to scan for git repos |
| `--depth`  | `3`       | Maximum directory depth to scan      |
| `--detail` | `false`   | Show recent commits + lines changed  |
| `--format` | `table`   | Output format: `table` or `json`     |
| `--time`   | `false`   | Show performance timing breakdown    |

## CLI Usage

```bash
just scan                          # basic table (~/source, depth 3)
just detail                        # detail mode
just json                          # JSON output
just time                          # performance timing waterfall
just full                          # detail + time combined
just scan ~/projects 2             # override path and depth
just run --path ~/source --depth 5 # raw flag passthrough
```

## Current Status

**Phase 1 (Foundation) and Phase 2 (Advanced Features): Complete.**

All features shipped and working:
- Basic table output with branch, clean/dirty status, last active time
- Unpushed/unpulled detection (↑/↓ indicators)
- Ghost repo detection (👻 for repos inactive >6 months)
- Detail mode (`--detail`) — last 5 commits + lines changed in last 7 days
- Daily commit tally across all repos
- JSON output (`--format json`)
- Performance tracing (`--time`) — OpenTelemetry waterfall visualization
- Hybrid git approach — `git status --porcelain` for worktree, go-git for everything else
- Benchmark test suite for analyzer and scanner performance profiling

## What's Left / Next Steps

### Features
- **Language breakdown** — use `gocloc` or similar to show primary languages per repo
- **Config file** — support a `.pulserc` or similar for default path/depth
- **Filter/search** — filter repos by name, branch, or dirty status (e.g., `--dirty`)
- **Watch mode** — re-scan on an interval and refresh the table
- **Caching** — cache scan results with a TTL so repeated runs don't re-scan unchanged repos

### Rendering Layer Swap
The architecture is ready for alternative frontends:
1. **HTTP/JSON API** — add `internal/server/handler.go` + `cmd/pulse-server/main.go`. Calls the same `pulse.Run()`, returns JSON over HTTP. Zero changes to core.
2. **OpenTUI + SolidJS** — consume the JSON API from a SolidJS frontend. The Go core stays as-is, just add the server entrypoint.

### Polish
- **Install script / `go install`** — make it easy to install globally
- **Error reporting** — surface scan errors more visibly in table mode
