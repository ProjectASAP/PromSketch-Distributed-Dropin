package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetadataAPI_Series(t *testing.T) {
	// Create a mock backend server
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/series" {
			t.Errorf("Expected path /api/v1/series, got %s", r.URL.Path)
		}

		// Check query parameters
		match := r.URL.Query().Get("match[]")
		if match == "" {
			t.Errorf("Expected match[] parameter")
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"status": "success",
			"data": [
				{"__name__": "http_requests_total", "job": "api", "status": "200"},
				{"__name__": "http_requests_total", "job": "api", "status": "404"}
			]
		}`))
	}))
	defer mockBackend.Close()

	// Create metadata API
	metadataAPI := NewMetadataAPI(mockBackend.URL)

	// Test series request
	req := httptest.NewRequest("GET", "/api/v1/series?match[]={job=\"api\"}&start=1234567890&end=1234571490", nil)
	w := httptest.NewRecorder()

	metadataAPI.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check content type
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type=application/json, got %s", ct)
	}

	// Check metrics
	metrics := metadataAPI.Metrics()
	if metrics.SeriesRequests != 1 {
		t.Errorf("Expected 1 series request, got %d", metrics.SeriesRequests)
	}
}

func TestMetadataAPI_Labels(t *testing.T) {
	// Create a mock backend server
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/labels" {
			t.Errorf("Expected path /api/v1/labels, got %s", r.URL.Path)
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"status": "success",
			"data": ["__name__", "job", "instance", "status"]
		}`))
	}))
	defer mockBackend.Close()

	// Create metadata API
	metadataAPI := NewMetadataAPI(mockBackend.URL)

	// Test labels request
	req := httptest.NewRequest("GET", "/api/v1/labels", nil)
	w := httptest.NewRecorder()

	metadataAPI.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check content type
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type=application/json, got %s", ct)
	}

	// Check metrics
	metrics := metadataAPI.Metrics()
	if metrics.LabelsRequests != 1 {
		t.Errorf("Expected 1 labels request, got %d", metrics.LabelsRequests)
	}
}

func TestMetadataAPI_LabelValues(t *testing.T) {
	// Create a mock backend server
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/label/job/values" {
			t.Errorf("Expected path /api/v1/label/job/values, got %s", r.URL.Path)
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"status": "success",
			"data": ["api", "web", "db"]
		}`))
	}))
	defer mockBackend.Close()

	// Create metadata API
	metadataAPI := NewMetadataAPI(mockBackend.URL)

	// Test label values request
	req := httptest.NewRequest("GET", "/api/v1/label/job/values", nil)
	w := httptest.NewRecorder()

	metadataAPI.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check content type
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type=application/json, got %s", ct)
	}

	// Check metrics
	metrics := metadataAPI.Metrics()
	if metrics.LabelValuesRequests != 1 {
		t.Errorf("Expected 1 label values request, got %d", metrics.LabelValuesRequests)
	}
}

func TestMetadataAPI_InvalidEndpoint(t *testing.T) {
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockBackend.Close()

	metadataAPI := NewMetadataAPI(mockBackend.URL)

	// Test invalid endpoint
	req := httptest.NewRequest("GET", "/api/v1/invalid", nil)
	w := httptest.NewRecorder()

	metadataAPI.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestMetadataAPI_BackendError(t *testing.T) {
	// Create a mock backend server that returns errors
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer mockBackend.Close()

	metadataAPI := NewMetadataAPI(mockBackend.URL)

	// Test series request with backend error
	req := httptest.NewRequest("GET", "/api/v1/series?match[]={job=\"api\"}", nil)
	w := httptest.NewRecorder()

	metadataAPI.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	// Check metrics
	metrics := metadataAPI.Metrics()
	if metrics.Errors != 1 {
		t.Errorf("Expected 1 error, got %d", metrics.Errors)
	}
}
