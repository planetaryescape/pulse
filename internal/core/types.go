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

type RepoTimings struct {
	RepoName       string        `json:"repo_name"`
	PlainOpen      time.Duration `json:"plain_open"`
	Branch         time.Duration `json:"branch"`
	WorktreeStatus time.Duration `json:"worktree_status"`
	LastCommit     time.Duration `json:"last_commit"`
	RemoteStatus   time.Duration `json:"remote_status"`
	RecentCommits  time.Duration `json:"recent_commits,omitempty"`
	LinesChanged   time.Duration `json:"lines_changed,omitempty"`
	Total          time.Duration `json:"total"`
}

type Timings struct {
	FindRepos    time.Duration  `json:"find_repos"`
	Analysis     time.Duration  `json:"analysis"`
	RepoTimings  []RepoTimings  `json:"repo_timings,omitempty"`
}

type ScanResult struct {
	Repos        []RepoStatus   `json:"repos"`
	TotalRepos   int            `json:"total_repos"`
	DailyCommits map[string]int `json:"daily_commits,omitempty"`
	ScanDuration time.Duration  `json:"scan_duration"`
	Errors       []ScanError    `json:"errors,omitempty"`
	Timings      *Timings       `json:"timings,omitempty"`
}

type ScanError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}
