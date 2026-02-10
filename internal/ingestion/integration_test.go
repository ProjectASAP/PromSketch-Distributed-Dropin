package ingestion_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/gogo/protobuf/proto"

	"github.com/promsketch/promsketch-dropin/internal/backend"
	"github.com/promsketch/promsketch-dropin/internal/config"
	"github.com/promsketch/promsketch-dropin/internal/ingestion/pipeline"
	"github.com/promsketch/promsketch-dropin/internal/storage"
)

// mockBackend for testing
type mockBackend struct {
	writeCalls []*prompb.WriteRequest
}

func (m *mockBackend) Write(ctx context.Context, req *prompb.WriteRequest) error {
	m.writeCalls = append(m.writeCalls, req)
	return nil
}

func (m *mockBackend) Query(ctx context.Context, query string, ts time.Time) (*backend.QueryResult, error) {
	return nil, nil
}

func (m *mockBackend) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*backend.QueryResult, error) {
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

func TestIngestionPipeline_EndToEnd(t *testing.T) {
	// Create configuration
	cfg := &config.Config{
		Ingestion: config.IngestionConfig{
			RemoteWrite: config.RemoteWriteConfig{
				Enabled: true,
			},
		},
		Backend: config.BackendConfig{
			Type:          "mock",
			URL:           "http://localhost:8428",
			BatchSize:     10,
			FlushInterval: 100 * time.Millisecond,
			MaxRetries:    1,
			Timeout:       5 * time.Second,
		},
		Sketch: config.SketchConfig{
			NumPartitions: 4,
			Targets: []config.SketchTarget{
				{Match: "http_requests_total"},
			},
			Defaults: config.SketchDefaults{
				EHParams: config.EHParams{
					WindowSize: 1800,
					K:          50,
					KllK:       256,
				},
			},
		},
	}

	// Create storage
	stor, err := storage.NewStorage(&cfg.Sketch)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create mock backend and forwarder
	mockBE := &mockBackend{}
	fwd := backend.NewForwarder(mockBE, &cfg.Backend)
	if err := fwd.Start(); err != nil {
		t.Fatalf("Failed to start forwarder: %v", err)
	}
	defer fwd.Stop()

	// Create pipeline
	pipe, err := pipeline.NewPipeline(cfg, stor, fwd)
	if err != nil {
		t.Fatalf("Failed to create pipeline: %v", err)
	}

	// Create write request
	writeReq := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "http_requests_total"},
					{Name: "job", Value: "api"},
					{Name: "status", Value: "200"},
				},
				Samples: []prompb.Sample{
					{Value: 42.0, Timestamp: 1000},
					{Value: 43.0, Timestamp: 2000},
				},
			},
		},
	}

	// Marshal and compress
	data, err := proto.Marshal(writeReq)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}
	compressed := snappy.Encode(nil, data)

	// Create HTTP request
	req := httptest.NewRequest("POST", "/api/v1/write", bytes.NewReader(compressed))
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")

	// Handle request
	w := httptest.NewRecorder()
	handler := pipe.RemoteWriteHandler()
	handler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d: %s", http.StatusNoContent, w.Code, w.Body.String())
	}

	// Wait for async processing
	time.Sleep(200 * time.Millisecond)

	// Verify storage received samples
	storageMetrics := stor.Metrics()
	if storageMetrics.SamplesInserted != 2 {
		t.Errorf("Expected 2 samples inserted into storage, got %d", storageMetrics.SamplesInserted)
	}

	// Verify backend received samples
	if len(mockBE.writeCalls) == 0 {
		t.Error("Expected backend to receive write calls")
	}

	// Verify pipeline metrics
	pipeMetrics := pipe.Metrics()
	if pipeMetrics.TotalSamplesReceived != 2 {
		t.Errorf("Expected 2 total samples received, got %d", pipeMetrics.TotalSamplesReceived)
	}
}

func TestIngestionPipeline_NonMatchingMetric(t *testing.T) {
	cfg := &config.Config{
		Ingestion: config.IngestionConfig{
			RemoteWrite: config.RemoteWriteConfig{
				Enabled: true,
			},
		},
		Backend: config.BackendConfig{
			Type:          "mock",
			URL:           "http://localhost:8428",
			BatchSize:     10,
			FlushInterval: 100 * time.Millisecond,
			MaxRetries:    1,
			Timeout:       5 * time.Second,
		},
		Sketch: config.SketchConfig{
			NumPartitions: 4,
			Targets: []config.SketchTarget{
				{Match: "http_requests_total"}, // Only match this metric
			},
			Defaults: config.SketchDefaults{
				EHParams: config.EHParams{
					WindowSize: 1800,
					K:          50,
					KllK:       256,
				},
			},
		},
	}

	stor, err := storage.NewStorage(&cfg.Sketch)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	mockBE := &mockBackend{}
	fwd := backend.NewForwarder(mockBE, &cfg.Backend)
	fwd.Start()
	defer fwd.Stop()

	pipe, err := pipeline.NewPipeline(cfg, stor, fwd)
	if err != nil {
		t.Fatalf("Failed to create pipeline: %v", err)
	}

	// Send a non-matching metric
	writeReq := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "other_metric"},
					{Name: "job", Value: "api"},
				},
				Samples: []prompb.Sample{
					{Value: 100.0, Timestamp: 1000},
				},
			},
		},
	}

	data, _ := proto.Marshal(writeReq)
	compressed := snappy.Encode(nil, data)

	req := httptest.NewRequest("POST", "/api/v1/write", bytes.NewReader(compressed))
	w := httptest.NewRecorder()
	handler := pipe.RemoteWriteHandler()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	time.Sleep(200 * time.Millisecond)

	// Storage should not have inserted samples (non-matching metric)
	storageMetrics := stor.Metrics()
	if storageMetrics.SamplesInserted > 0 {
		t.Errorf("Expected 0 samples inserted for non-matching metric, got %d", storageMetrics.SamplesInserted)
	}

	// But backend should still receive samples
	if len(mockBE.writeCalls) == 0 {
		t.Error("Expected backend to receive write calls even for non-matching metrics")
	}
}
