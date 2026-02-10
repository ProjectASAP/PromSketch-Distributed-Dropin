package api

import (
	"testing"

	"github.com/promsketch/promsketch-dropin/internal/query/parser"
)

func TestReconstructLabelsFromQuery(t *testing.T) {
	api := &QueryAPI{}

	tests := []struct {
		name     string
		query    string
		expected map[string]string
	}{
		{
			name:  "metric name only",
			query: "avg_over_time(http_requests_total[5m])",
			expected: map[string]string{
				"__name__": "http_requests_total",
			},
		},
		{
			name:  "metric with single label",
			query: `avg_over_time(http_requests_total{job="api"}[5m])`,
			expected: map[string]string{
				"__name__": "http_requests_total",
				"job":      "api",
			},
		},
		{
			name:  "metric with multiple labels",
			query: `sum_over_time(http_requests_total{job="api",status="200"}[5m])`,
			expected: map[string]string{
				"__name__": "http_requests_total",
				"job":      "api",
				"status":   "200",
			},
		},
		{
			name:  "quantile_over_time with labels",
			query: `quantile_over_time(0.95, http_duration_seconds{job="web",instance="localhost:8080"}[5m])`,
			expected: map[string]string{
				"__name__":  "http_duration_seconds",
				"job":       "web",
				"instance":  "localhost:8080",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the query
			p := parser.NewParser()
			queryInfo, err := p.Parse(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse query: %v", err)
			}

			// Reconstruct labels
			labels := api.reconstructLabelsFromQuery(queryInfo)

			// Check all expected labels are present
			for k, v := range tt.expected {
				if labels[k] != v {
					t.Errorf("Expected label %s=%s, got %s", k, v, labels[k])
				}
			}

			// Check no extra labels
			if len(labels) != len(tt.expected) {
				t.Errorf("Expected %d labels, got %d: %v", len(tt.expected), len(labels), labels)
			}
		})
	}
}

func TestReconstructLabelsFromQuery_Nil(t *testing.T) {
	api := &QueryAPI{}

	labels := api.reconstructLabelsFromQuery(nil)

	if len(labels) != 0 {
		t.Errorf("Expected empty labels for nil QueryInfo, got %d labels", len(labels))
	}
}
