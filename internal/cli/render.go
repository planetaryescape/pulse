package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/guidefari/pulse/internal/core"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
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

func RenderTimings(result *core.ScanResult) {
	t := result.Timings
	if t == nil {
		return
	}

	fmt.Printf("\n%s  Performance Breakdown\n", cyan("⏱"))
	fmt.Printf("  %-20s %s\n", "Directory scan:", t.FindRepos.Round(time.Millisecond))
	fmt.Printf("  %-20s %s\n", "Analysis (total):", t.Analysis.Round(time.Millisecond))

	if len(t.RepoTimings) > 0 {
		var min, max, sum time.Duration
		min = t.RepoTimings[0].Total
		for _, rt := range t.RepoTimings {
			sum += rt.Total
			if rt.Total < min {
				min = rt.Total
			}
			if rt.Total > max {
				max = rt.Total
			}
		}
		avg := sum / time.Duration(len(t.RepoTimings))

		fmt.Printf("  %-20s min=%s avg=%s max=%s\n", "Per-repo:",
			min.Round(time.Millisecond),
			avg.Round(time.Millisecond),
			max.Round(time.Millisecond))

		var slowest core.RepoTimings
		for _, rt := range t.RepoTimings {
			if rt.Total > slowest.Total {
				slowest = rt
			}
		}

		fmt.Printf("\n  %s  Slowest repo: %s (%s)\n", dim("▸"), slowest.RepoName, slowest.Total.Round(time.Millisecond))
		fmt.Printf("    %-18s %s\n", "PlainOpen:", slowest.PlainOpen.Round(time.Millisecond))
		fmt.Printf("    %-18s %s\n", "Branch:", slowest.Branch.Round(time.Millisecond))
		fmt.Printf("    %-18s %s\n", "WorktreeStatus:", slowest.WorktreeStatus.Round(time.Millisecond))
		fmt.Printf("    %-18s %s\n", "LastCommit:", slowest.LastCommit.Round(time.Millisecond))
		fmt.Printf("    %-18s %s\n", "RemoteStatus:", slowest.RemoteStatus.Round(time.Millisecond))
		if slowest.RecentCommits > 0 {
			fmt.Printf("    %-18s %s\n", "RecentCommits:", slowest.RecentCommits.Round(time.Millisecond))
		}
		if slowest.LinesChanged > 0 {
			fmt.Printf("    %-18s %s\n", "LinesChanged:", slowest.LinesChanged.Round(time.Millisecond))
		}
	}

	fmt.Printf("\n  %-20s %s\n", "Total:", result.ScanDuration.Round(time.Millisecond))
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
