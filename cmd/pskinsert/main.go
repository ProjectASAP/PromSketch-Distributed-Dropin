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
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"

	"github.com/promsketch/promsketch-dropin/internal/backend"
	"github.com/promsketch/promsketch-dropin/internal/backendfactory"
	"github.com/promsketch/promsketch-dropin/internal/cluster/hash"
	"github.com/promsketch/promsketch-dropin/internal/cluster/health"
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

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       "healthy",
			"component":    "pskinsert",
			"node_health":  healthChecker.GetHealthStatus(),
		})
	})

	// Metrics endpoint (Prometheus text exposition format)
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		m := insertRouter.Metrics()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprintf(w, "# HELP pskinsert_samples_routed_total Total samples routed to sketch nodes.\n")
		fmt.Fprintf(w, "# TYPE pskinsert_samples_routed_total counter\n")
		fmt.Fprintf(w, "pskinsert_samples_routed_total %d\n", m.SamplesRouted)
		fmt.Fprintf(w, "# HELP pskinsert_samples_failed_total Total samples that failed routing.\n")
		fmt.Fprintf(w, "# TYPE pskinsert_samples_failed_total counter\n")
		fmt.Fprintf(w, "pskinsert_samples_failed_total %d\n", m.SamplesFailed)
		fmt.Fprintf(w, "# HELP pskinsert_rpcs_sent_total Total gRPC insert RPCs sent.\n")
		fmt.Fprintf(w, "# TYPE pskinsert_rpcs_sent_total counter\n")
		fmt.Fprintf(w, "pskinsert_rpcs_sent_total %d\n", m.RPCsSent)
		fmt.Fprintf(w, "# HELP pskinsert_rpcs_failed_total Total gRPC insert RPCs failed.\n")
		fmt.Fprintf(w, "# TYPE pskinsert_rpcs_failed_total counter\n")
		fmt.Fprintf(w, "pskinsert_rpcs_failed_total %d\n", m.RPCsFailed)
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

	forwarder.Stop()
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
