package merger

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	promlabels "github.com/prometheus/prometheus/model/labels"

	pb "github.com/promsketch/promsketch-dropin/api/psksketch/v1"
	"github.com/promsketch/promsketch-dropin/internal/backend"
	"github.com/promsketch/promsketch-dropin/internal/pskinsert/client"
	"github.com/promsketch/promsketch-dropin/internal/query/capabilities"
	"github.com/promsketch/promsketch-dropin/internal/query/parser"
)

// MergerMetrics tracks merger statistics
type MergerMetrics struct {
	SketchQueries  uint64
	BackendQueries uint64
	SketchHits     uint64
	SketchMisses   uint64
	MergeErrors    uint64
}

// QueryResult represents the result of a query
type QueryResult struct {
	Source          string      // "sketch" or "backend"
	Data            interface{}
	QueryInfo       *parser.QueryInfo
	ExecutionTimeMs float64
}

// SketchVector represents a sketch query result (parallel to promsketch.Vector)
type SketchSample struct {
	T int64
	F float64
}

// Merger fans out queries to all psksketch nodes and merges results
type Merger struct {
	pool         *client.Pool
	backend      backend.Backend
	capabilities *capabilities.Registry
	parser       *parser.Parser
	queryTimeout time.Duration
	metrics      MergerMetrics
	querySem     chan struct{} // semaphore for MaxConcurrentQueries
}

// NewMerger creates a new query merger
func NewMerger(
	pool *client.Pool,
	backend backend.Backend,
	capabilities *capabilities.Registry,
	parser *parser.Parser,
	queryTimeout time.Duration,
	maxConcurrentQueries int,
) *Merger {
	if maxConcurrentQueries <= 0 {
		maxConcurrentQueries = 100
	}
	return &Merger{
		pool:         pool,
		backend:      backend,
		capabilities: capabilities,
		parser:       parser,
		queryTimeout: queryTimeout,
		querySem:     make(chan struct{}, maxConcurrentQueries),
	}
}

// Query executes an instant query by fanning out to all psksketch nodes
func (m *Merger) Query(ctx context.Context, query string, ts time.Time) (*QueryResult, error) {
	// Acquire semaphore
	select {
	case m.querySem <- struct{}{}:
		defer func() { <-m.querySem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	startTime := time.Now()

	// Parse the query
	queryInfo, err := m.parser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	// Check if sketches can handle this query
	capability := m.capabilities.CanHandle(queryInfo)
	if !capability.CanHandleWithSketches {
		// Fall back to backend
		atomic.AddUint64(&m.metrics.BackendQueries, 1)
		result, err := m.backend.Query(ctx, query, ts)
		if err != nil {
			return nil, fmt.Errorf("backend query failed: %w", err)
		}
		return &QueryResult{
			Source:          "backend",
			Data:            result,
			QueryInfo:       queryInfo,
			ExecutionTimeMs: float64(time.Since(startTime).Milliseconds()),
		}, nil
	}

	atomic.AddUint64(&m.metrics.SketchQueries, 1)

	// Build labels from query
	lbls := buildLabelsFromQuery(queryInfo)
	pbLabels := promLabelsToPBLabels(lbls)

	// Extract function arguments (e.g., quantile value)
	otherArgs := 0.0
	if queryInfo.FunctionName == "quantile_over_time" {
		otherArgs = queryInfo.QuantileValue()
	}

	tsMilli := ts.UnixMilli()
	// Use the query's range window for MinTime (e.g., avg_over_time(m[5m]) → [T-5m, T])
	minTimeMilli := tsMilli - queryInfo.Range
	if minTimeMilli >= tsMilli {
		minTimeMilli = tsMilli // no range specified, use point query
	}

	// Fan-out to all psksketch nodes in parallel
	clients := m.pool.AllClients()
	type nodeResult struct {
		nodeID  string
		samples []*pb.Sample
		err     error
	}

	resultsCh := make(chan nodeResult, len(clients))

	queryCtx, queryCancel := context.WithTimeout(ctx, m.queryTimeout)
	defer queryCancel()

	for nodeID, c := range clients {
		go func(id string, client pb.SketchServiceClient) {
			req := &pb.EvalRequest{
				FuncName:  queryInfo.FunctionName,
				Labels:    pbLabels,
				OtherArgs: otherArgs,
				MinTime:   minTimeMilli,
				MaxTime:   tsMilli,
				CurTime:   time.Now().UnixMilli(),
			}

			resp, err := client.Eval(queryCtx, req)
			if err != nil {
				resultsCh <- nodeResult{nodeID: id, err: err}
				return
			}
			if resp.Error != "" {
				resultsCh <- nodeResult{nodeID: id, err: fmt.Errorf("%s", resp.Error)}
				return
			}
			resultsCh <- nodeResult{nodeID: id, samples: resp.Samples}
		}(nodeID, c)
	}

	// Collect results
	var allSamples []SketchSample
	var errs []error

	for i := 0; i < len(clients); i++ {
		res := <-resultsCh
		if res.err != nil {
			errs = append(errs, res.err)
			continue
		}
		for _, s := range res.samples {
			allSamples = append(allSamples, SketchSample{T: s.Timestamp, F: s.Value})
		}
	}

	// If we got results, return them
	if len(allSamples) > 0 {
		atomic.AddUint64(&m.metrics.SketchHits, 1)
		return &QueryResult{
			Source:          "sketch",
			Data:            allSamples,
			QueryInfo:       queryInfo,
			ExecutionTimeMs: float64(time.Since(startTime).Milliseconds()),
		}, nil
	}

	// No sketch results - fall back to backend
	atomic.AddUint64(&m.metrics.SketchMisses, 1)

	if len(errs) > 0 {
		log.Printf("All sketch nodes returned errors, falling back to backend: %v", errs[0])
	}

	atomic.AddUint64(&m.metrics.BackendQueries, 1)
	result, err := m.backend.Query(ctx, query, ts)
	if err != nil {
		return nil, fmt.Errorf("backend query failed: %w", err)
	}

	return &QueryResult{
		Source:          "backend",
		Data:            result,
		QueryInfo:       queryInfo,
		ExecutionTimeMs: float64(time.Since(startTime).Milliseconds()),
	}, nil
}

// QueryRange executes a range query by fanning out to all psksketch nodes
func (m *Merger) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*QueryResult, error) {
	// Acquire semaphore
	select {
	case m.querySem <- struct{}{}:
		defer func() { <-m.querySem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	startExec := time.Now()

	queryInfo, err := m.parser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	capability := m.capabilities.CanHandle(queryInfo)
	if !capability.CanHandleWithSketches {
		atomic.AddUint64(&m.metrics.BackendQueries, 1)
		result, err := m.backend.QueryRange(ctx, query, start, end, step)
		if err != nil {
			return nil, fmt.Errorf("backend range query failed: %w", err)
		}
		return &QueryResult{
			Source:          "backend",
			Data:            result,
			QueryInfo:       queryInfo,
			ExecutionTimeMs: float64(time.Since(startExec).Milliseconds()),
		}, nil
	}

	atomic.AddUint64(&m.metrics.SketchQueries, 1)

	// Build labels from query
	lbls := buildLabelsFromQuery(queryInfo)
	pbLabels := promLabelsToPBLabels(lbls)

	otherArgs := 0.0
	if queryInfo.FunctionName == "quantile_over_time" {
		otherArgs = queryInfo.QuantileValue()
	}

	// For range queries, evaluate at each step point
	// Fan out each step to all nodes
	clients := m.pool.AllClients()
	var allResults [][]SketchSample
	hasAnyResults := false

	for ts := start; ts.Before(end) || ts.Equal(end); ts = ts.Add(step) {
		type nodeResult struct {
			samples []*pb.Sample
			err     error
		}

		resultsCh := make(chan nodeResult, len(clients))
		queryCtx, queryCancel := context.WithTimeout(ctx, m.queryTimeout)

		for _, c := range clients {
			go func(client pb.SketchServiceClient) {
				evalMaxTime := ts.UnixMilli()
				evalMinTime := evalMaxTime - queryInfo.Range
				if evalMinTime >= evalMaxTime {
					evalMinTime = evalMaxTime
				}
				req := &pb.EvalRequest{
					FuncName:  queryInfo.FunctionName,
					Labels:    pbLabels,
					OtherArgs: otherArgs,
					MinTime:   evalMinTime,
					MaxTime:   evalMaxTime,
					CurTime:   time.Now().UnixMilli(),
				}
				resp, err := client.Eval(queryCtx, req)
				if err != nil {
					resultsCh <- nodeResult{err: err}
					return
				}
				resultsCh <- nodeResult{samples: resp.Samples}
			}(c)
		}

		var stepSamples []SketchSample
		for i := 0; i < len(clients); i++ {
			res := <-resultsCh
			if res.err != nil {
				continue
			}
			for _, s := range res.samples {
				stepSamples = append(stepSamples, SketchSample{T: s.Timestamp, F: s.Value})
			}
		}
		queryCancel()

		if len(stepSamples) > 0 {
			hasAnyResults = true
		}
		allResults = append(allResults, stepSamples)
	}

	if hasAnyResults {
		atomic.AddUint64(&m.metrics.SketchHits, 1)
		return &QueryResult{
			Source:          "sketch",
			Data:            allResults,
			QueryInfo:       queryInfo,
			ExecutionTimeMs: float64(time.Since(startExec).Milliseconds()),
		}, nil
	}

	// Fall back to backend
	atomic.AddUint64(&m.metrics.SketchMisses, 1)
	atomic.AddUint64(&m.metrics.BackendQueries, 1)
	result, err := m.backend.QueryRange(ctx, query, start, end, step)
	if err != nil {
		return nil, fmt.Errorf("backend range query failed: %w", err)
	}

	return &QueryResult{
		Source:          "backend",
		Data:            result,
		QueryInfo:       queryInfo,
		ExecutionTimeMs: float64(time.Since(startExec).Milliseconds()),
	}, nil
}

// Metrics returns the current merger metrics
func (m *Merger) Metrics() MergerMetrics {
	return MergerMetrics{
		SketchQueries:  atomic.LoadUint64(&m.metrics.SketchQueries),
		BackendQueries: atomic.LoadUint64(&m.metrics.BackendQueries),
		SketchHits:     atomic.LoadUint64(&m.metrics.SketchHits),
		SketchMisses:   atomic.LoadUint64(&m.metrics.SketchMisses),
		MergeErrors:    atomic.LoadUint64(&m.metrics.MergeErrors),
	}
}

// buildLabelsFromQuery constructs labels.Labels from QueryInfo
func buildLabelsFromQuery(queryInfo *parser.QueryInfo) promlabels.Labels {
	lblsBuilder := promlabels.NewBuilder(promlabels.EmptyLabels())

	if queryInfo.MetricName != "" {
		lblsBuilder.Set(promlabels.MetricName, queryInfo.MetricName)
	}

	for _, matcher := range queryInfo.LabelMatchers {
		if matcher.Type == parser.MatchEqual {
			lblsBuilder.Set(matcher.Name, matcher.Value)
		}
	}

	return lblsBuilder.Labels()
}

// promLabelsToPBLabels converts Prometheus labels to protobuf labels
func promLabelsToPBLabels(lbls promlabels.Labels) []*pb.Label {
	result := make([]*pb.Label, 0, len(lbls))
	for _, l := range lbls {
		result = append(result, &pb.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}
	return result
}

// Concurrency-safe wait group helper
var _ = sync.WaitGroup{}
