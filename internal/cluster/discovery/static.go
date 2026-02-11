package discovery

import (
	"context"

	"github.com/promsketch/promsketch-dropin/internal/cluster/hash"
)

// StaticDiscovery returns a fixed list of nodes from configuration
type StaticDiscovery struct {
	nodes []*hash.Node
}

// NewStaticDiscovery creates a new static discovery from a list of nodes
func NewStaticDiscovery(nodes []*hash.Node) *StaticDiscovery {
	return &StaticDiscovery{
		nodes: nodes,
	}
}

// Discover returns the static list of nodes
func (d *StaticDiscovery) Discover() ([]*hash.Node, error) {
	result := make([]*hash.Node, len(d.nodes))
	copy(result, d.nodes)
	return result, nil
}

// Watch is a no-op for static discovery since nodes never change
func (d *StaticDiscovery) Watch(ctx context.Context, callback func([]*hash.Node)) error {
	// Static discovery never changes, so we just block until context is cancelled
	<-ctx.Done()
	return ctx.Err()
}

// Close is a no-op for static discovery
func (d *StaticDiscovery) Close() error {
	return nil
}
