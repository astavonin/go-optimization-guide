#!/usr/bin/env python3
"""Unit tests for bootstrap.sh package installation.

Regression coverage for the 2026-09-05 failure: the EC2 bootstrap died at
"failed: package installation" because it requested the unversioned
``kernel-tools`` package. Amazon Linux 2023 had rolled its "latest" AMI from
kernel 6.1 to 6.18, which preinstalls ``kernel6.18-tools``; the unversioned
name resolves to the 6.1.x build and hard-conflicts with it.
"""

import os
import re
import subprocess
import tempfile
from pathlib import Path

BOOTSTRAP = Path(__file__).parent / "bootstrap.sh"


def _bootstrap_source(path: Path = BOOTSTRAP) -> str:
    return path.read_text()


def _mandatory_install_packages(source: str) -> list[str]:
    """Packages from yum install lines that must succeed (no `|| true` guard).

    A line ending in a `|| ...` fallback chain is tolerant of failure; a bare
    line aborts the run under `set -euo pipefail`.
    """
    packages = []
    for raw in source.splitlines():
        line = raw.strip()
        if not line.startswith("yum install"):
            continue
        if line.endswith("\\") or "||" in line:
            continue
        packages.extend(line.replace("yum install -y", "").split())
    return packages


def test_kernel_tools_is_not_a_mandatory_package():
    """Reproduces the observed failure.

    Requesting unversioned `kernel-tools` in the mandatory package list is what
    aborted bootstrap on a kernel-6.18 AMI. It may only appear as a fallback.
    """
    mandatory = _mandatory_install_packages(_bootstrap_source())

    assert "kernel-tools" not in mandatory, (
        "bootstrap.sh requests unversioned 'kernel-tools' as a mandatory "
        "package; this conflicts with the preinstalled kernelX.Y-tools on "
        f"current AL2023 AMIs. Mandatory packages: {mandatory}"
    )


def test_base_packages_still_installed():
    """The fix must not drop the packages the run genuinely depends on."""
    mandatory = _mandatory_install_packages(_bootstrap_source())

    for pkg in ("git", "python3", "jq", "util-linux"):
        assert pkg in mandatory, f"bootstrap.sh no longer installs {pkg}"


def _derive_kernel_tools_name(uname_release: str) -> str:
    """Run bootstrap.sh's own KERNEL_TOOLS assignment against a stubbed uname.

    Executes the real line extracted from the script rather than a
    reimplementation, so the test tracks the script if it changes.
    """
    source = _bootstrap_source()
    match = re.search(r'^\s*(KERNEL_TOOLS=.*)$', source, re.MULTILINE)
    assert match, "bootstrap.sh has no KERNEL_TOOLS assignment"
    assignment = match.group(1)

    with tempfile.TemporaryDirectory() as tmp:
        stub = Path(tmp) / "uname"
        stub.write_text(f'#!/bin/sh\necho "{uname_release}"\n')
        stub.chmod(0o755)

        env = dict(os.environ, PATH=f"{tmp}:{os.environ['PATH']}")
        result = subprocess.run(
            ["bash", "-c", f'{assignment}\necho "$KERNEL_TOOLS"'],
            capture_output=True, text=True, env=env, check=True
        )
    return result.stdout.strip()


def test_kernel_tools_name_matches_running_kernel():
    """The AMI that broke the March-era script must resolve to its own package."""
    assert _derive_kernel_tools_name(
        "6.18.44-99.149.amzn2023.x86_64") == "kernel6.18-tools"
    assert _derive_kernel_tools_name(
        "6.18.44-99.149.amzn2023.aarch64") == "kernel6.18-tools"
    assert _derive_kernel_tools_name(
        "6.12.0-1.amzn2023.x86_64") == "kernel6.12-tools"


def test_unversioned_kernel_tools_remains_a_fallback():
    """Kernel 6.1 AMIs ship plain `kernel-tools`; that path must still work."""
    source = _bootstrap_source()

    assert re.search(r'\|\|\s*yum install -y kernel-tools', source), (
        "bootstrap.sh must fall back to unversioned 'kernel-tools' for "
        "kernel 6.1 AMIs, where no kernel6.1-tools package exists"
    )


def test_missing_cpupower_is_surfaced():
    """Silent loss of cpupower would invalidate the published methodology.

    `cpupower idle-set -D 0` has no sysfs fallback, so deep C-states would
    stay enabled while the tracking page still claims they are disabled.
    """
    source = _bootstrap_source()

    assert "cpupower unavailable" in source, (
        "bootstrap.sh must warn when cpupower is missing — deep C-state "
        "disabling has no fallback and its absence changes result variance"
    )
