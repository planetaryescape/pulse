package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/guidefari/pulse/internal/core"
	"github.com/guidefari/pulse/internal/tracing"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
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
		tablewriter.WithHeader([]string{"Repo", "Status", "Last Active", "Branch", "Ahead/Behind"}),
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
			timeAgo(repo.LastCommitTime),
			branch,
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

	if len(result.NonGitPaths) > 0 {
		fmt.Printf("\n%s  %d non-git directories: %s\n",
			dim("📁"),
			len(result.NonGitPaths),
			dim(strings.Join(result.NonGitPaths, ", ")))
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
	for i := len(result.Repos) - 1; i >= 0; i-- {
		repo := result.Repos[i]
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

type parsedSpans struct {
	findReposDur time.Duration
	processDur   time.Duration
	processStart time.Time
	analyzes     []analyzeInfo
	children     map[trace.SpanID][]childSpan
}

type analyzeInfo struct {
	repo   string
	start  time.Time
	end    time.Time
	dur    time.Duration
	spanID trace.SpanID
}

type childSpan struct {
	name   string
	start  time.Time
	end    time.Time
	dur    time.Duration
	spanID trace.SpanID
}

func parseSpans(spans []sdktrace.ReadOnlySpan) parsedSpans {
	p := parsedSpans{
		children: make(map[trace.SpanID][]childSpan),
	}

	for _, s := range spans {
		dur := s.EndTime().Sub(s.StartTime())
		switch s.Name() {
		case "find_repos":
			p.findReposDur = dur
		case "process":
			p.processDur = dur
			p.processStart = s.StartTime()
		case "analyze":
			repo := ""
			for _, attr := range s.Attributes() {
				if attr.Key == attribute.Key("repo") {
					repo = attr.Value.AsString()
				}
			}
			p.analyzes = append(p.analyzes, analyzeInfo{
				repo:   repo,
				start:  s.StartTime(),
				end:    s.EndTime(),
				dur:    dur,
				spanID: s.SpanContext().SpanID(),
			})
		default:
			parent := s.Parent()
			if !parent.IsValid() {
				continue
			}
			p.children[parent.SpanID()] = append(p.children[parent.SpanID()], childSpan{
				name:   s.Name(),
				start:  s.StartTime(),
				end:    s.EndTime(),
				dur:    dur,
				spanID: s.SpanContext().SpanID(),
			})
		}
	}

	sort.Slice(p.analyzes, func(i, j int) bool {
		return p.analyzes[i].start.Before(p.analyzes[j].start)
	})

	return p
}

func RenderTimings(exp *tracing.CollectingExporter) {
	if exp == nil {
		return
	}

	p := parseSpans(exp.Spans())

	fmt.Printf("\n%s  Performance Breakdown\n", cyan("⏱"))
	fmt.Printf("  %-20s %s\n", "Directory scan:", p.findReposDur.Round(time.Millisecond))
	fmt.Printf("  %-20s %s\n", "Analysis (total):", p.processDur.Round(time.Millisecond))

	if len(p.analyzes) == 0 {
		fmt.Println()
		return
	}

	minDur := p.analyzes[0].dur
	var maxDur, sum time.Duration
	var slowestIdx int
	for i, a := range p.analyzes {
		sum += a.dur
		if a.dur < minDur {
			minDur = a.dur
		}
		if a.dur > maxDur {
			maxDur = a.dur
			slowestIdx = i
		}
	}
	avg := sum / time.Duration(len(p.analyzes))

	fmt.Printf("  %-20s min=%s avg=%s max=%s\n", "Per-repo:",
		minDur.Round(time.Millisecond),
		avg.Round(time.Millisecond),
		maxDur.Round(time.Millisecond))

	renderWaterfall(p)
	renderSpanTree(p, slowestIdx)

	fmt.Println()
}

const waterfallWidth = 50

func renderWaterfall(p parsedSpans) {
	if len(p.analyzes) == 0 {
		return
	}

	totalDur := p.processDur
	if totalDur == 0 {
		return
	}

	maxNameLen := 0
	for _, a := range p.analyzes {
		if len(a.repo) > maxNameLen {
			maxNameLen = len(a.repo)
		}
	}
	if maxNameLen > 20 {
		maxNameLen = 20
	}

	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Printf("\n  %s  Waterfall (%s)\n", cyan("▸"), totalDur.Round(time.Millisecond))

	for _, a := range p.analyzes {
		offset := a.start.Sub(p.processStart)
		startCol := int(float64(offset) / float64(totalDur) * waterfallWidth)
		barLen := int(float64(a.dur) / float64(totalDur) * waterfallWidth)
		if barLen < 1 {
			barLen = 1
		}
		if startCol+barLen > waterfallWidth {
			barLen = waterfallWidth - startCol
		}
		if startCol < 0 {
			startCol = 0
		}

		name := a.repo
		if len(name) > maxNameLen {
			name = name[:maxNameLen-1] + "…"
		}

		bar := strings.Repeat(" ", startCol) + yellow(strings.Repeat("█", barLen)) + strings.Repeat(" ", waterfallWidth-startCol-barLen)
		fmt.Printf("  %-*s %s %s\n", maxNameLen, name, bar, dim(a.dur.Round(time.Millisecond).String()))
	}
}

func renderSpanTree(p parsedSpans, slowestIdx int) {
	slowest := p.analyzes[slowestIdx]

	fmt.Printf("\n  %s  Slowest: %s (%s)\n", cyan("▸"), slowest.repo, slowest.dur.Round(time.Millisecond))

	if slowest.dur == 0 {
		return
	}

	renderChildren(p, slowest.spanID, slowest.start, slowest.dur, "    ")
}

func renderChildren(p parsedSpans, parentID trace.SpanID, rootStart time.Time, totalDur time.Duration, indent string) {
	children := p.children[parentID]
	if len(children) == 0 {
		return
	}

	sort.Slice(children, func(i, j int) bool {
		return children[i].start.Before(children[j].start)
	})

	barWidth := 30
	maxNameLen := 0
	for _, c := range children {
		if len(c.name) > maxNameLen {
			maxNameLen = len(c.name)
		}
	}

	for i, c := range children {
		isLast := i == len(children)-1
		connector := "├─"
		if isLast {
			connector = "└─"
		}

		offset := c.start.Sub(rootStart)
		startCol := int(float64(offset) / float64(totalDur) * float64(barWidth))
		barLen := int(float64(c.dur) / float64(totalDur) * float64(barWidth))
		if barLen < 1 {
			barLen = 1
		}
		if startCol < 0 {
			startCol = 0
		}
		if startCol+barLen > barWidth {
			barLen = barWidth - startCol
		}

		bar := strings.Repeat("░", startCol) + strings.Repeat("█", barLen) + strings.Repeat("░", barWidth-startCol-barLen)
		fmt.Printf("%s%s %-*s %s %s\n", indent, dim(connector), maxNameLen, c.name, dim(bar), dim(c.dur.Round(time.Millisecond).String()))

		grandchildren := p.children[c.spanID]
		if len(grandchildren) > 0 {
			childIndent := indent + "│  "
			if isLast {
				childIndent = indent + "   "
			}
			renderChildren(p, c.spanID, rootStart, totalDur, childIndent)
		}
	}
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
