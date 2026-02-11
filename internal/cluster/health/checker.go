package health

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/promsketch/promsketch-dropin/internal/cluster/hash"
)

// NodeHealth tracks the health state of a single node
type NodeHealth struct {
	NodeID         string
	Address        string
	Healthy        bool
	CircuitBreaker *CircuitBreaker
	LastCheckTime  time.Time
	LastError      error
}

// HealthChecker periodically checks the health of psksketch nodes
type HealthChecker struct {
	nodes         map[string]*NodeHealth
	checkInterval time.Duration
	checkTimeout  time.Duration
	checkFn       func(ctx context.Context, address string) error // Pluggable health check function
	mu            sync.RWMutex
	cancel        context.CancelFunc
}

// HealthCheckerConfig holds health checker configuration
type HealthCheckerConfig struct {
	Interval         time.Duration
	Timeout          time.Duration
	FailureThreshold int
	CircuitTimeout   time.Duration
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(cfg *HealthCheckerConfig, checkFn func(ctx context.Context, address string) error) *HealthChecker {
	if cfg.Interval == 0 {
		cfg.Interval = 10 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.FailureThreshold == 0 {
		cfg.FailureThreshold = 3
	}
	if cfg.CircuitTimeout == 0 {
		cfg.CircuitTimeout = 30 * time.Second
	}

	return &HealthChecker{
		nodes:         make(map[string]*NodeHealth),
		checkInterval: cfg.Interval,
		checkTimeout:  cfg.Timeout,
		checkFn:       checkFn,
	}
}

// RegisterNodes registers nodes for health checking
func (hc *HealthChecker) RegisterNodes(nodes []*hash.Node, failureThreshold int, circuitTimeout time.Duration) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	for _, node := range nodes {
		hc.nodes[node.ID] = &NodeHealth{
			NodeID:         node.ID,
			Address:        node.Address,
			Healthy:        true, // Assume healthy initially
			CircuitBreaker: NewCircuitBreaker(failureThreshold, circuitTimeout),
		}
	}
}

// Start begins periodic health checking
func (hc *HealthChecker) Start(ctx context.Context) {
	ctx, hc.cancel = context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(hc.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				hc.checkAll(ctx)
			}
		}
	}()
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	if hc.cancel != nil {
		hc.cancel()
	}
}

// checkAll checks health of all registered nodes
func (hc *HealthChecker) checkAll(ctx context.Context) {
	hc.mu.RLock()
	nodesCopy := make([]*NodeHealth, 0, len(hc.nodes))
	for _, nh := range hc.nodes {
		nodesCopy = append(nodesCopy, nh)
	}
	hc.mu.RUnlock()

	var wg sync.WaitGroup
	for _, nh := range nodesCopy {
		wg.Add(1)
		go func(nh *NodeHealth) {
			defer wg.Done()
			hc.checkNode(ctx, nh)
		}(nh)
	}
	wg.Wait()
}

// checkNode checks health of a single node
func (hc *HealthChecker) checkNode(ctx context.Context, nh *NodeHealth) {
	checkCtx, cancel := context.WithTimeout(ctx, hc.checkTimeout)
	defer cancel()

	err := hc.checkFn(checkCtx, nh.Address)

	hc.mu.Lock()
	defer hc.mu.Unlock()

	nh.LastCheckTime = time.Now()
	nh.LastError = err

	if err != nil {
		nh.CircuitBreaker.RecordFailure()
		if nh.Healthy {
			log.Printf("Node %s (%s) is now unhealthy: %v", nh.NodeID, nh.Address, err)
			nh.Healthy = false
		}
	} else {
		nh.CircuitBreaker.RecordSuccess()
		if !nh.Healthy {
			log.Printf("Node %s (%s) is now healthy", nh.NodeID, nh.Address)
			nh.Healthy = true
		}
	}
}

// IsHealthy returns true if the node is considered healthy
func (hc *HealthChecker) IsHealthy(nodeID string) bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	nh, ok := hc.nodes[nodeID]
	if !ok {
		return false
	}
	return nh.Healthy && nh.CircuitBreaker.Allow()
}

// RecordFailure records a failure for a specific node (called on RPC errors)
func (hc *HealthChecker) RecordFailure(nodeID string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	nh, ok := hc.nodes[nodeID]
	if !ok {
		return
	}
	nh.CircuitBreaker.RecordFailure()
	if nh.CircuitBreaker.GetState() == StateOpen {
		nh.Healthy = false
	}
}

// RecordSuccess records a success for a specific node (called on successful RPCs)
func (hc *HealthChecker) RecordSuccess(nodeID string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	nh, ok := hc.nodes[nodeID]
	if !ok {
		return
	}
	nh.CircuitBreaker.RecordSuccess()
	nh.Healthy = true
}

// GetHealthStatus returns health status for all nodes
func (hc *HealthChecker) GetHealthStatus() map[string]bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	status := make(map[string]bool, len(hc.nodes))
	for id, nh := range hc.nodes {
		status[id] = nh.Healthy
	}
	return status
}
