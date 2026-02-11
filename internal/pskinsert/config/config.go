package config

import (
	"fmt"
	"os"
	"time"

	"github.com/promsketch/promsketch-dropin/internal/cluster"
	mainconfig "github.com/promsketch/promsketch-dropin/internal/config"
	"gopkg.in/yaml.v3"
)

// Config represents the pskinsert configuration
type Config struct {
	Server  ServerConfig          `yaml:"server"`
	Cluster cluster.ClusterConfig `yaml:"cluster"`
	Backend mainconfig.BackendConfig `yaml:"backend"`
	Sketch  SketchConfig          `yaml:"sketch"`
}

// ServerConfig configures the HTTP server
type ServerConfig struct {
	ListenAddress string        `yaml:"listen_address"`
	ReadTimeout   time.Duration `yaml:"read_timeout"`
	WriteTimeout  time.Duration `yaml:"write_timeout"`
}

// SketchConfig holds sketch target matching config for pskinsert
type SketchConfig struct {
	Targets  []mainconfig.SketchTarget `yaml:"targets"`
	Defaults mainconfig.SketchDefaults `yaml:"defaults"`
}

// LoadConfig loads pskinsert configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var wrapper struct {
		PskInsert Config `yaml:"pskinsert"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg := &wrapper.PskInsert
	cfg.applyDefaults()

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.ListenAddress == "" {
		c.Server.ListenAddress = ":8480"
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 30 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 30 * time.Second
	}
	c.Cluster.ApplyDefaults()

	// Backend defaults
	if c.Backend.Timeout == 0 {
		c.Backend.Timeout = 30 * time.Second
	}
	if c.Backend.MaxRetries == 0 {
		c.Backend.MaxRetries = 3
	}
	if c.Backend.BatchSize == 0 {
		c.Backend.BatchSize = 1000
	}
	if c.Backend.FlushInterval == 0 {
		c.Backend.FlushInterval = 5 * time.Second
	}
}

func (c *Config) validate() error {
	if len(c.Cluster.Discovery.StaticNodes) == 0 && c.Cluster.Discovery.Type == "static" {
		return fmt.Errorf("static discovery requires at least one node")
	}
	if c.Backend.URL == "" {
		return fmt.Errorf("backend URL is required")
	}
	if c.Backend.Type == "" {
		return fmt.Errorf("backend type is required")
	}
	return nil
}
