package analyzer

import (
	"fmt"
	"sync"
	"time"
)

// jobRegistry is an in-memory, thread-safe store of ScanResult snapshots,
// keyed by job ID. The Moku native backend uses it to satisfy the
// industry-style async contract: SubmitScan records a pending job here,
// the scanning goroutine updates the entry as the pipeline progresses,
// and GetScan reads from here.
//
// Burp and ZAP adapters do NOT use this registry — they delegate to the
// remote scanner and merely cache the most recent GetScan response.
//
// Step A status: skeleton. Step B wires Moku's SubmitScan and GetScan into
// this registry and starts a background cleanup goroutine gated by
// MokuConfig.JobRetention.
type jobRegistry struct {
	mu   sync.RWMutex
	jobs map[string]*jobEntry
}

// jobEntry is one slot in the registry. storedAt records the terminal state
// time so the cleanup goroutine can evict old entries according to the
// configured retention window.
type jobEntry struct {
	result   *ScanResult
	storedAt time.Time
}

func newJobRegistry() *jobRegistry {
	return &jobRegistry{jobs: make(map[string]*jobEntry)}
}

// put inserts or overwrites the entry for jobID. Safe for concurrent use.
func (r *jobRegistry) put(jobID string, result *ScanResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[jobID] = &jobEntry{result: result, storedAt: time.Now()}
}

// get returns the stored ScanResult for jobID, or an error when the id is
// unknown to the registry.
func (r *jobRegistry) get(jobID string) (*ScanResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job %q not found", jobID)
	}
	return entry.result, nil
}

// sweepOlderThan removes terminal entries whose storedAt is older than
// retention. Non-terminal jobs are never evicted regardless of age so an
// in-progress scan can always be queried.
func (r *jobRegistry) sweepOlderThan(retention time.Duration) int {
	if retention <= 0 {
		return 0
	}
	cutoff := time.Now().Add(-retention)
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for id, entry := range r.jobs {
		if entry.result == nil {
			continue
		}
		if !entry.result.Status.IsTerminal() {
			continue
		}
		if entry.storedAt.Before(cutoff) {
			delete(r.jobs, id)
			removed++
		}
	}
	return removed
}
