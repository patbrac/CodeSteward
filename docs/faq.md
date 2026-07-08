# FAQ

### Is this AI?

No. CodeSteward is fully **deterministic** and uses **no AI or LLMs** in v0. It
runs a fixed set of rules over your git diff, CODEOWNERS, and configuration.
Identical inputs always produce byte-identical output. There is no model, no
network inference, and no code understanding beyond path-based rules.

### Does it block merges?

No. CodeSteward is **comment-only**. It never blocks, fails required checks,
labels PRs, or gates merges. The `codesteward scan` command never exits non-zero
because of report content — a non-zero exit only ever means a real error (bad
usage, unresolved base ref, failed comment post). Every report ends with the
reminder: _Comment-only mode. CodeSteward is not blocking this PR._

### Where is the score?

CodeSteward computes an internal numeric readiness score (0–100), but it is
**hidden from the Markdown comment by default** — the comment shows a
plain-language status and review burden instead, to keep the tone helpful rather
than judgmental. You can see the score in two ways:

- Add `--show-score` to a local scan to print it in the Markdown.
- Use `--format json`; the JSON report always includes the `score` field.

### How do I hide or see the details?

The underlying findings live in a collapsed `<details>` block titled "Why
CodeSteward flagged this." Click it to expand the full list. The top of the
comment always shows the status, review burden, ownership, tests, and up to five
action items; the collapsed section holds the complete detail.

### Does it label PRs or assign reviewers?

No. Auto-labeling and auto-reviewer assignment are explicit **non-goals** for
v0. CodeSteward will not add labels, request reviewers, or moderate
contributors. It only leaves one informational comment.

### Does it work on both public and private repositories?

Yes — CodeSteward runs anywhere it can read the git history and reach the
provider API. The free open-source CLI and CI integrations work on both public
and private repos on GitHub and GitLab, including self-managed instances (via
`GITHUB_API_URL` / `CI_API_V4_URL`). It only needs the right token and
permissions to post the comment.

### How does it decide ownership?

From your `CODEOWNERS` file. It classifies each match as **specific**, **broad**,
or **fallback**, and treats catch-all (fallback-only) coverage as **partial**
ownership. See [CODEOWNERS support](codeowners.md).

### Why does it say my description is empty when I ran it locally?

In a bare local run, the PR description is **not evaluated** unless you provide
one with `--description` or `--description-file` — this avoids noisy description
warnings during local use. In CI, the GitHub/GitLab provider always passes the
real PR/MR body (even if empty), so description rules are evaluated there.

### Why is it posting a new comment every time?

It matches its previous comment by the hidden marker
`<!-- codesteward-report -->`. If that line was deleted or edited, CodeSteward
can no longer find its comment and creates a new one. See
[troubleshooting](troubleshooting.md#comment-not-updating-a-duplicate-appears).

### Is the output stable enough to diff or snapshot?

Yes. Determinism is a hard requirement: sorted iteration everywhere order
matters, and no timestamps, random values, or absolute paths in any report. You
can safely snapshot-test the Markdown or JSON output.

### What does CodeSteward not do?

No LLM review, no AI-generated-code detection, no security scanning, no deep
semantic analysis, no blocking, no auto-labeling, no auto-reviewer requests, no
contributor moderation. It is a maintainer-time protection tool, not a policy
engine.

### See also

- [Getting started](getting-started.md)
- [Configuration reference](config.md)
- [CODEOWNERS support](codeowners.md)
- [Troubleshooting](troubleshooting.md)
