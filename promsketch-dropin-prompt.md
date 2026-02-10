# PromSketch-Dropin: Claude Code Development Prompt

## Context

In `/mydata/promsketch`, there is an existing sketch library for cloud observability metrics that supports rule-based dynamic window queries. The goal is to build **PromSketch-Dropin** (`/mydata/PromSketch-Dropin`) — a fully standalone, distributed component that wraps the PromSketch algorithm into a drop-in-compatible system that works seamlessly with the Prometheus/VictoriaMetrics ecosystem.

**"Standalone" means:**
- Compatible metric ingestion API (Prometheus remote write / scrape target)
- Compatible query API (PromQL/MetricsQL HTTP endpoints matching Prometheus and VictoriaMetrics)
- Has a custom Grafana datasource plugin (can be based on the Prometheus datasource type, but branded as PromSketch and extensible for sketch-specific query options in the future)
- From the user's perspective, nothing changes — same collectors, same queries, same dashboards

## Architecture Overview

PromSketch-Dropin is a **sketch-augmented metrics proxy** that sits alongside (not replacing) existing observability backends. It intercepts metrics at ingestion time, maintains sketch data structures for efficient approximate queries, and transparently routes queries to either its sketch engine or the backend depending on capability.

```
                          DATA INGESTION                                          QUERY PATH
                          ════════════                                            ══════════

  ┌──────────────┐        ┌───────────────────────────────────────────────────────────────────────────┐
  │ Scrape       │        │                         PromSketch-Dropin                                 │
  │ Targets      │        │                                                                           │
  │ (node_exp,   │◄───────┤  ┌──────────────────┐                                                    │
  │  app, etc.)  │ scrape │  │ Built-in Scrape  │                                                    │
  └──────────────┘        │  │ Manager          │                                                    │
                          │  └────────┬─────────┘                                                    │
                          │           │                                                              │
                          │           ▼                                                              │
  ┌──────────────┐        │  ┌──────────────────┐         ┌──────────────────────┐                   │
  │ Prometheus / │ remote │  │                  │ insert  │  PromSketch          │                   │
  │ VM Agent     │─write─►│  │    Ingestion     │────────►│  Instances           │                   │
  │              │        │  │    Pipeline       │         │                      │                   │
  └──────────────┘        │  │                  │         │  • consistent hash   │                   │
                          │  └────────┬─────────┘         │    by metric name    │                   │
                          │           │                   │  • EH per time       │                   │
                          │           │ forward all       │    series             │                   │
                          │           │ raw samples       └──────────┬───────────┘                   │
                          │           │                              │                               │
                          │           ▼                              │ query                         │
                          │  ┌──────────────────┐                   │                               │
                          │  │ Backend          │                   ▼                               │
                          │  │ Forwarder        │    ┌──────────────────────────────┐                │
                          │  │ (remote write)   │    │        Query Router          │◄──── /api/v1/  │
                          │  └────────┬─────────┘    │                              │      query     │
                          │           │              │  1. Parse MetricsQL          │      query_range│
                          │           │              │  2. Can sketch answer?       │      series    │
                          │           │              │     YES ──► sketch engine    │      labels    │
                          │           │              │     NO  ──► fallback query   │                │
                          │           │              └─────┬──────────────┬─────────┘                │
                          │           │                    │              │                          │
                          │           │              sketch│         fall-│                          │
                          │           │              result│         back │                          │
                          │           │                    ▼              │                          │
                          │           │              ┌───────────┐       │                          │
                          │           │              │  Response  │       │                          │
                          │           │              │  (Prom API │◄──────┘                          │
                          │           │              │   format)  │                                  │
                          │           │              └─────┬─────┘                                  │
                          │           │                    │                                        │
                          │           │              ┌─────▼──────────────────┐                     │
                          │           │              │ Built-in Query UI      │                     │
                          │           │              │ (embedded vmui)        │                     │
                          │           │              └────────────────────────┘                     │
                          │           │                                                             │
                          └───────────┼─────────────────────────────────────────────────────────────┘
                                      │                    ▲
                                      │ forward            │ query fallback
                                      ▼                    │
                          ┌──────────────────────────────────────────────┐
                          │           Backend System                     │
                          │      (VictoriaMetrics / Prometheus)          │
                          │                                              │
                          │  • stores all raw samples                    │
                          │  • answers fallback queries                  │
                          └──────────────────────────────────────────────┘
                                                   ▲
                                                   │ query via /api/v1/* endpoints
                                                   │
                          ┌────────────────────────┴───────────────────────┐
                          │                                                │
                   ┌──────┴──────────┐                          ┌──────────┴──────────┐
                   │     Grafana     │                          │      pskctl CLI     │
                   │  (PromSketch    │                          │                     │
                   │   datasource    │                          │  backfill ─► insert │
                   │   plugin)       │                          │  bench insert       │
                   └─────────────────┘                          │  bench accuracy     │
                                                                │  check config       │
                                                                └─────────────────────┘
```

**Two ingestion modes (both supported, can run simultaneously):**
- **Remote Write Receiver:** Accepts Prometheus remote write at `/api/v1/write` — Prometheus or VictoriaMetrics scrapes targets and forwards to PromSketch-Dropin.
- **Built-in Scrape Manager:** Reads a Prometheus-style `scrape_configs` YAML and scrapes targets directly, no external Prometheus needed.

## Detailed Requirements

### 1. Metric Ingestion Layer

- **Implement both ingestion modes (can run simultaneously):**
  - **Remote Write Receiver:** Expose a Prometheus-compatible remote write endpoint (`/api/v1/write`) so that an external Prometheus or VictoriaMetrics can scrape targets and forward samples to PromSketch-Dropin.
  - **Built-in Scrape Manager:** Implement a scrape manager that reads a Prometheus-style `scrape_configs` YAML and scrapes targets directly. Research using the Prometheus scrape package/library to do this cleanly rather than reimplementing from scratch.
  - Both modes feed into the same ingestion pipeline.
- **Forward all raw samples** to a configurable backend metrics system (VictoriaMetrics or Prometheus remote write endpoint). This ensures full data retention in the backend — PromSketch-Dropin does NOT replace storage, it augments query capability.
- **Configuration:** YAML config file specifying:
  - Listen address/port for ingestion
  - Backend forwarding URL(s) (e.g., VictoriaMetrics remote write endpoint)
  - Backend type (`victoriametrics` | `prometheus`, extensible to `influxdb`, `clickhouse` later)
  - Scrape configs (if implementing Option B)

### 2. PromSketch Library Integration

- **Copy the entire `/mydata/promsketch` core code** into the PromSketch-Dropin repo as an internal library/package. This allows independent development and code changes without affecting the original repo.
- On every received metric sample, call the PromSketch library API to insert data into the appropriate PromSketch instance.
- **Partitioning:** Multiple PromSketch instances, partitioned by **consistent hashing of metric names**. Design this so the number of instances is configurable and can be scaled.
- **Per-time-series structure:** For each **configured** time series metric, maintain an Exponential Histogram (EH) instance via the PromSketch library. Not all time series need sketch instances — this is controlled by configuration:
  - **Sketch target configuration:** A YAML section specifying which time series should have EH instances created, supporting:
    - Exact metric name match: `http_request_duration_seconds`
    - Regex patterns: `http_.*`, `node_cpu_.*`
    - Label matchers: `{job="apiserver", __name__=~"request_.*"}`
    - Wildcard / catch-all: `*` (create EH for all incoming time series — use with caution)
  - Example config:
    ```yaml
    sketch_targets:
      - match: '{__name__=~"http_request_duration_.*"}'
        eh_params:
          window_size: 3600    # optional per-target EH parameter overrides
      - match: '{__name__="node_cpu_seconds_total"}'
      - match: '{job="apiserver"}'
    sketch_defaults:
      eh_params:
        window_size: 1800      # default EH parameters for all sketch targets
    ```
  - Time series not matching any sketch target are still forwarded to the backend but do **not** get a PromSketch instance (no sketch overhead for untracked metrics).
  - Support dynamic reload of this configuration (SIGHUP or HTTP endpoint) without restarting the service.
- Handle instance lifecycle: creation of new PromSketch instances when new metric names appear, cleanup/expiration policies for stale metrics.

### 3. Query API Layer (Prometheus/VictoriaMetrics Compatible)

Implement HTTP endpoints that are **wire-compatible** with Prometheus and VictoriaMetrics query APIs:

- `GET/POST /api/v1/query` — instant query
- `GET/POST /api/v1/query_range` — range query
- `GET/POST /api/v1/series` — series metadata
- `GET/POST /api/v1/labels` — label names
- `GET/POST /api/v1/label/<name>/values` — label values
- Any other endpoints needed for Grafana's Prometheus datasource to work (search online for the full list Grafana expects)

The response format must match the Prometheus API JSON response format so the plugin can reuse existing Prometheus query/visualization logic. Additionally, build a **custom Grafana datasource plugin** ("PromSketch") that:

- Wraps the standard Prometheus datasource behavior (query, query_range, series, labels)
- Is branded as "PromSketch" as a distinct datasource type in Grafana
- Is extensible for future sketch-specific query options (e.g., toggling sketch vs. fallback mode, displaying sketch accuracy metadata, selecting sketch-specific aggregation functions)
- Research Grafana's plugin SDK and scaffolding tools (`@grafana/create-plugin`) for building this cleanly

### 4. MetricsQL Query Router (Smart Query Dispatch)

This is the core intelligence layer:

- **Parse incoming queries** using a MetricsQL parser (research existing Go/Rust libraries, e.g., VictoriaMetrics has a MetricsQL parser in Go that may be reusable).
- **Analyze each query** to determine if it can be answered by PromSketch instances:
  - If **yes** → route to PromSketch instances, aggregate results across partitions, return response.
  - If **no** → transparently **proxy/forward the query** to the configured backend system (VictoriaMetrics or Prometheus) and return the backend's response as-is.
- **Query capability detection:** Define a clear interface/registry of which query operations/functions PromSketch can handle. This should be easily extensible as PromSketch gains new capabilities.
- **Backend abstraction:** Design the backend query interface as a pluggable abstraction:
  ```
  trait/interface BackendQuerier {
      query(expr, time) -> Result
      query_range(expr, start, end, step) -> Result
  }
  ```
  Implement for Prometheus and VictoriaMetrics first. The abstraction should make it straightforward to add InfluxDB, ClickHouse, etc. later.

### 5. Configuration & Deployment

- Single YAML configuration file covering:
  - Ingestion settings (listen address, scrape configs)
  - Backend settings (type, URL, auth)
  - PromSketch settings (number of partitions, EH parameters, memory limits)
  - Query settings (listen address for query API, fallback backend URL)
- Dockerized deployment with docker-compose example including:
  - PromSketch-Dropin
  - VictoriaMetrics (as backend)
  - Grafana (pre-configured with PromSketch-Dropin as a Prometheus datasource)
  - A sample metrics producer (e.g., node_exporter) for demo

### 6. Language & Tech Decisions

- Research and recommend the best implementation language. Consider:
  - **Go**: natural fit for Prometheus ecosystem, rich library support (Prometheus client libs, MetricsQL parser from VM)
  - **Rust**: if the existing PromSketch library is in Rust
  - Check what language `/mydata/promsketch` is written in and align accordingly.
- Use existing Prometheus ecosystem libraries wherever possible rather than reimplementing protocols.

## Implementation Order

1. **First:** Examine `/mydata/promsketch` to understand the library API, language, and data structures.
2. **Second:** Set up the project structure with the PromSketch library copied in.
3. **Third:** Implement the ingestion layer (remote write receiver + backend forwarding).
4. **Fourth:** Wire up PromSketch insertion on ingestion.
5. **Fifth:** Implement the query API endpoints with the query router (sketch vs. fallback).
6. **Sixth:** Test with Grafana as a Prometheus datasource.
7. **Seventh:** Build `pskctl` CLI (backfill, bench insert, bench accuracy, check).
8. **Eighth:** Docker-compose demo setup.

### 7. `pskctl` — Unified CLI Tool

Provide a single CLI binary called `pskctl` with hierarchical subcommands, following the patterns of Prometheus's `promtool` (subcommand groups like `tsdb`, `check`, `push`) and VictoriaMetrics's `vmctl` (mode-based subcommands like `influx`, `prometheus`, `vm-native` with per-mode flags).

**Top-level command structure:**
```
$ pskctl --help
NAME:
    pskctl - PromSketch Control, CLI tooling for PromSketch-Dropin

USAGE:
    pskctl [global options] command [command options] [arguments...]

COMMANDS:
    backfill      Backfill historical data into PromSketch instances
    bench         Run throughput and accuracy benchmarks
    check         Validate configuration files
    version       Print version information
    help          Help about any command

GLOBAL OPTIONS:
    --log.level       Log verbosity (debug, info, warn, error)
    --log.format      Log format (logfmt, json)
```

#### `pskctl backfill` — Historical Data Backfill

Modeled after `promtool tsdb create-blocks-from` and `vmctl vm-native` / `vmctl prometheus`. Reads historical data from a backend and replays it into PromSketch-Dropin's insertion pipeline.

```bash
# Backfill from VictoriaMetrics via its export API
pskctl backfill --source-type victoriametrics \
    --source-url http://victoria:8428 \
    --start "2025-01-01T00:00:00Z" \
    --end "2025-02-01T00:00:00Z" \
    --target http://localhost:9100

# Backfill from Prometheus via remote read
pskctl backfill --source-type prometheus \
    --source-url http://prometheus:9090 \
    --start "2025-01-01T00:00:00Z" \
    --end "2025-02-01T00:00:00Z" \
    --target http://localhost:9100

# Backfill from an OpenMetrics / JSON export file
pskctl backfill --source-type file \
    --source-path /path/to/export.json \
    --target http://localhost:9100

# Filter by metric names or label matchers (like vmctl's --vm-native-filter-match)
pskctl backfill --source-type victoriametrics \
    --source-url http://victoria:8428 \
    --match '{__name__=~"http_.*"}' \
    --start "2025-01-01T00:00:00Z" \
    --end "2025-02-01T00:00:00Z" \
    --target http://localhost:9100
```

**Flags:**
- `--source-type` — Backend type: `victoriametrics`, `prometheus`, `file` (extensible to `influxdb`, `clickhouse`)
- `--source-url` / `--source-path` — Source address or file path
- `--target` — PromSketch-Dropin address
- `--start`, `--end` — Time range (RFC3339 or Unix timestamp)
- `--match` — Series selector to filter which metrics to backfill
- `--concurrency` — Number of concurrent workers (like vmctl's `--vm-concurrency`)
- `--step-interval` — Split backfill into time chunks: `month`, `week`, `day`, `hour` (like vmctl's `--vm-native-step-interval`)
- `--dry-run` — Preview what would be backfilled without inserting
- `--rate-limit` — Max samples/sec to avoid overwhelming the system
- `--silent` / `-s` — Skip confirmation prompt (like vmctl's `-s` flag)
- `--disable-progress-bar` — Disable progress bar during import

**Behavior:**
- Performs initial readiness check on target (`/health` endpoint) before starting
- Shows progress bar with samples processed, time range covered, samples/sec
- Prints summary statistics on completion (total samples, duration, throughput, errors)
- Resumable: if interrupted, can resume from where it left off based on time range checkpointing

#### `pskctl bench` — Throughput & Accuracy Benchmarks

**Subcommands:**

```
$ pskctl bench --help
COMMANDS:
    insert     Run insertion throughput benchmark
    accuracy   Run query accuracy comparison benchmark
```

**`pskctl bench insert`** — Insertion throughput testing:
```bash
# Generate synthetic metrics and measure insertion rate
pskctl bench insert --target http://localhost:9100 \
    --num-series 10000 \
    --samples-per-series 1000 \
    --batch-size 500 \
    --concurrency 8 \
    --duration 5m

# Replay real metrics from an export for realistic load testing
pskctl bench insert --target http://localhost:9100 \
    --source-file /path/to/exported_metrics.json \
    --replay-speed 2x
```
- **Flags:** `--num-series`, `--samples-per-series`, `--batch-size`, `--concurrency`, `--duration`, `--source-file`, `--replay-speed`
- **Report:** samples/sec, p50/p95/p99 insert latency, error rate, memory usage over time
- Support configurable metric cardinality (number of unique label combinations)
- Support different metric types (counters, gauges, histograms)

**`pskctl bench accuracy`** — Query accuracy comparison:
```bash
# Compare PromSketch results against exact backend results
pskctl bench accuracy --promsketch-url http://localhost:9100 \
    --backend-url http://victoria:8428 \
    --queries-file /path/to/queries.yaml \
    --time-range "2025-01-01T00:00:00Z,2025-02-01T00:00:00Z" \
    --step 1m

# Auto-generate common query patterns
pskctl bench accuracy --promsketch-url http://localhost:9100 \
    --backend-url http://victoria:8428 \
    --auto-generate \
    --query-types "quantile,rate,avg_over_time,histogram_quantile"
```
- **Flags:** `--promsketch-url`, `--backend-url`, `--queries-file`, `--time-range`, `--step`, `--auto-generate`, `--query-types`, `--output-format`
- For each query, issue the same query to both PromSketch and backend, compare results
- **Per-query report:** relative error, absolute error, latency (sketch vs. backend)
- **Aggregate report:** mean/median/p95/max relative error, % of queries within error bounds (<1%, <5%)
- Support custom query sets via YAML:
  ```yaml
  queries:
    - expr: 'histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))'
      description: "P99 latency"
    - expr: 'avg_over_time(node_cpu_seconds_total{mode="idle"}[1h])'
      description: "1h avg CPU idle"
  ```
- `--output-format` — Output as `json`, `csv`, or `table` for CI/benchmarking pipelines

#### `pskctl check` — Configuration Validation

```bash
# Validate PromSketch-Dropin config file
pskctl check config /path/to/promsketch.yaml

# Validate sketch_targets configuration
pskctl check sketch-targets /path/to/promsketch.yaml
```
- Validates YAML syntax, required fields, endpoint reachability
- Checks sketch_targets matchers for valid PromQL/MetricsQL syntax

## Key Design Principles

- **Transparency:** From the user's perspective, PromSketch-Dropin looks exactly like a Prometheus/VictoriaMetrics instance. No client changes, no query changes, no Grafana plugin changes.
- **Extensibility:** Backend systems are pluggable. Query capability detection is registry-based.
- **Correctness first:** Always fall back to the backend when unsure if PromSketch can answer a query. Never return incorrect results.
- **Observability:** PromSketch-Dropin should expose its own Prometheus metrics (ingestion rate, query latency, sketch vs. fallback ratio, partition stats).
