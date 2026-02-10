package remotewrite

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
)

// Handler handles Prometheus remote write requests
type Handler struct {
	receiver Receiver
	metrics  *HandlerMetrics
}

// Receiver processes incoming write requests
type Receiver interface {
	Receive(req *prompb.WriteRequest) error
}

// HandlerMetrics tracks handler statistics
type HandlerMetrics struct {
	RequestsReceived  uint64
	RequestsFailed    uint64
	SamplesReceived   uint64
	BytesReceived     uint64
}

// NewHandler creates a new remote write handler
func NewHandler(receiver Receiver) *Handler {
	return &Handler{
		receiver: receiver,
		metrics:  &HandlerMetrics{},
	}
}

// ServeHTTP implements http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.metrics.RequestsReceived++

	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		h.metrics.RequestsFailed++
		return
	}

	// Read the request body
	compressed, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
		h.metrics.RequestsFailed++
		return
	}
	defer r.Body.Close()

	h.metrics.BytesReceived += uint64(len(compressed))

	// Decompress using snappy
	decompressed, err := snappy.Decode(nil, compressed)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to decompress request: %v", err), http.StatusBadRequest)
		h.metrics.RequestsFailed++
		return
	}

	// Unmarshal protobuf
	var req prompb.WriteRequest
	if err := proto.Unmarshal(decompressed, &req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to unmarshal request: %v", err), http.StatusBadRequest)
		h.metrics.RequestsFailed++
		return
	}

	// Count samples
	for _, ts := range req.Timeseries {
		h.metrics.SamplesReceived += uint64(len(ts.Samples))
	}

	// Process the request
	if err := h.receiver.Receive(&req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to process request: %v", err), http.StatusInternalServerError)
		h.metrics.RequestsFailed++
		return
	}

	// Return success
	w.WriteHeader(http.StatusNoContent)
}

// Metrics returns the current handler metrics
func (h *Handler) Metrics() HandlerMetrics {
	return *h.metrics
}
