package discovery

import (
	"context"

	"github.com/promsketch/promsketch-dropin/internal/cluster/hash"
)

// NodeDiscovery is the interface for discovering psksketch nodes
type NodeDiscovery interface {
	// Discover returns the current set of known nodes
	Discover() ([]*hash.Node, error)

	// Watch starts watching for node changes and calls the callback when nodes change
	Watch(ctx context.Context, callback func([]*hash.Node)) error

	// Close stops the discovery mechanism
	Close() error
}
