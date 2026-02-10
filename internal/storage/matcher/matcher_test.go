package matcher

import (
	"testing"

	"github.com/prometheus/prometheus/model/labels"

	"github.com/promsketch/promsketch-dropin/internal/config"
)

func TestMatcher_ExactMatch(t *testing.T) {
	targets := []config.SketchTarget{
		{Match: "http_requests_total"},
		{Match: "node_cpu_seconds_total"},
	}

	m, err := NewMatcher(targets)
	if err != nil {
		t.Fatalf("Failed to create matcher: %v", err)
	}

	tests := []struct {
		name     string
		labels   labels.Labels
		expected bool
	}{
		{
			name: "exact match",
			labels: labels.Labels{
				{Name: "__name__", Value: "http_requests_total"},
				{Name: "job", Value: "api"},
			},
			expected: true,
		},
		{
			name: "no match",
			labels: labels.Labels{
				{Name: "__name__", Value: "other_metric"},
				{Name: "job", Value: "api"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, matched := m.Matches(tt.labels)
			if matched != tt.expected {
				t.Errorf("Expected match=%v, got %v", tt.expected, matched)
			}
		})
	}
}

func TestMatcher_RegexMatch(t *testing.T) {
	targets := []config.SketchTarget{
		{Match: "http_.*"},
		{Match: "node_cpu_.*"},
	}

	m, err := NewMatcher(targets)
	if err != nil {
		t.Fatalf("Failed to create matcher: %v", err)
	}

	tests := []struct {
		name     string
		labels   labels.Labels
		expected bool
	}{
		{
			name: "regex match http",
			labels: labels.Labels{
				{Name: "__name__", Value: "http_requests_total"},
			},
			expected: true,
		},
		{
			name: "regex match node_cpu",
			labels: labels.Labels{
				{Name: "__name__", Value: "node_cpu_seconds_total"},
			},
			expected: true,
		},
		{
			name: "no match",
			labels: labels.Labels{
				{Name: "__name__", Value: "memory_usage"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, matched := m.Matches(tt.labels)
			if matched != tt.expected {
				t.Errorf("Expected match=%v, got %v", tt.expected, matched)
			}
		})
	}
}

func TestMatcher_LabelMatchers(t *testing.T) {
	targets := []config.SketchTarget{
		{Match: `{job="api"}`},
		{Match: `{__name__=~"http_.*", status="200"}`},
	}

	m, err := NewMatcher(targets)
	if err != nil {
		t.Fatalf("Failed to create matcher: %v", err)
	}

	tests := []struct {
		name     string
		labels   labels.Labels
		expected bool
	}{
		{
			name: "job=api match",
			labels: labels.Labels{
				{Name: "__name__", Value: "any_metric"},
				{Name: "job", Value: "api"},
			},
			expected: true,
		},
		{
			name: "regex and exact label match",
			labels: labels.Labels{
				{Name: "__name__", Value: "http_requests_total"},
				{Name: "status", Value: "200"},
			},
			expected: true,
		},
		{
			name: "regex match but wrong status",
			labels: labels.Labels{
				{Name: "__name__", Value: "http_requests_total"},
				{Name: "status", Value: "500"},
			},
			expected: false,
		},
		{
			name: "wrong job",
			labels: labels.Labels{
				{Name: "__name__", Value: "any_metric"},
				{Name: "job", Value: "batch"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, matched := m.Matches(tt.labels)
			if matched != tt.expected {
				t.Errorf("Expected match=%v, got %v", tt.expected, matched)
			}
		})
	}
}

func TestMatcher_Wildcard(t *testing.T) {
	targets := []config.SketchTarget{
		{Match: "*"},
	}

	m, err := NewMatcher(targets)
	if err != nil {
		t.Fatalf("Failed to create matcher: %v", err)
	}

	// Wildcard should match everything
	labels := labels.Labels{
		{Name: "__name__", Value: "any_metric"},
		{Name: "job", Value: "any_job"},
	}

	_, matched := m.Matches(labels)
	if !matched {
		t.Error("Wildcard should match all metrics")
	}
}

func TestMatcher_MultipleTargets(t *testing.T) {
	targets := []config.SketchTarget{
		{Match: "http_requests_total"},
		{Match: "http_.*_duration"},
		{Match: `{job="api"}`},
	}

	m, err := NewMatcher(targets)
	if err != nil {
		t.Fatalf("Failed to create matcher: %v", err)
	}

	if m.TargetCount() != 3 {
		t.Errorf("Expected 3 targets, got %d", m.TargetCount())
	}
}

func TestMatcher_InvalidRegex(t *testing.T) {
	targets := []config.SketchTarget{
		{Match: "http_[invalid"},
	}

	_, err := NewMatcher(targets)
	if err == nil {
		t.Error("Expected error for invalid regex, got nil")
	}
}
