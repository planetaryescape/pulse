package core

import (
	"context"
	"sync"
)

type Pool struct {
	workerCount int
}

func NewPool(workerCount int) *Pool {
	return &Pool{workerCount: workerCount}
}

func (p *Pool) Process(ctx context.Context, repoPaths []string, analyzer *Analyzer) ([]RepoStatus, []ScanError) {
	type result struct {
		status *RepoStatus
		err    *ScanError
	}

	jobs := make(chan string, len(repoPaths))
	results := make(chan result, len(repoPaths))

	var wg sync.WaitGroup
	for i := 0; i < p.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				status, err := analyzer.Analyze(ctx, path)
				if err != nil {
					results <- result{err: &ScanError{Path: path, Message: err.Error()}}
				} else {
					results <- result{status: status}
				}
			}
		}()
	}

	for _, path := range repoPaths {
		jobs <- path
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var statuses []RepoStatus
	var errors []ScanError
	for r := range results {
		if r.status != nil {
			statuses = append(statuses, *r.status)
		}
		if r.err != nil {
			errors = append(errors, *r.err)
		}
	}

	SortByLastActive(statuses)
	return statuses, errors
}
