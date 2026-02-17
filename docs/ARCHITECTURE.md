# PromSketch-Dropin Architecture

## System Architecture

```
                        ┌─────────────────────────────────────────────────────────────────────┐
                        │                      Visualization & Alerting                       │
                        │                                                                     │
                        │  ┌──────────────────────┐           ┌────────────────────────────┐  │
                        │  │       Grafana         │           │         vmalert             │  │
                        │  │  (Prometheus-type      │           │  (rule evaluation via       │  │
                        │  │   datasource)          │           │   pskquery endpoint)        │  │
                        │  └──────────┬───────────┘           └─────────────┬──────────────┘  │
                        │             │ PromQL / MetricsQL                  │                  │
                        └─────────────┼────────────────────────────────────┼──────────────────┘
                                      │                                    │
                                      ▼                                    ▼
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                  PromSketch-Dropin Cluster                                          │
│                                                                                                     │
│   ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐  │
│     QUERY PATH  (pskquery :8480)                                                              │  │
│   │                                                                                              │
│      ┌──────────────────────────────────────────────────────────────┐                          │  │
│   │  │                     Query Router                             │                            │
│      │                                                              │                          │  │
│   │  │  1. Parse PromQL/MetricsQL (metricsql parser)                │                            │
│      │  2. Check capability registry:                               │                          │  │
│   │  │     ┌──────────────────────────────────────────────────┐     │                            │
│      │     │ Sketch-capable:                                  │     │                          │  │
│   │  │     │   avg_over_time   sum_over_time                  │     │                            │
│      │     │   count_over_time quantile_over_time             │     │                          │  │
│   │  │     └──────────────────────────────────────────────────┘     │                            │
│      │  3. Route:                                                   │                          │  │
│   │  │     CAN sketch  ──► Fan-out to all psksketch nodes (gRPC)   │                            │
│      │     CANNOT      ──► Fallback to backend (HTTP)              │                          │  │
│   │  │  4. Merge results from nodes, return Prometheus JSON        │                            │
│      └──────────┬──────────────────────────────────┬───────────────┘                          │  │
│   │             │ gRPC fan-out                     │ HTTP fallback                               │
│    ─ ─ ─ ─ ─ ─ ┼ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┼ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘  │
│                 │                                  │                                            │
│                 ▼                                  │                                            │
│   ┌─────────────────────────────────────────┐     │                                            │
│   │        psksketch nodes (gRPC :8481)     │     │                                            │
│   │                                         │     │                                            │
│   │  ┌─────────────┐  ┌─────────────┐      │     │                                            │
│   │  │ psksketch-1 │  │ psksketch-2 │      │     │                                            │
│   │  │ part [0–5]  │  │ part [6–11] │      │     │                                            │
│   │  └─────────────┘  └─────────────┘      │     │                                            │
│   │  ┌─────────────┐                       │     │                                            │
│   │  │ psksketch-3 │  Per-partition:        │     │                                            │
│   │  │ part [12–15]│  • Exponential Hist.   │     │                                            │
│   │  └─────────────┘  • KLL (quantiles)     │     │                                            │
│   │                   • UnivMon (cardinality)│     │                                            │
│   │                   • Uniform Sampling     │     │                                            │
│   │                                         │     │                                            │
│   │  gRPC API:                              │     │                                            │
│   │    Insert / BatchInsert                 │     │                                            │
│   │    LookUp / Eval                        │     │                                            │
│   │    Health / Stats                       │     │                                            │
│   └──────────────▲──────────────────────────┘     │                                            │
│                  │                                 │                                            │
│   ┌ ─ ─ ─ ─ ─ ─ ┼ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐  │
│     INGESTION PATH  (pskinsert :8480)             │                                        │  │
│   │              │                                 │                                           │
│      ┌───────────┴──────────────────────────────┐  │                                       │  │
│   │  │          Ingestion Router                │  │                                           │
│      │                                          │  │                                       │  │
│   │  │  1. Receive Prometheus remote write      │  │                                           │
│      │     (Snappy + Protobuf)                  │  │                                       │  │
│   │  │  2. Consistent hash (xxhash) by          │  │                                           │
│      │     metric name ──► partition ID          │  │                                       │  │
│   │  │  3. Route to psksketch node(s)           │  │                                           │
│      │     (replication factor = 2)             │  │                                       │  │
│   │  │  4. Forward ALL raw samples to           │  │                                           │
│      │     backend (batched, async)             │  │                                       │  │
│   │  └──────────────────────────────┬───────────┘  │                                           │
│      gRPC insert ▲                  │ remote write  │                                       │  │
│   │              │                  ▼              │                                            │
│    ─ ─ ─ ─ ─ ─ ─│─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┼ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘  │
│                  │                  │              │                                            │
└──────────────────┼──────────────────┼──────────────┼────────────────────────────────────────────┘
                   │                  │              │
                   │                  ▼              ▼
                   │  ┌────────────────────────────────────────────┐
                   │  │       Backend: VictoriaMetrics :8428       │
                   │  │                                            │
                   │  │  • Stores ALL raw samples (full retention) │
                   │  │  • Answers fallback queries                │
                   │  │  • /api/v1/write  (ingest)                 │
                   │  │  • /api/v1/query  (instant)                │
                   │  │  • /api/v1/query_range  (range)            │
                   │  │  • /api/v1/series, /labels  (metadata)     │
                   │  └────────────────────────────────────────────┘
                   │
┌──────────────────┼───────────────────┐
│  Metric Sources  │                   │
│                  │ remote_write      │
│  ┌────────────┐  │                   │
│  │ Prometheus │──┘                   │
│  │ (scraper)  │     ┌─────────────┐  │
│  │            │────►│node_exporter│  │
│  └────────────┘     └─────────────┘  │
│                     ┌─────────────┐  │
│                     │ app metrics │  │
│                     └─────────────┘  │
└──────────────────────────────────────┘

    CLI TOOLING
    ┌──────────────────────────────────┐
    │            pskctl                │
    │                                  │
    │  backfill  - replay historical   │
    │              data from backend   │
    │  bench     - insertion throughput │
    │              & accuracy tests    │
    │  check     - validate configs    │
    │  version   - print version       │
    └──────────────────────────────────┘
```

## Data Flow

### Ingestion Path
```
Prometheus ──remote_write──► pskinsert ──┬──gRPC──► psksketch nodes (sketch insert)
                                         └──HTTP──► VictoriaMetrics  (raw storage)
```

### Query Path
```
Grafana/vmalert ──PromQL──► pskquery ──┬──gRPC──► psksketch nodes (sketch eval) ──► merge ──► response
                                       └──HTTP──► VictoriaMetrics  (fallback)   ──────────► response
```

## Component Summary

| Component | Binary | Port(s) | Role | Stateful? | Scalable? |
|---|---|---|---|---|---|
| **pskinsert** | `pskinsert` | HTTP :8480 | Ingestion router, consistent hash, backend forwarder | No | Horizontally |
| **pskquery** | `pskquery` | HTTP :8480 | Query router, fan-out/merge, fallback | No | Horizontally |
| **psksketch** | `psksketch` | gRPC :8481, HTTP :8482 | Sketch storage, per-partition sketch instances | Yes | Add nodes + repartition |
| **pskctl** | `pskctl` | — | CLI: backfill, bench, check | — | — |

---

## Feature List

### Core Capabilities

- **Drop-in Prometheus/VictoriaMetrics compatibility** — exposes the same HTTP API (`/api/v1/query`, `/query_range`, `/series`, `/labels`, `/label/*/values`), works with existing Grafana dashboards, alerting rules, and PromQL/MetricsQL queries without modification

- **Sketch-augmented approximate queries** — maintains streaming sketch data structures per time series for sub-linear memory, constant-time approximate aggregation queries over arbitrary time windows

- **Transparent query routing** — automatically determines whether a query can be answered by sketches or must fall back to the exact backend; users never need to know which path is used

- **Full raw data preservation** — all ingested samples are forwarded to the backend (VictoriaMetrics/Prometheus), so PromSketch-Dropin augments but never replaces the storage layer

### Sketch Algorithms & Supported Queries

| Query Function | Sketch Type | Description |
|---|---|---|
| `avg_over_time` | Uniform Sampling | Approximate average over sliding window |
| `sum_over_time` | Uniform Sampling | Approximate sum over sliding window |
| `sum2_over_time` | Uniform Sampling | Sum of squares over window |
| `stddev_over_time` | Uniform Sampling | Standard deviation over window |
| `stdvar_over_time` | Uniform Sampling | Variance over window |
| `count_over_time` | EH + UnivMon | Count of samples in window |
| `distinct_over_time` | EH + UnivMon | Distinct value count in window |
| `min_over_time` | EH + KLL | Minimum value in window |
| `max_over_time` | EH + KLL | Maximum value in window |
| `quantile_over_time` | EH + KLL | Arbitrary quantile (p50, p95, p99, etc.) |
| `entropy_over_time` | EH + UnivMon | Entropy estimation over window |
| `l1_over_time` | EH + UnivMon | L1 norm over window |
| `l2_over_time` | EH + UnivMon | L2 norm over window |

Any query not in this list is automatically forwarded to the backend for exact evaluation.

### Distributed Cluster Architecture

- **3-tier design**: stateless insert routers, stateless query routers, stateful sketch storage nodes — each tier scales independently
- **Consistent hashing** (xxhash) by metric name maps each time series to a deterministic partition
- **Configurable partitioning**: 16 partitions by default, split across N sketch nodes
- **Replication factor 2**: each partition is stored on a primary + one replica node via rendezvous hashing
- **gRPC inter-node communication**: pskinsert→psksketch (insert), pskquery→psksketch (eval)
- **Health checking with circuit breaker**: periodic gRPC health probes, automatic circuit-open on repeated failures, half-open recovery
- **Node discovery**: static configuration or Kubernetes headless service DNS

### Ingestion Pipeline

- **Prometheus remote write receiver** (`/api/v1/write`): accepts Snappy-compressed Protobuf, standard Prometheus remote write protocol
- **Backend forwarding**: batched async forwarding to VictoriaMetrics (configurable batch size, flush interval, retry with exponential backoff)
- **Sketch target matching**: configurable rules determine which time series get sketch instances
  - Exact metric name, regex patterns, label matchers
  - Per-target EH parameter overrides (window size, k, kll_k)
  - Unmatched metrics are forwarded to backend only (zero sketch overhead)
- **Memory-bounded storage**: configurable per-node memory limit for sketch instances

### Query API (Prometheus-compatible)

| Endpoint | Method | Description |
|---|---|---|
| `/api/v1/query` | GET/POST | Instant query (PromQL/MetricsQL) |
| `/api/v1/query_range` | GET/POST | Range query with start/end/step |
| `/api/v1/series` | GET/POST | Series metadata (proxied to backend) |
| `/api/v1/labels` | GET/POST | Label names (proxied to backend) |
| `/api/v1/label/<name>/values` | GET/POST | Label values (proxied to backend) |
| `/health` | GET | Component health status |
| `/metrics` | GET | Prometheus-format self-metrics |

### Observability (Self-Metrics)

- `pskquery_queries_total{source="sketch|backend"}` — query count by routing path
- `pskquery_sketch_hits_total` / `pskquery_sketch_misses_total` — sketch hit/miss ratio
- `pskquery_query_duration_seconds{type="instant|range"}` — query latency quantiles (p50, p90, p99)
- `pskquery_backend_duration_seconds` — backend fallback latency quantiles
- `pskquery_merge_errors_total` — merge error count
- `pskinsert_requests_total` — ingestion request count
- Per-node: series count, sketch coverage, sample throughput, memory usage

### CLI Tooling (`pskctl`)

- **`pskctl backfill`** — replay historical data from VictoriaMetrics or Prometheus into sketch instances; supports time range chunking, metric filtering, checkpoint/resume, dry-run mode
- **`pskctl bench insert`** — synthetic insertion throughput benchmark; measures samples/sec
- **`pskctl bench accuracy`** — query accuracy comparison between PromSketch and exact backend; reports absolute and relative error
- **`pskctl check config`** — validate YAML configuration files
- **`pskctl version`** — print build version, git commit, build date

### Deployment

- **Docker Compose cluster**: single `docker-compose.cluster.yml` brings up the full stack:
  - 3x psksketch nodes (partitioned storage)
  - 1x pskinsert (ingestion router)
  - 1x pskquery (query router)
  - VictoriaMetrics (backend storage)
  - Prometheus (scraper, remote_write to pskinsert)
  - Grafana (pre-provisioned datasources + dashboards)
  - vmalert (alert rule evaluation against pskquery)
  - node_exporter (sample metric source)
- **Single-server mode**: monolithic `promsketch-dropin` binary for simple deployments
- **Multi-stage Docker builds**: minimal production images per component
- **Kubernetes-ready**: headless service DNS discovery for psksketch nodes

### Backend Abstraction

Pluggable backend interface (`backend.Backend`):
```go
type Backend interface {
    Write(ctx, *prompb.WriteRequest) error
    Query(ctx, query, time) (*QueryResult, error)
    QueryRange(ctx, query, start, end, step) (*QueryResult, error)
    Health(ctx) error
}
```
Currently implemented: **VictoriaMetrics**, **Prometheus**. Designed for extension to InfluxDB, ClickHouse, etc.
