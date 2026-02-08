# Pulse

A CLI tool that recursively scans directories for Git repos and displays a status summary.

## Motivation

- Out of sight, out of mind
- Came back to my laptop after a day or two away, needed a way to quickly jog my memory on what I've been working on

## Features

- **Status overview** — branch, clean/dirty, last active time for every repo
- **Ahead/behind** — unpushed (↑) and unpulled (↓) commit counts vs origin
- **Ghost detection** — flags repos inactive for 6+ months with 👻
- **Detail mode** — last 5 commits and lines changed (last 7 days) per repo
- **Non-git detection** — notes directories sitting alongside repos that aren't git-tracked
- **JSON output** — pipe to `jq` or feed to another tool
- **Performance tracing** — OpenTelemetry waterfall timeline with `--time`

## Install

```bash
go install github.com/guidefari/pulse/cmd/pulse@latest
```

Or build locally:

```bash
just build
```

## Usage

```bash
pulse --path ~/source --depth 3             # basic table
pulse --path ~/source --depth 3 --detail    # recent commits + lines changed
pulse --path ~/source --format json         # JSON output
pulse --path ~/source --time                # performance breakdown
```

With [just](https://github.com/casey/just):

```bash
just scan              # ~/source, depth 3
just detail            # with recent commits
just json              # JSON output
just time              # performance waterfall
just full              # detail + time
just scan ~/projects 2 # override path and depth
```

## Flags

| Flag       | Default | Description                         |
|------------|---------|-------------------------------------|
| `--path`   | `.`     | Root directory to scan              |
| `--depth`  | `3`     | Maximum directory depth             |
| `--detail` | `false` | Show recent commits + lines changed |
| `--format` | `table` | Output format: `table` or `json`    |
| `--time`   | `false` | Show performance timing breakdown   |

## How it works

Pulse walks your directory tree with [godirwalk](https://github.com/karrick/godirwalk), finds `.git` directories, then analyzes each repo in parallel using a worker pool. It uses [go-git](https://github.com/go-git/go-git) for branch, commit, and remote status — but shells out to `git status --porcelain` for worktree status (10-20x faster).

Results are sorted oldest-first so the repos you've worked on most recently appear at the bottom, closest to your terminal prompt.

> **Note:** Ahead/behind counts are based on local remote-tracking refs. Run `git fetch` to get current remote state.
