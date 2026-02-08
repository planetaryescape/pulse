package core

import (
	"path/filepath"
	"strings"
	"time"

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
		config.GhostThreshold = 6 * 30 * 24 * time.Hour
	}
	return &Scanner{config: config}
}

func (s *Scanner) Scan() (*ScanResult, error) {
	start := time.Now()

	repoPaths, err := s.findRepos()
	if err != nil {
		return nil, err
	}

	analyzer := NewAnalyzer(s.config.DetailMode, s.config.GhostThreshold)
	pool := NewPool(s.config.WorkerCount)

	statuses, scanErrors := pool.Process(repoPaths, analyzer)

	result := &ScanResult{
		Repos:        statuses,
		TotalRepos:   len(statuses),
		ScanDuration: time.Since(start),
		Errors:       scanErrors,
	}

	if s.config.DetailMode {
		result.DailyCommits = s.tallyDailyCommits(statuses)
	}

	return result, nil
}

func (s *Scanner) findRepos() ([]string, error) {
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

	return repos, err
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
