#!/usr/bin/env python3
"""Safely extract, install, run offline, and uninstall a release archive."""

from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
import tarfile
import tempfile
import zipfile
from pathlib import Path, PurePosixPath


ROOT = Path(__file__).resolve().parents[2]


def safe_name(name: str) -> PurePosixPath:
    path = PurePosixPath(name)
    if path.is_absolute() or not path.parts or any(part in {"", ".", ".."} for part in path.parts):
        raise ValueError(f"unsafe archive member: {name!r}")
    return path


def extract(archive: Path, destination: Path) -> None:
    if archive.name.endswith(".tar.gz"):
        with tarfile.open(archive, "r:gz") as source:
            for member in source.getmembers():
                relative = safe_name(member.name)
                target = destination.joinpath(*relative.parts)
                if member.isdir():
                    target.mkdir(parents=True, exist_ok=True)
                    continue
                if not member.isfile():
                    raise ValueError(f"links and special files are forbidden: {member.name}")
                target.parent.mkdir(parents=True, exist_ok=True)
                handle = source.extractfile(member)
                if handle is None:
                    raise ValueError(f"cannot read archive member: {member.name}")
                target.write_bytes(handle.read())
                target.chmod(member.mode & 0o777)
    elif archive.suffix == ".zip":
        with zipfile.ZipFile(archive) as source:
            for member in source.infolist():
                relative = safe_name(member.filename)
                target = destination.joinpath(*relative.parts)
                if member.is_dir():
                    target.mkdir(parents=True, exist_ok=True)
                    continue
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(source.read(member))
    else:
        raise ValueError(f"unsupported archive: {archive}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--archive", type=Path, default=os.environ.get("RELEASE_ARCHIVE"))
    parser.add_argument("--version", default=os.environ.get("RELEASE_VERSION"))
    options = parser.parse_args()
    if not options.archive or not options.version:
        parser.error("--archive/--version or RELEASE_ARCHIVE/RELEASE_VERSION are required")
    archive = options.archive.resolve()
    if not archive.is_file():
        parser.error(f"archive does not exist: {archive}")

    work = Path(tempfile.mkdtemp(prefix="steward-clean-install-"))
    install = work / "install" / "bin"
    try:
        unpacked = work / "unpacked"
        extract(archive, unpacked)
        expected_name = "steward.exe" if archive.suffix == ".zip" else "steward"
        candidates = sorted(unpacked.rglob(expected_name))
        if len(candidates) != 1:
            raise RuntimeError(f"archive must contain exactly one {expected_name}; found {len(candidates)}")
        archive_root = candidates[0].parent
        for required in ("LICENSE", "NOTICE", "README.md", "THIRD_PARTY_LICENSES.txt"):
            path = archive_root / required
            if not path.is_file() or path.stat().st_size == 0:
                raise RuntimeError(f"archive is missing required material: {required}")
        attributions = (archive_root / "THIRD_PARTY_LICENSES.txt").read_text(encoding="utf-8")
        if "Package:" not in attributions or "Declared license:" not in attributions:
            raise RuntimeError("third-party attribution material has no dependency entries")
        install.mkdir(parents=True)
        binary = install / expected_name
        shutil.copy2(candidates[0], binary)
        if expected_name == "steward":
            binary.chmod(0o755)

        version = subprocess.run(
            [str(binary), "version"], text=True, capture_output=True, timeout=30, check=False
        )
        if version.returncode != 0 or options.version not in version.stdout:
            raise RuntimeError(f"installed version check failed: {version!r}")
        subprocess.run(
            [sys.executable, str(ROOT / "scripts" / "ci" / "no_network_smoke.py"), "--binary", str(binary)],
            cwd=ROOT,
            timeout=180,
            check=True,
        )

        shutil.rmtree(work / "install")
        if binary.exists():
            raise RuntimeError("uninstall left the steward binary behind")
    finally:
        shutil.rmtree(work, ignore_errors=True)

    print(f"clean install/run/offline/uninstall passed for {archive.name}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
