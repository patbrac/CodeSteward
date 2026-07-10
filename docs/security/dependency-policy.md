# Dependency update, vulnerability, and license policy

Dependencies are part of Code Steward's attack surface and redistribution obligations. This policy applies to runtime, build, development, CI-action, packaging, and plugin-SDK dependencies, including transitive dependencies and generated artifacts.

## Selection

Before adding a dependency, document why the standard library or an existing dependency is insufficient, the minimum features used, maintenance/release health, unsafe/native/build-script code, network behavior, transitive graph, supported native targets, license, and deterministic/offline consequences. Prefer small, actively maintained libraries with a clear security process and reproducible source.

Do not add a dependency that causes the scanner to execute repository code, makes a default command contact the network, leaks source, or makes core analysis conditional on a commercial service.

## License classes

The automated allow list may include the following SPDX licenses when attribution/source terms are satisfied:

- `Apache-2.0`, `Apache-2.0 WITH LLVM-exception`
- `MIT`, `ISC`, `BSD-2-Clause`, `BSD-3-Clause`, `Zlib`
- `BSL-1.0`, `CC0-1.0`, `Unlicense`
- `Unicode-3.0` and equivalent reviewed Unicode data licenses

MPL-2.0, LGPL, data/font/content licenses, non-standard expressions, missing metadata, and packages with multiple unclear license choices require case-by-case maintainer and legal-compatibility review before merge. GPL/AGPL-family, SSPL, Commons Clause, Business Source License, PolyForm, “non-commercial,” “ethical use,” and other source-available/restricted terms are not allowed in distributed project artifacts without an accepted public RFC, ADR, and legal review.

The SPDX string alone is not conclusive. Review bundled code, notices, build artifacts, exceptions, and whether features change what is distributed. Required attributions go in `NOTICE` or generated release attribution material and the SBOM.

Exceptions are recorded in [`dependency-exceptions.md`](./dependency-exceptions.md) with package/version/source, scope, rationale, obligations, approvers, expiry/review date, and replacement plan. An undocumented exception fails the audit.

## Sources and lockfile

- Commit `Cargo.lock` for the workspace and all release builds. Pull requests that change resolution include and explain the lockfile diff.
- Use the public crates.io registry unless an exception records another immutable source. Git dependencies must pin a full commit and require a replacement/registry plan.
- Do not use unreviewed alternate registries, mutable branches/tags, path dependencies outside the repository, or vendored source without provenance and license inventory.
- Release and CI use `--locked`. Offline verification uses the already fetched, locked dependency set; it is not evidence of source authenticity by itself.
- New duplicate versions, wildcard requirements, unmaintained crates, and native/build-script dependencies receive explicit review.

## Automated and human checks

The expected Rust audit is:

```sh
cargo deny check advisories bans licenses sources
```

Format/lint/test jobs and any configured update service complement, but do not replace, human review. CI also scans secrets, inventories the complete dependency graph, produces a release SBOM, and pins third-party actions by immutable commit. A check that is not yet configured must be described as planned, not passed.

Maintainers review automated update pull requests at least weekly and perform a full direct-dependency freshness and exception review monthly. Patch/minor updates normally land individually or in small related groups with tests on native targets. Major updates require migration/compatibility review and an ADR when they alter a durable boundary. Security updates are handled immediately rather than waiting for the cadence.

## Vulnerabilities and mitigations

Use upstream advisories, RustSec, CVE data, exploitability in Code Steward's enabled features, and the project threat model. CVSS informs but does not replace contextual severity.

| Severity | Required action |
|---|---|
| Critical | Restrict release if needed; target fix or actionable mitigation within 7 calendar days. |
| High | Block a new release; target fix or actionable mitigation within 14 calendar days. |
| Medium | Triage within 14 days and target remediation within 30 days or the next minor release. |
| Low / informational | Record and address through normal maintenance based on exploitability. |

A vulnerable dependency that is unreachable in enabled features still requires a documented analysis. A temporary waiver states affected versions/features, reachability, compensating controls, owner, expiry (no more than 30 days for critical/high), upstream link, and removal plan. Critical/high waivers need security-responder plus maintainer approval; no high-severity risk may remain without a reviewed, time-bounded mitigation.

Security fixes for the public scanner remain public after coordinated disclosure and cannot be reserved for paid users.

## Update failure and rollback

Dependency pull requests include regression tests appropriate to the changed surface. Parser/Git/SQLite/terminal/process changes require hostile fixtures and native-platform review. If a release dependency causes correctness, security, or compatibility regression, stop promotion, revert/pin to the last reviewed version when safe, publish an advisory when users are exposed, and never silently replace an existing artifact.

Review this policy through an ADR for allow-list or automation changes; weakening contributor/user rights or the open-core/security contract requires an RFC.
