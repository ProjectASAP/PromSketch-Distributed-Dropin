package router

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/promsketch/promsketch-dropin/internal/backend"
	"github.com/promsketch/promsketch-dropin/internal/promsketch"
	"github.com/promsketch/promsketch-dropin/internal/query/capabilities"
	"github.com/promsketch/promsketch-dropin/internal/query/parser"
	"github.com/promsketch/promsketch-dropin/internal/storage"
)

// QueryRouter routes queries to either sketches or backend
type QueryRouter struct {
	storage      *storage.Storage
	backend      backend.Backend
	parser       *parser.Parser
	capabilities *capabilities.Registry
	metrics      *RouterMetrics
}

// RouterMetrics tracks query routing decisions
type RouterMetrics struct {
	SketchQueries   int64
	BackendQueries  int64
	SketchHits      int64
	SketchMisses    int64
	ParsingErrors   int64
	ExecutionErrors int64
}

// QueryResult represents the result of a query
type QueryResult struct {
	// Whether the query was answered by sketches or backend
	Source string // "sketch" or "backend"

	// The actual result data
	Data interface{}

	// Query information for label reconstruction
	QueryInfo *parser.QueryInfo

	// Execution metrics
	ExecutionTimeMs float64
}

// NewRouter creates a new query router
func NewRouter(
	storage *storage.Storage,
	backend backend.Backend,
	parser *parser.Parser,
	capabilities *capabilities.Registry,
) *QueryRouter {
	return &QueryRouter{
		storage:      storage,
		backend:      backend,
		parser:       parser,
		capabilities: capabilities,
		metrics:      &RouterMetrics{},
	}
}

// Query executes an instant query
func (r *QueryRouter) Query(ctx context.Context, query string, ts time.Time) (*QueryResult, error) {
	startTime := time.Now()

	// Parse the query
	queryInfo, err := r.parser.Parse(query)
	if err != nil {
		r.metrics.ParsingErrors++
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	// Check if we can handle this with sketches
	capability := r.capabilities.CanHandle(queryInfo)

	if capability.CanHandleWithSketches {
		r.metrics.SketchQueries++

		// Try to execute with sketches
		result, err := r.executeWithSketches(queryInfo, ts.UnixMilli(), ts.UnixMilli(), time.Now().UnixMilli())
		if err == nil && result != nil {
			r.metrics.SketchHits++
			return &QueryResult{
				Source:          "sketch",
				Data:            result,
				QueryInfo:       queryInfo,
				ExecutionTimeMs: float64(time.Since(startTime).Milliseconds()),
			}, nil
		}

		// Sketch miss - fall back to backend
		r.metrics.SketchMisses++
	}

	// Fall back to backend
	r.metrics.BackendQueries++
	backendResult, err := r.backend.Query(ctx, query, ts)
	if err != nil {
		r.metrics.ExecutionErrors++
		return nil, fmt.Errorf("backend query failed: %w", err)
	}

	return &QueryResult{
		Source:          "backend",
		Data:            backendResult,
		QueryInfo:       queryInfo,
		ExecutionTimeMs: float64(time.Since(startTime).Milliseconds()),
	}, nil
}

// QueryRange executes a range query
func (r *QueryRouter) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*QueryResult, error) {
	startExec := time.Now()

	// Parse the query
	queryInfo, err := r.parser.Parse(query)
	if err != nil {
		r.metrics.ParsingErrors++
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	// Check if we can handle this with sketches
	capability := r.capabilities.CanHandle(queryInfo)

	if capability.CanHandleWithSketches {
		r.metrics.SketchQueries++

		// Try to execute with sketches
		result, err := r.executeWithSketchesRange(queryInfo, start, end, step)
		if err == nil && result != nil {
			r.metrics.SketchHits++
			return &QueryResult{
				Source:          "sketch",
				Data:            result,
				QueryInfo:       queryInfo,
				ExecutionTimeMs: float64(time.Since(startExec).Milliseconds()),
			}, nil
		}

		// Sketch miss - fall back to backend
		r.metrics.SketchMisses++
	}

	// Fall back to backend
	r.metrics.BackendQueries++
	backendResult, err := r.backend.QueryRange(ctx, query, start, end, step)
	if err != nil {
		r.metrics.ExecutionErrors++
		return nil, fmt.Errorf("backend query failed: %w", err)
	}

	return &QueryResult{
		Source:          "backend",
		Data:            backendResult,
		QueryInfo:       queryInfo,
		ExecutionTimeMs: float64(time.Since(startExec).Milliseconds()),
	}, nil
}

// executeWithSketches executes a query using sketches for an instant query
func (r *QueryRouter) executeWithSketches(queryInfo *parser.QueryInfo, mint, maxt, curTime int64) (promsketch.Vector, error) {
	// Build labels from query
	lbls := r.buildLabelsFromQuery(queryInfo)

	// Check if sketch can answer this query
	if !r.storage.LookUp(lbls, queryInfo.FunctionName, mint, maxt) {
		return nil, fmt.Errorf("sketch data not available")
	}

	// Extract quantile argument if needed
	otherArgs := 0.0
	if queryInfo.FunctionName == "quantile_over_time" {
		// For quantile_over_time, we need to extract the quantile value from the query
		// This would need to be parsed from the expression
		// For now, default to 0.5 (median)
		otherArgs = 0.5
	}

	// Execute query with sketches
	result, err := r.storage.Eval(queryInfo.FunctionName, lbls, otherArgs, mint, maxt, curTime)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// executeWithSketchesRange executes a range query using sketches
func (r *QueryRouter) executeWithSketchesRange(queryInfo *parser.QueryInfo, start, end time.Time, step time.Duration) (interface{}, error) {
	// Build labels from query
	lbls := r.buildLabelsFromQuery(queryInfo)

	mint := start.UnixMilli()
	maxt := end.UnixMilli()
	curTime := time.Now().UnixMilli()

	// Check if sketch can answer this query
	if !r.storage.LookUp(lbls, queryInfo.FunctionName, mint, maxt) {
		return nil, fmt.Errorf("sketch data not available for time range")
	}

	// For range queries, we need to evaluate at each step
	results := make([]interface{}, 0)

	for ts := start; ts.Before(end) || ts.Equal(end); ts = ts.Add(step) {
		tsMilli := ts.UnixMilli()

		// Extract quantile argument if needed
		otherArgs := 0.0
		if queryInfo.FunctionName == "quantile_over_time" {
			otherArgs = 0.5 // Default to median
		}

		result, err := r.storage.Eval(queryInfo.FunctionName, lbls, otherArgs, tsMilli, tsMilli, curTime)
		if err != nil {
			continue // Skip this timestamp on error
		}

		results = append(results, result)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no sketch results available")
	}

	return results, nil
}

// buildLabelsFromQuery constructs labels.Labels from QueryInfo
func (r *QueryRouter) buildLabelsFromQuery(queryInfo *parser.QueryInfo) labels.Labels {
	lblsBuilder := labels.NewBuilder(labels.EmptyLabels())

	// Add metric name if present
	if queryInfo.MetricName != "" {
		lblsBuilder.Set(labels.MetricName, queryInfo.MetricName)
	}

	// Add label matchers
	for _, matcher := range queryInfo.LabelMatchers {
		// Only add exact matches to the label set
		// For now, we'll use the first label matcher's value
		if matcher.Type == parser.MatchEqual {
			lblsBuilder.Set(matcher.Name, matcher.Value)
		}
	}

	return lblsBuilder.Labels()
}

// Metrics returns the current router metrics
func (r *QueryRouter) Metrics() *RouterMetrics {
	return r.metrics
}

// DecisionReason provides a human-readable reason for routing decisions
func (r *QueryRouter) DecisionReason(query string) (string, error) {
	queryInfo, err := r.parser.Parse(query)
	if err != nil {
		return "", fmt.Errorf("failed to parse query: %w", err)
	}

	capability := r.capabilities.CanHandle(queryInfo)
	if capability.CanHandleWithSketches {
		return fmt.Sprintf("Can route to sketches: function=%s", capability.RequiredFunction), nil
	}

	return fmt.Sprintf("Must route to backend: %s", capability.Reason), nil
}
