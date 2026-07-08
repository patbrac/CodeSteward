# Troubleshooting

Common problems running CodeSteward locally and in CI, with the exact fixes.

## Shallow clone failures

**Symptom.** The scan fails resolving the base ref, with an error mentioning a
shallow clone or a ref that cannot be found — for example, a merge base between
your branch and the base ref cannot be computed.

**Cause.** CodeSteward shells out to `git` and needs full history to find the
merge base between head and base. Most CI checkouts are shallow by default.

**Fixes.**

- **GitHub Actions** — set full-depth checkout:

  ```yaml
  - uses: actions/checkout@v4
    with:
      fetch-depth: 0
  ```

- **GitLab CI** — set `GIT_DEPTH: 0`:

  ```yaml
  variables:
    GIT_DEPTH: 0
  ```

- **Local clone** — deepen the existing clone:

  ```bash
  git fetch --unshallow
  ```

  If the repository is already complete, `git fetch --unshallow` reports
  nothing to do, which is fine.

## Missing or insufficient tokens

### GitHub: 403 when posting the comment

**Symptom.** The scan runs and prints the report, but posting the PR comment
fails with `403`.

**Cause.** The workflow lacks `pull-requests: write`.

**Fix.** Grant both required permissions:

```yaml
permissions:
  contents: read
  pull-requests: write
```

A `401` usually means `GITHUB_TOKEN` is missing or malformed; ensure you are not
overriding it with an empty value.

### GitLab: 401 or 403 when posting the note

**Symptom.** The scan runs and prints the report, but posting the MR note fails
with `401` or `403`.

**Cause.** No usable token. `CI_JOB_TOKEN` alone usually cannot post MR notes.

**Fix.** Create a **project access token** with the `api` scope and store it as a
masked CI/CD variable named `CODESTEWARD_GITLAB_TOKEN`. CodeSteward prefers
`CODESTEWARD_GITLAB_TOKEN`, then falls back to `CI_JOB_TOKEN`. See
[GitLab setup](gitlab.md#token-setup).

## Missing CODEOWNERS

**Symptom.** Ownership shows as `Missing` (when you changed production files) or
`Not evaluated` (when you did not), and you expected owners to be recognized.

**Cause.** No CODEOWNERS file was found in the dialect's search locations, or it
lives somewhere CodeSteward does not look.

**Fixes.**

- Place CODEOWNERS in a supported location for your dialect (see
  [CODEOWNERS support](codeowners.md#discovery-locations)).
- Set `ownership.dialect` explicitly (`github`, `gitlab`, or `auto`) if
  auto-detection picks the wrong location.
- Confirm the file parses cleanly:

  ```bash
  codesteward codeowners validate
  ```

## No config file warning

**Symptom.** stderr shows `warning: no config file found; using built-in
defaults`.

**Cause.** Neither `.codesteward.yaml` nor `.codesteward.yml` exists at the
repo root, and no `--config` was passed. **This is not an error** — CodeSteward
runs with safe defaults.

**Fix (optional).** Add `.codesteward.yaml` at the repository root to customize
behavior, then validate it:

```bash
codesteward config validate
```

If you passed `--config <path>` and the file is missing or unreadable, that is a
fatal error (not a warning) — check the path.

## Comment not updating (a duplicate appears)

**Symptom.** A new CodeSteward comment/note appears on each push instead of the
existing one updating.

**Cause.** CodeSteward finds its previous comment by the hidden marker
`<!-- codesteward-report -->`. If that marker line was deleted or edited (for
example, someone reformatted the comment), CodeSteward can no longer find the
comment and posts a fresh one.

**Fixes.**

- Do not edit or remove the `<!-- codesteward-report -->` marker line.
- Delete the stray comment(s) once; the next run will create a single canonical
  comment and keep updating it.

## Base ref not found

**Symptom.** An error saying the base ref could not be resolved or verified.

**Cause.** CodeSteward could not determine a base ref, or the resolved ref does
not exist locally.

**How base resolution works.** CodeSteward tries, in order: the `--base` flag,
`GITHUB_BASE_REF`, `CI_MERGE_REQUEST_TARGET_BRANCH_NAME`, the `origin/HEAD`
symbolic ref, a local `main`, then a local `master`. If a base name `X` is not a
valid local ref but `origin/X` is, `origin/X` is used.

**Fixes.**

- Pass the base explicitly: `codesteward scan --base main`.
- Ensure the base branch is fetched (often the same shallow-clone fix above:
  `fetch-depth: 0` / `GIT_DEPTH: 0` / `git fetch --unshallow`).
- Fetch the branch by name if needed: `git fetch origin main`.

## `git` not found

**Symptom.** The scan fails immediately with an error about `git`.

**Cause.** CodeSteward requires the `git` executable on `PATH`.

**Fix.** Install git and ensure it is on `PATH`. In minimal container images,
add the git package.

## `--comment` outside CI

**Symptom.** `codesteward scan --comment` exits `1` locally, saying no provider
was detected.

**Cause.** `--comment` requires a detected GitHub Actions or GitLab CI
environment.

**Fix.** Use `--dry-run` to preview locally, or run inside CI where the provider
environment variables are set:

```bash
codesteward scan --comment --dry-run
```

## Exit codes reference

| Exit code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Runtime/scan error, or `config validate`/`codeowners validate` found fatal errors. |
| `2` | Usage error (unknown command or flag, bad value). |

`codesteward scan` never exits non-zero because of report content — it is a
comment-only tool.

## See also

- [Getting started](getting-started.md)
- [GitHub setup](github.md)
- [GitLab setup](gitlab.md)
- [FAQ](faq.md)
