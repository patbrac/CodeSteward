# CodeSteward Rules

CodeSteward findings come from a fixed catalog of deterministic rules. Every finding maps to one rule ID, one severity, and one penalty. This document lists the full catalog, explains how findings become a score and a status, and describes the `needs_owner_routing` override.

Findings are emitted **per file** where noted (so action items name the exact file), but scoring counts **each rule ID at most once** — see [Scoring model](#scoring-model).

## Rule catalog

Severities: `info`, `warning`, `action_required`. Penalties are the points subtracted from the starting score of 100 (once per distinct rule ID).

### Ownership rules (CS-OWN)

| Rule ID | Severity | Penalty | Triggers when | How to resolve |
|---|---|---|---|---|
| CS-OWN-001 | action_required | 25 | A production file has no CODEOWNERS match (one finding per file). | Add specific ownership for the path, or ask a maintainer to route this area. |
| CS-OWN-002 | warning | 10 | A production file is covered only by a fallback rule such as `* @maintainers` (one finding per file). | Add specific ownership for the file's directory (`<dir>/**`), or ask a maintainer to route this area. |
| CS-OWN-003 | warning | 10 | The change touches more distinct ownership areas than `max_ownership_areas` (one finding). | Split the change so each PR touches fewer ownership areas. |
| CS-OWN-004 | action_required | 15 | A sensitive file has no CODEOWNERS match (one finding per file). | Add ownership for the sensitive path so a maintainer is responsible for it. |

### Test rules (CS-TST)

| Rule ID | Severity | Penalty | Triggers when | How to resolve |
|---|---|---|---|---|
| CS-TST-001 | action_required | 25 | A production file changed but no matching test exists or was changed (one finding per file). | Add or update matching tests for the file. |
| CS-TST-002 | warning | 15 | A matching test exists on disk but was not changed in this PR/MR (one finding per file). | Update the existing test to cover the changes. |
| CS-TST-003 | info | 10 | A file requires tests but no path mapping matched it (one finding per file). | Add a `path_mappings` entry so CodeSteward can locate expected tests for this layout. |

### Scope rules (CS-SCP)

| Rule ID | Severity | Penalty | Triggers when | How to resolve |
|---|---|---|---|---|
| CS-SCP-001 | warning | 20 | Files changed exceed `max_files_changed` (one finding). | Split the change into smaller PRs. |
| CS-SCP-002 | warning | 15 | Lines changed (additions + deletions) exceed `max_lines_changed` (one finding). | Split the change into smaller PRs. |
| CS-SCP-003 | warning | 10 | Production source and a dependency manifest/lockfile changed in the same PR (one finding). | Split dependency changes from runtime changes. |
| CS-SCP-004 | warning | 10 | Production source, docs, and config/CI all changed together (one finding). | Split unrelated concerns into separate PRs. |
| CS-SCP-005 | info | 0 | More than 4 distinct top-level areas were touched (one finding; advisory only). | Consider whether the change can be narrowed to fewer areas. |

### Description rules (CS-DSC)

| Rule ID | Severity | Penalty | Triggers when | How to resolve |
|---|---|---|---|---|
| CS-DSC-001 | warning | 5 | The description is empty/whitespace and `warn_if_empty` is on (one finding). | Add a short description explaining the motivation and test plan. |
| CS-DSC-002 | warning | 10 | The description is non-empty but shorter than `min_length` (one finding). | Expand the description to meet the configured minimum length. |
| CS-DSC-003 | warning | 15 | A configured required section is missing (one finding per missing section). | Add the missing required section to the description. |
| CS-DSC-004 | warning | 10 | `require_linked_issue` is on and no `#123` or issue URL reference is present (one finding). | Reference the related issue (e.g. `#123` or an issue URL). |

### Sensitive path rules (CS-SNS)

Each sensitive file fires exactly one CS-SNS rule, chosen by priority: lockfile > CI/release workflow > manifest > configured-other. Each rule produces one finding that lists all matching paths.

| Rule ID | Severity | Penalty | Triggers when | How to resolve |
|---|---|---|---|---|
| CS-SNS-001 | warning | 15 | A lockfile changed (one finding, all lockfile paths listed). | Call out the lockfile change in the description so maintainers can verify it. |
| CS-SNS-002 | warning | 15 | A CI/release workflow changed (one finding, paths listed). | Call out the CI workflow change in the description so maintainers can verify it. |
| CS-SNS-003 | warning | 10 | A package manifest changed (one finding, paths listed). | Call out the package manifest change in the description so maintainers can verify it. |
| CS-SNS-004 | warning | 10 | Another configured sensitive path changed (one finding, paths listed). | Call out the sensitive path change in the description so maintainers can verify it. |

### Sensitive path classification

A sensitive file fires exactly one CS-SNS rule. The built-in sets (case-sensitive basenames / path globs) are:

- **Lockfiles (CS-SNS-001):** `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`, `bun.lockb`, `go.sum`, `Cargo.lock`, `poetry.lock`, `Gemfile.lock`, `composer.lock`.
- **CI/release (CS-SNS-002):** `.github/workflows/**`, `.gitlab-ci.yml`, `scripts/release/**`.
- **Manifests (CS-SNS-003):** `package.json`, `go.mod`, `Cargo.toml`, `pyproject.toml`, `Gemfile`, `composer.json`.
- **Configured-other (CS-SNS-004):** anything listed in the config `sensitive_paths` that is not caught by a set above.

## Scoring model

The internal score is computed deterministically:

```
score = clamp(100 - Σ penalty(ruleID) for each DISTINCT rule ID present, 0, 100)
```

- The score **starts at 100**.
- Each **distinct rule ID** present in the findings subtracts its penalty **once**, no matter how many files fired that rule. For example, ten files each firing CS-TST-001 still subtract 25 in total, not 250.
- The result is **clamped** to the range `[0, 100]`.

The score is included in the JSON report and hidden from the Markdown comment by default. Use `codesteward scan --show-score` to reveal it in Markdown.

### Worked example

For the demo Scenario 2 (change to `src/runtime/cache.ts` with an empty description) three distinct rules fire:

- CS-OWN-002 (fallback-only ownership): −10
- CS-TST-001 (missing matching test): −25
- CS-DSC-001 (empty description): −5

Score = `100 − 10 − 25 − 5 = 60`.

## Status thresholds

The score maps to a readiness status by these fixed bands:

| Score range | Status |
|---|---|
| 85–100 | `ready_for_maintainer_review` |
| 65–84 | `reviewable_with_notes` |
| 40–64 | `needs_contributor_action` |
| 0–39 | `high_review_burden` |

A score of 60 (Scenario 2) therefore maps to `needs_contributor_action`, unless the ownership override below applies.

## `needs_owner_routing` override

The status is overridden to `needs_owner_routing` when **all** of the following hold:

1. The ownership state is `missing`, and
2. The score is below 85, and
3. The combined penalty from ownership rules (CS-OWN-*) is **greater than or equal to** the combined penalty of every other rule category put together.

This surfaces changes where the dominant problem is that no owner is responsible for the touched code — the most useful action a maintainer can take is to route it to an owner, rather than asking the contributor for generic fixes.

## Review burden mapping

The review burden level is derived from the score and the findings:

- **high** — the score is below 40, **or** CS-SCP-001 (too many files) or CS-SCP-002 (too many lines) fired.
- **low** — the score is at least 85 **and** no CS-SCP-* or CS-SNS-* findings fired.
- **medium** — every other case.

## Exit criteria

CodeSteward produces one exit criterion per `action_required` finding, deduplicated by rule ID and first path. Each exit criterion's description is the finding's action item — a concrete condition that, once met, would clear the finding.

## Canonical order

Findings (and therefore action items and detail lines) always render in a fixed canonical order so output is byte-identical across runs:

1. Severity rank — `action_required` (0), then `warning` (1), then `info` (2).
2. Rule ID ascending.
3. First path entry ascending.
4. Message ascending.

Paths within a single finding are also sorted ascending.
