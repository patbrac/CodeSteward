# example-typescript-package

A tiny TypeScript library used to demonstrate what CodeSteward does on real
pull requests. It is intentionally small — a tokenizer, a recursive-descent
expression parser, and a least-recently-used cache — so the interesting part is
the *ownership and review-readiness signal*, not the code.

```text
src/
  parser/
    tokenize.ts   # owned by @parser-maintainers
    parse.ts      # owned by @parser-maintainers
  runtime/
    cache.ts      # NO specific owner — only the "* @maintainers" fallback
  public/
    index.ts      # owned by @api-maintainers
tests/
  parser/
    tokenize.test.ts
docs/
  usage.md        # owned by @docs-maintainers
package.json
CODEOWNERS
.codesteward.yaml
```

## The intentional ownership gap

The `CODEOWNERS` file opens with a `* @maintainers` catch-all and then gives
specific owners to `src/parser/`, `src/public/`, and `docs/`:

```text
* @maintainers
/src/parser/ @parser-maintainers
/src/public/ @api-maintainers
/docs/ @docs-maintainers
```

Order matters: GitHub CODEOWNERS is last-match-wins, so the catch-all is listed
**first** and the specific rules below it override it for their areas.
`src/runtime/` has **no specific owner**, so any change there is covered only by
the fallback rule. CodeSteward treats fallback-only coverage as *partial*
ownership — that gap is what scenarios 2 and 3 exercise.

## Running the scenarios

Each script copies this example into a throwaway git repo, applies a branch of
edits, and runs `codesteward scan` against it. Point `CODESTEWARD_BIN` at your
built binary (it defaults to `codesteward` on your `PATH`):

```sh
CODESTEWARD_BIN=../../bin/codesteward ./scenarios/scenario-1-good-pr.sh
CODESTEWARD_BIN=../../bin/codesteward ./scenarios/scenario-2-missing-tests.sh
CODESTEWARD_BIN=../../bin/codesteward ./scenarios/scenario-3-broad-pr.sh
```

The temporary repo is removed automatically when the script exits.

### Scenario 1 — Good PR

Changes `src/parser/tokenize.ts` and its matching test
`tests/parser/tokenize.test.ts`, with a real (>= 80 character) PR description.

| Signal | Result |
|---|---|
| Status | Ready for maintainer review |
| Ownership | Complete |
| Tests | Present (matching test changed) |
| Review burden | Low |
| Findings | none — internal score 100 |

The parser file has a specific owner and its matching test was updated in the
same PR, so there is nothing for a maintainer to chase.

### Scenario 2 — Missing tests and weak ownership

Changes only `src/runtime/cache.ts`, with an empty description.

| Signal | Result |
|---|---|
| Status | Needs contributor action |
| Ownership | Partial |
| Tests | Missing matching updates |
| Review burden | Medium |
| Findings | `CS-OWN-002` (fallback-only), `CS-TST-001` (no matching test), `CS-DSC-001` (empty description) — internal score 60 |

`src/runtime/` is covered only by the `* @maintainers` fallback, no matching
test file exists or was changed, and the description is empty.

### Scenario 3 — Broad PR

Changes `src/parser/parse.ts`, `src/runtime/cache.ts`, `docs/usage.md`,
`package.json`, and `.github/workflows/release.yml` together, with an empty
description.

| Signal | Result |
|---|---|
| Status | High review burden |
| Review burden | High |
| Findings | `CS-OWN-002`, `CS-TST-001`, `CS-SCP-003` (source + dependency manifest), `CS-SCP-004` (source + docs + config/CI), `CS-DSC-001`, `CS-SNS-002` (CI workflow), `CS-SNS-003` (package manifest) — internal score 15 |

One PR touches parser code, unowned runtime code, docs, the package manifest,
and a CI release workflow at once. CodeSteward flags the sensitive manifest and
workflow changes, the mixed concerns, the missing tests, the fallback-only
ownership, and the empty description, and asks the contributor to split the
change and add context.

> CodeSteward runs in comment-only mode: it never blocks the PR, it just tells
> the contributor what would make review easier.
