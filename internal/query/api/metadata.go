package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/promsketch/promsketch-dropin/internal/metrics"
)

// MetadataAPI handles Prometheus metadata API endpoints
type MetadataAPI struct {
	backendURL string
	httpClient *http.Client
	metrics    *metadataMetrics
}

// metadataMetrics is the internal atomic state for metadata endpoint counters
type metadataMetrics struct {
	seriesRequests      atomic.Int64
	labelsRequests      atomic.Int64
	labelValuesRequests atomic.Int64
	errors              atomic.Int64
}

// MetadataMetrics is a point-in-time snapshot of metadata endpoint usage
type MetadataMetrics struct {
	SeriesRequests      int64
	LabelsRequests      int64
	LabelValuesRequests int64
	Errors              int64
}

// NewMetadataAPI creates a new metadata API handler
func NewMetadataAPI(backendURL string) *MetadataAPI {
	return &MetadataAPI{
		backendURL: backendURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		metrics: &metadataMetrics{},
	}
}

// ServeHTTP handles metadata endpoint requests
func (m *MetadataAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/v1/series":
		m.handleSeries(w, r)
	case path == "/api/v1/labels":
		m.handleLabels(w, r)
	case strings.HasPrefix(path, "/api/v1/label/") && strings.HasSuffix(path, "/values"):
		m.handleLabelValues(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// handleSeries handles /api/v1/series requests
// Returns list of time series matching label matchers
func (m *MetadataAPI) handleSeries(w http.ResponseWriter, r *http.Request) {
	m.metrics.seriesRequests.Add(1)
	metrics.MetadataRequestsTotal.WithLabelValues("series").Inc()

	// Parse query parameters
	if err := r.ParseForm(); err != nil {
		m.metrics.errors.Add(1)
		metrics.MetadataErrorsTotal.Inc()
		writeError(w, "Invalid query parameters", http.StatusBadRequest)
		return
	}

	// Build backend URL
	backendURL, err := url.Parse(m.backendURL)
	if err != nil {
		m.metrics.errors.Add(1)
		metrics.MetadataErrorsTotal.Inc()
		writeError(w, "Backend URL error", http.StatusInternalServerError)
		return
	}

	backendURL.Path = "/api/v1/series"
	backendURL.RawQuery = r.URL.RawQuery

	// Proxy request to backend
	resp, err := m.proxyRequest(r.Context(), backendURL.String())
	if err != nil {
		m.metrics.errors.Add(1)
		metrics.MetadataErrorsTotal.Inc()
		writeError(w, fmt.Sprintf("Backend query failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// handleLabels handles /api/v1/labels requests
// Returns list of all label names
func (m *MetadataAPI) handleLabels(w http.ResponseWriter, r *http.Request) {
	m.metrics.labelsRequests.Add(1)
	metrics.MetadataRequestsTotal.WithLabelValues("labels").Inc()

	// Parse query parameters
	if err := r.ParseForm(); err != nil {
		m.metrics.errors.Add(1)
		metrics.MetadataErrorsTotal.Inc()
		writeError(w, "Invalid query parameters", http.StatusBadRequest)
		return
	}

	// Build backend URL
	backendURL, err := url.Parse(m.backendURL)
	if err != nil {
		m.metrics.errors.Add(1)
		metrics.MetadataErrorsTotal.Inc()
		writeError(w, "Backend URL error", http.StatusInternalServerError)
		return
	}

	backendURL.Path = "/api/v1/labels"
	backendURL.RawQuery = r.URL.RawQuery

	// Proxy request to backend
	resp, err := m.proxyRequest(r.Context(), backendURL.String())
	if err != nil {
		m.metrics.errors.Add(1)
		metrics.MetadataErrorsTotal.Inc()
		writeError(w, fmt.Sprintf("Backend query failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// handleLabelValues handles /api/v1/label/{name}/values requests
// Returns list of values for a specific label
func (m *MetadataAPI) handleLabelValues(w http.ResponseWriter, r *http.Request) {
	m.metrics.labelValuesRequests.Add(1)
	metrics.MetadataRequestsTotal.WithLabelValues("label_values").Inc()

	// Extract label name from path
	// Path format: /api/v1/label/{name}/values
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		m.metrics.errors.Add(1)
		metrics.MetadataErrorsTotal.Inc()
		writeError(w, "Invalid label path", http.StatusBadRequest)
		return
	}
	labelName := parts[4]

	// Parse query parameters
	if err := r.ParseForm(); err != nil {
		m.metrics.errors.Add(1)
		metrics.MetadataErrorsTotal.Inc()
		writeError(w, "Invalid query parameters", http.StatusBadRequest)
		return
	}

	// Build backend URL
	backendURL, err := url.Parse(m.backendURL)
	if err != nil {
		m.metrics.errors.Add(1)
		metrics.MetadataErrorsTotal.Inc()
		writeError(w, "Backend URL error", http.StatusInternalServerError)
		return
	}

	backendURL.Path = fmt.Sprintf("/api/v1/label/%s/values", labelName)
	backendURL.RawQuery = r.URL.RawQuery

	// Proxy request to backend
	resp, err := m.proxyRequest(r.Context(), backendURL.String())
	if err != nil {
		m.metrics.errors.Add(1)
		metrics.MetadataErrorsTotal.Inc()
		writeError(w, fmt.Sprintf("Backend query failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// proxyRequest sends a request to the backend and returns the response
func (m *MetadataAPI) proxyRequest(ctx context.Context, backendURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", backendURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// writeError writes a Prometheus-compatible error response
func writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := map[string]interface{}{
		"status": "error",
		"error":  message,
	}

	json.NewEncoder(w).Encode(resp)
}

// Metrics returns a point-in-time snapshot of the current metadata metrics
func (m *MetadataAPI) Metrics() MetadataMetrics {
	return MetadataMetrics{
		SeriesRequests:      m.metrics.seriesRequests.Load(),
		LabelsRequests:      m.metrics.labelsRequests.Load(),
		LabelValuesRequests: m.metrics.labelValuesRequests.Load(),
		Errors:              m.metrics.errors.Load(),
	}
}
