package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/promsketch/promsketch-dropin/internal/promsketch"
	"github.com/promsketch/promsketch-dropin/internal/query/parser"
	"github.com/promsketch/promsketch-dropin/internal/query/router"
)

// QueryAPI provides Prometheus-compatible query endpoints
type QueryAPI struct {
	router  *router.QueryRouter
	metrics *APIMetrics
}

// APIMetrics tracks API request metrics
type APIMetrics struct {
	QueryRequests      int64
	QueryRangeRequests int64
	QueryErrors        int64
	QueryRangeErrors   int64
}

// NewQueryAPI creates a new query API handler
func NewQueryAPI(r *router.QueryRouter) *QueryAPI {
	return &QueryAPI{
		router:  r,
		metrics: &APIMetrics{},
	}
}

// PrometheusResponse represents the Prometheus API response format
type PrometheusResponse struct {
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	ErrorType string      `json:"errorType,omitempty"`
	Error     string      `json:"error,omitempty"`
	Warnings  []string    `json:"warnings,omitempty"`
}

// QueryData represents instant query response data
type QueryData struct {
	ResultType string        `json:"resultType"`
	Result     []QueryResult `json:"result"`
}

// QueryRangeData represents range query response data
type QueryRangeData struct {
	ResultType string             `json:"resultType"`
	Result     []QueryRangeResult `json:"result"`
}

// QueryResult represents a single instant query result
type QueryResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"` // [timestamp, value]
}

// QueryRangeResult represents a single range query result
type QueryRangeResult struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"` // [[timestamp, value], ...]
}

// ServeHTTP handles both GET and POST requests
func (api *QueryAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Route based on path
	switch r.URL.Path {
	case "/api/v1/query":
		api.handleQuery(w, r)
	case "/api/v1/query_range":
		api.handleQueryRange(w, r)
	default:
		api.sendError(w, http.StatusNotFound, "not_found", "endpoint not found")
	}
}

// handleQuery handles instant queries
func (api *QueryAPI) handleQuery(w http.ResponseWriter, r *http.Request) {
	api.metrics.QueryRequests++

	// Parse query parameters
	query := r.FormValue("query")
	if query == "" {
		api.metrics.QueryErrors++
		api.sendError(w, http.StatusBadRequest, "bad_data", "query parameter is required")
		return
	}

	// Parse time parameter (optional, defaults to now)
	var ts time.Time
	timeParam := r.FormValue("time")
	if timeParam == "" {
		ts = time.Now()
	} else {
		// Try parsing as Unix timestamp (float)
		timeFloat, err := strconv.ParseFloat(timeParam, 64)
		if err != nil {
			api.metrics.QueryErrors++
			api.sendError(w, http.StatusBadRequest, "bad_data", "invalid time parameter")
			return
		}
		ts = time.Unix(int64(timeFloat), 0)
	}

	// Execute query
	result, err := api.router.Query(r.Context(), query, ts)
	if err != nil {
		api.metrics.QueryErrors++
		api.sendError(w, http.StatusInternalServerError, "execution", err.Error())
		return
	}

	// Convert result to Prometheus format
	data := api.convertToQueryData(result)

	// Send response
	api.sendSuccess(w, data)
}

// handleQueryRange handles range queries
func (api *QueryAPI) handleQueryRange(w http.ResponseWriter, r *http.Request) {
	api.metrics.QueryRangeRequests++

	// Parse query parameters
	query := r.FormValue("query")
	if query == "" {
		api.metrics.QueryRangeErrors++
		api.sendError(w, http.StatusBadRequest, "bad_data", "query parameter is required")
		return
	}

	startParam := r.FormValue("start")
	endParam := r.FormValue("end")
	stepParam := r.FormValue("step")

	if startParam == "" || endParam == "" || stepParam == "" {
		api.metrics.QueryRangeErrors++
		api.sendError(w, http.StatusBadRequest, "bad_data", "start, end, and step parameters are required")
		return
	}

	// Parse start time
	startFloat, err := strconv.ParseFloat(startParam, 64)
	if err != nil {
		api.metrics.QueryRangeErrors++
		api.sendError(w, http.StatusBadRequest, "bad_data", "invalid start parameter")
		return
	}
	start := time.Unix(int64(startFloat), 0)

	// Parse end time
	endFloat, err := strconv.ParseFloat(endParam, 64)
	if err != nil {
		api.metrics.QueryRangeErrors++
		api.sendError(w, http.StatusBadRequest, "bad_data", "invalid end parameter")
		return
	}
	end := time.Unix(int64(endFloat), 0)

	// Parse step duration
	step, err := parseDuration(stepParam)
	if err != nil {
		api.metrics.QueryRangeErrors++
		api.sendError(w, http.StatusBadRequest, "bad_data", "invalid step parameter: "+err.Error())
		return
	}

	// Execute range query
	result, err := api.router.QueryRange(r.Context(), query, start, end, step)
	if err != nil {
		api.metrics.QueryRangeErrors++
		api.sendError(w, http.StatusInternalServerError, "execution", err.Error())
		return
	}

	// Convert result to Prometheus format
	data := api.convertToQueryRangeData(result, start, end, step)

	// Send response
	api.sendSuccess(w, data)
}

// convertToQueryData converts router result to Prometheus instant query format
func (api *QueryAPI) convertToQueryData(result *router.QueryResult) *QueryData {
	data := &QueryData{
		ResultType: "vector",
		Result:     make([]QueryResult, 0),
	}

	// Handle sketch results
	if result.Source == "sketch" {
		if vec, ok := result.Data.(promsketch.Vector); ok {
			// Reconstruct labels from query
			labels := api.reconstructLabelsFromQuery(result.QueryInfo)

			for i := range vec {
				sample := vec[i]
				queryResult := QueryResult{
					Metric: labels,
					Value:  []interface{}{float64(sample.T) / 1000.0, fmt.Sprintf("%f", sample.F)},
				}

				data.Result = append(data.Result, queryResult)
			}
		}
	} else {
		// Handle backend results - already in Prometheus format
		if resultArray, ok := result.Data.([]interface{}); ok {
			for _, item := range resultArray {
				if itemMap, ok := item.(map[string]interface{}); ok {
					// Extract metric labels
					metric := make(map[string]string)
					if metricData, ok := itemMap["metric"].(map[string]interface{}); ok {
						for k, v := range metricData {
							if strVal, ok := v.(string); ok {
								metric[k] = strVal
							}
						}
					}

					// Extract value [timestamp, value]
					var value []interface{}
					if valueData, ok := itemMap["value"].([]interface{}); ok {
						value = valueData
					}

					queryResult := QueryResult{
						Metric: metric,
						Value:  value,
					}
					data.Result = append(data.Result, queryResult)
				}
			}
		}
	}

	return data
}

// convertToQueryRangeData converts router result to Prometheus range query format
func (api *QueryAPI) convertToQueryRangeData(result *router.QueryResult, start, end time.Time, step time.Duration) *QueryRangeData {
	data := &QueryRangeData{
		ResultType: "matrix",
		Result:     make([]QueryRangeResult, 0),
	}

	// Handle sketch results
	if result.Source == "sketch" {
		// Range query results are a slice of vectors (one per timestamp)
		if results, ok := result.Data.([]interface{}); ok {
			// Reconstruct labels from query
			labels := api.reconstructLabelsFromQuery(result.QueryInfo)

			rangeResult := &QueryRangeResult{
				Metric: labels,
				Values: make([][]interface{}, 0),
			}

			for _, res := range results {
				if vec, ok := res.(promsketch.Vector); ok {
					for i := range vec {
						sample := vec[i]

						// Add sample value
						rangeResult.Values = append(rangeResult.Values, []interface{}{
							float64(sample.T) / 1000.0,
							fmt.Sprintf("%f", sample.F),
						})
					}
				}
			}

			if len(rangeResult.Values) > 0 {
				data.Result = append(data.Result, *rangeResult)
			}
		}
	} else {
		// Handle backend results - already in Prometheus format
		if resultArray, ok := result.Data.([]interface{}); ok {
			for _, item := range resultArray {
				if itemMap, ok := item.(map[string]interface{}); ok {
					// Extract metric labels
					metric := make(map[string]string)
					if metricData, ok := itemMap["metric"].(map[string]interface{}); ok {
						for k, v := range metricData {
							if strVal, ok := v.(string); ok {
								metric[k] = strVal
							}
						}
					}

					// Extract values [[timestamp, value], ...]
					var values [][]interface{}
					if valuesData, ok := itemMap["values"].([]interface{}); ok {
						for _, v := range valuesData {
							if valueArray, ok := v.([]interface{}); ok {
								values = append(values, valueArray)
							}
						}
					}

					rangeResult := QueryRangeResult{
						Metric: metric,
						Values: values,
					}
					data.Result = append(data.Result, rangeResult)
				}
			}
		}
	}

	return data
}

// sendSuccess sends a successful response
func (api *QueryAPI) sendSuccess(w http.ResponseWriter, data interface{}) {
	response := PrometheusResponse{
		Status: "success",
		Data:   data,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// sendError sends an error response
func (api *QueryAPI) sendError(w http.ResponseWriter, statusCode int, errorType, errorMsg string) {
	response := PrometheusResponse{
		Status:    "error",
		ErrorType: errorType,
		Error:     errorMsg,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// parseDuration parses a Prometheus-style duration string
// Supports: s (seconds), m (minutes), h (hours), d (days), w (weeks), y (years)
func parseDuration(s string) (time.Duration, error) {
	// First try parsing as a number (seconds)
	if num, err := strconv.ParseFloat(s, 64); err == nil {
		return time.Duration(num * float64(time.Second)), nil
	}

	// Try parsing as Go duration
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// Parse Prometheus-style duration
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	value, err := strconv.ParseFloat(s[:len(s)-1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %s", s)
	}

	unit := s[len(s)-1:]
	var multiplier float64
	switch unit {
	case "s":
		multiplier = 1
	case "m":
		multiplier = 60
	case "h":
		multiplier = 3600
	case "d":
		multiplier = 86400
	case "w":
		multiplier = 604800
	case "y":
		multiplier = 31536000
	default:
		return 0, fmt.Errorf("unknown duration unit: %s", unit)
	}

	return time.Duration(value * multiplier * float64(time.Second)), nil
}

// reconstructLabelsFromQuery builds a label map from QueryInfo
func (api *QueryAPI) reconstructLabelsFromQuery(queryInfo *parser.QueryInfo) map[string]string {
	if queryInfo == nil {
		return make(map[string]string)
	}

	labels := make(map[string]string)

	// Add metric name if present
	if queryInfo.MetricName != "" {
		labels["__name__"] = queryInfo.MetricName
	}

	// Add label matchers (only exact matches)
	for _, matcher := range queryInfo.LabelMatchers {
		if matcher.Type == parser.MatchEqual {
			labels[matcher.Name] = matcher.Value
		}
	}

	return labels
}

// Metrics returns the current API metrics
func (api *QueryAPI) Metrics() *APIMetrics {
	return api.metrics
}
