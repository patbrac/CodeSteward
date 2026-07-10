#!/usr/bin/env python3
"""Record and bound the Linux artifact's referenced GLIBC symbol versions."""

from __future__ import annotations

import argparse
import os
import re
import subprocess
from pathlib import Path


VERSION = re.compile(r"GLIBC_(\d+)\.(\d+)")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=Path, default=os.environ.get("GLIBC_BINARY"))
    parser.add_argument("--output", type=Path, default=os.environ.get("GLIBC_OUTPUT"))
    parser.add_argument("--maximum", default="2.39")
    options = parser.parse_args()
    if not options.binary or not options.binary.is_file() or not options.output:
        parser.error("GLIBC_BINARY/--binary and GLIBC_OUTPUT/--output are required")

    maximum = tuple(map(int, options.maximum.split(".")))
    result = subprocess.run(
        ["readelf", "--version-info", str(options.binary)],
        text=True,
        capture_output=True,
        check=True,
    )
    versions = sorted({tuple(map(int, match)) for match in VERSION.findall(result.stdout)})
    if not versions:
        raise SystemExit("no referenced GLIBC versions found in Linux release binary")
    observed = versions[-1]
    if observed > maximum:
        raise SystemExit(f"maximum referenced GLIBC {observed} exceeds qualified baseline {maximum}")

    options.output.parent.mkdir(parents=True, exist_ok=True)
    options.output.write_text(
        "qualified_runner=ubuntu-24.04\n"
        f"qualified_glibc_max={maximum[0]}.{maximum[1]}\n"
        f"observed_glibc_max={observed[0]}.{observed[1]}\n"
        + "referenced_versions="
        + ",".join(f"{major}.{minor}" for major, minor in versions)
        + "\n",
        encoding="ascii",
        newline="\n",
    )
    print(options.output.read_text(encoding="ascii"), end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
