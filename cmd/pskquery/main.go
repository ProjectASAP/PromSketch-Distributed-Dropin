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

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/promsketch/promsketch-dropin/internal/backendfactory"
	"github.com/promsketch/promsketch-dropin/internal/metrics"
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

	metrics.SetBuildInfo(version, gitCommit, buildDate, "pskquery")
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

	// Metrics endpoint (Prometheus client_golang)
	mux.Handle("/metrics", promhttp.Handler())

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
		// Convert sketch series results to Prometheus format
		if seriesResults, ok := result.Data.([]merger.SketchSeriesResult); ok {
			resultItems := make([]map[string]interface{}, 0)
			for _, sr := range seriesResults {
				for _, s := range sr.Samples {
					ts := float64(s.T) / 1000.0
					if ts == 0 && result.QueryInfo != nil {
						// Use query timestamp for instant queries where sketch returns T=0
						ts = float64(time.Now().UnixMilli()) / 1000.0
					}
					resultItems = append(resultItems, map[string]interface{}{
						"metric": sr.Labels,
						"value":  []interface{}{ts, fmt.Sprintf("%f", s.F)},
					})
				}
			}

			data = map[string]interface{}{
				"resultType": "vector",
				"result":     resultItems,
			}
		} else {
			// Range query results or legacy format
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
