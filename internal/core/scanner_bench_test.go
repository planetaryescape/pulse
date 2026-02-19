package core

import (
	"context"
	"os"
	"testing"
)

func benchPath(b *testing.B) string {
	b.Helper()
	path := os.Getenv("PULSE_BENCH_PATH")
	if path == "" {
		path, _ = os.Getwd()
		path = path + "/../.."
	}
	return path
}

func BenchmarkFindRepos(b *testing.B) {
	path := benchPath(b)
	s := NewScanner(ScanConfig{
		RootPath:       path,
		MaxDepth:       3,
		GhostThreshold: DefaultGhostThreshold,
		WorkerCount:    4,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.findRepos()
	}
}

func BenchmarkFullScan(b *testing.B) {
	path := benchPath(b)
	s := NewScanner(ScanConfig{
		RootPath:       path,
		MaxDepth:       2,
		GhostThreshold: DefaultGhostThreshold,
		WorkerCount:    4,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Scan(context.Background())
	}
}

func BenchmarkFullScanDetail(b *testing.B) {
	path := benchPath(b)
	s := NewScanner(ScanConfig{
		RootPath:       path,
		MaxDepth:       2,
		DetailMode:     true,
		GhostThreshold: DefaultGhostThreshold,
		WorkerCount:    4,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Scan(context.Background())
	}
}
