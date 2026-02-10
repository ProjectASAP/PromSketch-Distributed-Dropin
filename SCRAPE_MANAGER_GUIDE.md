# Scrape Manager Implementation Guide

## Overview

This guide provides a comprehensive implementation plan for a **built-in scrape manager** that would allow PromSketch-Dropin to scrape metrics directly from targets, eliminating the need for an external Prometheus instance.

**Current Status**: Not implemented (Low priority - Prometheus remote write works well)

**Effort Estimate**: High (3-4 weeks for full implementation)

**Impact**: Low (Prometheus + remote write is the recommended architecture)

---

## Why Built-in Scraping?

### **Potential Benefits**

1. **Simplified Deployment**
   - Single binary instead of Prometheus + PromSketch-Dropin
   - Reduced operational complexity
   - Fewer moving parts

2. **Direct Sketch Integration**
   - Skip remote write overhead
   - Faster sketch updates
   - Reduced network traffic

3. **Self-Contained Solution**
   - No external dependencies
   - Easier to package and distribute
   - Simpler configuration

### **Why Low Priority?**

The Prometheus + remote write architecture is **recommended** because:
- ✅ Prometheus is battle-tested for scraping
- ✅ Remote write provides reliable delivery
- ✅ Separation of concerns (scraping vs. storage)
- ✅ Can scale scraping independently
- ✅ Mature service discovery support
- ✅ Extensive metric relabeling capabilities

---

## Architecture Overview

```
┌─────────────────────────────────────────────┐
│ PromSketch-Dropin                           │
│                                             │
│  ┌──────────────┐                          │
│  │ Scrape       │                          │
│  │ Manager      │                          │
│  │              │                          │
│  │ ┌──────────┐ │                          │
│  │ │ Service  │ │  Discovers targets       │
│  │ │ Discovery│ │  (static, DNS, K8s, etc.)│
│  │ └────┬─────┘ │                          │
│  │      │       │                          │
│  │ ┌────▼─────┐ │                          │
│  │ │ Target   │ │  Manages scrape targets  │
│  │ │ Manager  │ │  (health, labels, etc.)  │
│  │ └────┬─────┘ │                          │
│  │      │       │                          │
│  │ ┌────▼─────┐ │                          │
│  │ │ Scraper  │ │  HTTP scrape, parse      │
│  │ │ Pool     │ │  metrics, convert        │
│  │ └────┬─────┘ │                          │
│  └──────┼───────┘                          │
│         │                                  │
│         ▼                                  │
│  ┌──────────────┐                          │
│  │ Ingestion    │  Same as remote write    │
│  │ Pipeline     │  path                    │
│  └──────┬───────┘                          │
│         │                                  │
│         ├──────► Storage (Sketches)        │
│         └──────► Backend Forwarder         │
│                                             │
└─────────────────────────────────────────────┘
```

---

## Implementation Steps

### **Phase 1: Core Scraping (1-2 weeks)**

#### 1.1 Scrape Configuration

```yaml
# configs/promsketch-dropin.yaml
scrape:
  global:
    scrape_interval: 15s
    scrape_timeout: 10s
    evaluation_interval: 15s

  scrape_configs:
    - job_name: 'prometheus'
      static_configs:
        - targets: ['localhost:9090']

    - job_name: 'node-exporter'
      static_configs:
        - targets:
            - 'localhost:9100'
            - 'node1:9100'
            - 'node2:9100'
          labels:
            environment: 'production'

    - job_name: 'kubernetes-pods'
      kubernetes_sd_configs:
        - role: pod
      relabel_configs:
        - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
          action: keep
          regex: true
```

#### 1.2 Scrape Manager Structure

```go
// internal/scrape/manager.go
package scrape

import (
    "context"
    "sync"
    "time"

    "github.com/prometheus/prometheus/config"
    "github.com/prometheus/prometheus/discovery"
    "github.com/prometheus/prometheus/scrape"
)

type Manager struct {
    ctx        context.Context
    cancel     context.CancelFunc
    scrapeManager *scrape.Manager
    discoveryManager *discovery.Manager
    targets    map[string]*Target
    mu         sync.RWMutex

    // Metrics
    metrics    *ScrapeMetrics
}

type ScrapeMetrics struct {
    TargetsUp        int64
    TargetsDown      int64
    SamplesScraped   int64
    ScrapesTotal     int64
    ScrapesFailed    int64
    ScrapeDuration   time.Duration
}

func NewManager(cfg *config.Config) (*Manager, error) {
    ctx, cancel := context.WithCancel(context.Background())

    m := &Manager{
        ctx:     ctx,
        cancel:  cancel,
        targets: make(map[string]*Target),
        metrics: &ScrapeMetrics{},
    }

    // Initialize Prometheus scrape manager
    m.scrapeManager = scrape.NewManager(nil, m)

    // Initialize service discovery
    m.discoveryManager = discovery.NewManager(ctx, nil)

    return m, nil
}

func (m *Manager) Start() error {
    // Start service discovery
    go m.discoveryManager.Run()

    // Start scrape manager
    go m.scrapeManager.Run(m.discoveryManager.SyncCh())

    return nil
}

func (m *Manager) Stop() error {
    m.cancel()
    m.scrapeManager.Stop()
    return nil
}

// Appender interface for Prometheus scrape manager
func (m *Manager) Appender(ctx context.Context) storage.Appender {
    return &scrapeAppender{
        manager: m,
        ctx:     ctx,
    }
}
```

#### 1.3 Target Management

```go
// internal/scrape/target.go
package scrape

import (
    "net/url"
    "time"

    "github.com/prometheus/common/model"
    "github.com/prometheus/prometheus/model/labels"
)

type Target struct {
    URL         *url.URL
    Labels      labels.Labels
    Health      TargetHealth
    LastScrape  time.Time
    LastError   error
    ScrapeCount int64

    // Metrics
    SampleCount int64
    ScrapeTime  time.Duration
}

type TargetHealth int

const (
    HealthUnknown TargetHealth = iota
    HealthUp
    HealthDown
)

func (t *Target) String() string {
    return t.URL.String()
}

func (t *Target) IsHealthy() bool {
    return t.Health == HealthUp
}

func (t *Target) UpdateHealth(health TargetHealth, err error) {
    t.Health = health
    t.LastError = err
    t.LastScrape = time.Now()
}
```

#### 1.4 Scrape Appender

```go
// internal/scrape/appender.go
package scrape

import (
    "context"

    "github.com/prometheus/prometheus/model/labels"
    "github.com/prometheus/prometheus/storage"
)

type scrapeAppender struct {
    manager *Manager
    ctx     context.Context
    samples []*sample
}

type sample struct {
    labels    labels.Labels
    timestamp int64
    value     float64
}

func (a *scrapeAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
    a.samples = append(a.samples, &sample{
        labels:    l,
        timestamp: t,
        value:     v,
    })

    return ref, nil
}

func (a *scrapeAppender) Commit() error {
    // Convert samples to remote write format
    writeReq := convertToRemoteWrite(a.samples)

    // Send to ingestion pipeline (reuse existing pipeline)
    return a.manager.sendToIngestion(writeReq)
}

func (a *scrapeAppender) Rollback() error {
    a.samples = nil
    return nil
}

func convertToRemoteWrite(samples []*sample) *prompb.WriteRequest {
    // Group samples by series
    seriesMap := make(map[string]*prompb.TimeSeries)

    for _, s := range samples {
        key := s.labels.String()

        if _, exists := seriesMap[key]; !exists {
            // Create new series
            lbls := make([]prompb.Label, 0, len(s.labels))
            for _, l := range s.labels {
                lbls = append(lbls, prompb.Label{
                    Name:  l.Name,
                    Value: l.Value,
                })
            }

            seriesMap[key] = &prompb.TimeSeries{
                Labels:  lbls,
                Samples: make([]prompb.Sample, 0),
            }
        }

        // Add sample to series
        seriesMap[key].Samples = append(seriesMap[key].Samples, prompb.Sample{
            Timestamp: s.timestamp,
            Value:     s.value,
        })
    }

    // Convert map to slice
    timeSeries := make([]prompb.TimeSeries, 0, len(seriesMap))
    for _, ts := range seriesMap {
        timeSeries = append(timeSeries, *ts)
    }

    return &prompb.WriteRequest{
        Timeseries: timeSeries,
    }
}
```

### **Phase 2: Service Discovery (1 week)**

#### 2.1 Static Configuration

```go
// internal/scrape/discovery/static.go
package discovery

import (
    "context"

    "github.com/prometheus/prometheus/discovery/targetgroup"
)

type StaticConfig struct {
    Targets []string
    Labels  map[string]string
}

func (c *StaticConfig) Discover(ctx context.Context) chan []*targetgroup.Group {
    ch := make(chan []*targetgroup.Group)

    go func() {
        defer close(ch)

        group := &targetgroup.Group{
            Targets: make([]model.LabelSet, 0),
        }

        for _, target := range c.Targets {
            labels := model.LabelSet{
                model.AddressLabel: model.LabelValue(target),
            }

            // Add custom labels
            for k, v := range c.Labels {
                labels[model.LabelName(k)] = model.LabelValue(v)
            }

            group.Targets = append(group.Targets, labels)
        }

        select {
        case ch <- []*targetgroup.Group{group}:
        case <-ctx.Done():
        }
    }()

    return ch
}
```

#### 2.2 DNS Service Discovery

```go
// internal/scrape/discovery/dns.go
package discovery

import (
    "context"
    "net"
    "time"
)

type DNSConfig struct {
    Names          []string
    RefreshInterval time.Duration
    Port           int
}

func (c *DNSConfig) Discover(ctx context.Context) chan []*targetgroup.Group {
    ch := make(chan []*targetgroup.Group)

    go func() {
        defer close(ch)

        ticker := time.NewTicker(c.RefreshInterval)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                groups := c.resolve()
                select {
                case ch <- groups:
                case <-ctx.Done():
                    return
                }
            case <-ctx.Done():
                return
            }
        }
    }()

    return ch
}

func (c *DNSConfig) resolve() []*targetgroup.Group {
    var groups []*targetgroup.Group

    for _, name := range c.Names {
        ips, err := net.LookupIP(name)
        if err != nil {
            continue
        }

        group := &targetgroup.Group{
            Source: "dns/" + name,
        }

        for _, ip := range ips {
            target := net.JoinHostPort(ip.String(), fmt.Sprintf("%d", c.Port))
            group.Targets = append(group.Targets, model.LabelSet{
                model.AddressLabel: model.LabelValue(target),
            })
        }

        groups = append(groups, group)
    }

    return groups
}
```

#### 2.3 Kubernetes Service Discovery

```go
// internal/scrape/discovery/kubernetes.go
package discovery

import (
    "context"

    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

type KubernetesConfig struct {
    Role       string // pod, service, endpoints, node
    Namespaces []string
}

func (c *KubernetesConfig) Discover(ctx context.Context) chan []*targetgroup.Group {
    // Use Prometheus's existing Kubernetes discovery
    // This is complex and would reuse Prometheus code

    ch := make(chan []*targetgroup.Group)

    go func() {
        defer close(ch)

        // Initialize Kubernetes client
        config, err := rest.InClusterConfig()
        if err != nil {
            return
        }

        clientset, err := kubernetes.NewForConfig(config)
        if err != nil {
            return
        }

        // Watch for pods/services/endpoints/nodes
        // Convert to target groups
        // Send updates to channel

        // This would be ~500 lines of code
        // Recommend using Prometheus's k8s discovery package directly
    }()

    return ch
}
```

### **Phase 3: Metrics Parsing (3-4 days)**

```go
// internal/scrape/parser.go
package scrape

import (
    "io"

    "github.com/prometheus/common/expfmt"
    "github.com/prometheus/prometheus/model/labels"
)

type MetricParser struct{}

func NewMetricParser() *MetricParser {
    return &MetricParser{}
}

func (p *MetricParser) Parse(body io.Reader, contentType string) ([]MetricFamily, error) {
    var parser expfmt.TextParser

    // Parse Prometheus text format
    metricFamilies, err := parser.TextToMetricFamilies(body)
    if err != nil {
        return nil, err
    }

    // Convert to internal format
    var families []MetricFamily
    for _, mf := range metricFamilies {
        family := MetricFamily{
            Name: *mf.Name,
            Type: mf.GetType().String(),
        }

        for _, m := range mf.Metric {
            metric := Metric{
                Labels: make(labels.Labels, 0),
            }

            // Add metric labels
            for _, l := range m.Label {
                metric.Labels = append(metric.Labels, labels.Label{
                    Name:  *l.Name,
                    Value: *l.Value,
                })
            }

            // Extract value based on type
            switch mf.GetType() {
            case dto.MetricType_COUNTER:
                metric.Value = *m.Counter.Value
            case dto.MetricType_GAUGE:
                metric.Value = *m.Gauge.Value
            // ... handle other types
            }

            family.Metrics = append(family.Metrics, metric)
        }

        families = append(families, family)
    }

    return families, nil
}
```

### **Phase 4: Integration (2-3 days)**

```go
// cmd/promsketch-dropin/main.go

// Add scrape manager initialization
if cfg.Scrape.Enabled {
    log.Printf("Initializing scrape manager...")

    scrapeManager, err := scrape.NewManager(cfg)
    if err != nil {
        log.Fatalf("Failed to create scrape manager: %v", err)
    }

    // Connect scrape manager to ingestion pipeline
    scrapeManager.SetIngestionPipeline(pipe)

    if err := scrapeManager.Start(); err != nil {
        log.Fatalf("Failed to start scrape manager: %v", err)
    }

    log.Printf("Scrape manager started with %d jobs", len(cfg.Scrape.ScrapeConfigs))
}
```

---

## Configuration Example

```yaml
# Full configuration with scraping
server:
  listen_address: ":9100"

backend:
  type: victoriametrics
  url: "http://victoria-metrics:8428"

# NEW: Scraping configuration
scrape:
  enabled: true  # Enable built-in scraping

  global:
    scrape_interval: 15s
    scrape_timeout: 10s

  scrape_configs:
    # Static targets
    - job_name: 'node-exporter'
      static_configs:
        - targets:
            - 'node1:9100'
            - 'node2:9100'
            - 'node3:9100'

    # DNS service discovery
    - job_name: 'api-servers'
      dns_sd_configs:
        - names:
            - 'api.internal.example.com'
          type: 'A'
          port: 9090

    # Kubernetes pods
    - job_name: 'kubernetes-pods'
      kubernetes_sd_configs:
        - role: pod
          namespaces:
            names:
              - default
              - production

      relabel_configs:
        - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
          action: keep
          regex: true

        - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
          action: replace
          target_label: __metrics_path__
          regex: (.+)

# Sketch configuration (same as before)
sketches:
  num_partitions: 16
  targets:
    - match: "http_.*"
    - match: "node_.*"
```

---

## Testing Strategy

### Unit Tests

```go
func TestScrapeManager_StartStop(t *testing.T) {
    cfg := &config.Config{
        ScrapeConfigs: []*config.ScrapeConfig{
            {
                JobName: "test",
                ServiceDiscoveryConfigs: []discovery.Config{
                    &discovery.StaticConfig{
                        Targets: []string{"localhost:9100"},
                    },
                },
            },
        },
    }

    manager, err := scrape.NewManager(cfg)
    assert.NoError(t, err)

    err = manager.Start()
    assert.NoError(t, err)

    time.Sleep(100 * time.Millisecond)

    err = manager.Stop()
    assert.NoError(t, err)
}
```

### Integration Tests

```go
func TestScrapeManager_Integration(t *testing.T) {
    // Start test HTTP server exposing metrics
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "# HELP test_metric A test metric")
        fmt.Fprintln(w, "# TYPE test_metric gauge")
        fmt.Fprintln(w, "test_metric 42")
    }))
    defer ts.Close()

    // Configure scrape manager
    cfg := &config.Config{
        ScrapeConfigs: []*config.ScrapeConfig{
            {
                JobName:        "test",
                ScrapeInterval: model.Duration(1 * time.Second),
                ServiceDiscoveryConfigs: []discovery.Config{
                    &discovery.StaticConfig{
                        Targets: []string{ts.URL},
                    },
                },
            },
        },
    }

    manager, err := scrape.NewManager(cfg)
    assert.NoError(t, err)

    err = manager.Start()
    assert.NoError(t, err)
    defer manager.Stop()

    // Wait for scrape
    time.Sleep(2 * time.Second)

    // Verify metrics were scraped
    metrics := manager.Metrics()
    assert.Greater(t, metrics.SamplesScraped, int64(0))
}
```

---

## Alternative: Minimal Scraping

If full scrape manager is too complex, implement **minimal scraping**:

### Simple Static Scraper

```go
// internal/scrape/simple.go
type SimpleScraper struct {
    targets  []string
    interval time.Duration
    pipeline *pipeline.Pipeline
}

func (s *SimpleScraper) Start() {
    ticker := time.NewTicker(s.interval)

    go func() {
        for range ticker.C {
            for _, target := range s.targets {
                s.scrapeTarget(target)
            }
        }
    }()
}

func (s *SimpleScraper) scrapeTarget(target string) {
    resp, err := http.Get("http://" + target + "/metrics")
    if err != nil {
        return
    }
    defer resp.Body.Close()

    // Parse and send to pipeline
    // ...
}
```

**Effort**: 2-3 days instead of 3-4 weeks

**Features**:
- Static target list only
- No service discovery
- Basic metric parsing
- Simple error handling

---

## Recommendation

**Do NOT implement built-in scraping** unless:
- You cannot use Prometheus for some reason
- You need a completely self-contained solution
- You're willing to invest 3-4 weeks of development

**Instead**:
- Use Prometheus + remote write (current approach)
- Leverage Prometheus's mature scraping capabilities
- Focus on sketch-specific features

If scraping is absolutely needed:
1. Start with simple static scraper (2-3 days)
2. Add DNS discovery if needed (1-2 days)
3. Only add Kubernetes discovery if absolutely required (1 week)

The Prometheus + remote write architecture is **production-proven** and **recommended** for most deployments.
