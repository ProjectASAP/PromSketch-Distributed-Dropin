#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_DIR/docker-compose.cluster.yml"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()  { echo -e "${BLUE}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
fail()  { echo -e "${RED}[FAIL]${NC}  $*"; }

# ── Helpers ──────────────────────────────────────────────────────────────────

wait_for_health() {
  local name="$1" url="$2" max_wait="${3:-60}"
  local elapsed=0
  while [ $elapsed -lt $max_wait ]; do
    if curl -sf --max-time 2 "$url" >/dev/null 2>&1; then
      ok "$name is healthy ($url)"
      return 0
    fi
    sleep 2
    elapsed=$((elapsed + 2))
  done
  fail "$name not healthy after ${max_wait}s ($url)"
  return 1
}

# ── Main ─────────────────────────────────────────────────────────────────────

echo ""
echo "======================================================"
echo "  PromSketch-Dropin vmalert E2E Demo (Cluster Mode)"
echo "======================================================"
echo ""
info "Architecture:"
info "  Prometheus -> pskinsert -> psksketch-{1,2,3} -> VictoriaMetrics"
info "  vmalert -> pskquery -> [sketch or VM] -> VictoriaMetrics"
info "  Grafana <- pskquery / VictoriaMetrics / Prometheus"
echo ""
info "Recording rules:"
info "  sketch-eligible: quantile_over_time, avg_over_time  (answered by PromSketch)"
info "  backend-only:    rate, increase                     (answered by VictoriaMetrics)"
echo ""

# Step 1: Build and start
info "Step 1: Building and starting cluster..."
cd "$PROJECT_DIR"
sudo docker-compose -f "$COMPOSE_FILE" up -d --build 2>&1 | tail -5
echo ""

# Step 2: Wait for services
info "Step 2: Waiting for services to be healthy..."
wait_for_health "VictoriaMetrics" "http://localhost:8428/health" 90
wait_for_health "psksketch-1"    "http://localhost:8491/health" 90
wait_for_health "psksketch-2"    "http://localhost:8493/health" 90
wait_for_health "psksketch-3"    "http://localhost:8495/health" 90
wait_for_health "pskinsert"      "http://localhost:8480/health" 90
wait_for_health "pskquery"       "http://localhost:9100/health" 90
wait_for_health "Grafana"        "http://localhost:3000/api/health" 90
wait_for_health "Prometheus"     "http://localhost:9090/-/healthy" 90
wait_for_health "vmalert"        "http://localhost:8880/health" 90
echo ""

# Step 3: Wait for data to flow + recording rules to evaluate
info "Step 3: Waiting for data + recording rule evaluations (~120s)..."
for i in $(seq 1 8); do
  sleep 15
  # Check if VictoriaMetrics has recording rule data
  SKETCH_RULE=$(curl -sf 'http://localhost:8428/api/v1/query' \
    --data-urlencode 'query=sketch:node_cpu_p95:5m' 2>/dev/null \
    | python3 -c "import sys,json; r=json.load(sys.stdin)['data']['result']; print(len(r))" 2>/dev/null || echo "0")
  BACKEND_RULE=$(curl -sf 'http://localhost:8428/api/v1/query' \
    --data-urlencode 'query=backend:node_cpu_rate:5m' 2>/dev/null \
    | python3 -c "import sys,json; r=json.load(sys.stdin)['data']['result']; print(len(r))" 2>/dev/null || echo "0")
  info "  ${i}/8 ... sketch rules: $SKETCH_RULE result(s), backend rules: $BACKEND_RULE result(s)"
  if [ "$SKETCH_RULE" -gt "0" ] && [ "$BACKEND_RULE" -gt "0" ]; then
    ok "Recording rules are producing data!"
    break
  fi
done
echo ""

# Step 4: Validate recording rule results
info "Step 4: Validating recording rule results..."
echo ""

# Sketch-eligible rule result
SKETCH_P95=$(curl -sf 'http://localhost:8428/api/v1/query' \
  --data-urlencode 'query=sketch:node_cpu_p95:5m' 2>/dev/null \
  | python3 -c "import sys,json; r=json.load(sys.stdin)['data']['result']; print(r[0]['value'][1] if r else 'N/A')" 2>/dev/null || echo "N/A")

# Direct VM result for comparison
VM_P95=$(curl -sf 'http://localhost:8428/api/v1/query' \
  --data-urlencode 'query=quantile_over_time(0.95, node_cpu_seconds_total{mode="idle", cpu="0"}[5m])' 2>/dev/null \
  | python3 -c "import sys,json; r=json.load(sys.stdin)['data']['result']; print(r[0]['value'][1] if r else 'N/A')" 2>/dev/null || echo "N/A")

# Backend-only rule result
BACKEND_RATE=$(curl -sf 'http://localhost:8428/api/v1/query' \
  --data-urlencode 'query=backend:node_cpu_rate:5m' 2>/dev/null \
  | python3 -c "import sys,json; r=json.load(sys.stdin)['data']['result']; print(r[0]['value'][1] if r else 'N/A')" 2>/dev/null || echo "N/A")

echo "  ┌──────────────────────────────────────────────────────┐"
echo "  │  Recording Rule Results (from vmalert → pskquery)    │"
echo "  ├──────────────────────┬───────────────────────────────┤"
printf "  │  sketch:cpu_p95:5m   │  %-28s│\n" "$SKETCH_P95"
printf "  │  VM direct p95       │  %-28s│\n" "$VM_P95"
printf "  │  backend:cpu_rate:5m │  %-28s│\n" "$BACKEND_RATE"
echo "  └──────────────────────┴───────────────────────────────┘"
echo ""

if [ "$VM_P95" != "N/A" ] && [ "$SKETCH_P95" != "N/A" ]; then
  MATCH=$(python3 -c "
vm=float('$VM_P95'); ps=float('$SKETCH_P95')
err = abs(vm-ps)/abs(vm)*100 if vm != 0 else 0
print(f'Relative error: {err:.2f}%')
print('EXACT MATCH' if vm==ps else f'Within {err:.2f}% error')
" 2>/dev/null || echo "Could not compare")
  info "  $MATCH"
fi

echo ""

# Step 5: Check pskquery metrics (sketch hits vs backend queries)
info "Step 5: Checking pskquery routing metrics..."
PSKQUERY_METRICS=$(curl -sf 'http://localhost:9100/metrics' 2>/dev/null || echo "")
if [ -n "$PSKQUERY_METRICS" ]; then
  SKETCH_HITS=$(echo "$PSKQUERY_METRICS" | grep '^pskquery_sketch_hits_total' | awk '{print $2}' || echo "N/A")
  SKETCH_MISSES=$(echo "$PSKQUERY_METRICS" | grep '^pskquery_sketch_misses_total' | awk '{print $2}' || echo "N/A")
  BACKEND_QUERIES=$(echo "$PSKQUERY_METRICS" | grep '^pskquery_queries_total{source="backend"}' | awk '{print $2}' || echo "N/A")
  SKETCH_QUERIES=$(echo "$PSKQUERY_METRICS" | grep '^pskquery_queries_total{source="sketch"}' | awk '{print $2}' || echo "N/A")

  echo "  ┌──────────────────────────────────────────────────────┐"
  echo "  │  pskquery Routing Statistics                         │"
  echo "  ├──────────────────────┬───────────────────────────────┤"
  printf "  │  Sketch queries      │  %-28s│\n" "$SKETCH_QUERIES"
  printf "  │  Sketch hits         │  %-28s│\n" "$SKETCH_HITS"
  printf "  │  Sketch misses       │  %-28s│\n" "$SKETCH_MISSES"
  printf "  │  Backend queries     │  %-28s│\n" "$BACKEND_QUERIES"
  echo "  └──────────────────────┴───────────────────────────────┘"
  echo ""
  if [ "$SKETCH_HITS" != "N/A" ] && [ "$SKETCH_HITS" -gt "0" ] 2>/dev/null; then
    ok "PromSketch sketches are answering queries!"
  fi
  if [ "$BACKEND_QUERIES" != "N/A" ] && [ "$BACKEND_QUERIES" -gt "0" ] 2>/dev/null; then
    ok "Backend (VictoriaMetrics) is answering non-sketch queries!"
  fi
fi

echo ""

# Step 6: Summary
echo "======================================================"
echo "  vmalert E2E Demo Complete"
echo "======================================================"
echo ""
info "Services:"
info "  Grafana:          http://localhost:3000  (admin/admin)"
info "  VictoriaMetrics:  http://localhost:8428"
info "  Prometheus:       http://localhost:9090"
info "  pskinsert:        http://localhost:8480"
info "  pskquery:         http://localhost:9100"
info "  vmalert:          http://localhost:8880"
echo ""
info "Dashboards:"
info "  E2E Quantile:  http://localhost:3000/d/promsketch-e2e"
info "  vmalert Demo:  http://localhost:3000/d/promsketch-vmalert"
echo ""
info "To stop:  sudo docker compose -f $COMPOSE_FILE down"
info "To clean: sudo docker compose -f $COMPOSE_FILE down -v"
echo ""
