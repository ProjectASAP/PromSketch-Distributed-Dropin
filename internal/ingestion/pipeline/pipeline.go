package pipeline

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"

	"github.com/promsketch/promsketch-dropin/internal/backend"
	"github.com/promsketch/promsketch-dropin/internal/config"
	"github.com/promsketch/promsketch-dropin/internal/ingestion/remotewrite"
	"github.com/promsketch/promsketch-dropin/internal/ingestion/scrape"
	"github.com/promsketch/promsketch-dropin/internal/metrics"
	"github.com/promsketch/promsketch-dropin/internal/storage"
)

// Pipeline coordinates ingestion from multiple sources
type Pipeline struct {
	config          *config.Config
	storage         *storage.Storage
	forwarder       *backend.Forwarder
	remoteWriteHdlr *remotewrite.Handler
	scrapeManager   *scrape.Manager
	metrics         *pipelineMetrics
	mu              sync.RWMutex
}

// pipelineMetrics is the internal atomic state for pipeline counters
type pipelineMetrics struct {
	totalSamplesReceived    atomic.Uint64
	sketchSamplesInserted   atomic.Uint64
	backendSamplesForwarded atomic.Uint64
	errors                  atomic.Uint64
}

// PipelineMetrics is a point-in-time snapshot of pipeline statistics
type PipelineMetrics struct {
	TotalSamplesReceived    uint64
	SketchSamplesInserted   uint64
	BackendSamplesForwarded uint64
	Errors                  uint64
}

// NewPipeline creates a new ingestion pipeline
func NewPipeline(
	cfg *config.Config,
	stor *storage.Storage,
	fwd *backend.Forwarder,
) (*Pipeline, error) {
	p := &Pipeline{
		config:    cfg,
		storage:   stor,
		forwarder: fwd,
		metrics:   &pipelineMetrics{},
	}

	// Initialize remote write handler if enabled
	if cfg.Ingestion.RemoteWrite.Enabled {
		p.remoteWriteHdlr = remotewrite.NewHandler(p)
	}

	// Initialize scrape manager if enabled
	if cfg.Ingestion.Scrape.Enabled {
		// Note: Scrape manager integration requires more setup
		// For now, we'll skip it in favor of remote write
	}

	return p, nil
}

// Receive implements remotewrite.Receiver
// Receive implements remotewrite.Receiver
func (p *Pipeline) Receive(req *prompb.WriteRequest) error {
	// Process each time series in the request
	for i := range req.Timeseries {
		if err := p.processTimeSeries(&req.Timeseries[i]); err != nil {
			p.metrics.errors.Add(1)
			metrics.IngestionErrorsTotal.Inc()
			// Log error but continue processing
			continue
		}
	}


	return nil
}

// processTimeSeries processes a single time series
func (p *Pipeline) processTimeSeries(ts *prompb.TimeSeries) error {
	// Convert prompb labels to Prometheus labels
	lbls := prompbLabelsToLabels(ts.Labels)

	// Insert each sample into PromSketch storage
	for _, sample := range ts.Samples {
		p.metrics.totalSamplesReceived.Add(1)
		metrics.IngestionSamplesTotal.Inc()

		if err := p.storage.Insert(lbls, sample.Timestamp, sample.Value); err != nil {
			// Log error but continue
		} else {
			p.metrics.sketchSamplesInserted.Add(1)
			metrics.IngestionSketchSamplesTotal.Inc()
		}
	}

	// Forward the entire TimeSeries to backend once (not per sample)
	if err := p.forwarder.Forward(ts); err != nil {
		// Log error
	} else {
		p.metrics.backendSamplesForwarded.Add(uint64(len(ts.Samples)))
		metrics.IngestionBackendSamplesTotal.Add(float64(len(ts.Samples)))
	}

	return nil
}

// RemoteWriteHandler returns the HTTP handler for remote write
func (p *Pipeline) RemoteWriteHandler() *remotewrite.Handler {
	return p.remoteWriteHdlr
}

// Start starts the pipeline
func (p *Pipeline) Start(ctx context.Context) error {
	// Start scrape manager if enabled
	if p.scrapeManager != nil {
		if err := p.scrapeManager.Start(ctx); err != nil {
			return fmt.Errorf("failed to start scrape manager: %w", err)
		}
	}

	return nil
}

// Stop stops the pipeline
func (p *Pipeline) Stop() error {
	if p.scrapeManager != nil {
		p.scrapeManager.Stop()
	}
	return nil
}

// Metrics returns a point-in-time snapshot of the current pipeline metrics
func (p *Pipeline) Metrics() PipelineMetrics {
	return PipelineMetrics{
		TotalSamplesReceived:    p.metrics.totalSamplesReceived.Load(),
		SketchSamplesInserted:   p.metrics.sketchSamplesInserted.Load(),
		BackendSamplesForwarded: p.metrics.backendSamplesForwarded.Load(),
		Errors:                  p.metrics.errors.Load(),
	}
}

// prompbLabelsToLabels converts prompb labels to Prometheus labels
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
