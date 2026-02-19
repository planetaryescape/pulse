package core

import (
	"context"
	"os"
	"testing"

	"github.com/go-git/go-git/v5"
)

func benchRepo(b *testing.B) string {
	b.Helper()
	path := os.Getenv("PULSE_BENCH_REPO")
	if path == "" {
		path, _ = os.Getwd()
		path = path + "/../.."
	}
	if _, err := git.PlainOpen(path); err != nil {
		b.Skipf("no git repo at %s: %v", path, err)
	}
	return path
}

func BenchmarkAnalyze(b *testing.B) {
	repoPath := benchRepo(b)
	ghostThreshold := DefaultGhostThreshold

	b.Run("Full", func(b *testing.B) {
		a := NewAnalyzer(false, false, ghostThreshold)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			a.Analyze(context.Background(), repoPath)
		}
	})

	b.Run("FullDetail", func(b *testing.B) {
		a := NewAnalyzer(true, false, ghostThreshold)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			a.Analyze(context.Background(), repoPath)
		}
	})

	b.Run("PlainOpen", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			git.PlainOpen(repoPath)
		}
	})

	b.Run("Branch", func(b *testing.B) {
		repo, _ := git.PlainOpen(repoPath)
		a := NewAnalyzer(false, false, ghostThreshold)
		status := &RepoStatus{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			a.analyzeBranch(repo, status)
		}
	})

	b.Run("WorktreeStatus", func(b *testing.B) {
		a := NewAnalyzer(false, false, ghostThreshold)
		status := &RepoStatus{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			a.analyzeWorktree(repoPath, status)
		}
	})

	b.Run("LastCommit", func(b *testing.B) {
		repo, _ := git.PlainOpen(repoPath)
		a := NewAnalyzer(false, false, ghostThreshold)
		status := &RepoStatus{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			a.analyzeLastCommit(repo, status)
		}
	})

	b.Run("RemoteStatus", func(b *testing.B) {
		repo, _ := git.PlainOpen(repoPath)
		a := NewAnalyzer(false, false, ghostThreshold)
		status := &RepoStatus{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			a.analyzeRemoteStatus(repo, status)
		}
	})

	b.Run("RecentCommits", func(b *testing.B) {
		repo, _ := git.PlainOpen(repoPath)
		a := NewAnalyzer(true, false, ghostThreshold)
		status := &RepoStatus{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			status.RecentCommits = nil
			a.analyzeRecentCommits(repo, status)
		}
	})

	b.Run("LinesChanged", func(b *testing.B) {
		repo, _ := git.PlainOpen(repoPath)
		a := NewAnalyzer(true, false, ghostThreshold)
		status := &RepoStatus{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			status.LinesChanged = nil
			a.analyzeLinesChanged(repo, status)
		}
	})
}
