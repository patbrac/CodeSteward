# ADR 0006: Supported-platform policy

## Status

Accepted; exact target coordinates selected by ADR 0007

## Date

2026-07-10

## Context

The project promises cross-platform local value. “Cross-platform” is not verifiable unless support distinguishes native operating systems from containers, identifies release-blocking targets, and requires installation plus semantic-parity evidence. The current plans name Linux, macOS, and Windows but do not select OS versions, CPU architectures, Linux libc/distribution baselines, installer formats, or support lifetimes. No CI or package evidence exists yet, so this ADR must not imply it does.

## Decision

Windows, macOS, and Linux are first-class native OS families. By 1.0, the published matrix must contain at least one native Tier 1 target from each family. A container image may be supported in addition to native packages, but it does not satisfy any OS family's native commitment.

Use these tiers:

- **Tier 1 — Release blocking:** Official native artifacts, documented install/upgrade/uninstall, security coverage, offline operation, and canonical semantic parity. Failure blocks a stable release.
- **Tier 2 — Maintained best effort:** Built or regularly tested by the project, but failures may not block release. Limitations and expected maintainer response are public.
- **Tier 3 — Community/experimental:** May compile or be packaged externally; no project support or release promise. It must not be described simply as “supported.”

OS-family support and a target matrix are distinct. Phase 0 records these exact target coordinates as unresolved rather than choosing them without evidence:

| Decision coordinate | Phase 0 value |
|---|---|
| Exact supported Windows version(s) | `TBD` |
| Exact supported macOS version(s) | `TBD` |
| Exact supported Linux distribution/kernel baseline(s) | `TBD` |
| CPU architecture and tier mapping for each OS family | `TBD` |
| Linux glibc baseline and any musl target | `TBD` |
| Native archive, installer, and package formats | `TBD` |
| Artifact signing, Windows signing, and macOS signing/notarization mechanisms | `TBD` |
| Tier 1 support windows, deprecation notice, and end-of-support policy | `TBD` |

This table preserves the Phase 0 state. [ADR 0007](./0007-phase-1-native-release-matrix.md) resolves the Phase 1 qualification targets, archives, development-attestation approach, explicit platform-signing deferrals, and `0.x` window. Those selections are targets until their native evidence passes; any coordinate not selected by ADR 0007 remains `TBD`.

### Required native parity coverage

The eventual target decision and Tier 1 CI must cover the matrix below on native Windows, macOS, and Linux. These are required future fixtures and assertions, not completed tests.

| Area | Required fixtures and assertions |
|---|---|
| Windows and portable paths | Repository-relative canonical paths; Windows drive-letter roots; UNC roots; reserved Windows names; long paths; `.`/`..` traversal; slash/backslash input; no root escape or machine-root contribution to stable identity. |
| Case behavior and collisions | Case-sensitive and case-insensitive filesystems; paths differing only by case; checkout collisions; deterministic diagnostics rather than silent entity conflation. |
| Unicode | NFC and NFD equivalents; non-ASCII filenames, identities, and terminal text; stable normalization policy; malformed/control input is rejected or sanitized consistently. |
| Line endings and executable state | CRLF and LF versions of the same semantic content; Git executable-bit changes; `core.autocrlf` variants; identical semantic observations where content meaning is unchanged. |
| Links and filesystem indirection | In-root and escaping Unix symlinks; Windows junctions and other reparse points; loops and broken links; canonical containment with outside-root reads denied. |
| Git parity | Repository and worktree Git configuration; `.mailmap`; shallow/full and merge history; rename lineage; identical Git-bundle observations; hooks, filters, helpers, and external diff programs remain unexecuted. |
| Terminal and process behavior | UTF-8 and native terminal encoding boundaries; TTY and redirected output; control-sequence sanitization; stdout/stderr separation; identical documented exit codes; child/plugin timeout and cleanup behavior. |
| Native state locations | Documented per-OS config, cache, report, and private temporary directories; permissions; cleanup; export and deletion; no semantic dependence on an absolute machine path. |
| SQLite | Native file locking, concurrent readers/writers, interruption, busy handling, atomic migration, rollback, corruption diagnostics, and recovery without stale or platform-specific semantic output. |
| Offline install and execution | Native install, upgrade, rollback, uninstall, default no-network scan, and cache deletion without relying on a container or compatibility subsystem. |

Publish those choices as an amendment or superseding ADR before calling any artifact stable. Select each target using the following rubric:

1. Demonstrated user/design-partner and CI-runner demand.
2. Vendor security-support lifetime.
3. Rust, Tree-sitter, SQLite, Git, signing, and packaging feasibility.
4. Availability of repeatable hosted and clean-room test environments.
5. Native install, upgrade, uninstall, and rollback reliability.
6. Filesystem, process, terminal, cache, and plugin-security behavior.
7. Canonical semantic parity with the shared Git-bundle corpus.
8. Artifact size, build/release cost, and maintainer capacity.
9. A documented deprecation and user-migration path.

Documentation must use “targeted” or “experimental” until a target's tier evidence is archived. Containers and WSL can be separate targets, not proxies for native Windows validation.

## Alternatives considered

- **Container-only support:** Rejected because it does not exercise native filesystems, shells, terminals, process control, state locations, installation, or Windows/macOS security behavior.
- **Linux-only or Linux/macOS-first-class support with Windows best effort:** Rejected because native Windows is an explicit product-family commitment rather than a compatibility afterthought.
- **Promise every current OS version and architecture:** Rejected because it is unbounded and unverifiable. Exact targets must be selected from demand, vendor support, toolchain feasibility, and archived evidence.
- **Choose exact versions and packages during Phase 0:** Deferred because there is no CI, package, design-partner, installation, or maintenance-cost evidence yet. The coordinates remain explicitly `TBD`.
- **Treat every compiling community target as supported:** Rejected because compilation alone does not prove deterministic, offline, secure, installable behavior. Tier 2 and Tier 3 preserve experimentation without overstating support.

## Consequences

- Release engineering must handle genuinely native behavior across three OS families.
- Platform-specific output differences are defects unless explicitly non-semantic or a documented incomplete capability.
- Supporting every OS version and architecture is not promised.
- Tier changes require public release notes; removal from Tier 1 requires a support window and migration guidance.
- Early development can proceed before the exact matrix is chosen, but releases cannot overstate support.

## Verification

The following is the required evidence plan; none of it is claimed complete by this ADR:

- Publish the exact OS/version/architecture/libc/package matrix and tier for each target.
- Build, install, run, upgrade, uninstall, and verify signed artifacts in clean environments.
- Run one immutable Git-bundle corpus on every Tier 1 target and compare canonical semantic reports.
- Run no-network, non-execution, path containment, terminal safety, SQLite migration/recovery, parser, and plugin process-lifecycle suites natively.
- Verify platform-appropriate config/cache/temp paths and complete export/deletion.
- Test CLI exit codes and redirected output in native shells, including PowerShell on Windows.
- Archive results in CI/releases and block stable releases on any Tier 1 failure.
- Treat container-only evidence as container evidence, never as native OS evidence.

## Supersession

Changing the three native OS-family commitment, tier definitions, required parity coverage, or the exact target matrix requires a public RFC and a new ADR that explicitly supersedes ADR 0006. Selecting values for the `TBD` coordinates may be an approved amendment or superseding ADR, but it must include design-partner demand, vendor/toolchain lifetime, clean install/upgrade/uninstall results, the complete native parity matrix above, security/offline checks, and canonical report comparisons. Removing or demoting a Tier 1 target requires compatibility impact analysis, public notice, a support/deprecation window, migration guidance, and archived evidence that remaining targets still satisfy the product contract.
