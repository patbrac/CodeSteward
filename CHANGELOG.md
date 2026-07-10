# Changelog

All notable changes to Code Steward are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and releases follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) once a public version is cut.

## [Unreleased]

## [0.1.0-alpha.1] - 2026-07-10

### Added

- Initial Apache-2.0 licensing, DCO contribution process, governance, support, security, trademark, maintainer, and Code of Conduct documents.
- Initial threat model, dependency policy, release/deprecation policy, ADR template, and public RFC process.
- Phase 1 Rust workspace boundaries, versioned configuration/report/plugin/export schemas, and the offline `steward version`, `steward doctor`, and `steward config validate` commands.
- Native CI and release qualification targets for Ubuntu 24.04 x86-64 (`x86_64-unknown-linux-gnu`), macOS 15 Apple silicon (`aarch64-apple-darwin`), and Windows Server 2025 x64 (`x86_64-pc-windows-msvc`).
- Development-release archives with SHA-256 checksums, SPDX JSON SBOMs, and GitHub keyless build-provenance and SBOM attestations.

### Changed

- This prerelease replaces the superseded, untagged Go prototype with the bounded Phase 1 Rust foundation.

### Removed

- The prior prototype's scan, analyzer, integration, installer, and example surfaces are unavailable in this prerelease. No migration compatibility with that untagged prototype is promised.

### Release status and compatibility

- This is a pre-alpha development prerelease of the planned `0.1` feature line. It is not production-supported, and only the exact native qualification targets above are eligible for Phase 1 qualification after their release workflow passes.
- The v0 configuration, report, plugin, and export schemas are being released for the first time, so there is no schema migration from an earlier public version.
- Git ingestion, repository scanning, analyzers, and finding production are outside this Phase 1 release. There is therefore no analyzer-rule change, expected finding churn, or benchmark baseline yet.
- This release contains no identified security fix.

### Known limitations

- Windows artifacts are not Authenticode-signed. macOS artifacts are not Apple Developer ID-signed or notarized. The GitHub keyless attestations verify workflow provenance but do not prevent native operating-system warnings or replace those platform trust systems.
- The Linux artifact is a user-local tarball with a GitHub keyless attestation, not a distribution package or package-manager-signed artifact; it has no distribution or package-manager signature.

[Unreleased]: https://github.com/patbrac/CodeSteward/compare/v0.1.0-alpha.1...HEAD
[0.1.0-alpha.1]: https://github.com/patbrac/CodeSteward/compare/7b7ccaae0969d0f492cc2dfe717a99a8378682a7...v0.1.0-alpha.1
