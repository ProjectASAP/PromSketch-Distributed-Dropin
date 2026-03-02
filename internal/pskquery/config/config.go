package config

import (
	"fmt"
	"os"
	"time"

	"github.com/promsketch/promsketch-dropin/internal/cluster"
	mainconfig "github.com/promsketch/promsketch-dropin/internal/config"
	"gopkg.in/yaml.v3"
)

// Config represents the pskquery configuration
type Config struct {
	Server  ServerConfig             `yaml:"server"`
	Cluster cluster.ClusterConfig    `yaml:"cluster"`
	Backend mainconfig.BackendConfig `yaml:"backend"`
	Query   QueryConfig              `yaml:"query"`
}

// ServerConfig configures the HTTP server
type ServerConfig struct {
	ListenAddress string        `yaml:"listen_address"`
	ReadTimeout   time.Duration `yaml:"read_timeout"`
	WriteTimeout  time.Duration `yaml:"write_timeout"`
}

// QueryConfig configures query behavior
type QueryConfig struct {
	EnableFallback       bool                           `yaml:"enable_fallback"`
	FallbackTimeout      time.Duration                  `yaml:"fallback_timeout"`
	QueryTimeout         time.Duration                  `yaml:"query_timeout"`
	MaxConcurrentQueries int                            `yaml:"max_concurrent_queries"`
	Approximation        mainconfig.ApproximationConfig `yaml:"approximation"`
}

// LoadConfig loads pskquery configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var wrapper struct {
		PskQuery Config `yaml:"pskquery"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg := &wrapper.PskQuery
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.ListenAddress == "" {
		c.Server.ListenAddress = ":8480"
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 60 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 60 * time.Second
	}
	c.Cluster.ApplyDefaults()

	if c.Query.QueryTimeout == 0 {
		c.Query.QueryTimeout = 30 * time.Second
	}
	if c.Query.FallbackTimeout == 0 {
		c.Query.FallbackTimeout = 60 * time.Second
	}
	if c.Query.MaxConcurrentQueries == 0 {
		c.Query.MaxConcurrentQueries = 100
	}
	if c.Query.Approximation.Epsilon == 0 {
		c.Query.Approximation.Epsilon = 0.02
	}
	if c.Query.Approximation.Confidence == 0 {
		c.Query.Approximation.Confidence = 0.95
	}
	if c.Backend.Timeout == 0 {
		c.Backend.Timeout = 60 * time.Second
	}
}

func (c *Config) validate() error {
	if c.Query.Approximation.Epsilon <= 0 || c.Query.Approximation.Epsilon > 1 {
		return fmt.Errorf("pskquery.query.approximation.epsilon must be in range (0, 1], got %v", c.Query.Approximation.Epsilon)
	}
	if c.Query.Approximation.Confidence <= 0 || c.Query.Approximation.Confidence > 1 {
		return fmt.Errorf("pskquery.query.approximation.confidence must be in range (0, 1], got %v", c.Query.Approximation.Confidence)
	}
	return nil
}
