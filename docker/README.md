# PromSketch-Dropin Docker Demo

This docker-compose setup provides a complete demonstration environment for PromSketch-Dropin.

## Architecture

```
┌─────────────┐     scrape      ┌──────────────┐
│ Prometheus  │────────────────>│ Node Exporter│
└──────┬──────┘                 └──────────────┘
       │ remote_write
       ▼
┌─────────────────────┐
│ PromSketch-Dropin   │
│   :9100             │
└──────┬──────────────┘
       │ forward
       ▼
┌─────────────────────┐          ┌──────────┐
│ VictoriaMetrics     │<─────────│ Grafana  │
│   :8428             │  query   │   :3000  │
└─────────────────────┘          └──────────┘
```

## Services

- **PromSketch-Dropin** (`:9100`) - Main service with sketch storage
- **VictoriaMetrics** (`:8428`) - Backend storage
- **Grafana** (`:3000`) - Visualization (admin/admin)
- **Prometheus** (`:9090`) - Metrics scraper
- **Node Exporter** (`:9101`) - Sample metrics producer

## Quick Start

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f promsketch-dropin

# Stop all services
docker-compose down

# Remove all data
docker-compose down -v
```

## Accessing Services

- **Grafana**: http://localhost:3000 (admin/admin)
- **PromSketch-Dropin**: http://localhost:9100
  - Health: http://localhost:9100/health
  - Metrics: http://localhost:9100/metrics
  - Query: http://localhost:9100/api/v1/query
- **VictoriaMetrics**: http://localhost:8428
- **Prometheus**: http://localhost:9090
- **Node Exporter**: http://localhost:9101/metrics

## Testing Queries in Grafana

1. Open Grafana at http://localhost:3000
2. Login with admin/admin
3. Go to Explore
4. Select "PromSketch-Dropin" datasource
5. Try queries:
   ```promql
   avg_over_time(node_cpu_seconds_total[5m])
   sum_over_time(node_network_receive_bytes_total[5m])
   quantile_over_time(0.95, node_memory_MemAvailable_bytes[5m])
   ```

## Configuration

The configuration file is at `configs/promsketch-dropin.example.yaml`.

Key settings:
- Backend: VictoriaMetrics at http://victoriametrics:8428
- Sketch targets: Configured in the YAML file
- Remote write: Enabled on port 9100

## Troubleshooting

### PromSketch-Dropin won't start
Check that VictoriaMetrics is healthy:
```bash
curl http://localhost:8428/health
```

### No metrics in Grafana
1. Check Prometheus is scraping: http://localhost:9090/targets
2. Check PromSketch-Dropin metrics: http://localhost:9100/metrics
3. Check remote write is working in Prometheus logs

### Backend errors
Check VictoriaMetrics logs:
```bash
docker-compose logs victoriametrics
```
