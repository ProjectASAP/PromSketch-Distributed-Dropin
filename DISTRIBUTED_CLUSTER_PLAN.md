# Distributed PromSketch-Dropin Cluster Implementation Plan

## Context

PromSketch-Dropin currently runs as a monolithic service with in-memory sketch partitions. This limits scalability and creates a single point of failure. The goal is to redesign it as a distributed cluster similar to VictoriaMetrics-cluster's vmsketch architecture, enabling:

- **Horizontal scalability**: Add more nodes to handle increased load
- **High availability**: Tolerate node failures through replication
- **Explicit partition ownership**: Each node owns specific partition ranges for predictable data distribution

This change transforms a single-process architecture into a three-component distributed system.

---

## Architecture Overview

The distributed cluster consists of three components:

```
┌─────────────────────────────────────────────────────────────┐
│                    CLIENT (Prometheus/Grafana)               │
└───────────────┬─────────────────────────┬───────────────────┘
                │                         │
        (remote write)              (PromQL query)
                │                         │
                ▼                         ▼
        ┌───────────────┐         ┌──────────────┐
        │  pskinsert    │         │  pskquery    │  (Stateless)
        │  (Router)     │         │  (Merger)    │  (Load Balanced)
        └───────┬───────┘         └──────┬───────┘
                │                        │
                │ consistent hash        │ fan-out
                │ + replication          │ to all nodes
                │                        │
        ┌───────┴────────────────────────┴────────┐
        │                                         │
        ▼                                         ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ psksketch-1  │  │ psksketch-2  │  │ psksketch-3  │  (Stateful)
│ Partitions:  │  │ Partitions:  │  │ Partitions:  │
│   0-5        │  │   6-11       │  │   12-15      │
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │                 │                 │
       └─────────────────┴─────────────────┘
                         │
                         ▼ (forward raw data)
                 ┌───────────────┐
                 │ VictoriaMetrics│
                 │   (Backend)    │
                 └───────────────┘
```

### Component Responsibilities

1. **pskinsert**: Accepts remote write, routes to psksketch nodes via consistent hashing, forwards raw data to backend
2. **psksketch**: Stores sketches for assigned partition range, serves gRPC requests (Insert/LookUp/Eval)
3. **pskquery**: Accepts PromQL queries, fan-out to all psksketch nodes, merges results, fallback to backend

---

## Key Design Decisions (User Requirements)

Based on user input:

1. ✅ **Backend forwarding**: From pskinsert only (centralized, avoid duplicates)
2. ✅ **Replication factor**: Default 2 (tolerate single node failure)
3. ✅ **Discovery**: Static config + Kubernetes headless service support
4. ✅ **Partitioning**: Partition ranges per node (explicit assignment, e.g., node-1: 0-5, node-2: 6-11)

---

## Implementation Steps

### Phase 1: Protocol & Infrastructure (Foundation)

**1.1 Define gRPC Protocol**

Create `/mydata/PromSketch-Dropin/api/psksketch/v1/psksketch.proto`:

```protobuf
syntax = "proto3";
package psksketch.v1;

service SketchService {
  rpc Insert(InsertRequest) returns (InsertResponse);
  rpc LookUp(LookUpRequest) returns (LookUpResponse);
  rpc Eval(EvalRequest) returns (EvalResponse);
  rpc Health(HealthRequest) returns (HealthResponse);
  rpc Stats(StatsRequest) returns (StatsResponse);
}

message Label {
  string name = 1;
  string value = 2;
}

message InsertRequest {
  repeated Label labels = 1;
  int64 timestamp = 2;
  double value = 3;
}

message InsertResponse {
  bool success = 1;
  string error = 2;
}

message LookUpRequest {
  repeated Label labels = 1;
  string func_name = 2;
  int64 min_time = 3;
  int64 max_time = 4;
}

message LookUpResponse {
  bool can_answer = 1;
}

message EvalRequest {
  string func_name = 1;
  repeated Label labels = 2;
  double other_args = 3;
  int64 min_time = 4;
  int64 max_time = 5;
  int64 cur_time = 6;
}

message Sample {
  int64 timestamp = 1;
  double value = 2;
}

message EvalResponse {
  repeated Sample samples = 1;
  string error = 2;
}

message HealthRequest {}
message HealthResponse {
  bool healthy = 1;
  string version = 2;
}

message StatsRequest {}
message StatsResponse {
  uint64 total_series = 1;
  uint64 sketched_series = 2;
  uint64 samples_inserted = 3;
}
```

**Files to create:**
- `api/psksketch/v1/psksketch.proto` - Protocol definition
- `Makefile` target for `protoc` code generation

**1.2 Implement Consistent Hashing with Partition Mapping**

Create `/mydata/PromSketch-Dropin/internal/cluster/hash/partitioner.go`:

Since user wants partition ranges per node (not all partitions per node), the design is:
1. Hash metric name to partition ID (0-15 with 16 total partitions)
2. Look up which node owns that partition ID
3. Select that node + replication nodes

```go
package hash

import "github.com/cespare/xxhash/v2"

type PartitionMapper struct {
    totalPartitions int
    nodeAssignments map[int]string  // partition ID -> node ID
    nodes           map[string]*Node
}

type Node struct {
    ID             string
    Address        string
    PartitionStart int  // inclusive
    PartitionEnd   int  // exclusive
}

// GetPartitionID returns partition ID for a metric name
func (pm *PartitionMapper) GetPartitionID(metricName string) int {
    hash := xxhash.Sum64String(metricName)
    return int(hash % uint64(pm.totalPartitions))
}

// GetNodesForPartition returns primary + replica nodes for a partition
func (pm *PartitionMapper) GetNodesForPartition(partitionID int, replicaCount int) []*Node {
    // Get primary node that owns this partition
    primaryNodeID := pm.nodeAssignments[partitionID]
    primary := pm.nodes[primaryNodeID]

    // Select replica nodes (rendezvous hash for deterministic selection)
    // Details omitted - use highest random weight among other nodes
    replicas := pm.selectReplicaNodes(partitionID, primary, replicaCount)

    return append([]*Node{primary}, replicas...)
}
```

**Files to create:**
- `internal/cluster/hash/partitioner.go` - Partition mapping logic
- `internal/cluster/hash/partitioner_test.go` - Unit tests

**Reuse from existing codebase:**
- `/mydata/PromSketch-Dropin/internal/storage/partition/partition.go` - FNV-64a hashing (replace with xxhash)

**1.3 Implement Node Discovery**

Create `/mydata/PromSketch-Dropin/internal/cluster/discovery/discovery.go`:

```go
package discovery

type NodeDiscovery interface {
    Discover() ([]*Node, error)
    Watch(ctx context.Context, callback func([]*Node))
}

type StaticDiscovery struct {
    nodes []*Node
}

type KubernetesDiscovery struct {
    namespace string
    service   string
    // Uses K8s API to discover pods
}
```

**Files to create:**
- `internal/cluster/discovery/discovery.go` - Interface definition
- `internal/cluster/discovery/static.go` - Static node list
- `internal/cluster/discovery/kubernetes.go` - K8s headless service discovery

**1.4 Implement Circuit Breaker & Health Checking**

Create `/mydata/PromSketch-Dropin/internal/cluster/health/checker.go`:

```go
package health

type CircuitBreaker struct {
    state         State  // Closed, Open, HalfOpen
    failures      int
    threshold     int
    timeout       time.Duration
    lastFailTime  time.Time
}

type HealthChecker struct {
    nodes          map[string]*NodeHealth
    checkInterval  time.Duration
    checkTimeout   time.Duration
}

type NodeHealth struct {
    nodeID         string
    healthy        bool
    circuitBreaker *CircuitBreaker
}
```

**Files to create:**
- `internal/cluster/health/checker.go` - Health checking implementation
- `internal/cluster/health/circuit_breaker.go` - Circuit breaker pattern

---

### Phase 2: psksketch (Sketch Storage Node)

**2.1 Extract and Adapt Storage Logic**

Create `/mydata/PromSketch-Dropin/cmd/psksketch/main.go`:

```go
package main

func main() {
    // 1. Load config (including partition range assignment)
    // 2. Initialize Storage (reuse existing)
    // 3. Start gRPC server
    // 4. Start HTTP server for metrics/health
    // 5. Register with discovery (if using service registry)
    // 6. Graceful shutdown
}
```

**Reuse from existing codebase:**
- `/mydata/PromSketch-Dropin/internal/storage/storage.go` - Core storage logic (minimal changes)
- `/mydata/PromSketch-Dropin/internal/storage/partition/partition.go` - Local partitioning
- `/mydata/PromSketch-Dropin/internal/storage/matcher/matcher.go` - Target matching
- `/mydata/PromSketch-Dropin/internal/promsketch/promsketches.go` - Sketch instances

**2.2 Implement gRPC Server**

Create `/mydata/PromSketch-Dropin/internal/psksketch/server/grpc.go`:

```go
package server

type SketchServer struct {
    storage *storage.Storage
    config  *config.SketchConfig
}

func (s *SketchServer) Insert(ctx context.Context, req *pb.InsertRequest) (*pb.InsertResponse, error) {
    // Convert pb.Label to labels.Labels
    lbls := convertLabels(req.Labels)

    // Call existing storage.Insert()
    err := s.storage.Insert(lbls, req.Timestamp, req.Value)

    return &pb.InsertResponse{Success: err == nil, Error: errorString(err)}, nil
}

func (s *SketchServer) LookUp(ctx context.Context, req *pb.LookUpRequest) (*pb.LookUpResponse, error) {
    lbls := convertLabels(req.Labels)
    canAnswer := s.storage.LookUp(lbls, req.FuncName, req.MinTime, req.MaxTime)
    return &pb.LookUpResponse{CanAnswer: canAnswer}, nil
}

func (s *SketchServer) Eval(ctx context.Context, req *pb.EvalRequest) (*pb.EvalResponse, error) {
    lbls := convertLabels(req.Labels)
    result, err := s.storage.Eval(req.FuncName, lbls, req.OtherArgs, req.MinTime, req.MaxTime, req.CurTime)

    // Convert promsketch.Vector to pb.EvalResponse
    samples := make([]*pb.Sample, len(result))
    for i, s := range result {
        samples[i] = &pb.Sample{Timestamp: s.T, Value: s.F}
    }

    return &pb.EvalResponse{Samples: samples, Error: errorString(err)}, nil
}
```

**Files to create:**
- `cmd/psksketch/main.go` - Entry point
- `internal/psksketch/server/grpc.go` - gRPC service implementation
- `internal/psksketch/config/config.go` - psksketch-specific config

**2.3 Configuration**

Create `/mydata/PromSketch-Dropin/configs/psksketch.example.yaml`:

```yaml
psksketch:
  # Node identity
  node:
    id: "psksketch-1"

    # Partition range this node owns
    partition_start: 0   # inclusive
    partition_end: 6     # exclusive (owns 0-5)

  # gRPC server
  server:
    listen_address: ":8481"
    max_recv_msg_size: 10485760  # 10MB
    max_send_msg_size: 10485760

  # HTTP server (metrics, health)
  http:
    listen_address: ":8482"

  # Storage (same as monolithic)
  storage:
    num_partitions: 16  # Total partitions across cluster

    targets:
      - match: '{__name__=~"http_.*"}'
        eh_params:
          window_size: 3600
      - match: '{__name__=~"node_.*"}'

    defaults:
      eh_params:
        window_size: 1800
        k: 50
        kll_k: 256

    memory_limit: "8GB"
```

---

### Phase 3: pskinsert (Ingestion Router)

**3.1 Implement Routing Logic**

Create `/mydata/PromSketch-Dropin/cmd/pskinsert/main.go`:

```go
package main

func main() {
    // 1. Load config
    // 2. Initialize node discovery
    // 3. Initialize partitioner (partition-to-node mapping)
    // 4. Initialize gRPC client pool for psksketch nodes
    // 5. Initialize backend forwarder
    // 6. Start HTTP server for remote write
    // 7. Start health checker
    // 8. Graceful shutdown
}
```

Create `/mydata/PromSketch-Dropin/internal/pskinsert/router/router.go`:

```go
package router

type Router struct {
    partitioner    *hash.PartitionMapper
    clients        map[string]pb.SketchServiceClient
    healthChecker  *health.HealthChecker
    replicaCount   int
}

func (r *Router) Insert(lbls labels.Labels, timestamp int64, value float64) error {
    metricName := lbls.Get("__name__")

    // 1. Get partition ID
    partitionID := r.partitioner.GetPartitionID(metricName)

    // 2. Get primary + replica nodes for this partition
    nodes := r.partitioner.GetNodesForPartition(partitionID, r.replicaCount)

    // 3. Send Insert RPC to each node (in parallel)
    var wg sync.WaitGroup
    var errs []error

    for _, node := range nodes {
        if !r.healthChecker.IsHealthy(node.ID) {
            continue  // Skip unhealthy nodes
        }

        wg.Add(1)
        go func(n *hash.Node) {
            defer wg.Done()

            client := r.clients[n.ID]
            req := &pb.InsertRequest{
                Labels:    convertLabels(lbls),
                Timestamp: timestamp,
                Value:     value,
            }

            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()

            _, err := client.Insert(ctx, req)
            if err != nil {
                r.healthChecker.RecordFailure(n.ID)
                errs = append(errs, err)
            }
        }(node)
    }

    wg.Wait()

    // Succeed if at least one node accepted (quorum = 1)
    if len(errs) == len(nodes) && len(nodes) > 0 {
        return fmt.Errorf("all nodes failed: %v", errs)
    }

    return nil
}
```

**Reuse from existing codebase:**
- `/mydata/PromSketch-Dropin/internal/ingestion/remotewrite/handler.go` - HTTP handler
- `/mydata/PromSketch-Dropin/internal/ingestion/pipeline/pipeline.go` - Pipeline structure
- `/mydata/PromSketch-Dropin/internal/backend/forwarder.go` - Backend forwarding (unchanged)

**Files to create:**
- `cmd/pskinsert/main.go` - Entry point
- `internal/pskinsert/router/router.go` - Routing logic with consistent hashing
- `internal/pskinsert/client/pool.go` - gRPC client pool management
- `internal/pskinsert/config/config.go` - pskinsert-specific config

**3.2 Configuration**

Create `/mydata/PromSketch-Dropin/configs/pskinsert.example.yaml`:

```yaml
pskinsert:
  # HTTP server (remote write endpoint)
  server:
    listen_address: ":8480"
    read_timeout: 30s
    write_timeout: 30s

  # Cluster configuration
  cluster:
    # Total partitions in cluster
    total_partitions: 16

    # Replication factor
    replication_factor: 2

    # Node discovery
    discovery:
      type: "static"  # or "kubernetes"

      # Static nodes with partition assignments
      static_nodes:
        - id: "psksketch-1"
          address: "psksketch-1:8481"
          partition_start: 0
          partition_end: 6
        - id: "psksketch-2"
          address: "psksketch-2:8481"
          partition_start: 6
          partition_end: 12
        - id: "psksketch-3"
          address: "psksketch-3:8481"
          partition_start: 12
          partition_end: 16

      # Kubernetes discovery
      kubernetes:
        namespace: "default"
        service: "psksketch"
        port: 8481

    # Health checking
    health_check:
      interval: 10s
      timeout: 5s
      failure_threshold: 3

    # Circuit breaker
    circuit_breaker:
      failure_threshold: 5
      timeout: 30s

  # Backend forwarding (unchanged from monolithic)
  backend:
    type: "victoriametrics"
    url: "http://victoria:8428"
    remote_write_url: "http://victoria:8428/api/v1/write"
    batch_size: 1000
    flush_interval: 5s
    max_retries: 3
```

---

### Phase 4: pskquery (Query Router & Merger)

**4.1 Implement Fan-out Query Logic**

Create `/mydata/PromSketch-Dropin/cmd/pskquery/main.go`:

```go
package main

func main() {
    // 1. Load config
    // 2. Initialize node discovery
    // 3. Initialize gRPC client pool for psksketch nodes
    // 4. Initialize backend client
    // 5. Initialize query router
    // 6. Start HTTP server for query endpoints
    // 7. Graceful shutdown
}
```

Create `/mydata/PromSketch-Dropin/internal/pskquery/merger/merger.go`:

```go
package merger

type Merger struct {
    clients       map[string]pb.SketchServiceClient
    backend       backend.Backend
    capabilities  *capabilities.Registry
    parser        *parser.Parser
}

func (m *Merger) Query(ctx context.Context, query string, ts int64) (*QueryResult, error) {
    // 1. Parse query
    queryInfo, err := m.parser.Parse(query)
    if err != nil {
        return nil, err
    }

    // 2. Check if sketches can handle this query
    capability := m.capabilities.CanHandle(queryInfo)
    if !capability.CanHandleWithSketches {
        // Fall back to backend
        return m.backend.Query(ctx, query, ts)
    }

    // 3. Fan-out to all psksketch nodes in parallel
    type nodeResult struct {
        nodeID  string
        samples []*pb.Sample
        err     error
    }

    resultsCh := make(chan nodeResult, len(m.clients))

    lbls := buildLabelsFromQuery(queryInfo)

    for nodeID, client := range m.clients {
        go func(id string, c pb.SketchServiceClient) {
            req := &pb.EvalRequest{
                FuncName:  queryInfo.FunctionName,
                Labels:    convertLabels(lbls),
                OtherArgs: 0.5,  // default quantile
                MinTime:   ts - queryInfo.Range,
                MaxTime:   ts,
                CurTime:   ts,
            }

            resp, err := c.Eval(ctx, req)
            resultsCh <- nodeResult{nodeID: id, samples: resp.GetSamples(), err: err}
        }(nodeID, client)
    }

    // 4. Collect and merge results
    var allSamples []*pb.Sample
    var errs []error

    for i := 0; i < len(m.clients); i++ {
        res := <-resultsCh
        if res.err != nil {
            errs = append(errs, res.err)
            continue
        }
        allSamples = append(allSamples, res.samples...)
    }

    // 5. If no successful results, fall back to backend
    if len(allSamples) == 0 && len(errs) > 0 {
        return m.backend.Query(ctx, query, ts)
    }

    // 6. Merge samples (sketches should be aggregatable)
    // For now, simple approach: take non-empty result
    // TODO: Implement proper sketch merging

    return &QueryResult{
        Source: "sketch",
        Data:   convertSamplesToVector(allSamples),
    }, nil
}
```

**Reuse from existing codebase:**
- `/mydata/PromSketch-Dropin/internal/query/router/router.go` - Query routing structure
- `/mydata/PromSketch-Dropin/internal/query/parser/parser.go` - Query parsing (unchanged)
- `/mydata/PromSketch-Dropin/internal/query/capabilities/registry.go` - Capability detection (unchanged)
- `/mydata/PromSketch-Dropin/internal/query/api/handlers.go` - HTTP handlers (minimal changes)
- `/mydata/PromSketch-Dropin/internal/backend/*` - Backend fallback (unchanged)

**Files to create:**
- `cmd/pskquery/main.go` - Entry point
- `internal/pskquery/merger/merger.go` - Fan-out query and result merging
- `internal/pskquery/client/pool.go` - gRPC client pool management
- `internal/pskquery/config/config.go` - pskquery-specific config

**4.2 Configuration**

Create `/mydata/PromSketch-Dropin/configs/pskquery.example.yaml`:

```yaml
pskquery:
  # HTTP server (query endpoints)
  server:
    listen_address: ":8480"
    read_timeout: 60s
    write_timeout: 60s

  # Cluster configuration
  cluster:
    # Node discovery (same as pskinsert)
    discovery:
      type: "static"
      static_nodes:
        - id: "psksketch-1"
          address: "psksketch-1:8481"
        - id: "psksketch-2"
          address: "psksketch-2:8481"
        - id: "psksketch-3"
          address: "psksketch-3:8481"

      kubernetes:
        namespace: "default"
        service: "psksketch"
        port: 8481

    # Query settings
    query_timeout: 30s
    max_concurrent_queries: 100

  # Backend (for fallback and metadata queries)
  backend:
    type: "victoriametrics"
    url: "http://victoria:8428"
    timeout: 60s

  # Query behavior
  query:
    enable_fallback: true
    fallback_timeout: 60s
```

---

### Phase 5: Docker Deployment

**5.1 Create Docker Compose for Cluster**

Create `/mydata/PromSketch-Dropin/docker-compose.cluster.yml`:

```yaml
version: '3.8'

services:
  # Backend storage
  victoriametrics:
    image: victoriametrics/victoria-metrics:latest
    container_name: victoriametrics
    ports:
      - "8428:8428"
    volumes:
      - vmdata:/victoria-metrics-data
    command:
      - "--storageDataPath=/victoria-metrics-data"
      - "--httpListenAddr=:8428"
    networks:
      - promsketch-cluster

  # psksketch nodes (3 instances with different partition ranges)
  psksketch-1:
    build:
      context: .
      dockerfile: Dockerfile.psksketch
    container_name: psksketch-1
    ports:
      - "8481:8481"
      - "8491:8482"
    volumes:
      - ./configs/psksketch-1.yaml:/etc/promsketch/config.yaml:ro
    command:
      - "--config.file=/etc/promsketch/config.yaml"
    networks:
      - promsketch-cluster

  psksketch-2:
    build:
      context: .
      dockerfile: Dockerfile.psksketch
    container_name: psksketch-2
    ports:
      - "8483:8481"
      - "8493:8482"
    volumes:
      - ./configs/psksketch-2.yaml:/etc/promsketch/config.yaml:ro
    command:
      - "--config.file=/etc/promsketch/config.yaml"
    networks:
      - promsketch-cluster

  psksketch-3:
    build:
      context: .
      dockerfile: Dockerfile.psksketch
    container_name: psksketch-3
    ports:
      - "8485:8481"
      - "8495:8482"
    volumes:
      - ./configs/psksketch-3.yaml:/etc/promsketch/config.yaml:ro
    command:
      - "--config.file=/etc/promsketch/config.yaml"
    networks:
      - promsketch-cluster

  # pskinsert (can scale horizontally with load balancer)
  pskinsert:
    build:
      context: .
      dockerfile: Dockerfile.pskinsert
    container_name: pskinsert
    ports:
      - "8480:8480"
    volumes:
      - ./configs/pskinsert.yaml:/etc/promsketch/config.yaml:ro
    command:
      - "--config.file=/etc/promsketch/config.yaml"
    depends_on:
      - psksketch-1
      - psksketch-2
      - psksketch-3
      - victoriametrics
    networks:
      - promsketch-cluster

  # pskquery (can scale horizontally with load balancer)
  pskquery:
    build:
      context: .
      dockerfile: Dockerfile.pskquery
    container_name: pskquery
    ports:
      - "9100:8480"
    volumes:
      - ./configs/pskquery.yaml:/etc/promsketch/config.yaml:ro
    command:
      - "--config.file=/etc/promsketch/config.yaml"
    depends_on:
      - psksketch-1
      - psksketch-2
      - psksketch-3
      - victoriametrics
    networks:
      - promsketch-cluster

  # Grafana
  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    ports:
      - "3000:3000"
    volumes:
      - grafana-storage:/var/lib/grafana
      - ./docker/grafana/provisioning:/etc/grafana/provisioning:ro
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    depends_on:
      - pskquery
    networks:
      - promsketch-cluster

  # Prometheus (for scraping and remote write)
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./docker/prometheus/prometheus-cluster.yml:/etc/prometheus/prometheus.yml:ro
    command:
      - "--config.file=/etc/prometheus/prometheus.yml"
    depends_on:
      - pskinsert
      - node-exporter
    networks:
      - promsketch-cluster

  # Node exporter (sample metrics)
  node-exporter:
    image: prom/node-exporter:latest
    container_name: node-exporter
    ports:
      - "9101:9100"
    networks:
      - promsketch-cluster

volumes:
  vmdata:
  grafana-storage:

networks:
  promsketch-cluster:
    driver: bridge
```

**5.2 Create Dockerfiles**

Create three Dockerfiles:
- `Dockerfile.psksketch` - Build psksketch binary
- `Dockerfile.pskinsert` - Build pskinsert binary
- `Dockerfile.pskquery` - Build pskquery binary

(Multi-stage builds similar to existing Dockerfile)

**5.3 Update Prometheus Config**

Create `/mydata/PromSketch-Dropin/docker/prometheus/prometheus-cluster.yml`:

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'node-exporter'
    static_configs:
      - targets: ['node-exporter:9100']

  - job_name: 'psksketch'
    static_configs:
      - targets:
        - 'psksketch-1:8482'
        - 'psksketch-2:8482'
        - 'psksketch-3:8482'

remote_write:
  - url: http://pskinsert:8480/api/v1/write
```

---

## Critical Files Summary

### New Files to Create

**gRPC Protocol:**
- `api/psksketch/v1/psksketch.proto`

**Cluster Infrastructure:**
- `internal/cluster/hash/partitioner.go` - Partition-to-node mapping
- `internal/cluster/hash/partitioner_test.go`
- `internal/cluster/discovery/discovery.go` - Node discovery interface
- `internal/cluster/discovery/static.go` - Static discovery
- `internal/cluster/discovery/kubernetes.go` - K8s discovery
- `internal/cluster/health/checker.go` - Health checking
- `internal/cluster/health/circuit_breaker.go`

**psksketch Component:**
- `cmd/psksketch/main.go`
- `internal/psksketch/server/grpc.go`
- `internal/psksketch/config/config.go`
- `configs/psksketch.example.yaml`
- `configs/psksketch-1.yaml`, `psksketch-2.yaml`, `psksketch-3.yaml`
- `Dockerfile.psksketch`

**pskinsert Component:**
- `cmd/pskinsert/main.go`
- `internal/pskinsert/router/router.go`
- `internal/pskinsert/client/pool.go`
- `internal/pskinsert/config/config.go`
- `configs/pskinsert.example.yaml`
- `Dockerfile.pskinsert`

**pskquery Component:**
- `cmd/pskquery/main.go`
- `internal/pskquery/merger/merger.go`
- `internal/pskquery/client/pool.go`
- `internal/pskquery/config/config.go`
- `configs/pskquery.example.yaml`
- `Dockerfile.pskquery`

**Deployment:**
- `docker-compose.cluster.yml`
- `docker/prometheus/prometheus-cluster.yml`

### Existing Files to Reuse (Minor Modifications)

**Storage Layer** (reuse in psksketch):
- `/mydata/PromSketch-Dropin/internal/storage/storage.go` - Wrap with gRPC
- `/mydata/PromSketch-Dropin/internal/storage/partition/partition.go` - Use xxhash instead of FNV
- `/mydata/PromSketch-Dropin/internal/storage/matcher/matcher.go` - Unchanged
- `/mydata/PromSketch-Dropin/internal/promsketch/promsketches.go` - Unchanged

**Ingestion Layer** (reuse in pskinsert):
- `/mydata/PromSketch-Dropin/internal/ingestion/remotewrite/handler.go` - Minor changes
- `/mydata/PromSketch-Dropin/internal/ingestion/pipeline/pipeline.go` - Replace storage with router
- `/mydata/PromSketch-Dropin/internal/backend/forwarder.go` - Unchanged

**Query Layer** (reuse in pskquery):
- `/mydata/PromSketch-Dropin/internal/query/parser/parser.go` - Unchanged
- `/mydata/PromSketch-Dropin/internal/query/capabilities/registry.go` - Unchanged
- `/mydata/PromSketch-Dropin/internal/query/api/handlers.go` - Minor changes
- `/mydata/PromSketch-Dropin/internal/backend/prometheus/client.go` - Unchanged
- `/mydata/PromSketch-Dropin/internal/backend/victoriametrics/client.go` - Unchanged

---

## Implementation Order

### Week 1: Foundation
1. Define gRPC protocol (`psksketch.proto`)
2. Generate Go code from protobuf
3. Implement partition mapper with static node assignments
4. Implement static node discovery
5. Implement health checker and circuit breaker

### Week 2: psksketch
1. Create psksketch binary and config
2. Implement gRPC server wrapping existing storage
3. Add health/stats endpoints
4. Test standalone with gRPC client

### Week 3: pskinsert
1. Create pskinsert binary and config
2. Implement routing logic with gRPC client pool
3. Integrate backend forwarding
4. Test insert path with multiple psksketch nodes

### Week 4: pskquery
1. Create pskquery binary and config
2. Implement fan-out query logic
3. Implement result merging
4. Add backend fallback
5. Test query path with multiple psksketch nodes

### Week 5: Integration & Testing
1. Create Docker Compose cluster setup
2. End-to-end testing (write → query)
3. Chaos testing (kill nodes, verify replication)
4. Performance benchmarking vs. monolithic

### Week 6: Kubernetes Support & Polish
1. Implement Kubernetes discovery
2. Add operational metrics
3. Documentation updates
4. Production hardening

---

## Verification Strategy

### Unit Tests
- Partition mapper: verify hash distribution, partition-to-node mapping
- Circuit breaker: state transitions, threshold triggering
- Health checker: failure detection, recovery
- gRPC services: Insert/LookUp/Eval correctness

### Integration Tests
1. **Insert Path Test**:
   - Send remote write to pskinsert
   - Verify data arrives at correct psksketch nodes (based on partition)
   - Verify replication (data on 2 nodes)
   - Verify backend forwarding

2. **Query Path Test**:
   - Send PromQL query to pskquery
   - Verify fan-out to all psksketch nodes
   - Verify result merging
   - Verify fallback when sketch unavailable

3. **Node Failure Test**:
   - Kill one psksketch node
   - Verify inserts still succeed (replica)
   - Verify queries still work (partial results)
   - Restart node, verify recovery

4. **Partition Rebalancing Test** (future):
   - Add new psksketch node
   - Update partition assignments
   - Verify data redistribution

### System Tests
1. Deploy full cluster with Docker Compose
2. Run Prometheus remote write load (1000 samples/sec)
3. Run Grafana queries
4. Verify accuracy vs. monolithic version
5. Measure latency (insert, query)
6. Measure resource usage per component

### Performance Benchmarks
- Insert throughput: compare distributed vs. monolithic
- Query latency: compare distributed vs. monolithic
- Network overhead: measure gRPC traffic
- Replication overhead: 1 vs. 2 vs. 3 replicas

---

## Trade-offs and Risks

| Decision | Trade-off | Mitigation |
|----------|-----------|------------|
| **Partition ranges per node** | Manual rebalancing when scaling | Document partition assignment strategy, provide tooling |
| **Write quorum = 1** | Weaker consistency | Acceptable for approximate sketches; backend has full data |
| **Fan-out all nodes for queries** | Higher query load | psksketch is stateful and optimized for reads |
| **gRPC vs. HTTP** | Binary protocol complexity | Better performance; use HTTP gateway if needed |
| **Static discovery initially** | Manual node registration | Add K8s support in parallel; extensible design |
| **No automatic partition migration** | Requires downtime for rebalancing | Plan for future: implement partition migration protocol |

---

## Success Criteria

1. ✅ **Functional**: Cluster accepts remote write and serves queries
2. ✅ **Fault Tolerant**: Tolerate single node failure without data loss
3. ✅ **Accurate**: Query results match monolithic version within sketch error bounds
4. ✅ **Performant**: Insert throughput ≥ monolithic, query latency ≤ 2x monolithic
5. ✅ **Scalable**: Can add more psksketch nodes to handle more data
6. ✅ **Observable**: Metrics for insert rate, query rate, node health, partition distribution

---

## Open Questions for Future

1. **Automatic partition migration**: How to reassign partitions without downtime?
2. **Sketch merging**: How to properly merge sketches from different nodes for overlapping data?
3. **Load balancing**: Should pskinsert/pskquery run behind a load balancer?
4. **Dynamic partition count**: Can we increase total partitions after deployment?
5. **Snapshot/restore**: How to backup and restore sketch state?

These can be addressed in future iterations after the core distributed system is operational.
