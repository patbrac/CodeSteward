#!/usr/bin/env python3
"""Verify DCO sign-offs for every non-merge commit in a pull request.

The checker is deliberately local-only: it reads commit objects from the checked-out
repository and does not call the GitHub API.  The workflow supplies the immutable
pull-request base and head object IDs.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path


SIGN_OFF = re.compile(r"^\s*(.*?)\s*<([^<>]+)>\s*$")
MAX_COMMITS = 10_000
MAX_COMMIT_OBJECT_BYTES = 1_048_576
GIT_TIMEOUT_SECONDS = 30

# These exact identities are GitHub-operated automation accounts.  Keeping the
# allow-list narrow avoids turning a generic `[bot]` suffix into a DCO bypass.
AUTOMATION_AUTHORS = frozenset(
    {
        ("dependabot[bot]", "49699333+dependabot[bot]@users.noreply.github.com"),
        ("github-actions[bot]", "41898282+github-actions[bot]@users.noreply.github.com"),
    }
)


class DcoError(RuntimeError):
    """Raised when the repository or requested commit range cannot be inspected."""


def terminal_safe(value: str, limit: int = 500) -> str:
    """Bound diagnostics and replace terminal/control sequences from Git metadata."""

    sanitized = "".join(character if character.isprintable() else "?" for character in value)
    if len(sanitized) > limit:
        return sanitized[:limit] + "..."
    return sanitized


@dataclass(frozen=True)
class Identity:
    name: str
    email: str

    @property
    def normalized(self) -> tuple[str, str]:
        return (" ".join(self.name.split()).casefold(), self.email.strip().casefold())


@dataclass(frozen=True)
class Commit:
    object_id: str
    parents: tuple[str, ...]
    author: Identity
    committer: Identity
    message: str


def git(repository: Path, *arguments: str, stdin: str | None = None) -> str:
    """Run Git without a shell and decode hostile metadata losslessly enough to report."""

    try:
        result = subprocess.run(
            ["git", *arguments],
            cwd=repository,
            input=None if stdin is None else stdin.encode("utf-8", errors="surrogatepass"),
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=GIT_TIMEOUT_SECONDS,
            check=False,
        )
    except subprocess.TimeoutExpired as error:
        command = terminal_safe("git " + " ".join(arguments), limit=300)
        raise DcoError(f"{command} exceeded {GIT_TIMEOUT_SECONDS} seconds") from error
    stdout = result.stdout.decode("utf-8", errors="replace")
    if result.returncode != 0:
        stderr = result.stderr.decode("utf-8", errors="replace").strip()
        command = terminal_safe("git " + " ".join(arguments), limit=300)
        detail = terminal_safe(stderr or "unknown Git error")
        raise DcoError(f"{command} failed: {detail}")
    return stdout


def resolve_commit(repository: Path, revision: str) -> str:
    if not revision or "\0" in revision or "\n" in revision or "\r" in revision:
        raise DcoError("a commit revision is empty or contains a control character")
    return git(
        repository,
        "rev-parse",
        "--verify",
        "--end-of-options",
        f"{revision}^{{commit}}",
    ).strip()


def pull_request_commits(repository: Path, base: str, head: str) -> list[str]:
    base_id = resolve_commit(repository, base)
    head_id = resolve_commit(repository, head)
    merge_base = git(repository, "merge-base", base_id, head_id).strip()
    if not merge_base:
        raise DcoError("pull-request base and head have no merge base")
    output = git(
        repository,
        "rev-list",
        "--reverse",
        "--topo-order",
        f"--max-count={MAX_COMMITS + 1}",
        f"{merge_base}..{head_id}",
    )
    commits = [line for line in output.splitlines() if line]
    if len(commits) > MAX_COMMITS:
        raise DcoError(
            f"pull request contains more than {MAX_COMMITS} commits; split it into reviewable ranges"
        )
    return commits


def read_commit(repository: Path, object_id: str) -> Commit:
    try:
        object_size = int(git(repository, "cat-file", "-s", object_id).strip())
    except ValueError as error:
        raise DcoError(f"could not read the size of commit {object_id}") from error
    if object_size > MAX_COMMIT_OBJECT_BYTES:
        raise DcoError(
            f"commit {object_id} exceeds the {MAX_COMMIT_OBJECT_BYTES}-byte metadata limit"
        )
    fields = git(repository, "show", "-s", "--format=%P%x00%an%x00%ae%x00%cn%x00%ce", object_id)
    values = fields.rstrip("\r\n").split("\0")
    if len(values) != 5:
        raise DcoError(f"could not parse identities for commit {object_id}")
    parents, author_name, author_email, committer_name, committer_email = values
    message = git(repository, "show", "-s", "--format=%B", object_id)
    return Commit(
        object_id=object_id,
        parents=tuple(parents.split()),
        author=Identity(author_name, author_email),
        committer=Identity(committer_name, committer_email),
        message=message,
    )


def signed_off_identities(repository: Path, message: str) -> set[tuple[str, str]]:
    trailers = git(repository, "interpret-trailers", "--parse", stdin=message)
    identities: set[tuple[str, str]] = set()
    for line in trailers.splitlines():
        key, separator, value = line.partition(":")
        if not separator or key.strip().casefold() != "signed-off-by":
            continue
        match = SIGN_OFF.fullmatch(value)
        if match:
            identities.add(Identity(match.group(1), match.group(2)).normalized)
    return identities


def failure_reason(repository: Path, commit: Commit) -> str | None:
    # Git hosting creates merge commits mechanically.  The commits carrying the
    # contribution remain in the range and are checked individually.
    if len(commit.parents) > 1:
        return None
    if commit.author.normalized in AUTOMATION_AUTHORS:
        return None
    signers = signed_off_identities(repository, commit.message)
    associated = {commit.author.normalized, commit.committer.normalized}
    if signers & associated:
        return None
    if not signers:
        return "missing a Signed-off-by trailer"
    return "has no Signed-off-by trailer matching its author or committer identity"


def verify_range(repository: Path, base: str, head: str) -> list[str]:
    failures: list[str] = []
    for object_id in pull_request_commits(repository, base, head):
        commit = read_commit(repository, object_id)
        reason = failure_reason(repository, commit)
        if reason:
            subject = commit.message.splitlines()[0] if commit.message.splitlines() else "<empty subject>"
            failures.append(f"{object_id[:12]} {reason}: {terminal_safe(subject, limit=120)}")
    return failures


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repository", type=Path, default=Path.cwd())
    parser.add_argument("--base", required=True, help="pull-request base commit object ID")
    parser.add_argument("--head", required=True, help="pull-request head commit object ID")
    arguments = parser.parse_args()

    try:
        failures = verify_range(arguments.repository.resolve(), arguments.base, arguments.head)
    except (DcoError, OSError) as error:
        print(f"DCO check could not inspect the pull request: {error}", file=sys.stderr)
        return 2
    if failures:
        print("DCO sign-off check failed:", file=sys.stderr)
        for failure in failures:
            print(f"- {failure}", file=sys.stderr)
        print(
            "Add a matching Signed-off-by trailer with `git commit --signoff`; "
            "see CONTRIBUTING.md.",
            file=sys.stderr,
        )
        return 1
    print("DCO sign-off check passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
