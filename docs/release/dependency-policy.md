# Dependency, update, vulnerability, and license policy

## Scope and authority

**Cargo.lock** is committed and all CI/release builds use the locked graph. **deny.toml** is the executable Rust dependency policy. **.github/dependabot.yml** schedules Cargo, GitHub Actions, and schema-tool updates weekly in America/Chicago.

Every dependency change must explain why it is needed, preserve the non-execution/offline boundary, and pass all native checks. Dependabot updates are ordinary pull requests and receive the same review.

## Licenses

Dependencies are denied unless their SPDX expression is explicitly allowed by **deny.toml**. The initial allowlist is Apache-2.0, Apache-2.0 WITH LLVM-exception, MIT, BSD-2-Clause, BSD-3-Clause, ISC, Unicode-3.0, and Zlib. MPL-2.0 and other licenses outside that list require a package-specific review and exception. Development and build dependencies are included. Version wildcards remain denied; wildcard versions implicit in private in-workspace path dependencies are allowed because Cargo does not publish these crates.

An exception requires:

- exact package name and version range;
- SPDX license and why the normal allowlist cannot apply;
- source/license-text review and product-distribution impact;
- approving maintainers;
- owner, review date, and removal/renewal deadline;
- a narrow entry in cargo-deny plus a matching record in the authoritative **docs/security/dependency-exceptions.md** register.

No undocumented exception is permitted.

## Advisories and response

Cargo-deny checks the current RustSec database and denies known vulnerability and unsoundness advisories. Yanked dependencies fail. Unmaintained advisories fail when they affect a direct workspace dependency; transitive maintenance risk remains part of dependency-review inventory.

- Critical/exploited dependency issues: triage immediately, mitigation or release decision within 24 hours.
- High severity: reviewed mitigation or upgrade plan within 3 business days and remediation target no later than 7 days.
- Moderate/low: schedule according to reachability and exposure, with a tracked owner.

A temporary advisory ignore must identify the advisory/package, affected code path, reachability evidence, compensating control, owner, approval, and expiration. The release workflow must not proceed with an unmitigated high-severity dependency issue.

## Sources and automation

Unknown Cargo registries and Git dependencies fail. Adding a registry or Git source requires a supply-chain review and digest/revision strategy. Wildcard dependency versions fail.

Every external action is pinned to a full 40-character commit SHA. Dependabot may propose a new SHA, but the review must confirm it belongs to the upstream repository and records the corresponding upstream release. TruffleHog is downloaded at a fixed version and checked against a hard-coded SHA-256 before execution. Syft is fixed to an explicit version inside the pinned SBOM action.

GitHub Dependabot security updates and the dependency graph are repository settings and must be enabled by an administrator; the YAML schedule alone cannot enable them.
