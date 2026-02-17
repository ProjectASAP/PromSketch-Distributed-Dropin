package merger

import (
	"math"
	"sort"
	"sync"
)

// QuantileTracker maintains a ring buffer of recent observations
// and computes quantiles on demand.
type QuantileTracker struct {
	mu   sync.Mutex
	buf  []float64
	pos  int
	full bool
}

// NewQuantileTracker creates a tracker with the given window size.
func NewQuantileTracker(size int) *QuantileTracker {
	return &QuantileTracker{buf: make([]float64, size)}
}

// Observe records a new value.
func (q *QuantileTracker) Observe(v float64) {
	q.mu.Lock()
	q.buf[q.pos] = v
	q.pos++
	if q.pos >= len(q.buf) {
		q.pos = 0
		q.full = true
	}
	q.mu.Unlock()
}

// Quantile returns the given quantile (0–1) from recent observations.
// Returns NaN if no observations exist.
func (q *QuantileTracker) Quantile(phi float64) float64 {
	q.mu.Lock()
	n := q.pos
	if q.full {
		n = len(q.buf)
	}
	if n == 0 {
		q.mu.Unlock()
		return math.NaN()
	}
	sorted := make([]float64, n)
	if q.full {
		copy(sorted, q.buf)
	} else {
		copy(sorted, q.buf[:n])
	}
	q.mu.Unlock()

	sort.Float64s(sorted)
	idx := int(phi * float64(n-1))
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

// Count returns the number of observations in the current window.
func (q *QuantileTracker) Count() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.full {
		return len(q.buf)
	}
	return q.pos
}
