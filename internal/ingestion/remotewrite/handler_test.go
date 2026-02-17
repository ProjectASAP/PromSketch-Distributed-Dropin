package remotewrite

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"

	vmprompb "github.com/zzylol/VictoriaMetrics/lib/prompb"
)

type mockReceiver struct {
	mu             sync.Mutex
	receivedCalls  int
	totalSamples   int
	totalSeries    int
	receiveFunc    func(tss []vmprompb.TimeSeries) error
}

func (m *mockReceiver) ReceiveVMTimeSeries(tss []vmprompb.TimeSeries) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.receivedCalls++
	m.totalSeries += len(tss)
	for _, ts := range tss {
		m.totalSamples += len(ts.Samples)
	}
	if m.receiveFunc != nil {
		return m.receiveFunc(tss)
	}
	return nil
}

func TestHandler_ValidRequest(t *testing.T) {
	mock := &mockReceiver{}
	handler := NewHandler(mock)

	// Create a write request using Prometheus prompb types (same wire format)
	req := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "test_metric"},
					{Name: "job", Value: "test"},
				},
				Samples: []prompb.Sample{
					{Value: 42.0, Timestamp: 1000},
				},
			},
		},
	}

	// Marshal and compress
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}
	compressed := snappy.Encode(nil, data)

	// Create HTTP request
	httpReq := httptest.NewRequest("POST", "/api/v1/write", bytes.NewReader(compressed))
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("Content-Encoding", "snappy")

	// Create response recorder
	w := httptest.NewRecorder()

	// Handle request
	handler.ServeHTTP(w, httpReq)

	// Check response
	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d: %s", http.StatusNoContent, w.Code, w.Body.String())
	}

	// Check that receiver was called
	mock.mu.Lock()
	calls := mock.receivedCalls
	series := mock.totalSeries
	samples := mock.totalSamples
	mock.mu.Unlock()

	if calls != 1 {
		t.Errorf("Expected 1 received call, got %d", calls)
	}
	if series != 1 {
		t.Errorf("Expected 1 series, got %d", series)
	}
	if samples != 1 {
		t.Errorf("Expected 1 sample, got %d", samples)
	}

	// Verify metrics
	metrics := handler.Metrics()
	if metrics.RequestsReceived != 1 {
		t.Errorf("Expected 1 request received, got %d", metrics.RequestsReceived)
	}
	if metrics.SamplesReceived != 1 {
		t.Errorf("Expected 1 sample received, got %d", metrics.SamplesReceived)
	}
}

func TestHandler_InvalidMethod(t *testing.T) {
	mock := &mockReceiver{}
	handler := NewHandler(mock)

	httpReq := httptest.NewRequest("GET", "/api/v1/write", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	metrics := handler.Metrics()
	if metrics.RequestsFailed != 1 {
		t.Errorf("Expected 1 failed request, got %d", metrics.RequestsFailed)
	}
}

func TestHandler_InvalidData(t *testing.T) {
	mock := &mockReceiver{}
	handler := NewHandler(mock)

	// Send invalid data (not valid snappy/zstd)
	httpReq := httptest.NewRequest("POST", "/api/v1/write", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandler_MultipleSamples(t *testing.T) {
	mock := &mockReceiver{}
	handler := NewHandler(mock)

	// Create a write request with multiple samples
	req := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "metric1"},
				},
				Samples: []prompb.Sample{
					{Value: 1.0, Timestamp: 1000},
					{Value: 2.0, Timestamp: 2000},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "metric2"},
				},
				Samples: []prompb.Sample{
					{Value: 3.0, Timestamp: 3000},
				},
			},
		},
	}

	data, _ := proto.Marshal(req)
	compressed := snappy.Encode(nil, data)

	httpReq := httptest.NewRequest("POST", "/api/v1/write", bytes.NewReader(compressed))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d: %s", http.StatusNoContent, w.Code, w.Body.String())
	}

	metrics := handler.Metrics()
	if metrics.SamplesReceived != 3 {
		t.Errorf("Expected 3 samples received, got %d", metrics.SamplesReceived)
	}
}
