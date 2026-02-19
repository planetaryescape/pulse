package core

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/guidefari/pulse/internal/tracing"
	"github.com/karrick/godirwalk"
)

type Scanner struct {
	config ScanConfig
}

func NewScanner(config ScanConfig) *Scanner {
	if config.WorkerCount <= 0 {
		config.WorkerCount = 4
	}
	if config.MaxDepth <= 0 {
		config.MaxDepth = 3
	}
	if config.GhostThreshold <= 0 {
		config.GhostThreshold = DefaultGhostThreshold
	}
	return &Scanner{config: config}
}

func (s *Scanner) Scan(ctx context.Context) (*ScanResult, error) {
	start := time.Now()

	ctx, findSpan := tracing.Tracer().Start(ctx, "find_repos")
	found, err := s.findRepos()
	findSpan.End()
	if err != nil {
		return nil, err
	}

	analyzer := NewAnalyzer(s.config.DetailMode, s.config.Fetch, s.config.GhostThreshold)
	pool := NewPool(s.config.WorkerCount)

	_, processSpan := tracing.Tracer().Start(ctx, "process")
	statuses, scanErrors := pool.Process(ctx, found.repos, analyzer)
	processSpan.End()

	result := &ScanResult{
		Repos:        statuses,
		TotalRepos:   len(statuses),
		NonGitPaths:  found.nonGitPaths,
		ScanDuration: time.Since(start),
		Errors:       scanErrors,
	}

	if s.config.DetailMode {
		result.DailyCommits = s.tallyDailyCommits(statuses)
	}

	return result, nil
}

type findResult struct {
	repos       []string
	nonGitPaths []string
}

func (s *Scanner) findRepos() (*findResult, error) {
	rootDepth := strings.Count(filepath.Clean(s.config.RootPath), string(filepath.Separator))
	var repos []string
	skipDirs := map[string]bool{
		"node_modules": true,
		".Trash":       true,
		"vendor":       true,
	}

	err := godirwalk.Walk(s.config.RootPath, &godirwalk.Options{
		Unsorted: true,
		Callback: func(path string, de *godirwalk.Dirent) error {
			if !de.IsDir() {
				return nil
			}

			name := de.Name()
			if skipDirs[name] {
				return godirwalk.SkipThis
			}

			currentDepth := strings.Count(filepath.Clean(path), string(filepath.Separator)) - rootDepth
			if currentDepth > s.config.MaxDepth {
				return godirwalk.SkipThis
			}

			if name == ".git" {
				repos = append(repos, filepath.Dir(path))
				return godirwalk.SkipThis
			}

			return nil
		},
		ErrorCallback: func(path string, err error) godirwalk.ErrorAction {
			return godirwalk.SkipNode
		},
	})

	nonGitPaths := findNonGitSiblings(s.config.RootPath, repos, skipDirs)

	return &findResult{repos: repos, nonGitPaths: nonGitPaths}, err
}

func findNonGitSiblings(rootPath string, repos []string, skipDirs map[string]bool) []string {
	repoSet := make(map[string]bool, len(repos))
	parentDirs := make(map[string]bool)
	for _, r := range repos {
		repoSet[r] = true
		parentDirs[filepath.Dir(r)] = true
	}

	var nonGit []string
	for parent := range parentDirs {
		entries, err := os.ReadDir(parent)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if name == ".git" || skipDirs[name] {
				continue
			}
			childPath := filepath.Join(parent, name)
			if repoSet[childPath] {
				continue
			}
			containsRepo := false
			for _, r := range repos {
				if strings.HasPrefix(r, childPath+string(filepath.Separator)) {
					containsRepo = true
					break
				}
			}
			if !containsRepo {
				rel, err := filepath.Rel(rootPath, childPath)
				if err != nil {
					rel = filepath.Base(childPath)
				}
				nonGit = append(nonGit, rel)
			}
		}
	}

	sort.Strings(nonGit)
	return nonGit
}

func (s *Scanner) tallyDailyCommits(statuses []RepoStatus) map[string]int {
	tally := make(map[string]int)
	today := time.Now().Format("2006-01-02")

	for _, status := range statuses {
		for _, commit := range status.RecentCommits {
			date := commit.Timestamp.Format("2006-01-02")
			if date == today {
				tally[date]++
			}
		}
	}

	return tally
}
