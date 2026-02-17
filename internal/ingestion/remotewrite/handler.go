package remotewrite

import (
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"

	"github.com/promsketch/promsketch-dropin/internal/metrics"
)

// Handler handles Prometheus remote write requests
type Handler struct {
	receiver Receiver
	metrics  *handlerMetrics
}

// Receiver processes incoming write requests
type Receiver interface {
	Receive(req *prompb.WriteRequest) error
}

// handlerMetrics is the internal atomic state for handler counters
type handlerMetrics struct {
	requestsReceived atomic.Uint64
	requestsFailed   atomic.Uint64
	samplesReceived  atomic.Uint64
	bytesReceived    atomic.Uint64
}

// HandlerMetrics is a point-in-time snapshot of handler statistics
type HandlerMetrics struct {
	RequestsReceived uint64
	RequestsFailed   uint64
	SamplesReceived  uint64
	BytesReceived    uint64
}

// NewHandler creates a new remote write handler
func NewHandler(receiver Receiver) *Handler {
	return &Handler{
		receiver: receiver,
		metrics:  &handlerMetrics{},
	}
}

// ServeHTTP implements http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.metrics.requestsReceived.Add(1)
	metrics.RemoteWriteRequestsTotal.Inc()

	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		h.metrics.requestsFailed.Add(1)
		metrics.RemoteWriteRequestFailuresTotal.Inc()
		return
	}

	// Read the request body
	compressed, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
		h.metrics.requestsFailed.Add(1)
		metrics.RemoteWriteRequestFailuresTotal.Inc()
		return
	}
	defer r.Body.Close()

	h.metrics.bytesReceived.Add(uint64(len(compressed)))
	metrics.RemoteWriteBytesTotal.Add(float64(len(compressed)))

	// Decompress using snappy
	decompressed, err := snappy.Decode(nil, compressed)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to decompress request: %v", err), http.StatusBadRequest)
		h.metrics.requestsFailed.Add(1)
		metrics.RemoteWriteRequestFailuresTotal.Inc()
		return
	}

	// Unmarshal protobuf
	var req prompb.WriteRequest
	if err := proto.Unmarshal(decompressed, &req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to unmarshal request: %v", err), http.StatusBadRequest)
		h.metrics.requestsFailed.Add(1)
		metrics.RemoteWriteRequestFailuresTotal.Inc()
		return
	}

	// Count samples
	for _, ts := range req.Timeseries {
		n := uint64(len(ts.Samples))
		h.metrics.samplesReceived.Add(n)
		metrics.RemoteWriteSamplesTotal.Add(float64(n))
	}

	// Process the request
	if err := h.receiver.Receive(&req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to process request: %v", err), http.StatusInternalServerError)
		h.metrics.requestsFailed.Add(1)
		metrics.RemoteWriteRequestFailuresTotal.Inc()
		return
	}

	// Return success
	metrics.RemoteWriteRequestDuration.Observe(time.Since(start).Seconds())
	w.WriteHeader(http.StatusNoContent)
}

// Metrics returns a point-in-time snapshot of the current handler metrics
func (h *Handler) Metrics() HandlerMetrics {
	return HandlerMetrics{
		RequestsReceived: h.metrics.requestsReceived.Load(),
		RequestsFailed:   h.metrics.requestsFailed.Load(),
		SamplesReceived:  h.metrics.samplesReceived.Load(),
		BytesReceived:    h.metrics.bytesReceived.Load(),
	}
}
