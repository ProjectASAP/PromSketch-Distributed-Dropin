package prometheus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/gogo/protobuf/proto"

	"github.com/promsketch/promsketch-dropin/internal/backend"
	"github.com/promsketch/promsketch-dropin/internal/config"
)

// Client implements the Backend interface for Prometheus
type Client struct {
	config     *config.BackendConfig
	httpClient *http.Client
	baseURL    *url.URL
	writeURL   string
}

// NewClient creates a new Prometheus backend client
func NewClient(cfg *config.BackendConfig) (*Client, error) {
	baseURL, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid backend URL: %w", err)
	}

	writeURL := cfg.RemoteWriteURL
	if writeURL == "" {
		writeURL = cfg.URL + "/api/v1/write"
	}

	client := &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		baseURL:  baseURL,
		writeURL: writeURL,
	}

	return client, nil
}

// Write sends time series samples to Prometheus
func (c *Client) Write(ctx context.Context, req *prompb.WriteRequest) error {
	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal write request: %w", err)
	}

	compressed := snappy.Encode(nil, data)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.writeURL, bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("Content-Encoding", "snappy")
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	// Add authentication if configured
	if c.config.BasicAuth != nil {
		httpReq.SetBasicAuth(c.config.BasicAuth.Username, c.config.BasicAuth.Password)
	} else if c.config.BearerToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.config.BearerToken)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send write request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("write request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Query executes an instant query against Prometheus
func (c *Client) Query(ctx context.Context, query string, ts time.Time) (*backend.QueryResult, error) {
	queryURL := fmt.Sprintf("%s/api/v1/query", c.baseURL.String())

	params := url.Values{}
	params.Set("query", query)
	params.Set("time", fmt.Sprintf("%d", ts.Unix()))

	return c.executeQuery(ctx, queryURL, params)
}

// QueryRange executes a range query against Prometheus
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*backend.QueryResult, error) {
	queryURL := fmt.Sprintf("%s/api/v1/query_range", c.baseURL.String())

	params := url.Values{}
	params.Set("query", query)
	params.Set("start", fmt.Sprintf("%d", start.Unix()))
	params.Set("end", fmt.Sprintf("%d", end.Unix()))
	params.Set("step", fmt.Sprintf("%ds", int(step.Seconds())))

	return c.executeQuery(ctx, queryURL, params)
}

// prometheusResponse represents the Prometheus API response format
type prometheusResponse struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
	Error  string          `json:"error,omitempty"`
}

// prometheusData represents the data section of a Prometheus response
type prometheusData struct {
	ResultType string          `json:"resultType"`
	Result     json.RawMessage `json:"result"`
}

// executeQuery is a helper to execute HTTP queries
func (c *Client) executeQuery(ctx context.Context, queryURL string, params url.Values) (*backend.QueryResult, error) {
	fullURL := fmt.Sprintf("%s?%s", queryURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create query request: %w", err)
	}

	// Add authentication
	if c.config.BasicAuth != nil {
		req.SetBasicAuth(c.config.BasicAuth.Username, c.config.BasicAuth.Password)
	} else if c.config.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.BearerToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse Prometheus response
	var promResp prometheusResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %w", err)
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("query failed: %s", promResp.Error)
	}

	// Parse data section
	var data prometheusData
	if err := json.Unmarshal(promResp.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to parse response data: %w", err)
	}

	// Keep result as json.RawMessage to avoid re-parsing
	var result interface{}
	if err := json.Unmarshal(data.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse result: %w", err)
	}

	return &backend.QueryResult{
		ResultType: data.ResultType,
		Result:     result,
	}, nil
}

// Name returns the backend type name
func (c *Client) Name() string {
	return "prometheus"
}

// Health checks Prometheus health
func (c *Client) Health(ctx context.Context) error {
	healthURL := fmt.Sprintf("%s/-/healthy", c.baseURL.String())

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status code %d", resp.StatusCode)
	}

	return nil
}

// Close closes the client connection
func (c *Client) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
