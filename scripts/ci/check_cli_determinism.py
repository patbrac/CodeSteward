#!/usr/bin/env python3
"""Repeat Phase 1 CLI surfaces under varied process environments."""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
import tempfile
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


def invoke(binary: Path, arguments: list[str], cwd: Path, variation: int) -> tuple[int, str, str]:
    environment = os.environ.copy()
    environment.update(
        {
            "NO_COLOR": "1",
            "TERM": "dumb",
            "TZ": "UTC" if variation % 2 == 0 else "Pacific/Honolulu",
            "LANG": "C" if variation % 2 == 0 else "en_US.UTF-8",
            "LC_ALL": "C" if variation % 2 == 0 else "en_US.UTF-8",
        }
    )
    result = subprocess.run(
        [str(binary), *arguments],
        cwd=cwd,
        env=environment,
        text=True,
        encoding="utf-8",
        errors="strict",
        capture_output=True,
        timeout=30,
        check=False,
    )
    return result.returncode, result.stdout.replace("\r\n", "\n"), result.stderr.replace("\r\n", "\n")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=Path, required=True)
    options = parser.parse_args()
    binary = options.binary.resolve()
    if not binary.is_file():
        parser.error(f"binary does not exist: {binary}")

    config = valid_config()
    commands = (["--help"], ["version"], ["config", "validate", str(config)])
    with tempfile.TemporaryDirectory(prefix="steward-determinism-a-") as first, tempfile.TemporaryDirectory(
        prefix="steward-determinism-b-"
    ) as second:
        directories = (Path(first), Path(second), Path(first), Path(second))
        for command in commands:
            observations = [invoke(binary, list(command), cwd, index) for index, cwd in enumerate(directories)]
            if observations[0][0] != 0:
                print(f"command failed: {command!r}: {observations[0]}", file=sys.stderr)
                return 1
            if any(observation != observations[0] for observation in observations[1:]):
                print(f"non-deterministic output for {command!r}: {observations!r}", file=sys.stderr)
                return 1
    print("Phase 1 CLI ordering/determinism smoke passed across four varied runs")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
