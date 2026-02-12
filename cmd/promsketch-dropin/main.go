package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/promsketch/promsketch-dropin/internal/backend"
	"github.com/promsketch/promsketch-dropin/internal/backendfactory"
	"github.com/promsketch/promsketch-dropin/internal/config"
	"github.com/promsketch/promsketch-dropin/internal/ingestion/pipeline"
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

	log.Printf("Starting PromSketch-Dropin...")
	log.Printf("  Server listen address: %s", cfg.Server.ListenAddress)
	log.Printf("  Backend type: %s", cfg.Backend.Type)
	log.Printf("  Backend URL: %s", cfg.Backend.URL)
	log.Printf("  Sketch partitions: %d", cfg.Sketch.NumPartitions)
	log.Printf("  Sketch targets: %d", len(cfg.Sketch.Targets))
	log.Printf("  Remote write enabled: %v", cfg.Ingestion.RemoteWrite.Enabled)
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
	queryAPI := api.NewQueryAPI(queryRouter)

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

	// Metrics endpoint (basic)
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		storageMetrics := stor.Metrics()
		forwarderMetrics := forwarder.Metrics()
		pipelineMetrics := pipe.Metrics()
		routerMetrics := queryRouter.Metrics()
		apiMetrics := queryAPI.Metrics()
		metadataMetrics := metadataAPI.Metrics()

		fmt.Fprintf(w, "# PromSketch-Dropin Metrics\n")
		fmt.Fprintf(w, "storage_total_series %d\n", storageMetrics.TotalSeries)
		fmt.Fprintf(w, "storage_sketched_series %d\n", storageMetrics.SketchedSeries)
		fmt.Fprintf(w, "storage_samples_inserted %d\n", storageMetrics.SamplesInserted)
		fmt.Fprintf(w, "storage_sketch_errors %d\n", storageMetrics.SketchInsertErrors)
		fmt.Fprintf(w, "forwarder_samples_forwarded %d\n", forwarderMetrics.SamplesForwarded)
		fmt.Fprintf(w, "forwarder_samples_dropped %d\n", forwarderMetrics.SamplesDropped)
		fmt.Fprintf(w, "forwarder_batches_sent %d\n", forwarderMetrics.BatchesSent)
		fmt.Fprintf(w, "forwarder_batches_failed %d\n", forwarderMetrics.BatchesFailed)
		fmt.Fprintf(w, "pipeline_samples_received %d\n", pipelineMetrics.TotalSamplesReceived)
		fmt.Fprintf(w, "pipeline_sketch_samples %d\n", pipelineMetrics.SketchSamplesInserted)
		fmt.Fprintf(w, "pipeline_backend_samples %d\n", pipelineMetrics.BackendSamplesForwarded)
		fmt.Fprintf(w, "pipeline_errors %d\n", pipelineMetrics.Errors)
		fmt.Fprintf(w, "router_sketch_queries %d\n", routerMetrics.SketchQueries)
		fmt.Fprintf(w, "router_backend_queries %d\n", routerMetrics.BackendQueries)
		fmt.Fprintf(w, "router_sketch_hits %d\n", routerMetrics.SketchHits)
		fmt.Fprintf(w, "router_sketch_misses %d\n", routerMetrics.SketchMisses)
		fmt.Fprintf(w, "router_parsing_errors %d\n", routerMetrics.ParsingErrors)
		fmt.Fprintf(w, "router_execution_errors %d\n", routerMetrics.ExecutionErrors)
		fmt.Fprintf(w, "api_query_requests %d\n", apiMetrics.QueryRequests)
		fmt.Fprintf(w, "api_query_range_requests %d\n", apiMetrics.QueryRangeRequests)
		fmt.Fprintf(w, "api_query_errors %d\n", apiMetrics.QueryErrors)
		fmt.Fprintf(w, "api_query_range_errors %d\n", apiMetrics.QueryRangeErrors)
		fmt.Fprintf(w, "metadata_series_requests %d\n", metadataMetrics.SeriesRequests)
		fmt.Fprintf(w, "metadata_labels_requests %d\n", metadataMetrics.LabelsRequests)
		fmt.Fprintf(w, "metadata_label_values_requests %d\n", metadataMetrics.LabelValuesRequests)
		fmt.Fprintf(w, "metadata_errors %d\n", metadataMetrics.Errors)
	})

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
