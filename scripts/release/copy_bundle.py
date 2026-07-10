#!/usr/bin/env python3
"""Copy an action-produced attestation bundle to a stable asset name."""

from __future__ import annotations

import os
import shutil
from pathlib import Path


source = Path(os.environ["SOURCE_BUNDLE"])
destination = Path(os.environ["DESTINATION_BUNDLE"])
if not source.is_file():
    raise SystemExit(f"attestation bundle not found: {source}")
destination.parent.mkdir(parents=True, exist_ok=True)
shutil.copyfile(source, destination)
print(f"copied attestation bundle to {destination}")
