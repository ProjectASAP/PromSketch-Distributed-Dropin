package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/spf13/cobra"
)

func newBenchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Run throughput and accuracy benchmarks",
		Long:  "Run benchmarks to test insertion throughput and query accuracy",
	}

	cmd.AddCommand(newBenchInsertCmd())
	cmd.AddCommand(newBenchAccuracyCmd())

	return cmd
}

func newBenchInsertCmd() *cobra.Command {
	var (
		targetURL        string
		numSeries        int
		samplesPerSeries int
		batchSize        int
		concurrency      int
		duration         time.Duration
	)

	cmd := &cobra.Command{
		Use:   "insert",
		Short: "Run insertion throughput benchmark",
		Long: `Generate synthetic metrics and measure insertion throughput.

This benchmark creates random time series and sends them to PromSketch-Dropin
via the remote write API, measuring samples/sec and latency.`,
		Example: `  # Generate 10k series with 1000 samples each
  pskctl bench insert --target http://localhost:9100 \
    --num-series 10000 \
    --samples-per-series 1000 \
    --batch-size 500 \
    --concurrency 8`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Insertion Throughput Benchmark\n")
			fmt.Printf("  Target:             %s\n", targetURL)
			fmt.Printf("  Series:             %d\n", numSeries)
			fmt.Printf("  Samples per series: %d\n", samplesPerSeries)
			fmt.Printf("  Batch size:         %d\n", batchSize)
			fmt.Printf("  Concurrency:        %d\n", concurrency)
			if duration > 0 {
				fmt.Printf("  Duration:           %s\n", duration)
			}
			fmt.Printf("\n")

			// Run benchmark
			start := time.Now()
			totalSamples := 0
			errors := 0

			// Generate and send metrics
			for i := 0; i < numSeries; i++ {
				samples := make([]prompb.Sample, samplesPerSeries)
				baseTime := time.Now().Add(-time.Hour).UnixMilli()

				for j := 0; j < samplesPerSeries; j++ {
					samples[j] = prompb.Sample{
						Value:     rand.Float64() * 100,
						Timestamp: baseTime + int64(j*1000),
					}
				}

				ts := prompb.TimeSeries{
					Labels: []prompb.Label{
						{Name: "__name__", Value: fmt.Sprintf("bench_metric_%d", i)},
						{Name: "instance", Value: fmt.Sprintf("instance_%d", i%10)},
						{Name: "job", Value: "benchmark"},
					},
					Samples: samples,
				}

				// Send in batches
				if (i+1)%batchSize == 0 || i == numSeries-1 {
					req := &prompb.WriteRequest{
						Timeseries: []prompb.TimeSeries{ts},
					}

					if err := sendRemoteWrite(targetURL, req); err != nil {
						errors++
					} else {
						totalSamples += samplesPerSeries
					}
				}
			}

			elapsed := time.Since(start)
			samplesPerSec := float64(totalSamples) / elapsed.Seconds()

			fmt.Printf("\nResults:\n")
			fmt.Printf("  Total samples:    %d\n", totalSamples)
			fmt.Printf("  Duration:         %s\n", elapsed)
			fmt.Printf("  Samples/sec:      %.2f\n", samplesPerSec)
			fmt.Printf("  Errors:           %d\n", errors)

			if errors > 0 {
				fmt.Printf("\n⚠️  Benchmark completed with errors\n")
			} else {
				fmt.Printf("\n✅ Benchmark completed successfully\n")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&targetURL, "target", "http://localhost:9100", "PromSketch-Dropin URL")
	cmd.Flags().IntVar(&numSeries, "num-series", 1000, "Number of time series to generate")
	cmd.Flags().IntVar(&samplesPerSeries, "samples-per-series", 100, "Number of samples per series")
	cmd.Flags().IntVar(&batchSize, "batch-size", 100, "Batch size for remote write")
	cmd.Flags().IntVar(&concurrency, "concurrency", 1, "Number of concurrent writers")
	cmd.Flags().DurationVar(&duration, "duration", 0, "Duration to run benchmark (0 = run once)")

	return cmd
}

func newBenchAccuracyCmd() *cobra.Command {
	var (
		promsketchURL string
		backendURL    string
		queryFile     string
		timeRange     string
		autoGenerate  bool
	)

	cmd := &cobra.Command{
		Use:   "accuracy",
		Short: "Run query accuracy comparison benchmark",
		Long: `Compare PromSketch query results against exact backend results.

This benchmark executes the same queries against both PromSketch and the backend,
then compares the results to measure accuracy (relative error, absolute error).`,
		Example: `  # Compare accuracy using custom queries
  pskctl bench accuracy \
    --promsketch-url http://localhost:9100 \
    --backend-url http://victoria:8428 \
    --queries-file queries.yaml

  # Auto-generate common query patterns
  pskctl bench accuracy \
    --promsketch-url http://localhost:9100 \
    --backend-url http://victoria:8428 \
    --auto-generate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Query Accuracy Benchmark\n")
			fmt.Printf("  PromSketch URL: %s\n", promsketchURL)
			fmt.Printf("  Backend URL:    %s\n", backendURL)
			if queryFile != "" {
				fmt.Printf("  Query file:     %s\n", queryFile)
			}
			if autoGenerate {
				fmt.Printf("  Auto-generate:  enabled\n")
			}
			fmt.Printf("\n")

			// Sample queries for auto-generate mode
			queries := []string{
				"avg_over_time(http_requests_total[5m])",
				"sum_over_time(http_requests_total[5m])",
				"count_over_time(http_requests_total[5m])",
				"quantile_over_time(0.95, http_duration_seconds[5m])",
			}

			if autoGenerate {
				fmt.Printf("Running %d auto-generated queries...\n\n", len(queries))

				var totalAbsError, totalRelError float64
				var successCount int

				for i, query := range queries {
					fmt.Printf("[%d/%d] %s\n", i+1, len(queries), query)

					// Execute query on PromSketch
					promsketchResult, err := executeQuery(promsketchURL, query, time.Now())
					if err != nil {
						fmt.Printf("  PromSketch: ❌ Error: %v\n", err)
						fmt.Printf("\n")
						continue
					}
					fmt.Printf("  PromSketch: ✅ %d results\n", len(promsketchResult))

					// Execute query on backend
					backendResult, err := executeQuery(backendURL, query, time.Now())
					if err != nil {
						fmt.Printf("  Backend:    ❌ Error: %v\n", err)
						fmt.Printf("\n")
						continue
					}
					fmt.Printf("  Backend:    ✅ %d results\n", len(backendResult))

					// Compare results
					absError, relError, err := compareResults(promsketchResult, backendResult)
					if err != nil {
						fmt.Printf("  Accuracy:   ⚠️  %v\n", err)
					} else {
						fmt.Printf("  Accuracy:   Abs Error: %.4f, Rel Error: %.2f%%\n", absError, relError*100)
						totalAbsError += absError
						totalRelError += relError
						successCount++
					}
					fmt.Printf("\n")
				}

				if successCount > 0 {
					avgAbsError := totalAbsError / float64(successCount)
					avgRelError := totalRelError / float64(successCount)
					fmt.Printf("📊 Summary Statistics\n")
					fmt.Printf("  Successful comparisons: %d/%d\n", successCount, len(queries))
					fmt.Printf("  Average absolute error: %.4f\n", avgAbsError)
					fmt.Printf("  Average relative error: %.2f%%\n", avgRelError*100)
					fmt.Printf("\n")
				}
			}

			fmt.Printf("✅ Accuracy benchmark completed\n")

			return nil
		},
	}

	cmd.Flags().StringVar(&promsketchURL, "promsketch-url", "http://localhost:9100", "PromSketch-Dropin URL")
	cmd.Flags().StringVar(&backendURL, "backend-url", "", "Backend URL (VictoriaMetrics/Prometheus)")
	cmd.Flags().StringVar(&queryFile, "queries-file", "", "YAML file with custom queries")
	cmd.Flags().StringVar(&timeRange, "time-range", "", "Time range for queries")
	cmd.Flags().BoolVar(&autoGenerate, "auto-generate", false, "Auto-generate common query patterns")

	return cmd
}

func sendRemoteWrite(targetURL string, req *prompb.WriteRequest) error {
	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	compressed := snappy.Encode(nil, data)

	httpReq, err := http.NewRequest("POST", targetURL+"/api/v1/write", bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("Content-Encoding", "snappy")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	httpReq = httpReq.WithContext(ctx)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// prometheusResponse represents a Prometheus API response
type prometheusResponse struct {
	Status string                 `json:"status"`
	Data   prometheusResponseData `json:"data"`
	Error  string                 `json:"error,omitempty"`
}

type prometheusResponseData struct {
	ResultType string                   `json:"resultType"`
	Result     []prometheusQueryResult  `json:"result"`
}

type prometheusQueryResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"` // [timestamp, value]
}

// executeQuery executes a Prometheus query and returns the numeric results
func executeQuery(baseURL, query string, ts time.Time) ([]float64, error) {
	queryURL := fmt.Sprintf("%s/api/v1/query", baseURL)

	params := url.Values{}
	params.Set("query", query)
	params.Set("time", fmt.Sprintf("%d", ts.Unix()))

	fullURL := fmt.Sprintf("%s?%s", queryURL, params.Encode())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var promResp prometheusResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("query failed: %s", promResp.Error)
	}

	// Extract numeric values from results
	values := make([]float64, 0, len(promResp.Data.Result))
	for _, result := range promResp.Data.Result {
		if len(result.Value) >= 2 {
			// Value is at index 1 (index 0 is timestamp)
			switch v := result.Value[1].(type) {
			case string:
				var val float64
				_, err := fmt.Sscanf(v, "%f", &val)
				if err == nil {
					values = append(values, val)
				}
			case float64:
				values = append(values, v)
			}
		}
	}

	return values, nil
}

// compareResults compares two sets of query results and calculates error metrics
func compareResults(promsketch, backend []float64) (absError, relError float64, err error) {
	if len(promsketch) == 0 || len(backend) == 0 {
		return 0, 0, fmt.Errorf("one or both result sets are empty")
	}

	if len(promsketch) != len(backend) {
		return 0, 0, fmt.Errorf("result count mismatch: PromSketch=%d, Backend=%d", len(promsketch), len(backend))
	}

	var totalAbsError, totalRelError float64
	validComparisons := 0

	for i := 0; i < len(promsketch); i++ {
		ps := promsketch[i]
		be := backend[i]

		// Skip NaN or Inf values
		if math.IsNaN(ps) || math.IsNaN(be) || math.IsInf(ps, 0) || math.IsInf(be, 0) {
			continue
		}

		// Absolute error
		absErr := math.Abs(ps - be)
		totalAbsError += absErr

		// Relative error (avoid division by zero)
		if be != 0 {
			relErr := math.Abs((ps - be) / be)
			totalRelError += relErr
			validComparisons++
		}
	}

	if validComparisons == 0 {
		return 0, 0, fmt.Errorf("no valid comparisons (all values were zero or invalid)")
	}

	avgAbsError := totalAbsError / float64(len(promsketch))
	avgRelError := totalRelError / float64(validComparisons)

	return avgAbsError, avgRelError, nil
}
