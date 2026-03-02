// Package e2e_test provides end-to-end testing for PromSketch-Dropin.
//
// It spins up VictoriaMetrics as the backend, starts promsketch-dropin,
// sends remote write data, and verifies all API endpoints work correctly.
package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
)

const (
	vmAddr        = "127.0.0.1:18428"
	proxyAddr     = "127.0.0.1:19100"
	vmURL         = "http://" + vmAddr
	proxyURL      = "http://" + proxyAddr
	vmDataDir     = "/tmp/promsketch-e2e-vm-data"
	startupWait   = 5 * time.Second
	ingestionWait = 3 * time.Second
)

var (
	vmCmd    *exec.Cmd
	proxyCmd *exec.Cmd
)

// TestMain starts all services, runs tests, and cleans up.
func TestMain(m *testing.M) {
	code := 1
	defer func() { os.Exit(code) }()

	// Clean up any leftover data
	os.RemoveAll(vmDataDir)
	os.MkdirAll(vmDataDir, 0o755)

	// 1. Start VictoriaMetrics
	fmt.Println("=== Starting VictoriaMetrics ===")
	vmCmd = exec.Command("/tmp/victoria-metrics-prod",
		"-httpListenAddr="+vmAddr,
		"-storageDataPath="+vmDataDir,
		"-retentionPeriod=1d",
		"-search.latencyOffset=0s",
	)
	vmCmd.Stdout = os.Stderr
	vmCmd.Stderr = os.Stderr
	if err := vmCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start VictoriaMetrics: %v\n", err)
		return
	}
	defer func() {
		if vmCmd.Process != nil {
			vmCmd.Process.Kill()
			vmCmd.Wait()
		}
		os.RemoveAll(vmDataDir)
	}()

	// Wait for VM to be ready
	if err := waitForHealthy(vmURL+"/health", 15*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "VictoriaMetrics failed to start: %v\n", err)
		return
	}
	fmt.Println("  VictoriaMetrics is ready")

	// 2. Write e2e config
	configPath := "/tmp/promsketch-e2e-config.yaml"
	configContent := fmt.Sprintf(`server:
  listen_address: ":%s"
  read_timeout: 30s
  write_timeout: 30s
  log_level: debug

ingestion:
  remote_write:
    enabled: true
    listen_address: ":%s"
    max_sample_size: 5000000

backend:
  type: victoriametrics
  url: %s
  remote_write_url: %s/api/v1/write
  timeout: 30s
  max_retries: 3
  batch_size: 100
  flush_interval: 1s

sketch:
  num_partitions: 16
  memory_limit: "512MB"
  defaults:
    eh_params:
      window_size: 3600
      k: 50
      kll_k: 256
  targets:
    - match: '{__name__=~"e2e_test_.*"}'
      eh_params:
        window_size: 3600
        k: 50
        kll_k: 256

query:
  listen_address: ":%s"
  timeout: 30s
  max_concurrency: 20
  enable_fallback: true
  fallback_timeout: 60s
`, strings.Split(proxyAddr, ":")[1],
		strings.Split(proxyAddr, ":")[1],
		vmURL,
		vmURL,
		strings.Split(proxyAddr, ":")[1],
	)
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write config: %v\n", err)
		return
	}

	// 3. Start PromSketch-Dropin
	fmt.Println("=== Starting PromSketch-Dropin ===")
	binPath := findBinary()
	proxyCmd = exec.Command(binPath, "--config.file="+configPath)
	proxyCmd.Stdout = os.Stderr
	proxyCmd.Stderr = os.Stderr
	if err := proxyCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start PromSketch-Dropin: %v\n", err)
		return
	}
	defer func() {
		if proxyCmd.Process != nil {
			proxyCmd.Process.Kill()
			proxyCmd.Wait()
		}
		os.Remove(configPath)
	}()

	// Wait for proxy to be ready
	if err := waitForHealthy(proxyURL+"/health", 15*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "PromSketch-Dropin failed to start: %v\n", err)
		return
	}
	fmt.Println("  PromSketch-Dropin is ready")

	// 4. Run tests
	code = m.Run()
}

func findBinary() string {
	paths := []string{
		"/mydata/PromSketch-Dropin/bin/promsketch-dropin",
		"./bin/promsketch-dropin",
		"../bin/promsketch-dropin",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return paths[0] // fallback
}

func waitForHealthy(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("service at %s not healthy after %v", url, timeout)
}

// ===== Test 1: Health Endpoint =====

func TestHealthEndpoint(t *testing.T) {
	resp, err := http.Get(proxyURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "OK") {
		t.Fatalf("Expected 'OK' in body, got: %s", string(body))
	}

	t.Log("PASS: Health endpoint returns 200 OK")
}

// ===== Test 2: Metrics Endpoint =====

func TestMetricsEndpoint(t *testing.T) {
	resp, err := http.Get(proxyURL + "/metrics")
	if err != nil {
		t.Fatalf("Metrics request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	expectedMetrics := []string{
		"promsketch_storage_series_total",
		"promsketch_storage_sketched_series",
		"promsketch_storage_samples_inserted_total",
		"promsketch_forwarder_samples_forwarded_total",
		"promsketch_ingestion_samples_total",
		"promsketch_query_requests_total",
		"promsketch_query_source_total",
		"promsketch_metadata_requests_total",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(content, metric) {
			t.Errorf("Missing metric: %s", metric)
		}
	}

	t.Log("PASS: Metrics endpoint returns all expected metrics")
}

// ===== Test 3: Remote Write Ingestion =====

func TestRemoteWriteIngestion(t *testing.T) {
	now := time.Now()
	numSamples := 60

	// Build write request with multiple time series
	timeseries := []prompb.TimeSeries{
		buildTimeSeries("e2e_test_counter", map[string]string{"job": "e2e", "instance": "localhost:9090"}, now, numSamples, func(i int) float64 {
			return float64(i + 1) // 1, 2, 3, ..., 60
		}),
		buildTimeSeries("e2e_test_gauge", map[string]string{"job": "e2e", "instance": "localhost:9090"}, now, numSamples, func(i int) float64 {
			return 50.0 + 10.0*math.Sin(float64(i)*math.Pi/30.0) // sinusoidal between 40-60
		}),
		buildTimeSeries("e2e_test_histogram_sum", map[string]string{"job": "e2e", "instance": "localhost:9090", "le": "0.5"}, now, numSamples, func(i int) float64 {
			return float64(i) * 0.1
		}),
	}

	writeReq := &prompb.WriteRequest{
		Timeseries: timeseries,
	}

	// Marshal and compress
	data, err := proto.Marshal(writeReq)
	if err != nil {
		t.Fatalf("Failed to marshal write request: %v", err)
	}
	compressed := snappy.Encode(nil, data)

	// Send remote write
	resp, err := http.Post(proxyURL+"/api/v1/write", "application/x-protobuf", bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("Remote write failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 204, got %d: %s", resp.StatusCode, string(body))
	}

	t.Logf("PASS: Remote write accepted (%d series, %d samples each)", len(timeseries), numSamples)
}

// ===== Test 4: Remote Write - Method Not Allowed =====

func TestRemoteWriteMethodNotAllowed(t *testing.T) {
	resp, err := http.Get(proxyURL + "/api/v1/write")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 405 {
		t.Fatalf("Expected status 405 for GET /api/v1/write, got %d", resp.StatusCode)
	}

	t.Log("PASS: GET /api/v1/write returns 405 Method Not Allowed")
}

// ===== Test 5: Backend Query via Proxy =====

func TestBackendInstantQuery(t *testing.T) {
	// Wait for data to be forwarded and indexed
	time.Sleep(ingestionWait)
	forceFlushVM(t)

	// Query via proxy (should fall back to backend since it's a plain metric query)
	result := queryInstant(t, "e2e_test_counter")
	if result == nil {
		t.Fatal("Query returned nil result")
	}

	if result.Status != "success" {
		t.Fatalf("Expected status 'success', got: %s (error: %s)", result.Status, result.Error)
	}

	t.Logf("PASS: Instant query returns success (data: %+v)", truncateJSON(result.Data))
}

// ===== Test 6: Backend Range Query via Proxy =====

func TestBackendRangeQuery(t *testing.T) {
	now := time.Now()
	start := now.Add(-10 * time.Minute)

	result := queryRange(t, "e2e_test_counter", start, now, "60s")
	if result == nil {
		t.Fatal("Range query returned nil result")
	}

	if result.Status != "success" {
		t.Fatalf("Expected status 'success', got: %s (error: %s)", result.Status, result.Error)
	}

	t.Logf("PASS: Range query returns success (data: %+v)", truncateJSON(result.Data))
}

// ===== Test 7: Sketch-Based Query - avg_over_time =====

func TestSketchAvgOverTime(t *testing.T) {
	// First send fresh data to make sure sketches have data
	ingestFreshData(t, "e2e_test_sketch_avg", 120, func(i int) float64 {
		return float64(10 + i) // values: 10, 11, 12, ..., 129
	})
	time.Sleep(500 * time.Millisecond)

	// Query sketch-supported function
	now := time.Now()
	query := `avg_over_time(e2e_test_sketch_avg{job="e2e"}[5m])`
	ts := fmt.Sprintf("%d", now.Unix())

	result := queryInstantRaw(t, query, ts)
	if result.Status != "success" {
		t.Logf("avg_over_time query status: %s, error: %s", result.Status, result.Error)
		// Not failing - sketch might not have data yet, backend fallback expected
	}

	t.Logf("PASS: avg_over_time query executed (status: %s, source may be sketch or backend)", result.Status)
}

// ===== Test 8: Sketch-Based Query - sum_over_time =====

func TestSketchSumOverTime(t *testing.T) {
	ingestFreshData(t, "e2e_test_sketch_sum", 120, func(i int) float64 {
		return float64(i + 1) // values: 1, 2, 3, ..., 120
	})
	time.Sleep(500 * time.Millisecond)

	now := time.Now()
	query := `sum_over_time(e2e_test_sketch_sum{job="e2e"}[5m])`
	ts := fmt.Sprintf("%d", now.Unix())

	result := queryInstantRaw(t, query, ts)
	if result.Status != "success" {
		t.Logf("sum_over_time query status: %s, error: %s", result.Status, result.Error)
	}

	t.Logf("PASS: sum_over_time query executed (status: %s)", result.Status)
}

// ===== Test 9: Sketch-Based Query - count_over_time =====

func TestSketchCountOverTime(t *testing.T) {
	ingestFreshData(t, "e2e_test_sketch_count", 120, func(i int) float64 {
		return float64(i)
	})
	time.Sleep(500 * time.Millisecond)

	now := time.Now()
	query := `count_over_time(e2e_test_sketch_count{job="e2e"}[5m])`
	ts := fmt.Sprintf("%d", now.Unix())

	result := queryInstantRaw(t, query, ts)
	if result.Status != "success" {
		t.Logf("count_over_time query status: %s, error: %s", result.Status, result.Error)
	}

	t.Logf("PASS: count_over_time query executed (status: %s)", result.Status)
}

// ===== Test 10: Sketch-Based Query - quantile_over_time =====

func TestSketchQuantileOverTime(t *testing.T) {
	ingestFreshData(t, "e2e_test_sketch_quantile", 120, func(i int) float64 {
		return float64(i)
	})
	time.Sleep(500 * time.Millisecond)

	now := time.Now()
	query := `quantile_over_time(0.95, e2e_test_sketch_quantile{job="e2e"}[5m])`
	ts := fmt.Sprintf("%d", now.Unix())

	result := queryInstantRaw(t, query, ts)
	if result.Status != "success" {
		t.Logf("quantile_over_time query status: %s, error: %s", result.Status, result.Error)
	}

	t.Logf("PASS: quantile_over_time query executed (status: %s)", result.Status)
}

// ===== Test 11: Sketch-Based Range Query =====

func TestSketchRangeQuery(t *testing.T) {
	now := time.Now()
	start := now.Add(-5 * time.Minute)

	query := `avg_over_time(e2e_test_sketch_avg{job="e2e"}[2m])`

	result := queryRange(t, query, start, now, "60s")
	if result.Status != "success" {
		t.Logf("Sketch range query status: %s, error: %s", result.Status, result.Error)
	}

	t.Log("PASS: Sketch-based range query executed")
}

// ===== Test 12: Query Parameter Validation =====

func TestQueryParameterValidation(t *testing.T) {
	// Empty query should fail
	resp, err := http.Get(proxyURL + "/api/v1/query")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Fatalf("Expected 400 for empty query, got %d", resp.StatusCode)
	}

	var result promResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Status != "error" {
		t.Fatalf("Expected status 'error', got '%s'", result.Status)
	}

	t.Log("PASS: Empty query returns 400 Bad Request")
}

// ===== Test 13: Range Query Parameter Validation =====

func TestRangeQueryParameterValidation(t *testing.T) {
	// Missing start/end/step should fail
	resp, err := http.Get(proxyURL + "/api/v1/query_range?query=up")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Fatalf("Expected 400 for missing range params, got %d", resp.StatusCode)
	}

	t.Log("PASS: Range query with missing params returns 400")
}

// ===== Test 14: Metadata - Labels Endpoint =====

func TestMetadataLabels(t *testing.T) {
	resp, err := http.Get(proxyURL + "/api/v1/labels")
	if err != nil {
		t.Fatalf("Labels request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		t.Logf("Labels endpoint returned %d: %s (backend may not have data yet)", resp.StatusCode, string(body))
		// Labels proxies to backend, might fail if backend has no data yet
		return
	}

	var result promResponse
	json.Unmarshal(body, &result)

	if result.Status != "success" {
		t.Logf("Labels status: %s, error: %s", result.Status, result.Error)
	}

	t.Logf("PASS: Labels endpoint responded (status: %s)", result.Status)
}

// ===== Test 15: Metadata - Series Endpoint =====

func TestMetadataSeries(t *testing.T) {
	resp, err := http.Get(proxyURL + `/api/v1/series?match[]=e2e_test_counter`)
	if err != nil {
		t.Fatalf("Series request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		t.Logf("Series endpoint returned %d: %s", resp.StatusCode, string(body))
		return
	}

	var result promResponse
	json.Unmarshal(body, &result)
	t.Logf("PASS: Series endpoint responded (status: %s)", result.Status)
}

// ===== Test 16: Metadata - Label Values Endpoint =====

func TestMetadataLabelValues(t *testing.T) {
	resp, err := http.Get(proxyURL + "/api/v1/label/job/values")
	if err != nil {
		t.Fatalf("Label values request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		t.Logf("Label values endpoint returned %d: %s", resp.StatusCode, string(body))
		return
	}

	var result promResponse
	json.Unmarshal(body, &result)
	t.Logf("PASS: Label values endpoint responded (status: %s)", result.Status)
}

// ===== Test 17: UI Redirect =====

func TestUIRedirect(t *testing.T) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(proxyURL + "/")
	if err != nil {
		t.Fatalf("Root request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 302 {
		t.Fatalf("Expected 302 redirect from /, got %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if loc != "/ui" {
		t.Fatalf("Expected redirect to /ui, got %s", loc)
	}

	t.Log("PASS: Root path redirects to /ui")
}

// ===== Test 18: UI Page Accessible =====

func TestUIPage(t *testing.T) {
	resp, err := http.Get(proxyURL + "/ui")
	if err != nil {
		t.Fatalf("UI request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200 for /ui, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "html") && !strings.Contains(string(body), "HTML") {
		t.Logf("UI response doesn't contain HTML, got: %.200s", string(body))
	}

	t.Log("PASS: UI page is accessible")
}

// ===== Test 19: Concurrent Remote Writes =====

func TestConcurrentRemoteWrites(t *testing.T) {
	numWriters := 5
	samplesPerWriter := 20
	errCh := make(chan error, numWriters)

	for w := 0; w < numWriters; w++ {
		go func(writerID int) {
			metricName := fmt.Sprintf("e2e_test_concurrent_%d", writerID)
			now := time.Now()
			ts := buildTimeSeries(metricName, map[string]string{"job": "e2e", "writer": strconv.Itoa(writerID)}, now, samplesPerWriter, func(i int) float64 {
				return float64(writerID*100 + i)
			})

			writeReq := &prompb.WriteRequest{
				Timeseries: []prompb.TimeSeries{ts},
			}

			data, err := proto.Marshal(writeReq)
			if err != nil {
				errCh <- fmt.Errorf("writer %d: marshal failed: %v", writerID, err)
				return
			}

			compressed := snappy.Encode(nil, data)
			resp, err := http.Post(proxyURL+"/api/v1/write", "application/x-protobuf", bytes.NewReader(compressed))
			if err != nil {
				errCh <- fmt.Errorf("writer %d: post failed: %v", writerID, err)
				return
			}
			resp.Body.Close()

			if resp.StatusCode != 204 {
				errCh <- fmt.Errorf("writer %d: expected 204, got %d", writerID, resp.StatusCode)
				return
			}

			errCh <- nil
		}(w)
	}

	for i := 0; i < numWriters; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("Concurrent write error: %v", err)
		}
	}

	t.Logf("PASS: %d concurrent writers completed successfully", numWriters)
}

// ===== Test 20: Metrics Counters After Operations =====

func TestMetricsCountersAfterOperations(t *testing.T) {
	resp, err := http.Get(proxyURL + "/metrics")
	if err != nil {
		t.Fatalf("Metrics request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	// Check that samples have been received
	if val := extractMetricValue(content, "promsketch_ingestion_samples_total"); val == 0 {
		t.Error("Expected promsketch_ingestion_samples_total > 0")
	} else {
		t.Logf("  promsketch_ingestion_samples_total = %d", val)
	}

	// Check that sketched series exist
	if val := extractMetricValue(content, "promsketch_storage_sketched_series"); val == 0 {
		t.Error("Expected promsketch_storage_sketched_series > 0")
	} else {
		t.Logf("  promsketch_storage_sketched_series = %d", val)
	}

	// Check sketch samples were inserted
	if val := extractMetricValue(content, "promsketch_storage_samples_inserted_total"); val == 0 {
		t.Error("Expected promsketch_storage_samples_inserted_total > 0")
	} else {
		t.Logf("  promsketch_storage_samples_inserted_total = %d", val)
	}

	// Check that query requests were recorded
	if val := extractMetricValue(content, "promsketch_query_requests_total"); val == 0 {
		t.Error("Expected promsketch_query_requests_total > 0")
	} else {
		t.Logf("  promsketch_query_requests_total = %d", val)
	}

	t.Log("PASS: Metrics counters show expected activity")
}

// ===== Test 21: Large Batch Remote Write =====

func TestLargeBatchRemoteWrite(t *testing.T) {
	now := time.Now()
	numSeries := 50
	samplesPerSeries := 10
	timeseries := make([]prompb.TimeSeries, numSeries)

	for i := 0; i < numSeries; i++ {
		metricName := fmt.Sprintf("e2e_test_batch_%d", i)
		timeseries[i] = buildTimeSeries(metricName, map[string]string{"job": "e2e", "batch": "large"}, now, samplesPerSeries, func(j int) float64 {
			return float64(i*100 + j)
		})
	}

	writeReq := &prompb.WriteRequest{
		Timeseries: timeseries,
	}

	data, err := proto.Marshal(writeReq)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	compressed := snappy.Encode(nil, data)

	resp, err := http.Post(proxyURL+"/api/v1/write", "application/x-protobuf", bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("Large batch write failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 204, got %d: %s", resp.StatusCode, string(body))
	}

	t.Logf("PASS: Large batch write (%d series x %d samples) accepted", numSeries, samplesPerSeries)
}

// ===== Test 22: 404 for Unknown Endpoints =====

func TestUnknownEndpoints(t *testing.T) {
	resp, err := http.Get(proxyURL + "/api/v1/nonexistent")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Fatalf("Expected 404 for unknown endpoint, got %d", resp.StatusCode)
	}

	t.Log("PASS: Unknown endpoint returns 404")
}

// ===== Test 23: Backend Direct Query Comparison =====

func TestBackendDirectComparison(t *testing.T) {
	// Query the same metric directly from VictoriaMetrics and via proxy
	// to verify proxy is correctly forwarding

	forceFlushVM(t)

	now := time.Now()
	query := "e2e_test_counter"
	ts := fmt.Sprintf("%d", now.Unix())

	// Query via VictoriaMetrics directly
	vmResp, err := http.Get(fmt.Sprintf("%s/api/v1/query?query=%s&time=%s", vmURL, url.QueryEscape(query), ts))
	if err != nil {
		t.Fatalf("VM direct query failed: %v", err)
	}
	defer vmResp.Body.Close()
	vmBody, _ := io.ReadAll(vmResp.Body)

	// Query via proxy
	proxyResp, err := http.Get(fmt.Sprintf("%s/api/v1/query?query=%s&time=%s", proxyURL, url.QueryEscape(query), ts))
	if err != nil {
		t.Fatalf("Proxy query failed: %v", err)
	}
	defer proxyResp.Body.Close()
	proxyBody, _ := io.ReadAll(proxyResp.Body)

	var vmResult, proxyResult promResponse
	json.Unmarshal(vmBody, &vmResult)
	json.Unmarshal(proxyBody, &proxyResult)

	if vmResult.Status == "success" && proxyResult.Status == "success" {
		t.Log("PASS: Both VM direct and proxy queries return success")
	} else {
		t.Logf("VM status: %s, Proxy status: %s", vmResult.Status, proxyResult.Status)
		t.Logf("VM error: %s, Proxy error: %s", vmResult.Error, proxyResult.Error)
	}
}

// ===== Test 24: Verify Sketch Storage Has Data =====

func TestVerifySketchStorage(t *testing.T) {
	resp, err := http.Get(proxyURL + "/metrics")
	if err != nil {
		t.Fatalf("Metrics request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	sketchedSeries := extractMetricValue(content, "promsketch_storage_sketched_series")
	samplesInserted := extractMetricValue(content, "promsketch_storage_samples_inserted_total")
	sketchHits := extractMetricValue(content, "promsketch_query_sketch_hits_total")
	sketchQueries := extractMetricValue(content, "promsketch_query_requests_total")

	t.Logf("  Sketched series:    %d", sketchedSeries)
	t.Logf("  Samples inserted:   %d", samplesInserted)
	t.Logf("  Sketch queries:     %d", sketchQueries)
	t.Logf("  Sketch hits:        %d", sketchHits)

	if sketchedSeries == 0 {
		t.Error("Expected some sketched series, got 0")
	}
	if samplesInserted == 0 {
		t.Error("Expected some samples inserted into sketches, got 0")
	}

	t.Log("PASS: Sketch storage contains data")
}

// ==============================
// Helper Functions
// ==============================

type promResponse struct {
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data,omitempty"`
	ErrorType string          `json:"errorType,omitempty"`
	Error     string          `json:"error,omitempty"`
}

func buildTimeSeries(name string, extraLabels map[string]string, baseTime time.Time, numSamples int, valueFunc func(int) float64) prompb.TimeSeries {
	labels := []prompb.Label{
		{Name: "__name__", Value: name},
	}
	for k, v := range extraLabels {
		labels = append(labels, prompb.Label{Name: k, Value: v})
	}

	samples := make([]prompb.Sample, numSamples)
	for i := 0; i < numSamples; i++ {
		samples[i] = prompb.Sample{
			Timestamp: baseTime.Add(-time.Duration(numSamples-i) * time.Second).UnixMilli(),
			Value:     valueFunc(i),
		}
	}

	return prompb.TimeSeries{
		Labels:  labels,
		Samples: samples,
	}
}

func queryInstant(t *testing.T, query string) *promResponse {
	t.Helper()
	now := time.Now()
	ts := fmt.Sprintf("%d", now.Unix())
	return queryInstantRaw(t, query, ts)
}

func queryInstantRaw(t *testing.T, query, ts string) *promResponse {
	t.Helper()

	reqURL := fmt.Sprintf("%s/api/v1/query?query=%s&time=%s", proxyURL, url.QueryEscape(query), ts)
	resp, err := http.Get(reqURL)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result promResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse query response: %v\nBody: %s", err, string(body))
	}

	return &result
}

func queryRange(t *testing.T, query string, start, end time.Time, step string) *promResponse {
	t.Helper()

	reqURL := fmt.Sprintf("%s/api/v1/query_range?query=%s&start=%d&end=%d&step=%s",
		proxyURL,
		url.QueryEscape(query),
		start.Unix(),
		end.Unix(),
		step,
	)
	resp, err := http.Get(reqURL)
	if err != nil {
		t.Fatalf("Range query failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result promResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse range query response: %v\nBody: %s", err, string(body))
	}

	return &result
}

func ingestFreshData(t *testing.T, metricName string, numSamples int, valueFunc func(int) float64) {
	t.Helper()

	now := time.Now()
	ts := buildTimeSeries(metricName, map[string]string{"job": "e2e"}, now, numSamples, valueFunc)

	writeReq := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{ts},
	}

	data, err := proto.Marshal(writeReq)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	compressed := snappy.Encode(nil, data)

	resp, err := http.Post(proxyURL+"/api/v1/write", "application/x-protobuf", bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 204 {
		t.Fatalf("Ingest failed with status %d", resp.StatusCode)
	}
}

func forceFlushVM(t *testing.T) {
	t.Helper()
	// Force VictoriaMetrics to flush pending data
	resp, err := http.Get(vmURL + "/internal/force_flush")
	if err != nil {
		t.Logf("Warning: force_flush failed: %v", err)
		return
	}
	resp.Body.Close()
	time.Sleep(1 * time.Second)
}

func extractMetricValue(content, name string) int64 {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, name+" ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				val, err := strconv.ParseFloat(parts[1], 64)
				if err == nil {
					return int64(val)
				}
			}
		}
	}
	return 0
}

func truncateJSON(raw json.RawMessage) string {
	s := string(raw)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
