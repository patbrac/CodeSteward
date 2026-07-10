#!/usr/bin/env python3
"""Create a normalized native steward archive and its SHA-256 sidecar."""

from __future__ import annotations

import argparse
import gzip
import hashlib
import os
import re
import tarfile
import zipfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SAFE = re.compile(r"^[0-9A-Za-z._+-]+$")


def add_tar_file(archive: tarfile.TarFile, name: str, data: bytes, mode: int) -> None:
    info = tarfile.TarInfo(name)
    info.size = len(data)
    info.mode = mode
    info.uid = 0
    info.gid = 0
    info.uname = "root"
    info.gname = "root"
    info.mtime = 0
    archive.addfile(info, __import__("io").BytesIO(data))


def package_tar(path: Path, root_name: str, files: list[tuple[str, bytes, int]]) -> None:
    with path.open("wb") as raw:
        with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0, compresslevel=9) as compressed:
            with tarfile.open(fileobj=compressed, mode="w", format=tarfile.PAX_FORMAT) as archive:
                directory = tarfile.TarInfo(root_name + "/")
                directory.type = tarfile.DIRTYPE
                directory.mode = 0o755
                directory.uid = directory.gid = 0
                directory.uname = directory.gname = "root"
                directory.mtime = 0
                archive.addfile(directory)
                for relative, data, mode in files:
                    add_tar_file(archive, f"{root_name}/{relative}", data, mode)


def package_zip(path: Path, root_name: str, files: list[tuple[str, bytes, int]]) -> None:
    with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        directory = zipfile.ZipInfo(root_name + "/", (1980, 1, 1, 0, 0, 0))
        directory.create_system = 3
        directory.external_attr = (0o755 << 16) | 0x10
        archive.writestr(directory, b"")
        for relative, data, mode in files:
            info = zipfile.ZipInfo(f"{root_name}/{relative}", (1980, 1, 1, 0, 0, 0))
            info.create_system = 3
            info.external_attr = mode << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(info, data, compress_type=zipfile.ZIP_DEFLATED, compresslevel=9)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=Path, default=os.environ.get("PACKAGE_BINARY"))
    parser.add_argument("--target", default=os.environ.get("PACKAGE_TARGET"))
    parser.add_argument("--version", default=os.environ.get("PACKAGE_VERSION"))
    parser.add_argument("--format", choices=("tar.gz", "zip"), default=os.environ.get("PACKAGE_FORMAT"))
    parser.add_argument("--output-dir", type=Path, default=os.environ.get("PACKAGE_OUTPUT_DIR", "dist"))
    parser.add_argument("--attributions", type=Path, default=os.environ.get("ATTRIBUTIONS_PATH"))
    parser.add_argument("--github-output", type=Path, default=os.environ.get("GITHUB_OUTPUT"))
    options = parser.parse_args()
    if not options.binary or not options.binary.is_file():
        parser.error(f"release binary does not exist: {options.binary}")
    if not options.target or not SAFE.fullmatch(options.target):
        parser.error("invalid or missing target")
    if not options.version or not SAFE.fullmatch(options.version):
        parser.error("invalid or missing version")
    if not options.format:
        parser.error("archive format is required")
    if not options.attributions or not options.attributions.is_file() or options.attributions.stat().st_size == 0:
        parser.error("ATTRIBUTIONS_PATH/--attributions must name generated third-party license text")
    attribution_text = options.attributions.read_text(encoding="utf-8")
    if "THIRD-PARTY LICENSES FOR STEWARD" not in attribution_text or "Package:" not in attribution_text:
        parser.error("generated third-party license text is incomplete")

    output_dir = options.output_dir.resolve()
    output_dir.mkdir(parents=True, exist_ok=True)
    asset_name = f"steward-{options.version}-{options.target}.{options.format}"
    archive_path = output_dir / asset_name
    root_name = f"steward-{options.version}-{options.target}"
    binary_name = "steward.exe" if options.target.endswith("windows-msvc") else "steward"
    files: list[tuple[str, bytes, int]] = [
        (binary_name, options.binary.read_bytes(), 0o755),
        ("LICENSE", (ROOT / "LICENSE").read_bytes(), 0o644),
        ("NOTICE", (ROOT / "NOTICE").read_bytes(), 0o644),
        ("README.md", (ROOT / "README.md").read_bytes(), 0o644),
        ("THIRD_PARTY_LICENSES.txt", attribution_text.encode("utf-8"), 0o644),
    ]
    if options.format == "tar.gz":
        package_tar(archive_path, root_name, files)
    else:
        package_zip(archive_path, root_name, files)

    digest = hashlib.sha256(archive_path.read_bytes()).hexdigest()
    checksum_path = output_dir / f"{asset_name}.sha256"
    checksum_path.write_text(f"{digest}  {asset_name}\n", encoding="ascii", newline="\n")
    if options.github_output:
        with options.github_output.open("a", encoding="utf-8", newline="\n") as output:
            output.write(f"asset-name={asset_name}\n")
            output.write(f"asset-path={archive_path.as_posix()}\n")
    print(f"created {archive_path} ({digest})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
