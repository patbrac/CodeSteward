#!/usr/bin/env python3
"""Validate a release tag against steward-cli and emit workflow outputs."""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SEMVER_TAG = re.compile(
    r"^v(?P<version>(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)"
    r"(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?)$"
)


def git_output(repository: Path, *arguments: str) -> str:
    result = subprocess.run(
        ["git", *arguments],
        cwd=repository,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise ValueError(result.stderr.strip() or f"git {' '.join(arguments)} failed")
    return result.stdout.strip()


def validate_git_release(tag: str, repository: Path = ROOT) -> None:
    tag_ref = f"refs/tags/{tag}"
    if git_output(repository, "cat-file", "-t", tag_ref) != "tag":
        raise ValueError(f"release tag {tag} must be an annotated tag")
    tag_commit = git_output(repository, "rev-parse", "--verify", f"{tag_ref}^{{commit}}")
    head_commit = git_output(repository, "rev-parse", "--verify", "HEAD^{commit}")
    if tag_commit != head_commit:
        raise ValueError(f"release tag {tag} does not identify the checked-out commit")
    main_commit = git_output(
        repository,
        "rev-parse",
        "--verify",
        "refs/remotes/origin/main^{commit}",
    )
    ancestry = subprocess.run(
        ["git", "merge-base", "--is-ancestor", tag_commit, main_commit],
        cwd=repository,
        capture_output=True,
        check=False,
    )
    if ancestry.returncode == 1:
        raise ValueError(f"release commit {tag_commit} is not reachable from origin/main")
    if ancestry.returncode != 0:
        raise ValueError("git could not verify release ancestry against origin/main")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--tag", default=os.environ.get("RELEASE_TAG"))
    parser.add_argument("--github-output", type=Path, default=os.environ.get("GITHUB_OUTPUT"))
    options = parser.parse_args()
    if not options.tag:
        parser.error("--tag or RELEASE_TAG is required")
    match = SEMVER_TAG.fullmatch(options.tag)
    if not match:
        parser.error("release tag must be vMAJOR.MINOR.PATCH with an optional SemVer prerelease")
    version = match.group("version")
    try:
        validate_git_release(options.tag)
    except ValueError as error:
        raise SystemExit(str(error)) from error

    metadata = json.loads(
        subprocess.check_output(
            ["cargo", "metadata", "--locked", "--format-version=1", "--no-deps"],
            cwd=ROOT,
            text=True,
        )
    )
    packages = [package for package in metadata["packages"] if package["name"] == "steward-cli"]
    if len(packages) != 1:
        raise SystemExit("workspace must contain exactly one steward-cli package")
    if packages[0]["version"] != version:
        raise SystemExit(
            f"tag version {version} does not match steward-cli version {packages[0]['version']}"
        )

    matrix_path = ROOT / "scripts" / "release" / "targets.json"
    matrix = json.loads(matrix_path.read_text(encoding="utf-8"))
    if len(matrix.get("include", [])) < 3:
        raise SystemExit("release target matrix must contain all three native OS families")

    if options.github_output:
        with options.github_output.open("a", encoding="utf-8", newline="\n") as output:
            output.write(f"version={version}\n")
            output.write(f"matrix={json.dumps(matrix, separators=(',', ':'))}\n")
    print(f"release {options.tag} matches steward-cli {version}; {len(matrix['include'])} native targets")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
