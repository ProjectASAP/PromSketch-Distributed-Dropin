package storage

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	promlabels "github.com/prometheus/prometheus/model/labels"
	sketchlabels "github.com/zzylol/prometheus-sketches/model/labels"

	"github.com/promsketch/promsketch-dropin/internal/config"
	"github.com/promsketch/promsketch-dropin/internal/promsketch"
	"github.com/promsketch/promsketch-dropin/internal/storage/matcher"
	"github.com/promsketch/promsketch-dropin/internal/storage/partition"
)

// Storage manages PromSketch instances across partitions
type Storage struct {
	config         *config.SketchConfig
	partitions     []*promsketch.PromSketches
	partitioner    *partition.Partitioner
	matcher        *matcher.Matcher
	metrics        *StorageMetrics
	memoryLimit    uint64 // parsed memory limit in bytes; 0 means unlimited
	memoryUsed     atomic.Uint64
	partitionStart int // inclusive; owned partition range start
	partitionEnd   int // exclusive; owned partition range end
	mu             sync.RWMutex
}

// StorageMetrics tracks storage statistics (all fields accessed atomically)
type StorageMetrics struct {
	TotalSeries        atomic.Uint64
	SketchedSeries     atomic.Uint64
	SamplesInserted    atomic.Uint64
	SketchInsertErrors atomic.Uint64
	MemoryRejections   atomic.Uint64
}

// parseMemoryLimit parses a human-readable memory limit string (e.g. "4GB", "512MB")
// into bytes. Returns 0 if the string is empty (unlimited).
func parseMemoryLimit(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}

	s = strings.ToUpper(s)

	// Check longest suffixes first to avoid "MB" matching "B"
	type suffixMult struct {
		suffix string
		mult   uint64
	}
	multipliers := []suffixMult{
		{"TB", 1024 * 1024 * 1024 * 1024},
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
		{"B", 1},
	}

	for _, sm := range multipliers {
		if strings.HasSuffix(s, sm.suffix) {
			numStr := strings.TrimSuffix(s, sm.suffix)
			numStr = strings.TrimSpace(numStr)
			val, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid memory limit %q: %w", s, err)
			}
			return uint64(val * float64(sm.mult)), nil
		}
	}

	// Try parsing as plain bytes
	val, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory limit %q: must end with B, KB, MB, GB, or TB", s)
	}
	return val, nil
}

// NewStorage creates a new storage manager
func NewStorage(cfg *config.SketchConfig) (*Storage, error) {
	// Create matcher from sketch targets
	m, err := matcher.NewMatcher(cfg.Targets)
	if err != nil {
		return nil, fmt.Errorf("failed to create matcher: %w", err)
	}

	// Parse memory limit
	memLimit, err := parseMemoryLimit(cfg.MemoryLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to parse memory_limit: %w", err)
	}

	// Determine owned partition range
	partStart := cfg.PartitionStart
	partEnd := cfg.PartitionEnd
	if partStart == 0 && partEnd == 0 {
		// Monolithic mode: own all partitions
		partEnd = cfg.NumPartitions
	}
	numOwned := partEnd - partStart

	// Only allocate partitions this node owns
	partitions := make([]*promsketch.PromSketches, numOwned)
	for i := 0; i < numOwned; i++ {
		partitions[i] = promsketch.NewPromSketches()
	}

	s := &Storage{
		config:         cfg,
		partitions:     partitions,
		partitioner:    partition.NewPartitioner(cfg.NumPartitions),
		matcher:        m,
		metrics:        &StorageMetrics{},
		memoryLimit:    memLimit,
		partitionStart: partStart,
		partitionEnd:   partEnd,
	}

	return s, nil
}

// Insert inserts a sample into the appropriate PromSketch instance
func (s *Storage) Insert(lbls promlabels.Labels, timestamp int64, value float64) error {
	// Check if this metric matches any sketch targets
	target, matches := s.matcher.Matches(lbls)
	if !matches {
		// Don't create sketch for this metric
		return nil
	}

	// Get partition for this metric
	partitionID := s.partitioner.GetPartition(lbls)
	if partitionID < s.partitionStart || partitionID >= s.partitionEnd {
		return fmt.Errorf("partition %d not owned by this node [%d, %d)", partitionID, s.partitionStart, s.partitionEnd)
	}
	localIdx := partitionID - s.partitionStart
	ps := s.partitions[localIdx]

	// Convert Prometheus labels to promsketch labels
	sketchLabels := convertLabels(lbls)

	// Check if sketch instance exists for this series
	// If not, create it based on the target configuration
	if !s.hasSketchInstance(ps, sketchLabels) {
		// Enforce memory limit before creating new sketch instances
		// Approximate ~64KB per sketch series (4 functions * ~16KB each)
		const estimatedBytesPerSeries = 64 * 1024
		if s.memoryLimit > 0 && s.memoryUsed.Load()+estimatedBytesPerSeries > s.memoryLimit {
			s.metrics.MemoryRejections.Add(1)
			return fmt.Errorf("memory limit exceeded (%d bytes used, limit %d bytes): rejecting new series", s.memoryUsed.Load(), s.memoryLimit)
		}
		if err := s.createSketchInstance(ps, sketchLabels, target); err != nil {
			s.metrics.SketchInsertErrors.Add(1)
			return fmt.Errorf("failed to create sketch instance: %w", err)
		}
		s.memoryUsed.Add(estimatedBytesPerSeries)
		s.metrics.SketchedSeries.Add(1)
	}

	// Insert sample into sketch
	// Convert timestamp to milliseconds (Prometheus uses milliseconds)
	err := ps.SketchInsert(sketchLabels, timestamp, value)
	if err != nil {
		s.metrics.SketchInsertErrors.Add(1)
		return fmt.Errorf("failed to insert into sketch: %w", err)
	}

	s.metrics.SamplesInserted.Add(1)
	return nil
}

// hasSketchInstance checks if a sketch instance exists for a label set
func (s *Storage) hasSketchInstance(ps *promsketch.PromSketches, lbls sketchlabels.Labels) bool {
	// We can check by attempting a lookup
	// For now, we'll use a simple heuristic: try to get coverage
	minTime, maxTime := ps.PrintCoverage(lbls, "avg_over_time")
	return minTime != -1 && maxTime != -1
}

// createSketchInstance creates sketch instances for a time series based on target config
func (s *Storage) createSketchInstance(ps *promsketch.PromSketches, lbls sketchlabels.Labels, target *config.SketchTarget) error {
	// Determine window size
	windowSize := s.config.Defaults.EHParams.WindowSize
	if target.EHParams != nil && target.EHParams.WindowSize > 0 {
		windowSize = target.EHParams.WindowSize
	}

	// Create sketch instances for common query functions
	// In production, this could be configurable per target
	functions := []string{
		"avg_over_time",
		"sum_over_time",
		"count_over_time",
		"quantile_over_time",
	}

	for _, funcName := range functions {
		err := ps.NewSketchCacheInstance(
			lbls,
			funcName,
			windowSize*1000, // Convert seconds to milliseconds
			100000,          // item_window_size (for sampling)
			1.0,             // value_scale
		)
		if err != nil {
			// Some functions might not be supported, that's okay
			continue
		}
	}

	return nil
}

// LookUp checks if a query can be answered by the sketches
func (s *Storage) LookUp(lbls promlabels.Labels, funcName string, mint, maxt int64) bool {
	partitionID := s.partitioner.GetPartition(lbls)
	if partitionID < s.partitionStart || partitionID >= s.partitionEnd {
		return false
	}
	localIdx := partitionID - s.partitionStart
	ps := s.partitions[localIdx]

	sketchLabels := convertLabels(lbls)
	return ps.LookUp(sketchLabels, funcName, mint, maxt)
}

// Eval executes a sketch query
func (s *Storage) Eval(funcName string, lbls promlabels.Labels, otherArgs float64, mint, maxt, curTime int64) (promsketch.Vector, error) {
	partitionID := s.partitioner.GetPartition(lbls)
	if partitionID < s.partitionStart || partitionID >= s.partitionEnd {
		return nil, fmt.Errorf("partition %d not owned by this node [%d, %d)", partitionID, s.partitionStart, s.partitionEnd)
	}
	localIdx := partitionID - s.partitionStart
	ps := s.partitions[localIdx]

	sketchLabels := convertLabels(lbls)
	result, annots := ps.Eval(funcName, sketchLabels, otherArgs, mint, maxt, curTime)
	if errs := annots.AsErrors(); len(errs) > 0 {
		return nil, fmt.Errorf("sketch eval failed for %s: %v", funcName, errs[0])
	}
	return result, nil
}

// convertLabels converts Prometheus labels to promsketch labels
func convertLabels(lbls promlabels.Labels) sketchlabels.Labels {
	// sketchlabels.Labels is an alias for prometheus labels from the promsketch library
	// We need to convert from prometheus/prometheus to zzylol/prometheus-sketches
	sketchLabels := make(sketchlabels.Labels, 0, len(lbls))
	for _, lbl := range lbls {
		sketchLabels = append(sketchLabels, sketchlabels.Label{
			Name:  lbl.Name,
			Value: lbl.Value,
		})
	}
	return sketchLabels
}

// MetricsSnapshot is a point-in-time copy of storage metrics (safe to pass around)
type MetricsSnapshot struct {
	TotalSeries        uint64
	SketchedSeries     uint64
	SamplesInserted    uint64
	SketchInsertErrors uint64
	MemoryRejections   uint64
}

// Metrics returns a point-in-time snapshot of the current storage metrics
func (s *Storage) Metrics() MetricsSnapshot {
	return MetricsSnapshot{
		TotalSeries:        s.metrics.TotalSeries.Load(),
		SketchedSeries:     s.metrics.SketchedSeries.Load(),
		SamplesInserted:    s.metrics.SamplesInserted.Load(),
		SketchInsertErrors: s.metrics.SketchInsertErrors.Load(),
		MemoryRejections:   s.metrics.MemoryRejections.Load(),
	}
}

// Stop stops all background workers
func (s *Storage) Stop() error {
	for _, ps := range s.partitions {
		ps.StopBackground()
	}
	return nil
}
