package storage

import (
	"fmt"
	"sync"

	promlabels "github.com/prometheus/prometheus/model/labels"
	sketchlabels "github.com/zzylol/prometheus-sketches/model/labels"

	"github.com/promsketch/promsketch-dropin/internal/config"
	"github.com/promsketch/promsketch-dropin/internal/promsketch"
	"github.com/promsketch/promsketch-dropin/internal/storage/matcher"
	"github.com/promsketch/promsketch-dropin/internal/storage/partition"
)

// Storage manages PromSketch instances across partitions
type Storage struct {
	config      *config.SketchConfig
	partitions  []*promsketch.PromSketches
	partitioner *partition.Partitioner
	matcher     *matcher.Matcher
	metrics     *StorageMetrics
	mu          sync.RWMutex
}

// StorageMetrics tracks storage statistics
type StorageMetrics struct {
	TotalSeries        uint64
	SketchedSeries     uint64
	SamplesInserted    uint64
	SketchInsertErrors uint64
}

// NewStorage creates a new storage manager
func NewStorage(cfg *config.SketchConfig) (*Storage, error) {
	// Create matcher from sketch targets
	m, err := matcher.NewMatcher(cfg.Targets)
	if err != nil {
		return nil, fmt.Errorf("failed to create matcher: %w", err)
	}

	// Create partitions
	partitions := make([]*promsketch.PromSketches, cfg.NumPartitions)
	for i := 0; i < cfg.NumPartitions; i++ {
		partitions[i] = promsketch.NewPromSketches()
	}

	s := &Storage{
		config:      cfg,
		partitions:  partitions,
		partitioner: partition.NewPartitioner(cfg.NumPartitions),
		matcher:     m,
		metrics:     &StorageMetrics{},
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
	ps := s.partitions[partitionID]

	// Convert Prometheus labels to promsketch labels
	sketchLabels := convertLabels(lbls)

	// Check if sketch instance exists for this series
	// If not, create it based on the target configuration
	if !s.hasSketchInstance(ps, sketchLabels) {
		if err := s.createSketchInstance(ps, sketchLabels, target); err != nil {
			s.metrics.SketchInsertErrors++
			return fmt.Errorf("failed to create sketch instance: %w", err)
		}
		s.metrics.SketchedSeries++
	}

	// Insert sample into sketch
	// Convert timestamp to milliseconds (Prometheus uses milliseconds)
	err := ps.SketchInsert(sketchLabels, timestamp, value)
	if err != nil {
		s.metrics.SketchInsertErrors++
		return fmt.Errorf("failed to insert into sketch: %w", err)
	}

	s.metrics.SamplesInserted++
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
	ps := s.partitions[partitionID]

	sketchLabels := convertLabels(lbls)
	return ps.LookUp(sketchLabels, funcName, mint, maxt)
}

// Eval executes a sketch query
func (s *Storage) Eval(funcName string, lbls promlabels.Labels, otherArgs float64, mint, maxt, curTime int64) (promsketch.Vector, error) {
	partitionID := s.partitioner.GetPartition(lbls)
	ps := s.partitions[partitionID]

	sketchLabels := convertLabels(lbls)
	result, _ := ps.Eval(funcName, sketchLabels, otherArgs, mint, maxt, curTime)
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

// Metrics returns the current storage metrics
func (s *Storage) Metrics() StorageMetrics {
	return *s.metrics
}

// Stop stops all background workers
func (s *Storage) Stop() error {
	for _, ps := range s.partitions {
		ps.StopBackground()
	}
	return nil
}
