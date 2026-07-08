# CodeSteward Reports

CodeSteward produces one normalized report per scan. That report renders two ways:

- **Markdown** — the compact, contributor-facing comment posted to a PR/MR (or printed to stdout). The numeric score is hidden by default.
- **JSON** — the full, stable report for automation. It includes every field, including the numeric `score`.

Choose the format with `--format markdown` (default) or `--format json`, and write to a file with `--output`.

## Markdown report anatomy

The Markdown renderer emits this exact structure. Below, each line is annotated.

```markdown
<!-- codesteward-report -->

## CodeSteward: <StatusDisplay>

**Review burden:** <Low|Medium|High>  
**Ownership:** <Complete|Partial|Missing|Not evaluated>  
**Tests:** <TestsDisplay>

<intro line>

### Before maintainer review

- <up to 5 action items>

<details>
<summary>Why CodeSteward flagged this</summary>

- <every finding Message, in canonical order>

</details>

_Comment-only mode. CodeSteward is not blocking this PR._
```

Line by line:

1. **`<!-- codesteward-report -->`** — the hidden marker (exported as `report.Marker`). CodeSteward searches existing comments/notes for this marker to update its own comment instead of posting a duplicate. Always the first line.
2. **Blank line.**
3. **`## CodeSteward: <StatusDisplay>`** — the heading. `StatusDisplay` is the human form of the readiness status:
   - `ready_for_maintainer_review` → `Ready for maintainer review`
   - `reviewable_with_notes` → `Reviewable with notes`
   - `needs_contributor_action` → `Needs contributor action`
   - `high_review_burden` → `High review burden`
   - `needs_owner_routing` → `Needs owner routing`
4. **Blank line.**
5. **`**Review burden:** <Low|Medium|High>`** — the burden level, capitalized. Ends with two trailing spaces (a Markdown hard break). Always rendered.
6. **`**Ownership:** <Complete|Partial|Missing|Not evaluated>`** — the ownership state, capitalized. Ends with two trailing spaces. Always rendered, even when `Not evaluated`.
7. **`**Tests:** <TestsDisplay>`** — the test state in human form. No trailing spaces (last line of the header block). `TestsDisplay` mapping:
   - `matching_test_changed` → `Present`
   - `missing_matching_test` → `Missing matching updates`
   - `existing_test_found_but_not_changed` → `Existing tests not updated`
   - `not_required` → `Not required`
   - `not_evaluated` → `Not evaluated`
8. **(Optional) `**Internal score:** <n>/100`** — inserted immediately after the Tests line **only** when `--show-score` is passed. Hidden by default.
9. **Blank line.**
10. **Intro line** — one sentence:
    - If there are no action items: `Thanks for the contribution. This looks ready for maintainer review.`
    - Otherwise: `Thanks for the contribution. A few changes would make this easier for maintainers to review.`
11. **`### Before maintainer review`** section — the action items. Each item is one finding's non-empty `action` field, in canonical finding order, deduplicated by exact string match, capped at **5**. The **entire section (heading included) is omitted** when there are no action items.
12. **`<details>` block** — a collapsed section titled `Why CodeSteward flagged this` listing **every** finding's `message` in canonical order. The **entire block is omitted** when there are no findings.
13. **`_Comment-only mode. CodeSteward is not blocking this PR._`** — the disclaimer. Always the last line, exactly as shown.

The two static header labels (`Review burden`, `Ownership`) and the disclaimer always render. The score line only appears with `--show-score`.

### Example (Scenario 2)

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

## JSON report schema

The JSON report is `json.MarshalIndent` with two-space indentation and a trailing newline. Field **names are snake_case** and field **order is fixed** (it is part of the stable schema). The `score` field is JSON-only — it never appears in the default Markdown comment (see [note below](#the-score-is-json-only)).

Top-level object (`Report`):

| Field | Type | Notes |
|---|---|---|
| `schema_version` | string | Always `"0.1"` in v0. |
| `tool` | object | Tool identity — see below. |
| `base` | string | Resolved base ref. |
| `head` | string | Resolved head ref. |
| `comment_only` | bool | Always `true` in v0. |
| `status` | string (enum) | One of the readiness statuses. |
| `score` | int | 0–100 internal score. Present in JSON only. |
| `review_burden` | string (enum) | `low` \| `medium` \| `high`. |
| `ownership` | object | Ownership summary. |
| `tests` | object | Tests summary. |
| `scope` | object | Scope summary. |
| `description` | object | Description summary. |
| `findings` | array | Findings in canonical order. |
| `exit_criteria` | array | One per action-required finding. |
| `warnings` | array of string | Non-fatal scan warnings; **omitted when empty**. |

`status` enum values: `ready_for_maintainer_review`, `reviewable_with_notes`, `needs_contributor_action`, `high_review_burden`, `needs_owner_routing`.

### `tool`

| Field | Type | Notes |
|---|---|---|
| `name` | string | Tool name (`codesteward`). |
| `version` | string | Tool version (e.g. `0.1.0-dev`). |

### `ownership`

| Field | Type | Notes |
|---|---|---|
| `state` | string (enum) | `complete` \| `partial` \| `missing` \| `not_evaluated`. |
| `areas_touched` | int | Distinct ownership areas touched. |
| `max_areas` | int | Configured `max_ownership_areas`. |
| `files` | array | Per-file ownership; **omitted when empty**. |

Each entry in `ownership.files` (`FileOwnership`):

| Field | Type | Notes |
|---|---|---|
| `path` | string | Repo-relative path. |
| `owners` | array of string | Matched owners; **omitted when empty**. |
| `pattern` | string | Matching CODEOWNERS pattern; **omitted when empty**. |
| `class` | string (enum) | `specific` \| `broad` \| `fallback` \| `missing`. |

### `tests`

| Field | Type | Notes |
|---|---|---|
| `state` | string (enum) | `not_required` \| `matching_test_changed` \| `existing_test_found_but_not_changed` \| `missing_matching_test` \| `not_evaluated`. |
| `files` | array | Per-file test expectations; **omitted when empty**. |

Each entry in `tests.files` (`FileTestExpectation`):

| Field | Type | Notes |
|---|---|---|
| `path` | string | Repo-relative production path. |
| `state` | string (enum) | Same enum as `tests.state`, per file. |
| `candidates` | array of string | Expected test paths; **omitted when empty**. |
| `matched_test` | string | The matched test path; **omitted when empty**. |

### `scope`

| Field | Type | Notes |
|---|---|---|
| `files_changed` | int | Files counted toward limits (ignored files excluded). |
| `lines_added` | int | Total additions. |
| `lines_deleted` | int | Total deletions. |
| `top_level_areas` | array of string | Top-level areas touched (root files use `(root)`); **omitted when empty**. |
| `max_files_changed` | int | Configured limit. |
| `max_lines_changed` | int | Configured limit. |
| `exceeds_file_limit` | bool | Whether `files_changed` exceeded the limit. |
| `exceeds_line_limit` | bool | Whether lines changed exceeded the limit. |

### `description`

| Field | Type | Notes |
|---|---|---|
| `provided` | bool | Whether a non-empty description was present. |
| `length` | int | Rune length of the trimmed description. |
| `evaluated` | bool | Whether the description was evaluated at all (false suppresses all description findings). |

### `findings`

Each entry (`Finding`), in canonical order:

| Field | Type | Notes |
|---|---|---|
| `rule_id` | string | Rule ID (e.g. `CS-TST-001`). |
| `severity` | string (enum) | `info` \| `warning` \| `action_required`. |
| `message` | string | Past-tense fact, shown in the details section. |
| `action` | string | Imperative action item; **omitted when empty**. |
| `paths` | array of string | Sorted paths; **omitted when empty**. |

### `exit_criteria`

Each entry (`ExitCriterion`):

| Field | Type | Notes |
|---|---|---|
| `rule_id` | string | Rule ID of the source finding. |
| `description` | string | The condition that clears it (the finding's action). |

## The score is JSON-only

The numeric `score` field is always present in the JSON report but is **hidden from the Markdown comment by default**. This keeps the contributor-facing comment focused on actions rather than a number. To reveal the score in Markdown, pass `--show-score`, which adds an `**Internal score:** <n>/100` line after the Tests line. The JSON output is unaffected by `--show-score` — it always contains `score`.

## Complete example JSON

A realistic JSON report for the demo Scenario 2 (change to `src/runtime/cache.ts`, empty description). The `warnings` field is omitted because there were none.

```json
{
  "schema_version": "0.1",
  "tool": {
    "name": "codesteward",
    "version": "0.1.0-dev"
  },
  "base": "main",
  "head": "HEAD",
  "comment_only": true,
  "status": "needs_contributor_action",
  "score": 60,
  "review_burden": "medium",
  "ownership": {
    "state": "partial",
    "areas_touched": 1,
    "max_areas": 2,
    "files": [
      {
        "path": "src/runtime/cache.ts",
        "owners": [
          "@maintainers"
        ],
        "pattern": "*",
        "class": "fallback"
      }
    ]
  },
  "tests": {
    "state": "missing_matching_test",
    "files": [
      {
        "path": "src/runtime/cache.ts",
        "state": "missing_matching_test",
        "candidates": [
          "src/runtime/cache.spec.ts",
          "src/runtime/cache.test.ts",
          "tests/runtime/cache.spec.ts",
          "tests/runtime/cache.test.ts"
        ]
      }
    ]
  },
  "scope": {
    "files_changed": 1,
    "lines_added": 40,
    "lines_deleted": 5,
    "top_level_areas": [
      "src"
    ],
    "max_files_changed": 12,
    "max_lines_changed": 500,
    "exceeds_file_limit": false,
    "exceeds_line_limit": false
  },
  "description": {
    "provided": false,
    "length": 0,
    "evaluated": true
  },
  "findings": [
    {
      "rule_id": "CS-TST-001",
      "severity": "action_required",
      "message": "`src/runtime/cache.ts` changed, but no matching test file was changed.",
      "action": "Add or update matching tests for `src/runtime/cache.ts`.",
      "paths": [
        "src/runtime/cache.ts"
      ]
    },
    {
      "rule_id": "CS-DSC-001",
      "severity": "warning",
      "message": "The PR description is empty.",
      "action": "Add a short description explaining the motivation and test plan."
    },
    {
      "rule_id": "CS-OWN-002",
      "severity": "warning",
      "message": "`src/runtime/cache.ts` is covered only by fallback ownership: `* @maintainers`.",
      "action": "Add specific ownership for `src/runtime/**` or ask a maintainer to route this area.",
      "paths": [
        "src/runtime/cache.ts"
      ]
    }
  ],
  "exit_criteria": [
    {
      "rule_id": "CS-TST-001",
      "description": "Add or update matching tests for `src/runtime/cache.ts`."
    }
  ]
}
```
