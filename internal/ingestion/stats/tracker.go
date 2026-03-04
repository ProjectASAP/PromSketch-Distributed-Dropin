package stats

import (
	"sync"
	"sync/atomic"
	"time"
)

// Snapshot is a point-in-time view of ingestion throughput statistics.
type Snapshot struct {
	TotalSamples      uint64
	RatePerSec        float64
	AvgRatePerSec     float64
	SamplesInInterval uint64
	IntervalSeconds   float64
	Timestamp         time.Time
}

// Tracker keeps rolling ingestion throughput statistics.
type Tracker struct {
	totalSamples atomic.Uint64

	interval time.Duration
	window   int

	mu        sync.RWMutex
	snapshot  Snapshot
	rates     []float64
	lastTime  time.Time
	lastTotal uint64

	stopCh chan struct{}
	doneCh chan struct{}

	started atomic.Bool
}

// NewTracker creates a new tracker.
func NewTracker(interval time.Duration, window int) *Tracker {
	if interval <= 0 {
		interval = time.Second
	}
	if window < 1 {
		window = 1
	}
	return &Tracker{
		interval: interval,
		window:   window,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start begins periodic throughput calculations.
func (t *Tracker) Start() {
	if !t.started.CompareAndSwap(false, true) {
		return
	}

	t.mu.Lock()
	t.lastTime = time.Now()
	t.snapshot.Timestamp = t.lastTime
	t.mu.Unlock()

	ticker := time.NewTicker(t.interval)
	go func() {
		defer close(t.doneCh)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.update()
			case <-t.stopCh:
				return
			}
		}
	}()
}

// Stop terminates the tracker goroutine.
func (t *Tracker) Stop() {
	if !t.started.Load() {
		return
	}
	select {
	case <-t.stopCh:
		// already stopped
	default:
		close(t.stopCh)
	}
	<-t.doneCh
}

// AddSamples increments the total ingested sample counter.
func (t *Tracker) AddSamples(n uint64) {
	if n == 0 {
		return
	}
	t.totalSamples.Add(n)
}

// Snapshot returns the latest stats snapshot.
func (t *Tracker) Snapshot() Snapshot {
	total := t.totalSamples.Load()

	t.mu.RLock()
	s := t.snapshot
	t.mu.RUnlock()

	s.TotalSamples = total
	if s.Timestamp.IsZero() {
		s.Timestamp = time.Now()
	}
	return s
}

func (t *Tracker) update() {
	now := time.Now()
	total := t.totalSamples.Load()

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.lastTime.IsZero() {
		t.lastTime = now
		t.lastTotal = total
		t.snapshot = Snapshot{
			TotalSamples: total,
			Timestamp:    now,
		}
		return
	}

	intervalSec := now.Sub(t.lastTime).Seconds()
	if intervalSec <= 0 {
		return
	}

	samples := total - t.lastTotal
	rate := float64(samples) / intervalSec

	t.rates = append(t.rates, rate)
	if len(t.rates) > t.window {
		t.rates = t.rates[1:]
	}

	var avg float64
	for _, r := range t.rates {
		avg += r
	}
	if len(t.rates) > 0 {
		avg /= float64(len(t.rates))
	}

	t.snapshot = Snapshot{
		TotalSamples:      total,
		RatePerSec:        rate,
		AvgRatePerSec:     avg,
		SamplesInInterval: samples,
		IntervalSeconds:   intervalSec,
		Timestamp:         now,
	}
	t.lastTime = now
	t.lastTotal = total
}
