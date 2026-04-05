package backend

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/prometheus/prompb"

	"github.com/promsketch/promsketch-dropin/internal/config"
)

// mockBackend implements Backend for testing
type mockBackend struct {
	mu         sync.Mutex
	writeFunc  func(ctx context.Context, req *prompb.WriteRequest) error
	writeCalls []*prompb.WriteRequest
}

func (m *mockBackend) Write(ctx context.Context, req *prompb.WriteRequest) error {
	m.mu.Lock()
	m.writeCalls = append(m.writeCalls, req)
	m.mu.Unlock()
	if m.writeFunc != nil {
		return m.writeFunc(ctx, req)
	}
	return nil
}

func (m *mockBackend) GetWriteCalls() []*prompb.WriteRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeCalls
}

func (m *mockBackend) Query(ctx context.Context, query string, ts time.Time) (*QueryResult, error) {
	return nil, nil
}

func (m *mockBackend) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*QueryResult, error) {
	return nil, nil
}

func (m *mockBackend) Name() string {
	return "mock"
}

func (m *mockBackend) Health(ctx context.Context) error {
	return nil
}

func (m *mockBackend) Close() error {
	return nil
}

func TestForwarder_Batching(t *testing.T) {
	mock := &mockBackend{}
	cfg := &config.BackendConfig{
		BatchSize:     3,
		FlushInterval: 10 * time.Second,
		MaxRetries:    1,
		Timeout:       5 * time.Second,
	}

	fwd := NewForwarder(mock, cfg)
	if err := fwd.Start(); err != nil {
		t.Fatalf("Failed to start forwarder: %v", err)
	}
	defer fwd.Stop()

	// Send 3 samples (should trigger a batch)
	for i := 0; i < 3; i++ {
		ts := &prompb.TimeSeries{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "test_metric"},
			},
			Samples: []prompb.Sample{
				{Value: float64(i), Timestamp: int64(i)},
			},
		}
		if err := fwd.Forward(ts); err != nil {
			t.Fatalf("Failed to forward sample %d: %v", i, err)
		}
	}

	// Wait for batch to be sent
	time.Sleep(100 * time.Millisecond)

	writeCalls := mock.GetWriteCalls()
	if len(writeCalls) != 1 {
		t.Errorf("Expected 1 write call, got %d", len(writeCalls))
	}

	if len(writeCalls) > 0 && len(writeCalls[0].Timeseries) != 3 {
		t.Errorf("Expected batch of 3, got %d", len(writeCalls[0].Timeseries))
	}
}

func TestForwarder_FlushInterval(t *testing.T) {
	mock := &mockBackend{}
	cfg := &config.BackendConfig{
		BatchSize:     100,
		FlushInterval: 100 * time.Millisecond,
		MaxRetries:    1,
		Timeout:       5 * time.Second,
	}

	fwd := NewForwarder(mock, cfg)
	if err := fwd.Start(); err != nil {
		t.Fatalf("Failed to start forwarder: %v", err)
	}
	defer fwd.Stop()

	// Send 1 sample (won't trigger batch size)
	ts := &prompb.TimeSeries{
		Labels: []prompb.Label{
			{Name: "__name__", Value: "test_metric"},
		},
		Samples: []prompb.Sample{
			{Value: 42.0, Timestamp: 1000},
		},
	}
	if err := fwd.Forward(ts); err != nil {
		t.Fatalf("Failed to forward sample: %v", err)
	}

	// Wait for flush interval
	time.Sleep(200 * time.Millisecond)

	writeCalls := mock.GetWriteCalls()
	if len(writeCalls) != 1 {
		t.Errorf("Expected 1 write call, got %d", len(writeCalls))
	}
}

func TestForwarder_Metrics(t *testing.T) {
	mock := &mockBackend{}
	cfg := &config.BackendConfig{
		BatchSize:     2,
		FlushInterval: 10 * time.Second,
		MaxRetries:    1,
		Timeout:       5 * time.Second,
	}

	fwd := NewForwarder(mock, cfg)
	if err := fwd.Start(); err != nil {
		t.Fatalf("Failed to start forwarder: %v", err)
	}
	defer fwd.Stop()

	// Send 2 samples
	for i := 0; i < 2; i++ {
		ts := &prompb.TimeSeries{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "test_metric"},
			},
			Samples: []prompb.Sample{
				{Value: float64(i), Timestamp: int64(i)},
			},
		}
		fwd.Forward(ts)
	}

	time.Sleep(100 * time.Millisecond)

	metrics := fwd.Metrics()
	if metrics.SamplesForwarded != 2 {
		t.Errorf("Expected 2 samples forwarded, got %d", metrics.SamplesForwarded)
	}
	if metrics.BatchesSent != 1 {
		t.Errorf("Expected 1 batch sent, got %d", metrics.BatchesSent)
	}
}
