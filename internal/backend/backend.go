package backend

import (
	"context"
	"time"

	"github.com/prometheus/prometheus/prompb"
)

// Backend defines the interface for metric storage backends
type Backend interface {
	// Write sends time series samples to the backend
	Write(ctx context.Context, req *prompb.WriteRequest) error

	// Query executes a PromQL/MetricsQL query against the backend
	Query(ctx context.Context, query string, ts time.Time) (*QueryResult, error)

	// QueryRange executes a range query against the backend
	QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*QueryResult, error)

	// Name returns the backend type name
	Name() string

	// Health checks if the backend is healthy
	Health(ctx context.Context) error

	// Close closes the backend connection
	Close() error
}

// QueryResult represents a query result from the backend
type QueryResult struct {
	ResultType string      `json:"resultType"`
	Result     interface{} `json:"result"`
}

// Sample represents a single metric sample
type Sample struct {
	Labels    map[string]string
	Timestamp int64   // milliseconds
	Value     float64
}
