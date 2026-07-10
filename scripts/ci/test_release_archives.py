#!/usr/bin/env python3
"""Regression tests for release archive attribution enforcement."""

from __future__ import annotations

import sys
import tempfile
import unittest
import zipfile
from pathlib import Path


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "release"))
from verify_release_assets import verify_archive  # noqa: E402


class ReleaseArchiveTests(unittest.TestCase):
    def archive(self, directory: Path, attributions: bytes) -> Path:
        path = directory / "steward-test-x86_64-pc-windows-msvc.zip"
        with zipfile.ZipFile(path, "w") as archive:
            for name, content in {
                "steward-test/steward.exe": b"binary",
                "steward-test/LICENSE": b"project license",
                "steward-test/NOTICE": b"notice",
                "steward-test/README.md": b"readme",
                "steward-test/THIRD_PARTY_LICENSES.txt": attributions,
            }.items():
                archive.writestr(name, content)
        return path

    def test_complete_attribution_is_accepted(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            archive = self.archive(
                Path(temporary),
                b"THIRD-PARTY LICENSES FOR STEWARD\nPackage: example 1.0.0\nDeclared license: MIT\n",
            )
            verify_archive(archive, "steward.exe")

    def test_empty_attribution_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            archive = self.archive(Path(temporary), b"")
            with self.assertRaises(SystemExit):
                verify_archive(archive, "steward.exe")


if __name__ == "__main__":
    unittest.main()
