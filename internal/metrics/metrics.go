// Package metrics provides centralized Prometheus metrics for PromSketch-Dropin.
// All metrics are defined as package-level variables using promauto so they are
// automatically registered with the default prometheus registry.
// This package MUST NOT import any other internal/ packages to prevent import cycles.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ---- Build info ----

var BuildInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "promsketch_build_info",
	Help: "Build information for the PromSketch-Dropin component.",
}, []string{"version", "commit", "date", "component"})

// SetBuildInfo is a convenience helper to set build info gauge to 1.
func SetBuildInfo(version, commit, date, component string) {
	BuildInfo.WithLabelValues(version, commit, date, component).Set(1)
}

// ---- Ingestion (pipeline) ----

var (
	IngestionSamplesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_ingestion_samples_total",
		Help: "Total samples received by the ingestion pipeline.",
	})
	IngestionSketchSamplesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_ingestion_sketch_samples_total",
		Help: "Total samples inserted into sketch storage.",
	})
	IngestionBackendSamplesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_ingestion_backend_samples_total",
		Help: "Total samples forwarded to the backend.",
	})
	IngestionErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_ingestion_errors_total",
		Help: "Total ingestion pipeline errors.",
	})
)

// ---- Remote write handler ----

var (
	RemoteWriteRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_remotewrite_requests_total",
		Help: "Total remote write requests received.",
	})
	RemoteWriteRequestFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_remotewrite_request_failures_total",
		Help: "Total remote write requests that failed.",
	})
	RemoteWriteSamplesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_remotewrite_samples_total",
		Help: "Total samples received via remote write.",
	})
	RemoteWriteBytesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_remotewrite_bytes_total",
		Help: "Total compressed bytes received via remote write.",
	})
	RemoteWriteRequestDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "promsketch_remotewrite_request_duration_seconds",
		Help:    "Duration of remote write request handling.",
		Buckets: prometheus.DefBuckets,
	})
)

// ---- OTLP handler ----

var (
	OTLPRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_otlp_requests_total",
		Help: "Total OTLP metrics requests received.",
	})
	OTLPRequestFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_otlp_request_failures_total",
		Help: "Total OTLP metrics requests that failed.",
	})
	OTLPSamplesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_otlp_samples_total",
		Help: "Total samples received via OTLP.",
	})
	OTLPRequestDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "promsketch_otlp_request_duration_seconds",
		Help:    "Duration of OTLP request handling.",
		Buckets: prometheus.DefBuckets,
	})
)

// ---- Query ----

var (
	QueryRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "promsketch_query_requests_total",
		Help: "Total query requests by type.",
	}, []string{"type"})
	QueryErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "promsketch_query_errors_total",
		Help: "Total query errors by type.",
	}, []string{"type"})
	QueryDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "promsketch_query_duration_seconds",
		Help:    "Duration of query execution.",
		Buckets: prometheus.DefBuckets,
	}, []string{"type"})
	QuerySourceTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "promsketch_query_source_total",
		Help: "Total queries by source (sketch vs backend).",
	}, []string{"source"})
	QuerySketchHitsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_query_sketch_hits_total",
		Help: "Total queries successfully answered by sketches.",
	})
	QuerySketchMissesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_query_sketch_misses_total",
		Help: "Total queries that fell back from sketch to backend.",
	})
)

// ---- Storage ----

var (
	StorageSeriesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "promsketch_storage_series_total",
		Help: "Total number of series tracked.",
	})
	StorageSketchedSeries = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "promsketch_storage_sketched_series",
		Help: "Number of series with active sketches.",
	})
	StorageSamplesInsertedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_storage_samples_inserted_total",
		Help: "Total samples inserted into sketch storage.",
	})
	StorageInsertErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_storage_insert_errors_total",
		Help: "Total sketch insertion errors.",
	})
	StorageMemoryUsedBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "promsketch_storage_memory_used_bytes",
		Help: "Approximate memory used by sketch storage.",
	})
	StorageMemoryLimitBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "promsketch_storage_memory_limit_bytes",
		Help: "Memory limit for sketch storage (0 = unlimited).",
	})
	StorageMemoryRejectionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_storage_memory_rejections_total",
		Help: "Total series rejected due to memory limit.",
	})
)

// ---- Forwarder ----

var (
	ForwarderSamplesForwardedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_forwarder_samples_forwarded_total",
		Help: "Total samples forwarded to the backend.",
	})
	ForwarderSamplesDroppedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_forwarder_samples_dropped_total",
		Help: "Total samples dropped due to full queue.",
	})
	ForwarderBatchesSentTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_forwarder_batches_sent_total",
		Help: "Total batches sent to the backend.",
	})
	ForwarderBatchesFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_forwarder_batches_failed_total",
		Help: "Total batches that failed to send.",
	})
	ForwarderBatchDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "promsketch_forwarder_batch_duration_seconds",
		Help:    "Duration of batch send operations.",
		Buckets: prometheus.DefBuckets,
	})
	ForwarderQueueLength = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "promsketch_forwarder_queue_length",
		Help: "Current number of samples queued for forwarding.",
	})
)

// ---- Metadata ----

var (
	MetadataRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "promsketch_metadata_requests_total",
		Help: "Total metadata API requests by endpoint.",
	}, []string{"endpoint"})
	MetadataErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_metadata_errors_total",
		Help: "Total metadata API errors.",
	})
)

// ---- Cluster / insert router (pskinsert) ----

var (
	PartitionCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "promsketch_partition_count",
		Help: "Number of partitions managed by this node.",
	})
	InsertRouterSamplesRoutedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_insert_router_samples_routed_total",
		Help: "Total samples routed to sketch nodes.",
	})
	InsertRouterSamplesFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_insert_router_samples_failed_total",
		Help: "Total samples that failed routing.",
	})
	InsertRouterRPCsSentTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_insert_router_rpcs_sent_total",
		Help: "Total gRPC insert RPCs sent.",
	})
	InsertRouterRPCsFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_insert_router_rpcs_failed_total",
		Help: "Total gRPC insert RPCs that failed.",
	})
)

// ---- Merger (pskquery) ----

var (
	MergerSketchQueriesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_merger_sketch_queries_total",
		Help: "Total queries sent to sketch nodes.",
	})
	MergerBackendQueriesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_merger_backend_queries_total",
		Help: "Total queries sent to the backend.",
	})
	MergerSketchHitsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_merger_sketch_hits_total",
		Help: "Total queries answered by sketch nodes.",
	})
	MergerSketchMissesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_merger_sketch_misses_total",
		Help: "Total queries that fell back to backend after sketch miss.",
	})
	MergerErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promsketch_merger_errors_total",
		Help: "Total merger errors.",
	})
	MergerQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "promsketch_merger_query_duration_seconds",
		Help:    "Duration of merger query execution.",
		Buckets: prometheus.DefBuckets,
	}, []string{"type"})
	MergerBackendDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "promsketch_merger_backend_duration_seconds",
		Help:    "Duration of backend queries as observed by the merger.",
		Buckets: prometheus.DefBuckets,
	})
)
