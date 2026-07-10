# Phase 1 native platform support

This matrix selects the release-blocking Phase 1 qualification targets. A target becomes proven Tier 1 only after CI and a tagged development release build, test, install, run offline, and uninstall on the exact coordinate. Selection alone is not evidence that the target passed. It does not claim compatibility with every version or architecture in an operating-system family.

| OS family | Tier | Native GitHub runner | Rust target | Development package | Qualification boundary |
|---|---:|---|---|---|---|
| Linux | Tier 1 qualification | ubuntu-24.04, x64 | x86_64-unknown-linux-gnu | tar.gz | Ubuntu 24.04 LTS and glibc 2.39 baseline |
| macOS | Tier 1 qualification | macos-15, Apple M1/arm64 | aarch64-apple-darwin | tar.gz | macOS 15 on Apple silicon |
| Windows | Tier 1 qualification | windows-2025, x64 | x86_64-pc-windows-msvc | zip | Windows Server 2025 x64 |

The workflow uses explicit runner labels, never a moving “latest” alias. GitHub’s current runner reference identifies ubuntu-24.04 as x64, macos-15 as arm64, and windows-2025 as x64. The release target manifest is **scripts/release/targets.json**.

## Limitations

- The Linux artifact is built against Ubuntu 24.04’s glibc 2.39 line. Debian, Ubuntu versions older than 24.04, other distributions, musl, and older glibc versions are not qualified. Successful execution elsewhere is not a support commitment.
- The macOS artifact is arm64 only. Intel Macs and Rosetta execution are unqualified. The Phase 1 archive is not Apple-notarized.
- Windows Server 2025 x64 is the selected Tier 1 qualification coordinate and is not proven until its native CI/release evidence passes. Windows 11 x64 remains unqualified Tier 3/community-experimental until separate native evidence promotes it. Windows on arm64 and WSL are also unqualified and do not substitute for native Windows evidence.
- Native archives are the Phase 1 package format. OS installers, package-manager formulae, Authenticode signing, Apple code signing/notarization, upgrades, and rollback packages are deferred.
- GitHub keyless artifact attestations cryptographically sign the archive digest and bind it to the tagged source and release workflow. They are the development signing and provenance mechanism; they do not replace Authenticode or Apple notarization in the operating-system trust UI.
- Phase 1 determinism checks the CLI’s initial surfaces and byte reproducibility on Linux. Full report-corpus semantic parity grows with later analyzer phases.

## Release-blocking evidence

Each coordinate must pass formatting, Clippy, workspace tests, schema checks, public-contract checks, dependency policy, deterministic ordering, and native no-network execution. A tagged artifact must then pass archive extraction, version and doctor commands, config validation, OS-level outbound-network denial, and removal.

The Linux two-build job must produce byte-identical stripped release binaries. If it does not, the job fails and archives **reproducibility-delta.txt** for investigation; prose cannot waive the check.

## Review cadence and tier changes

Review runner availability and vendor security support at least quarterly and before every minor release. A runner image change, architecture change, lower Linux libc baseline, promotion of Windows 11, or removal/demotion of a Tier 1 coordinate requires an ADR/RFC compatibility review and updated native evidence.

Primary references:

- https://docs.github.com/en/actions/reference/runners/github-hosted-runners
- https://packages.ubuntu.com/noble/libc6
- https://github.com/actions/runner-images
