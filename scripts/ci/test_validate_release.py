#!/usr/bin/env python3
"""Release-tag ancestry regression tests."""

from __future__ import annotations

import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "release"))
from validate_release import validate_git_release  # noqa: E402


def git(repository: Path, *arguments: str) -> None:
    subprocess.run(["git", *arguments], cwd=repository, check=True, capture_output=True)


class ReleaseTagTests(unittest.TestCase):
    def repository(self) -> tuple[tempfile.TemporaryDirectory[str], Path]:
        temporary = tempfile.TemporaryDirectory()
        repository = Path(temporary.name)
        git(repository, "init", "--quiet")
        git(repository, "config", "user.name", "Release Test")
        git(repository, "config", "user.email", "release-test@example.invalid")
        (repository / "README.md").write_text("initial\n", encoding="utf-8")
        git(repository, "add", "README.md")
        git(repository, "commit", "--quiet", "-m", "initial")
        git(repository, "update-ref", "refs/remotes/origin/main", "HEAD")
        return temporary, repository

    def test_annotated_tag_reachable_from_origin_main_is_accepted(self) -> None:
        temporary, repository = self.repository()
        with temporary:
            git(repository, "tag", "-a", "v0.1.0", "-m", "release")
            validate_git_release("v0.1.0", repository)

    def test_unmerged_release_commit_is_rejected(self) -> None:
        temporary, repository = self.repository()
        with temporary:
            (repository / "README.md").write_text("unmerged\n", encoding="utf-8")
            git(repository, "add", "README.md")
            git(repository, "commit", "--quiet", "-m", "unmerged")
            git(repository, "tag", "-a", "v0.1.0", "-m", "release")
            with self.assertRaisesRegex(ValueError, "not reachable from origin/main"):
                validate_git_release("v0.1.0", repository)

    def test_lightweight_tag_is_rejected(self) -> None:
        temporary, repository = self.repository()
        with temporary:
            git(repository, "tag", "v0.1.0")
            with self.assertRaisesRegex(ValueError, "annotated"):
                validate_git_release("v0.1.0", repository)


if __name__ == "__main__":
    unittest.main()
