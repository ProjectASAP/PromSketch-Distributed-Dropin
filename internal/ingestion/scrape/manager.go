package scrape

import (
	"context"
	"flag"
	"fmt"

	"github.com/zzylol/VictoriaMetrics/lib/auth"
	"github.com/zzylol/VictoriaMetrics/lib/prompbmarshal"
	"github.com/zzylol/VictoriaMetrics/lib/promscrape"

	"github.com/promsketch/promsketch-dropin/internal/config"
)

// Receiver processes scraped time series from promscrape.
type Receiver interface {
	ReceiveVMMarshalTimeSeries(tss []prompbmarshal.TimeSeries) error
}

// Manager wraps VictoriaMetrics promscrape for Prometheus-compatible scraping
// with full service discovery (static, k8s, consul, dns, etc.) and relabeling.
type Manager struct {
	config   *config.ScrapeConfig
	receiver Receiver
}

// NewManager creates a new scrape manager
func NewManager(cfg *config.ScrapeConfig, receiver Receiver) (*Manager, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("scrape manager is not enabled")
	}

	return &Manager{
		config:   cfg,
		receiver: receiver,
	}, nil
}

// Start starts the scrape manager using VictoriaMetrics promscrape.
// The ctx parameter is accepted for API compatibility but promscrape
// manages its own lifecycle internally.
func (m *Manager) Start(ctx context.Context) error {
	if m.config.ConfigFile != "" {
		if err := flag.Set("promscrape.config", m.config.ConfigFile); err != nil {
			return fmt.Errorf("failed to set promscrape.config flag: %w", err)
		}
	}

	promscrape.Init(func(at *auth.Token, wr *prompbmarshal.WriteRequest) {
		if wr == nil {
			return
		}
		// Errors are logged internally by the pipeline; we don't fail the scrape.
		_ = m.receiver.ReceiveVMMarshalTimeSeries(wr.Timeseries)
	})

	return nil
}

// Stop stops the scrape manager
func (m *Manager) Stop() error {
	promscrape.Stop()
	return nil
}
