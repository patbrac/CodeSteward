# Configuration Reference

CodeSteward reads project configuration from a YAML file at the repository root.
It works with zero configuration using safe built-in defaults, so a config file
is optional — but a small config tailors production paths, thresholds, and test
mappings to your project.

## File discovery order

CodeSteward looks for its config in this order and uses the **first** match:

1. The path passed with `--config <path>` (if the file is missing or unreadable,
   this is a fatal error).
2. `<repo-root>/.codesteward.yaml`
3. `<repo-root>/.codesteward.yml`
4. Otherwise: built-in defaults, with the stderr warning
   `no config file found; using built-in defaults`.

`.codesteward.yaml` is preferred over `.codesteward.yml` when both exist.

## Merge semantics

A loaded config is **merged over the defaults**:

- Keys you omit keep their default values.
- A key you set to an **empty list** overrides the default to empty (for
  example, `required_sections: []` and `sensitive_paths: []` both mean "none").
- **Unknown keys are warnings, not errors** — CodeSteward reports them and
  continues, so a typo never breaks a scan.

## Validate your config

```bash
codesteward config validate
codesteward config validate --config path/to/.codesteward.yaml
codesteward config validate --repo-root /path/to/repo
```

Validation reports two kinds of problems: `error:` entries (negative
thresholds, invalid path-mapping placeholders, unknown dialect) are **fatal**
and make the command exit `1`; `warning:` entries (such as unknown keys and
invalid globs) are informational and do not fail the command.

## Full example

A complete config equal to the built-in defaults:

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
  dialect: auto
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

Below, each section documents every key, its type, default, and meaning.

---

## `project`

Project metadata.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `name` | string | `""` | Human-readable project name. Informational only. |

```yaml
project:
  name: my-library
```

---

## `mode`

Operating mode.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `comment_only` | bool | `true` | CodeSteward is comment-only in v0. This is always effectively on — CodeSteward never blocks, labels, or assigns. The flag is reserved and surfaced in the report footer. |

```yaml
mode:
  comment_only: true
```

---

## `review_readiness`

Scope thresholds that drive scope findings and review-burden estimation.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `max_files_changed` | int | `12` | Above this many changed files, fire `CS-SCP-001` (too many files). |
| `max_lines_changed` | int | `500` | Above this many changed lines (additions + deletions), fire `CS-SCP-002` (too many lines). |
| `max_ownership_areas` | int | `2` | Above this many distinct ownership areas touched, fire `CS-OWN-003` (too many ownership areas). |

All three must be non-negative; a negative value is a validation error.

```yaml
review_readiness:
  max_files_changed: 20
  max_lines_changed: 800
  max_ownership_areas: 3
```

---

## `ownership`

CODEOWNERS-based ownership analysis. See [CODEOWNERS support](codeowners.md).

| Key | Type | Default | Meaning |
|---|---|---|---|
| `use_codeowners` | bool | `true` | Enable ownership analysis. When false (or no CODEOWNERS file is found), ownership is reported as `not_evaluated`. |
| `dialect` | string | `auto` | CODEOWNERS dialect: `github`, `gitlab`, or `auto`. Controls discovery locations and parsing. An unknown value is a validation error. |
| `production_paths` | list of globs | `[src/**, lib/**, packages/**]` | Globs identifying production source files that require ownership and tests. |
| `ignore_paths` | list of globs | `[docs/**, examples/**, README.md]` | Globs for files excluded from ownership and test expectations. |

```yaml
ownership:
  use_codeowners: true
  dialect: auto
  production_paths:
    - src/**
    - lib/**
    - packages/**
  ignore_paths:
    - docs/**
    - examples/**
    - README.md
```

A file matching `test_paths` is treated as a test and is never counted as
production, even if it also matches `production_paths`.

---

## `tests`

Path-aware test expectation engine. See the demo mapping below.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `require_for` | list of globs | `[src/**, lib/**, packages/**]` | Changed files matching these globs are expected to have matching test updates. |
| `test_paths` | list of globs | `[tests/**, test/**, **/*.test.*, **/*.spec.*]` | Globs identifying test files. |
| `path_mappings` | list of mappings | see below | Rules mapping a production file to its expected test candidates. |

Each `path_mappings` entry has:

| Key | Type | Meaning |
|---|---|---|
| `from` | string | Source pattern with `{path}`, `{name}`, `{ext}` placeholders. |
| `expect` | list of strings | Expected test candidate patterns using the same placeholders. |

Placeholder semantics:

- `{path}` — the directory path after the literal prefix in `from` (may be
  empty). `src/{path}/{name}.{ext}` matches both `src/a.ts` (with `path=""`) and
  `src/x/y/a.ts` (with `path="x/y"`). When `path` is empty, double slashes in
  expansions collapse.
- `{name}` — the basename without its final extension.
- `{ext}` — the final extension, without the dot.

Default mapping:

```yaml
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
```

For example, changing `src/parser/tokenize.ts` expects one of:

```text
tests/parser/tokenize.test.ts
tests/parser/tokenize.spec.ts
src/parser/tokenize.test.ts
src/parser/tokenize.spec.ts
```

Invalid placeholders in a mapping (anything other than `{path}`, `{name}`,
`{ext}`) are a validation error.

---

## `pr_description`

PR/MR description quality checks. These are only evaluated when a description
source is available (a provider always supplies one; a local run supplies one
only via `--description` or `--description-file`).

| Key | Type | Default | Meaning |
|---|---|---|---|
| `warn_if_empty` | bool | `true` | Warn (`CS-DSC-001`) when the description is empty/whitespace. |
| `min_length` | int | `80` | Warn (`CS-DSC-002`) when a non-empty description is shorter than this many characters. |
| `required_sections` | list of strings | `[]` | Section names that must appear as a heading or bold line. Each missing one fires `CS-DSC-003`. Empty means no section requirement. |
| `require_linked_issue` | bool | `false` | When true, warn (`CS-DSC-004`) if no `#123` or `/issues/123` reference is present. |

```yaml
pr_description:
  warn_if_empty: true
  min_length: 120
  required_sections:
    - Summary
    - Test plan
  require_linked_issue: true
```

---

## `sensitive_paths`

Additional globs treated as sensitive, beyond the built-in lockfile, CI/release,
and manifest sets. A change to any of these fires a sensitive-path finding so
maintainers can verify it.

| Key | Type | Default |
|---|---|---|
| `sensitive_paths` | list of globs | `[package.json, package-lock.json, pnpm-lock.yaml, yarn.lock, .github/workflows/**, .gitlab-ci.yml, scripts/release/**]` |

```yaml
sensitive_paths:
  - package.json
  - package-lock.json
  - pnpm-lock.yaml
  - yarn.lock
  - .github/workflows/**
  - .gitlab-ci.yml
  - scripts/release/**
  - deploy/**
```

Set to an empty list to add no extra sensitive paths (the built-in sets still
apply):

```yaml
sensitive_paths: []
```

---

## Glob syntax

All path lists use doublestar globs, matched against slash-separated,
repo-root-relative paths:

- `*` matches any run of non-separator characters (may be empty).
- `?` matches exactly one non-separator character.
- `**` as a full segment matches zero or more path segments.
- `dir/**` matches everything under `dir` (but not the bare `dir` itself).
- A pattern with no slash (`*.md`, `package.json`) matches the path's basename
  **and** the whole path.
- A pattern ending in `/` matches the directory and everything under it.

An invalid glob anywhere in the config is reported as a validation **warning**
by `codesteward config validate` — it is surfaced but does not fail the command.

## See also

- [Getting started](getting-started.md)
- [CODEOWNERS support](codeowners.md)
- [Troubleshooting](troubleshooting.md)
