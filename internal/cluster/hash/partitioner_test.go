package hash

import (
	"testing"
)

func TestNewPartitionMapper(t *testing.T) {
	nodes := []*Node{
		{ID: "node-1", Address: "node-1:8481", PartitionStart: 0, PartitionEnd: 6},
		{ID: "node-2", Address: "node-2:8481", PartitionStart: 6, PartitionEnd: 12},
		{ID: "node-3", Address: "node-3:8481", PartitionStart: 12, PartitionEnd: 16},
	}

	pm, err := NewPartitionMapper(16, nodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pm.TotalPartitions() != 16 {
		t.Errorf("expected 16 partitions, got %d", pm.TotalPartitions())
	}
}

func TestNewPartitionMapper_OverlappingPartitions(t *testing.T) {
	nodes := []*Node{
		{ID: "node-1", Address: "node-1:8481", PartitionStart: 0, PartitionEnd: 8},
		{ID: "node-2", Address: "node-2:8481", PartitionStart: 6, PartitionEnd: 16}, // overlaps with node-1
	}

	_, err := NewPartitionMapper(16, nodes)
	if err == nil {
		t.Fatal("expected error for overlapping partitions")
	}
}

func TestNewPartitionMapper_MissingPartitions(t *testing.T) {
	nodes := []*Node{
		{ID: "node-1", Address: "node-1:8481", PartitionStart: 0, PartitionEnd: 8},
		// Missing partitions 8-15
	}

	_, err := NewPartitionMapper(16, nodes)
	if err == nil {
		t.Fatal("expected error for missing partitions")
	}
}

func TestGetPartitionID_Deterministic(t *testing.T) {
	nodes := []*Node{
		{ID: "node-1", Address: "node-1:8481", PartitionStart: 0, PartitionEnd: 8},
		{ID: "node-2", Address: "node-2:8481", PartitionStart: 8, PartitionEnd: 16},
	}

	pm, err := NewPartitionMapper(16, nodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Same metric name should always get the same partition
	metric := "http_requests_total"
	p1 := pm.GetPartitionID(metric)
	p2 := pm.GetPartitionID(metric)

	if p1 != p2 {
		t.Errorf("expected same partition for same metric, got %d and %d", p1, p2)
	}

	if p1 < 0 || p1 >= 16 {
		t.Errorf("partition %d out of range [0, 16)", p1)
	}
}

func TestGetPrimaryNode(t *testing.T) {
	nodes := []*Node{
		{ID: "node-1", Address: "node-1:8481", PartitionStart: 0, PartitionEnd: 6},
		{ID: "node-2", Address: "node-2:8481", PartitionStart: 6, PartitionEnd: 12},
		{ID: "node-3", Address: "node-3:8481", PartitionStart: 12, PartitionEnd: 16},
	}

	pm, err := NewPartitionMapper(16, nodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Partition 0 should be node-1
	n := pm.GetPrimaryNode(0)
	if n.ID != "node-1" {
		t.Errorf("expected node-1 for partition 0, got %s", n.ID)
	}

	// Partition 6 should be node-2
	n = pm.GetPrimaryNode(6)
	if n.ID != "node-2" {
		t.Errorf("expected node-2 for partition 6, got %s", n.ID)
	}

	// Partition 15 should be node-3
	n = pm.GetPrimaryNode(15)
	if n.ID != "node-3" {
		t.Errorf("expected node-3 for partition 15, got %s", n.ID)
	}
}

func TestGetNodesForPartition_Replication(t *testing.T) {
	nodes := []*Node{
		{ID: "node-1", Address: "node-1:8481", PartitionStart: 0, PartitionEnd: 6},
		{ID: "node-2", Address: "node-2:8481", PartitionStart: 6, PartitionEnd: 12},
		{ID: "node-3", Address: "node-3:8481", PartitionStart: 12, PartitionEnd: 16},
	}

	pm, err := NewPartitionMapper(16, nodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With replication factor 2, should get primary + 1 replica
	result := pm.GetNodesForPartition(0, 2)
	if len(result) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(result))
	}

	// First should be the primary (node-1 owns partition 0)
	if result[0].ID != "node-1" {
		t.Errorf("expected primary node-1, got %s", result[0].ID)
	}

	// Second should be a different node
	if result[1].ID == "node-1" {
		t.Error("replica should be a different node than primary")
	}

	// Replication factor 1 should return just primary
	result = pm.GetNodesForPartition(0, 1)
	if len(result) != 1 {
		t.Fatalf("expected 1 node, got %d", len(result))
	}
}

func TestGetNodesForPartition_ReplicationDeterministic(t *testing.T) {
	nodes := []*Node{
		{ID: "node-1", Address: "node-1:8481", PartitionStart: 0, PartitionEnd: 6},
		{ID: "node-2", Address: "node-2:8481", PartitionStart: 6, PartitionEnd: 12},
		{ID: "node-3", Address: "node-3:8481", PartitionStart: 12, PartitionEnd: 16},
	}

	pm, err := NewPartitionMapper(16, nodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Same partition should always get the same replica nodes
	r1 := pm.GetNodesForPartition(3, 2)
	r2 := pm.GetNodesForPartition(3, 2)

	if r1[0].ID != r2[0].ID || r1[1].ID != r2[1].ID {
		t.Error("replica selection is not deterministic")
	}
}

func TestDistribution(t *testing.T) {
	nodes := []*Node{
		{ID: "node-1", Address: "node-1:8481", PartitionStart: 0, PartitionEnd: 6},
		{ID: "node-2", Address: "node-2:8481", PartitionStart: 6, PartitionEnd: 12},
		{ID: "node-3", Address: "node-3:8481", PartitionStart: 12, PartitionEnd: 16},
	}

	pm, err := NewPartitionMapper(16, nodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test distribution across many metric names
	nodeCounts := make(map[string]int)
	metrics := []string{
		"http_requests_total", "http_request_duration_seconds",
		"node_cpu_seconds_total", "node_memory_bytes",
		"disk_io_operations", "network_bytes_received",
		"process_open_fds", "go_goroutines",
		"up", "scrape_duration_seconds",
	}

	for _, metric := range metrics {
		node := pm.GetNodeForMetric(metric)
		nodeCounts[node.ID]++
	}

	// With 10 metrics and 3 nodes, we expect some distribution
	if len(nodeCounts) < 2 {
		t.Log("Warning: All metrics mapped to same node (unlikely but possible)")
	}

	t.Logf("Distribution: %v", nodeCounts)
}
