package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"github.com/promsketch/promsketch-dropin/internal/backend"
	"github.com/promsketch/promsketch-dropin/internal/backend/prometheus"
	"github.com/promsketch/promsketch-dropin/internal/backend/victoriametrics"
	"github.com/promsketch/promsketch-dropin/internal/config"
	"github.com/spf13/cobra"
)

// BackfillCheckpoint tracks backfill progress for resume capability
type BackfillCheckpoint struct {
	SourceType   string              `json:"source_type"`
	SourceURL    string              `json:"source_url"`
	TargetURL    string              `json:"target_url"`
	StartTime    time.Time           `json:"start_time"`
	EndTime      time.Time           `json:"end_time"`
	MatchPattern string              `json:"match_pattern"`
	Series       []map[string]string `json:"series"`
	LastChunkEnd time.Time           `json:"last_chunk_end"`
	TotalSamples int                 `json:"total_samples"`
	TotalErrors  int                 `json:"total_errors"`
	ChunksCompleted int              `json:"chunks_completed"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

func newBackfillCmd() *cobra.Command {
	var (
		sourceType     string
		sourceURL      string
		targetURL      string
		startTime      string
		endTime        string
		matchPattern   string
		dryRun         bool
		silent         bool
		resume         bool
		checkpointFile string
	)

	cmd := &cobra.Command{
		Use:   "backfill",
		Short: "Backfill historical data into PromSketch-Dropin",
		Long: `Backfill reads historical data from a backend (VictoriaMetrics, Prometheus, etc.)
and replays it into PromSketch-Dropin's insertion pipeline.

This is useful for populating sketch data from existing metrics.`,
		Example: `  # Backfill from VictoriaMetrics
  pskctl backfill --source-type victoriametrics \
    --source-url http://victoria:8428 \
    --target http://localhost:9100 \
    --start "2025-01-01T00:00:00Z" \
    --end "2025-02-01T00:00:00Z"

  # Backfill specific metrics
  pskctl backfill --source-type prometheus \
    --source-url http://prometheus:9090 \
    --target http://localhost:9100 \
    --match '{job="api"}' \
    --start "2025-01-01T00:00:00Z" \
    --end "2025-02-01T00:00:00Z"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var start, end time.Time
			var err error
			var checkpoint *BackfillCheckpoint

			// Set default checkpoint file if not specified
			if checkpointFile == "" {
				checkpointFile = "backfill-checkpoint.json"
			}

			// Handle resume mode
			if resume {
				checkpoint, err = loadCheckpoint(checkpointFile)
				if err != nil {
					return fmt.Errorf("failed to load checkpoint: %w", err)
				}

				// Restore configuration from checkpoint
				sourceType = checkpoint.SourceType
				sourceURL = checkpoint.SourceURL
				targetURL = checkpoint.TargetURL
				start = checkpoint.LastChunkEnd // Resume from last completed chunk
				end = checkpoint.EndTime
				matchPattern = checkpoint.MatchPattern

				if !silent {
					fmt.Printf("📂 Resuming backfill from checkpoint\n")
					fmt.Printf("  Last chunk completed: %s\n", checkpoint.LastChunkEnd.Format(time.RFC3339))
					fmt.Printf("  Progress: %d samples, %d chunks, %d errors\n\n",
						checkpoint.TotalSamples, checkpoint.ChunksCompleted, checkpoint.TotalErrors)
				}
			} else {
				// Parse time range for new backfill
				start, err = time.Parse(time.RFC3339, startTime)
				if err != nil {
					return fmt.Errorf("invalid start time: %w", err)
				}

				end, err = time.Parse(time.RFC3339, endTime)
				if err != nil {
					return fmt.Errorf("invalid end time: %w", err)
				}

				// Create new checkpoint
				checkpoint = &BackfillCheckpoint{
					SourceType:   sourceType,
					SourceURL:    sourceURL,
					TargetURL:    targetURL,
					StartTime:    start,
					EndTime:      end,
					MatchPattern: matchPattern,
					LastChunkEnd: start,
					CreatedAt:    time.Now(),
				}
			}

			if !silent {
				fmt.Printf("Backfill Configuration:\n")
				fmt.Printf("  Source Type:    %s\n", sourceType)
				fmt.Printf("  Source URL:     %s\n", sourceURL)
				fmt.Printf("  Target URL:     %s\n", targetURL)
				fmt.Printf("  Time Range:     %s to %s\n", start.Format(time.RFC3339), end.Format(time.RFC3339))
				if matchPattern != "" {
					fmt.Printf("  Match Pattern:  %s\n", matchPattern)
				}
				fmt.Printf("  Checkpoint File: %s\n", checkpointFile)
				fmt.Printf("  Dry Run:        %v\n\n", dryRun)
			}

			if dryRun {
				fmt.Printf("Dry run - no data will be written\n")
				return nil
			}

			// Create source backend client
			var sourceBackend backend.Backend
			backendCfg := &config.BackendConfig{
				URL:     sourceURL,
				Timeout: 30 * time.Second,
			}

			switch sourceType {
			case "victoriametrics":
				sourceBackend, err = victoriametrics.NewClient(backendCfg)
			case "prometheus":
				sourceBackend, err = prometheus.NewClient(backendCfg)
			default:
				return fmt.Errorf("unsupported source type: %s (supported: victoriametrics, prometheus)", sourceType)
			}

			if err != nil {
				return fmt.Errorf("failed to create source backend: %w", err)
			}

			// Execute backfill with checkpoint
			ctx := context.Background()
			return executeBackfillWithCheckpoint(ctx, sourceBackend, checkpoint, checkpointFile, silent)
		},
	}

	cmd.Flags().StringVar(&sourceType, "source-type", "", "Source backend type (victoriametrics, prometheus)")
	cmd.Flags().StringVar(&sourceURL, "source-url", "", "Source backend URL")
	cmd.Flags().StringVar(&targetURL, "target", "", "PromSketch-Dropin URL")
	cmd.Flags().StringVar(&startTime, "start", "", "Start time (RFC3339 format)")
	cmd.Flags().StringVar(&endTime, "end", "", "End time (RFC3339 format)")
	cmd.Flags().StringVar(&matchPattern, "match", "", "Metric selector pattern")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would be backfilled without writing")
	cmd.Flags().BoolVarP(&silent, "silent", "s", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume from checkpoint file")
	cmd.Flags().StringVar(&checkpointFile, "checkpoint-file", "backfill-checkpoint.json", "Checkpoint file path")

	// Make flags conditionally required (not required when resuming)
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if !resume {
			// These flags are required for new backfills
			if sourceType == "" {
				return fmt.Errorf("--source-type is required")
			}
			if sourceURL == "" {
				return fmt.Errorf("--source-url is required")
			}
			if targetURL == "" {
				return fmt.Errorf("--target is required")
			}
			if startTime == "" {
				return fmt.Errorf("--start is required")
			}
			if endTime == "" {
				return fmt.Errorf("--end is required")
			}
		}
		return nil
	}

	return cmd
}

// executeBackfill performs the actual backfill operation
func executeBackfill(ctx context.Context, sourceBackend backend.Backend, sourceURL, targetURL string, start, end time.Time, matchPattern string, silent bool) error {
	// 1. Query source for all matching series
	if !silent {
		fmt.Printf("Step 1: Discovering metrics...\n")
	}

	series, err := discoverSeries(ctx, sourceURL, matchPattern, start, end)
	if err != nil {
		return fmt.Errorf("failed to discover series: %w", err)
	}

	if len(series) == 0 {
		fmt.Printf("⚠️  No metrics found matching pattern\n")
		return nil
	}

	if !silent {
		fmt.Printf("Found %d time series to backfill\n\n", len(series))
	}

	// 2. Chunk the time range
	chunkDuration := 1 * time.Hour
	totalChunks := int(end.Sub(start) / chunkDuration)
	if end.Sub(start)%chunkDuration != 0 {
		totalChunks++
	}

	if !silent {
		fmt.Printf("Step 2: Backfilling data in %d chunks (%s each)...\n", totalChunks, chunkDuration)
	}

	// 3. Process each chunk
	totalSamples := 0
	totalErrors := 0
	chunkNum := 0

	for current := start; current.Before(end); current = current.Add(chunkDuration) {
		chunkNum++
		chunkEnd := current.Add(chunkDuration)
		if chunkEnd.After(end) {
			chunkEnd = end
		}

		// Query data for this chunk
		samples, err := backfillChunk(ctx, sourceBackend, targetURL, series, current, chunkEnd)
		if err != nil {
			totalErrors++
			fmt.Printf("  [%d/%d] Error: %v\n", chunkNum, totalChunks, err)
			continue
		}

		totalSamples += samples
		if !silent {
			fmt.Printf("  [%d/%d] %s to %s: %d samples\n", 
				chunkNum, totalChunks, 
				current.Format("2006-01-02 15:04"), 
				chunkEnd.Format("2006-01-02 15:04"), 
				samples)
		}
	}

	// 4. Summary
	fmt.Printf("\n✅ Backfill completed\n")
	fmt.Printf("  Total samples:  %d\n", totalSamples)
	fmt.Printf("  Total series:   %d\n", len(series))
	fmt.Printf("  Time range:     %s to %s\n", start.Format(time.RFC3339), end.Format(time.RFC3339))
	if totalErrors > 0 {
		fmt.Printf("  Errors:         %d chunks failed\n", totalErrors)
	}

	return nil
}

// discoverSeries queries the backend for all series matching the pattern
func discoverSeries(ctx context.Context, sourceURL, matchPattern string, start, end time.Time) ([]map[string]string, error) {
	// Build series query URL
	query := matchPattern
	if query == "" {
		query = "{__name__=~\".+\"}" // Match all series
	}

	u, err := url.Parse(sourceURL)
	if err != nil {
		return nil, err
	}

	u.Path = "/api/v1/series"
	q := u.Query()
	q.Set("match[]", query)
	q.Set("start", fmt.Sprintf("%d", start.Unix()))
	q.Set("end", fmt.Sprintf("%d", end.Unix()))
	u.RawQuery = q.Encode()

	// Query series
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("series query failed: %d - %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		Status string `json:"status"`
		Data   []map[string]string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("series query returned status: %s", result.Status)
	}

	return result.Data, nil
}

// backfillChunk backfills data for a specific time chunk
func backfillChunk(ctx context.Context, sourceBackend backend.Backend, targetURL string, series []map[string]string, start, end time.Time) (int, error) {
	totalSamples := 0

	// Query range data for each series
	for _, labels := range series {
		// Build query for this series
		query := buildQueryFromLabels(labels)

		// Query range
		step := 15 * time.Second // Standard scrape interval
		result, err := sourceBackend.QueryRange(ctx, query, start, end, step)
		if err != nil {
			// Skip this series on error, don't fail entire chunk
			continue
		}

		// Convert to remote write format and send
		samples, err := convertAndSendToTarget(result, labels, targetURL)
		if err != nil {
			continue
		}

		totalSamples += samples
	}

	return totalSamples, nil
}

// buildQueryFromLabels builds a PromQL query from label map
func buildQueryFromLabels(labels map[string]string) string {
	metricName := labels["__name__"]
	if metricName == "" {
		metricName = "up" // Fallback
	}

	// For instant value query, just use metric name
	// The range is specified in QueryRange call
	return metricName
}

// convertAndSendToTarget converts backend result to remote write and sends to target
func convertAndSendToTarget(result *backend.QueryResult, labels map[string]string, targetURL string) (int, error) {
	// Parse the result data
	// Backend returns data in Prometheus format
	data, ok := result.Result.([]interface{})
	if !ok {
		return 0, fmt.Errorf("unexpected result format")
	}

	if len(data) == 0 {
		return 0, nil
	}

	// Convert to remote write format
	timeSeries := make([]prompb.TimeSeries, 0)

	for _, item := range data {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract metric labels
		metricLabels, ok := itemMap["metric"].(map[string]interface{})
		if !ok {
			continue
		}

		// Build label set
		lbls := make([]prompb.Label, 0)
		for k, v := range metricLabels {
			if vStr, ok := v.(string); ok {
				lbls = append(lbls, prompb.Label{
					Name:  k,
					Value: vStr,
				})
			}
		}

		// Extract values
		values, ok := itemMap["values"].([]interface{})
		if !ok {
			// Try single value
			value, ok := itemMap["value"].([]interface{})
			if ok {
				values = value
			}
		}

		if len(values) == 0 {
			continue
		}

		// Convert values to samples
		samples := make([]prompb.Sample, 0)
		for _, val := range values {
			valArray, ok := val.([]interface{})
			if !ok || len(valArray) != 2 {
				continue
			}

			timestamp, ok := valArray[0].(float64)
			if !ok {
				continue
			}

			var value float64
			switch v := valArray[1].(type) {
			case string:
				fmt.Sscanf(v, "%f", &value)
			case float64:
				value = v
			default:
				continue
			}

			samples = append(samples, prompb.Sample{
				Timestamp: int64(timestamp * 1000), // Convert to milliseconds
				Value:     value,
			})
		}

		if len(samples) > 0 {
			timeSeries = append(timeSeries, prompb.TimeSeries{
				Labels:  lbls,
				Samples: samples,
			})
		}
	}

	if len(timeSeries) == 0 {
		return 0, nil
	}

	// Send to target
	writeReq := &prompb.WriteRequest{
		Timeseries: timeSeries,
	}

	if err := sendRemoteWrite(targetURL, writeReq); err != nil {
		return 0, err
	}

	// Count total samples
	totalSamples := 0
	for _, ts := range timeSeries {
		totalSamples += len(ts.Samples)
	}

	return totalSamples, nil
}

// loadCheckpoint loads a checkpoint from file
func loadCheckpoint(filename string) (*BackfillCheckpoint, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoint file: %w", err)
	}

	var checkpoint BackfillCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to parse checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// saveCheckpoint saves a checkpoint to file
func saveCheckpoint(filename string, checkpoint *BackfillCheckpoint) error {
	checkpoint.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint file: %w", err)
	}

	return nil
}

// executeBackfillWithCheckpoint performs backfill with checkpoint support
func executeBackfillWithCheckpoint(ctx context.Context, sourceBackend backend.Backend, checkpoint *BackfillCheckpoint, checkpointFile string, silent bool) error {
	// 1. Discover series (if not already in checkpoint)
	if len(checkpoint.Series) == 0 {
		if !silent {
			fmt.Printf("Step 1: Discovering metrics...\n")
		}

		series, err := discoverSeries(ctx, checkpoint.SourceURL, checkpoint.MatchPattern, checkpoint.StartTime, checkpoint.EndTime)
		if err != nil {
			return fmt.Errorf("failed to discover series: %w", err)
		}

		if len(series) == 0 {
			fmt.Printf("⚠️  No metrics found matching pattern\n")
			return nil
		}

		checkpoint.Series = series

		if !silent {
			fmt.Printf("Found %d time series to backfill\n\n", len(series))
		}

		// Save initial checkpoint with series list
		if err := saveCheckpoint(checkpointFile, checkpoint); err != nil {
			fmt.Printf("Warning: failed to save checkpoint: %v\n", err)
		}
	} else {
		if !silent {
			fmt.Printf("Step 1: Loaded %d series from checkpoint\n\n", len(checkpoint.Series))
		}
	}

	// 2. Chunk the time range
	chunkDuration := 1 * time.Hour
	totalChunks := int(checkpoint.EndTime.Sub(checkpoint.StartTime) / chunkDuration)
	if checkpoint.EndTime.Sub(checkpoint.StartTime)%chunkDuration != 0 {
		totalChunks++
	}

	if !silent {
		fmt.Printf("Step 2: Backfilling data in %d chunks (%s each)...\n", totalChunks, chunkDuration)
	}

	// 3. Process each chunk (starting from LastChunkEnd)
	chunkNum := checkpoint.ChunksCompleted

	for current := checkpoint.LastChunkEnd; current.Before(checkpoint.EndTime); current = current.Add(chunkDuration) {
		chunkNum++
		chunkEnd := current.Add(chunkDuration)
		if chunkEnd.After(checkpoint.EndTime) {
			chunkEnd = checkpoint.EndTime
		}

		// Query data for this chunk
		samples, err := backfillChunk(ctx, sourceBackend, checkpoint.TargetURL, checkpoint.Series, current, chunkEnd)
		if err != nil {
			checkpoint.TotalErrors++
			fmt.Printf("  [%d/%d] Error: %v\n", chunkNum, totalChunks, err)

			// Save checkpoint even on error
			if err := saveCheckpoint(checkpointFile, checkpoint); err != nil {
				fmt.Printf("Warning: failed to save checkpoint: %v\n", err)
			}
			continue
		}

		checkpoint.TotalSamples += samples
		checkpoint.LastChunkEnd = chunkEnd
		checkpoint.ChunksCompleted = chunkNum

		if !silent {
			fmt.Printf("  [%d/%d] %s to %s: %d samples\n",
				chunkNum, totalChunks,
				current.Format("2006-01-02 15:04"),
				chunkEnd.Format("2006-01-02 15:04"),
				samples)
		}

		// Save checkpoint after each successful chunk
		if err := saveCheckpoint(checkpointFile, checkpoint); err != nil {
			fmt.Printf("Warning: failed to save checkpoint: %v\n", err)
		}
	}

	// 4. Summary
	fmt.Printf("\n✅ Backfill completed\n")
	fmt.Printf("  Total samples:  %d\n", checkpoint.TotalSamples)
	fmt.Printf("  Total series:   %d\n", len(checkpoint.Series))
	fmt.Printf("  Time range:     %s to %s\n", checkpoint.StartTime.Format(time.RFC3339), checkpoint.EndTime.Format(time.RFC3339))
	if checkpoint.TotalErrors > 0 {
		fmt.Printf("  Errors:         %d chunks failed\n", checkpoint.TotalErrors)
	}
	fmt.Printf("  Checkpoint:     %s\n", checkpointFile)

	return nil
}
