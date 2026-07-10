#!/usr/bin/env python3
"""Run default Phase 1 CLI surfaces with native outbound networking denied."""

from __future__ import annotations

import argparse
import os
import platform
import subprocess
import sys
import uuid
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def valid_config() -> Path:
    candidates = sorted(
        path
        for path in (ROOT / "schemas" / "config").rglob("*")
        if path.is_file() and "valid" in path.parts and path.suffix in {".json", ".toml"}
    )
    if not candidates:
        raise FileNotFoundError("no valid config example under schemas/config")
    return candidates[0]


def powershell(command: str, environment: dict[str, str]) -> None:
    result = subprocess.run(
        ["powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", command],
        env=environment,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or result.stdout.strip())


def run_command(prefix: list[str], binary: Path, arguments: list[str], allowed_codes: set[int]) -> None:
    environment = os.environ.copy()
    environment.update(
        {
            "HTTP_PROXY": "http://127.0.0.1:1",
            "HTTPS_PROXY": "http://127.0.0.1:1",
            "ALL_PROXY": "socks5://127.0.0.1:1",
            "NO_PROXY": "",
            "NO_COLOR": "1",
            "TERM": "dumb",
        }
    )
    result = subprocess.run(
        [*prefix, str(binary), *arguments],
        cwd=ROOT,
        env=environment,
        text=True,
        encoding="utf-8",
        errors="replace",
        capture_output=True,
        timeout=30,
        check=False,
    )
    if result.returncode not in allowed_codes:
        raise RuntimeError(
            f"offline command {[str(binary), *arguments]!r} returned {result.returncode}; "
            f"stdout={result.stdout!r}; stderr={result.stderr!r}"
        )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=Path, required=True)
    options = parser.parse_args()
    binary = options.binary.resolve()
    if not binary.is_file():
        parser.error(f"binary does not exist: {binary}")

    system = platform.system()
    prefix: list[str] = []
    rule_name: str | None = None
    firewall_installed = False
    firewall_environment = os.environ.copy()
    try:
        if system == "Linux":
            prefix = ["sudo", "-n", "unshare", "--net", "--"]
        elif system == "Darwin":
            prefix = ["sandbox-exec", "-p", "(version 1)(allow default)(deny network*)"]
        elif system == "Windows":
            rule_name = "steward-ci-" + uuid.uuid4().hex
            firewall_environment["STEWARD_CI_RULE"] = rule_name
            firewall_environment["STEWARD_CI_BINARY"] = str(binary)
            powershell(
                "$ErrorActionPreference='Stop'; "
                "New-NetFirewallRule -Name $env:STEWARD_CI_RULE -DisplayName $env:STEWARD_CI_RULE "
                "-Direction Outbound -Action Block -Program $env:STEWARD_CI_BINARY -Profile Any | Out-Null",
                firewall_environment,
            )
            firewall_installed = True
        else:
            raise RuntimeError(f"unsupported native isolation platform: {system}")

        commands = (
            ([], {0, 2}),
            (["version"], {0}),
            (["doctor"], {0}),
            (["config", "validate", str(valid_config())], {0}),
        )
        for arguments, allowed_codes in commands:
            run_command(prefix, binary, arguments, allowed_codes)
    finally:
        if rule_name is not None and firewall_installed:
            powershell(
                "$ErrorActionPreference='Stop'; "
                "Get-NetFirewallRule -Name $env:STEWARD_CI_RULE -ErrorAction SilentlyContinue | Remove-NetFirewallRule",
                firewall_environment,
            )

    print(f"native no-network smoke passed on {system}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
