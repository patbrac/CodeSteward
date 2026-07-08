# CODEOWNERS Support

CodeSteward reads your repository's `CODEOWNERS` file to decide whether the
production files a PR/MR touches have clear ownership. It supports both GitHub
and GitLab dialects and classifies each match as **specific**, **broad**, or
**fallback**.

## Discovery locations

CodeSteward searches for a `CODEOWNERS` file in dialect-specific locations, using
the first that exists. Set the dialect with `ownership.dialect` in your config
(default `auto`).

| Dialect | Search order |
|---|---|
| `github` | `.github/CODEOWNERS`, then `CODEOWNERS`, then `docs/CODEOWNERS` |
| `gitlab` | `CODEOWNERS`, then `docs/CODEOWNERS`, then `.gitlab/CODEOWNERS` |
| `auto` | union in order: `.github/CODEOWNERS`, root `CODEOWNERS`, `docs/CODEOWNERS`, `.gitlab/CODEOWNERS` |

If no CODEOWNERS file exists, discovery returns nothing (not an error). Ownership
is then reported as `missing` if production files were changed, or
`not_evaluated` if none were.

## Dialect auto-detection

With `dialect: auto` (the default), CodeSteward picks parsing semantics from
where the file was found:

- Found under `.github/` → **GitHub** semantics.
- Found under `.gitlab/` → **GitLab** semantics.
- Found elsewhere (root or `docs/`) → GitHub semantics, but GitLab
  `[Section]` headers are still parsed leniently (treated as sections, with a
  warning).

## Supported syntax

- **Comments and blank lines** — lines starting with `#` and empty lines are
  ignored.
- **Path patterns** — gitignore-style. A leading `/` anchors to the repository
  root; a pattern with no slash (other than a trailing one) matches a basename
  anywhere; a trailing `/` matches the directory's contents; `*` and `**` behave
  as globs.
- **Multiple owners per rule** — space-separated after the pattern.
- **Owner forms** — `@username`, `@org/team`, and bare email addresses.
- **Rules with no owners** — treated as explicitly unowned (the pattern still
  matches, but the file is reported as having no owner).

Unsupported syntax (for example `!` negation) and malformed owners produce
validation warnings, not scan failures.

## Last-match-wins

For GitHub-style rules, the **last** matching rule in the file wins — exactly
like GitHub. Order your CODEOWNERS from most general to most specific:

```text
*                 @maintainers
/src/**           @core-team
/src/parser/**    @parser-maintainers
```

Here a change to `src/parser/tokenize.ts` is owned by `@parser-maintainers`
(the last match), and a change to `src/runtime/cache.ts` is owned by
`@core-team`.

## GitLab sections

GitLab groups rules under `[Section]` headers. CodeSteward parses them at a
basic level:

- Within each section, last-match-wins applies.
- The owners for a path are the **union** across all sections that match it,
  sorted and deduplicated.
- Optional sections (`^[Section]`) are parsed but not used for v0 scoring.

```text
[Backend]
/src/**  @backend-team

[Docs]
/docs/**  @docs-team
```

## Ownership classification

Every matched pattern is classified into one of three classes. This is what
drives the Ownership summary (`Complete` / `Partial` / `Missing`).

| Class | Definition | Examples |
|---|---|---|
| **specific** | Two or more concrete path segments — the pattern targets a precise area. | `/src/parser/**`, `/src/public/index.ts`, `packages/api/**` |
| **broad** | A single anchored top-level segment, or a bare extension pattern. | `/src/**`, `/src/`, `src/`, `*.md`, `*.js` |
| **fallback** | A catch-all that matches everything. | `*`, `**`, `/**`, `/` |
| **missing** | No rule matched the path at all. | — |

### Fallback is treated as partial ownership

A file covered **only** by a fallback (catch-all) rule like `* @maintainers` is
not considered properly owned. CodeSteward treats fallback-only coverage as
**partial** ownership and emits `CS-OWN-002`, suggesting you add specific
ownership for that area. This is intentional: a catch-all does not tell
maintainers who actually knows a given subsystem.

### How the Ownership summary is computed

Across the changed production files:

- **Missing** — at least one relevant production file has no owner (matched no
  rule, or matched a rule with no owners).
- **Partial** — no file is unowned, but at least one is covered only by a
  fallback rule.
- **Complete** — every relevant production file is covered by a specific or
  broad rule.

Broad and specific both count as "covered"; only fallback-only coverage
downgrades the summary to partial.

### Ownership areas

CodeSteward also counts distinct **ownership areas** touched — the set of
matched rule patterns among production files, plus a synthetic
`unowned:<top-level-dir>` area for each unowned file. When the count exceeds
`review_readiness.max_ownership_areas` (default 2), it fires `CS-OWN-003`,
suggesting the change be split so each PR touches fewer areas.

## Validate your CODEOWNERS

```bash
codesteward codeowners validate
codesteward codeowners validate --repo-root /path/to/repo
codesteward codeowners validate --dialect github
codesteward codeowners validate --dialect gitlab
codesteward codeowners validate --dialect auto
```

Validation reports:

- invalid or unparseable lines,
- empty owner lists (warning),
- malformed owners (not `@user`, `@org/team`, or an email address),
- unsupported syntax such as `!` negation,
- section headers when the dialect is `github`,
- a catch-all rule ordered **after** a more-specific rule (warning): because
  the last matching rule wins, the catch-all silently overrides the earlier,
  more-specific rules — move it first. For the `github` dialect this is checked
  across the whole file; for `gitlab` it is checked within each section.

## Example

The demo package uses this CODEOWNERS, with `src/runtime/` intentionally left to
fallback-only ownership. The catch-all is listed **first** because the last
matching rule wins, so the specific rules below override it for their areas:

```text
*             @maintainers
/src/parser/  @parser-maintainers
/src/public/  @api-maintainers
/docs/        @docs-maintainers
```

- `src/parser/tokenize.ts` → specific (`@parser-maintainers`) → complete.
- `src/public/index.ts` → specific (`@api-maintainers`) → complete.
- `src/runtime/cache.ts` → fallback only (`@maintainers`) → partial,
  `CS-OWN-002`.

## See also

- [Configuration reference](config.md) — `ownership.*` keys
- [Getting started](getting-started.md)
- [Troubleshooting](troubleshooting.md)
