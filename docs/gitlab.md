# CodeSteward on GitLab

CodeSteward runs inside GitLab CI on merge request pipelines, generates a
compact review-readiness report, and posts it as a **single, self-updating** MR
note.

## Quick start

Include the CodeSteward CI template and run it on merge request pipelines. Add
this to your `.gitlab-ci.yml`:

```yaml
codesteward:
  image: ghcr.io/codesteward-ai/codesteward:v0.1.0
  variables:
    GIT_DEPTH: 0
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
  script:
    - codesteward scan --comment
```

Or include the maintained template directly:

```yaml
include:
  - remote: 'https://raw.githubusercontent.com/codesteward-ai/codesteward/v0.1.0/gitlab/codesteward.gitlab-ci.yml'
```

## Token setup

Posting an MR note requires an API token. **`CI_JOB_TOKEN` alone usually cannot
post notes**: the job token has a narrow, project-scoped permission set and, on
most GitLab instances and plans, is not authorized to write MR notes through the
Notes API. Relying on it typically results in a `401` or `403`.

The reliable approach is a **project (or group) access token** with the `api`
scope:

1. In your project, go to **Settings → Access Tokens**.
2. Create a token with the **`api`** scope and at least the **Reporter** role
   (Developer is fine too).
3. Store it as a **masked** CI/CD variable named **`CODESTEWARD_GITLAB_TOKEN`**
   (Settings → CI/CD → Variables).

CodeSteward selects the token in this order:

1. `CODESTEWARD_GITLAB_TOKEN` (recommended — a project access token with `api`
   scope),
2. `CI_JOB_TOKEN` (fallback; frequently insufficient for posting notes).

If both are missing or unauthorized, the scan still runs and prints the report,
but posting fails with an actionable `401`/`403` error mentioning token
permissions. See
[troubleshooting](troubleshooting.md#missing-or-insufficient-tokens).

## Why `GIT_DEPTH: 0` is required

GitLab CI clones shallowly by default (`GIT_DEPTH` defaults to `20` or `50`).
CodeSteward needs full history to find the merge base between the MR source and
target branches. Set `GIT_DEPTH: 0` to fetch the entire history:

```yaml
variables:
  GIT_DEPTH: 0
```

Without it, the scan can fail with a base-ref-not-found error in a shallow
clone. See [troubleshooting](troubleshooting.md#shallow-clone-failures).

## How the note is detected and updated

CodeSteward embeds a hidden HTML marker at the top of every report:

```html
<!-- codesteward-report -->
```

On each run the GitLab client:

1. Lists the MR's notes via the Notes API.
2. Finds the **first** note whose body contains the marker.
3. If found, **PUTs** (updates) that note with the new report.
4. If not found, **POSTs** a new note.

This keeps exactly one CodeSteward note per MR, updated in place on every push,
with no duplicates. Do not delete or edit the marker line, or CodeSteward will
lose track of its note and create a new one.

## Base and head refs

Inside GitLab CI, CodeSteward reads the pipeline variables to determine the
project, MR IID, base/target ref, and MR description. It honors
`CI_MERGE_REQUEST_TARGET_BRANCH_NAME` during base resolution, so you normally do
not need to pass `--base` or `--head`.

The provider always passes the real MR description (even when empty), so
description rules are evaluated on GitLab.

## Dry-run mode

Preview the report and the intended action without posting. The intended action
is logged to stderr; no API write occurs.

```yaml
codesteward:
  script:
    - codesteward scan --comment --dry-run
```

Use this to try CodeSteward before provisioning `CODESTEWARD_GITLAB_TOKEN`.

## Output-only mode

To generate the report but never post a note, omit `--comment`. Write it to
stdout or a file:

```bash
codesteward scan --format markdown --output codesteward-report.md
codesteward scan --format json --output codesteward-report.json
```

Combine with GitLab CI `artifacts:` to keep the report as a downloadable
artifact.

## Environment variables CodeSteward reads on GitLab

| Variable | Purpose |
|---|---|
| `GITLAB_CI` | Detects that it is running inside GitLab CI. |
| `CI_PROJECT_ID` | Numeric project ID used for the Notes API. |
| `CI_MERGE_REQUEST_IID` | MR internal ID (the per-project MR number). |
| `CI_API_V4_URL` | GitLab API v4 base URL (supports self-managed instances). |
| `CI_MERGE_REQUEST_TARGET_BRANCH_NAME` | Target branch, used during ref resolution. |
| `CI_MERGE_REQUEST_DESCRIPTION` | MR description, evaluated by description rules. |
| `CODESTEWARD_GITLAB_TOKEN` | Preferred token (project access token, `api` scope). |
| `CI_JOB_TOKEN` | Fallback token (usually insufficient to post notes). |

## Exit codes

`codesteward scan` **never** exits non-zero because of report content — it does
not block merges. It exits non-zero only on real errors:

| Exit code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Runtime/scan error (unresolved base ref, `--comment` with no detected provider, failed note post). |
| `2` | Usage error (unknown command or flag, bad value). |

## See also

- [Getting started](getting-started.md)
- [GitHub setup](github.md)
- [Troubleshooting](troubleshooting.md)
