package cluster

import (
	"time"

	"github.com/promsketch/promsketch-dropin/internal/cluster/hash"
)

// ClusterConfig holds the cluster-level configuration shared by pskinsert and pskquery
type ClusterConfig struct {
	TotalPartitions   int              `yaml:"total_partitions"`
	ReplicationFactor int              `yaml:"replication_factor"`
	Discovery         DiscoveryConfig  `yaml:"discovery"`
	HealthCheck       HealthCheckConfig `yaml:"health_check"`
	CircuitBreaker    CircuitBreakerConfig `yaml:"circuit_breaker"`
}

// DiscoveryConfig configures node discovery
type DiscoveryConfig struct {
	Type        string       `yaml:"type"` // "static" or "kubernetes"
	StaticNodes []StaticNode `yaml:"static_nodes"`
	Kubernetes  *KubernetesConfig `yaml:"kubernetes,omitempty"`
}

// StaticNode represents a statically configured psksketch node
type StaticNode struct {
	ID             string `yaml:"id"`
	Address        string `yaml:"address"`
	PartitionStart int    `yaml:"partition_start"`
	PartitionEnd   int    `yaml:"partition_end"`
}

// KubernetesConfig holds Kubernetes discovery configuration
type KubernetesConfig struct {
	Namespace       string        `yaml:"namespace"`
	Service         string        `yaml:"service"`
	Port            int           `yaml:"port"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
}

// HealthCheckConfig configures health checking
type HealthCheckConfig struct {
	Interval         time.Duration `yaml:"interval"`
	Timeout          time.Duration `yaml:"timeout"`
	FailureThreshold int           `yaml:"failure_threshold"`
}

// CircuitBreakerConfig configures the circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold int           `yaml:"failure_threshold"`
	Timeout          time.Duration `yaml:"timeout"`
}

// ApplyDefaults sets default values
func (c *ClusterConfig) ApplyDefaults() {
	if c.TotalPartitions == 0 {
		c.TotalPartitions = 16
	}
	if c.ReplicationFactor == 0 {
		c.ReplicationFactor = 2
	}
	if c.HealthCheck.Interval == 0 {
		c.HealthCheck.Interval = 10 * time.Second
	}
	if c.HealthCheck.Timeout == 0 {
		c.HealthCheck.Timeout = 5 * time.Second
	}
	if c.HealthCheck.FailureThreshold == 0 {
		c.HealthCheck.FailureThreshold = 3
	}
	if c.CircuitBreaker.FailureThreshold == 0 {
		c.CircuitBreaker.FailureThreshold = 5
	}
	if c.CircuitBreaker.Timeout == 0 {
		c.CircuitBreaker.Timeout = 30 * time.Second
	}
}

// ToHashNodes converts static node configs to hash.Node slice
func (c *ClusterConfig) ToHashNodes() []*hash.Node {
	nodes := make([]*hash.Node, 0, len(c.Discovery.StaticNodes))
	for _, sn := range c.Discovery.StaticNodes {
		nodes = append(nodes, &hash.Node{
			ID:             sn.ID,
			Address:        sn.Address,
			PartitionStart: sn.PartitionStart,
			PartitionEnd:   sn.PartitionEnd,
		})
	}
	return nodes
}
