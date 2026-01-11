#!/usr/bin/env bash
set -euo pipefail

# ==========================
# Fixed Target
# ==========================
DNS_SERVER="127.0.0.1"
DNS_PORT="1053"

# ==========================
# Tunables
# ==========================

NPROC=$(nproc)
echo $NPROC

DURATION="${DURATION:-30}"          # seconds per test
CLIENTS="${CLIENTS:-$NPROC}"          # dnsperf threads
QPS="${QPS:-100000}"                # target QPS
OUTPUT_DIR="${OUTPUT_DIR:-./dnsperf-results}"
DOMAIN_FILE="${DOMAIN_FILE:-./queries.txt}"
TCP="${TCP:-false}"

# ==========================
# Helpers
# ==========================
log() {
  echo "[$(date '+%H:%M:%S')] $*"
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    log "ERROR: missing command: $1"
    exit 1
  }
}

# ==========================
# Install dnsperf if missing
# ==========================
if ! command -v dnsperf >/dev/null 2>&1; then
  log "dnsperf not found, installing..."
  sudo apt-get update
  sudo apt-get install -y dnsperf
fi

require_cmd dnsperf
require_cmd grep
require_cmd sed

mkdir -p "$OUTPUT_DIR"

QUERY_COUNT=$(wc -l < "$DOMAIN_FILE")

log "DNS target: $DNS_SERVER:$DNS_PORT"
log "Query count: $QUERY_COUNT"
log "Target QPS: $QPS"
log "Duration: $DURATION s"
log "Clients: $CLIENTS"
log "TCP mode: $TCP"

# ==========================
# Run dnsperf
# ==========================
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
RESULT_FILE="$OUTPUT_DIR/dnsperf-$TIMESTAMP.txt"

DNSPERF_OPTS=(
  -s "$DNS_SERVER"
  -p "$DNS_PORT"
  -d "$DOMAIN_FILE"
  -l "$DURATION"
  -Q "$QPS"
  -c "$CLIENTS"
)

if [ "$TCP" = "true" ]; then
  DNSPERF_OPTS+=(-T)
fi

log "Starting dnsperf..."
dnsperf "${DNSPERF_OPTS[@]}"
