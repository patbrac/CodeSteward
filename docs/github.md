# CodeSteward on GitHub

CodeSteward runs inside GitHub Actions on pull requests, generates a compact
review-readiness report, and posts it as a **single, self-updating** PR comment.

## Quick start

Add `.github/workflows/codesteward.yml` to your repository:

```yaml
name: CodeSteward

on:
  pull_request:
    types: [opened, synchronize, edited, reopened]

permissions:
  contents: read
  pull-requests: write

jobs:
  codesteward:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: codesteward-ai/codesteward/actions/github@v0
        with:
          comment: true
```

That is the whole setup. On every matching PR event CodeSteward scans the diff
and leaves (or updates) one comment.

## Required permissions

CodeSteward needs exactly two permissions. Grant them with a top-level
`permissions:` block as shown above, or per-job.

| Permission | Level | Why |
|---|---|---|
| `contents` | `read` | Read the checked-out repository and its git history to compute the diff. |
| `pull-requests` | `write` | Create and update the CodeSteward comment on the PR. |

If `pull-requests` is only `read` (or unset), the scan still runs and prints the
report, but posting the comment fails with a `403`. See
[troubleshooting](troubleshooting.md#missing-or-insufficient-tokens).

## Why `fetch-depth: 0` is required

By default `actions/checkout` performs a **shallow clone** (`fetch-depth: 1`),
which fetches only the tip commit. CodeSteward needs to find the merge base
between the PR head and its base branch, which requires full history. Set:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0
```

Without it, the scan fails with an actionable error explaining that the base ref
could not be resolved in a shallow clone. See
[troubleshooting](troubleshooting.md#shallow-clone-failures).

## How the comment is detected and updated

CodeSteward embeds a hidden HTML marker at the top of every report:

```html
<!-- codesteward-report -->
```

On each run the GitHub client:

1. Lists the PR's issue comments (paginated, 100 per page).
2. Finds the **first** comment whose body contains the marker.
3. If found, **PATCHes** that comment with the new report.
4. If not found, **POSTs** a new comment.

Because it matches on the marker, CodeSteward maintains exactly one comment per
PR and updates it in place on every push — it never posts duplicates. Do not
delete or edit the marker line; if you do, CodeSteward can no longer find its
comment and will create a new one.

## Base and head refs

Inside GitHub Actions, CodeSteward reads the PR event payload
(`GITHUB_EVENT_PATH`) to determine the PR number, base ref, head ref, and PR
description (body). It also honors `GITHUB_BASE_REF` for base resolution. You
normally do not need to pass `--base` or `--head` yourself.

The provider always passes the real PR body as the description (even when
empty), so description rules are evaluated on GitHub. This differs from a bare
local run, where the description is not evaluated unless you pass
`--description` or `--description-file`.

## Dry-run mode

Use dry-run to compute and preview the report without posting anything. The
intended action (create vs. update) is logged to stderr and no API write is
made.

```yaml
      - uses: codesteward-ai/codesteward/actions/github@v0
        with:
          comment: true
          dry-run: true
```

Or directly on the CLI:

```bash
codesteward scan --comment --dry-run
```

This is the safe way to try CodeSteward on a repository before granting or
exercising `pull-requests: write`.

## Output-only mode

To generate the report but never post a comment, simply omit `comment: true`
(or set it to `false`). The report is written to stdout, or to a file with
`--output`:

```bash
codesteward scan --format markdown --output codesteward-report.md
codesteward scan --format json --output codesteward-report.json
```

This is useful for uploading the report as a build artifact or feeding the JSON
to other tooling.

## Environment variables CodeSteward reads on GitHub

| Variable | Purpose |
|---|---|
| `GITHUB_ACTIONS` | Detects that it is running inside GitHub Actions. |
| `GITHUB_REPOSITORY` | `owner/repo` slug used for the comments API. |
| `GITHUB_EVENT_PATH` | Path to the event JSON; parsed for PR number, body, base/head. |
| `GITHUB_API_URL` | API base URL (supports GitHub Enterprise). |
| `GITHUB_BASE_REF` | Base branch name, used during ref resolution. |
| `GITHUB_TOKEN` | Token used to authenticate comment posting. |

The default `GITHUB_TOKEN` provided by Actions is sufficient as long as
`pull-requests: write` is granted.

## Exit codes

`codesteward scan` **never** exits non-zero because of report content — it is a
comment-only tool and does not block merges. It exits non-zero only on genuine
errors:

| Exit code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Runtime/scan error (for example, unresolved base ref, or `--comment` requested with no detected provider). |
| `2` | Usage error (unknown command or flag, bad value). |

If you run `codesteward scan --comment` outside a detected CI provider,
CodeSteward exits `1` and suggests `--dry-run` or running inside CI.

## See also

- [Getting started](getting-started.md)
- [GitLab setup](gitlab.md)
- [Troubleshooting](troubleshooting.md)
