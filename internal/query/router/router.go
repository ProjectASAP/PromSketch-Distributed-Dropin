package router

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/promsketch/promsketch-dropin/internal/backend"
	"github.com/promsketch/promsketch-dropin/internal/metrics"
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
	metrics      *routerMetrics
}

// routerMetrics is the internal atomic state for query routing counters
type routerMetrics struct {
	sketchQueries   atomic.Int64
	backendQueries  atomic.Int64
	sketchHits      atomic.Int64
	sketchMisses    atomic.Int64
	parsingErrors   atomic.Int64
	executionErrors atomic.Int64
}

// RouterMetrics is a point-in-time snapshot of query routing metrics
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
		metrics:      &routerMetrics{},
	}
}

// Query executes an instant query
func (r *QueryRouter) Query(ctx context.Context, query string, ts time.Time) (*QueryResult, error) {
	startTime := time.Now()
	defer func() {
		metrics.QueryDurationSeconds.WithLabelValues("instant").Observe(time.Since(startTime).Seconds())
	}()

	// Parse the query
	queryInfo, err := r.parser.Parse(query)
	if err != nil {
		r.metrics.parsingErrors.Add(1)
		metrics.QueryErrorsTotal.WithLabelValues("instant").Inc()
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	// Check if we can handle this with sketches
	capability := r.capabilities.CanHandle(queryInfo)

	if capability.CanHandleWithSketches {
		r.metrics.sketchQueries.Add(1)
		metrics.QuerySourceTotal.WithLabelValues("sketch").Inc()

		// Use the query's range window for MinTime (e.g., avg_over_time(m[5m]) → [T-5m, T])
		maxTimeMilli := ts.UnixMilli()
		minTimeMilli := maxTimeMilli - queryInfo.Range
		if minTimeMilli >= maxTimeMilli {
			minTimeMilli = maxTimeMilli
		}

		// Try to execute with sketches
		result, err := r.executeWithSketches(queryInfo, minTimeMilli, maxTimeMilli, time.Now().UnixMilli())
		if err == nil && result != nil {
			r.metrics.sketchHits.Add(1)
			metrics.QuerySketchHitsTotal.Inc()
			return &QueryResult{
				Source:          "sketch",
				Data:            result,
				QueryInfo:       queryInfo,
				ExecutionTimeMs: float64(time.Since(startTime).Milliseconds()),
			}, nil
		}

		// Sketch miss - fall back to backend
		r.metrics.sketchMisses.Add(1)
		metrics.QuerySketchMissesTotal.Inc()
	}

	// Fall back to backend
	r.metrics.backendQueries.Add(1)
	metrics.QuerySourceTotal.WithLabelValues("backend").Inc()
	backendResult, err := r.backend.Query(ctx, query, ts)
	if err != nil {
		r.metrics.executionErrors.Add(1)
		metrics.QueryErrorsTotal.WithLabelValues("instant").Inc()
		return nil, fmt.Errorf("backend query failed: %w", err)
	}

	return &QueryResult{
		Source:          "backend",
		Data:            backendResult.Result,
		QueryInfo:       queryInfo,
		ExecutionTimeMs: float64(time.Since(startTime).Milliseconds()),
	}, nil
}

// QueryRange executes a range query
func (r *QueryRouter) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*QueryResult, error) {
	startExec := time.Now()
	defer func() {
		metrics.QueryDurationSeconds.WithLabelValues("range").Observe(time.Since(startExec).Seconds())
	}()

	// Parse the query
	queryInfo, err := r.parser.Parse(query)
	if err != nil {
		r.metrics.parsingErrors.Add(1)
		metrics.QueryErrorsTotal.WithLabelValues("range").Inc()
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	// Check if we can handle this with sketches
	capability := r.capabilities.CanHandle(queryInfo)

	if capability.CanHandleWithSketches {
		r.metrics.sketchQueries.Add(1)
		metrics.QuerySourceTotal.WithLabelValues("sketch").Inc()

		// Try to execute with sketches
		result, err := r.executeWithSketchesRange(queryInfo, start, end, step)
		if err == nil && result != nil {
			r.metrics.sketchHits.Add(1)
			metrics.QuerySketchHitsTotal.Inc()
			return &QueryResult{
				Source:          "sketch",
				Data:            result,
				QueryInfo:       queryInfo,
				ExecutionTimeMs: float64(time.Since(startExec).Milliseconds()),
			}, nil
		}

		// Sketch miss - fall back to backend
		r.metrics.sketchMisses.Add(1)
		metrics.QuerySketchMissesTotal.Inc()
	}

	// Fall back to backend
	r.metrics.backendQueries.Add(1)
	metrics.QuerySourceTotal.WithLabelValues("backend").Inc()
	backendResult, err := r.backend.QueryRange(ctx, query, start, end, step)
	if err != nil {
		r.metrics.executionErrors.Add(1)
		metrics.QueryErrorsTotal.WithLabelValues("range").Inc()
		return nil, fmt.Errorf("backend query failed: %w", err)
	}

	return &QueryResult{
		Source:          "backend",
		Data:            backendResult.Result,
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

	// Extract function arguments (e.g., quantile value)
	otherArgs := 0.0
	if queryInfo.FunctionName == "quantile_over_time" {
		otherArgs = queryInfo.QuantileValue()
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

	curTime := time.Now().UnixMilli()

	// For range queries, we need to evaluate at each step
	results := make([]interface{}, 0)

	// Extract function arguments outside the loop (they don't change per step)
	otherArgs := 0.0
	if queryInfo.FunctionName == "quantile_over_time" {
		otherArgs = queryInfo.QuantileValue()
	}

	for ts := start; ts.Before(end) || ts.Equal(end); ts = ts.Add(step) {
		evalMaxTime := ts.UnixMilli()
		evalMinTime := evalMaxTime - queryInfo.Range
		if evalMinTime >= evalMaxTime {
			evalMinTime = evalMaxTime
		}

		// Coverage should be checked against the actual rollup window for each step,
		// not the full query [start,end] range.
		if !r.storage.LookUp(lbls, queryInfo.FunctionName, evalMinTime, evalMaxTime) {
			continue
		}

		result, err := r.storage.Eval(queryInfo.FunctionName, lbls, otherArgs, evalMinTime, evalMaxTime, curTime)
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

// Metrics returns a point-in-time snapshot of the current router metrics
func (r *QueryRouter) Metrics() RouterMetrics {
	return RouterMetrics{
		SketchQueries:   r.metrics.sketchQueries.Load(),
		BackendQueries:  r.metrics.backendQueries.Load(),
		SketchHits:      r.metrics.sketchHits.Load(),
		SketchMisses:    r.metrics.sketchMisses.Load(),
		ParsingErrors:   r.metrics.parsingErrors.Load(),
		ExecutionErrors: r.metrics.executionErrors.Load(),
	}
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
