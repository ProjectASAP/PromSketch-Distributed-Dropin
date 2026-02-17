package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/promsketch/promsketch-dropin/internal/backendfactory"
	"github.com/promsketch/promsketch-dropin/internal/pskinsert/client"
	pskconfig "github.com/promsketch/promsketch-dropin/internal/pskquery/config"
	"github.com/promsketch/promsketch-dropin/internal/pskquery/merger"
	"github.com/promsketch/promsketch-dropin/internal/query/api"
	"github.com/promsketch/promsketch-dropin/internal/query/capabilities"
	"github.com/promsketch/promsketch-dropin/internal/query/parser"
)

var (
	version   = "dev"
	gitCommit = "unknown"
	buildDate = "unknown"
)

func main() {
	configFile := flag.String("config.file", "pskquery.yaml", "Path to configuration file")
	// Note: "version" flag may already be registered by VictoriaMetrics lib/buildinfo
	if flag.CommandLine.Lookup("version") == nil {
		flag.Bool("version", false, "Show version information")
	}
	flag.Parse()

	if f := flag.CommandLine.Lookup("version"); f != nil && f.Value.String() == "true" {
		fmt.Printf("pskquery\n")
		fmt.Printf("  version:    %s\n", version)
		fmt.Printf("  git commit: %s\n", gitCommit)
		fmt.Printf("  build date: %s\n", buildDate)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := pskconfig.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting pskquery...")
	log.Printf("  Listen address: %s", cfg.Server.ListenAddress)
	log.Printf("  Sketch nodes: %d", len(cfg.Cluster.Discovery.StaticNodes))
	log.Printf("  Backend: %s (%s)", cfg.Backend.Type, cfg.Backend.URL)
	log.Printf("  Fallback enabled: %v", cfg.Query.EnableFallback)

	// Build node list from config
	nodes := cfg.Cluster.ToHashNodes()

	// Initialize gRPC client pool to psksketch nodes
	pool, err := client.NewPool(nodes)
	if err != nil {
		log.Fatalf("Failed to create client pool: %v", err)
	}
	defer pool.Close()

	// Initialize backend client (for fallback queries)
	backendClient, err := backendfactory.NewBackend(&cfg.Backend)
	if err != nil {
		log.Fatalf("Failed to create backend client: %v", err)
	}

	// Initialize query components (reuse from monolithic version)
	queryParser := parser.NewParser()
	capRegistry := capabilities.NewRegistry()

	// Initialize merger
	queryMerger := merger.NewMerger(
		pool,
		backendClient,
		capRegistry,
		queryParser,
		cfg.Query.QueryTimeout,
		cfg.Query.MaxConcurrentQueries,
	)

	// Setup HTTP server
	mux := http.NewServeMux()

	// Instant query endpoint
	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
		query := r.FormValue("query")
		if query == "" {
			sendError(w, http.StatusBadRequest, "bad_data", "query parameter is required")
			return
		}

		var ts time.Time
		timeParam := r.FormValue("time")
		if timeParam == "" {
			ts = time.Now()
		} else {
			var parseErr error
			ts, parseErr = parseTimestamp(timeParam)
			if parseErr != nil {
				sendError(w, http.StatusBadRequest, "bad_data", "invalid time parameter")
				return
			}
		}

		result, err := queryMerger.Query(r.Context(), query, ts)
		if err != nil {
			sendError(w, http.StatusInternalServerError, "execution", err.Error())
			return
		}

		sendQueryResult(w, result)
	})

	// Range query endpoint
	mux.HandleFunc("/api/v1/query_range", func(w http.ResponseWriter, r *http.Request) {
		query := r.FormValue("query")
		if query == "" {
			sendError(w, http.StatusBadRequest, "bad_data", "query parameter is required")
			return
		}

		startParam := r.FormValue("start")
		endParam := r.FormValue("end")
		stepParam := r.FormValue("step")

		if startParam == "" || endParam == "" || stepParam == "" {
			sendError(w, http.StatusBadRequest, "bad_data", "start, end, and step parameters are required")
			return
		}

		start, err := parseTimestamp(startParam)
		if err != nil {
			sendError(w, http.StatusBadRequest, "bad_data", "invalid start parameter")
			return
		}

		end, err := parseTimestamp(endParam)
		if err != nil {
			sendError(w, http.StatusBadRequest, "bad_data", "invalid end parameter")
			return
		}

		step, err := parseDuration(stepParam)
		if err != nil {
			sendError(w, http.StatusBadRequest, "bad_data", "invalid step parameter")
			return
		}

		result, err := queryMerger.QueryRange(r.Context(), query, start, end, step)
		if err != nil {
			sendError(w, http.StatusInternalServerError, "execution", err.Error())
			return
		}

		sendQueryResult(w, result)
	})

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"component": "pskquery",
		})
	})

	// Metadata endpoints (proxied to backend for Grafana autocomplete)
	metadataAPI := api.NewMetadataAPI(cfg.Backend.URL)
	mux.HandleFunc("/api/v1/series", metadataAPI.ServeHTTP)
	mux.HandleFunc("/api/v1/labels", metadataAPI.ServeHTTP)
	mux.HandleFunc("/api/v1/label/", metadataAPI.ServeHTTP)

	// Metrics endpoint (Prometheus text exposition format)
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		m := queryMerger.Metrics()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprintf(w, "# HELP pskquery_queries_total Total number of queries processed.\n")
		fmt.Fprintf(w, "# TYPE pskquery_queries_total counter\n")
		fmt.Fprintf(w, "pskquery_queries_total{source=\"sketch\"} %d\n", m.SketchQueries)
		fmt.Fprintf(w, "pskquery_queries_total{source=\"backend\"} %d\n", m.BackendQueries)
		fmt.Fprintf(w, "# HELP pskquery_sketch_hits_total Queries answered by sketches.\n")
		fmt.Fprintf(w, "# TYPE pskquery_sketch_hits_total counter\n")
		fmt.Fprintf(w, "pskquery_sketch_hits_total %d\n", m.SketchHits)
		fmt.Fprintf(w, "# HELP pskquery_sketch_misses_total Queries that fell back to backend.\n")
		fmt.Fprintf(w, "# TYPE pskquery_sketch_misses_total counter\n")
		fmt.Fprintf(w, "pskquery_sketch_misses_total %d\n", m.SketchMisses)
		fmt.Fprintf(w, "# HELP pskquery_merge_errors_total Merge errors encountered.\n")
		fmt.Fprintf(w, "# TYPE pskquery_merge_errors_total counter\n")
		fmt.Fprintf(w, "pskquery_merge_errors_total %d\n", m.MergeErrors)
		fmt.Fprintf(w, "# HELP pskquery_query_duration_seconds_total Total time spent processing queries.\n")
		fmt.Fprintf(w, "# TYPE pskquery_query_duration_seconds_total counter\n")
		fmt.Fprintf(w, "pskquery_query_duration_seconds_total{type=\"instant\"} %f\n", float64(m.InstantQueryDurationUs)/1e6)
		fmt.Fprintf(w, "pskquery_query_duration_seconds_total{type=\"range\"} %f\n", float64(m.RangeQueryDurationUs)/1e6)
		fmt.Fprintf(w, "# HELP pskquery_query_count_total Total number of queries by type.\n")
		fmt.Fprintf(w, "# TYPE pskquery_query_count_total counter\n")
		fmt.Fprintf(w, "pskquery_query_count_total{type=\"instant\"} %d\n", m.InstantQueryCount)
		fmt.Fprintf(w, "pskquery_query_count_total{type=\"range\"} %d\n", m.RangeQueryCount)
		// Quantile summary for query latency (pskquery end-to-end)
		quantiles := []float64{0.5, 0.9, 0.99}
		fmt.Fprintf(w, "# HELP pskquery_query_duration_seconds Query latency quantiles over recent observations.\n")
		fmt.Fprintf(w, "# TYPE pskquery_query_duration_seconds summary\n")
		for _, qtype := range []string{"instant", "range"} {
			vals := queryMerger.LatencyQuantiles(qtype, quantiles)
			for i, q := range quantiles {
				fmt.Fprintf(w, "pskquery_query_duration_seconds{type=\"%s\",quantile=\"%.2f\"} %g\n", qtype, q, vals[i])
			}
			if qtype == "instant" {
				fmt.Fprintf(w, "pskquery_query_duration_seconds_sum{type=\"%s\"} %f\n", qtype, float64(m.InstantQueryDurationUs)/1e6)
				fmt.Fprintf(w, "pskquery_query_duration_seconds_count{type=\"%s\"} %d\n", qtype, m.InstantQueryCount)
			} else {
				fmt.Fprintf(w, "pskquery_query_duration_seconds_sum{type=\"%s\"} %f\n", qtype, float64(m.RangeQueryDurationUs)/1e6)
				fmt.Fprintf(w, "pskquery_query_duration_seconds_count{type=\"%s\"} %d\n", qtype, m.RangeQueryCount)
			}
		}
		// Backend (VictoriaMetrics) latency as measured from pskquery
		fmt.Fprintf(w, "# HELP pskquery_backend_duration_seconds Backend query latency as observed by pskquery.\n")
		fmt.Fprintf(w, "# TYPE pskquery_backend_duration_seconds summary\n")
		bvals := queryMerger.LatencyQuantiles("backend", quantiles)
		for i, q := range quantiles {
			fmt.Fprintf(w, "pskquery_backend_duration_seconds{quantile=\"%.2f\"} %g\n", q, bvals[i])
		}
		fmt.Fprintf(w, "pskquery_backend_duration_seconds_sum %f\n", float64(m.BackendQueryDurationUs)/1e6)
		fmt.Fprintf(w, "pskquery_backend_duration_seconds_count %d\n", m.BackendQueryCount)
	})

	httpServer := &http.Server{
		Addr:         cfg.Server.ListenAddress,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		log.Printf("pskquery HTTP server listening on %s", cfg.Server.ListenAddress)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("Received signal %s, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	pool.Close()
	log.Printf("pskquery shutdown complete")
}

type prometheusResponse struct {
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	ErrorType string      `json:"errorType,omitempty"`
	Error     string      `json:"error,omitempty"`
}

func sendError(w http.ResponseWriter, statusCode int, errorType, errorMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(prometheusResponse{
		Status:    "error",
		ErrorType: errorType,
		Error:     errorMsg,
	})
}

func sendQueryResult(w http.ResponseWriter, result *merger.QueryResult) {
	var data interface{}

	if result.Source == "sketch" {
		// Convert sketch samples to Prometheus format
		if samples, ok := result.Data.([]merger.SketchSample); ok {
			// Build label map from query
			labels := make(map[string]string)
			if result.QueryInfo != nil && result.QueryInfo.MetricName != "" {
				labels["__name__"] = result.QueryInfo.MetricName
			}

			resultItems := make([]map[string]interface{}, 0)
			for _, s := range samples {
				resultItems = append(resultItems, map[string]interface{}{
					"metric": labels,
					"value":  []interface{}{float64(s.T) / 1000.0, fmt.Sprintf("%f", s.F)},
				})
			}

			data = map[string]interface{}{
				"resultType": "vector",
				"result":     resultItems,
			}
		} else {
			// Range query results
			data = result.Data
		}
	} else {
		// Backend results are already in Prometheus format
		data = result.Data
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prometheusResponse{
		Status: "success",
		Data:   data,
	})
}

func parseTimestamp(s string) (time.Time, error) {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		sec := int64(f)
		nsec := int64((f - float64(sec)) * 1e9)
		return time.Unix(sec, nsec), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse %q as timestamp", s)
}

func parseDuration(s string) (time.Duration, error) {
	if num, err := strconv.ParseFloat(s, 64); err == nil {
		return time.Duration(num * float64(time.Second)), nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	return 0, fmt.Errorf("invalid duration: %s", s)
}
