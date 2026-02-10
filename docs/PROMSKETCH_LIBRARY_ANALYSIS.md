# PromSketch Library Analysis

## Summary

The PromSketch library is a Go-based implementation of approximation-first timeseries query algorithms, providing sketch-based data structures for efficient window-based aggregation queries in cloud observability systems.

**Location**: `/mydata/promsketch` → Copied to `/mydata/PromSketch-Dropin/internal/promsketch`

**Language**: Go 1.22.5

**License**: Apache 2.0

## Core API

### Main Entry Point: `PromSketches`

```go
type PromSketches struct {
    lastSeriesID atomic.Uint64
    numSeries    atomic.Uint64
    series       *sketchSeries
}
```

**Key Methods**:

1. **`NewPromSketches() *PromSketches`**
   - Creates a new PromSketches instance
   - Initializes with default stripe size (16384)

2. **`NewSketchCacheInstance(lset labels.Labels, funcName string, time_window_size int64, item_window_size int64, value_scale float64) error`**
   - Creates a new sketch instance for a specific time series
   - `lset`: Label set identifying the time series
   - `funcName`: Query function to optimize for (e.g., "avg_over_time", "quantile_over_time")
   - Returns error if sketch type is not supported

3. **`SketchInsert(lset labels.Labels, t int64, val float64) error`**
   - **PRIMARY INSERTION METHOD** for production use
   - Inserts a sample (timestamp, value) into the appropriate sketch instance
   - Thread-safe
   - Returns nil if no sketch instance exists for the series

4. **`LookUp(lset labels.Labels, funcName string, mint, maxt int64) bool`**
   - Checks if a query can be answered by the sketch
   - Returns true if the time range [mint, maxt] is covered

5. **`Eval(funcName string, lset labels.Labels, otherArgs float64, mint, maxt, cur_time int64) (Vector, annotations.Annotations)`**
   - Executes a query function on the sketch
   - Returns a Vector of results

6. **`PrintCoverage(lset labels.Labels, funcName string) (int64, int64)`**
   - Returns min and max timestamp coverage for a series

## Data Structures

### 1. Exponential Histogram for Quantiles (EHKLLx)

```go
type ExpoHistogramKLL struct {
    s_count          int           // Number of buckets
    klls             []*kll.Sketch // KLL sketches for each bucket
    max_time         []int64       // Max timestamp per bucket
    min_time         []int64       // Min timestamp per bucket
    bucketsize       []int64       // Number of samples per bucket
    k                int64         // EH parameter
    time_window_size int64         // Time window in milliseconds
    kll_k            int           // KLL sketch size parameter
    mutex            sync.RWMutex  // Concurrency control
}
```

**Purpose**: Approximate quantile queries (min, max, quantile_over_time)

**Key Methods**:
- `Update(time int64, value float64)`: Insert a sample
- `Cover(mint, maxt int64) bool`: Check if time range is covered
- `QueryIntervalMergeKLL(t1, t2 int64) *kll.Sketch`: Merge buckets for query

**Configuration**:
```go
type EHKLLConfig struct {
    K                int64  // EH bucket limit (default: 50)
    Kll_k            int    // KLL sketch size (default: 256)
    Time_window_size int64  // Window in milliseconds
}
```

### 2. Exponential Histogram for UnivMon (EHUniv)

```go
type ExpoHistogramUnivOptimized struct {
    // UnivMon sketch for distinct/entropy/L1/L2 queries
    s_count          int
    univs            []*UnivSketch
    k                int64
    time_window_size int64
    mutex            sync.Mutex
}
```

**Purpose**: Cardinality, entropy, L1, L2 norms

**Key Methods**:
- `Update(time int64, value float64)`: Insert a sample
- `QueryIntervalMergeUniv(t1, t2, cur_time int64)`: Query merged UnivMon

### 3. Uniform Sampling

```go
type UniformSampling struct {
    Arr              []Sample  // Sample array
    Max_size         int       // Maximum samples to keep
    Time_window_size int64     // Window size
    Sampling_rate    float64   // Sampling probability
    mutex            sync.RWMutex
}
```

**Purpose**: Average, sum, count, stddev, stdvar queries

**Key Methods**:
- `Insert(t int64, x float64)`: Insert with sampling
- `QueryAvg(t1, t2 int64) float64`
- `QuerySum(t1, t2 int64) float64`
- `QueryCount(t1, t2 int64) float64`
- `QueryStddev(t1, t2 int64) float64`

**Configuration**:
```go
type SamplingConfig struct {
    Sampling_rate    float64  // e.g., 0.2 = 20% sampling
    Time_window_size int64
    Max_size         int      // Max samples to store
}
```

## Supported Query Functions

The library maps PromQL-style functions to sketch types:

```go
var funcSketchMap = map[string]([]SketchType){
    "avg_over_time":      {USampling},
    "count_over_time":    {USampling},
    "entropy_over_time":  {EHUniv},
    "max_over_time":      {EHKLL},
    "min_over_time":      {EHKLL},
    "stddev_over_time":   {USampling},
    "stdvar_over_time":   {USampling},
    "sum_over_time":      {USampling},
    "sum2_over_time":     {USampling},
    "distinct_over_time": {EHUniv},
    "l1_over_time":       {EHUniv},
    "l2_over_time":       {EHUniv},
    "quantile_over_time": {EHKLL},
}
```

**Query Function Implementations** (in `functions.go`):
- Each function takes: `(ctx, series *memSeries, c float64, t1, t2, t int64)`
- Returns: `Vector` (slice of `Sample`)

## Time Series Management

### Label-based Indexing

```go
type memSeries struct {
    id              TSId
    lset            labels.Labels      // Prometheus label set
    sketchInstances *SketchInstances   // Associated sketches
    oldestTimestamp int64
}
```

- Each time series is identified by a `labels.Labels` set
- Hash-based lookup for fast series retrieval
- Stripe locking for concurrent access

### Sketch Instance Lifecycle

1. **Creation**: When `NewSketchCacheInstance()` is called
2. **Insertion**: `SketchInsert()` adds samples to existing instances
3. **Query**: `LookUp()` checks coverage, `Eval()` executes query
4. **Cleanup**: `StopBackground()` cleans up expired buckets

## Integration Points for PromSketch-Dropin

### 1. Ingestion Pipeline

```go
// For each incoming sample:
func IngestSample(metricName string, labels labels.Labels, timestamp int64, value float64) {
    // 1. Check if metric matches sketch_targets config
    if matcher.Matches(labels) {
        // 2. Insert into PromSketch instance
        promsketch.SketchInsert(labels, timestamp, value)
    }

    // 3. Forward to backend (always, for full retention)
    backend.Write(labels, timestamp, value)
}
```

### 2. Query Router

```go
// For each query:
func RouteQuery(expr string, mint, maxt int64) Result {
    // 1. Parse query to extract function and labels
    funcName, labels := parseQuery(expr)

    // 2. Check if sketch can answer
    if promsketch.LookUp(labels, funcName, mint, maxt) {
        // 3. Execute sketch query
        return promsketch.Eval(funcName, labels, phi, mint, maxt, time.Now().UnixMilli())
    }

    // 4. Fallback to backend
    return backend.Query(expr, mint, maxt)
}
```

### 3. Sketch Instance Creation

Based on `sketch_targets` configuration:

```go
func CreateSketchInstance(labels labels.Labels, match SketchTargetConfig) error {
    // Determine which functions this target needs
    // For HTTP latency metrics: quantile_over_time, avg_over_time
    functions := []string{"quantile_over_time", "avg_over_time"}

    for _, funcName := range functions {
        err := promsketch.NewSketchCacheInstance(
            labels,
            funcName,
            match.EHParams.WindowSize * 1000, // Convert to ms
            100000, // item_window_size (for sampling)
            1.0,    // value_scale
        )
        if err != nil {
            return err
        }
    }
    return nil
}
```

## Dependencies

From `go.mod`:

**Core Dependencies**:
- `github.com/zzylol/prometheus-sketches` - Prometheus integration types
- `github.com/zzylol/go-kll` - KLL quantile sketch
- `github.com/DataDog/sketches-go` - DDSketch (unused in active code)

**Hashing**:
- `github.com/OneOfOne/xxhash`
- `github.com/cespare/xxhash`
- `github.com/spaolacci/murmur3`

**Data Structures**:
- `github.com/RoaringBitmap/roaring/v2` - Compressed bitmaps

**Prometheus Ecosystem**:
- `github.com/zzylol/VictoriaMetrics` - VictoriaMetrics types
- `github.com/prometheus/client_model`
- `github.com/prometheus/common`

## Thread Safety

- All sketch data structures use `sync.RWMutex` or `sync.Mutex`
- Series lookup uses stripe-based locking (16k stripes)
- Safe for concurrent insertion and queries

## Memory Characteristics

- **EHKLLx**: ~O(K * kll_k) per series
  - Default K=50, kll_k=256 → ~12KB per series
- **EHUniv**: ~O(K * UnivMon_size) per series
- **Uniform Sampling**: O(max_size * 16 bytes)
  - Default max_size depends on sampling_rate * item_window_size

## Limitations and Considerations

1. **Approximate Results**: All sketch queries return approximate results with bounded error
2. **Time Window Constraint**: Queries must fall within the configured time_window_size
3. **No Historical Backfill by Default**: Sketches start fresh; need separate backfill mechanism
4. **Function-Specific Sketches**: Each query function requires specific sketch types
5. **Memory Management**: No built-in memory limits; need external control

## Recommendations for PromSketch-Dropin

1. **Default Sketch Configuration**:
   ```yaml
   eh_params:
     window_size: 1800  # 30 minutes
     k: 50
     kll_k: 256
   sampling_params:
     sampling_rate: 0.2
     max_size: 10000
   ```

2. **Selective Sketch Creation**:
   - Only create sketches for high-cardinality or frequently-queried metrics
   - Use `sketch_targets` config to control overhead

3. **Query Coverage Checking**:
   - Always call `LookUp()` before `Eval()` to avoid errors
   - Implement dynamic window expansion with `UpdateWindow()`

4. **Concurrency**:
   - Leverage stripe-based partitioning (num_partitions config)
   - Use goroutines for parallel sketch queries across partitions

5. **Monitoring**:
   - Expose metrics on sketch memory usage
   - Track sketch hit rate (coverage) vs. backend fallback rate
