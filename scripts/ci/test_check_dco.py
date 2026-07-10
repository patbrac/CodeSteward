#!/usr/bin/env python3
"""Regression tests for the local DCO commit-range checker."""

from __future__ import annotations

import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


sys.path.insert(0, str(Path(__file__).resolve().parent))
import check_dco  # noqa: E402
from check_dco import DcoError, verify_range  # noqa: E402


class DcoCheckTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.repository = Path(self.temporary.name)
        self.git("init", "--initial-branch=main")
        self.git("config", "user.name", "Base Author")
        self.git("config", "user.email", "base@example.com")
        self.commit_index = 0
        self.commit("base", "Signed-off-by: Base Author <base@example.com>")
        self.base = self.git("rev-parse", "HEAD").strip()

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def git(self, *arguments: str) -> str:
        result = subprocess.run(
            ["git", *arguments],
            cwd=self.repository,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
        )
        if result.returncode != 0:
            self.fail(f"git {' '.join(arguments)} failed: {result.stderr}")
        return result.stdout

    def commit(
        self,
        subject: str,
        trailer: str | None,
        *,
        name: str = "Contributor",
        email: str = "contributor@example.com",
    ) -> str:
        self.git("config", "user.name", name)
        self.git("config", "user.email", email)
        self.commit_index += 1
        marker = self.repository / f"content-{self.commit_index}.txt"
        marker.write_text(subject + "\n", encoding="utf-8")
        self.git("add", marker.name)
        arguments = ["commit", "-m", subject]
        if trailer is not None:
            arguments.extend(["-m", trailer])
        self.git(*arguments)
        return self.git("rev-parse", "HEAD").strip()

    def test_matching_author_signoff_passes(self) -> None:
        head = self.commit(
            "signed contribution",
            "Signed-off-by: Contributor <contributor@example.com>",
        )
        self.assertEqual(verify_range(self.repository, self.base, head), [])

    def test_missing_signoff_fails(self) -> None:
        head = self.commit("unsigned contribution", None)
        failures = verify_range(self.repository, self.base, head)
        self.assertEqual(len(failures), 1)
        self.assertIn("missing a Signed-off-by trailer", failures[0])

    def test_unrelated_signoff_fails(self) -> None:
        head = self.commit(
            "wrong signer",
            "Signed-off-by: Someone Else <else@example.com>",
        )
        failures = verify_range(self.repository, self.base, head)
        self.assertEqual(len(failures), 1)
        self.assertIn("matching its author or committer", failures[0])

    def test_hostile_subject_control_bytes_are_sanitized(self) -> None:
        head = self.commit("unsafe \x1b[31m subject", None)
        failures = verify_range(self.repository, self.base, head)
        self.assertEqual(len(failures), 1)
        self.assertNotIn("\x1b", failures[0])
        self.assertIn("unsafe ?[31m subject", failures[0])

    def test_body_text_is_not_accepted_as_a_trailer(self) -> None:
        self.git("config", "user.name", "Contributor")
        self.git("config", "user.email", "contributor@example.com")
        (self.repository / "body-trailer.txt").write_text("body trailer\n", encoding="utf-8")
        self.git("add", "body-trailer.txt")
        self.git(
            "commit",
            "-m",
            "body-only signoff",
            "-m",
            "Signed-off-by: Contributor <contributor@example.com>\n\nThis text follows the apparent trailer.",
        )
        head = self.git("rev-parse", "HEAD").strip()
        self.assertEqual(len(verify_range(self.repository, self.base, head)), 1)

    def test_merge_commit_is_skipped_but_its_contribution_is_checked(self) -> None:
        self.git("checkout", "-b", "topic")
        self.commit("topic", "Signed-off-by: Contributor <contributor@example.com>")
        self.git("checkout", "main")
        self.commit("main", "Signed-off-by: Contributor <contributor@example.com>")
        self.git("merge", "--no-ff", "topic", "-m", "merge without signoff")
        head = self.git("rev-parse", "HEAD").strip()
        self.assertEqual(verify_range(self.repository, self.base, head), [])

    def test_exact_dependabot_identity_is_exempt(self) -> None:
        head = self.commit(
            "automated update",
            None,
            name="dependabot[bot]",
            email="49699333+dependabot[bot]@users.noreply.github.com",
        )
        self.assertEqual(verify_range(self.repository, self.base, head), [])

    def test_bot_like_name_with_unapproved_email_is_not_exempt(self) -> None:
        head = self.commit(
            "spoofed automation",
            None,
            name="dependabot[bot]",
            email="attacker@example.com",
        )
        self.assertEqual(len(verify_range(self.repository, self.base, head)), 1)

    def test_commit_count_limit_fails_closed(self) -> None:
        self.commit("first", "Signed-off-by: Contributor <contributor@example.com>")
        head = self.commit("second", "Signed-off-by: Contributor <contributor@example.com>")
        with patch.object(check_dco, "MAX_COMMITS", 1):
            with self.assertRaisesRegex(DcoError, "more than 1 commits"):
                verify_range(self.repository, self.base, head)

    def test_commit_metadata_size_limit_fails_closed(self) -> None:
        head = self.commit("bounded", "Signed-off-by: Contributor <contributor@example.com>")
        with patch.object(check_dco, "MAX_COMMIT_OBJECT_BYTES", 1):
            with self.assertRaisesRegex(DcoError, "metadata limit"):
                verify_range(self.repository, self.base, head)


if __name__ == "__main__":
    unittest.main()
