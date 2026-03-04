#!/usr/bin/env bash
# One-time AWS resource setup for benchmark runner.
# Creates IAM role/profile, security group, and key pair.
#
# Usage:
#   ./setup-aws.sh            # create resources (idempotent)
#   ./setup-aws.sh --destroy  # remove all created resources

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

# ---------------------------------------------------------------------------
# IAM trust policy: allow EC2 to assume the role
# ---------------------------------------------------------------------------
TRUST_POLICY='{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": { "Service": "ec2.amazonaws.com" },
    "Action": "sts:AssumeRole"
  }]
}'

# ---------------------------------------------------------------------------
# Inline policy: instances with this role may terminate any EC2 instance
# tagged ManagedBy=benchmark-automation (not just themselves). This is an
# intentional trade-off for a single-operator tool — aws:SourceInstanceId
# conditions on ec2:TerminateInstances are not supported. For concurrent
# multi-operator use, scope this policy to specific instance ARNs at launch time.
# ---------------------------------------------------------------------------
INLINE_POLICY='{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": "ec2:TerminateInstances",
    "Resource": "*",
    "Condition": {
      "StringEquals": {
        "ec2:ResourceTag/ManagedBy": "benchmark-automation"
      }
    }
  }]
}'

# ---------------------------------------------------------------------------
# Helper: check that aws CLI is present and credentials are configured
# ---------------------------------------------------------------------------
check_prereqs() {
    if ! command -v aws &>/dev/null; then
        echo -e "${RED}✗${NC} aws CLI not found. Install it first."
        exit 1
    fi
    if ! aws sts get-caller-identity &>/dev/null; then
        echo -e "${RED}✗${NC} AWS credentials not configured."
        exit 1
    fi
    echo -e "${GREEN}✓${NC} AWS credentials OK ($(aws sts get-caller-identity --query 'Arn' --output text))"
}

# ---------------------------------------------------------------------------
# Create IAM role + instance profile
# ---------------------------------------------------------------------------
create_iam() {
    echo
    echo -e "${CYAN}→ IAM role and instance profile${NC}"

    if aws iam get-role --role-name "$ROLE_NAME" &>/dev/null; then
        echo "  ✓ Role $ROLE_NAME already exists, skipping"
    else
        aws iam create-role \
            --role-name "$ROLE_NAME" \
            --assume-role-policy-document "$TRUST_POLICY" \
            --tags '[{"Key":"ManagedBy","Value":"benchmark-automation"}]' \
            --output text --query 'Role.Arn' | \
            xargs -I{} echo "  ✓ Role created: {}"
    fi

    # Always re-apply inline policy (idempotent)
    aws iam put-role-policy \
        --role-name "$ROLE_NAME" \
        --policy-name "$POLICY_NAME" \
        --policy-document "$INLINE_POLICY"
    echo "  ✓ Inline policy $POLICY_NAME applied"

    if aws iam get-instance-profile --instance-profile-name "$PROFILE_NAME" &>/dev/null; then
        echo "  ✓ Instance profile $PROFILE_NAME already exists, skipping"
    else
        aws iam create-instance-profile \
            --instance-profile-name "$PROFILE_NAME" \
            --output json > /dev/null
        aws iam add-role-to-instance-profile \
            --instance-profile-name "$PROFILE_NAME" \
            --role-name "$ROLE_NAME"
        echo "  ✓ Instance profile $PROFILE_NAME created and role attached"
    fi
}

# ---------------------------------------------------------------------------
# Create security group (egress: TCP 443 only; ingress: none by default)
# ---------------------------------------------------------------------------
create_sg() {
    echo
    echo -e "${CYAN}→ Security group${NC}"

    SG_ID=$(aws ec2 describe-security-groups \
        --filters "Name=group-name,Values=$SG_NAME" "Name=tag:ManagedBy,Values=benchmark-automation" \
        --query 'SecurityGroups[0].GroupId' \
        --output text 2>/dev/null || true)

    if [ -n "$SG_ID" ] && [ "$SG_ID" != "None" ]; then
        echo "  ✓ Security group $SG_NAME already exists ($SG_ID), skipping"
        return
    fi

    SG_ID=$(aws ec2 create-security-group \
        --group-name "$SG_NAME" \
        --description "Benchmark runner - SSH ingress added and revoked dynamically" \
        --query 'GroupId' \
        --output text)

    aws ec2 create-tags \
        --resources "$SG_ID" \
        --tags "Key=ManagedBy,Value=benchmark-automation" \
               "Key=Name,Value=$SG_NAME"

    # Remove the default allow-all egress rules and restrict to HTTPS only
    aws ec2 revoke-security-group-egress \
        --group-id "$SG_ID" \
        --ip-permissions '[{"IpProtocol":"-1","IpRanges":[{"CidrIp":"0.0.0.0/0"}]}]' \
        > /dev/null 2>&1 || true
    aws ec2 revoke-security-group-egress \
        --group-id "$SG_ID" \
        --ip-permissions '[{"IpProtocol":"-1","Ipv6Ranges":[{"CidrIpv6":"::/0"}]}]' \
        > /dev/null 2>&1 || true

    aws ec2 authorize-security-group-egress \
        --group-id "$SG_ID" \
        --ip-permissions '[{"IpProtocol":"tcp","FromPort":443,"ToPort":443,"IpRanges":[{"CidrIp":"0.0.0.0/0","Description":"HTTPS for packages and Go downloads"}]}]'

    echo "  ✓ Security group $SG_NAME created ($SG_ID)"
    echo "  ✓ Egress: TCP 443 → 0.0.0.0/0 only"
}

# ---------------------------------------------------------------------------
# Create EC2 key pair and save private key locally
# ---------------------------------------------------------------------------
create_key_pair() {
    echo
    echo -e "${CYAN}→ Key pair${NC}"

    if aws ec2 describe-key-pairs --key-names "$KEY_NAME" &>/dev/null; then
        echo "  ✓ Key pair $KEY_NAME already exists in AWS, skipping"
        if [ ! -f "$KEY_PATH" ]; then
            echo -e "  ${YELLOW}⚠${NC} Local key file $KEY_PATH is missing."
            echo "    Delete the key pair in AWS and re-run setup to regenerate."
        else
            echo "  ✓ Local key file: $KEY_PATH"
        fi
        return
    fi

    # Pre-create with mode 600 so the private key is never world-readable,
    # even transiently between the write and a subsequent chmod call.
    mkdir -p "$(dirname "$KEY_PATH")"
    install -m 600 /dev/null "$KEY_PATH"
    aws ec2 create-key-pair \
        --key-name "$KEY_NAME" \
        --key-type rsa \
        --key-format pem \
        --query 'KeyMaterial' \
        --output text > "$KEY_PATH"

    echo "  ✓ Key pair $KEY_NAME created"
    echo "  ✓ Private key saved to $KEY_PATH"
}

# ---------------------------------------------------------------------------
# Destroy all resources created by this script
# ---------------------------------------------------------------------------
destroy_resources() {
    echo "=== Destroying benchmark runner AWS resources ==="

    echo
    echo -e "${CYAN}→ Key pair${NC}"
    if aws ec2 describe-key-pairs --key-names "$KEY_NAME" &>/dev/null; then
        aws ec2 delete-key-pair --key-name "$KEY_NAME"
        echo "  ✓ Key pair $KEY_NAME deleted from AWS"
    else
        echo "  ✓ Key pair $KEY_NAME not found, skipping"
    fi
    if [ -f "$KEY_PATH" ]; then
        rm -f "$KEY_PATH"
        echo "  ✓ Local file $KEY_PATH removed"
    fi

    echo
    echo -e "${CYAN}→ Security group${NC}"
    SG_ID=$(aws ec2 describe-security-groups \
        --filters "Name=group-name,Values=$SG_NAME" "Name=tag:ManagedBy,Values=benchmark-automation" \
        --query 'SecurityGroups[0].GroupId' \
        --output text 2>/dev/null || true)
    if [ -n "$SG_ID" ] && [ "$SG_ID" != "None" ]; then
        aws ec2 delete-security-group --group-id "$SG_ID"
        echo "  ✓ Security group $SG_NAME ($SG_ID) deleted"
    else
        echo "  ✓ Security group $SG_NAME not found, skipping"
    fi

    echo
    echo -e "${CYAN}→ IAM resources${NC}"
    if aws iam get-instance-profile --instance-profile-name "$PROFILE_NAME" &>/dev/null; then
        aws iam remove-role-from-instance-profile \
            --instance-profile-name "$PROFILE_NAME" \
            --role-name "$ROLE_NAME" 2>/dev/null || true
        aws iam delete-instance-profile --instance-profile-name "$PROFILE_NAME"
        echo "  ✓ Instance profile $PROFILE_NAME deleted"
    else
        echo "  ✓ Instance profile $PROFILE_NAME not found, skipping"
    fi
    if aws iam get-role --role-name "$ROLE_NAME" &>/dev/null; then
        aws iam delete-role-policy \
            --role-name "$ROLE_NAME" \
            --policy-name "$POLICY_NAME" 2>/dev/null || true
        aws iam delete-role --role-name "$ROLE_NAME"
        echo "  ✓ IAM role $ROLE_NAME deleted"
    else
        echo "  ✓ IAM role $ROLE_NAME not found, skipping"
    fi

    echo
    echo -e "${GREEN}✓ All benchmark runner AWS resources destroyed${NC}"
}

# ---------------------------------------------------------------------------
# Print summary of created resources
# ---------------------------------------------------------------------------
print_summary() {
    echo
    echo "=== Setup complete ==="
    echo
    echo "Resources created/verified:"

    ROLE_ARN=$(aws iam get-role --role-name "$ROLE_NAME" \
        --query 'Role.Arn' --output text 2>/dev/null || echo "N/A")
    echo "  IAM role:         $ROLE_NAME ($ROLE_ARN)"
    echo "  Instance profile: $PROFILE_NAME"

    SG_ID=$(aws ec2 describe-security-groups \
        --filters "Name=group-name,Values=$SG_NAME" "Name=tag:ManagedBy,Values=benchmark-automation" \
        --query 'SecurityGroups[0].GroupId' \
        --output text 2>/dev/null || echo "N/A")
    echo "  Security group:   $SG_NAME ($SG_ID)"
    echo "  Key pair:         $KEY_NAME → $KEY_PATH"

    echo
    echo "Next step:"
    echo "  ./run-benchmark.sh amd64 1.24.0 1.25.0"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
case "${1:-}" in
    --destroy)
        check_prereqs
        destroy_resources
        ;;
    "")
        echo "=== Benchmark runner AWS setup ==="
        check_prereqs
        create_iam
        create_sg
        create_key_pair
        print_summary
        ;;
    *)
        echo "Usage: $0 [--destroy]"
        echo
        echo "  (no args)   Create AWS resources (idempotent)"
        echo "  --destroy   Remove all created resources"
        exit 1
        ;;
esac
