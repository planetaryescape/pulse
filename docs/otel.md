# OpenTelemetry in `pulse`

This document explains how OpenTelemetry (OTEL) is implemented in this codebase.

## Scope

OTEL here is used for local, in-process performance tracing of a single CLI run.

- Enabled only when `--time` is passed.
- Exported to an in-memory collector.
- Rendered back to the terminal as timing tables/charts.
- Not exported to OTLP or any remote backend.

## Quick Flow

```text
CLI (--time)
  -> tracing.Init(enabled)
      -> set global tracer provider
      -> use CollectingExporter (in memory)
  -> scanner/analyzer create spans
  -> shutdown() flushes spans
  -> cli.RenderTimings(exporter) prints breakdown + charts
```

## Initialization

`cmd/pulse/main.go` wires tracing at program startup:

```go
cliConfig := cli.ParseFlags()
exporter, shutdown := tracing.Init(cliConfig.ShowTimings)
ctx := context.Background()

result, err := pulse.Run(ctx, config)
if err != nil {
    // ...
}

shutdown(ctx)

if cliConfig.ShowTimings {
    cli.RenderTimings(exporter)
}
```

`internal/tracing/tracing.go` decides between noop and real tracing:

```go
func Init(enabled bool) (*CollectingExporter, func(context.Context) error) {
    if !enabled {
        otel.SetTracerProvider(noop.NewTracerProvider())
        return nil, func(context.Context) error { return nil }
    }

    exp := &CollectingExporter{}
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exp)),
    )
    otel.SetTracerProvider(tp)
    return exp, tp.Shutdown
}
```

## Exporter Design

The exporter stores spans in memory for later rendering:

```go
type CollectingExporter struct {
    mu    sync.Mutex
    spans []sdktrace.ReadOnlySpan
}

func (e *CollectingExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
    e.mu.Lock()
    e.spans = append(e.spans, spans...)
    e.mu.Unlock()
    return nil
}
```

Implications:

- No network export path.
- No external OTEL collector dependency.
- Data lifetime is one process execution.

## Span Instrumentation

### Top-level scan spans

`internal/core/scanner.go`:

```go
ctx, findSpan := tracing.Tracer().Start(ctx, "find_repos")
found, err := s.findRepos()
findSpan.End()

_, processSpan := tracing.Tracer().Start(ctx, "process")
statuses, scanErrors := pool.Process(ctx, found.repos, analyzer)
processSpan.End()
```

### Per-repo analyze span and children

`internal/core/analyzer.go`:

```go
ctx, span := tracing.Tracer().Start(ctx, "analyze",
    trace.WithAttributes(attribute.String("repo", filepath.Base(repoPath))),
)
defer span.End()

_, plainSpan := tracing.Tracer().Start(ctx, "plain_open")
repo, err := git.PlainOpen(repoPath)
plainSpan.End()

_, branchSpan := tracing.Tracer().Start(ctx, "branch")
a.analyzeBranch(repo, status)
branchSpan.End()

_, wtSpan := tracing.Tracer().Start(ctx, "worktree_status")
a.analyzeWorktree(repoPath, status)
wtSpan.End()
```

Additional child spans include:

- `last_commit`
- optional `fetch` (`--fetch`)
- `remote_status`
- optional `recent_commits` and `lines_changed` (`--detail`)
- `daily_activity`

## Span Hierarchy Illustration

```text
find_repos
process
  analyze(repo=A)
    plain_open
    branch
    worktree_status
    last_commit
    fetch?            (if --fetch)
    remote_status
    recent_commits?   (if --detail)
    lines_changed?    (if --detail)
    daily_activity
  analyze(repo=B)
    ...
  analyze(repo=N)
    ...
```

Because repo analysis runs in a worker pool, sibling `analyze` spans overlap in time.

## Rendering Path

`internal/cli/render.go` parses collected spans and renders:

1. Directory scan duration (`find_repos`)
2. Total analysis duration (`process`)
3. Per-repo min/avg/max (`analyze`)
4. Waterfall timeline of repo analyze spans
5. Span tree for the slowest repo

Key parser logic:

```go
switch s.Name() {
case "find_repos":
    p.findReposDur = dur
case "process":
    p.processDur = dur
    p.processStart = s.StartTime()
case "analyze":
    // repo attr extraction, store timing record
default:
    // attach as child span by parent SpanID
}
```

Waterfall math (normalized to fixed width) is driven by:

```go
offset := a.start.Sub(p.processStart)
startCol := int(float64(offset) / float64(totalDur) * waterfallWidth)
barLen := int(float64(a.dur) / float64(totalDur) * waterfallWidth)
```

## Terminal Illustration

Example conceptual output:

```text
⏱  Performance Breakdown
  Directory scan:       140ms
  Analysis (total):     1.28s
  Per-repo:             min=42ms avg=115ms max=390ms

  ▸  Waterfall (1.28s)
  repo-a     ██████                              150ms
  repo-b         ███████████                     310ms
  repo-c                    ███                  80ms

  ▸  Slowest: repo-b (390ms)
    ├─ plain_open       ░░███░░░░░░░░░░░░░░░░░░░░░░░░ 24ms
    ├─ worktree_status  ░░░░██████████░░░░░░░░░░░░░░ 120ms
    └─ remote_status    ░░░░░░░░░████████░░░░░░░░░░░ 98ms
```

## Flags That Affect OTEL

- `--time`: enables tracing and timing output.
- `--fetch`: adds `fetch` span work inside each repo analysis.
- `--detail`: adds `recent_commits` and `lines_changed` spans.

## What Is Not Implemented

- No OTLP exporter.
- No metrics or logs pipeline.
- No trace context propagation across process boundaries.
- No resource/service metadata customization (service.name, env, etc.).

## References

- `internal/tracing/tracing.go`
- `cmd/pulse/main.go`
- `internal/core/scanner.go`
- `internal/core/analyzer.go`
- `internal/cli/render.go`
- `internal/cli/flags.go`
