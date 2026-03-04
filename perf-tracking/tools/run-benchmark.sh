#!/usr/bin/env bash
# Local operator script: launch an EC2 instance, wait for benchmark
# collection to finish, download results, then terminate the instance.
#
# Usage:
#   ./run-benchmark.sh <arch> <version> [<version> ...]
#   ./run-benchmark.sh amd64 1.24.0 1.25.0
#   ./run-benchmark.sh arm64 1.23.0 1.24.0 1.25.0
#
# Prerequisites (one-time setup):
#   ./setup-aws.sh

set -euo pipefail

# --- ANSI colours ---
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=benchmark-config.sh
source "$SCRIPT_DIR/benchmark-config.sh"
BOOTSTRAP_SCRIPT="$SCRIPT_DIR/bootstrap.sh"

TIMESTAMP=$(date +%Y%m%d-%H%M%S)

# State used by cleanup trap
INSTANCE_ID=""
SG_ID=""
OPERATOR_IP=""
SSH_AUTHORIZED=false

# ---------------------------------------------------------------------------
# Usage / argument validation
# ---------------------------------------------------------------------------
show_usage() {
    echo "Usage: $0 <arch> <version> [<version> ...]"
    echo
    echo "  arch      Target architecture: amd64 or arm64"
    echo "  version   One or more Go versions, e.g. 1.24.0 1.25.0"
    echo
    echo "Examples:"
    echo "  $0 amd64 1.24.0 1.25.0"
    echo "  $0 arm64 1.23.0 1.24.0 1.25.0"
    echo
    echo "One-time AWS setup (run once before first use):"
    echo "  ./setup-aws.sh"
}

if [ $# -lt 2 ]; then
    show_usage
    exit 1
fi

ARCH="$1"
shift
VERSIONS=("$@")

for v in "${VERSIONS[@]}"; do
    if [[ ! "$v" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?$ ]]; then
        echo -e "${RED}✗${NC} Invalid Go version format: '$v' (expected X.Y.Z or X.Y)"
        exit 1
    fi
done

VERSIONS_STR="${VERSIONS[*]}"

case "$ARCH" in
    amd64)
        AMI_PARAM="/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64"
        INSTANCE_TYPE="c6i.xlarge"
        ;;
    arm64)
        AMI_PARAM="/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64"
        INSTANCE_TYPE="c7g.xlarge"
        ;;
    *)
        echo -e "${RED}✗${NC} Unknown architecture '$ARCH'. Must be amd64 or arm64."
        exit 1
        ;;
esac

echo "=== Benchmark runner — arch=$ARCH versions=$VERSIONS_STR ==="
echo

# ---------------------------------------------------------------------------
# Prerequisites
# ---------------------------------------------------------------------------
for tool in aws ssh scp curl; do
    if ! command -v "$tool" &>/dev/null; then
        echo -e "${RED}✗${NC} Required tool not found: $tool"
        exit 1
    fi
done

if [ ! -f "$KEY_PATH" ]; then
    echo -e "${RED}✗${NC} SSH key not found: $KEY_PATH"
    echo "  Run ./setup-aws.sh first."
    exit 1
fi

if [ ! -f "$BOOTSTRAP_SCRIPT" ]; then
    echo -e "${RED}✗${NC} bootstrap.sh not found at $BOOTSTRAP_SCRIPT"
    exit 1
fi

# ---------------------------------------------------------------------------
# Resolve AL2023 AMI and look up security group
# ---------------------------------------------------------------------------
echo -e "${CYAN}→${NC} Resolving AMI for $ARCH..."
AMI_ID=$(aws ssm get-parameter \
    --name "$AMI_PARAM" \
    --query 'Parameter.Value' \
    --output text)
echo -e "  ${GREEN}✓${NC} AMI: $AMI_ID"

echo -e "${CYAN}→${NC} Looking up security group $SG_NAME..."
SG_ID=$(aws ec2 describe-security-groups \
    --filters "Name=group-name,Values=$SG_NAME" "Name=tag:ManagedBy,Values=benchmark-automation" \
    --query 'SecurityGroups[0].GroupId' \
    --output text)
if [ -z "$SG_ID" ] || [ "$SG_ID" = "None" ]; then
    echo -e "${RED}✗${NC} Security group $SG_NAME not found. Run ./setup-aws.sh first."
    exit 1
fi
echo -e "  ${GREEN}✓${NC} Security group: $SG_ID"

# ---------------------------------------------------------------------------
# Operator IP (for SSH ingress rule)
# ---------------------------------------------------------------------------
echo -e "${CYAN}→${NC} Detecting operator IP..."
OPERATOR_IP="${OPERATOR_IP:-$(curl -sf --max-time 10 https://checkip.amazonaws.com || true)}"
if [ -z "$OPERATOR_IP" ]; then
    echo -e "${RED}✗${NC} Could not detect operator public IP."
    echo "  Check connectivity to https://checkip.amazonaws.com or export OPERATOR_IP=<your-ip>."
    exit 1
fi
echo -e "  ${GREEN}✓${NC} Operator IP: $OPERATOR_IP"

# ---------------------------------------------------------------------------
# Cleanup trap: terminate instance and revoke SSH rule on any exit
# ---------------------------------------------------------------------------
cleanup_instance() {
    local exit_code=$?
    echo
    echo -e "${CYAN}→${NC} Running cleanup..."

    if [ -n "${INSTANCE_ID}" ]; then
        echo -e "  ${CYAN}→${NC} Terminating instance $INSTANCE_ID..."
        # Skip wait instance-terminated in the trap — it can hang for minutes
        # and blocks the SSH revoke that must happen promptly.
        if aws ec2 terminate-instances --instance-ids "$INSTANCE_ID" \
                --output text > /dev/null 2>&1; then
            echo -e "  ${GREEN}✓${NC} Instance terminated"
        else
            echo -e "  ${YELLOW}⚠${NC} Instance terminate call failed — may need manual cleanup"
        fi
        INSTANCE_ID=""
    fi

    if [ "$SSH_AUTHORIZED" = "true" ] && [ -n "$OPERATOR_IP" ]; then
        echo -e "  ${CYAN}→${NC} Revoking SSH access for $OPERATOR_IP..."
        if aws ec2 revoke-security-group-ingress \
                --group-id "$SG_ID" --protocol tcp --port 22 \
                --cidr "${OPERATOR_IP}/32" > /dev/null 2>&1; then
            echo -e "  ${GREEN}✓${NC} SSH access revoked"
        else
            echo -e "  ${YELLOW}⚠${NC} SSH revoke failed — rule may need manual removal"
        fi
        SSH_AUTHORIZED=false
    fi

    rm -f "${KNOWN_HOSTS_FILE:-}" 2>/dev/null || true
    rm -f "${SSH_ERR_FILE:-}" 2>/dev/null || true

    exit "$exit_code"
}

trap cleanup_instance EXIT INT TERM

# ---------------------------------------------------------------------------
# Step 5: Authorize SSH ingress from operator IP
# ---------------------------------------------------------------------------
echo -e "${CYAN}→${NC} Authorizing SSH from $OPERATOR_IP..."
if SG_ERR=$(aws ec2 authorize-security-group-ingress \
        --group-id "$SG_ID" \
        --protocol tcp \
        --port 22 \
        --cidr "${OPERATOR_IP}/32" 2>&1); then
    SSH_AUTHORIZED=true
    echo -e "  ${GREEN}✓${NC} SSH rule added"
elif [[ "$SG_ERR" == *"InvalidPermission.Duplicate"* ]]; then
    # Rule already exists (stale from a previous run that did not clean up).
    # Do NOT set SSH_AUTHORIZED=true — we did not add it, so we must not revoke it.
    echo -e "  ${YELLOW}⚠${NC} SSH rule already exists (stale from previous run — will not revoke on exit)"
else
    echo -e "${RED}✗${NC} Failed to authorize SSH ingress: $SG_ERR"
    exit 1
fi

# ---------------------------------------------------------------------------
# Step 6: Build user-data payload (GO_VERSIONS line + bootstrap.sh body)
# Inject the GO_VERSIONS export as the second line (after the shebang) using
# head/printf/tail — avoids sed quoting issues with embedded spaces/quotes.
# ---------------------------------------------------------------------------
echo -e "${CYAN}→${NC} Building user-data payload..."
USER_DATA=$(
    head -1 "$BOOTSTRAP_SCRIPT"
    printf 'export GO_VERSIONS="%s"\n' "$VERSIONS_STR"
    tail -n +2 "$BOOTSTRAP_SCRIPT"
)

# ---------------------------------------------------------------------------
# Step 6 (cont.): Launch EC2 instance
# ---------------------------------------------------------------------------
echo -e "${CYAN}→${NC} Launching EC2 instance ($INSTANCE_TYPE, $AMI_ID)..."
INSTANCE_ID=$(aws ec2 run-instances \
    --image-id "$AMI_ID" \
    --instance-type "$INSTANCE_TYPE" \
    --key-name "$KEY_NAME" \
    --security-group-ids "$SG_ID" \
    --iam-instance-profile "Name=$PROFILE_NAME" \
    --user-data "$USER_DATA" \
    --metadata-options "HttpTokens=required,HttpEndpoint=enabled" \
    --tag-specifications \
        "ResourceType=instance,Tags=[{Key=Name,Value=benchmark-runner-${ARCH}-${TIMESTAMP}},{Key=ManagedBy,Value=benchmark-automation}]" \
    --count 1 \
    --query 'Instances[0].InstanceId' \
    --output text)
echo -e "  ${GREEN}✓${NC} Instance launched: $INSTANCE_ID"

# ---------------------------------------------------------------------------
# Step 7: Wait for instance to reach running state
# ---------------------------------------------------------------------------
echo -e "${CYAN}→${NC} Waiting for instance to reach running state..."
aws ec2 wait instance-running --instance-ids "$INSTANCE_ID"

PUBLIC_IP=$(aws ec2 describe-instances \
    --instance-ids "$INSTANCE_ID" \
    --query 'Reservations[0].Instances[0].PublicIpAddress' \
    --output text)
if [ -z "$PUBLIC_IP" ] || [ "$PUBLIC_IP" = "None" ]; then
    echo -e "${RED}✗${NC} Instance $INSTANCE_ID has no public IP."
    echo "  Ensure the default VPC subnet has auto-assign public IP enabled."
    exit 1
fi
echo -e "  ${GREEN}✓${NC} Instance running — public IP: $PUBLIC_IP"

# ---------------------------------------------------------------------------
# Step 9: Wait for SSH to become available (max 5 min)
# ---------------------------------------------------------------------------
echo -e "${CYAN}→${NC} Waiting for SSH (max 5 min)..."
# Use a per-run known-hosts file with accept-new to avoid TOFU bypass (StrictHostKeyChecking=no)
# while still failing on changed host keys for subsequent connections in the same session.
KNOWN_HOSTS_FILE="/tmp/benchmark-known-hosts-$$"
SSH_OPTS=(
    -o StrictHostKeyChecking=accept-new
    -o "UserKnownHostsFile=$KNOWN_HOSTS_FILE"
    -o BatchMode=yes
    -o ConnectTimeout=5
    -i "$KEY_PATH"
)
SSH_ELAPSED=0
SSH_TIMEOUT=300

while true; do
    if ssh "${SSH_OPTS[@]}" "ec2-user@$PUBLIC_IP" "echo ready" >/dev/null 2>&1; then
        echo -e "  ${GREEN}✓${NC} SSH ready"
        break
    fi

    SSH_ELAPSED=$((SSH_ELAPSED + 10))
    if [ "$SSH_ELAPSED" -ge "$SSH_TIMEOUT" ]; then
        echo -e "${RED}✗${NC} SSH did not become available within ${SSH_TIMEOUT}s"
        exit 1
    fi

    printf "  . (${SSH_ELAPSED}s)\r"
    sleep 10
done

# ---------------------------------------------------------------------------
# Step 10: Poll for /tmp/benchmark-done marker (hard limit: 7 hours)
# ---------------------------------------------------------------------------
echo -e "${CYAN}→${NC} Polling for benchmark completion (checking every 60s, hard limit 7h)..."
POLL_ELAPSED=0
HARD_TIMEOUT=25200  # 7 hours in seconds
POLL_INTERVAL=60

# Poll for /tmp/benchmark-done marker
SSH_ERR_FILE="/tmp/ssh-poll-err-$$"
CONSECUTIVE_SSH_FAILS=0

while true; do
    if ssh "${SSH_OPTS[@]}" "ec2-user@$PUBLIC_IP" \
            "test -f /tmp/benchmark-done" 2>"$SSH_ERR_FILE"; then
        echo
        echo -e "  ${GREEN}✓${NC} Marker file found"
        rm -f "$SSH_ERR_FILE"
        break
    fi

    # If SSH wrote to stderr it is a connectivity issue, not just the marker missing
    if [ -s "$SSH_ERR_FILE" ]; then
        CONSECUTIVE_SSH_FAILS=$((CONSECUTIVE_SSH_FAILS + 1))
        if [ "$CONSECUTIVE_SSH_FAILS" -ge 5 ]; then
            echo -e "  ${YELLOW}⚠${NC} Repeated SSH connectivity issue: $(head -1 "$SSH_ERR_FILE")"
            CONSECUTIVE_SSH_FAILS=0
        fi
    else
        CONSECUTIVE_SSH_FAILS=0
    fi
    rm -f "$SSH_ERR_FILE"

    POLL_ELAPSED=$((POLL_ELAPSED + POLL_INTERVAL))
    if [ "$POLL_ELAPSED" -ge "$HARD_TIMEOUT" ]; then
        echo
        echo -e "${RED}✗${NC} Hard timeout reached (7 hours) — terminating instance"
        exit 1
    fi

    HOURS=$((POLL_ELAPSED / 3600))
    MINS=$(( (POLL_ELAPSED % 3600) / 60 ))
    printf "  → Elapsed: %dh %02dm\r" "$HOURS" "$MINS"
    sleep "$POLL_INTERVAL"
done

# ---------------------------------------------------------------------------
# Step 11: Read marker and decide whether to download
# ---------------------------------------------------------------------------
MARKER=$(ssh "${SSH_OPTS[@]}" "ec2-user@$PUBLIC_IP" "cat /tmp/benchmark-done" | head -1)
echo -e "  Marker: $MARKER"

if [ "$MARKER" != "success" ]; then
    echo -e "${RED}✗${NC} Benchmark failed: $MARKER"
    LOG_FILE="./bootstrap-${ARCH}-${TIMESTAMP}.log"
    echo -e "${CYAN}→${NC} Downloading diagnostic log..."
    scp "${SSH_OPTS[@]}" \
        "ec2-user@${PUBLIC_IP}:/var/log/benchmark-bootstrap.log" \
        "$LOG_FILE" 2>/dev/null && \
        echo -e "  ${GREEN}✓${NC} Log saved: $LOG_FILE" || \
        echo -e "  ${YELLOW}⚠${NC} Could not download bootstrap log"
    exit 1
fi

# ---------------------------------------------------------------------------
# Step 12: Download results
# ---------------------------------------------------------------------------
RESULTS_DIR="$(pwd)/results-${ARCH}-${TIMESTAMP}"
mkdir -p "$RESULTS_DIR"
echo -e "${CYAN}→${NC} Downloading results to $RESULTS_DIR..."

scp -r "${SSH_OPTS[@]}" \
    "ec2-user@${PUBLIC_IP}:/home/ec2-user/repo/perf-tracking/results/stable/" \
    "$RESULTS_DIR/"

echo -e "  ${GREEN}✓${NC} Results downloaded"

# ---------------------------------------------------------------------------
# Steps 13-14: Terminate instance and revoke SSH rule explicitly
# (cleanup trap will also attempt this but is idempotent via the flag guards)
# ---------------------------------------------------------------------------
echo -e "${CYAN}→${NC} Terminating instance $INSTANCE_ID..."
aws ec2 terminate-instances --instance-ids "$INSTANCE_ID" \
    --output text > /dev/null
aws ec2 wait instance-terminated --instance-ids "$INSTANCE_ID"
echo -e "  ${GREEN}✓${NC} Instance terminated"
INSTANCE_ID=""  # prevent double-termination in EXIT trap

if [ "$SSH_AUTHORIZED" = "true" ] && [ -n "$OPERATOR_IP" ]; then
    echo -e "${CYAN}→${NC} Revoking SSH access for $OPERATOR_IP..."
    aws ec2 revoke-security-group-ingress \
        --group-id "$SG_ID" \
        --protocol tcp \
        --port 22 \
        --cidr "${OPERATOR_IP}/32" > /dev/null 2>&1 || true
    echo -e "  ${GREEN}✓${NC} SSH access revoked"
    SSH_AUTHORIZED=false  # prevent double-revocation in EXIT trap
fi

# ---------------------------------------------------------------------------
# Step 15: Print results location
# ---------------------------------------------------------------------------
echo
echo -e "${GREEN}✓ Benchmark run complete${NC}"
echo "  Results: $RESULTS_DIR"
echo
echo "To analyse results:"
echo "  ls $RESULTS_DIR/stable/"
