package query_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"github.com/promsketch/promsketch-dropin/internal/backend"
	"github.com/promsketch/promsketch-dropin/internal/config"
	"github.com/promsketch/promsketch-dropin/internal/query/api"
	"github.com/promsketch/promsketch-dropin/internal/query/capabilities"
	"github.com/promsketch/promsketch-dropin/internal/query/parser"
	"github.com/promsketch/promsketch-dropin/internal/query/router"
	"github.com/promsketch/promsketch-dropin/internal/storage"
)

// mockBackend for testing
type mockBackend struct {
	queryCalls      int
	queryRangeCalls int
}

func (m *mockBackend) Write(ctx context.Context, req *prompb.WriteRequest) error {
	return nil
}

func (m *mockBackend) Query(ctx context.Context, query string, ts time.Time) (*backend.QueryResult, error) {
	m.queryCalls++
	return &backend.QueryResult{
		ResultType: "vector",
		Result:     []interface{}{},
	}, nil
}

func (m *mockBackend) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*backend.QueryResult, error) {
	m.queryRangeCalls++
	return &backend.QueryResult{
		ResultType: "matrix",
		Result:     []interface{}{},
	}, nil
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

func TestQueryRouter_Integration(t *testing.T) {
	// Create storage
	cfg := &config.SketchConfig{
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
	}

	stor, err := storage.NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create mock backend
	mockBE := &mockBackend{}

	// Create query components
	p := parser.NewParser()
	cap := capabilities.NewRegistry()
	r := router.NewRouter(stor, mockBE, p, cap)

	// Test instant query with supported function
	t.Run("InstantQuery_SupportedFunction", func(t *testing.T) {
		query := `avg_over_time(http_requests_total[5m])`
		ts := time.Now()

		result, err := r.Query(context.Background(), query, ts)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		// Should try sketches first, then fall back to backend
		// Since we don't have sketch data, it should fall back
		if result.Source != "backend" {
			t.Errorf("Expected source=backend, got %s", result.Source)
		}

		if mockBE.queryCalls != 1 {
			t.Errorf("Expected 1 backend query call, got %d", mockBE.queryCalls)
		}
	})

	// Test range query
	t.Run("RangeQuery_SupportedFunction", func(t *testing.T) {
		mockBE.queryRangeCalls = 0

		query := `sum_over_time(http_requests_total[5m])`
		start := time.Now().Add(-1 * time.Hour)
		end := time.Now()
		step := 1 * time.Minute

		result, err := r.QueryRange(context.Background(), query, start, end, step)
		if err != nil {
			t.Fatalf("QueryRange failed: %v", err)
		}

		if result.Source != "backend" {
			t.Errorf("Expected source=backend, got %s", result.Source)
		}

		if mockBE.queryRangeCalls != 1 {
			t.Errorf("Expected 1 backend query range call, got %d", mockBE.queryRangeCalls)
		}
	})

	// Test unsupported function
	t.Run("UnsupportedFunction_FallbackToBackend", func(t *testing.T) {
		mockBE.queryCalls = 0

		query := `rate(http_requests_total[5m])`
		ts := time.Now()

		result, err := r.Query(context.Background(), query, ts)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if result.Source != "backend" {
			t.Errorf("Expected source=backend for unsupported function, got %s", result.Source)
		}

		if mockBE.queryCalls != 1 {
			t.Errorf("Expected 1 backend query call, got %d", mockBE.queryCalls)
		}
	})

	// Check metrics
	metrics := r.Metrics()
	if metrics.SketchQueries < 2 {
		t.Errorf("Expected at least 2 sketch queries attempted, got %d", metrics.SketchQueries)
	}
	if metrics.BackendQueries < 3 {
		t.Errorf("Expected at least 3 backend queries, got %d", metrics.BackendQueries)
	}
}

func TestQueryAPI_Handlers(t *testing.T) {
	// Create storage and router
	cfg := &config.SketchConfig{
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
	}

	stor, err := storage.NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	mockBE := &mockBackend{}
	p := parser.NewParser()
	cap := capabilities.NewRegistry()
	r := router.NewRouter(stor, mockBE, p, cap)

	// Create API handler
	queryAPI := api.NewQueryAPI(r)

	// Test instant query endpoint
	t.Run("InstantQuery_Endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/query?query=avg_over_time(http_requests_total[5m])&time=1234567890", nil)
		w := httptest.NewRecorder()

		queryAPI.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		// Check content type
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Expected Content-Type=application/json, got %s", ct)
		}
	})

	// Test missing query parameter
	t.Run("MissingQuery_Parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/query", nil)
		w := httptest.NewRecorder()

		queryAPI.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	// Test range query endpoint
	t.Run("RangeQuery_Endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/query_range?query=sum_over_time(http_requests_total[5m])&start=1234567890&end=1234571490&step=60", nil)
		w := httptest.NewRecorder()

		queryAPI.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Test missing range parameters
	t.Run("MissingRangeParams", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/query_range?query=sum_over_time(http_requests_total[5m])", nil)
		w := httptest.NewRecorder()

		queryAPI.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	// Test invalid endpoint
	t.Run("InvalidEndpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/invalid", nil)
		w := httptest.NewRecorder()

		queryAPI.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	// Check API metrics
	metrics := queryAPI.Metrics()
	if metrics.QueryRequests < 1 {
		t.Errorf("Expected at least 1 query request, got %d", metrics.QueryRequests)
	}
	if metrics.QueryRangeRequests < 1 {
		t.Errorf("Expected at least 1 query range request, got %d", metrics.QueryRangeRequests)
	}
}
