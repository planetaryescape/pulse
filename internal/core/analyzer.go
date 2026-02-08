package core

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/guidefari/pulse/internal/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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

func (a *Analyzer) Analyze(ctx context.Context, repoPath string) (*RepoStatus, error) {
	ctx, span := tracing.Tracer().Start(ctx, "analyze",
		trace.WithAttributes(attribute.String("repo", filepath.Base(repoPath))),
	)
	defer span.End()

	_, plainSpan := tracing.Tracer().Start(ctx, "plain_open")
	repo, err := git.PlainOpen(repoPath)
	plainSpan.End()
	if err != nil {
		return nil, err
	}

	status := &RepoStatus{
		Name: filepath.Base(repoPath),
		Path: repoPath,
	}

	_, branchSpan := tracing.Tracer().Start(ctx, "branch")
	a.analyzeBranch(repo, status)
	branchSpan.End()

	_, wtSpan := tracing.Tracer().Start(ctx, "worktree_status")
	a.analyzeWorktree(repoPath, status)
	wtSpan.End()

	_, lcSpan := tracing.Tracer().Start(ctx, "last_commit")
	a.analyzeLastCommit(repo, status)
	lcSpan.End()

	_, rsSpan := tracing.Tracer().Start(ctx, "remote_status")
	a.analyzeRemoteStatus(repo, status)
	rsSpan.End()

	if a.detailMode {
		_, rcSpan := tracing.Tracer().Start(ctx, "recent_commits")
		a.analyzeRecentCommits(repo, status)
		rcSpan.End()

		_, lcSpan := tracing.Tracer().Start(ctx, "lines_changed")
		a.analyzeLinesChanged(repo, status)
		lcSpan.End()
	}

	status.IsGhost = time.Since(status.LastCommitTime) > a.ghostThreshold

	return status, nil
}

func (a *Analyzer) analyzeBranch(repo *git.Repository, status *RepoStatus) {
	head, err := repo.Head()
	if err != nil {
		status.Branch = "unknown"
		return
	}
	status.Branch = head.Name().Short()
}

func (a *Analyzer) analyzeWorktree(repoPath string, status *RepoStatus) {
	out, err := exec.Command("git", "-C", repoPath, "status", "--porcelain").Output()
	if err != nil {
		return
	}

	lines := bytes.Count(bytes.TrimRight(out, "\n"), []byte("\n"))
	if len(out) > 0 {
		lines++
	}

	status.IsClean = len(out) == 0
	status.ChangedFiles = lines
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
