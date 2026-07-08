# CodeSteward Phased Build Plan

## Product Summary

**CodeSteward** is an Apache-licensed, Go-based, deterministic PR/MR readiness bot for open-source maintainers.

Its purpose is to reduce maintainer review overload by producing compact, helpful review-readiness comments based on:

- CODEOWNERS ownership coverage
- PR/MR scope and review burden
- Path-aware test expectations
- PR/MR description quality
- Sensitive file and dependency changes
- Deterministic internal readiness scoring

CodeSteward is not an AI reviewer, linter, security scanner, or blocking policy engine in v0. It is a maintainer-time protection tool.

---

## Locked v0 Decisions

| Area | Decision |
|---|---|
| Core language | Go |
| License | Apache 2.0 |
| Primary user | Small open-source library maintainers |
| First demo target | TypeScript package |
| Core behavior | Deterministic |
| AI/LLM usage | None in v0 |
| Supported platforms | GitHub and GitLab |
| Output style | Compact Markdown report |
| Report score | Internal numeric score only |
| User-facing score | Hidden by default |
| First public release | Includes GitHub/GitLab comment posting |
| Blocking behavior | No blocking in v0 |
| Auto-labeling | No auto-labeling in v0 |
| Auto-reviewer assignment | No auto-reviewer assignment in v0 |
| Contributor moderation | No moderation features in v0 |
| Config file | `.codesteward.yaml` preferred, `.codesteward.yml` supported |
| Ownership source | Real CODEOWNERS support |
| Fallback ownership | Treated as partial ownership by default |
| Test checking | Path-aware |
| CLI name | `codesteward` |
| Package manager/build system | Go modules |

---

## v0 Product Promise

Install CodeSteward on a GitHub or GitLab repository and it will leave one compact PR/MR comment telling contributors:

1. Whether the change is ready for maintainer review.
2. Whether the changed files have clear ownership.
3. Whether matching tests appear to have been updated.
4. Whether the PR/MR is too large or too broad.
5. What specific actions would reduce maintainer burden before human review.

Example comment:

```markdown
<!-- codesteward-report -->

## CodeSteward: Needs contributor action

**Review burden:** High  
**Ownership:** Partial  
**Tests:** Missing matching updates

Thanks for the contribution. A few changes would make this easier for maintainers to review.

### Before maintainer review

- Add or update matching tests for `src/runtime/cache.ts`.
- Add specific ownership for `src/runtime/**` or ask a maintainer to route this area.
- Consider splitting dependency changes from runtime changes.
- Add a short description explaining the motivation and test plan.

<details>
<summary>Why CodeSteward flagged this</summary>

- `src/runtime/cache.ts` changed, but no matching test file was changed.
- `src/runtime/cache.ts` is only covered by fallback ownership: `* @maintainers`.
- `package.json` changed alongside runtime source files.
- The PR description is empty.

</details>

_Comment-only mode. CodeSteward is not blocking this PR._
```

---

## v0 Non-Goals

Do not include these in v0:

- LLM-based code review
- AI-generated code detection
- Security vulnerability scanning
- Deep semantic code analysis
- Blocking checks
- Auto-labeling
- Auto-reviewer requests
- Contributor open-PR limits
- Moderation rules
- SaaS dashboard
- Full CI artifact report
- Enterprise governance features
- Language-specific analyzers
- Historical trend dashboard
- SSO/SAML/SCIM

---

## Recommended Repository Structure

```text
codesteward/
  cmd/
    codesteward/
      main.go

  internal/
    cli/
    config/
    git/
    diff/
    codeowners/
    ownership/
    tests/
    readiness/
    rules/
    report/
    providers/
      github/
      gitlab/

  pkg/
    model/
    engine/

  examples/
    typescript-package/
      src/
      tests/
      docs/
      CODEOWNERS
      .codesteward.yaml

  actions/
    github/
      action.yml

  gitlab/
    codesteward.gitlab-ci.yml

  docs/
    getting-started.md
    github.md
    gitlab.md
    config.md
    codeowners.md
    rules.md
    reports.md

  .github/
    workflows/
      ci.yml
      release.yml

  go.mod
  go.sum
  Makefile
  LICENSE
  README.md
```

Use `internal/` for most implementation packages until the public API stabilizes. Keep `pkg/model` and `pkg/engine` small and conservative.

---

# Phase 0 — Product and Technical Specification

## Goal

Create the written foundation for the project before implementation begins.

## Tasks

- [ ] Create `README.md` with the product promise and first example report.
- [ ] Create `docs/product-spec.md` explaining the v0 scope.
- [ ] Create `docs/non-goals.md` to prevent scope creep.
- [ ] Define the default compact Markdown report format.
- [ ] Define the initial JSON report schema.
- [ ] Define the config schema for `.codesteward.yaml` and `.codesteward.yml`.
- [ ] Define readiness statuses.
- [ ] Define review burden levels.
- [ ] Define ownership states.
- [ ] Define test states.
- [ ] Define rule IDs and finding severity levels.
- [ ] Define the hidden comment marker: `<!-- codesteward-report -->`.

## Key Design Outputs

### Readiness statuses

```text
ready_for_maintainer_review
reviewable_with_notes
needs_contributor_action
high_review_burden
needs_owner_routing
```

### Review burden levels

```text
low
medium
high
```

### Ownership states

```text
complete
partial
missing
not_evaluated
```

### Test states

```text
not_required
matching_test_changed
existing_test_found_but_not_changed
missing_matching_test
not_evaluated
```

### Finding severity levels

```text
info
warning
action_required
```

## Exit Criteria

- [ ] The project has a clear v0 spec.
- [ ] The report format is defined before implementation.
- [ ] The JSON schema is defined before implementation.
- [ ] The config schema is defined before implementation.
- [ ] The team can explain what CodeSteward is and is not in one paragraph.

---

# Phase 1 — Go Project Scaffold and CLI Skeleton

## Goal

Create the initial Go repository structure and a working `codesteward` CLI with placeholder commands.

## Tasks

- [ ] Initialize Go module.
- [ ] Add Apache 2.0 `LICENSE`.
- [ ] Add root `README.md`.
- [ ] Add `Makefile` with common commands.
- [ ] Add basic CI workflow for lint/test/build.
- [ ] Implement CLI command structure.
- [ ] Add `codesteward version`.
- [ ] Add `codesteward scan` placeholder.
- [ ] Add `codesteward ownership audit` placeholder.
- [ ] Add `codesteward config validate` placeholder.
- [ ] Add `codesteward codeowners validate` placeholder.
- [ ] Add structured error handling.
- [ ] Add standard logging behavior.
- [ ] Add `--format` flag with `markdown` and `json` accepted.
- [ ] Add `--output` flag.
- [ ] Add `--config` flag.
- [ ] Add `--repo-root` flag.

## Suggested Commands

```bash
codesteward version
codesteward scan
codesteward scan --base main --head HEAD --format markdown
codesteward scan --base main --format json --output codesteward-report.json
codesteward ownership audit
codesteward config validate
codesteward codeowners validate
```

## Exit Criteria

- [ ] `go test ./...` passes.
- [ ] `go build -o bin/codesteward ./cmd/codesteward` succeeds.
- [ ] CLI commands exist and return predictable placeholder output.
- [ ] CLI supports `--help` for all commands.
- [ ] CLI exits with stable non-zero codes on invalid usage.
- [ ] CI runs on every pull request.

---

# Phase 2 — Configuration System

## Goal

Support project configuration through `.codesteward.yaml` and `.codesteward.yml`, with safe defaults when no config exists.

## Tasks

- [ ] Implement config discovery order:
  - [ ] `--config <path>`
  - [ ] `.codesteward.yaml`
  - [ ] `.codesteward.yml`
- [ ] Implement YAML parsing.
- [ ] Implement default config values.
- [ ] Implement config validation.
- [ ] Implement unknown-key warnings.
- [ ] Implement invalid-glob warnings.
- [ ] Implement docs for all config keys.
- [ ] Add unit tests for config loading.
- [ ] Add unit tests for config precedence.
- [ ] Add unit tests for missing config behavior.

## Initial Config Shape

```yaml
project:
  name: example-typescript-package

mode:
  comment_only: true

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
  path_mappings:
    - from: "src/{path}/{name}.{ext}"
      expect:
        - "tests/{path}/{name}.test.{ext}"
        - "tests/{path}/{name}.spec.{ext}"
        - "src/{path}/{name}.test.{ext}"
        - "src/{path}/{name}.spec.{ext}"

pr_description:
  warn_if_empty: true
  min_length: 80
  required_sections: []
  require_linked_issue: false

sensitive_paths:
  - package.json
  - package-lock.json
  - pnpm-lock.yaml
  - yarn.lock
  - .github/workflows/**
  - .gitlab-ci.yml
  - scripts/release/**
```

## Default Behavior Without Config

- [ ] Run with built-in defaults.
- [ ] Warn that no config file was found.
- [ ] Do not fail the scan.
- [ ] Treat `src/**`, `lib/**`, and `packages/**` as likely production paths.
- [ ] Treat `tests/**`, `test/**`, `**/*.test.*`, and `**/*.spec.*` as likely test paths.

## Exit Criteria

- [ ] `.codesteward.yaml` is preferred over `.codesteward.yml`.
- [ ] `--config` overrides default discovery.
- [ ] Invalid config produces helpful errors.
- [ ] Missing config produces safe default behavior.
- [ ] Config validation command works.
- [ ] Config documentation exists.

---

# Phase 3 — Git and Diff Engine

## Goal

Build the provider-neutral change detection layer that powers the scan.

## Tasks

- [ ] Detect repository root.
- [ ] Detect current branch.
- [ ] Accept `--base` and `--head` refs.
- [ ] Default `--head` to `HEAD`.
- [ ] Default `--base` from provider environment or repository default branch when possible.
- [ ] Run git diff to collect changed files.
- [ ] Collect file status:
  - [ ] added
  - [ ] modified
  - [ ] deleted
  - [ ] renamed
  - [ ] copied, if available
- [ ] Collect line additions and deletions.
- [ ] Normalize file paths to slash-separated repository-relative paths.
- [ ] Detect binary files.
- [ ] Detect generated or vendored files only through config or simple path heuristics.
- [ ] Add tests using fixture repositories.
- [ ] Add support for shallow clone warnings.

## Changed File Model

```go
type ChangedFile struct {
    Path          string
    OldPath       string
    Status        string
    Additions     int
    Deletions     int
    IsBinary      bool
    IsTest        bool
    IsProduction  bool
    IsSensitive   bool
}
```

## Exit Criteria

- [ ] `codesteward scan --base main --head HEAD` can list changed files.
- [ ] Added, modified, deleted, and renamed files are handled.
- [ ] Line counts are captured.
- [ ] Path normalization is stable across operating systems.
- [ ] Shallow clone problems produce actionable messages.
- [ ] Diff engine has unit and integration tests.

---

# Phase 4 — CODEOWNERS Discovery, Parsing, and Validation

## Goal

Support real CODEOWNERS ownership analysis for GitHub and GitLab.

## Tasks

- [ ] Discover CODEOWNERS file.
- [ ] Support GitHub locations:
  - [ ] `.github/CODEOWNERS`
  - [ ] `CODEOWNERS`
  - [ ] `docs/CODEOWNERS`
- [ ] Support GitLab locations:
  - [ ] `CODEOWNERS`
  - [ ] `docs/CODEOWNERS`
  - [ ] `.gitlab/CODEOWNERS`
- [ ] Implement provider dialect mode:
  - [ ] `github`
  - [ ] `gitlab`
  - [ ] `auto`
- [ ] Parse comments and blank lines.
- [ ] Parse path patterns.
- [ ] Parse multiple owners per rule.
- [ ] Support owner forms:
  - [ ] `@username`
  - [ ] `@org/team`
  - [ ] email address
- [ ] Implement last-match-wins behavior for GitHub-style rules.
- [ ] Implement GitLab section parsing at a basic level.
- [ ] Warn on unsupported syntax.
- [ ] Warn on invalid lines.
- [ ] Implement `codesteward codeowners validate`.
- [ ] Add extensive CODEOWNERS fixtures.

## Ownership Match Classification

```text
specific
broad
fallback
missing
```

A catch-all rule like this is fallback ownership:

```text
* @maintainers
```

A broad path like this may be broad ownership:

```text
/src/** @core-team
```

A more precise path like this is specific ownership:

```text
/src/parser/** @parser-maintainers
```

## Exit Criteria

- [ ] CodeSteward can find the correct CODEOWNERS file.
- [ ] CodeSteward can parse common GitHub CODEOWNERS files.
- [ ] CodeSteward can parse common GitLab CODEOWNERS files.
- [ ] CodeSteward can match changed files to owners.
- [ ] Fallback ownership is treated as partial ownership.
- [ ] Invalid CODEOWNERS syntax produces helpful validation output.
- [ ] Unit tests cover common and edge-case patterns.

---

# Phase 5 — Ownership Analysis

## Goal

Use CODEOWNERS and config to determine whether a PR/MR has clear ownership or creates maintainer routing burden.

## Tasks

- [ ] Identify production files from config.
- [ ] Ignore configured paths.
- [ ] Match changed files to CODEOWNERS entries.
- [ ] Determine ownership coverage for each changed production file.
- [ ] Detect missing ownership.
- [ ] Detect fallback-only ownership.
- [ ] Detect broad ownership.
- [ ] Count ownership areas touched.
- [ ] Detect when the PR/MR exceeds `max_ownership_areas`.
- [ ] Generate ownership findings.
- [ ] Generate ownership exit criteria.
- [ ] Add `codesteward ownership audit` for repository-wide analysis.

## Ownership Summary Logic

```text
complete:
  all relevant production files have specific or acceptable broad ownership

partial:
  at least one relevant production file has fallback-only or broad ownership

missing:
  at least one relevant production file has no owner

not_evaluated:
  ownership is disabled or no relevant files were changed
```

## Example Findings

```text
No owner found for src/runtime/cache.ts.
```

```text
src/runtime/cache.ts is covered only by fallback ownership: * @maintainers.
```

```text
This change touches 4 ownership areas, above the configured limit of 2.
```

## Exit Criteria

- [ ] Ownership status is calculated correctly.
- [ ] Missing owners are identified.
- [ ] Fallback owners are treated as partial ownership.
- [ ] Ownership area count is calculated.
- [ ] Ownership findings produce contributor-friendly action items.
- [ ] Repository-wide ownership audit works.

---

# Phase 6 — Path-Aware Test Expectation Engine

## Goal

Detect whether production changes have matching test updates without performing full semantic coverage analysis.

## Tasks

- [ ] Identify production files using `tests.require_for` and ownership production paths.
- [ ] Identify test files using `tests.test_paths`.
- [ ] Implement path mapping parser.
- [ ] Implement `{path}`, `{name}`, and `{ext}` placeholders.
- [ ] Generate expected test candidates for each changed production file.
- [ ] Detect whether expected test candidate was changed.
- [ ] Detect whether expected test candidate exists but was not changed.
- [ ] Detect whether no matching test exists.
- [ ] Allow config-driven overrides.
- [ ] Add fixture tests for TypeScript-style layouts.
- [ ] Keep behavior language-agnostic.

## Test State Logic

```text
not_required:
  no changed files require tests

matching_test_changed:
  at least one expected matching test file changed

existing_test_found_but_not_changed:
  matching test exists, but was not changed in the PR/MR

missing_matching_test:
  no matching test file exists or changed

not_evaluated:
  tests disabled or mappings invalid
```

## Example Mapping

Changed file:

```text
src/parser/tokenize.ts
```

Expected test candidates:

```text
tests/parser/tokenize.test.ts
tests/parser/tokenize.spec.ts
src/parser/tokenize.test.ts
src/parser/tokenize.spec.ts
```

## Example Findings

```text
src/runtime/cache.ts changed, but no matching test file was changed.
```

```text
A matching test exists for src/parser/tokenize.ts, but this PR did not update it.
```

## Exit Criteria

- [ ] Path-aware test mappings work for TypeScript demo repo.
- [ ] The engine remains language-agnostic.
- [ ] Changed test files are detected.
- [ ] Existing-but-unchanged test files are detected.
- [ ] Missing matching tests are detected.
- [ ] Test findings produce clear action items.

---

# Phase 7 — Scope, Description, and Sensitive Path Rules

## Goal

Add deterministic review-readiness rules beyond ownership and tests.

## Tasks

### Scope Rules

- [ ] Detect too many files changed.
- [ ] Detect too many lines changed.
- [ ] Detect too many top-level areas changed.
- [ ] Detect source + docs + config changes in one PR/MR.
- [ ] Detect source + dependency file changes in one PR/MR.
- [ ] Suggest splitting broad PRs.

### Description Rules

- [ ] Warn when PR/MR description is empty.
- [ ] Warn when PR/MR description is shorter than `min_length`.
- [ ] Enforce required sections only when configured.
- [ ] Require linked issue only when configured.
- [ ] Detect basic issue references when configured.

### Sensitive Path Rules

- [ ] Detect package manifest changes.
- [ ] Detect lockfile changes.
- [ ] Detect CI workflow changes.
- [ ] Detect release script changes.
- [ ] Detect configured sensitive paths.
- [ ] Generate action items for sensitive changes.

## Default Thresholds

```yaml
review_readiness:
  max_files_changed: 12
  max_lines_changed: 500
  max_ownership_areas: 2
```

## Default Production Paths

```yaml
ownership:
  production_paths:
    - src/**
    - lib/**
    - packages/**
```

## Default Test Paths

```yaml
tests:
  test_paths:
    - tests/**
    - test/**
    - "**/*.test.*"
    - "**/*.spec.*"
```

## Exit Criteria

- [ ] Scope rules produce findings and action items.
- [ ] Empty descriptions warn by default.
- [ ] Description templates are only enforced when configured.
- [ ] Sensitive file changes are detected.
- [ ] Dependency and lockfile changes are detected.
- [ ] Rules remain deterministic and explainable.

---

# Phase 8 — Readiness Scoring and Report Model

## Goal

Convert findings into an internal numeric score, user-facing status, review burden level, ownership summary, test summary, and exit criteria.

## Tasks

- [ ] Implement normalized `Finding` model.
- [ ] Implement normalized `ExitCriterion` model.
- [ ] Implement scoring engine.
- [ ] Implement status mapping.
- [ ] Implement review burden mapping.
- [ ] Implement ownership summary aggregation.
- [ ] Implement test summary aggregation.
- [ ] Ensure score is included in JSON.
- [ ] Ensure score is hidden from Markdown by default.
- [ ] Add unit tests for scoring.
- [ ] Add snapshot tests for report objects.

## Initial Internal Scoring

Start at `100`.

### Ownership penalties

```text
-25 no owner for production file
-15 sensitive path has no owner
-10 only fallback owner matched
-10 more than 2 ownership areas touched
```

### Test penalties

```text
-25 production file changed with no matching test found
-15 existing matching test found but not changed
-10 test mapping could not be resolved
```

### Scope penalties

```text
-20 files changed exceed limit
-15 lines changed exceed limit
-10 source + dependency files changed together
-10 source + docs + config changed together
```

### Description penalties

```text
-15 missing required section
-10 description shorter than configured minimum
-10 linked issue missing when required
-5 empty description warning
```

### Sensitive path penalties

```text
-15 lockfile changed
-15 CI/release workflow changed
-10 package manifest changed
-10 configured sensitive path touched
```

## Status Mapping

```text
85–100: ready_for_maintainer_review
65–84: reviewable_with_notes
40–64: needs_contributor_action
0–39: high_review_burden
```

`needs_owner_routing` may override the display status when ownership is the dominant issue.

## Exit Criteria

- [ ] Every finding has a rule ID.
- [ ] Every action-required finding can produce an exit criterion.
- [ ] Internal score is deterministic.
- [ ] Status mapping is stable.
- [ ] Markdown hides the numeric score by default.
- [ ] JSON includes the numeric score.
- [ ] Snapshot tests cover representative reports.

---

# Phase 9 — Compact Markdown and JSON Report Rendering

## Goal

Render the report into compact Markdown for PR/MR comments and stable JSON for automation.

## Tasks

- [ ] Implement Markdown renderer.
- [ ] Implement JSON renderer.
- [ ] Add hidden comment marker.
- [ ] Limit visible action items to 3–5.
- [ ] Put detailed findings in a collapsed `<details>` section.
- [ ] Include comment-only disclaimer.
- [ ] Ensure Markdown is readable in GitHub.
- [ ] Ensure Markdown is readable in GitLab.
- [ ] Add snapshot tests for Markdown output.
- [ ] Add schema tests for JSON output.

## Markdown Report Structure

```markdown
<!-- codesteward-report -->

## CodeSteward: <Status>

**Review burden:** <Low|Medium|High>  
**Ownership:** <Complete|Partial|Missing|Not evaluated>  
**Tests:** <Present|Missing matching updates|Not required|Not evaluated>

Thanks for the contribution. A few changes would make this easier for maintainers to review.

### Before maintainer review

- <Action item 1>
- <Action item 2>
- <Action item 3>

<details>
<summary>Why CodeSteward flagged this</summary>

- <Finding detail 1>
- <Finding detail 2>
- <Finding detail 3>

</details>

_Comment-only mode. CodeSteward is not blocking this PR._
```

## Exit Criteria

- [ ] Markdown output is compact and contributor-friendly.
- [ ] Markdown output does not expose the numeric score.
- [ ] Markdown includes no more than 5 visible action items.
- [ ] Markdown includes detailed findings in a collapsed section.
- [ ] JSON output includes the full normalized report.
- [ ] Markdown and JSON outputs are covered by snapshot tests.

---

# Phase 10 — GitHub Integration

## Goal

Support GitHub pull requests with a composite action and single-comment update behavior.

## Tasks

- [ ] Create GitHub composite action under `actions/github/action.yml`.
- [ ] Add binary download/install step.
- [ ] Detect GitHub Actions environment.
- [ ] Detect PR number from event payload.
- [ ] Detect base and head refs from event payload.
- [ ] Run `codesteward scan` in GitHub Actions.
- [ ] Post a new PR comment when no CodeSteward comment exists.
- [ ] Update the existing CodeSteward comment when marker exists.
- [ ] Avoid duplicate comments.
- [ ] Support dry-run mode.
- [ ] Support output-only mode.
- [ ] Document required permissions.
- [ ] Add integration tests where practical using mocked GitHub API calls.

## Example GitHub Workflow

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

      - uses: codesteward-ai/codesteward-action@v0
        with:
          comment: true
```

## GitHub Comment Behavior

```text
1. Generate Markdown report.
2. List existing PR comments.
3. Find comment containing <!-- codesteward-report -->.
4. If found, update it.
5. If not found, create it.
6. Never create duplicate CodeSteward comments on repeated pushes.
```

## Exit Criteria

- [ ] GitHub Action can run on pull requests.
- [ ] GitHub Action can generate a report.
- [ ] GitHub Action can post a PR comment.
- [ ] GitHub Action can update the existing CodeSteward comment.
- [ ] Duplicate comments are avoided.
- [ ] Required permissions are documented.
- [ ] GitHub usage is documented in `docs/github.md`.

---

# Phase 11 — GitLab Integration

## Goal

Support GitLab merge requests through a container image and single-note update behavior.

## Tasks

- [ ] Create GitLab CI template under `gitlab/codesteward.gitlab-ci.yml`.
- [ ] Build container image for CodeSteward.
- [ ] Detect GitLab CI environment.
- [ ] Detect merge request IID.
- [ ] Detect project ID.
- [ ] Detect base and head refs from GitLab CI variables.
- [ ] Run `codesteward scan` in GitLab CI.
- [ ] Post a new MR note when no CodeSteward note exists.
- [ ] Update existing CodeSteward note when marker exists.
- [ ] Avoid duplicate notes.
- [ ] Support dry-run mode.
- [ ] Support output-only mode.
- [ ] Document token requirements.
- [ ] Add integration tests where practical using mocked GitLab API calls.

## Example GitLab CI

```yaml
codesteward:
  image: ghcr.io/codesteward-ai/codesteward:v0.1.0
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
  script:
    - codesteward scan --comment
```

## GitLab Note Behavior

```text
1. Generate Markdown report.
2. List existing MR notes.
3. Find note containing <!-- codesteward-report -->.
4. If found, update it.
5. If not found, create it.
6. Never create duplicate CodeSteward notes on repeated pushes.
```

## Exit Criteria

- [ ] GitLab CI template can run on merge requests.
- [ ] GitLab integration can generate a report.
- [ ] GitLab integration can post an MR note.
- [ ] GitLab integration can update the existing CodeSteward note.
- [ ] Duplicate notes are avoided.
- [ ] Token requirements are documented.
- [ ] GitLab usage is documented in `docs/gitlab.md`.

---

# Phase 12 — TypeScript Demo Package

## Goal

Create a small TypeScript library example that demonstrates CodeSteward’s value clearly.

## Tasks

- [ ] Create `examples/typescript-package`.
- [ ] Add package structure.
- [ ] Add source files.
- [ ] Add test files.
- [ ] Add docs files.
- [ ] Add `package.json`.
- [ ] Add `.codesteward.yaml`.
- [ ] Add CODEOWNERS with intentional ownership gap.
- [ ] Create demo scenario scripts or fixture diffs.
- [ ] Add README section showing expected reports.
- [ ] Use this demo in docs and release materials.

## Suggested Structure

```text
examples/typescript-package/
  src/
    parser/
      tokenize.ts
      parse.ts
    runtime/
      cache.ts
    public/
      index.ts
  tests/
    parser/
      tokenize.test.ts
  docs/
    usage.md
  package.json
  CODEOWNERS
  .codesteward.yaml
```

## Example CODEOWNERS

```text
/src/parser/ @parser-maintainers
/src/public/ @api-maintainers
/docs/ @docs-maintainers
* @maintainers
```

`src/runtime/` is intentionally covered only by fallback ownership.

## Demo Scenario 1 — Good PR

Changed files:

```text
src/parser/tokenize.ts
tests/parser/tokenize.test.ts
```

Expected result:

```text
Ready for maintainer review
Ownership complete
Matching tests changed
Review burden low
```

## Demo Scenario 2 — Missing Tests and Weak Ownership

Changed files:

```text
src/runtime/cache.ts
```

Expected result:

```text
Needs contributor action
Ownership partial
Missing matching tests
Review burden medium or high
```

## Demo Scenario 3 — Broad PR

Changed files:

```text
src/parser/parse.ts
src/runtime/cache.ts
docs/usage.md
package.json
.github/workflows/release.yml
```

Expected result:

```text
High review burden
Touches multiple areas
Sensitive files changed
Consider splitting PR
```

## Exit Criteria

- [ ] Demo package exists.
- [ ] Demo package has meaningful CODEOWNERS.
- [ ] Demo package includes an intentional ownership gap.
- [ ] At least 3 demo scenarios exist.
- [ ] Demo reports clearly show the product value.
- [ ] README can explain the product using the demo in under 60 seconds.

---

# Phase 13 — Packaging, Distribution, and Release Automation

## Goal

Make CodeSteward easy to install locally and in CI.

## Tasks

- [ ] Build binaries for Linux, macOS, and Windows.
- [ ] Build amd64 and arm64 binaries where practical.
- [ ] Create release workflow.
- [ ] Add checksums.
- [ ] Publish container image.
- [ ] Publish GitHub Action version tag.
- [ ] Create install script.
- [ ] Document local installation.
- [ ] Document GitHub installation.
- [ ] Document GitLab installation.
- [ ] Add version command metadata.

## Initial Distribution Targets

```text
Local: install script
GitHub: composite action that downloads the binary
GitLab: container image first, install script second
```

## Later Distribution Targets

```text
Homebrew
asdf plugin
Scoop
Apt/yum repositories
```

## Exit Criteria

- [ ] Users can install CodeSteward locally.
- [ ] Users can run CodeSteward in GitHub Actions.
- [ ] Users can run CodeSteward in GitLab CI.
- [ ] Release binaries are published with checksums.
- [ ] Container image is published.
- [ ] Install instructions are documented and tested.

---

# Phase 14 — Documentation and Maintainer Experience

## Goal

Create documentation that makes the project easy for open-source maintainers to adopt.

## Tasks

- [ ] Write `docs/getting-started.md`.
- [ ] Write `docs/github.md`.
- [ ] Write `docs/gitlab.md`.
- [ ] Write `docs/config.md`.
- [ ] Write `docs/codeowners.md`.
- [ ] Write `docs/rules.md`.
- [ ] Write `docs/reports.md`.
- [ ] Add troubleshooting guide.
- [ ] Add FAQ.
- [ ] Add contribution guide.
- [ ] Add code of conduct.
- [ ] Add security policy.
- [ ] Add issue templates.
- [ ] Add PR template.

## README Must Include

- [ ] One-sentence product description.
- [ ] Screenshot or Markdown example of a CodeSteward comment.
- [ ] GitHub install snippet.
- [ ] GitLab install snippet.
- [ ] Local CLI snippet.
- [ ] Explanation of comment-only mode.
- [ ] Explanation that CodeSteward is deterministic and does not use AI in v0.
- [ ] Link to config docs.
- [ ] Link to CODEOWNERS docs.

## Exit Criteria

- [ ] A new maintainer can understand CodeSteward in under 2 minutes.
- [ ] A GitHub repo can install it from docs alone.
- [ ] A GitLab repo can install it from docs alone.
- [ ] Config examples are copy-pasteable.
- [ ] Troubleshooting covers shallow clone issues, missing tokens, and missing CODEOWNERS.

---

# Phase 15 — v0.1.0 Public Release

## Goal

Ship the first public release with the complete v0 product experience.

## Required Features

- [ ] Go CLI.
- [ ] Apache 2.0 license.
- [ ] Config support for `.codesteward.yaml` and `.codesteward.yml`.
- [ ] Git diff engine.
- [ ] CODEOWNERS parsing.
- [ ] Ownership analysis.
- [ ] Path-aware test expectation analysis.
- [ ] Scope rules.
- [ ] Description rules.
- [ ] Sensitive path rules.
- [ ] Internal scoring.
- [ ] Compact Markdown report.
- [ ] JSON report.
- [ ] GitHub PR comment posting.
- [ ] GitLab MR note posting.
- [ ] Single-comment update behavior.
- [ ] TypeScript demo package.
- [ ] Installation docs.

## Release Checklist

- [ ] All tests pass.
- [ ] Snapshot reports reviewed.
- [ ] GitHub integration tested on a real PR.
- [ ] GitLab integration tested on a real MR.
- [ ] Demo repo output matches README.
- [ ] Release binaries published.
- [ ] Container image published.
- [ ] GitHub Action tag published.
- [ ] Release notes written.
- [ ] Known limitations documented.

## Exit Criteria

- [ ] A public GitHub repository can install CodeSteward and receive a compact PR comment.
- [ ] A public GitLab repository can install CodeSteward and receive a compact MR note.
- [ ] The comment updates instead of duplicating.
- [ ] The report is deterministic.
- [ ] The report is helpful without being hostile.
- [ ] The first release is usable by open-source maintainers.

---

# Phase 16 — Post-v0.1 Feedback and Hardening

## Goal

Use real maintainer feedback to improve defaults, reduce false positives, and stabilize the config/report schema.

## Tasks

- [ ] Collect feedback from initial OSS users.
- [ ] Track confusing findings.
- [ ] Track noisy rules.
- [ ] Track missing useful rules.
- [ ] Improve default thresholds.
- [ ] Improve path-aware test mapping behavior.
- [ ] Improve CODEOWNERS parser edge cases.
- [ ] Improve GitHub/GitLab API error handling.
- [ ] Improve docs based on setup friction.
- [ ] Add more fixture repositories.
- [ ] Add compatibility tests for common project layouts.

## Metrics to Watch

```text
Number of installs
Number of repos using comment posting
Number of false-positive reports
Number of duplicate comment bugs
Number of config-related issues
Number of CODEOWNERS parsing issues
Number of maintainers who keep the bot installed after first week
```

## Exit Criteria

- [ ] Default rules are less noisy.
- [ ] Most setup failures have clear troubleshooting docs.
- [ ] CODEOWNERS behavior is reliable on real repositories.
- [ ] The config schema is ready to stabilize.
- [ ] Maintainers report that CodeSteward saves review time.

---

# Phase 17 — v0.2 Candidate Features

## Goal

Plan the next layer without bloating v0.1.

## Candidate Features

- [ ] Full CI artifact report.
- [ ] Auto-labeling as opt-in.
- [ ] Auto-reviewer suggestions as opt-in.
- [ ] Language-specific presets.
- [ ] TypeScript preset.
- [ ] Python preset.
- [ ] Rust preset.
- [ ] Go preset.
- [ ] Better public API change detection.
- [ ] Better monorepo ownership analysis.
- [ ] Maintainer workload summaries.
- [ ] Local pre-push usage mode.
- [ ] Comment customization.
- [ ] Rule severity customization.
- [ ] Ignore comments for specific findings.
- [ ] SARIF or other machine-readable export, if useful.

## Important Rule

Do not add v0.2 features until v0.1 users confirm the core report is useful.

## Exit Criteria

- [ ] v0.2 scope is based on real usage, not speculation.
- [ ] v0.1 remains simple and stable.
- [ ] The roadmap preserves CodeSteward’s core identity as maintainer-time protection tooling.

---

# Phase 18 — Future Commercial Layer

## Goal

Define monetization options that do not betray the open-source core.

## Free Forever

- [ ] Local CLI.
- [ ] GitHub public repo support.
- [ ] GitLab public repo support.
- [ ] CODEOWNERS analysis.
- [ ] PR/MR readiness comments.
- [ ] Path-aware test checks.
- [ ] JSON and Markdown reports.

## Paid Later

- [ ] Private repo hosted reporting.
- [ ] Cross-repo dashboard.
- [ ] Organization-level ownership map.
- [ ] Historical review burden trends.
- [ ] Maintainer workload analytics.
- [ ] Slack/Discord notifications.
- [ ] Policy templates.
- [ ] Managed GitHub/GitLab app.
- [ ] Priority support.
- [ ] Enterprise support.

## Exit Criteria

- [ ] Public OSS users are not paywalled from the core product.
- [ ] Companies have a reason to pay for hosted/private/org-level features.
- [ ] The open-source repo remains the growth engine.

---

# Implementation Priority Summary

## Build First

```text
1. CLI skeleton
2. Config loading
3. Git diff engine
4. CODEOWNERS parser
5. Ownership analysis
6. Path-aware test checks
7. Readiness scoring
8. Compact Markdown report
9. JSON report
10. GitHub/GitLab comment posting
```

## Build Later

```text
1. Full CI artifact reports
2. Auto-labeling
3. Auto-reviewer requests
4. Language-specific presets
5. Hosted dashboard
6. Historical analytics
7. Paid private-repo/org features
```

## Do Not Build Yet

```text
1. LLM review
2. AI-code detection
3. Security scanning
4. Enterprise governance
5. Contributor moderation
6. Blocking mode
```

---

# Definition of Done for v0.1.0

CodeSteward v0.1.0 is done when:

- [ ] A maintainer can install CodeSteward on GitHub in less than 10 minutes.
- [ ] A maintainer can install CodeSteward on GitLab in less than 10 minutes.
- [ ] A PR/MR receives one compact CodeSteward comment.
- [ ] The comment updates instead of duplicating.
- [ ] The comment identifies ownership gaps.
- [ ] The comment identifies fallback-only ownership as partial ownership.
- [ ] The comment identifies missing path-aware test updates.
- [ ] The comment identifies overly broad PR/MR scope.
- [ ] The comment warns on empty descriptions by default.
- [ ] The comment gives 3–5 clear action items.
- [ ] The Markdown report does not expose the numeric score.
- [ ] The JSON report exposes the numeric score.
- [ ] The tool does not block, label, assign reviewers, or moderate contributors.
- [ ] The tool is deterministic and uses no AI.
- [ ] The TypeScript demo package clearly demonstrates the product value.
- [ ] The project is released under Apache 2.0.
