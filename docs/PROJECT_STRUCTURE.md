# PromSketch-Dropin Project Structure

## Overview

PromSketch-Dropin is a sketch-augmented metrics proxy that provides drop-in compatibility with the Prometheus/VictoriaMetrics ecosystem while leveraging approximate query techniques for improved performance.

## Directory Structure

```
PromSketch-Dropin/
├── cmd/
│   ├── promsketch-dropin/       # Main server binary
│   │   └── main.go              # Entry point for the server
│   └── pskctl/                  # CLI tool for management and benchmarking
│       └── main.go              # Entry point for the CLI
│
├── internal/                    # Private application code
│   ├── promsketch/              # Copy of the PromSketch library
│   │   ├── promsketches.go      # Main PromSketch API
│   │   ├── ExponentialHistogram.go  # EH data structures
│   │   ├── uniformsampling.go   # Uniform sampling implementation
│   │   ├── functions.go         # Query functions
│   │   └── ...                  # Other sketch components
│   │
│   ├── config/                  # Configuration management
│   │   └── config.go            # Config parsing and validation
│   │
│   ├── ingestion/               # Ingestion layer
│   │   ├── remotewrite/         # Prometheus remote write receiver
│   │   ├── scrape/              # Built-in scrape manager
│   │   └── pipeline/            # Ingestion pipeline (sketch insertion + backend forwarding)
│   │
│   ├── storage/                 # PromSketch instance management
│   │   ├── partition/           # Consistent hashing and partitioning
│   │   ├── matcher/             # Metric matching for sketch target selection
│   │   └── lifecycle/           # Instance creation and cleanup
│   │
│   ├── query/                   # Query API and routing
│   │   ├── api/                 # HTTP API handlers (/api/v1/query, /api/v1/query_range, etc.)
│   │   ├── router/              # Query routing logic (sketch vs. fallback)
│   │   ├── parser/              # MetricsQL/PromQL parser integration
│   │   └── engine/              # Sketch query execution engine
│   │
│   └── backend/                 # Backend abstraction
│       ├── interface.go         # Backend interface definition
│       ├── victoriametrics/     # VictoriaMetrics backend implementation
│       ├── prometheus/          # Prometheus backend implementation
│       └── forwarder/           # Backend sample forwarding logic
│
├── pkg/                         # Public libraries (if any)
│
├── grafana-plugin/              # Grafana datasource plugin
│   ├── src/                     # Plugin source code
│   ├── package.json             # Node.js dependencies
│   └── README.md                # Plugin documentation
│
├── configs/                     # Configuration examples
│   ├── promsketch-dropin.example.yaml  # Example config
│   └── scrape_config.example.yaml      # Example scrape config
│
├── docker/                      # Docker and deployment files
│   ├── Dockerfile               # Dockerfile for promsketch-dropin
│   └── compose/                 # Docker Compose examples
│       └── docker-compose.yaml  # Full stack demo
│
├── scripts/                     # Build and deployment scripts
│   ├── build.sh                 # Build script
│   └── test.sh                  # Test script
│
├── docs/                        # Documentation
│   ├── PROJECT_STRUCTURE.md     # This file
│   ├── ARCHITECTURE.md          # Architecture overview
│   ├── API.md                   # API documentation
│   └── DEVELOPMENT.md           # Development guide
│
├── go.mod                       # Go module definition
├── go.sum                       # Go module checksums
├── LICENSE                      # Apache 2.0 License
└── README.md                    # Project README

```

## Component Overview

### 1. Ingestion Layer (`internal/ingestion/`)

Handles two ingestion modes:
- **Remote Write Receiver**: Accepts Prometheus remote write at `/api/v1/write`
- **Built-in Scrape Manager**: Directly scrapes configured targets

Both modes feed into the same pipeline that:
1. Inserts samples into PromSketch instances (based on sketch targets config)
2. Forwards all raw samples to the backend storage system

### 2. Storage Layer (`internal/storage/`)

Manages PromSketch instances:
- **Partitioning**: Consistent hashing by metric name across N partitions
- **Matching**: Evaluates incoming metrics against `sketch_targets` config
- **Lifecycle**: Creates new instances for matching metrics, expires stale instances

### 3. Query Layer (`internal/query/`)

Implements Prometheus-compatible query API:
- **API Handlers**: `/api/v1/query`, `/api/v1/query_range`, `/api/v1/series`, `/api/v1/labels`, etc.
- **Query Router**: Analyzes queries to determine if sketch can answer
  - If **YES**: Route to sketch engine
  - If **NO**: Proxy to backend
- **Parser**: MetricsQL/PromQL parsing (likely reusing VictoriaMetrics parser)
- **Engine**: Executes sketch-based queries

### 4. Backend Layer (`internal/backend/`)

Pluggable backend abstraction:
- **Interface**: Common interface for all backends
- **Implementations**: VictoriaMetrics, Prometheus, (future: InfluxDB, ClickHouse)
- **Forwarder**: Forwards ingested samples to backend via remote write

### 5. PromSketch Library (`internal/promsketch/`)

Copy of the original `/mydata/promsketch` library, providing:
- **Core API**: `PromSketches` struct for managing sketch instances
- **Data Structures**: Exponential Histogram (EH), UnivMon, KLL, Uniform Sampling
- **Query Functions**: `avg_over_time`, `quantile_over_time`, `entropy_over_time`, etc.

### 6. CLI Tool (`cmd/pskctl/`)

Unified CLI for operations:
- `pskctl backfill`: Backfill historical data from backends
- `pskctl bench insert`: Insertion throughput benchmarking
- `pskctl bench accuracy`: Query accuracy comparison
- `pskctl check`: Configuration validation

### 7. Grafana Plugin (`grafana-plugin/`)

Custom Grafana datasource plugin:
- Based on Prometheus datasource type
- Branded as "PromSketch"
- Extensible for future sketch-specific features

## Key Design Decisions

### Language: Go
- PromSketch library is written in Go
- Natural fit for Prometheus ecosystem
- Rich library support (Prometheus client libs, MetricsQL parser)
- Excellent concurrency and performance

### Internal Package Strategy
- Copy PromSketch library into `internal/promsketch/`
- Allows independent development
- No dependency on external PromSketch repo
- Can modify as needed for integration

### Configuration-Driven Sketch Creation
- Not all time series get sketch instances
- `sketch_targets` config controls which metrics are sketched
- Supports exact matches, regex, label matchers, wildcard
- Unmatched metrics still forwarded to backend (no overhead)

### Backend Abstraction
- Pluggable interface for multiple backends
- Easy to add new backends (InfluxDB, ClickHouse)
- Separation of concerns: ingestion, sketch, backend

## Next Steps (Phase 2-8)

1. **Phase 2**: Implement ingestion layer (remote write + backend forwarding)
2. **Phase 3**: Wire up PromSketch insertion on ingestion
3. **Phase 4**: Implement query API endpoints with router
4. **Phase 5**: Test with Grafana
5. **Phase 6**: Build `pskctl` CLI
6. **Phase 7**: Docker-compose demo setup
7. **Phase 8**: Built-in scrape manager (optional)

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for build instructions, testing, and contribution guidelines.
