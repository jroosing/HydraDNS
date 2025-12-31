#!/bin/bash
# Benchmark HydraDNS with resource monitoring
# Usage: ./scripts/benchmark-with-stats.sh [duration_seconds]

DURATION=${1:-30}
CONTAINER="hydradns"
STATS_FILE="/tmp/hydradns_stats.csv"
SUMMARY_FILE="/tmp/hydradns_benchmark_summary.txt"

echo "=== HydraDNS Benchmark with Resource Monitoring ==="
echo "Duration: ${DURATION}s"
echo ""

# Check container is running
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
    echo "ERROR: Container '${CONTAINER}' is not running"
    exit 1
fi

# Check dnsperf is available
if ! command -v dnsperf &> /dev/null; then
    echo "ERROR: dnsperf is not installed"
    echo "Install with: sudo apt install dnsperf"
    exit 1
fi

# Check queries file exists
QUERIES_FILE="$(dirname "$0")/../queries.txt"
if [[ ! -f "$QUERIES_FILE" ]]; then
    QUERIES_FILE="./queries.txt"
fi
if [[ ! -f "$QUERIES_FILE" ]]; then
    echo "ERROR: queries.txt not found"
    exit 1
fi

# Get baseline stats
echo "--- Baseline (idle) ---"
docker stats "$CONTAINER" --no-stream --format "CPU: {{.CPUPerc}}, MEM: {{.MemUsage}}, NET I/O: {{.NetIO}}"
echo ""

# Clear stats file
echo "timestamp,cpu_percent,mem_usage,mem_limit,mem_percent,net_in,net_out" > "$STATS_FILE"

# Start stats collection in background (every 0.5s)
echo "Starting resource monitoring..."
(
    while true; do
        STATS=$(docker stats "$CONTAINER" --no-stream --format "{{.CPUPerc}},{{.MemUsage}},{{.MemPerc}},{{.NetIO}}" 2>/dev/null)
        if [[ -n "$STATS" ]]; then
            # Parse memory usage (e.g., "15.2MiB / 7.5GiB")
            MEM_USAGE=$(echo "$STATS" | cut -d',' -f2 | cut -d'/' -f1 | tr -d ' ')
            MEM_LIMIT=$(echo "$STATS" | cut -d',' -f2 | cut -d'/' -f2 | tr -d ' ')
            # Parse net I/O
            NET_IO=$(echo "$STATS" | cut -d',' -f4)
            NET_IN=$(echo "$NET_IO" | cut -d'/' -f1 | tr -d ' ')
            NET_OUT=$(echo "$NET_IO" | cut -d'/' -f2 | tr -d ' ')
            
            echo "$(date +%s.%N),$(echo "$STATS" | cut -d',' -f1),$MEM_USAGE,$MEM_LIMIT,$(echo "$STATS" | cut -d',' -f3),$NET_IN,$NET_OUT" >> "$STATS_FILE"
        fi
        sleep 0.5
    done
) &
STATS_PID=$!

# Give stats a moment to start
sleep 1

# Run benchmark
echo ""
echo "--- Running dnsperf benchmark (${DURATION}s) ---"
echo ""

DNS_SERVER="127.0.0.1"
DNS_PORT="1053"
NPROC=$(nproc)
CLIENTS="${CLIENTS:-$NPROC}"        # dnsperf threads
QPS="${QPS:-100000}"                # target QPS
OUTPUT_DIR="${OUTPUT_DIR:-./dnsperf-results}"
DOMAIN_FILE="${DOMAIN_FILE:-./queries.txt}"

DNSPERF_OPTS=(
  -s "$DNS_SERVER"
  -p "$DNS_PORT"
  -d "$DOMAIN_FILE"
  -l "$DURATION"
  -Q "$QPS"
  -c "$CLIENTS"
)

log "Starting dnsperf..."
dnsperf "${DNSPERF_OPTS[@]}"

DNSPERF_OUTPUT=$(dnsperf "${DNSPERF_OPTS[@]}" 2>&1)
echo "$DNSPERF_OUTPUT"

# Stop stats collection
kill $STATS_PID 2>/dev/null
wait $STATS_PID 2>/dev/null

echo ""
echo "--- Final stats (post-benchmark) ---"
docker stats "$CONTAINER" --no-stream --format "CPU: {{.CPUPerc}}, MEM: {{.MemUsage}}, NET I/O: {{.NetIO}}"

# Analyze collected stats
echo ""
echo "=== Resource Usage Analysis ==="

# Parse CSV and calculate stats
if [[ -f "$STATS_FILE" ]] && [[ $(wc -l < "$STATS_FILE") -gt 1 ]]; then
    # Skip header, get CPU values
    CPU_VALUES=$(tail -n +2 "$STATS_FILE" | cut -d',' -f2 | tr -d '%' | grep -E '^[0-9.]+$')
    MEM_VALUES=$(tail -n +2 "$STATS_FILE" | cut -d',' -f3)
    
    if [[ -n "$CPU_VALUES" ]]; then
        CPU_MAX=$(echo "$CPU_VALUES" | sort -n | tail -1)
        CPU_AVG=$(echo "$CPU_VALUES" | awk '{sum+=$1; count++} END {if(count>0) printf "%.2f", sum/count; else print "N/A"}')
        CPU_MIN=$(echo "$CPU_VALUES" | sort -n | head -1)
        
        echo ""
        echo "CPU Usage:"
        echo "  Min:  ${CPU_MIN}%"
        echo "  Avg:  ${CPU_AVG}%"
        echo "  Max:  ${CPU_MAX}%"
    fi
    
    # Get max memory (last value tends to be stable)
    MEM_MAX=$(echo "$MEM_VALUES" | tail -1)
    echo ""
    echo "Memory Usage:"
    echo "  Peak: $MEM_MAX"
    
    SAMPLE_COUNT=$(tail -n +2 "$STATS_FILE" | wc -l)
    echo ""
    echo "Samples collected: $SAMPLE_COUNT"
fi

# Extract QPS from dnsperf output
QPS=$(echo "$DNSPERF_OUTPUT" | grep "Queries per second" | awk '{print $4}')
QUERIES_SENT=$(echo "$DNSPERF_OUTPUT" | grep "Queries sent:" | awk '{print $3}')
QUERIES_COMPLETED=$(echo "$DNSPERF_OUTPUT" | grep "Queries completed:" | awk '{print $3}')
QUERIES_LOST=$(echo "$DNSPERF_OUTPUT" | grep "Queries lost:" | awk '{print $3}')

echo ""
echo "=== Summary ==="
echo "Queries Sent:      $QUERIES_SENT"
echo "Queries Completed: $QUERIES_COMPLETED"
echo "Queries Lost:      $QUERIES_LOST"
echo "QPS Achieved:      $QPS"
echo ""
echo "Stats saved to: $STATS_FILE"

# Recommendations
echo ""
echo "=== Recommendations for Home Use ==="
if [[ -n "$CPU_MAX" ]]; then
    CPU_MAX_INT=${CPU_MAX%.*}
    if [[ $CPU_MAX_INT -lt 100 ]]; then
        echo "CPU: 1 core should be sufficient (peak ${CPU_MAX}%)"
    elif [[ $CPU_MAX_INT -lt 200 ]]; then
        echo "CPU: 2 cores recommended (peak ${CPU_MAX}%)"
    else
        echo "CPU: 2+ cores recommended (peak ${CPU_MAX}%)"
    fi
fi

echo "Memory: 64-128 MB should be sufficient for home use"
echo ""
