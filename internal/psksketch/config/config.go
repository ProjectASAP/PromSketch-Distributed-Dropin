package config

import (
	"fmt"
	"os"
	"time"

	mainconfig "github.com/promsketch/promsketch-dropin/internal/config"
	"gopkg.in/yaml.v3"
)

// Config represents the psksketch node configuration
type Config struct {
	Node    NodeConfig        `yaml:"node"`
	Server  ServerConfig      `yaml:"server"`
	HTTP    HTTPConfig        `yaml:"http"`
	Storage mainconfig.SketchConfig `yaml:"storage"`
}

// NodeConfig identifies this psksketch node
type NodeConfig struct {
	ID             string `yaml:"id"`
	PartitionStart int    `yaml:"partition_start"` // inclusive
	PartitionEnd   int    `yaml:"partition_end"`   // exclusive
}

// ServerConfig configures the gRPC server
type ServerConfig struct {
	ListenAddress  string `yaml:"listen_address"`
	MaxRecvMsgSize int    `yaml:"max_recv_msg_size"`
	MaxSendMsgSize int    `yaml:"max_send_msg_size"`
}

// HTTPConfig configures the HTTP server for metrics and health
type HTTPConfig struct {
	ListenAddress string        `yaml:"listen_address"`
	ReadTimeout   time.Duration `yaml:"read_timeout"`
	WriteTimeout  time.Duration `yaml:"write_timeout"`
}

// LoadConfig loads psksketch configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Wrap in psksketch key for YAML parsing
	var wrapper struct {
		PskSketch Config `yaml:"psksketch"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg := &wrapper.PskSketch
	cfg.applyDefaults()

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.ListenAddress == "" {
		c.Server.ListenAddress = ":8481"
	}
	if c.Server.MaxRecvMsgSize == 0 {
		c.Server.MaxRecvMsgSize = 10 * 1024 * 1024 // 10MB
	}
	if c.Server.MaxSendMsgSize == 0 {
		c.Server.MaxSendMsgSize = 10 * 1024 * 1024 // 10MB
	}
	if c.HTTP.ListenAddress == "" {
		c.HTTP.ListenAddress = ":8482"
	}
	if c.HTTP.ReadTimeout == 0 {
		c.HTTP.ReadTimeout = 30 * time.Second
	}
	if c.HTTP.WriteTimeout == 0 {
		c.HTTP.WriteTimeout = 30 * time.Second
	}
	if c.Storage.NumPartitions == 0 {
		c.Storage.NumPartitions = 16
	}
	if c.Storage.Defaults.EHParams.WindowSize == 0 {
		c.Storage.Defaults.EHParams.WindowSize = 1800
	}
	if c.Storage.Defaults.EHParams.K == 0 {
		c.Storage.Defaults.EHParams.K = 50
	}
	if c.Storage.Defaults.EHParams.KllK == 0 {
		c.Storage.Defaults.EHParams.KllK = 256
	}
}

func (c *Config) validate() error {
	if c.Node.ID == "" {
		return fmt.Errorf("node.id is required")
	}
	if c.Node.PartitionStart < 0 {
		return fmt.Errorf("node.partition_start must be >= 0")
	}
	if c.Node.PartitionEnd <= c.Node.PartitionStart {
		return fmt.Errorf("node.partition_end must be > partition_start")
	}
	if c.Node.PartitionEnd > c.Storage.NumPartitions {
		return fmt.Errorf("node.partition_end (%d) exceeds total partitions (%d)",
			c.Node.PartitionEnd, c.Storage.NumPartitions)
	}
	return nil
}
