# PromSketch-Dropin Quick Start Guide

## Build

```bash
go build -o bin/promsketch-dropin ./cmd/promsketch-dropin
go build -o bin/pskctl ./cmd/pskctl
```

## Configure

Create `promsketch-dropin.yaml`:

```yaml
server:
  listen_address: ":9100"
  log_level: info

ingestion:
  remote_write:
    enabled: true
    listen_address: ":9100"

backend:
  type: victoriametrics
  url: http://localhost:8428
  remote_write_url: http://localhost:8428/api/v1/write
  timeout: 30s
  batch_size: 1000
  flush_interval: 5s

sketch:
  num_partitions: 16

  defaults:
    eh_params:
      window_size: 1800  # 30 minutes
      k: 50
      kll_k: 256

  targets:
    - match: 'http_requests_total'
    - match: 'http_.*_duration'
    - match: '{job="api"}'
```

## Run

```bash
./bin/promsketch-dropin --config.file=promsketch-dropin.yaml
```

## Test with Prometheus

Configure Prometheus to remote write to PromSketch-Dropin:

```yaml
# prometheus.yml
remote_write:
  - url: http://localhost:9100/api/v1/write
```

## Endpoints

### Ingestion
- `POST /api/v1/write` - Remote write receiver

### Query
- `GET/POST /api/v1/query` - Instant query (Prometheus-compatible)
- `GET/POST /api/v1/query_range` - Range query (Prometheus-compatible)

### System
- `GET /health` - Health check
- `GET /metrics` - Internal metrics

## Test Suite

```bash
# Run all tests
go test ./internal/...

# Run specific component tests
go test ./internal/backend -v
go test ./internal/storage/... -v
go test ./internal/ingestion/... -v
```

## Metrics

PromSketch-Dropin exposes metrics at `/metrics`:

```
storage_total_series          - Total time series tracked
storage_sketched_series       - Series with sketch instances
storage_samples_inserted      - Samples inserted into sketches
forwarder_samples_forwarded   - Samples forwarded to backend
forwarder_batches_sent        - Batches sent to backend
pipeline_samples_received     - Total samples received
```

## Architecture

```
Prometheus → PromSketch-Dropin → VictoriaMetrics
             (/api/v1/write)     (remote write)
                    ↓
             [Sketch Storage]
             (matching metrics)
```

## Sketch Target Patterns

```yaml
# Exact match
- match: 'http_requests_total'

# Regex
- match: 'http_.*'

# Label selector
- match: '{job="api"}'
- match: '{__name__=~"node_.*", mode="idle"}'

# Wildcard (all metrics)
- match: '*'
```

## Query Examples

### Instant Query
```bash
# Query current value
curl 'http://localhost:9100/api/v1/query?query=avg_over_time(http_requests_total[5m])&time=1234567890'
```

### Range Query
```bash
# Query over time range
curl 'http://localhost:9100/api/v1/query_range?query=sum_over_time(http_requests_total[5m])&start=1234567890&end=1234571490&step=60'
```

### Supported Sketch Functions
```promql
avg_over_time(metric[5m])
sum_over_time(metric[5m])
count_over_time(metric[5m])
quantile_over_time(0.95, metric[5m])
```

### Configure Grafana
Add PromSketch-Dropin as a Prometheus datasource:
- URL: `http://localhost:9100`
- Access: Server (default)
- No authentication required

## Next Steps

Phase 2-3 (Ingestion) ✅ Complete
Phase 4 (Query API) ✅ Complete

Ready for Phase 5: Grafana Integration Testing
- Test with Grafana dashboards
- Verify query compatibility
- Benchmark sketch vs. backend accuracy
