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

	"github.com/promsketch/promsketch-dropin/internal/psksketch/config"
	"github.com/promsketch/promsketch-dropin/internal/psksketch/server"
	"github.com/promsketch/promsketch-dropin/internal/storage"
)

var (
	version   = "dev"
	gitCommit = "unknown"
	buildDate = "unknown"
)

func main() {
	configFile := flag.String("config.file", "psksketch.yaml", "Path to configuration file")
	// Note: "version" flag may already be registered by VictoriaMetrics lib/buildinfo
	if flag.CommandLine.Lookup("version") == nil {
		flag.Bool("version", false, "Show version information")
	}
	flag.Parse()

	if f := flag.CommandLine.Lookup("version"); f != nil && f.Value.String() == "true" {
		fmt.Printf("psksketch\n")
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

	log.Printf("Starting psksketch node...")
	log.Printf("  Node ID: %s", cfg.Node.ID)
	log.Printf("  Partition range: [%d, %d)", cfg.Node.PartitionStart, cfg.Node.PartitionEnd)
	log.Printf("  gRPC listen address: %s", cfg.Server.ListenAddress)
	log.Printf("  HTTP listen address: %s", cfg.HTTP.ListenAddress)
	log.Printf("  Total partitions: %d", cfg.Storage.NumPartitions)
	log.Printf("  Sketch targets: %d", len(cfg.Storage.Targets))

	// Pass partition ownership from node config to storage config
	cfg.Storage.PartitionStart = cfg.Node.PartitionStart
	cfg.Storage.PartitionEnd = cfg.Node.PartitionEnd

	// Initialize storage layer (reuse existing storage)
	stor, err := storage.NewStorage(&cfg.Storage)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create gRPC server
	grpcServer := server.NewSketchServer(stor, cfg)

	// Start gRPC server in background
	go func() {
		if err := grpcServer.Start(); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// Start HTTP server for health and metrics
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "healthy",
			"node_id": cfg.Node.ID,
			"partitions": map[string]int{
				"start": cfg.Node.PartitionStart,
				"end":   cfg.Node.PartitionEnd,
			},
		})
	})
	httpMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := stor.Metrics()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"node_id":          cfg.Node.ID,
			"total_series":     metrics.TotalSeries,
			"sketched_series":  metrics.SketchedSeries,
			"samples_inserted": metrics.SamplesInserted,
			"insert_errors":    metrics.SketchInsertErrors,
		})
	})

	httpServer := &http.Server{
		Addr:         cfg.HTTP.ListenAddress,
		Handler:      httpMux,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	go func() {
		log.Printf("psksketch HTTP server listening on %s", cfg.HTTP.ListenAddress)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("Received signal %s, shutting down...", sig)

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	grpcServer.Stop()
	log.Printf("gRPC server stopped")

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	log.Printf("HTTP server stopped")

	if err := stor.Stop(); err != nil {
		log.Printf("Storage shutdown error: %v", err)
	}
	log.Printf("Storage stopped")

	log.Printf("psksketch node %s shutdown complete", cfg.Node.ID)
}
