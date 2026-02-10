package backendfactory

import (
	"fmt"

	"github.com/promsketch/promsketch-dropin/internal/backend"
	"github.com/promsketch/promsketch-dropin/internal/backend/prometheus"
	"github.com/promsketch/promsketch-dropin/internal/backend/victoriametrics"
	"github.com/promsketch/promsketch-dropin/internal/config"
)

// NewBackend creates a new backend client based on configuration
func NewBackend(cfg *config.BackendConfig) (backend.Backend, error) {
	switch cfg.Type {
	case "victoriametrics":
		return victoriametrics.NewClient(cfg)
	case "prometheus":
		return prometheus.NewClient(cfg)
	default:
		return nil, fmt.Errorf("unsupported backend type: %s", cfg.Type)
	}
}
