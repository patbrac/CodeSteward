# Maintainers

This file records project roles with merge, security, or release authority. Git history remains the source for individual contributions; this list records current responsibility.

## Current roles

| Person | GitHub | Roles | Areas |
|---|---|---|---|
| Initial project steward | [`@patbrac`](https://github.com/patbrac) | Project lead, maintainer, security responder, release manager | Repository-wide during bootstrap |

Concentrating all bootstrap roles in one person is a continuity risk, not a preferred steady state. Security and release operations should gain a second qualified maintainer before a production-stable release.

## Promotion

Any maintainer may nominate a contributor with the candidate's consent. Promotion is based on sustained stewardship, including:

- technically sound contributions and reviews over time;
- reliable handling of feedback, compatibility, security, privacy, and cross-platform consequences;
- constructive participation under the Code of Conduct;
- willingness and capacity to perform the role's operational duties;
- judgment about the deterministic/open-core product contract.

Commit count, employer, sales relationship, and automated-output volume are not promotion criteria. The candidate must disclose relevant conflicts of interest. Maintainers record the scope, evidence, decision, and effective date in a pull request changing this file. Until two maintainers can participate, the project lead decides and documents the rationale; afterward, a majority of non-conflicted maintainers approves.

Area reviewers are granted explicit paths or domains. Security responders need private-channel training and confidentiality agreement to the project process. Release managers must demonstrate artifact, provenance, rollback, and native-platform verification.

## Inactivity, resignation, and removal

A role holder may resign at any time. A maintainer inactive from review and operational duties for six months may be moved to emeritus status after private contact and a 30-day response period. Returning requires a scoped reappointment, not a new contribution threshold.

Access may be suspended immediately to protect users or the project after credential compromise, a serious Code of Conduct issue, mishandling of an embargo, or abuse of authority. Permanent involuntary removal requires a written rationale and majority decision by non-conflicted maintainers. When fewer than two non-conflicted maintainers remain, an independent respected area reviewer should be asked to review the process. Personal and embargoed details remain private, but the role change is public.

## Access review

Maintainer, CODEOWNER, security-advisory, package-publishing, signing, and release permissions are reviewed at least every six months and after every role change. Use least privilege, phishing-resistant multi-factor authentication where the service supports it, and no shared personal credentials.

The full authority and decision model is in [GOVERNANCE.md](GOVERNANCE.md), and role expectations are in the [contributor ladder](docs/governance/contributor-ladder.md).
