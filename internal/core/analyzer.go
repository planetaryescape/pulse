package core

import (
	"path/filepath"
	"sort"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type Analyzer struct {
	detailMode     bool
	ghostThreshold time.Duration
}

func NewAnalyzer(detailMode bool, ghostThreshold time.Duration) *Analyzer {
	return &Analyzer{
		detailMode:     detailMode,
		ghostThreshold: ghostThreshold,
	}
}

func (a *Analyzer) Analyze(repoPath string) (*RepoStatus, *RepoTimings, error) {
	var timings RepoTimings
	timings.RepoName = filepath.Base(repoPath)
	totalStart := time.Now()

	start := time.Now()
	repo, err := git.PlainOpen(repoPath)
	timings.PlainOpen = time.Since(start)
	if err != nil {
		return nil, nil, err
	}

	status := &RepoStatus{
		Name: filepath.Base(repoPath),
		Path: repoPath,
	}

	start = time.Now()
	a.analyzeBranch(repo, status)
	timings.Branch = time.Since(start)

	start = time.Now()
	a.analyzeWorktree(repo, status)
	timings.WorktreeStatus = time.Since(start)

	start = time.Now()
	a.analyzeLastCommit(repo, status)
	timings.LastCommit = time.Since(start)

	start = time.Now()
	a.analyzeRemoteStatus(repo, status)
	timings.RemoteStatus = time.Since(start)

	if a.detailMode {
		start = time.Now()
		a.analyzeRecentCommits(repo, status)
		timings.RecentCommits = time.Since(start)

		start = time.Now()
		a.analyzeLinesChanged(repo, status)
		timings.LinesChanged = time.Since(start)
	}

	status.IsGhost = time.Since(status.LastCommitTime) > a.ghostThreshold
	timings.Total = time.Since(totalStart)

	return status, &timings, nil
}

func (a *Analyzer) analyzeBranch(repo *git.Repository, status *RepoStatus) {
	head, err := repo.Head()
	if err != nil {
		status.Branch = "unknown"
		return
	}
	status.Branch = head.Name().Short()
}

func (a *Analyzer) analyzeWorktree(repo *git.Repository, status *RepoStatus) {
	wt, err := repo.Worktree()
	if err != nil {
		return
	}

	ws, err := wt.Status()
	if err != nil {
		return
	}

	status.IsClean = ws.IsClean()
	status.ChangedFiles = len(ws)
}

func (a *Analyzer) analyzeLastCommit(repo *git.Repository, status *RepoStatus) {
	iter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return
	}
	defer iter.Close()

	commit, err := iter.Next()
	if err != nil {
		return
	}
	status.LastCommitTime = commit.Author.When
}

func (a *Analyzer) analyzeRemoteStatus(repo *git.Repository, status *RepoStatus) {
	head, err := repo.Head()
	if err != nil || !head.Name().IsBranch() {
		return
	}

	branchName := head.Name().Short()
	remoteBranch := plumbing.NewRemoteReferenceName("origin", branchName)

	remoteRef, err := repo.Reference(remoteBranch, true)
	if err != nil {
		return
	}

	localHash := head.Hash()
	remoteHash := remoteRef.Hash()

	if localHash == remoteHash {
		return
	}

	localCommit, err := repo.CommitObject(localHash)
	if err != nil {
		return
	}
	remoteCommit, err := repo.CommitObject(remoteHash)
	if err != nil {
		return
	}

	mergeBase, err := localCommit.MergeBase(remoteCommit)
	if err != nil || len(mergeBase) == 0 {
		return
	}

	base := mergeBase[0].Hash

	status.UnpushedCommits = countCommitsBetween(repo, base, localHash)
	status.UnpulledCommits = countCommitsBetween(repo, base, remoteHash)
}

func countCommitsBetween(repo *git.Repository, from, to plumbing.Hash) int {
	if from == to {
		return 0
	}

	count := 0
	iter, err := repo.Log(&git.LogOptions{From: to})
	if err != nil {
		return 0
	}
	defer iter.Close()

	iter.ForEach(func(c *object.Commit) error {
		if c.Hash == from {
			return errStop
		}
		count++
		return nil
	})

	return count
}

var errStop = &stopIter{}

type stopIter struct{}

func (e *stopIter) Error() string { return "stop" }

func (a *Analyzer) analyzeRecentCommits(repo *git.Repository, status *RepoStatus) {
	iter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return
	}
	defer iter.Close()

	for i := 0; i < 5; i++ {
		c, err := iter.Next()
		if err != nil {
			break
		}
		status.RecentCommits = append(status.RecentCommits, Commit{
			Hash:      c.Hash.String()[:7],
			Author:    c.Author.Name,
			Message:   firstLine(c.Message),
			Timestamp: c.Author.When,
		})
	}
}

func (a *Analyzer) analyzeLinesChanged(repo *git.Repository, status *RepoStatus) {
	since := time.Now().Add(-7 * 24 * time.Hour)
	iter, err := repo.Log(&git.LogOptions{Since: &since})
	if err != nil {
		return
	}
	defer iter.Close()

	var added, removed int
	iter.ForEach(func(c *object.Commit) error {
		stats, err := c.Stats()
		if err != nil {
			return nil
		}
		for _, s := range stats {
			added += s.Addition
			removed += s.Deletion
		}
		return nil
	})

	if added > 0 || removed > 0 {
		status.LinesChanged = &LinesChanged{
			Added:   added,
			Removed: removed,
			Period:  7 * 24 * time.Hour,
		}
	}
}

func firstLine(s string) string {
	for i, c := range s {
		if c == '\n' {
			return s[:i]
		}
	}
	return s
}

func SortByLastActive(repos []RepoStatus) {
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].LastCommitTime.After(repos[j].LastCommitTime)
	})
}
