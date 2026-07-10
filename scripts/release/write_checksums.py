#!/usr/bin/env python3
"""Write a deterministic release-wide SHA256SUMS file."""

from __future__ import annotations

import argparse
import hashlib
from pathlib import Path


parser = argparse.ArgumentParser()
parser.add_argument("directory", type=Path)
options = parser.parse_args()
directory = options.directory.resolve()
assets = sorted(
    path
    for path in directory.iterdir()
    if path.is_file() and path.name != "SHA256SUMS" and not path.name.endswith(".sha256")
)
if not assets:
    raise SystemExit("no release assets found")
lines = [f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}" for path in assets]
(directory / "SHA256SUMS").write_text("\n".join(lines) + "\n", encoding="ascii", newline="\n")
print(f"wrote SHA256SUMS for {len(assets)} assets")
