package parser

import (
	"fmt"

	"github.com/zzylol/metricsql"
)

// QueryInfo contains parsed information about a query
type QueryInfo struct {
	// Original query string
	Query string

	// Parsed expression
	Expr metricsql.Expr

	// Function name (if it's a function call)
	FunctionName string

	// Numeric arguments to the function (e.g., quantile value for quantile_over_time)
	FuncArgs []float64

	// Metric selector (label matchers)
	MetricName string
	LabelMatchers []*LabelMatcher

	// Time range parameters
	Range int64 // Range duration in milliseconds

	// Whether this is an aggregate function
	IsAggregate bool
	AggregateOp string
}

// LabelMatcher represents a single label matcher
type LabelMatcher struct {
	Name  string
	Type  MatchType
	Value string
}

// MatchType represents the type of label match
type MatchType int

const (
	MatchEqual MatchType = iota
	MatchNotEqual
	MatchRegexp
	MatchNotRegexp
)

// Parser wraps MetricsQL parser
type Parser struct{}

// NewParser creates a new query parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses a PromQL/MetricsQL query
func (p *Parser) Parse(query string) (*QueryInfo, error) {
	expr, err := metricsql.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	info := &QueryInfo{
		Query: query,
		Expr:  expr,
	}

	// Extract query information
	p.extractInfo(expr, info)

	return info, nil
}

// extractInfo extracts relevant information from the parsed expression
func (p *Parser) extractInfo(expr metricsql.Expr, info *QueryInfo) {
	switch e := expr.(type) {
	case *metricsql.FuncExpr:
		info.FunctionName = e.Name

		// Extract numeric arguments (e.g., quantile value from quantile_over_time(0.99, ...))
		var rollupExpr *metricsql.RollupExpr
		for _, arg := range e.Args {
			switch a := arg.(type) {
			case *metricsql.NumberExpr:
				info.FuncArgs = append(info.FuncArgs, a.N)
			case *metricsql.RollupExpr:
				rollupExpr = a
			}
		}

		if rollupExpr != nil {
			// Extract window duration (using 0 as step since we don't know it yet)
			if rollupExpr.Window != nil {
				info.Range = rollupExpr.Window.Duration(0)
			}
			if me, ok := rollupExpr.Expr.(*metricsql.MetricExpr); ok {
				p.extractMetricInfo(me, info)
			}
		}

	case *metricsql.RollupExpr:
		// Standalone rollup expression (e.g., http_requests_total[5m])
		// No function name in this case
		if e.Window != nil {
			info.Range = e.Window.Duration(0)
		}
		if me, ok := e.Expr.(*metricsql.MetricExpr); ok {
			p.extractMetricInfo(me, info)
		}

	case *metricsql.AggrFuncExpr:
		info.IsAggregate = true
		info.AggregateOp = e.Name
		if len(e.Args) > 0 {
			p.extractInfo(e.Args[0], info)
		}

	case *metricsql.MetricExpr:
		p.extractMetricInfo(e, info)
	}
}

// extractMetricInfo extracts metric selector information
func (p *Parser) extractMetricInfo(me *metricsql.MetricExpr, info *QueryInfo) {
	// LabelFilterss is a slice of slices (or-delimited groups)
	// We'll process the first group for simplicity
	if len(me.LabelFilterss) == 0 {
		return
	}

	labelFilters := me.LabelFilterss[0]
	for _, lf := range labelFilters {
		if lf.Label == "__name__" {
			info.MetricName = lf.Value
		}

		matcher := &LabelMatcher{
			Name:  lf.Label,
			Value: lf.Value,
		}

		if lf.IsNegative {
			if lf.IsRegexp {
				matcher.Type = MatchNotRegexp
			} else {
				matcher.Type = MatchNotEqual
			}
		} else {
			if lf.IsRegexp {
				matcher.Type = MatchRegexp
			} else {
				matcher.Type = MatchEqual
			}
		}

		info.LabelMatchers = append(info.LabelMatchers, matcher)
	}
}

// QuantileValue returns the quantile parameter for quantile_over_time queries.
// Returns 0.5 (median) as default if not specified.
func (qi *QueryInfo) QuantileValue() float64 {
	if len(qi.FuncArgs) > 0 {
		return qi.FuncArgs[0]
	}
	return 0.5
}

// IsRangeQuery checks if the query is a range query (over time)
func (qi *QueryInfo) IsRangeQuery() bool {
	return qi.Range > 0
}

// GetMetricSelector returns a string representation of the metric selector
func (qi *QueryInfo) GetMetricSelector() string {
	if qi.MetricName != "" {
		return qi.MetricName
	}
	if len(qi.LabelMatchers) > 0 {
		return fmt.Sprintf("{%s}", qi.LabelMatchers[0].Name)
	}
	return ""
}
