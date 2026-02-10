package capabilities

import (
	"testing"

	"github.com/promsketch/promsketch-dropin/internal/query/parser"
)

func TestRegistry_CanHandle_SupportedFunctions(t *testing.T) {
	reg := NewRegistry()
	p := parser.NewParser()

	tests := []struct {
		name                  string
		query                 string
		shouldHandle          bool
		expectedFunction      string
		expectedRequiresQuant bool
	}{
		{
			name:             "avg_over_time",
			query:            `avg_over_time(http_requests_total[5m])`,
			shouldHandle:     true,
			expectedFunction: "avg_over_time",
		},
		{
			name:             "sum_over_time",
			query:            `sum_over_time(http_requests_total[5m])`,
			shouldHandle:     true,
			expectedFunction: "sum_over_time",
		},
		{
			name:             "count_over_time",
			query:            `count_over_time(http_requests_total[5m])`,
			shouldHandle:     true,
			expectedFunction: "count_over_time",
		},
		{
			name:                  "quantile_over_time",
			query:                 `quantile_over_time(0.95, http_requests_total[5m])`,
			shouldHandle:          true,
			expectedFunction:      "quantile_over_time",
			expectedRequiresQuant: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryInfo, err := p.Parse(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse query: %v", err)
			}

			capability := reg.CanHandle(queryInfo)

			if capability.CanHandleWithSketches != tt.shouldHandle {
				t.Errorf("Expected CanHandleWithSketches=%v, got %v (reason: %s)",
					tt.shouldHandle, capability.CanHandleWithSketches, capability.Reason)
			}

			if tt.shouldHandle {
				if capability.RequiredFunction != tt.expectedFunction {
					t.Errorf("Expected function=%s, got %s", tt.expectedFunction, capability.RequiredFunction)
				}
				if capability.RequiresQuantileArg != tt.expectedRequiresQuant {
					t.Errorf("Expected RequiresQuantileArg=%v, got %v",
						tt.expectedRequiresQuant, capability.RequiresQuantileArg)
				}
			}
		})
	}
}

func TestRegistry_CanHandle_UnsupportedCases(t *testing.T) {
	reg := NewRegistry()
	p := parser.NewParser()

	tests := []struct {
		name         string
		query        string
		shouldHandle bool
		reasonMatch  string
	}{
		{
			name:         "aggregate_function",
			query:        `sum(http_requests_total)`,
			shouldHandle: false,
			reasonMatch:  "aggregate",
		},
		{
			name:         "raw_metric",
			query:        `http_requests_total`,
			shouldHandle: false,
			reasonMatch:  "not a function",
		},
		{
			name:         "unsupported_function",
			query:        `rate(http_requests_total[5m])`,
			shouldHandle: false,
			reasonMatch:  "not supported",
		},
		{
			name:         "instant_query_no_range",
			query:        `http_requests_total`,
			shouldHandle: false,
			reasonMatch:  "not a function",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryInfo, err := p.Parse(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse query: %v", err)
			}

			capability := reg.CanHandle(queryInfo)

			if capability.CanHandleWithSketches != tt.shouldHandle {
				t.Errorf("Expected CanHandleWithSketches=%v, got %v",
					tt.shouldHandle, capability.CanHandleWithSketches)
			}

			if !tt.shouldHandle && capability.Reason == "" {
				t.Error("Expected a reason for not handling query, got empty string")
			}
		})
	}
}

func TestRegistry_GetSupportedFunctions(t *testing.T) {
	reg := NewRegistry()
	functions := reg.GetSupportedFunctions()

	expectedFunctions := map[string]bool{
		"avg_over_time":      true,
		"sum_over_time":      true,
		"count_over_time":    true,
		"quantile_over_time": true,
	}

	if len(functions) != len(expectedFunctions) {
		t.Errorf("Expected %d functions, got %d", len(expectedFunctions), len(functions))
	}

	for _, fn := range functions {
		if !expectedFunctions[fn] {
			t.Errorf("Unexpected function in supported list: %s", fn)
		}
	}
}

func TestRegistry_IsSupportedFunction(t *testing.T) {
	reg := NewRegistry()

	tests := []struct {
		function   string
		isSupported bool
	}{
		{"avg_over_time", true},
		{"sum_over_time", true},
		{"count_over_time", true},
		{"quantile_over_time", true},
		{"rate", false},
		{"increase", false},
		{"histogram_quantile", false},
	}

	for _, tt := range tests {
		t.Run(tt.function, func(t *testing.T) {
			result := reg.IsSupportedFunction(tt.function)
			if result != tt.isSupported {
				t.Errorf("Expected IsSupportedFunction(%s)=%v, got %v",
					tt.function, tt.isSupported, result)
			}
		})
	}
}
