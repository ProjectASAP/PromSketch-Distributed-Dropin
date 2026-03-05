# PromSketch-Dropin

PromSketch-Dropin is a Prometheus-compatible query and ingestion layer that adds sketch-based approximate analytics on top of your existing metrics backend.

It is designed to be a drop-in component between metric producers/consumers and storage systems such as VictoriaMetrics or Prometheus:
- Ingestion path: receives metric samples (Remote Write / OTLP / built-in Scrape Manager), updates sketches for selected metrics, and forwards all raw samples to backend storage.
- Query path: receives PromQL/MetricsQL queries, decides whether sketches can answer them, and either returns approximate results from sketches or falls back to exact backend queries.

This gives you faster and lower-memory execution for supported rolling-window functions, without losing full-fidelity historical data, because the original samples are still persisted in the backend.

Typical usage goals:
- Reduce query cost for high-cardinality or high-throughput metric workloads.
- Keep existing dashboards, alerts, and clients working through Prometheus-compatible APIs.
- Adopt approximation incrementally by enabling sketches only for selected metric targets.

## Existing Features

### Core capabilities

- Prometheus-compatible query and metadata APIs (`/query`, `/query_range`, `/series`, `/labels`, `/label/<name>/values`)
- Ingestion via Remote Write, OTLP metrics endpoint, and built-in Scrape Manager
- Smart query routing: sketch execution when supported, automatic backend fallback otherwise
- Approximation metadata in query response `warnings` (`epsilon`, `confidence`, `source`)
- Raw sample preservation: all incoming data is forwarded to backend storage
- Monolithic and distributed deployment modes
- Operational observability via health, Prometheus metrics, and ingestion stats

### Supported sketch functions

| Function | Status |
|---|---|
| `avg_over_time` | Supported |
| `sum_over_time` | Supported |
| `sum2_over_time` | Supported |
| `count_over_time` | Supported |
| `stddev_over_time` | Supported |
| `stdvar_over_time` | Supported |
| `quantile_over_time` | Supported |
| `min_over_time` | Supported |
| `max_over_time` | Supported |
| `entropy_over_time` | Supported |
| `distinct_over_time` | Supported |
| `l1_over_time` | Supported |
| `l2_over_time` | Supported |

### Endpoint summary

| Endpoint | Purpose |
|---|---|
| `POST /api/v1/write` | Prometheus Remote Write ingestion |
| `POST /opentelemetry/v1/metrics` | OTLP metrics ingestion |
| `GET/POST /api/v1/query` | Instant query |
| `GET/POST /api/v1/query_range` | Range query |
| `GET/POST /api/v1/series` | Series metadata (proxied to backend) |
| `GET/POST /api/v1/labels` | Label names (proxied to backend) |
| `GET/POST /api/v1/label/<name>/values` | Label values (proxied to backend) |
| `GET /ingest_stats` | Runtime ingestion counters and rates |
| `GET /health` | Service/component health |
| `GET /metrics` | Prometheus-format internal metrics |
| `GET /ui` | Built-in query UI (monolithic mode) |

### Distributed mode

- `pskinsert`: ingestion router (partitioning, replication-aware insert, backend forwarding)
- `psksketch`: stateful sketch storage node (gRPC + HTTP health/metrics)
- `pskquery`: query fan-out/merge layer with backend fallback
- Cluster capabilities: consistent hashing, configurable partitions/replication, health checks, circuit breaker, static/Kubernetes discovery

### Tooling

- `pskctl check config <file>`
  - Validates config syntax and key settings.
- `pskctl backfill`
  - Replays historical data from VictoriaMetrics/Prometheus into PromSketch ingestion.
  - Supports time-range chunking, checkpoint file, and resume mode.
- `pskctl bench insert`
  - Generates synthetic writes and measures ingestion throughput.
- `pskctl bench accuracy`
  - Compares PromSketch query results vs backend query results.
- `pskctl version`
  - Prints build/version metadata.

### Dashboard/plugin assets

- If you want to build or customize Grafana dashboards/plugins for PromSketch-Dropin, check:
  - `/grafana-dashboard-plugin`
  - `grafana-dashboard-plugin/README.md` for usage details.

## Demo Guide

Run all commands from the repository root folder

### Start demo stack

```bash
docker compose up -d --build
```

### Verify health

```bash
curl http://localhost:9100/health
curl http://localhost:8428/health
curl http://localhost:9090/-/healthy
```

### View logs

```bash
docker compose logs -f promsketch-dropin
```

### Stop demo stack

```bash
docker compose down
docker compose down -v   # erase volume/data
```

## Optional: Experiment with e2e tests

Run all commands from the repository root folder.

### 1) Build app binary

```bash
go build -o bin/promsketch-dropin ./cmd/promsketch-dropin
```

### 2) Extract `victoria-metrics-prod` binary from Docker image

```bash
cid=$(docker create victoriametrics/victoria-metrics:latest)
docker cp "$cid":/victoria-metrics-prod /tmp/victoria-metrics-prod
docker rm "$cid"
chmod +x /tmp/victoria-metrics-prod
```

### 3) Run e2e tests

```bash
go test ./e2e_test -v
```
