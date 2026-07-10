#!/usr/bin/env python3
"""Generate deterministic third-party license text for a target's non-dev graph."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
from collections import deque
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
LICENSE_PREFIXES = ("license", "licence", "copying", "notice", "unlicense")


def license_files(package: dict[str, object]) -> list[Path]:
    manifest_dir = Path(str(package["manifest_path"])).parent
    declared = package.get("license_file")
    if declared:
        path = Path(str(declared))
        if not path.is_absolute():
            path = manifest_dir / path
        return [path]
    return sorted(
        (
            path
            for path in manifest_dir.iterdir()
            if path.is_file() and path.name.casefold().startswith(LICENSE_PREFIXES)
        ),
        key=lambda path: path.name.casefold(),
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--target", default=os.environ.get("ATTRIBUTIONS_TARGET"))
    parser.add_argument("--output", type=Path, default=os.environ.get("ATTRIBUTIONS_OUTPUT"))
    options = parser.parse_args()
    if not options.target or not options.output:
        parser.error("ATTRIBUTIONS_TARGET/--target and ATTRIBUTIONS_OUTPUT/--output are required")

    metadata = json.loads(
        subprocess.check_output(
            [
                "cargo",
                "metadata",
                "--locked",
                "--format-version=1",
                "--filter-platform",
                options.target,
            ],
            cwd=ROOT,
            text=True,
        )
    )
    packages = {package["id"]: package for package in metadata["packages"]}
    nodes = {node["id"]: node for node in metadata["resolve"]["nodes"]}
    cli_ids = [package_id for package_id, package in packages.items() if package["name"] == "steward-cli"]
    if len(cli_ids) != 1:
        raise SystemExit("workspace must contain exactly one steward-cli package")

    reachable: set[str] = set()
    queue: deque[str] = deque(cli_ids)
    while queue:
        package_id = queue.popleft()
        if package_id in reachable:
            continue
        reachable.add(package_id)
        for edge in nodes[package_id]["deps"]:
            if any(kind.get("kind") != "dev" for kind in edge["dep_kinds"]):
                queue.append(edge["pkg"])

    external = sorted(
        (packages[package_id] for package_id in reachable if packages[package_id].get("source")),
        key=lambda package: (package["name"].casefold(), package["version"], package["source"]),
    )
    if not external:
        raise SystemExit("no external runtime/build dependencies found")

    cli = packages[cli_ids[0]]
    sections = [
        "THIRD-PARTY LICENSES FOR STEWARD",
        "================================",
        "",
        f"Steward version: {cli['version']}",
        f"Rust target: {options.target}",
        "Scope: locked non-development dependency graph (normal and build edges).",
        "Generated deterministically; no absolute build-machine paths are included.",
        "",
    ]
    for package in external:
        files = license_files(package)
        if not package.get("license"):
            raise SystemExit(f"dependency has no declared license: {package['name']} {package['version']}")
        if not files:
            raise SystemExit(f"dependency has no bundled license text: {package['name']} {package['version']}")
        sections.extend(
            [
                "-" * 78,
                f"Package: {package['name']} {package['version']}",
                f"Declared license: {package['license']}",
                f"Source: {package['source']}",
            ]
        )
        for path in files:
            if not path.is_file():
                raise SystemExit(f"declared license file is missing for {package['name']}: {path.name}")
            text = path.read_text(encoding="utf-8-sig").replace("\r\n", "\n").rstrip()
            if not text:
                raise SystemExit(f"empty license file for {package['name']}: {path.name}")
            sections.extend([f"License file: {path.name}", "", text, ""])

    output = "\n".join(sections).rstrip() + "\n"
    if str(ROOT) in output or str(Path.home()) in output:
        raise SystemExit("attribution output contains an absolute machine path")
    options.output.parent.mkdir(parents=True, exist_ok=True)
    options.output.write_text(output, encoding="utf-8", newline="\n")
    print(f"wrote {options.output} for {len(external)} locked non-dev dependencies")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
