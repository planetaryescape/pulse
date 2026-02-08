package core

import "sync"

type Pool struct {
	workerCount int
}

func NewPool(workerCount int) *Pool {
	return &Pool{workerCount: workerCount}
}

func (p *Pool) Process(repoPaths []string, analyzer *Analyzer) ([]RepoStatus, []RepoTimings, []ScanError) {
	type result struct {
		status  *RepoStatus
		timings *RepoTimings
		err     *ScanError
	}

	jobs := make(chan string, len(repoPaths))
	results := make(chan result, len(repoPaths))

	var wg sync.WaitGroup
	for i := 0; i < p.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				status, timings, err := analyzer.Analyze(path)
				if err != nil {
					results <- result{err: &ScanError{Path: path, Message: err.Error()}}
				} else {
					results <- result{status: status, timings: timings}
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
	var repoTimings []RepoTimings
	var errors []ScanError
	for r := range results {
		if r.status != nil {
			statuses = append(statuses, *r.status)
		}
		if r.timings != nil {
			repoTimings = append(repoTimings, *r.timings)
		}
		if r.err != nil {
			errors = append(errors, *r.err)
		}
	}

	SortByLastActive(statuses)
	return statuses, repoTimings, errors
}
