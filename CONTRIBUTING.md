# Contributing to CodeSteward

Thanks for your interest in CodeSteward — a deterministic, Apache-licensed PR/MR
review-readiness bot for open-source maintainers. This guide covers local
setup, the project layout, how to add a rule, and what we expect in a pull
request.

By participating you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## Guiding principles

Keep these in mind for every change:

- **Deterministic.** Identical inputs must produce byte-identical output. Never
  range over a map where order affects output — sort keys first. No timestamps,
  random values, absolute paths, or environment-dependent text in reports.
- **Minimal dependencies.** The Go standard library plus `gopkg.in/yaml.v3` are
  the *only* allowed dependencies. Do not add modules or run `go get`.
- **Comment-only, no AI.** v0 does not block, label, assign reviewers, moderate
  contributors, or use any AI/LLM. Features on the non-goals list stay out.
- **Helpful, not hostile.** Findings and action items are written to help
  contributors, in factual past tense (messages) and clear imperative (actions).

## Development setup

You need Go 1.24 or newer and `git` on your `PATH`.

```bash
git clone https://github.com/codesteward-ai/codesteward.git
cd codesteward

# Build everything
go build ./...

# Build the CLI binary
go build -o bin/codesteward ./cmd/codesteward

# Run the full test suite
go test ./...

# Format check (must print nothing)
gofmt -l .

# Vet
go vet ./...
```

Before opening a PR, make sure `gofmt -l .` prints nothing, `go vet ./...`
passes, and `go test ./...` passes.

## Project layout

```text
cmd/codesteward/        thin main; calls internal/cli.Main
internal/cli/           command dispatch, flags, exit codes
internal/config/        config load/validate/defaults
internal/git/           repo detection, ref resolution
internal/diff/          changed-file collection + classification
internal/globs/         ** glob matching (foundation)
internal/codeowners/    CODEOWNERS discovery/parse/match/validate
internal/ownership/     ownership analysis + audit
internal/tests/         path-aware test expectation engine
internal/rules/         scope, description, sensitive-path rules
internal/readiness/     scoring, status/burden mapping, report assembly
internal/report/        markdown + JSON renderers
internal/providers/     github + gitlab env detection and comment/note posting
internal/version/       version metadata (set via ldflags)
pkg/model/              shared, stable data model (types, rule IDs)
pkg/engine/             orchestration: Options -> *model.Report
docs/                   user documentation
examples/               TypeScript demo package and fixtures
```

Most implementation lives under `internal/` until the public API stabilizes.
`pkg/model` (shared types and rule IDs) and `pkg/engine` (orchestration) are the
only public packages, and are kept small and conservative.

## How the pipeline fits together

A scan flows through `pkg/engine.Run`:

```text
detect repo -> load config -> resolve refs -> diff.Collect -> diff.Classify
  -> codeowners Discover/Parse -> ownership.Analyze -> tests.Analyze
  -> rules.Scope/Description/Sensitive -> readiness.BuildReport
  -> report.RenderMarkdown / RenderJSON
```

Each rule produces one or more `Finding`s with a rule ID, severity, past-tense
message, and imperative action. `readiness` turns findings into a score, status,
and review burden, then assembles the normalized `model.Report`.

## How to add a rule

Rules are the core of CodeSteward. Adding one is a deliberate, four-part change:

1. **Pick a rule ID and penalty.** IDs follow the `CS-<CATEGORY>-<NNN>` scheme
   (`OWN` ownership, `TST` tests, `SCP` scope, `DSC` description, `SNS`
   sensitive paths). Add the constant to the rule-ID list in `pkg/model` and
   choose a severity (`info`, `warning`, `action_required`) and an integer
   score penalty. Scoring subtracts each distinct rule ID's penalty at most
   once.

2. **Emit the finding.** Add the detection logic to the package that owns the
   category (`internal/ownership`, `internal/tests`, or `internal/rules`).
   Write the message in factual past tense and the action as a clear
   imperative. Sort any `Paths` on the finding. Keep everything deterministic.

3. **Document it.** Add the rule to `docs/rules.md` (ID, severity, penalty, when
   it fires, and the message/action templates), and update any related config
   documentation in `docs/config.md`.

4. **Test it.** Add table-driven unit tests with `testdata/` fixtures in the
   emitting package, and update the renderer/readiness snapshot tests if the
   rule affects representative reports. Cover both the firing and non-firing
   cases.

Because scoring, status mapping, and the reference scenarios are contractual,
new rules should not silently shift the reference-scenario outputs — if they do,
call it out explicitly in your PR.

## Pull request expectations

- **One focused change per PR.** Split unrelated changes.
- **Tests included.** New behavior needs table-driven tests with `testdata/`
  fixtures; bug fixes need a regression test.
- **Green checks.** `gofmt -l .` prints nothing, `go vet ./...` passes,
  `go test ./...` passes.
- **Determinism preserved.** No new nondeterminism (map iteration order,
  timestamps, randomness, absolute paths).
- **Docs updated.** User-visible changes update the relevant file under `docs/`.
- **Clear description.** Explain the motivation and a test plan. Link the issue
  you are addressing.
- **Scope respected.** Do not add dependencies or reach for v0 non-goals
  (AI review, security scanning, blocking, labeling, reviewer assignment,
  moderation).

## Reporting bugs and requesting features

Use the issue templates:

- **Bug report** — steps to reproduce, expected vs. actual, environment.
- **Feature request** — the problem, proposed solution, alternatives.

For security issues, do **not** open a public issue — follow the
[security policy](SECURITY.md).

## License

By contributing, you agree that your contributions are licensed under the
Apache License 2.0, the same license as the project.
