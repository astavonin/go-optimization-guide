#!/usr/bin/env python3
"""Unit tests for run-benchmark.sh stale SSH rule pruning.

Regression coverage for the 2026-09-05 finding: a port-22 ingress rule for
217.29.24.132/32 was left on benchmark-runner-sg by an earlier run whose
cleanup trap never fired (the trap cannot run on SIGKILL, host sleep, or a
dropped connection). The security group is documented as "SSH ingress added
and revoked dynamically", so a persistent rule violates its own contract.

The prune block is exercised by extracting it from the script and running it
against a stubbed `aws` CLI, so these tests cover the real shell code rather
than a reimplementation of it.
"""

import os
import re
import subprocess
import tempfile
from pathlib import Path

RUN_BENCHMARK = Path(__file__).parent / "run-benchmark.sh"

# Mirrors `aws ec2 describe-security-groups ... --output text`, which emits
# CIDRs tab-separated on one line.
STUB_AWS = """#!/bin/sh
for arg in "$@"; do
    if [ "$arg" = "describe-security-groups" ]; then
        printf '%s\\n' "$STUB_RULES"
        exit 0
    fi
    if [ "$arg" = "revoke-security-group-ingress" ]; then
        # Record each revoked CIDR so the test can assert on them.
        while [ $# -gt 0 ]; do
            if [ "$1" = "--cidr" ]; then echo "$2" >> "$STUB_REVOKED"; fi
            shift
        done
        exit 0
    fi
done
exit 0
"""


def _extract_prune_block() -> str:
    """Pull the prune block out of run-benchmark.sh verbatim."""
    source = RUN_BENCHMARK.read_text()
    match = re.search(
        r'^echo -e "\$\{CYAN\}→\$\{NC\} Pruning stale SSH rules\.\.\."$.*?'
        r'^fi$',
        source, re.MULTILINE | re.DOTALL
    )
    assert match, "run-benchmark.sh has no stale-SSH-rule prune block"
    return match.group(0)


def _run_prune(existing_cidrs: list[str], operator_ip: str) -> list[str]:
    """Run the real prune block; return the CIDRs it revoked."""
    with tempfile.TemporaryDirectory() as tmp:
        tmpdir = Path(tmp)
        stub = tmpdir / "aws"
        stub.write_text(STUB_AWS)
        stub.chmod(0o755)
        revoked = tmpdir / "revoked"
        revoked.touch()

        script = (
            'RED=""; YELLOW=""; GREEN=""; CYAN=""; NC=""\n'
            'SG_ID="sg-test"\n'
            f'OPERATOR_IP="{operator_ip}"\n'
            + _extract_prune_block() + "\n"
        )

        env = dict(
            os.environ,
            PATH=f"{tmpdir}:{os.environ['PATH']}",
            STUB_RULES="\t".join(existing_cidrs),
            STUB_REVOKED=str(revoked),
        )
        subprocess.run(["bash", "-c", script], env=env,
                       capture_output=True, text=True, check=True)
        return [c for c in revoked.read_text().splitlines() if c]


def test_stale_rule_from_another_ip_is_revoked():
    """Reproduces the observed leak: a rule for a former operator IP."""
    revoked = _run_prune(["217.29.24.132/32"], "92.62.77.196")

    assert revoked == ["217.29.24.132/32"], (
        f"stale rule should have been pruned, got {revoked}"
    )


def test_current_operator_rule_is_preserved():
    """Pruning must never revoke the rule this run depends on."""
    revoked = _run_prune(["92.62.77.196/32"], "92.62.77.196")

    assert revoked == [], (
        f"the current operator's own rule must survive, but {revoked} was revoked"
    )


def test_concurrent_run_from_same_ip_is_not_cut_off():
    """Two runs from this machine share one rule; neither may prune it.

    This is the case that made concurrent amd64+arm64 collection possible.
    """
    revoked = _run_prune(["92.62.77.196/32"], "92.62.77.196")

    assert "92.62.77.196/32" not in revoked


def test_multiple_stale_rules_all_revoked():
    """A leak can accumulate across several failed runs."""
    revoked = _run_prune(
        ["217.29.24.132/32", "203.0.113.7/32", "92.62.77.196/32"],
        "92.62.77.196",
    )

    assert sorted(revoked) == ["203.0.113.7/32", "217.29.24.132/32"]
    assert "92.62.77.196/32" not in revoked


def test_no_rules_is_not_an_error():
    """The common case after a clean run: nothing to prune."""
    assert _run_prune([], "92.62.77.196") == []


def test_usage_documents_the_concurrency_caveat():
    """Pruning silently breaks multi-operator concurrency; that must be stated."""
    source = RUN_BENCHMARK.read_text()

    assert "different operator IP" in source, (
        "show_usage must warn that concurrent runs from different operator "
        "IPs prune each other's SSH rule"
    )
