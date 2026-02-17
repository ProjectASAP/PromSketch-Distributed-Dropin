package otlp

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/zzylol/VictoriaMetrics/lib/prompbmarshal"
	otlpstream "github.com/zzylol/VictoriaMetrics/lib/protoparser/opentelemetry/stream"

	"github.com/promsketch/promsketch-dropin/internal/metrics"
)

// Handler handles OpenTelemetry metrics ingestion requests.
type Handler struct {
	receiver Receiver
	metrics  *handlerMetrics
}

// Receiver processes incoming OTLP time series parsed by the VM OTLP stream parser.
type Receiver interface {
	ReceiveVMMarshalTimeSeries(tss []prompbmarshal.TimeSeries) error
}

// handlerMetrics is the internal atomic state for handler counters
type handlerMetrics struct {
	requestsReceived atomic.Uint64
	requestsFailed   atomic.Uint64
	samplesReceived  atomic.Uint64
}

// HandlerMetrics is a point-in-time snapshot of handler statistics
type HandlerMetrics struct {
	RequestsReceived uint64
	RequestsFailed   uint64
	SamplesReceived  uint64
}

// NewHandler creates a new OTLP handler
func NewHandler(receiver Receiver) *Handler {
	return &Handler{
		receiver: receiver,
		metrics:  &handlerMetrics{},
	}
}

// ServeHTTP implements http.Handler.
// It uses the VictoriaMetrics OTLP stream parser which converts OpenTelemetry
// protobuf metrics to Prometheus-format TimeSeries.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.metrics.requestsReceived.Add(1)
	metrics.OTLPRequestsTotal.Inc()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		h.metrics.requestsFailed.Add(1)
		metrics.OTLPRequestFailuresTotal.Inc()
		return
	}

	isGzipped := r.Header.Get("Content-Encoding") == "gzip"
	err := otlpstream.ParseStream(r.Body, isGzipped, nil, func(tss []prompbmarshal.TimeSeries) error {
		// Count samples
		var n uint64
		for i := range tss {
			n += uint64(len(tss[i].Samples))
		}
		h.metrics.samplesReceived.Add(n)
		metrics.OTLPSamplesTotal.Add(float64(n))

		return h.receiver.ReceiveVMMarshalTimeSeries(tss)
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to process OTLP request: %v", err), http.StatusBadRequest)
		h.metrics.requestsFailed.Add(1)
		metrics.OTLPRequestFailuresTotal.Inc()
		return
	}

	metrics.OTLPRequestDuration.Observe(time.Since(start).Seconds())
	w.WriteHeader(http.StatusNoContent)
}

// Metrics returns a point-in-time snapshot of the current handler metrics
func (h *Handler) Metrics() HandlerMetrics {
	return HandlerMetrics{
		RequestsReceived: h.metrics.requestsReceived.Load(),
		RequestsFailed:   h.metrics.requestsFailed.Load(),
		SamplesReceived:  h.metrics.samplesReceived.Load(),
	}
}
