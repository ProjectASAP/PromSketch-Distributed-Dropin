package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete configuration for PromSketch-Dropin
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Ingestion IngestionConfig `yaml:"ingestion"`
	Backend   BackendConfig   `yaml:"backend"`
	Sketch    SketchConfig    `yaml:"sketch"`
	Query     QueryConfig     `yaml:"query"`
}

// ServerConfig contains general server settings
type ServerConfig struct {
	ListenAddress string        `yaml:"listen_address"`
	ReadTimeout   time.Duration `yaml:"read_timeout"`
	WriteTimeout  time.Duration `yaml:"write_timeout"`
	LogLevel      string        `yaml:"log_level"`
	LogFormat     string        `yaml:"log_format"`
}

// IngestionConfig contains ingestion-related settings
type IngestionConfig struct {
	RemoteWrite RemoteWriteConfig `yaml:"remote_write"`
	Scrape      ScrapeConfig      `yaml:"scrape"`
}

// RemoteWriteConfig configures the remote write receiver
type RemoteWriteConfig struct {
	Enabled       bool   `yaml:"enabled"`
	ListenAddress string `yaml:"listen_address"`
	MaxSampleSize int    `yaml:"max_sample_size"`
}

// ScrapeConfig configures the built-in scrape manager
type ScrapeConfig struct {
	Enabled       bool   `yaml:"enabled"`
	ConfigFile    string `yaml:"config_file"`
	ScrapeInterval time.Duration `yaml:"scrape_interval"`
	ScrapeTimeout  time.Duration `yaml:"scrape_timeout"`
}

// BackendConfig configures the backend storage system
type BackendConfig struct {
	Type              string        `yaml:"type"` // victoriametrics, prometheus, influxdb, clickhouse
	URL               string        `yaml:"url"`
	RemoteWriteURL    string        `yaml:"remote_write_url"`
	Timeout           time.Duration `yaml:"timeout"`
	MaxRetries        int           `yaml:"max_retries"`
	BatchSize         int           `yaml:"batch_size"`
	FlushInterval     time.Duration `yaml:"flush_interval"`
	BasicAuth         *BasicAuth    `yaml:"basic_auth,omitempty"`
	BearerToken       string        `yaml:"bearer_token,omitempty"`
	BearerTokenFile   string        `yaml:"bearer_token_file,omitempty"`
}

// BasicAuth contains HTTP basic authentication credentials
type BasicAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// SketchConfig configures PromSketch instances
type SketchConfig struct {
	NumPartitions int            `yaml:"num_partitions"`
	Targets       []SketchTarget `yaml:"targets"`
	Defaults      SketchDefaults `yaml:"defaults"`
	MemoryLimit   string         `yaml:"memory_limit"`
}

// SketchTarget defines which time series should have sketch instances
type SketchTarget struct {
	Match    string          `yaml:"match"` // PromQL/MetricsQL matcher
	EHParams *EHParams       `yaml:"eh_params,omitempty"`
}

// SketchDefaults contains default parameters for all sketch targets
type SketchDefaults struct {
	EHParams EHParams `yaml:"eh_params"`
}

// EHParams contains Exponential Histogram parameters
type EHParams struct {
	WindowSize int64 `yaml:"window_size"` // Time window size in seconds
	K          int64 `yaml:"k"`           // EH K parameter
	KllK       int   `yaml:"kll_k"`       // KLL K parameter for quantile sketches
}

// QueryConfig configures the query API
type QueryConfig struct {
	ListenAddress    string        `yaml:"listen_address"`
	Timeout          time.Duration `yaml:"timeout"`
	MaxConcurrency   int           `yaml:"max_concurrency"`
	EnableFallback   bool          `yaml:"enable_fallback"`
	FallbackTimeout  time.Duration `yaml:"fallback_timeout"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	if err := cfg.applyDefaults(); err != nil {
		return nil, fmt.Errorf("failed to apply defaults: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// applyDefaults sets default values for unspecified config fields
func (c *Config) applyDefaults() error {
	// Server defaults
	if c.Server.ListenAddress == "" {
		c.Server.ListenAddress = ":9100"
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 30 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 30 * time.Second
	}
	if c.Server.LogLevel == "" {
		c.Server.LogLevel = "info"
	}
	if c.Server.LogFormat == "" {
		c.Server.LogFormat = "logfmt"
	}

	// Ingestion defaults
	if c.Ingestion.RemoteWrite.ListenAddress == "" {
		c.Ingestion.RemoteWrite.ListenAddress = ":9100"
	}
	if c.Ingestion.RemoteWrite.MaxSampleSize == 0 {
		c.Ingestion.RemoteWrite.MaxSampleSize = 5000000
	}

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

	// Sketch defaults
	if c.Sketch.NumPartitions == 0 {
		c.Sketch.NumPartitions = 16
	}
	if c.Sketch.Defaults.EHParams.WindowSize == 0 {
		c.Sketch.Defaults.EHParams.WindowSize = 1800 // 30 minutes
	}
	if c.Sketch.Defaults.EHParams.K == 0 {
		c.Sketch.Defaults.EHParams.K = 50
	}
	if c.Sketch.Defaults.EHParams.KllK == 0 {
		c.Sketch.Defaults.EHParams.KllK = 256
	}

	// Query defaults
	if c.Query.ListenAddress == "" {
		c.Query.ListenAddress = ":9100"
	}
	if c.Query.Timeout == 0 {
		c.Query.Timeout = 30 * time.Second
	}
	if c.Query.MaxConcurrency == 0 {
		c.Query.MaxConcurrency = 20
	}
	if c.Query.FallbackTimeout == 0 {
		c.Query.FallbackTimeout = 60 * time.Second
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate backend type
	switch c.Backend.Type {
	case "victoriametrics", "prometheus", "influxdb", "clickhouse":
		// Valid backend types
	default:
		return fmt.Errorf("unsupported backend type: %s", c.Backend.Type)
	}

	// Validate backend URL
	if c.Backend.URL == "" {
		return fmt.Errorf("backend URL is required")
	}

	// Validate at least one ingestion mode is enabled
	if !c.Ingestion.RemoteWrite.Enabled && !c.Ingestion.Scrape.Enabled {
		return fmt.Errorf("at least one ingestion mode (remote_write or scrape) must be enabled")
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.Server.LogLevel] {
		return fmt.Errorf("invalid log level: %s", c.Server.LogLevel)
	}

	return nil
}
