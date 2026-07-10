# Release engineering

- **process.md** defines protected-tag preparation, draft publication, failure handling, and external repository settings.
- **verification.md** is the consumer procedure for checksums, keyless signatures, provenance, SBOM attestations, install, and uninstall.
- **dependency-policy.md** defines dependency updates, licenses, advisories, action pins, and exceptions.
- **../security/dependency-exceptions.md** is the single authoritative dependency-exception register.
- **supply-chain-sources.md** records the primary sources used for the Phase 1 design and the exact action/tool pins.
- **workspace-boundaries.md** records the exact allowed internal Cargo graph enforced in CI.

The executable release contract is split between **.github/workflows/release.yml**, **scripts/release/targets.json**, and the verification scripts in **scripts/release/**.
