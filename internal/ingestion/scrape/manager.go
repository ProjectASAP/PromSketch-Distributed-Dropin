package scrape

import (
	"context"
	"fmt"

	"github.com/promsketch/promsketch-dropin/internal/config"
)

// Manager manages scrape targets
// NOTE: This is a simplified stub for Phase 2. Full implementation with
// Prometheus scraping library will be done in Phase 8.
type Manager struct {
	config *config.ScrapeConfig
	stopCh chan struct{}
}

// NewManager creates a new scrape manager
func NewManager(cfg *config.ScrapeConfig) (*Manager, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("scrape manager is not enabled")
	}

	m := &Manager{
		config: cfg,
		stopCh: make(chan struct{}),
	}

	return m, nil
}

// Start starts the scrape manager
func (m *Manager) Start(ctx context.Context) error {
	// TODO: Implement full scrape manager in Phase 8
	// For now, this is a placeholder that does nothing
	return nil
}

// Stop stops the scrape manager
func (m *Manager) Stop() error {
	close(m.stopCh)
	return nil
}
