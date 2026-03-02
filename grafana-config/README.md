# Grafana Setup (No Custom Plugin)

This integration uses only Grafana's built-in Prometheus datasource.

## 1) Add a standard Prometheus datasource

In Grafana, add **Prometheus** datasource and set URL to the PromSketch-Dropin endpoint, for example:

- `http://localhost:9100`

No custom Grafana plugin is required.

## 2) Ensure internal metrics are exposed

PromSketch-Dropin already exposes metrics on:

- `/metrics`

Examples:

- `promsketch_query_duration_seconds`
- `promsketch_query_sketch_hits_total`
- `promsketch_query_sketch_misses_total`

## 3) Scrape PromSketch-Dropin from Prometheus

Add a scrape job to your Prometheus config:

```yaml
scrape_configs:
  - job_name: promsketch
    static_configs:
      - targets: ["localhost:9100"]
```

Adjust host/port for your deployment.

## 4) Import dashboard JSON

Import:

- `grafana/promsketch-dashboard.json`

Dashboard includes:

- Sketch hit ratio
- Fallback rate
- Query latency (p50/p90/p99)
- Query results (sketch vs backend)
- Query throughput and errors
