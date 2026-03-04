package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"

	"github.com/zzylol/VictoriaMetrics/lib/auth"
	"github.com/zzylol/VictoriaMetrics/lib/prompbmarshal"
	"github.com/zzylol/VictoriaMetrics/lib/promscrape"
	otlpstream "github.com/zzylol/VictoriaMetrics/lib/protoparser/opentelemetry/stream"

	"github.com/promsketch/promsketch-dropin/internal/backend"
	"github.com/promsketch/promsketch-dropin/internal/backendfactory"
	"github.com/promsketch/promsketch-dropin/internal/cluster/hash"
	"github.com/promsketch/promsketch-dropin/internal/cluster/health"
	"github.com/promsketch/promsketch-dropin/internal/ingestion/pipeline"
	ingeststats "github.com/promsketch/promsketch-dropin/internal/ingestion/stats"
	"github.com/promsketch/promsketch-dropin/internal/metrics"
	"github.com/promsketch/promsketch-dropin/internal/pskinsert/client"
	pskconfig "github.com/promsketch/promsketch-dropin/internal/pskinsert/config"
	"github.com/promsketch/promsketch-dropin/internal/pskinsert/router"
)

var (
	version   = "dev"
	gitCommit = "unknown"
	buildDate = "unknown"
)

func main() {
	configFile := flag.String("config.file", "pskinsert.yaml", "Path to configuration file")
	// Note: "version" flag may already be registered by VictoriaMetrics lib/buildinfo
	if flag.CommandLine.Lookup("version") == nil {
		flag.Bool("version", false, "Show version information")
	}
	flag.Parse()

	if f := flag.CommandLine.Lookup("version"); f != nil && f.Value.String() == "true" {
		fmt.Printf("pskinsert\n")
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

	metrics.SetBuildInfo(version, gitCommit, buildDate, "pskinsert")
	log.Printf("Starting pskinsert...")
	log.Printf("  Listen address: %s", cfg.Server.ListenAddress)
	log.Printf("  Total partitions: %d", cfg.Cluster.TotalPartitions)
	log.Printf("  Replication factor: %d", cfg.Cluster.ReplicationFactor)
	log.Printf("  Sketch nodes: %d", len(cfg.Cluster.Discovery.StaticNodes))
	log.Printf("  Backend: %s (%s)", cfg.Backend.Type, cfg.Backend.URL)

	// Build node list from config
	nodes := cfg.Cluster.ToHashNodes()

	// Initialize partition mapper
	partitioner, err := hash.NewPartitionMapper(cfg.Cluster.TotalPartitions, nodes)
	if err != nil {
		log.Fatalf("Failed to create partition mapper: %v", err)
	}

	// Initialize gRPC client pool
	pool, err := client.NewPool(nodes)
	if err != nil {
		log.Fatalf("Failed to create client pool: %v", err)
	}
	defer pool.Close()

	// Initialize health checker with gRPC health check
	healthCfg := &health.HealthCheckerConfig{
		Interval:         cfg.Cluster.HealthCheck.Interval,
		Timeout:          cfg.Cluster.HealthCheck.Timeout,
		FailureThreshold: cfg.Cluster.HealthCheck.FailureThreshold,
		CircuitTimeout:   cfg.Cluster.CircuitBreaker.Timeout,
	}
	healthChecker := health.NewHealthChecker(healthCfg, func(ctx context.Context, address string) error {
		// Use gRPC health check
		for _, node := range nodes {
			if node.Address == address {
				c, ok := pool.GetClient(node.ID)
				if !ok {
					return fmt.Errorf("no client for node")
				}
				_, err := c.Health(ctx, nil)
				return err
			}
		}
		return fmt.Errorf("unknown address: %s", address)
	})
	healthChecker.RegisterNodes(nodes, cfg.Cluster.CircuitBreaker.FailureThreshold, cfg.Cluster.CircuitBreaker.Timeout)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	healthChecker.Start(ctx)

	// Initialize insert router
	insertRouter := router.NewRouter(partitioner, pool, healthChecker, cfg.Cluster.ReplicationFactor)

	// Initialize backend forwarder
	backendClient, err := backendfactory.NewBackend(&cfg.Backend)
	if err != nil {
		log.Fatalf("Failed to create backend client: %v", err)
	}
	forwarder := backend.NewForwarder(backendClient, &cfg.Backend)
	forwarder.Start()
	statsTracker := ingeststats.NewTracker(time.Second, 10)
	statsTracker.Start()

	// Setup HTTP server
	mux := http.NewServeMux()

	// Remote write endpoint
	mux.HandleFunc("/api/v1/write", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		compressed, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read body: %v", err), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		decompressed, err := snappy.Decode(nil, compressed)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to decompress: %v", err), http.StatusBadRequest)
			return
		}

		var req prompb.WriteRequest
		if err := proto.Unmarshal(decompressed, &req); err != nil {
			http.Error(w, fmt.Sprintf("Failed to unmarshal: %v", err), http.StatusBadRequest)
			return
		}

		// Route each time series to psksketch nodes
		for i := range req.Timeseries {
			ts := &req.Timeseries[i]
			lbls := prompbLabelsToLabels(ts.Labels)
			statsTracker.AddSamples(uint64(len(ts.Samples)))

			for _, sample := range ts.Samples {
				if err := insertRouter.Insert(lbls, sample.Timestamp, sample.Value); err != nil {
					// Log but don't fail the entire request
					log.Printf("Insert error: %v", err)
				}
			}

			// Forward to backend (always)
			if err := forwarder.Forward(ts); err != nil {
				log.Printf("Forward error: %v", err)
			}
		}

		w.WriteHeader(http.StatusNoContent)
	})

	// OTLP endpoint
	mux.HandleFunc("/opentelemetry/v1/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		isGzipped := r.Header.Get("Content-Encoding") == "gzip"
		err := otlpstream.ParseStream(r.Body, isGzipped, nil, func(tss []prompbmarshal.TimeSeries) error {
			for i := range tss {
				ts := &tss[i]
				statsTracker.AddSamples(uint64(len(ts.Samples)))
				lbls := pipeline.VMMarshalLabelsToLabels(ts.Labels)
				for _, sample := range ts.Samples {
					if err := insertRouter.Insert(lbls, sample.Timestamp, sample.Value); err != nil {
						log.Printf("Insert error: %v", err)
					}
				}
				promTS := pipeline.VMMarshalTSToPrompb(ts)
				if err := forwarder.Forward(promTS); err != nil {
					log.Printf("Forward error: %v", err)
				}
			}
			return nil
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to process OTLP request: %v", err), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	log.Printf("OTLP endpoint enabled at /opentelemetry/v1/metrics")

	// Start promscrape if configured
	if scrapeConfigFile := flag.Lookup("promscrape.config"); scrapeConfigFile != nil && scrapeConfigFile.Value.String() != "" {
		promscrape.Init(func(at *auth.Token, wr *prompbmarshal.WriteRequest) {
			if wr == nil {
				return
			}
			for i := range wr.Timeseries {
				ts := &wr.Timeseries[i]
				statsTracker.AddSamples(uint64(len(ts.Samples)))
				lbls := pipeline.VMMarshalLabelsToLabels(ts.Labels)
				for _, sample := range ts.Samples {
					if err := insertRouter.Insert(lbls, sample.Timestamp, sample.Value); err != nil {
						log.Printf("Scrape insert error: %v", err)
					}
				}
				promTS := pipeline.VMMarshalTSToPrompb(ts)
				if err := forwarder.Forward(promTS); err != nil {
					log.Printf("Scrape forward error: %v", err)
				}
			}
		})
		log.Printf("Promscrape enabled with config: %s", scrapeConfigFile.Value.String())
	}

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "healthy",
			"component":   "pskinsert",
			"node_health": healthChecker.GetHealthStatus(),
		})
	})

	// Metrics endpoint (Prometheus client_golang)
	mux.Handle("/metrics", promhttp.Handler())

	// Ingestion stats endpoint
	mux.HandleFunc("/ingest_stats", func(w http.ResponseWriter, r *http.Request) {
		stats := statsTracker.Snapshot()
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

	httpServer := &http.Server{
		Addr:         cfg.Server.ListenAddress,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		log.Printf("pskinsert HTTP server listening on %s", cfg.Server.ListenAddress)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("Received signal %s, shutting down...", sig)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	promscrape.Stop()
	forwarder.Stop()
	statsTracker.Stop()
	healthChecker.Stop()
	pool.Close()

	log.Printf("pskinsert shutdown complete")
}

func prompbLabelsToLabels(prompbLabels []prompb.Label) labels.Labels {
	lbls := make(labels.Labels, 0, len(prompbLabels))
	for _, l := range prompbLabels {
		lbls = append(lbls, labels.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}
	return lbls
}
