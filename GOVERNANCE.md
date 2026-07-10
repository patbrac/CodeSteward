# Governance

Code Steward is governed in public around technical merit, user trust, and long-term stewardship. Authority comes with responsibility for review, security, release quality, and community health—not with commit count or employer status.

## Project commitments

Maintainers must preserve the product contract unless it is changed through the public RFC process. In particular:

- deterministic evidence remains authoritative;
- default scans remain local, offline-capable, and non-executing;
- the complete deterministic scanner and generally useful security fixes remain open under Apache-2.0;
- native Windows, macOS, and Linux behavior is treated as product behavior, not an afterthought;
- commercial differentiation comes from managed operation and organization-scale workflow, not withheld analysis.

No maintainer can privately waive these commitments for a release or hosted deployment.

## Roles

The contributor ladder is:

1. **Contributor** — anyone participating through issues, documentation, code, design, testing, or review.
2. **Area reviewer** — a contributor trusted to review a defined area and whose approval is normally requested there.
3. **Maintainer** — a steward with merge authority and shared responsibility for project direction, contributor experience, compatibility, and operational health.
4. **Security responder** — a maintainer or specifically appointed reviewer with access to private reports and responsibility for coordinated disclosure.
5. **Release manager** — a maintainer appointed for a release who validates artifacts, provenance, compatibility evidence, and release notes.

One person may hold several roles. Current assignments and the promotion/removal process are in [MAINTAINERS.md](MAINTAINERS.md); detailed expectations are in the [contributor ladder](docs/governance/contributor-ladder.md).

## Decision levels

### Routine changes

Bug fixes, tests, bounded internal refactors, and documentation clarifications use a pull request with review from an appropriate CODEOWNER or area reviewer. Outside the bootstrap rules below, the author cannot be the sole approving reviewer. A maintainer may merge after required checks and review pass.

While fewer than two non-conflicted maintainers exist, the project lead may merge a code or other risk-bearing routine change they authored only when all required checks pass, the pull request publicly records the rationale and line-by-line self-review, and it has remained open for at least 72 hours for community review. The waiting period may be shortened only for an actively exploited vulnerability or a broken release, with the reason recorded publicly as soon as disclosure permits.

A truly documentation-only correction may use a narrower single-maintainer path without the 72-hour wait after all required checks pass and the pull request records a complete self-review. It may change prose, spelling, formatting, or a non-normative link only. It may not change code, tests, schemas, dependencies, generated files, behavior or support claims, release/security/privacy instructions, governance or contributor rights, license/trademark terms, workflow files, or anything under **scripts/release/**. Any doubt uses the 72-hour path or the higher process applicable to the file. Both single-maintainer paths end automatically when a second non-conflicted maintainer is appointed, and neither bypasses an RFC comment period.

Release automation has a higher steady-state quorum: changes under **.github/workflows/** and **scripts/release/** require two approving reviews. A time-bounded Phase 1 publication window provides the only exception. While the repository is still private, has never been published, and has only the initial project lead, that lead may merge the single pull request that establishes the public repository baseline without approving reviews. The pull request must retain the complete diff, passing available required checks, a file-by-file self-review of release-sensitive changes, the reason an independent reviewer is unavailable, the exact commit selected for publication, and an explicit UTC expiry no later than 14 calendar days after merge. It must be preserved when the repository becomes public, and no release tag may be created until that record is publicly visible.

The trusted-base `dco` workflow cannot run before that workflow exists on the base branch. For the publication pull request only, the lead must run the proposed checker's exact committed bytes from a clean checkout against the complete proposed commit range and attach the command, output, checker SHA-256, and reviewed commit IDs to the pull request. That recorded evidence substitutes only for the unavailable initial `dco` status; it is not a reusable DCO bypass.

After publication and before the window closes, the project lead may merge a Phase 1 bootstrap fix without approving reviews or the ordinary 72-hour wait only when a specific failing required check or development-prerelease run proves it necessary. The pull request must link the failed run, contain the smallest code, test, documentation, workflow, or release-script fix for that root cause, pass the trusted `dco` check and every other required check, and record a file-by-file self-review, platform/security impact, and rollback. It may not add a feature, change a public contract, broaden permissions, secrets or network access, weaken a check, remove an immutable pin, or bypass checksum, SBOM, provenance, or native-platform evidence. A change outside those bounds uses the ordinary process or waits for two non-conflicted approvals when release automation is affected.

The publication window closes irreversibly at the earlier of its recorded UTC expiry or the first successful immutable Phase 1 development prerelease. Every exception merge and the closing event are recorded publicly. After closure, the two-review requirement applies even if fewer than two maintainers remain; neither ordinary single-maintainer path overrides it.

### Architecture or schema changes

Changes that create a durable implementation constraint, cross-crate dependency, data migration, security boundary, or public-schema implementation choice require an [architecture decision record](docs/adr/README.md). The ADR is reviewed with the implementing pull request or before implementation.

### Public-contract changes

User-facing contracts, license or governance promises, public schema semantics, major roadmap commitments, the open-core boundary, and native platform commitments require a [public RFC](docs/rfc/README.md). The normal comment period is at least 14 calendar days after the proposal is announced and materially complete. Substantive revisions restart at least seven calendar days of review.

Embargoed vulnerability response may temporarily bypass public discussion when disclosure would create risk. Security responders must record the decision privately and publish the non-sensitive rationale and any contract change as soon as coordinated disclosure permits.

## How decisions are made

The project prefers reasoned consensus. Reviewers identify evidence, compatibility, security, privacy, cross-platform, and open-core consequences instead of counting reactions.

If consensus is not reached:

1. the proposal owner writes the unresolved alternatives and trade-offs;
2. participating maintainers state approve, reject, or abstain with a rationale;
3. a majority of non-conflicted maintainers decides, provided at least two maintainers participate;
4. while the project has fewer than two non-conflicted maintainers, the project lead decides and records why.

The initial project lead is `@patbrac`. This tie-break role does not permit bypassing license rights, the Code of Conduct, security embargoes, or the required RFC process.

## Conflicts of interest

Reviewers disclose financial, employment, personal, or competitive interests that a reasonable contributor could see as affecting judgment. A conflicted maintainer may provide facts but does not supply the decisive approval. Security details and personal information may be disclosed privately to the other maintainers.

## Transparent records

- Pull requests record routine decisions.
- ADRs record durable technical decisions and explicit supersession.
- RFCs record public-contract proposals, discussion windows, and resolution.
- Release notes and [CHANGELOG.md](CHANGELOG.md) record user-visible changes and deprecations.
- Private security details remain restricted until coordinated disclosure.

Governance changes themselves require an RFC. Minor corrections that do not change authority, contributor rights, or required process may use a routine pull request.
