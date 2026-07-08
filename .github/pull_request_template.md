<!--
Thanks for contributing to CodeSteward! Please fill out the sections below.
Keep changes focused and deterministic. See CONTRIBUTING.md.
-->

## Summary

<!-- What does this PR change, and why? Link the issue it addresses, e.g. Closes #123. -->

## Test plan

<!-- How did you verify this change? List commands and results. -->

- [ ] `gofmt -l .` prints nothing
- [ ] `go vet ./...` passes
- [ ] `go test ./...` passes
- [ ] Added or updated table-driven tests with `testdata/` fixtures

## Checklist

- [ ] The change is deterministic (no map-order dependence, timestamps, randomness, or absolute paths in output).
- [ ] No new dependencies (stdlib + `gopkg.in/yaml.v3` only; did not run `go get`).
- [ ] Documentation updated under `docs/` for any user-visible change.
- [ ] If a rule was added or changed: rule ID, penalty, docs, and tests are all updated.
- [ ] Reference scenarios still hold (or intentional changes are called out below).
- [ ] This change stays within v0 scope (no AI review, security scanning, blocking, auto-labeling, auto-reviewer assignment, or moderation).

## Notes for reviewers

<!-- Anything reviewers should pay special attention to, or intentional deviations from expected output. -->
