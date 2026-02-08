package core

import "time"

type ScanConfig struct {
	RootPath       string
	MaxDepth       int
	DetailMode     bool
	GhostThreshold time.Duration
	WorkerCount    int
}

type RepoStatus struct {
	Name            string        `json:"name"`
	Path            string        `json:"path"`
	Branch          string        `json:"branch"`
	IsClean         bool          `json:"is_clean"`
	ChangedFiles    int           `json:"changed_files"`
	LastCommitTime  time.Time     `json:"last_commit_time"`
	UnpushedCommits int           `json:"unpushed_commits"`
	UnpulledCommits int           `json:"unpulled_commits"`
	IsGhost         bool          `json:"is_ghost"`
	RecentCommits   []Commit      `json:"recent_commits,omitempty"`
	LinesChanged    *LinesChanged `json:"lines_changed,omitempty"`
}

type Commit struct {
	Hash      string    `json:"hash"`
	Author    string    `json:"author"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type LinesChanged struct {
	Added   int           `json:"added"`
	Removed int           `json:"removed"`
	Period  time.Duration `json:"period"`
}

type ScanResult struct {
	Repos        []RepoStatus   `json:"repos"`
	TotalRepos   int            `json:"total_repos"`
	NonGitPaths  []string       `json:"non_git_paths,omitempty"`
	DailyCommits map[string]int `json:"daily_commits,omitempty"`
	ScanDuration time.Duration  `json:"scan_duration"`
	Errors       []ScanError    `json:"errors,omitempty"`
}

type ScanError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}
