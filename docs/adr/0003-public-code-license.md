# ADR 0003: Public-code license

## Status

Accepted and adopted

## Date

2026-07-10

## Context

The project needs an OSI-approved license that supports broad local, CI, private-repository, plugin, and commercial integration use. The business plan depends on trust and distribution rather than making the community scanner intentionally incomplete. Apache-2.0 permits broad use and includes an express patent grant, but it also permits competing hosted use. That trade is intentional only if the business differentiates through operations, collaboration, organization context, reliability, support, and brand.

The public project name is Code Steward. Phase 1 proceeds on the approved naming, trademark-policy, contribution, and license assumptions. The repository includes the adopted license and notice files; this ADR records the technical and community boundary rather than offering legal advice.

## Decision

Adopt Apache License 2.0 for all public project code and artifacts: CLI, engine, built-in analyzers, language adapters, plugin protocol/SDKs, public schemas, local dashboard, and official CI integrations.

Use DCO sign-off for contribution provenance at launch rather than a broad relicensing CLA. Keep the proprietary control plane, billing, hosted identity, enterprise connectors, and support tooling in a separate private repository that consumes public schemas and protocols. Never describe proprietary code as open source.

The repository-root `LICENSE`, `NOTICE`, DCO instructions, dependency-license policy, and trademark policy implement this decision.

## Alternatives considered

- **MIT:** Rejected because Apache-2.0's express patent terms better match a multi-contributor analysis engine and SDK ecosystem.
- **AGPL or another strong network-copyleft license:** Rejected for launch because the project prioritizes broad local, CI, plugin, and commercial integration adoption and intends to compete through service quality rather than hosting restrictions.
- **Source-available license:** Rejected because it would conflict with the promise to publish the scanner as open source under an OSI-approved license.
- **Dual licensing backed by a broad relicensing CLA:** Deferred unless a concrete product need and contributor-rights analysis justify it. Launch uses DCO provenance and does not assume unilateral relicensing rights.
- **Proprietary or open-core analyzer split:** Rejected because a secret or more accurate paid analyzer would violate reproducibility and the community trust boundary.

## Consequences

- Users can run, inspect, modify, and integrate the complete deterministic scanner without a hosted account.
- Competitors may legally host the public code; the project accepts this in exchange for adoption and integration freedom.
- Security fixes and generally useful scanner behavior must remain public.
- Commercial scanner behavior cannot be silently patched or forked; scanner changes should be upstreamed first.
- Trademark policy, operational quality, community trust, and service value carry more commercial weight.

## Verification

- Keep `LICENSE`, `NOTICE`, DCO instructions, dependency-license policy, and trademark policy internally consistent before accepting external contributions.
- Audit that all public dependencies have allowed licenses or documented, counsel-reviewed exceptions.
- Confirm the public build contains the complete scanner, core analyzers, outputs, local policies, import/export, deletion, and security fixes.
- Compare identical local, CI, and hosted scanner inputs to prove no proprietary analyzer or hidden semantic override changes findings.
- Apply the community trust test to each commercial feature: paid value must be organization-scale operation, not withheld core analysis.

## Supersession

Changing the public-code license, contribution provenance model, or permanent open-core commitments requires a public RFC, a new ADR explicitly superseding ADR 0003, counsel review, and a documented community comment period. The proposal must identify rights in existing contributions, dependency/license compatibility, patent and trademark effects, treatment of already released versions, and migration consequences for users and plugins. No supersession may retroactively relicense third-party contributions without the required rights or move promised-open core analysis behind a paywall. Legal and naming approvals must be archived before public adoption is claimed.
