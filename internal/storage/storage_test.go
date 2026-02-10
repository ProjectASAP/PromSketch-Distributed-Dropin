package storage

import (
	"testing"

	"github.com/prometheus/prometheus/model/labels"

	"github.com/promsketch/promsketch-dropin/internal/config"
)

func TestStorage_Creation(t *testing.T) {
	cfg := &config.SketchConfig{
		NumPartitions: 4,
		Targets: []config.SketchTarget{
			{Match: "http_requests_total"},
		},
		Defaults: config.SketchDefaults{
			EHParams: config.EHParams{
				WindowSize: 1800,
				K:          50,
				KllK:       256,
			},
		},
	}

	stor, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	if stor == nil {
		t.Fatal("Storage is nil")
	}

	metrics := stor.Metrics()
	if metrics.TotalSeries != 0 {
		t.Errorf("Expected 0 total series, got %d", metrics.TotalSeries)
	}
}

func TestStorage_InsertMatchingMetric(t *testing.T) {
	cfg := &config.SketchConfig{
		NumPartitions: 4,
		Targets: []config.SketchTarget{
			{Match: "http_requests_total"},
		},
		Defaults: config.SketchDefaults{
			EHParams: config.EHParams{
				WindowSize: 1800,
				K:          50,
				KllK:       256,
			},
		},
	}

	stor, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	lbls := labels.Labels{
		{Name: "__name__", Value: "http_requests_total"},
		{Name: "job", Value: "api"},
	}

	// Insert a sample
	err = stor.Insert(lbls, 1000, 42.0)
	if err != nil {
		t.Errorf("Failed to insert sample: %v", err)
	}

	metrics := stor.Metrics()
	if metrics.SamplesInserted == 0 {
		t.Error("Expected at least 1 sample inserted")
	}
}

func TestStorage_InsertNonMatchingMetric(t *testing.T) {
	cfg := &config.SketchConfig{
		NumPartitions: 4,
		Targets: []config.SketchTarget{
			{Match: "http_requests_total"},
		},
		Defaults: config.SketchDefaults{
			EHParams: config.EHParams{
				WindowSize: 1800,
				K:          50,
				KllK:       256,
			},
		},
	}

	stor, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	lbls := labels.Labels{
		{Name: "__name__", Value: "other_metric"},
		{Name: "job", Value: "api"},
	}

	// Insert a sample for non-matching metric
	err = stor.Insert(lbls, 1000, 42.0)
	if err != nil {
		t.Errorf("Insert should succeed for non-matching metric: %v", err)
	}

	metrics := stor.Metrics()
	// Sample should not be inserted into sketch
	if metrics.SamplesInserted > 0 {
		t.Error("Non-matching metric should not be inserted into sketch")
	}
}

func TestStorage_MultiplePartitions(t *testing.T) {
	cfg := &config.SketchConfig{
		NumPartitions: 8,
		Targets: []config.SketchTarget{
			{Match: "*"}, // Match all metrics
		},
		Defaults: config.SketchDefaults{
			EHParams: config.EHParams{
				WindowSize: 1800,
				K:          50,
				KllK:       256,
			},
		},
	}

	stor, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Insert samples for different metrics
	metrics := []string{
		"http_requests_total",
		"node_cpu_seconds_total",
		"memory_usage_bytes",
		"disk_io_operations",
	}

	for _, metricName := range metrics {
		lbls := labels.Labels{
			{Name: "__name__", Value: metricName},
		}
		err := stor.Insert(lbls, 1000, 42.0)
		if err != nil {
			t.Errorf("Failed to insert %s: %v", metricName, err)
		}
	}

	storageMetrics := stor.Metrics()
	if storageMetrics.SamplesInserted != uint64(len(metrics)) {
		t.Errorf("Expected %d samples inserted, got %d", len(metrics), storageMetrics.SamplesInserted)
	}
}
