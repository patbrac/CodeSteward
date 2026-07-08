# CodeSteward Product Spec (v0)

CodeSteward is an Apache-licensed, Go-based, **deterministic** PR/MR review-readiness bot for open-source maintainers. Its job is to reduce maintainer review overload by leaving one compact, helpful review-readiness comment on each pull request or merge request.

## Product promise

Install CodeSteward on a GitHub or GitLab repository and it will leave one compact PR/MR comment telling contributors:

1. Whether the change is ready for maintainer review.
2. Whether the changed files have clear ownership.
3. Whether matching tests appear to have been updated.
4. Whether the PR/MR is too large or too broad.
5. What specific actions would reduce maintainer burden before human review.

CodeSteward is a **maintainer-time protection tool**, not a gatekeeper. In v0 it runs in comment-only mode: it posts (and updates) a single comment and never blocks a merge, sets a commit status, adds a label, or requests reviewers.

## v0 scope

CodeSteward v0 evaluates the diff between a base and head ref and produces a normalized report driven by five deterministic signal categories:

- **CODEOWNERS ownership coverage** — real `.github/CODEOWNERS`, `CODEOWNERS`, `docs/CODEOWNERS`, and `.gitlab/CODEOWNERS` parsing with GitHub and GitLab dialects, last-match-wins semantics, and fallback/broad/specific classification.
- **Path-aware test expectations** — configurable `{path}`/`{name}`/`{ext}` mappings from source files to expected test files, checking whether a matching test was changed, exists but was not changed, or is missing. No semantic coverage analysis.
- **Scope and review burden** — file count, line count, top-level areas touched, and mixed-concern combinations (source + dependencies, source + docs + config).
- **PR/MR description quality** — empty, too-short, missing-required-section, and missing-linked-issue checks (the latter two only when configured).
- **Sensitive path changes** — lockfiles, package manifests, CI/release workflows, and configured sensitive paths.

Findings roll up into an internal numeric score (0–100), a user-facing readiness status, a review burden level, ownership and test summaries, and a set of exit criteria. The score is emitted in JSON but hidden from the Markdown comment by default.

Locked v0 decisions include: Go implementation, Apache 2.0 license, deterministic core, no AI/LLM, GitHub and GitLab support, compact Markdown output, internal-only score hidden by default, single-comment update behavior, real CODEOWNERS support, fallback ownership treated as partial, path-aware test checking, config via `.codesteward.yaml` (preferred) or `.codesteward.yml`.

## Readiness statuses

The report carries exactly one readiness status. The enum values are:

| Status value | Meaning |
|---|---|
| `ready_for_maintainer_review` | The change looks ready for a human maintainer to review. |
| `reviewable_with_notes` | Reviewable, but there are minor notes worth addressing. |
| `needs_contributor_action` | The contributor should make changes before maintainer review. |
| `high_review_burden` | The change imposes a high review burden as-is. |
| `needs_owner_routing` | Ownership is the dominant issue; the change needs owner routing. |

`needs_owner_routing` is an override applied when ownership is missing and ownership penalties dominate all other categories combined (see [rules](rules.md)).

## Review burden levels

The enum values are:

| Burden value |
|---|
| `low` |
| `medium` |
| `high` |

## Ownership states

The ownership summary carries exactly one state. The enum values are:

| Ownership value | Meaning |
|---|---|
| `complete` | Every relevant production file has specific or broad ownership. |
| `partial` | At least one relevant production file is covered only by fallback ownership. |
| `missing` | At least one relevant production file has no owner. |
| `not_evaluated` | Ownership was disabled, or no relevant files were changed. |

## Test states

The tests summary carries exactly one state. The enum values are:

| Test value | Meaning |
|---|---|
| `not_required` | No changed file required tests. |
| `matching_test_changed` | At least one expected matching test file changed. |
| `existing_test_found_but_not_changed` | A matching test exists on disk but this change did not update it. |
| `missing_matching_test` | No matching test file exists or was changed. |
| `not_evaluated` | Tests were disabled, or no path mappings were configured. |

## Severity levels

Every finding carries exactly one severity. The enum values are:

| Severity value | Meaning |
|---|---|
| `info` | Advisory; may carry a small or zero penalty. |
| `warning` | Worth addressing before review. |
| `action_required` | The contributor should act; produces an exit criterion. |

## What CodeSteward is NOT

CodeSteward is **not** an AI reviewer, a linter, a security scanner, or a blocking policy engine. In v0 it specifically does not:

- Use any LLM or AI, or detect AI-generated code.
- Scan for security vulnerabilities or perform deep semantic analysis.
- Block checks, fail builds, or set required commit statuses.
- Auto-label PRs, auto-request or auto-assign reviewers.
- Limit the number of open PRs a contributor may have, or moderate contributors.
- Provide a SaaS dashboard, historical trend analytics, or enterprise governance.
- Ship language-specific analyzers.

See [non-goals.md](non-goals.md) for the complete v0 non-goals list.

## What it is, in one paragraph

CodeSteward is a deterministic, Apache-licensed CLI and CI bot that reads a PR/MR diff, checks ownership (CODEOWNERS), test updates (path-aware), scope, description, and sensitive files, and leaves one compact, non-blocking comment telling contributors how ready their change is for maintainer review and what would make it easier — no AI, no gatekeeping, just a repeatable readiness signal that protects maintainer time.
