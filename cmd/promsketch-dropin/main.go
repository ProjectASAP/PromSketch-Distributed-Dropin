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
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/promsketch/promsketch-dropin/internal/backend"
	"github.com/promsketch/promsketch-dropin/internal/backendfactory"
	"github.com/promsketch/promsketch-dropin/internal/config"
	"github.com/promsketch/promsketch-dropin/internal/ingestion/pipeline"
	"github.com/promsketch/promsketch-dropin/internal/metrics"
	"github.com/promsketch/promsketch-dropin/internal/query/api"
	"github.com/promsketch/promsketch-dropin/internal/query/capabilities"
	"github.com/promsketch/promsketch-dropin/internal/query/parser"
	"github.com/promsketch/promsketch-dropin/internal/query/router"
	"github.com/promsketch/promsketch-dropin/internal/storage"
	"github.com/promsketch/promsketch-dropin/internal/ui"
)

var (
	version   = "dev"
	gitCommit = "unknown"
	buildDate = "unknown"
)

func main() {
	var (
		configFile = flag.String("config.file", "promsketch-dropin.yaml", "Path to configuration file")
	)

	// Reuse existing "version" flag if already registered by a dependency
	showVersion := flag.CommandLine.Lookup("version")
	if showVersion == nil {
		flag.Bool("version", false, "Show version information")
		showVersion = flag.CommandLine.Lookup("version")
	}
	flag.Parse()

	if showVersion != nil && showVersion.Value.String() == "true" {
		fmt.Printf("promsketch-dropin\n")
		fmt.Printf("  version:    %s\n", version)
		fmt.Printf("  git commit: %s\n", gitCommit)
		fmt.Printf("  build date: %s\n", buildDate)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	metrics.SetBuildInfo(version, gitCommit, buildDate, "promsketch-dropin")
	log.Printf("Starting PromSketch-Dropin...")
	log.Printf("  Server listen address: %s", cfg.Server.ListenAddress)
	log.Printf("  Backend type: %s", cfg.Backend.Type)
	log.Printf("  Backend URL: %s", cfg.Backend.URL)
	log.Printf("  Sketch partitions: %d", cfg.Sketch.NumPartitions)
	log.Printf("  Sketch targets: %d", len(cfg.Sketch.Targets))
	log.Printf("  Remote write enabled: %v", cfg.Ingestion.RemoteWrite.Enabled)
	log.Printf("  OTLP enabled: %v", cfg.Ingestion.OTLP.Enabled)
	log.Printf("  Scrape manager enabled: %v", cfg.Ingestion.Scrape.Enabled)

	// 1. Initialize PromSketch storage layer
	log.Printf("Initializing PromSketch storage with %d partitions...", cfg.Sketch.NumPartitions)
	stor, err := storage.NewStorage(&cfg.Sketch)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// 2. Initialize backend client
	log.Printf("Connecting to backend: %s (%s)...", cfg.Backend.URL, cfg.Backend.Type)
	backendClient, err := backendfactory.NewBackend(&cfg.Backend)
	if err != nil {
		log.Fatalf("Failed to create backend client: %v", err)
	}

	// Health check backend
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := backendClient.Health(ctx); err != nil {
		log.Printf("Warning: Backend health check failed: %v", err)
	} else {
		log.Printf("Backend health check: OK")
	}
	cancel()

	// 3. Initialize backend forwarder
	log.Printf("Starting backend forwarder...")
	forwarder := backend.NewForwarder(backendClient, &cfg.Backend)
	if err := forwarder.Start(); err != nil {
		log.Fatalf("Failed to start forwarder: %v", err)
	}

	// 4. Initialize ingestion pipeline
	log.Printf("Initializing ingestion pipeline...")
	pipe, err := pipeline.NewPipeline(cfg, stor, forwarder)
	if err != nil {
		log.Fatalf("Failed to create pipeline: %v", err)
	}

	ctx = context.Background()
	if err := pipe.Start(ctx); err != nil {
		log.Fatalf("Failed to start pipeline: %v", err)
	}

	// 5. Initialize query router
	log.Printf("Initializing query router...")
	queryParser := parser.NewParser()
	queryCap := capabilities.NewRegistry()
	queryRouter := router.NewRouter(stor, backendClient, queryParser, queryCap)

	// 6. Initialize query API
	log.Printf("Initializing query API...")
	queryAPI := api.NewQueryAPIWithApproximation(queryRouter, cfg.Query.Approximation)

	// 7. Initialize metadata API
	log.Printf("Initializing metadata API...")
	metadataAPI := api.NewMetadataAPI(cfg.Backend.URL)

	// 8. Start HTTP server
	mux := http.NewServeMux()

	// Remote write endpoint
	if cfg.Ingestion.RemoteWrite.Enabled {
		handler := pipe.RemoteWriteHandler()
		mux.Handle("/api/v1/write", handler)
		log.Printf("Remote write endpoint enabled at /api/v1/write")
	}

	// OTLP endpoint
	if cfg.Ingestion.OTLP.Enabled {
		mux.Handle("/opentelemetry/v1/metrics", pipe.OTLPHandler())
		log.Printf("OTLP endpoint enabled at /opentelemetry/v1/metrics")
	}

	// Query endpoints
	mux.Handle("/api/v1/query", queryAPI)
	mux.Handle("/api/v1/query_range", queryAPI)
	log.Printf("Query endpoints enabled at /api/v1/query and /api/v1/query_range")

	// Metadata endpoints
	mux.Handle("/api/v1/series", metadataAPI)
	mux.Handle("/api/v1/labels", metadataAPI)
	mux.Handle("/api/v1/label/", metadataAPI)
	log.Printf("Metadata endpoints enabled at /api/v1/series, /api/v1/labels, /api/v1/label/{name}/values")

	// Query UI
	queryUI := ui.NewQueryUI("") // Empty string means use current host
	mux.Handle("/ui", queryUI)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/ui", http.StatusFound)
		} else {
			http.NotFound(w, r)
		}
	})
	log.Printf("Query UI enabled at /ui (root / redirects to /ui)")

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK\n")
	})

	// Ingestion stats endpoint
	mux.HandleFunc("/ingest_stats", func(w http.ResponseWriter, r *http.Request) {
		stats := pipe.IngestStats()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_ingested":      stats.TotalSamples,
			"rate_per_sec":        stats.RatePerSec,
			"avg_rate_per_sec":    stats.AvgRatePerSec,
			"samples_in_interval": stats.SamplesInInterval,
			"interval_seconds":    stats.IntervalSeconds,
			"timestamp_ms":        stats.Timestamp.UnixMilli(),
			"timestamp_rfc3339":   stats.Timestamp.Format(time.RFC3339),
		})
	})

	// Metrics endpoint (Prometheus client_golang)
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:         cfg.Server.ListenAddress,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in background
	go func() {
		log.Printf("HTTP server listening on %s", cfg.Server.ListenAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Printf("PromSketch-Dropin started successfully!")

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received signal %v, shutting down gracefully...", sig)

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// 1. Stop accepting new requests
	log.Printf("Stopping HTTP server...")
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	// 2. Stop pipeline
	log.Printf("Stopping ingestion pipeline...")
	if err := pipe.Stop(); err != nil {
		log.Printf("Pipeline stop error: %v", err)
	}

	// 3. Flush pending samples to backend
	log.Printf("Flushing pending samples...")
	if err := forwarder.Stop(); err != nil {
		log.Printf("Forwarder stop error: %v", err)
	}

	// 4. Stop storage
	log.Printf("Stopping storage...")
	if err := stor.Stop(); err != nil {
		log.Printf("Storage stop error: %v", err)
	}

	log.Printf("Shutdown complete")
}
