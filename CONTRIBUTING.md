# Contributing to Code Steward

Thank you for improving Code Steward. Contributions can be code, tests, fixtures, documentation, reproducible bug reports, design review, or cross-platform verification. Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

Do not put vulnerabilities, secrets, private source, personal data, or non-public repository history in an issue or pull request. Report suspected vulnerabilities through the private process in [SECURITY.md](SECURITY.md).

## Choose a contribution

For a small correction, open a focused pull request. For behavior changes, first open an issue describing the user problem, deterministic evidence, and compatibility impact. Durable architecture/schema changes require an ADR; public-contract, governance, license, or major roadmap changes require an RFC. [GOVERNANCE.md](GOVERNANCE.md) explains the boundary.

Use only synthetic or properly licensed fixtures. Do not contribute employer or customer code, repository history, credentials, identities, or reports without documented authorization.

## Developer Certificate of Origin

Code Steward uses the [Developer Certificate of Origin 1.1](https://developercertificate.org/) (DCO) instead of a broad Contributor License Agreement. Every human-authored, non-merge commit must include a sign-off certifying that you have the right to submit it under the project's license:

```text
Signed-off-by: Your Name <your.email@example.com>
```

Create it with:

```sh
git commit --signoff
```

The name must be a name by which you can be identified, and the email must match an address associated with the commit. A pull-request comment or checkbox does not replace commit sign-off. To repair your own most recent commit, use `git commit --amend --signoff`; coordinate with reviewers before rewriting a shared branch.

By signing off, you certify the DCO. You retain your copyright, and accepted contributions are licensed under Apache-2.0. Do not add a different license header or third-party code unless its provenance and license have been reviewed.

The `dco` pull-request check verifies each contributed non-merge commit from the pull request's merge base through its head. Its workflow and checker are loaded from the protected base commit; pull-request files are never executed by that job. A matching author or committer sign-off is accepted. Hosting-generated merge commits are checked through their individual contributing commits instead, and only the exact GitHub identities for Dependabot and GitHub Actions are exempt; a name ending in `[bot]` is not an exemption.

## Native development setup

Follow the [clean-clone prerequisites and build](README.md#prerequisites). Work in a native checkout on Windows, macOS, or Linux; WSL and containers are useful additional environments but are not evidence of native Windows behavior.

POSIX shell (Linux or macOS):

```sh
rustup component add rustfmt clippy
cargo fmt --all -- --check
cargo clippy --workspace --all-targets --all-features -- -D warnings
cargo build --workspace --locked
cargo test --workspace --locked
cargo doc --workspace --no-deps
```

PowerShell (native Windows):

```powershell
rustup component add rustfmt clippy
cargo fmt --all -- --check
cargo clippy --workspace --all-targets --all-features -- -D warnings
cargo build --workspace --locked
cargo test --workspace --locked
cargo doc --workspace --no-deps
```

These commands operate on the finite workspace. Initial dependency resolution may use the network; checked-in commands and normal Code Steward CLI operations must not require it. After dependencies are cached, an offline check can be run as follows.

POSIX:

```sh
CARGO_NET_OFFLINE=true cargo test --workspace --locked
```

PowerShell:

```powershell
$env:CARGO_NET_OFFLINE = "true"
cargo test --workspace --locked
Remove-Item Env:CARGO_NET_OFFLINE
```

Schema changes must include valid and invalid examples and pass the validation command documented in the affected schema directory. Dependency changes must follow [the dependency policy](docs/security/dependency-policy.md), include the lockfile, and explain new runtime, license, and supply-chain consequences.

## Engineering expectations

- Treat repository paths, filenames, Git metadata/configuration, parser input, source text, and plugin messages as hostile data.
- Do not add implicit execution of hooks, filters, helpers, build tools, package managers, interpreters, project binaries, or repository plugins.
- Keep default commands network-silent. New network or source-transmission capabilities require explicit user action, a security review, and clear documentation.
- Preserve deterministic ordering, identifiers, evidence, and exit behavior across native platforms. Do not derive stable output from absolute machine paths, locale, wall-clock time, hash-map iteration, or platform separators.
- Bound input size, history traversal, recursion, memory, subprocess lifetime, and diagnostic volume. Fail explicitly when a limit makes a result incomplete.
- Add tests that fail before the fix and cover Windows-specific behavior when paths, terminals, processes, reparse points, locking, or line endings are involved.
- Avoid logging source, secrets, access tokens, environment values, or private repository URLs.

More detail is in the [threat model](docs/security/threat-model.md) and [development guide](docs/contributing/development.md).

## Pull requests

Keep a pull request narrow enough to review. Describe the user-visible outcome, tests, evidence, security/privacy considerations, and platform effects. Update public schemas, ADRs, RFCs, migration notes, and the changelog when their contracts change.

Before requesting review:

1. rebase or merge the current target branch without discarding others' work;
2. run the format, lint, build, and test commands above;
3. verify every commit has a DCO sign-off;
4. inspect the diff for secrets, generated clutter, private source, and accidental machine paths;
5. document checks you could not run—never mark them as passed.

A maintainer or CODEOWNER reviews the contribution. Authors do not approve their own changes except under the narrow, documented bootstrap procedure in [GOVERNANCE.md](GOVERNANCE.md). Maintainers may request splitting, additional fixtures, an ADR/RFC, or native-platform evidence when risk warrants it.

## Review and credit

Reviews should be specific, respectful, and focused on the project contract. Contributors are credited in Git history and release notes where appropriate. Sustained, high-quality stewardship can lead to area-reviewer or maintainer responsibility under [MAINTAINERS.md](MAINTAINERS.md).
