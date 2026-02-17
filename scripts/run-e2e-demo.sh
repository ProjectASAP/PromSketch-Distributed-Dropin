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
echo "=============================================="
echo "  PromSketch-Dropin E2E Demo (Cluster Mode)"
echo "=============================================="
echo ""
info "Architecture:"
info "  Prometheus -> pskinsert -> psksketch-{1,2,3} -> VictoriaMetrics"
info "  Grafana <- pskquery <- psksketch-{1,2,3}"
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
echo ""

# Step 3: Wait for data to flow
info "Step 3: Waiting for Prometheus to scrape and forward data (90s)..."
for i in $(seq 1 6); do
  sleep 15
  # Check if VictoriaMetrics has any data
  COUNT=$(curl -sf 'http://localhost:8428/api/v1/series' \
    --data-urlencode 'match[]={__name__=~".+"}' 2>/dev/null \
    | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('data',[])))" 2>/dev/null || echo "0")
  info "  ${i}/6 ... $COUNT series in VictoriaMetrics"
  if [ "$COUNT" -gt "5" ]; then
    ok "Data is flowing! ($COUNT series found)"
    break
  fi
done
echo ""

# Step 4: Validate queries
info "Step 4: Running validation queries..."
echo ""

# Query VictoriaMetrics directly
info "  Querying VictoriaMetrics (direct)..."
VM_RESULT=$(curl -sf 'http://localhost:8428/api/v1/query' \
  --data-urlencode 'query=node_cpu_seconds_total{mode="idle", cpu="0"}' 2>/dev/null || echo '{"data":{"result":[]}}')
VM_COUNT=$(echo "$VM_RESULT" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['data']['result']))" 2>/dev/null || echo "0")
if [ "$VM_COUNT" -gt "0" ]; then
  ok "VictoriaMetrics has $VM_COUNT result(s) for node_cpu_seconds_total{mode=\"idle\"}"
else
  warn "VictoriaMetrics has no results yet — data may still be propagating"
fi

# Query through pskquery
info "  Querying pskquery (PromSketch)..."
PS_RESULT=$(curl -sf 'http://localhost:9100/api/v1/query' \
  --data-urlencode 'query=node_cpu_seconds_total{mode="idle", cpu="0"}' 2>/dev/null || echo '{"data":{"result":[]}}')
PS_COUNT=$(echo "$PS_RESULT" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['data']['result']))" 2>/dev/null || echo "0")
if [ "$PS_COUNT" -gt "0" ]; then
  ok "pskquery has $PS_COUNT result(s) for node_cpu_seconds_total{mode=\"idle\"}"
else
  warn "pskquery has no results yet"
fi

echo ""

# Step 5: Quantile comparison
info "Step 5: Quantile comparison (quantile_over_time p95)..."
QUERY='quantile_over_time(0.95, node_cpu_seconds_total{mode="idle", cpu="0"}[5m])'

VM_VAL=$(curl -sf 'http://localhost:8428/api/v1/query' \
  --data-urlencode "query=$QUERY" 2>/dev/null \
  | python3 -c "import sys,json; r=json.load(sys.stdin)['data']['result']; print(r[0]['value'][1] if r else 'N/A')" 2>/dev/null || echo "N/A")

PS_VAL=$(curl -sf 'http://localhost:9100/api/v1/query' \
  --data-urlencode "query=$QUERY" 2>/dev/null \
  | python3 -c "import sys,json; r=json.load(sys.stdin)['data']['result']; print(r[0]['value'][1] if r else 'N/A')" 2>/dev/null || echo "N/A")

echo ""
echo "  ┌────────────────────────────────────────────────┐"
echo "  │  quantile_over_time(0.95, node_cpu_*[5m])      │"
echo "  ├────────────────────┬───────────────────────────┤"
printf "  │  VictoriaMetrics   │  %-24s│\n" "$VM_VAL"
printf "  │  PromSketch        │  %-24s│\n" "$PS_VAL"
echo "  └────────────────────┴───────────────────────────┘"
echo ""

if [ "$VM_VAL" != "N/A" ] && [ "$PS_VAL" != "N/A" ]; then
  MATCH=$(python3 -c "
vm=float('$VM_VAL'); ps=float('$PS_VAL')
err = abs(vm-ps)/abs(vm)*100 if vm != 0 else 0
print(f'Relative error: {err:.2f}%')
print('EXACT MATCH' if vm==ps else f'Within {err:.2f}% error')
" 2>/dev/null || echo "Could not compare")
  info "  $MATCH"
fi

echo ""

# Step 6: Summary
echo "=============================================="
echo "  E2E Demo Complete"
echo "=============================================="
echo ""
info "Services:"
info "  Grafana:          http://localhost:3000  (admin/admin)"
info "  VictoriaMetrics:  http://localhost:8428"
info "  Prometheus:       http://localhost:9090"
info "  pskinsert:        http://localhost:8480"
info "  pskquery:         http://localhost:9100"
echo ""
info "Dashboard: http://localhost:3000/d/promsketch-e2e"
echo ""
info "To stop:  sudo docker compose -f $COMPOSE_FILE down"
info "To clean: sudo docker compose -f $COMPOSE_FILE down -v"
echo ""
