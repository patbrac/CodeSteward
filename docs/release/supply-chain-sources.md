# Phase 1 supply-chain source record

Reviewed 2026-07-10. Primary sources:

- GitHub-hosted runner labels, architectures, privileges, and arm64 macOS limitations: https://docs.github.com/en/actions/reference/runners/github-hosted-runners
- Full-length action SHA pinning: https://docs.github.com/en/actions/reference/security/secure-use
- Artifact-attestation permissions, SLSA provenance, SBOM predicates, and verification: https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations
- Offline attestation bundles and trusted roots: https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/verify-attestations-offline
- Immutable draft-then-publish releases: https://docs.github.com/en/code-security/concepts/supply-chain-security/immutable-releases
- Cargo Dependabot support/options: https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference
- Ubuntu 24.04 libc6/glibc 2.39 baseline: https://packages.ubuntu.com/noble/libc6
- cargo-deny license/advisory configuration: https://embarkstudios.github.io/cargo-deny/

Exact immutable action pins:

| Action | Upstream release | Commit |
|---|---|---|
| actions/checkout | v6.0.3 | df4cb1c069e1874edd31b4311f1884172cec0e10 |
| actions/setup-python | v6.3.0 | ece7cb06caefa5fff74198d8649806c4678c61a1 |
| actions/upload-artifact | v7.0.1 | 043fb46d1a93c77aae656e7c1c64a875d1fc6a0a |
| actions/download-artifact | v8.0.1 | 3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c |
| actions/attest | v4.1.1 | a1948c3f048ba23858d222213b7c278aabede763 |
| anchore/sbom-action | v0.24.0 | e22c389904149dbc22b58101806040fa8d37a610 |

Pinned non-action tooling:

- cargo-deny 0.20.2, installed by Cargo with --locked.
- Syft v1.46.0, requested explicitly from the pinned SBOM action.
- TruffleHog v3.95.9 Linux amd64 archive, SHA-256 f6d1106b85107d79527ed7a5b98b592beadd8b770dc3c9e8c1ad99e1b2cf127e.
- jsonschema 4.25.1 plus its fully version-pinned runtime graph in scripts/ci/requirements.txt for Draft 2020-12 schema-fixture validation.

The Git tag references above were resolved with git ls-remote and the workflow uses the dereferenced commit for the annotated actions/attest v4 tag.
