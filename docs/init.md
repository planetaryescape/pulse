# Pulse - CLI Git Activity Scanner

## Context
Building a new Go CLI tool that recursively scans directories for Git repos and summarizes their status. The key architectural requirement is **clean separation between core logic and rendering**, so the rendering layer can be swapped out later (e.g., OpenTUI + SolidJS frontend consuming JSON from the Go core).

## Project Structure

```
pulse/
├── go.mod
├── cmd/
│   └── pulse/
│       └── main.go              # CLI entry point (wiring only)
├── internal/
│   ├── core/
│   │   ├── types.go             # Core data structures (the contract)
│   │   ├── scanner.go           # Directory traversal + orchestration
│   │   ├── analyzer.go          # Single repo git analysis
│   │   └── pool.go              # Goroutine worker pool
│   └── cli/
│       ├── render.go            # CLI table + JSON rendering
│       └── flags.go             # CLI flag parsing
└── pkg/
    └── pulse/
        └── pulse.go             # Public API: pulse.Run(config) → ScanResult
```

**Separation principle**: `internal/core` returns pure Go structs, never does formatting. `internal/cli` is a thin presentation layer. `pkg/pulse` is the public API any consumer (CLI, HTTP server, future frontend) calls.

## Core Data Structures (`internal/core/types.go`)

- `ScanConfig` — root path, max depth, detail mode flag, ghost threshold, worker count
- `RepoStatus` — name, path, branch, clean/dirty, last commit time, unpushed/unpulled counts, ghost flag, recent commits (detail mode), lines changed stats (detail mode)
- `Commit` — hash, author, message, timestamp
- `LinesChangedStats` — added, removed, period
- `ScanResult` — list of RepoStatus, total count, daily commit tally, scan duration, errors
- `ScanError` — path + message for non-fatal errors during scanning

## Dependencies

- `github.com/go-git/go-git/v5` — pure Go git (no git binary needed)
- `github.com/karrick/godirwalk` — fast directory traversal
- `github.com/olekukoneko/tablewriter` — CLI table formatting
- `github.com/fatih/color` — status coloring (red dirty, green clean)

## Implementation Phases

### Phase 1: Foundation (MVP)
1. `go mod init github.com/guidefari/pulse`
2. Define core types in `internal/core/types.go`
3. Implement scanner (`internal/core/scanner.go`) — use godirwalk to find `.git` dirs, respect max depth, stop descending into repos
4. Implement analyzer (`internal/core/analyzer.go`) — open repo with go-git, get branch, check worktree status, get last commit time
5. Implement worker pool (`internal/core/pool.go`) — channel-based goroutine pool to analyze repos in parallel
6. Wire up CLI flags (`internal/cli/flags.go`) — `--path`, `--depth`, `--detail`, `--format`
7. Implement table renderer (`internal/cli/render.go`) — render ScanResult as colored table
8. Create main entry point (`cmd/pulse/main.go`) — parse flags → build config → call core → render
9. Create public API (`pkg/pulse/pulse.go`) — `Run(ScanConfig) (*ScanResult, error)`

### Phase 2: Advanced Features
10. Unpushed/unpulled detection — compare HEAD with remote tracking branch
11. Ghost repo detection — flag repos with last commit >6 months ago
12. Detail mode — last 5 commits + lines changed in last 7 days
13. Daily commit tally — aggregate commits across all repos for today
14. JSON output via `--format json`

## How the Separation Works

```
CLI:        flags → ScanConfig → pulse.Run() → ScanResult → RenderTable()
Future API: HTTP JSON body → ScanConfig → pulse.Run() → ScanResult → json.Encode()
Future TUI: UI state → ScanConfig → pulse.Run() → ScanResult → OpenTUI render
```

The core never imports presentation packages. The core never calls fmt.Print. It just returns structs.

## CLI Usage

```bash
pulse --path ~/source                        # basic table
pulse --path ~/source --depth 5 --detail     # detail mode
pulse --path ~/source --format json          # JSON output
```

## Current Status

**Phase 1: Complete.** All MVP features are implemented and working.

**Phase 2: Complete.** All advanced features shipped in the initial build:
- Unpushed/unpulled detection (↑/↓ indicators)
- Ghost repo detection (👻 for repos inactive >6 months)
- Detail mode (`--detail`) — last 5 commits + lines changed in last 7 days
- Daily commit tally across all repos
- JSON output (`--format json`)

### Known Observations
- Scan of ~26 repos takes ~24s — most time spent in `Worktree.Status()` (go-git iterates the full working tree). Acceptable for now.
- One edge case: repos with staged-only changes may show `✘ 0 changed` — the status map length doesn't always match the dirty file count. Minor display issue.

## What's Left / Next Steps

### Performance
- **Worktree status is slow** — could shell out to `git status --porcelain` instead of go-git's `Worktree.Status()` for a 10-20x speedup on large repos. Trade-off: requires git binary installed.
- **Caching** — cache scan results with a TTL so repeated runs don't re-scan unchanged repos.

### Features
- **Language breakdown** — use `gocloc` or similar to show primary languages per repo (e.g., "90% Go, 10% Markdown").
- **Config file** — support a `.pulserc` or similar for default path/depth so you don't have to pass flags every time.
- **Filter/search** — filter repos by name, branch, or dirty status (e.g., `--dirty` to only show repos with uncommitted changes).
- **Watch mode** — re-scan on an interval and refresh the table.

### Rendering Layer Swap
The architecture is ready for alternative frontends:
1. **HTTP/JSON API** — add `internal/server/handler.go` + `cmd/pulse-server/main.go`. Calls the same `pulse.Run()`, returns JSON over HTTP. Zero changes to core.
2. **OpenTUI + SolidJS** — consume the JSON API from a SolidJS frontend. The Go core stays as-is, just add the server entrypoint.

### Polish
- **Install script / `go install`** — make it easy to install globally.
- **`.gitignore`** — add the compiled binary.
- **Error reporting** — surface scan errors more visibly in table mode.

## Verification
1. `go build ./cmd/pulse` — should compile
2. `./pulse --path ~/source/oss` — should scan and display table of repos
3. `./pulse --path ~/source/oss --format json | jq .` — should output valid JSON
4. `./pulse --path ~/source/oss --detail` — should show recent commits for each repo
