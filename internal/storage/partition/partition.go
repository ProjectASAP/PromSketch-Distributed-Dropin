package partition

import (
	"hash/fnv"

	"github.com/prometheus/prometheus/model/labels"
)

// Partitioner handles consistent hashing of metric names to partition IDs
type Partitioner struct {
	numPartitions int
}

// NewPartitioner creates a new partitioner
func NewPartitioner(numPartitions int) *Partitioner {
	if numPartitions <= 0 {
		numPartitions = 1
	}
	return &Partitioner{
		numPartitions: numPartitions,
	}
}

// GetPartition returns the partition ID for a given label set
// Partition is determined by hashing the metric name
func (p *Partitioner) GetPartition(lbls labels.Labels) int {
	metricName := lbls.Get(labels.MetricName)
	return p.GetPartitionByName(metricName)
}

// GetPartitionByName returns the partition ID for a metric name
func (p *Partitioner) GetPartitionByName(metricName string) int {
	h := fnv.New64a()
	h.Write([]byte(metricName))
	hash := h.Sum64()
	return int(hash % uint64(p.numPartitions))
}

// NumPartitions returns the number of partitions
func (p *Partitioner) NumPartitions() int {
	return p.numPartitions
}
