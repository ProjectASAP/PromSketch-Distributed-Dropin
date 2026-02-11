package parser

import (
	"testing"
)

func TestParser_BasicFunctions(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name             string
		query            string
		expectedFunc     string
		expectedMetric   string
		expectRangeQuery bool
	}{
		{
			name:             "avg_over_time",
			query:            `avg_over_time(http_requests_total[5m])`,
			expectedFunc:     "avg_over_time",
			expectedMetric:   "http_requests_total",
			expectRangeQuery: true,
		},
		{
			name:             "sum_over_time",
			query:            `sum_over_time(node_cpu_seconds_total[1h])`,
			expectedFunc:     "sum_over_time",
			expectedMetric:   "node_cpu_seconds_total",
			expectRangeQuery: true,
		},
		{
			name:             "count_over_time",
			query:            `count_over_time(http_errors_total[10m])`,
			expectedFunc:     "count_over_time",
			expectedMetric:   "http_errors_total",
			expectRangeQuery: true,
		},
		{
			name:             "quantile_over_time",
			query:            `quantile_over_time(0.99, http_duration_seconds[5m])`,
			expectedFunc:     "quantile_over_time",
			expectedMetric:   "http_duration_seconds",
			expectRangeQuery: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := p.Parse(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse query: %v", err)
			}

			if info.FunctionName != tt.expectedFunc {
				t.Errorf("Expected function=%s, got %s", tt.expectedFunc, info.FunctionName)
			}

			if tt.expectedMetric != "" && info.MetricName != tt.expectedMetric {
				t.Errorf("Expected metric=%s, got %s", tt.expectedMetric, info.MetricName)
			}

			if info.IsRangeQuery() != tt.expectRangeQuery {
				t.Errorf("Expected IsRangeQuery()=%v, got %v", tt.expectRangeQuery, info.IsRangeQuery())
			}
		})
	}
}

func TestParser_LabelMatchers(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name           string
		query          string
		expectedLabels int
		checkLabel     string
		checkValue     string
	}{
		{
			name:           "single_label",
			query:          `avg_over_time(http_requests_total{job="api"}[5m])`,
			expectedLabels: 2, // __name__ and job
			checkLabel:     "job",
			checkValue:     "api",
		},
		{
			name:           "multiple_labels",
			query:          `sum_over_time(http_requests_total{job="api",status="200"}[5m])`,
			expectedLabels: 3, // __name__, job, status
			checkLabel:     "status",
			checkValue:     "200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := p.Parse(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse query: %v", err)
			}

			if len(info.LabelMatchers) != tt.expectedLabels {
				t.Errorf("Expected %d label matchers, got %d", tt.expectedLabels, len(info.LabelMatchers))
			}

			// Check for specific label
			found := false
			for _, matcher := range info.LabelMatchers {
				if matcher.Name == tt.checkLabel && matcher.Value == tt.checkValue {
					found = true
					if matcher.Type != MatchEqual {
						t.Errorf("Expected MatchEqual for label %s, got %v", tt.checkLabel, matcher.Type)
					}
					break
				}
			}

			if !found {
				t.Errorf("Expected to find label %s=%s", tt.checkLabel, tt.checkValue)
			}
		})
	}
}

func TestParser_AggregateDetection(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name        string
		query       string
		isAggregate bool
		aggregateOp string
	}{
		{
			name:        "sum_aggregate",
			query:       `sum(http_requests_total)`,
			isAggregate: true,
			aggregateOp: "sum",
		},
		{
			name:        "avg_aggregate",
			query:       `avg(node_cpu_seconds_total)`,
			isAggregate: true,
			aggregateOp: "avg",
		},
		{
			name:        "not_aggregate",
			query:       `avg_over_time(http_requests_total[5m])`,
			isAggregate: false,
			aggregateOp: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := p.Parse(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse query: %v", err)
			}

			if info.IsAggregate != tt.isAggregate {
				t.Errorf("Expected IsAggregate=%v, got %v", tt.isAggregate, info.IsAggregate)
			}

			if info.AggregateOp != tt.aggregateOp {
				t.Errorf("Expected AggregateOp=%s, got %s", tt.aggregateOp, info.AggregateOp)
			}
		})
	}
}

func TestParser_QuantileExtraction(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name             string
		query            string
		expectedQuantile float64
	}{
		{
			name:             "quantile_0.99",
			query:            `quantile_over_time(0.99, http_duration_seconds[5m])`,
			expectedQuantile: 0.99,
		},
		{
			name:             "quantile_0.5",
			query:            `quantile_over_time(0.5, http_duration_seconds[5m])`,
			expectedQuantile: 0.5,
		},
		{
			name:             "quantile_0.95",
			query:            `quantile_over_time(0.95, node_cpu_seconds_total{job="node"}[10m])`,
			expectedQuantile: 0.95,
		},
		{
			name:             "non_quantile_no_args",
			query:            `avg_over_time(http_requests_total[5m])`,
			expectedQuantile: 0.5, // default from QuantileValue()
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := p.Parse(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse query: %v", err)
			}

			got := info.QuantileValue()
			if got != tt.expectedQuantile {
				t.Errorf("Expected QuantileValue()=%v, got %v", tt.expectedQuantile, got)
			}
		})
	}
}

func TestParser_InvalidQueries(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "empty_query",
			query: "",
		},
		{
			name:  "malformed_query",
			query: "this is not a valid query",
		},
		{
			name:  "unclosed_bracket",
			query: `avg_over_time(http_requests_total[5m`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.Parse(tt.query)
			if err == nil {
				t.Error("Expected parse error, got nil")
			}
		})
	}
}

func TestParser_GetMetricSelector(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name             string
		query            string
		expectedSelector string
	}{
		{
			name:             "with_metric_name",
			query:            `avg_over_time(http_requests_total[5m])`,
			expectedSelector: "http_requests_total",
		},
		{
			name:             "with_labels_only",
			query:            `avg_over_time({job="api"}[5m])`,
			expectedSelector: "{job}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := p.Parse(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse query: %v", err)
			}

			selector := info.GetMetricSelector()
			if selector != tt.expectedSelector {
				t.Errorf("Expected selector=%s, got %s", tt.expectedSelector, selector)
			}
		})
	}
}
