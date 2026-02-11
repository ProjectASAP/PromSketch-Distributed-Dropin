package hash

import (
	"fmt"
	"sort"
	"sync"

	"github.com/cespare/xxhash/v2"
)

// Node represents a psksketch node in the cluster
type Node struct {
	ID             string
	Address        string
	PartitionStart int // inclusive
	PartitionEnd   int // exclusive
}

// OwnsPartition returns true if this node owns the given partition ID
func (n *Node) OwnsPartition(partitionID int) bool {
	return partitionID >= n.PartitionStart && partitionID < n.PartitionEnd
}

// PartitionMapper maps metric names to partition IDs and partitions to nodes
type PartitionMapper struct {
	totalPartitions int
	nodes           []*Node
	nodesByID       map[string]*Node
	partitionToNode map[int]*Node // partition ID -> owning node
	mu              sync.RWMutex
}

// NewPartitionMapper creates a new partition mapper
func NewPartitionMapper(totalPartitions int, nodes []*Node) (*PartitionMapper, error) {
	if totalPartitions <= 0 {
		return nil, fmt.Errorf("totalPartitions must be positive, got %d", totalPartitions)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("at least one node is required")
	}

	pm := &PartitionMapper{
		totalPartitions: totalPartitions,
		nodes:           make([]*Node, len(nodes)),
		nodesByID:       make(map[string]*Node, len(nodes)),
		partitionToNode: make(map[int]*Node, totalPartitions),
	}

	copy(pm.nodes, nodes)

	// Build partition-to-node mapping
	for _, node := range nodes {
		pm.nodesByID[node.ID] = node
		for p := node.PartitionStart; p < node.PartitionEnd; p++ {
			if p >= totalPartitions {
				return nil, fmt.Errorf("node %s partition range [%d, %d) exceeds total partitions %d",
					node.ID, node.PartitionStart, node.PartitionEnd, totalPartitions)
			}
			if existing, ok := pm.partitionToNode[p]; ok {
				return nil, fmt.Errorf("partition %d assigned to both %s and %s",
					p, existing.ID, node.ID)
			}
			pm.partitionToNode[p] = node
		}
	}

	// Verify all partitions are assigned
	for p := 0; p < totalPartitions; p++ {
		if _, ok := pm.partitionToNode[p]; !ok {
			return nil, fmt.Errorf("partition %d is not assigned to any node", p)
		}
	}

	return pm, nil
}

// GetPartitionID returns the partition ID for a metric name using xxhash
func (pm *PartitionMapper) GetPartitionID(metricName string) int {
	h := xxhash.Sum64String(metricName)
	return int(h % uint64(pm.totalPartitions))
}

// GetPrimaryNode returns the node that owns the given partition
func (pm *PartitionMapper) GetPrimaryNode(partitionID int) *Node {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.partitionToNode[partitionID]
}

// GetNodesForPartition returns primary + replica nodes for a partition.
// replicaCount is the total number of copies (including primary).
// For replicaCount=2, returns [primary, replica1].
func (pm *PartitionMapper) GetNodesForPartition(partitionID int, replicaCount int) []*Node {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	primary := pm.partitionToNode[partitionID]
	if primary == nil {
		return nil
	}

	if replicaCount <= 1 || len(pm.nodes) <= 1 {
		return []*Node{primary}
	}

	result := make([]*Node, 0, replicaCount)
	result = append(result, primary)

	// Select replica nodes using rendezvous (highest random weight) hashing
	// This ensures deterministic, stable replica selection
	type nodeScore struct {
		node  *Node
		score uint64
	}

	candidates := make([]nodeScore, 0, len(pm.nodes)-1)
	for _, n := range pm.nodes {
		if n.ID == primary.ID {
			continue
		}
		// Hash(partitionID + nodeID) for deterministic selection
		key := fmt.Sprintf("%d:%s", partitionID, n.ID)
		score := xxhash.Sum64String(key)
		candidates = append(candidates, nodeScore{node: n, score: score})
	}

	// Sort by score descending for deterministic replica selection
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Pick top (replicaCount - 1) replicas
	for i := 0; i < len(candidates) && len(result) < replicaCount; i++ {
		result = append(result, candidates[i].node)
	}

	return result
}

// GetNodeForMetric returns the primary node for a given metric name
func (pm *PartitionMapper) GetNodeForMetric(metricName string) *Node {
	partitionID := pm.GetPartitionID(metricName)
	return pm.GetPrimaryNode(partitionID)
}

// GetNodesForMetric returns all nodes (primary + replicas) for a given metric name
func (pm *PartitionMapper) GetNodesForMetric(metricName string, replicaCount int) []*Node {
	partitionID := pm.GetPartitionID(metricName)
	return pm.GetNodesForPartition(partitionID, replicaCount)
}

// AllNodes returns all nodes in the cluster
func (pm *PartitionMapper) AllNodes() []*Node {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]*Node, len(pm.nodes))
	copy(result, pm.nodes)
	return result
}

// GetNode returns a node by ID
func (pm *PartitionMapper) GetNode(id string) *Node {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.nodesByID[id]
}

// TotalPartitions returns the total number of partitions
func (pm *PartitionMapper) TotalPartitions() int {
	return pm.totalPartitions
}
