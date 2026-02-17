package capabilities

import (
	"github.com/promsketch/promsketch-dropin/internal/query/parser"
	"github.com/zzylol/metricsql"
)

// QueryCapability represents whether a query can be handled by sketches
type QueryCapability struct {
	CanHandleWithSketches bool
	Reason                string
	RequiredFunction      string
	RequiresQuantileArg   bool
}

// Registry determines if a query can be answered by PromSketch
type Registry struct {
	supportedFunctions map[string]bool
}

// NewRegistry creates a new capability registry
func NewRegistry() *Registry {
	return &Registry{
		supportedFunctions: map[string]bool{
			// USampling-backed functions
			"avg_over_time":    true,
			"sum_over_time":    true,
			"sum2_over_time":   true,
			"count_over_time":  true,
			"stddev_over_time": true,
			"stdvar_over_time": true,
			// EHKLL-backed functions
			"quantile_over_time": true,
			"min_over_time":      true,
			"max_over_time":      true,
			// EHUniv-backed functions
			"entropy_over_time":  true,
			"distinct_over_time": true,
			"l1_over_time":       true,
			"l2_over_time":       true,
		},
	}
}

// CanHandle determines if a query can be answered by sketches
func (r *Registry) CanHandle(queryInfo *parser.QueryInfo) *QueryCapability {
	// Only handle simple function queries for now
	// Complex queries (aggregations, binary ops, etc.) fall back to backend

	// Check if it's an aggregate function (sum, avg, count, etc.)
	// These can't be answered by sketches directly - need to fall back
	if queryInfo.IsAggregate {
		return &QueryCapability{
			CanHandleWithSketches: false,
			Reason:                "aggregate functions not supported by sketches",
		}
	}

	// Check if this is a supported function
	if queryInfo.FunctionName == "" {
		return &QueryCapability{
			CanHandleWithSketches: false,
			Reason:                "not a function query",
		}
	}

	// Check if function is in our supported list
	if !r.supportedFunctions[queryInfo.FunctionName] {
		return &QueryCapability{
			CanHandleWithSketches: false,
			Reason:                "function not supported: " + queryInfo.FunctionName,
		}
	}

	// Must be a range query (over time)
	if !queryInfo.IsRangeQuery() {
		return &QueryCapability{
			CanHandleWithSketches: false,
			Reason:                "not a range query",
		}
	}

	// Check if quantile_over_time has proper structure
	requiresQuantile := queryInfo.FunctionName == "quantile_over_time"

	return &QueryCapability{
		CanHandleWithSketches: true,
		RequiredFunction:      queryInfo.FunctionName,
		RequiresQuantileArg:   requiresQuantile,
	}
}

// AnalyzeQuery provides detailed analysis of query capabilities
func (r *Registry) AnalyzeQuery(expr metricsql.Expr) *QueryCapability {
	switch e := expr.(type) {
	case *metricsql.FuncExpr:
		// For function expressions, the function name is in e.Name
		// Check if it's a supported rollup function
		if r.supportedFunctions[e.Name] {
			// Check if first arg is a rollup expression (for validation)
			hasRollup := false
			if len(e.Args) > 0 {
				if _, ok := e.Args[0].(*metricsql.RollupExpr); ok {
					hasRollup = true
				}
			}

			if !hasRollup {
				return &QueryCapability{
					CanHandleWithSketches: false,
					Reason:                "rollup function requires time range",
				}
			}

			return &QueryCapability{
				CanHandleWithSketches: true,
				RequiredFunction:      e.Name,
				RequiresQuantileArg:   e.Name == "quantile_over_time",
			}
		}

		return &QueryCapability{
			CanHandleWithSketches: false,
			Reason:                "function not supported: " + e.Name,
		}

	case *metricsql.RollupExpr:
		// Standalone rollup expression (no function wrapper)
		// Can't be answered by sketches - need a function like avg_over_time
		return &QueryCapability{
			CanHandleWithSketches: false,
			Reason:                "rollup expression requires a function wrapper",
		}

	case *metricsql.AggrFuncExpr:
		// Aggregate functions require special handling
		// For now, we don't support them with sketches
		return &QueryCapability{
			CanHandleWithSketches: false,
			Reason:                "aggregate functions not supported",
		}

	case *metricsql.MetricExpr:
		// Raw metric query - can't be answered by sketches
		return &QueryCapability{
			CanHandleWithSketches: false,
			Reason:                "raw metric query, not a function",
		}

	default:
		return &QueryCapability{
			CanHandleWithSketches: false,
			Reason:                "unsupported expression type",
		}
	}
}

// GetSupportedFunctions returns the list of functions that sketches can handle
func (r *Registry) GetSupportedFunctions() []string {
	functions := make([]string, 0, len(r.supportedFunctions))
	for fn := range r.supportedFunctions {
		functions = append(functions, fn)
	}
	return functions
}

// IsSupportedFunction checks if a function is supported
func (r *Registry) IsSupportedFunction(funcName string) bool {
	return r.supportedFunctions[funcName]
}
