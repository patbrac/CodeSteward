# Development release process

## Repository controls required

Before the first tag, administrators must:

1. protect the default branch and require the seven checks listed below;
2. require two approving reviews for changes under **.github/workflows/** and **scripts/release/** through CODEOWNERS;
3. disallow force-pushes and tag deletion for release tags;
4. enable Dependabot security updates, private vulnerability reporting, and immutable releases;
5. restrict Actions to approved publishers and require full-length action SHA pins;
6. make artifact attestations available (public repository, or an eligible GitHub Enterprise Cloud private/internal repository).

Required check names:

- dco
- native-linux-x64
- native-macos-arm64
- native-windows-x64
- secret-scan
- reproducible-linux
- required

The aggregate **required** check detects cancellation/skipping, but it does not replace the five named evidence checks in branch protection.

The two-review rule is the steady-state control. [GOVERNANCE.md](../../GOVERNANCE.md) permits a time-bounded exception while the repository has never been public and only the initial project lead exists. The single pull request that establishes the public baseline may be merged without approving reviews after all available required checks pass and it records its complete diff, a file-by-file self-review of release-sensitive files, why an independent reviewer is unavailable, the exact publication commit, and a UTC expiry no later than 14 calendar days after merge. Because the trusted-base `dco` workflow does not exist on the old base, that sole pull request records a clean local run of the exact committed checker over the complete proposed range, its SHA-256, and the checked commit IDs. The pull request must remain in repository history and become publicly visible before any tag is created.

Until that expiry or the first successful immutable Phase 1 development prerelease, whichever occurs first, an audited project-lead bypass may be used only for the smallest code, test, documentation, workflow, or release-script fix proven necessary by a linked failing required check or development-prerelease run. The fix must pass `dco` and all other required checks, record file-by-file self-review plus platform/security impact and rollback, and must not add features, change public contracts, broaden permissions/secrets/network access, weaken checks, remove immutable pins, or bypass checksum/SBOM/provenance/native evidence. Record every use publicly and remove the bypass when the window closes. Every later change under **.github/workflows/** or **scripts/release/** requires two approving reviews, even during a single-maintainer period.

## Creating a release

1. Confirm the candidate reached the default branch through a pull request with the trusted `dco` status (or the recorded one-time publication evidence) and that its default-branch commit passes all six push-triggered CI checks.
2. Update all workspace package versions and **Cargo.lock** in a reviewed pull request.
3. Review dependency/license results and verify there is no unmitigated high-severity issue.
4. Create and push an annotated SemVer tag such as v0.1.0 from the reviewed commit on the default branch.
5. With a full-history checkout, the Release workflow rejects a lightweight tag, a tag not identifying its checked-out commit, a commit not reachable from `origin/main`, or a tag version that does not exactly match the steward-cli Cargo version. It never fetches or changes refs inside the validation script.
6. Each exact native runner builds its own target. The archive is installed into a clean temporary prefix, run with native outbound networking denied, and removed.
7. Each target produces an archive containing project and locked third-party license texts, a per-archive checksum, SPDX JSON SBOM, SLSA build-provenance bundle, and signed SBOM-attestation bundle.
8. The publish job downloads all matrix outputs, verifies the complete target set, writes release-wide SHA256SUMS, creates a draft marked as a prerelease, uploads every asset, and only then publishes the development/pre-alpha prerelease. Phase 1 artifacts are not production-supported releases.

With GitHub immutable releases enabled, publishing locks the tag and assets and creates GitHub’s release attestation. The workflow deliberately stages assets in a draft first.

## Failure and recovery

Build or attestation failure publishes nothing. A failure after draft creation leaves a draft for maintainers to inspect and delete. Never overwrite or move a published release tag. Fix the source through review and issue a new patch version.

If a release contains a security defect, follow **SECURITY.md**. Preserve the affected immutable release for audit unless legal or safety requirements demand removal, mark it affected in release notes/advisories, and publish a fixed patch release.

## External evidence

Workflow files can define required checks but cannot configure branch protection, two-person reviews, bypass actors, tag rules, private vulnerability reporting, immutable releases, or plan eligibility. Capture screenshots/API exports of those settings, the publication-window expiry and every audited bypass (if used), its closing event, and the first successful native CI/release runs before treating the external gates as complete.
