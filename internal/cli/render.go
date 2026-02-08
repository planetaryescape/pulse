package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/guidefari/pulse/internal/core"
	"github.com/guidefari/pulse/internal/tracing"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"go.opentelemetry.io/otel/attribute"
)

var (
	green = color.New(color.FgGreen).SprintFunc()
	red   = color.New(color.FgRed).SprintFunc()
	dim   = color.New(color.Faint).SprintFunc()
	cyan  = color.New(color.FgCyan).SprintFunc()
)

func Render(result *core.ScanResult, format string) {
	if format == "json" {
		RenderJSON(result)
		return
	}
	RenderTable(result)
}

func RenderTable(result *core.ScanResult) {
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeader([]string{"Repo", "Status", "Branch", "Last Active", "Ahead/Behind"}),
		tablewriter.WithHeaderAlignment(tw.AlignLeft),
		tablewriter.WithAlignment(tw.Alignment{tw.AlignLeft}),
		tablewriter.WithBorders(tw.Border{Left: tw.Off, Right: tw.Off, Top: tw.Off, Bottom: tw.Off}),
	)

	for _, repo := range result.Repos {
		status := green("✔ clean")
		if !repo.IsClean {
			status = red(fmt.Sprintf("✘ %d changed", repo.ChangedFiles))
		}

		branch := repo.Branch
		if repo.IsGhost {
			branch = dim(branch + " 👻")
		}

		aheadBehind := ""
		if repo.UnpushedCommits > 0 {
			aheadBehind += fmt.Sprintf("↑%d", repo.UnpushedCommits)
		}
		if repo.UnpulledCommits > 0 {
			if aheadBehind != "" {
				aheadBehind += " "
			}
			aheadBehind += fmt.Sprintf("↓%d", repo.UnpulledCommits)
		}

		table.Append([]string{
			repo.Name,
			status,
			branch,
			timeAgo(repo.LastCommitTime),
			aheadBehind,
		})
	}

	fmt.Printf("\n%s  Found %d repos (scanned in %s)\n\n",
		cyan("pulse"),
		result.TotalRepos,
		result.ScanDuration.Round(time.Millisecond))
	table.Render()

	if len(result.Errors) > 0 {
		fmt.Printf("\n%s  %d repos had errors\n", red("!"), len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("  %s: %s\n", dim(e.Path), e.Message)
		}
	}

	if result.DailyCommits != nil {
		today := time.Now().Format("2006-01-02")
		if count, ok := result.DailyCommits[today]; ok {
			fmt.Printf("\n%s  %d commits today across all repos\n", cyan("📊"), count)
		}
	}

	fmt.Println()
}

func RenderDetail(result *core.ScanResult) {
	for _, repo := range result.Repos {
		if len(repo.RecentCommits) == 0 {
			continue
		}

		fmt.Printf("\n%s %s\n", cyan("─────"), repo.Name)
		for _, c := range repo.RecentCommits {
			fmt.Printf("  %s %s %s\n",
				dim(c.Hash),
				c.Message,
				dim(timeAgo(c.Timestamp)))
		}

		if repo.LinesChanged != nil {
			fmt.Printf("  %s +%d %s -%d (last 7 days)\n",
				dim("lines:"),
				repo.LinesChanged.Added,
				dim("/"),
				repo.LinesChanged.Removed)
		}
	}
}

func RenderJSON(result *core.ScanResult) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(result)
}

func RenderTimings(exp *tracing.CollectingExporter) {
	if exp == nil {
		return
	}

	spans := exp.Spans()

	type analyzeSpan struct {
		repo     string
		duration time.Duration
	}

	var findReposDur, processDur time.Duration
	var analyzeSpans []analyzeSpan
	childSpans := make(map[string]map[string]time.Duration)

	for _, s := range spans {
		dur := s.EndTime().Sub(s.StartTime())
		name := s.Name()

		switch name {
		case "find_repos":
			findReposDur = dur
		case "process":
			processDur = dur
		case "analyze":
			repo := ""
			for _, attr := range s.Attributes() {
				if attr.Key == attribute.Key("repo") {
					repo = attr.Value.AsString()
				}
			}
			analyzeSpans = append(analyzeSpans, analyzeSpan{repo: repo, duration: dur})
		default:
			parent := s.Parent()
			if !parent.IsValid() {
				continue
			}
			pid := parent.SpanID().String()
			if childSpans[pid] == nil {
				childSpans[pid] = make(map[string]time.Duration)
			}
			childSpans[pid][name] = dur
		}
	}

	fmt.Printf("\n%s  Performance Breakdown\n", cyan("⏱"))
	fmt.Printf("  %-20s %s\n", "Directory scan:", findReposDur.Round(time.Millisecond))
	fmt.Printf("  %-20s %s\n", "Analysis (total):", processDur.Round(time.Millisecond))

	if len(analyzeSpans) > 0 {
		minDur := analyzeSpans[0].duration
		var maxDur, sum time.Duration
		var slowestIdx int
		for i, as := range analyzeSpans {
			sum += as.duration
			if as.duration < minDur {
				minDur = as.duration
			}
			if as.duration > maxDur {
				maxDur = as.duration
				slowestIdx = i
			}
		}
		avg := sum / time.Duration(len(analyzeSpans))

		fmt.Printf("  %-20s min=%s avg=%s max=%s\n", "Per-repo:",
			minDur.Round(time.Millisecond),
			avg.Round(time.Millisecond),
			maxDur.Round(time.Millisecond))

		slowest := analyzeSpans[slowestIdx]

		var slowestSpanID string
		for _, s := range spans {
			if s.Name() != "analyze" {
				continue
			}
			dur := s.EndTime().Sub(s.StartTime())
			if dur == slowest.duration {
				slowestSpanID = s.SpanContext().SpanID().String()
				break
			}
		}

		fmt.Printf("\n  %s  Slowest repo: %s (%s)\n", dim("▸"), slowest.repo, slowest.duration.Round(time.Millisecond))

		breakdown := childSpans[slowestSpanID]
		labels := []struct{ name, label string }{
			{"plain_open", "PlainOpen:"},
			{"branch", "Branch:"},
			{"worktree_status", "WorktreeStatus:"},
			{"last_commit", "LastCommit:"},
			{"remote_status", "RemoteStatus:"},
			{"recent_commits", "RecentCommits:"},
			{"lines_changed", "LinesChanged:"},
		}
		for _, l := range labels {
			if d, ok := breakdown[l.name]; ok {
				fmt.Printf("    %-18s %s\n", l.label, d.Round(time.Millisecond))
			}
		}
	}

	fmt.Println()
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/(24*365)))
	}
}
