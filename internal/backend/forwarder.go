package backend

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/prometheus/prompb"

	"github.com/promsketch/promsketch-dropin/internal/config"
)

// Forwarder batches and forwards samples to a backend
type Forwarder struct {
	backend       Backend
	config        *config.BackendConfig
	batchCh       chan *prompb.TimeSeries
	flushCh       chan struct{}
	stopCh        chan struct{}
	wg            sync.WaitGroup
	metrics       *forwarderMetrics
	mu            sync.Mutex
	currentBatch  []*prompb.TimeSeries
	lastFlush     time.Time
}

// forwarderMetrics is the internal atomic state for forwarder counters
type forwarderMetrics struct {
	samplesForwarded atomic.Uint64
	samplesDropped   atomic.Uint64
	batchesSent      atomic.Uint64
	batchesFailed    atomic.Uint64
	forwardLatencyMs atomic.Uint64
}

// ForwarderMetrics is a point-in-time snapshot of forwarder statistics
type ForwarderMetrics struct {
	SamplesForwarded uint64
	SamplesDropped   uint64
	BatchesSent      uint64
	BatchesFailed    uint64
	ForwardLatencyMs uint64
}

// NewForwarder creates a new backend forwarder
func NewForwarder(backend Backend, cfg *config.BackendConfig) *Forwarder {
	f := &Forwarder{
		backend:      backend,
		config:       cfg,
		batchCh:      make(chan *prompb.TimeSeries, cfg.BatchSize*10),
		flushCh:      make(chan struct{}, 1),
		stopCh:       make(chan struct{}),
		metrics:      &forwarderMetrics{},
		currentBatch: make([]*prompb.TimeSeries, 0, cfg.BatchSize),
		lastFlush:    time.Now(),
	}

	return f
}

// Start begins the forwarder background workers
func (f *Forwarder) Start() error {
	// Start batch accumulator
	f.wg.Add(1)
	go f.batchWorker()

	// Start flush timer
	f.wg.Add(1)
	go f.flushTimer()

	return nil
}

// Forward queues a time series for forwarding to the backend.
// The TimeSeries is deep-copied so the caller can safely reuse the buffer.
func (f *Forwarder) Forward(ts *prompb.TimeSeries) error {
	// Deep copy to decouple from caller's buffer
	copied := &prompb.TimeSeries{
		Labels:  make([]prompb.Label, len(ts.Labels)),
		Samples: make([]prompb.Sample, len(ts.Samples)),
	}
	copy(copied.Labels, ts.Labels)
	copy(copied.Samples, ts.Samples)

	select {
	case f.batchCh <- copied:
		return nil
	default:
		// Channel full, drop sample
		f.metrics.samplesDropped.Add(1)
		return fmt.Errorf("forwarder queue full, sample dropped")
	}
}

// batchWorker accumulates samples into batches
func (f *Forwarder) batchWorker() {
	defer f.wg.Done()

	for {
		select {
		case <-f.stopCh:
			// Flush remaining samples before stopping
			f.flush()
			return

		case ts := <-f.batchCh:
			f.mu.Lock()
			f.currentBatch = append(f.currentBatch, ts)
			shouldFlush := len(f.currentBatch) >= f.config.BatchSize
			f.mu.Unlock()

			if shouldFlush {
				f.triggerFlush()
			}

		case <-f.flushCh:
			f.flush()
		}
	}
}

// flushTimer periodically triggers flushes
func (f *Forwarder) flushTimer() {
	defer f.wg.Done()

	ticker := time.NewTicker(f.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-f.stopCh:
			return
		case <-ticker.C:
			f.triggerFlush()
		}
	}
}

// triggerFlush signals the batch worker to flush
func (f *Forwarder) triggerFlush() {
	select {
	case f.flushCh <- struct{}{}:
	default:
		// Flush already pending
	}
}

// flush sends the current batch to the backend
func (f *Forwarder) flush() {
	f.mu.Lock()
	if len(f.currentBatch) == 0 {
		f.mu.Unlock()
		return
	}

	batch := f.currentBatch
	f.currentBatch = make([]*prompb.TimeSeries, 0, f.config.BatchSize)
	f.lastFlush = time.Now()
	f.mu.Unlock()

	// Send to backend with retries
	err := f.sendBatch(batch)
	if err != nil {
		f.metrics.batchesFailed.Add(1)
		// In production, we might want to retry or log the error
		return
	}

	f.metrics.batchesSent.Add(1)
	f.metrics.samplesForwarded.Add(uint64(len(batch)))
}

// sendBatch sends a batch to the backend with retry logic
func (f *Forwarder) sendBatch(batch []*prompb.TimeSeries) error {
	// Convert []*prompb.TimeSeries to []prompb.TimeSeries
	timeseries := make([]prompb.TimeSeries, len(batch))
	for i, ts := range batch {
		if ts != nil {
			timeseries[i] = *ts
		}
	}

	req := &prompb.WriteRequest{
		Timeseries: timeseries,
	}

	var lastErr error
	for attempt := 0; attempt < f.config.MaxRetries; attempt++ {
		start := time.Now()

		ctx, cancel := context.WithTimeout(context.Background(), f.config.Timeout)
		err := f.backend.Write(ctx, req)
		cancel()

		latency := time.Since(start).Milliseconds()
		f.metrics.forwardLatencyMs.Store(uint64(latency))

		if err == nil {
			return nil
		}

		lastErr = err

		// Exponential backoff
		if attempt < f.config.MaxRetries-1 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("failed to send batch after %d retries: %w", f.config.MaxRetries, lastErr)
}

// Stop gracefully stops the forwarder
func (f *Forwarder) Stop() error {
	close(f.stopCh)
	f.wg.Wait()
	return f.backend.Close()
}

// Metrics returns a point-in-time snapshot of the current forwarder metrics
func (f *Forwarder) Metrics() ForwarderMetrics {
	return ForwarderMetrics{
		SamplesForwarded: f.metrics.samplesForwarded.Load(),
		SamplesDropped:   f.metrics.samplesDropped.Load(),
		BatchesSent:      f.metrics.batchesSent.Load(),
		BatchesFailed:    f.metrics.batchesFailed.Load(),
		ForwardLatencyMs: f.metrics.forwardLatencyMs.Load(),
	}
}
