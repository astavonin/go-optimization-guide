#!/bin/bash
# System checks for stable benchmark execution

set -e

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Thresholds
MAX_LOAD=2.0
MAX_TEMP=75
WARN_TEMP=65

check_cpu_governor() {
    echo "→ Checking CPU frequency governor..."

    if [ -d /sys/devices/system/cpu/cpu0/cpufreq ]; then
        GOVERNOR=$(cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor 2>/dev/null || echo "unknown")

        if [ "$GOVERNOR" = "performance" ]; then
            echo -e "  ${GREEN}✓${NC} CPU governor: $GOVERNOR (optimal)"
        else
            echo -e "  ${YELLOW}⚠${NC} CPU governor: $GOVERNOR (recommended: performance)"
            echo "    To set: sudo cpupower frequency-set -g performance"
        fi
    else
        echo "  ℹ CPU governor info not available"
    fi
}

check_system_load() {
    echo "→ Checking system load..."

    LOAD=$(uptime | awk -F'load average:' '{print $2}' | awk '{print $1}' | sed 's/,//')

    # Compare load (bash doesn't do floating point, use awk)
    IS_HIGH=$(awk -v load="$LOAD" -v max="$MAX_LOAD" 'BEGIN {print (load > max)}')

    if [ "$IS_HIGH" = "1" ]; then
        echo -e "  ${RED}✗${NC} System load: $LOAD (high, max recommended: $MAX_LOAD)"
        echo "    High load may affect benchmark stability"
        return 1
    else
        echo -e "  ${GREEN}✓${NC} System load: $LOAD (acceptable)"
    fi
}

check_cpu_temperature() {
    echo "→ Checking CPU temperature..."

    # Try different methods to get CPU temperature
    TEMP=""

    # Method 1: sensors command
    if command -v sensors &> /dev/null; then
        TEMP=$(sensors 2>/dev/null | grep -i 'Package id 0:' | awk '{print $4}' | sed 's/+//;s/°C//' || echo "")
    fi

    # Method 2: thermal zone
    if [ -z "$TEMP" ] && [ -f /sys/class/thermal/thermal_zone0/temp ]; then
        TEMP_MILLIS=$(cat /sys/class/thermal/thermal_zone0/temp)
        TEMP=$(awk "BEGIN {printf \"%.0f\", $TEMP_MILLIS/1000}")
    fi

    if [ -n "$TEMP" ]; then
        if [ "${TEMP%.*}" -gt "$MAX_TEMP" ]; then
            echo -e "  ${RED}✗${NC} CPU temperature: ${TEMP}°C (too hot, max: ${MAX_TEMP}°C)"
            echo "    Wait for CPU to cool down"
            return 1
        elif [ "${TEMP%.*}" -gt "$WARN_TEMP" ]; then
            echo -e "  ${YELLOW}⚠${NC} CPU temperature: ${TEMP}°C (warm, recommended: <${WARN_TEMP}°C)"
        else
            echo -e "  ${GREEN}✓${NC} CPU temperature: ${TEMP}°C (good)"
        fi
    else
        echo "  ℹ CPU temperature not available"
    fi
}

check_available_memory() {
    echo "→ Checking available memory..."

    # Get available memory in MB
    AVAILABLE_MB=$(free -m | awk '/^Mem:/ {print $7}')

    if [ "$AVAILABLE_MB" -lt 1000 ]; then
        echo -e "  ${YELLOW}⚠${NC} Available memory: ${AVAILABLE_MB}MB (low)"
        echo "    Low memory may affect benchmark stability"
    else
        echo -e "  ${GREEN}✓${NC} Available memory: ${AVAILABLE_MB}MB"
    fi
}

check_swap_usage() {
    echo "→ Checking swap usage..."

    SWAP_USED=$(free -m | awk '/^Swap:/ {print $3}')

    if [ "$SWAP_USED" -gt 100 ]; then
        echo -e "  ${YELLOW}⚠${NC} Swap in use: ${SWAP_USED}MB"
        echo "    Swap usage may affect benchmark performance"
    else
        echo -e "  ${GREEN}✓${NC} Swap usage: ${SWAP_USED}MB (minimal)"
    fi
}

# Main execution
echo "=== System Check for Stable Benchmarking ==="
echo

CHECKS_FAILED=0

check_cpu_governor || true
check_system_load || CHECKS_FAILED=1
check_cpu_temperature || CHECKS_FAILED=1
check_available_memory || true
check_swap_usage || true

echo
if [ $CHECKS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ System is ready for stable benchmarking${NC}"
    exit 0
else
    echo -e "${YELLOW}⚠ System checks failed - benchmarks may have high variance${NC}"
    exit 1
fi
