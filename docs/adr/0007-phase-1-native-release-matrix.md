# ADR 0007: Phase 1 native release qualification matrix

## Status

Accepted

## Date

2026-07-10

## Context

[ADR 0006](./0006-supported-platform-policy.md) makes Windows, macOS, and Linux first-class native operating-system families and defines Tier 1 as release blocking. Phase 1 needs concrete runner images, Rust target triples, archive formats, and a bounded pre-1.0 support window. A CI image proves only that exact qualification environment; it cannot justify an unbounded claim about every related desktop release, architecture, filesystem, libc, or compatibility subsystem.

Development releases also need publicly verifiable integrity and provenance. Platform code-signing identities and operational processes are not yet available, so Phase 1 must distinguish GitHub artifact attestation from Windows Authenticode and Apple Developer ID signing/notarization.

## Decision

Use this Phase 1 Tier 1 qualification matrix:

| OS family | Qualification environment | Rust target | Archive | Baseline note |
|---|---|---|---|---|
| Linux | GitHub-hosted `ubuntu-24.04` x86-64 runner | `x86_64-unknown-linux-gnu` | `.tar.gz` | Build and test on Ubuntu 24.04; record the maximum referenced GLIBC symbol version in each release and do not claim compatibility with an older glibc. |
| macOS | GitHub-hosted `macos-15` Apple silicon runner | `aarch64-apple-darwin` | `.tar.gz` | Qualify on macOS 15 arm64 only; Intel and earlier macOS releases remain unqualified. |
| Windows | GitHub-hosted `windows-2025` x64 runner | `x86_64-pc-windows-msvc` | `.zip` | Qualify in native PowerShell on Windows Server 2025; consumer Windows editions remain unqualified until separately tested. |

“Tier 1 qualification target” means failures on these exact environments block a development release after the workflow is required. It does not yet mean production support, nor does it claim all Ubuntu/Linux distributions, all macOS 15 hardware, or all Windows versions behave identically. WSL, containers, Rosetta, Wine, and cross-compilation do not substitute for a native row.

Each row must build, test, run the canonical fixture corpus, install from its archive, execute `steward version` and `steward doctor`, verify default no-network behavior, and uninstall/clean up using the documented native shell. Semantic results must match across rows after documented non-semantic metadata is removed.

Development releases publish SHA-256 checksums, an SBOM, and GitHub artifact attestations bound to the source revision and release workflow. Verification uses GitHub's public attestation verification plus an independent checksum comparison. Authenticode signing on Windows and Apple Developer ID signing/notarization on macOS are explicitly deferred; release notes and install instructions must disclose their absence and the resulting operating-system warnings. A development release must not describe GitHub attestation as Authenticode, notarization, or universal platform signing.

For `0.x` releases, the newest minor line receives normal fixes. The immediately previous minor receives critical/high security fixes for 90 days after its successor, after which it is unsupported. Deprecations normally remain usable for at least one subsequent minor release and 60 calendar days, whichever is longer; an actively exploitable security issue may require faster removal with migration guidance. The detailed policy is in the repository release/deprecation document.

## Alternatives considered

- **Older or “latest” floating runner labels:** Rejected because an old baseline lacks current lifecycle headroom and a floating label weakens reproducibility. Pinned major runner labels make the qualification evidence reviewable.
- **x86-64 macOS in Phase 1:** Deferred because the selected first-party hosted environment and expected future lifecycle favor arm64; this is not a declaration that Intel users are unimportant.
- **Windows 11 as the qualification host:** Deferred because GitHub-hosted CI provides the selected Windows Server image. Desktop editions require separate clean-machine evidence before a support claim.
- **musl or multiple Linux distributions:** Deferred until demand and native packaging evidence justify the maintenance cost. Container/musl builds may later be separate targets.
- **Cross-build every artifact from Linux:** Rejected as qualification evidence because it would not exercise native linkers, filesystems, terminals, installers, or process behavior.
- **Self-managed signing identities immediately:** Deferred until secret custody, rotation, incident response, and notarization/Authenticode operations are designed and tested. Checksums and GitHub attestations provide a verifiable Phase 1 foundation without overstating identity guarantees.

## Security impact

Pinned runner families narrow environmental drift but do not eliminate mutable images or dependency compromise. Workflows must use least-privilege permissions, pin third-party actions by full commit, consume the committed Cargo lockfile, generate provenance in the release workflow, and avoid exposing release authority to untrusted pull-request code. Absence of Authenticode/notarization increases social-engineering and operating-system warning risk, which must remain visible to users.

## Privacy and data impact

Qualification uses synthetic fixture repositories only. Release jobs must not upload private source, local paths, credentials, user identities, or cache contents. Attestations contain source/repository and workflow metadata needed to verify provenance, not scanned-user data.

## Compatibility and cross-platform impact

The matrix turns three OS-family promises into exact Phase 1 checks while leaving other versions/architectures untested rather than implicitly unsupported forever. Archive layout, executable naming, line endings, path handling, terminal output, exit codes, config/cache locations, and canonical reports must be documented and compared across rows. Adding or promoting a target requires the ADR 0006 parity evidence.

## Open-core and licensing impact

All artifacts contain the same Apache-2.0 deterministic scanner and public schemas. No platform receives proprietary analyzer behavior. Release SBOMs and notices must cover every distributed dependency.

## Consequences

- Phase 1 has a finite, native, release-blocking matrix.
- Users on nearby desktop versions may experiment but must not be told their target is qualified.
- The initial matrix omits Linux musl, Linux arm64, macOS Intel, and Windows arm64.
- Project documentation must disclose missing platform-native signing and avoid “signed” wording that obscures the distinction.
- Maintainers incur three native CI and install/uninstall paths plus semantic-parity review.

## Evidence and verification

Before calling a row qualified, archive:

- runner image/toolchain identity and Rust target;
- locked build, format/lint, unit/schema/golden, security, license, and secret-scan results;
- native archive install, `version`, `doctor`, offline/default-command, uninstall, and cleanup results;
- checksum, SBOM, and GitHub attestation generation and independent verification;
- canonical cross-platform report comparison;
- Linux GLIBC symbol-version inventory for the released binary;
- explicit release-note disclosure that Authenticode and Apple notarization are absent.

This ADR selects targets; it does not claim that these checks have run.

## Rollout and rollback

Land CI as non-required while stabilizing runner/toolchain failures, then require every matrix row before issuing a development release. If a runner image is unavailable or compromised, pause releases rather than silently skipping the row. An emergency replacement must document image/toolchain deltas and rerun the full qualification corpus. Artifacts whose checksum, SBOM, or attestation cannot be verified are withdrawn and replaced under a new version; published artifacts are never silently overwritten.

## Supersession

Changing an exact target, archive, qualification tier, support window, or signing/provenance strategy requires a new ADR that cites demand, vendor/toolchain lifecycle, native test evidence, migration impact, and release cost. Weakening the three-family commitment or parity requirements also requires a public RFC under ADR 0006. Authenticode or Apple notarization may be added by a focused ADR once identity custody, secret management, rotation, failure recovery, and clean-machine verification are defined.
