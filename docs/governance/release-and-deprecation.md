# Release and deprecation policy

## Release stages

- **Development/pre-alpha artifacts** prove build and verification plumbing. They are not production-supported.
- **Alpha (`0.x`)** validates behavior and schemas with early adopters. Breaking changes remain possible under the notice window below.
- **Beta** narrows change and requires migration tooling, native parity, and operational documentation before 1.0.
- **Stable (`1.x`)** provides the documented CLI/config/schema compatibility and support guarantees of its release line.

Semantic Versioning applies to released public contracts. Before 1.0, a minor version may contain breaking changes; patch versions must not deliberately break documented behavior.

## Pre-1.0 support window

The newest `0.x` minor line receives normal correctness and security fixes. The immediately previous minor line receives critical/high security fixes for 90 calendar days after the newer minor is released. Older minors, unpublished snapshots, and arbitrary commits are unsupported. A release note must state exact support dates; lack of maintainer capacity does not silently extend them.

For a documented CLI flag/command, configuration field, schema revision, plugin message, or output contract, a pre-1.0 deprecation normally remains usable for at least one subsequent minor and 60 calendar days, whichever is longer. Warnings identify the replacement and planned removal version/date. An actively exploitable security flaw may require faster disablement/removal with an advisory, migration/workaround, and recorded exception.

Public data schemas use explicit version directories. Readers should accept the documented compatible range; writers emit one declared version. An incompatible semantic change receives a new schema version and migration/export guidance rather than silently changing existing schema meaning.

## Stable policy

Before 1.0, an RFC will set the stable support duration and LTS policy. At minimum, incompatible stable changes require a new major version, public RFC, migration path, release notes, and a defined overlap period. 1.0 must not ship with that value unresolved.

## Native target qualification

[ADR 0007](../adr/0007-phase-1-native-release-matrix.md) selects finite Phase 1 Tier 1 qualification environments. “Tier 1” means release blocking only after its evidence and required checks are active; it is not a blanket claim about every OS version in a family. A release states the OS image, architecture/target triple, archive, toolchain, glibc baseline where relevant, and known signing/install warnings.

Demotion/removal of a Tier 1 target requires an RFC/ADR as applicable, advance notice under the deprecation window, migration guidance, and continued satisfaction of the product contract's native Windows, macOS, and Linux commitment.

## Release contents and process

Every promoted release must have:

- source revision/tag and reviewable changelog/release notes;
- green required format, lint, build, test, schema, secret, dependency/license, no-network/non-execution, determinism, and native matrix jobs;
- per-target archives with consistent layout and no private/debug material;
- SHA-256 checksum manifest, SBOM, and GitHub artifact attestation/provenance;
- public verification, install, upgrade, rollback, uninstall, cache/export/deletion, and known-limit instructions;
- supported-version/target statement and all deprecations/removals;
- license/notice attribution inventory.

The release manager verifies artifacts from a clean environment and records evidence. A workflow success alone is insufficient. Development releases explicitly disclose that Windows Authenticode and Apple signing/notarization are absent until a later approved process implements them.

Published versioned artifacts are immutable. If an artifact, checksum, SBOM, provenance record, or tag is wrong, stop distribution, document impact, revoke/mark affected material, and issue a new version. Never replace bytes under an existing version.

## Change control

Routine patch mechanics use pull-request review. Durable release-architecture changes use an ADR. Weakening compatibility/support promises, changing the license/open-core boundary, or changing major roadmap/platform commitments requires a public RFC.
