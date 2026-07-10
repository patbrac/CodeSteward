#!/usr/bin/env python3
"""Verify the complete target artifact, checksum, SBOM, and bundle matrix."""

from __future__ import annotations

import argparse
import hashlib
import json
import tarfile
import zipfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def verify_archive(path: Path, binary_name: str) -> None:
    contents: dict[str, bytes] = {}
    if path.name.endswith(".tar.gz"):
        with tarfile.open(path, "r:gz") as archive:
            for member in archive.getmembers():
                if member.isfile() and "/" in member.name:
                    handle = archive.extractfile(member)
                    if handle is None:
                        raise SystemExit(f"cannot read {member.name} from {path.name}")
                    contents[member.name.split("/", 1)[1]] = handle.read()
    else:
        with zipfile.ZipFile(path) as archive:
            for name in archive.namelist():
                if "/" in name and not name.endswith("/"):
                    contents[name.split("/", 1)[1]] = archive.read(name)
    files = set(contents)
    required = {binary_name, "LICENSE", "NOTICE", "README.md", "THIRD_PARTY_LICENSES.txt"}
    if not required.issubset(files):
        raise SystemExit(f"archive {path.name} lacks required files: {sorted(required - files)}")
    attributions = contents["THIRD_PARTY_LICENSES.txt"].decode("utf-8")
    if "THIRD-PARTY LICENSES FOR STEWARD" not in attributions or "Package:" not in attributions:
        raise SystemExit(f"archive {path.name} has empty or incomplete third-party attribution material")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("directory", type=Path)
    parser.add_argument("--version", required=True)
    options = parser.parse_args()
    directory = options.directory.resolve()
    matrix = json.loads((ROOT / "scripts" / "release" / "targets.json").read_text(encoding="utf-8"))
    expected_hashed: set[str] = set()
    for target in matrix["include"]:
        asset = f"steward-{options.version}-{target['target']}.{target['archive']}"
        expected = (
            asset,
            asset + ".spdx.json",
            asset + ".provenance.bundle.json",
            asset + ".sbom.bundle.json",
        )
        for name in expected:
            path = directory / name
            if not path.is_file() or path.stat().st_size == 0:
                raise SystemExit(f"missing or empty release asset: {name}")
            expected_hashed.add(name)
        verify_archive(directory / asset, "steward.exe" if target["archive"] == "zip" else "steward")
        if target["id"] == "linux-x64":
            glibc = asset + ".glibc.txt"
            glibc_path = directory / glibc
            if not glibc_path.is_file() or "qualified_glibc_max=2.39" not in glibc_path.read_text(encoding="ascii"):
                raise SystemExit(f"missing or invalid Linux glibc evidence: {glibc}")
            expected_hashed.add(glibc)
        sidecar = directory / (asset + ".sha256")
        expected_line = f"{hashlib.sha256((directory / asset).read_bytes()).hexdigest()}  {asset}\n"
        if sidecar.read_text(encoding="ascii") != expected_line:
            raise SystemExit(f"invalid checksum sidecar: {sidecar.name}")
        sbom = json.loads((directory / (asset + ".spdx.json")).read_text(encoding="utf-8"))
        if not str(sbom.get("spdxVersion", "")).startswith("SPDX-"):
            raise SystemExit(f"invalid SPDX document: {asset}.spdx.json")
        for suffix in (".provenance.bundle.json", ".sbom.bundle.json"):
            json.loads((directory / (asset + suffix)).read_text(encoding="utf-8"))

    sums = directory / "SHA256SUMS"
    actual_hashed: set[str] = set()
    for line in sums.read_text(encoding="ascii").splitlines():
        digest, name = line.split("  ", 1)
        if Path(name).name != name:
            raise SystemExit(f"unsafe checksum path: {name}")
        path = directory / name
        if hashlib.sha256(path.read_bytes()).hexdigest() != digest:
            raise SystemExit(f"SHA256SUMS mismatch: {name}")
        actual_hashed.add(name)
    if actual_hashed != expected_hashed:
        raise SystemExit(
            f"SHA256SUMS matrix mismatch: missing={sorted(expected_hashed - actual_hashed)}, "
            f"unexpected={sorted(actual_hashed - expected_hashed)}"
        )
    print(f"verified {len(matrix['include'])} release targets and {len(expected_hashed)} hashed assets")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
