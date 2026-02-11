package discovery

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"time"

	"github.com/promsketch/promsketch-dropin/internal/cluster/hash"
)

// KubernetesDiscovery discovers psksketch nodes via Kubernetes headless service DNS
type KubernetesDiscovery struct {
	namespace        string
	service          string
	port             int
	totalPartitions  int
	refreshInterval  time.Duration
	nodes            []*hash.Node
}

// KubernetesConfig holds Kubernetes discovery configuration
type KubernetesConfig struct {
	Namespace       string        `yaml:"namespace"`
	Service         string        `yaml:"service"`
	Port            int           `yaml:"port"`
	TotalPartitions int           `yaml:"total_partitions"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
}

// NewKubernetesDiscovery creates a Kubernetes-based node discovery
func NewKubernetesDiscovery(cfg *KubernetesConfig) *KubernetesDiscovery {
	if cfg.RefreshInterval == 0 {
		cfg.RefreshInterval = 30 * time.Second
	}
	if cfg.Port == 0 {
		cfg.Port = 8481
	}
	if cfg.TotalPartitions == 0 {
		cfg.TotalPartitions = 16
	}

	return &KubernetesDiscovery{
		namespace:       cfg.Namespace,
		service:         cfg.Service,
		port:            cfg.Port,
		totalPartitions: cfg.TotalPartitions,
		refreshInterval: cfg.RefreshInterval,
	}
}

// Discover resolves the headless service DNS and returns discovered nodes.
// Partitions are evenly distributed across discovered pods.
func (d *KubernetesDiscovery) Discover() ([]*hash.Node, error) {
	// Resolve headless service DNS: <service>.<namespace>.svc.cluster.local
	dnsName := fmt.Sprintf("%s.%s.svc.cluster.local", d.service, d.namespace)

	addrs, err := net.LookupHost(dnsName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", dnsName, err)
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("no endpoints found for %s", dnsName)
	}

	// Sort for deterministic partition assignment
	sort.Strings(addrs)

	// Distribute partitions evenly across discovered pods
	partitionsPerNode := d.totalPartitions / len(addrs)
	remainder := d.totalPartitions % len(addrs)

	nodes := make([]*hash.Node, 0, len(addrs))
	partStart := 0

	for i, addr := range addrs {
		count := partitionsPerNode
		if i < remainder {
			count++
		}

		node := &hash.Node{
			ID:             fmt.Sprintf("psksketch-%d", i),
			Address:        net.JoinHostPort(addr, strconv.Itoa(d.port)),
			PartitionStart: partStart,
			PartitionEnd:   partStart + count,
		}
		nodes = append(nodes, node)
		partStart += count
	}

	d.nodes = nodes
	return nodes, nil
}

// Watch periodically re-discovers nodes and calls the callback when nodes change
func (d *KubernetesDiscovery) Watch(ctx context.Context, callback func([]*hash.Node)) error {
	ticker := time.NewTicker(d.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			nodes, err := d.Discover()
			if err != nil {
				continue // Log error but keep trying
			}

			// Check if nodes changed
			if d.nodesChanged(nodes) {
				callback(nodes)
			}
		}
	}
}

// nodesChanged checks if the discovered nodes are different from the current set
func (d *KubernetesDiscovery) nodesChanged(newNodes []*hash.Node) bool {
	if len(d.nodes) != len(newNodes) {
		return true
	}
	for i, n := range d.nodes {
		if n.ID != newNodes[i].ID || n.Address != newNodes[i].Address {
			return true
		}
	}
	return false
}

// Close is a no-op for Kubernetes discovery
func (d *KubernetesDiscovery) Close() error {
	return nil
}
