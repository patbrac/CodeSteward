# CodeSteward

CodeSteward is an Apache-licensed, deterministic PR/MR review-readiness bot that leaves one compact comment telling contributors whether a change is ready for maintainer review — no AI, no blocking.

## What it does

CodeSteward looks at the diff between a base and head ref and reports, deterministically:

- Whether changed files have clear **CODEOWNERS** ownership.
- Whether matching **tests** appear to have been updated (path-aware, language-agnostic).
- Whether the change is too **large or too broad** in scope.
- Whether the PR/MR **description** is adequate.
- Whether **sensitive files** (lockfiles, manifests, CI workflows) changed.

It rolls those observations into an internal readiness score and a single, contributor-friendly comment. It never blocks, labels, assigns reviewers, or moderates contributors.

## Example comment

This is exactly what CodeSteward posts for a change to `src/runtime/cache.ts` with an empty description in the demo package (Scenario 2):

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

The `<!-- codesteward-report -->` marker is how CodeSteward finds and **updates** its own comment instead of posting a duplicate on every push.

## Comment-only mode

CodeSteward runs in **comment-only mode** in v0, and this is the only mode. It posts (or updates) a single Markdown comment and always exits successfully — the `scan` command never fails a build because of report content. CodeSteward does not set commit statuses, does not block merges, does not add labels, and does not request reviewers. The final line of every comment states this plainly: `_Comment-only mode. CodeSteward is not blocking this PR._`

## Deterministic, no AI in v0

CodeSteward is **fully deterministic and uses no AI or LLM in v0.** Identical inputs always produce byte-identical output: findings, action items, owners, and file lists are sorted with a fixed canonical order, and reports contain no timestamps, random values, or absolute paths. Every finding maps to a documented rule ID with a fixed penalty. This makes reports reproducible, reviewable, and safe to diff in CI.

## Install and run locally

Build the CLI from source (Go 1.24+):

```bash
git clone https://github.com/codesteward-ai/codesteward
cd codesteward
go build -o bin/codesteward ./cmd/codesteward
```

Then scan a change from your repository root:

```bash
# Scan the current branch against main, print the Markdown report to stdout
codesteward scan --base main --head HEAD --format markdown

# Write a JSON report (includes the internal score) to a file
codesteward scan --base main --format json --output codesteward-report.json

# Reveal the internal score in the Markdown report (hidden by default)
codesteward scan --base main --show-score
```

Other commands:

```bash
codesteward version
codesteward ownership audit            # repository-wide ownership coverage
codesteward config validate            # validate .codesteward.yaml
codesteward codeowners validate        # validate CODEOWNERS syntax
```

## Install on GitHub Actions

Add a workflow that checks out the full history (`fetch-depth: 0` is required so the diff base resolves) and runs the CodeSteward action with commenting enabled:

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

`pull-requests: write` is required so CodeSteward can post and update its single PR comment. See [docs/github.md](docs/github.md) for details.

## Install on GitLab CI

Add a job to `.gitlab-ci.yml` (or include the provided template) that runs on merge request pipelines:

```yaml
codesteward:
  image: ghcr.io/codesteward-ai/codesteward:v0.1.0
  variables:
    # Full history so CodeSteward can diff against the merge request target.
    GIT_DEPTH: 0
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
  script:
    - codesteward scan --comment
```

CodeSteward posts the MR note using `CODESTEWARD_GITLAB_TOKEN` if set, otherwise `CI_JOB_TOKEN`. See [docs/gitlab.md](docs/gitlab.md) for token requirements.

## 60-second walkthrough

The [`examples/typescript-package`](examples/typescript-package) demo is a small TypeScript library with a deliberate ownership gap: its `CODEOWNERS` gives specific owners to `/src/parser/`, `/src/public/`, and `/docs/`, but `src/runtime/` is covered only by the `* @maintainers` fallback. Three scenarios show the product in under a minute:

1. **Good PR** — change `src/parser/tokenize.ts` *and* `tests/parser/tokenize.test.ts` with a real description. CodeSteward reports **Ready for maintainer review**, ownership complete, matching tests changed, review burden low, score 100.
2. **Missing tests and weak ownership** — change only `src/runtime/cache.ts` with an empty description. CodeSteward reports **Needs contributor action**, ownership partial, missing matching tests, review burden medium (this is the example comment above).
3. **Broad PR** — change `src/parser/parse.ts`, `src/runtime/cache.ts`, `docs/usage.md`, `package.json`, and `.github/workflows/release.yml` together. CodeSteward reports **High review burden**, flags mixed concerns, dependency-plus-source changes, and sensitive CI/manifest changes, and suggests splitting the PR.

Run scenario 2 yourself:

```bash
cd examples/typescript-package
codesteward scan --base main --head HEAD
```

## Documentation

- [Product spec](docs/product-spec.md) — v0 scope, statuses, burdens, ownership and test states.
- [Non-goals](docs/non-goals.md) — what CodeSteward deliberately is not.
- [Rules](docs/rules.md) — the full rule catalog, scoring model, and status thresholds.
- [Reports](docs/reports.md) — Markdown and JSON report anatomy.
- [Config](docs/config.md) — every `.codesteward.yaml` key and its default.
- [CODEOWNERS](docs/codeowners.md) — discovery, dialects, and matching semantics.

## License

Apache License 2.0. Copyright 2026 The CodeSteward Authors. See [LICENSE](LICENSE).
