# Getting Started with CodeSteward

CodeSteward is a deterministic, Apache-licensed PR/MR **review-readiness** bot
for open-source maintainers. It leaves one compact comment on a pull request or
merge request telling contributors whether their change is ready for maintainer
review — based on CODEOWNERS coverage, scope, path-aware test expectations,
description quality, and sensitive-file changes.

CodeSteward is **not** an AI reviewer, linter, or security scanner. It never
blocks merges, labels PRs, or assigns reviewers. It runs in comment-only mode.

This guide takes you from zero to a first local scan, then wires CodeSteward
into CI.

---

## 1. Install locally

CodeSteward is a single static Go binary with no runtime dependencies. It does
require the `git` command to be available on your `PATH`, because it shells out
to `git diff` to collect changed files.

### Build from source

You need Go 1.24 or newer.

```bash
git clone https://github.com/codesteward-ai/codesteward.git
cd codesteward
go build -o bin/codesteward ./cmd/codesteward
```

Then put the binary somewhere on your `PATH`:

```bash
sudo mv bin/codesteward /usr/local/bin/codesteward
```

### Verify the install

```bash
codesteward version
```

You should see a line like:

```text
codesteward 0.1.0-dev (commit none, built unknown, go1.24)
```

The version, commit, and build date are populated at release time; a
locally-built binary reports the development defaults.

---

## 2. Run your first scan

From inside a git repository, compare your working branch against the base
branch. CodeSteward writes the human-readable report to **stdout** and all
diagnostics to **stderr**.

```bash
cd /path/to/your/repo
codesteward scan --base main --head HEAD
```

If you omit `--head`, it defaults to `HEAD`. If you omit `--base`, CodeSteward
resolves it in this order:

1. the `--base` flag (if given),
2. the `GITHUB_BASE_REF` environment variable,
3. the `CI_MERGE_REQUEST_TARGET_BRANCH_NAME` environment variable,
4. the `origin/HEAD` symbolic ref,
5. a local `main` branch if it exists,
6. a local `master` branch if it exists,
7. otherwise an error asking you to pass `--base` explicitly.

> **Shallow clones fail here.** CodeSteward needs full history to find the merge
> base between your branch and the base ref. If you see an error about a ref not
> being found in a shallow clone, run `git fetch --unshallow` or check out with
> full depth. See [troubleshooting](troubleshooting.md).

### Get machine-readable output

```bash
codesteward scan --base main --format json --output codesteward-report.json
```

The JSON report includes the internal numeric score (`score`, 0–100). The
Markdown report **never** shows the score.

---

## 3. Read the report

A typical Markdown report looks like this:

```markdown
<!-- codesteward-report -->

## CodeSteward: Needs contributor action

**Review burden:** Medium  
**Ownership:** Partial  
**Tests:** Missing matching updates

Thanks for the contribution. A few changes would make this easier for maintainers to review.

### Before maintainer review

- Add or update matching tests for `src/runtime/cache.ts`.
- Add a short description explaining the motivation and test plan.
- Add specific ownership for `src/runtime/**` or ask a maintainer to route this area.

<details>
<summary>Why CodeSteward flagged this</summary>

- `src/runtime/cache.ts` changed, but no matching test file was changed.
- The PR description is empty.
- `src/runtime/cache.ts` is covered only by fallback ownership: `* @maintainers`.

</details>

_Comment-only mode. CodeSteward is not blocking this PR._
```

How to read it:

- **The heading status** is the one-line verdict: `Ready for maintainer
  review`, `Reviewable with notes`, `Needs contributor action`, `High review
  burden`, or `Needs owner routing`.
- **Review burden** (Low / Medium / High) estimates how much maintainer effort
  the change will take to review.
- **Ownership** (Complete / Partial / Missing / Not evaluated) reflects
  CODEOWNERS coverage of the production files you changed. Fallback-only
  (catch-all `*`) coverage counts as **Partial**.
- **Tests** summarizes whether matching tests appear to have been updated.
- **Before maintainer review** lists up to five concrete action items. Do these
  and re-push; the comment updates in place.
- The collapsed **Why CodeSteward flagged this** section lists every underlying
  finding.
- The internal numeric score is deliberately hidden from the Markdown report.
  See it with `--show-score` locally or in the JSON output.

To see the hidden score during a local run:

```bash
codesteward scan --base main --show-score
```

---

## 4. Add configuration

CodeSteward works with zero configuration using safe built-in defaults; when no
config file is found it prints `warning: no config file found; using built-in
defaults` to stderr and continues.

To customize behavior, create `.codesteward.yaml` at your repository root:

```yaml
project:
  name: my-library

review_readiness:
  max_files_changed: 12
  max_lines_changed: 500
  max_ownership_areas: 2

ownership:
  use_codeowners: true
  production_paths:
    - src/**
    - lib/**
    - packages/**
  ignore_paths:
    - docs/**
    - examples/**
    - README.md

tests:
  require_for:
    - src/**
    - lib/**
    - packages/**
  test_paths:
    - tests/**
    - test/**
    - "**/*.test.*"
    - "**/*.spec.*"
```

CodeSteward prefers `.codesteward.yaml` over `.codesteward.yml`. Any config you
provide is **merged over the defaults**: keys you omit keep their default
values, and a key set to an empty list overrides the default to empty.

Validate your config before committing it:

```bash
codesteward config validate
```

See the full [configuration reference](config.md) for every key, its type, and
default.

---

## 5. Wire it into CI

CodeSteward posts a single, self-updating comment from CI. Pick your platform:

- **GitHub Actions** — see [docs/github.md](github.md). Requires
  `permissions: contents: read` and `pull-requests: write`, and
  `fetch-depth: 0` on your checkout.
- **GitLab CI** — see [docs/gitlab.md](gitlab.md). Requires a project access
  token with `api` scope stored as `CODESTEWARD_GITLAB_TOKEN`, and `GIT_DEPTH:
  0`.

Both integrations update the existing CodeSteward comment (matched by the hidden
`<!-- codesteward-report -->` marker) instead of posting duplicates.

---

## Next steps

- [Configuration reference](config.md)
- [CODEOWNERS support](codeowners.md)
- [GitHub setup](github.md)
- [GitLab setup](gitlab.md)
- [Troubleshooting](troubleshooting.md)
- [FAQ](faq.md)
