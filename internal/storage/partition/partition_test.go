package partition

import (
	"testing"

	"github.com/prometheus/prometheus/model/labels"
)

func TestPartitioner_ConsistentHashing(t *testing.T) {
	p := NewPartitioner(16)

	if p.NumPartitions() != 16 {
		t.Errorf("Expected 16 partitions, got %d", p.NumPartitions())
	}

	// Test that same metric name always goes to same partition
	metricName := "http_requests_total"
	lbls1 := labels.Labels{
		{Name: "__name__", Value: metricName},
		{Name: "job", Value: "api"},
	}
	lbls2 := labels.Labels{
		{Name: "__name__", Value: metricName},
		{Name: "job", Value: "different_job"},
	}

	partition1 := p.GetPartition(lbls1)
	partition2 := p.GetPartition(lbls2)

	if partition1 != partition2 {
		t.Errorf("Same metric name should map to same partition, got %d and %d", partition1, partition2)
	}

	// Verify partition is within range
	if partition1 < 0 || partition1 >= 16 {
		t.Errorf("Partition %d out of range [0, 16)", partition1)
	}
}

func TestPartitioner_DifferentMetrics(t *testing.T) {
	p := NewPartitioner(16)

	metrics := []string{
		"http_requests_total",
		"node_cpu_seconds_total",
		"memory_usage_bytes",
		"disk_io_operations",
	}

	partitions := make(map[int]bool)
	for _, metricName := range metrics {
		lbls := labels.Labels{
			{Name: "__name__", Value: metricName},
		}
		partition := p.GetPartition(lbls)

		if partition < 0 || partition >= 16 {
			t.Errorf("Partition %d out of range for metric %s", partition, metricName)
		}

		partitions[partition] = true
	}

	// With 4 metrics and 16 partitions, we should ideally have some distribution
	// (though hash collisions are possible)
	if len(partitions) < 2 {
		t.Log("Warning: All metrics mapped to same partition (unlikely but possible)")
	}
}

func TestPartitioner_SinglePartition(t *testing.T) {
	p := NewPartitioner(1)

	lbls := labels.Labels{
		{Name: "__name__", Value: "test_metric"},
	}

	partition := p.GetPartition(lbls)
	if partition != 0 {
		t.Errorf("Expected partition 0 with single partition, got %d", partition)
	}
}

func TestPartitioner_ZeroPartitions(t *testing.T) {
	// Should default to 1 partition
	p := NewPartitioner(0)

	if p.NumPartitions() != 1 {
		t.Errorf("Expected default of 1 partition, got %d", p.NumPartitions())
	}
}

func TestPartitioner_ByName(t *testing.T) {
	p := NewPartitioner(16)

	metricName := "http_requests_total"
	lbls := labels.Labels{
		{Name: "__name__", Value: metricName},
	}

	partition1 := p.GetPartition(lbls)
	partition2 := p.GetPartitionByName(metricName)

	if partition1 != partition2 {
		t.Errorf("GetPartition and GetPartitionByName should return same result, got %d and %d", partition1, partition2)
	}
}
