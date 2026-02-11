package router

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	promlabels "github.com/prometheus/prometheus/model/labels"

	pb "github.com/promsketch/promsketch-dropin/api/psksketch/v1"
	"github.com/promsketch/promsketch-dropin/internal/cluster/hash"
	"github.com/promsketch/promsketch-dropin/internal/cluster/health"
	"github.com/promsketch/promsketch-dropin/internal/pskinsert/client"
)

// RouterMetrics tracks routing statistics
type RouterMetrics struct {
	SamplesRouted   uint64
	SamplesFailed   uint64
	RPCsSent        uint64
	RPCsFailed      uint64
}

// Router routes incoming samples to the appropriate psksketch nodes
type Router struct {
	partitioner   *hash.PartitionMapper
	pool          *client.Pool
	healthChecker *health.HealthChecker
	replicaCount  int
	metrics       RouterMetrics
	insertTimeout time.Duration
}

// NewRouter creates a new insert router
func NewRouter(
	partitioner *hash.PartitionMapper,
	pool *client.Pool,
	healthChecker *health.HealthChecker,
	replicaCount int,
) *Router {
	return &Router{
		partitioner:   partitioner,
		pool:          pool,
		healthChecker: healthChecker,
		replicaCount:  replicaCount,
		insertTimeout: 5 * time.Second,
	}
}

// Insert routes a single sample to the appropriate psksketch node(s)
func (r *Router) Insert(lbls promlabels.Labels, timestamp int64, value float64) error {
	metricName := lbls.Get(promlabels.MetricName)
	partitionID := r.partitioner.GetPartitionID(metricName)
	nodes := r.partitioner.GetNodesForPartition(partitionID, r.replicaCount)

	if len(nodes) == 0 {
		atomic.AddUint64(&r.metrics.SamplesFailed, 1)
		return fmt.Errorf("no nodes available for partition %d", partitionID)
	}

	// Convert labels to protobuf format
	pbLabels := promLabelsToPBLabels(lbls)

	req := &pb.InsertRequest{
		Labels:    pbLabels,
		Timestamp: timestamp,
		Value:     value,
	}

	// Send to all nodes (primary + replicas) in parallel
	var wg sync.WaitGroup
	var errsMu sync.Mutex
	var errs []error
	successCount := 0

	for _, node := range nodes {
		if !r.healthChecker.IsHealthy(node.ID) {
			continue
		}

		client, ok := r.pool.GetClient(node.ID)
		if !ok {
			continue
		}

		wg.Add(1)
		go func(nodeID string, c pb.SketchServiceClient) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), r.insertTimeout)
			defer cancel()

			atomic.AddUint64(&r.metrics.RPCsSent, 1)
			resp, err := c.Insert(ctx, req)
			if err != nil {
				atomic.AddUint64(&r.metrics.RPCsFailed, 1)
				r.healthChecker.RecordFailure(nodeID)
				errsMu.Lock()
				errs = append(errs, fmt.Errorf("node %s: %w", nodeID, err))
				errsMu.Unlock()
				return
			}

			if !resp.Success {
				errsMu.Lock()
				errs = append(errs, fmt.Errorf("node %s: %s", nodeID, resp.Error))
				errsMu.Unlock()
				return
			}

			r.healthChecker.RecordSuccess(nodeID)
			errsMu.Lock()
			successCount++
			errsMu.Unlock()
		}(node.ID, client)
	}

	wg.Wait()

	// Succeed if at least one node accepted (write quorum = 1)
	if successCount > 0 {
		atomic.AddUint64(&r.metrics.SamplesRouted, 1)
		return nil
	}

	atomic.AddUint64(&r.metrics.SamplesFailed, 1)
	if len(errs) > 0 {
		return fmt.Errorf("all nodes failed for partition %d: %v", partitionID, errs[0])
	}
	return fmt.Errorf("no healthy nodes for partition %d", partitionID)
}

// BatchInsert routes a batch of time series to the appropriate psksketch nodes.
// Groups samples by target node to minimize RPC calls.
func (r *Router) BatchInsert(timeSeries []pb.TimeSeries) error {
	// Group time series by target node
	type nodeTimeSeries struct {
		nodeID string
		series []*pb.TimeSeries
	}

	nodeGroups := make(map[string]*pb.BatchInsertRequest)

	for i := range timeSeries {
		ts := &timeSeries[i]
		metricName := ""
		for _, l := range ts.Labels {
			if l.Name == "__name__" {
				metricName = l.Value
				break
			}
		}

		nodes := r.partitioner.GetNodesForMetric(metricName, r.replicaCount)
		for _, node := range nodes {
			if !r.healthChecker.IsHealthy(node.ID) {
				continue
			}
			if _, ok := nodeGroups[node.ID]; !ok {
				nodeGroups[node.ID] = &pb.BatchInsertRequest{}
			}
			nodeGroups[node.ID].TimeSeries = append(nodeGroups[node.ID].TimeSeries, ts)
		}
	}

	// Send batches to each node in parallel
	var wg sync.WaitGroup
	var errsMu sync.Mutex
	var errs []error

	for nodeID, batch := range nodeGroups {
		client, ok := r.pool.GetClient(nodeID)
		if !ok {
			continue
		}

		wg.Add(1)
		go func(id string, c pb.SketchServiceClient, b *pb.BatchInsertRequest) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), r.insertTimeout*2)
			defer cancel()

			atomic.AddUint64(&r.metrics.RPCsSent, 1)
			resp, err := c.BatchInsert(ctx, b)
			if err != nil {
				atomic.AddUint64(&r.metrics.RPCsFailed, 1)
				r.healthChecker.RecordFailure(id)
				errsMu.Lock()
				errs = append(errs, fmt.Errorf("node %s: %w", id, err))
				errsMu.Unlock()
				return
			}

			r.healthChecker.RecordSuccess(id)
			atomic.AddUint64(&r.metrics.SamplesRouted, uint64(resp.Inserted))
			if resp.Failed > 0 {
				log.Printf("Node %s: %d samples failed to insert", id, resp.Failed)
			}
		}(nodeID, client, batch)
	}

	wg.Wait()

	if len(errs) > 0 && len(nodeGroups) == len(errs) {
		return fmt.Errorf("all nodes failed: %v", errs[0])
	}

	return nil
}

// Metrics returns the current router metrics
func (r *Router) Metrics() RouterMetrics {
	return RouterMetrics{
		SamplesRouted: atomic.LoadUint64(&r.metrics.SamplesRouted),
		SamplesFailed: atomic.LoadUint64(&r.metrics.SamplesFailed),
		RPCsSent:      atomic.LoadUint64(&r.metrics.RPCsSent),
		RPCsFailed:    atomic.LoadUint64(&r.metrics.RPCsFailed),
	}
}

// promLabelsToPBLabels converts Prometheus labels to protobuf labels
func promLabelsToPBLabels(lbls promlabels.Labels) []*pb.Label {
	result := make([]*pb.Label, 0, len(lbls))
	for _, l := range lbls {
		result = append(result, &pb.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}
	return result
}
