#!/usr/bin/env python3
"""Check public repository and Cargo contracts without internal planning files."""

from __future__ import annotations

import json
import re
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
REQUIRED = (
    "Cargo.toml",
    "Cargo.lock",
    "LICENSE",
    "NOTICE",
    "README.md",
    "CODE_OF_CONDUCT.md",
    "CONTRIBUTING.md",
    "GOVERNANCE.md",
    "SECURITY.md",
    "SUPPORT.md",
    "TRADEMARKS.md",
    "deny.toml",
    ".github/workflows/dco.yml",
    "docs/adr",
    "docs/platform-support.md",
    "docs/release",
    "schemas",
    "scripts/ci/check_dco.py",
)
PIN = re.compile(r"^\s*uses:\s*([^\s@]+)@([0-9a-f]{40})(?:\s*#.*)?$")
WORKSPACE_PACKAGES = {"steward-cli", "steward-engine", "steward-policy", "steward-report"}
ALLOWED_INTERNAL_EDGES = {
    ("steward-cli", "steward-engine"),
    ("steward-engine", "steward-policy"),
    ("steward-engine", "steward-report"),
}


def validate_workspace_edges(package_names: set[str], edges: set[tuple[str, str]]) -> list[str]:
    errors: list[str] = []
    if package_names != WORKSPACE_PACKAGES:
        errors.append(
            "workspace package set differs from the approved Phase 1 boundary: "
            f"missing={sorted(WORKSPACE_PACKAGES - package_names)}, "
            f"unexpected={sorted(package_names - WORKSPACE_PACKAGES)}"
        )
    unexpected = edges - ALLOWED_INTERNAL_EDGES
    missing = ALLOWED_INTERNAL_EDGES - edges
    if unexpected:
        errors.append(f"forbidden internal workspace dependency edge(s): {sorted(unexpected)}")
    if missing:
        errors.append(f"required Phase 1 internal dependency edge(s) missing: {sorted(missing)}")
    return errors


def inside_root(value: str) -> bool:
    path = Path(value).resolve()
    try:
        path.relative_to(ROOT)
        return True
    except ValueError:
        return False


def main() -> int:
    errors: list[str] = []
    for relative in REQUIRED:
        if not (ROOT / relative).exists():
            errors.append(f"missing public repository contract: {relative}")

    workflow_root = ROOT / ".github" / "workflows"
    workflows = sorted(
        [*workflow_root.glob("*.yml"), *workflow_root.glob("*.yaml")],
        key=lambda path: path.as_posix(),
    )
    if not workflows:
        errors.append("no GitHub Actions workflows found")
    for workflow in workflows:
        for line_number, line in enumerate(workflow.read_text(encoding="utf-8").splitlines(), 1):
            stripped = line.strip()
            if stripped.startswith("uses:") or stripped.startswith("- uses:"):
                normalized = line.replace("- uses:", "uses:", 1)
                match = PIN.match(normalized)
                if not match and "uses: ./" not in normalized:
                    errors.append(f"{workflow.relative_to(ROOT)}:{line_number}: action is not pinned to a full commit SHA")
        text = workflow.read_text(encoding="utf-8")
        for forbidden in ("PRODUCT_PLAN_TASKS", "CODE_STEWARD_BUSINESS", "evidence/", "docs/validation", "docs/naming"):
            if forbidden in text:
                errors.append(f"{workflow.relative_to(ROOT)} references non-public path token {forbidden!r}")

    if (ROOT / "Cargo.toml").exists() and (ROOT / "Cargo.lock").exists():
        result = subprocess.run(
            ["cargo", "metadata", "--locked", "--format-version=1", "--no-deps"],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )
        if result.returncode != 0:
            errors.append(f"cargo metadata --locked failed: {result.stderr.strip()}")
        else:
            metadata = json.loads(result.stdout)
            workspace_ids = set(metadata["workspace_members"])
            members = [package for package in metadata["packages"] if package["id"] in workspace_ids]
            package_names = {package["name"] for package in members}
            internal_edges = {
                (package["name"], dependency["name"])
                for package in members
                for dependency in package.get("dependencies", [])
                if dependency["name"] in package_names
            }
            errors.extend(validate_workspace_edges(package_names, internal_edges))
            cli = [package for package in members if package["name"] == "steward-cli"]
            if len(cli) != 1:
                errors.append("workspace must contain exactly one steward-cli package")
            elif not any("bin" in target["kind"] and target["name"] == "steward" for target in cli[0]["targets"]):
                errors.append("steward-cli must expose a binary target named steward")
            for package in members:
                manifest = package["manifest_path"]
                if not inside_root(manifest):
                    errors.append(f"workspace manifest escapes repository root: {manifest}")
                if package.get("license") != "Apache-2.0":
                    errors.append(f"{package['name']} must declare Apache-2.0, got {package.get('license')!r}")
                if not package.get("rust_version"):
                    errors.append(f"{package['name']} must declare rust-version/MSRV")
                for dependency in package.get("dependencies", []):
                    path = dependency.get("path")
                    if path and not inside_root(path):
                        errors.append(f"{package['name']} has a path dependency outside the repository: {path}")

    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    print(f"public contracts passed; checked {len(workflows)} workflows and Cargo workspace metadata")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
