# Public request for comments process

RFCs are required for user-facing contract changes, license or governance promises, public-schema semantics, the open-core boundary, major roadmap commitments, and native platform-family commitments. Routine changes use pull requests; durable implementation/architecture decisions use ADRs.

## Lifecycle

1. Copy [`template.md`](./template.md) to `NNNN-short-title.md` using the next unused number and status `Draft`.
2. Open a pull request early enough to expose alternatives. A maintainer confirms that the proposal is complete enough for public review.
3. Change status to `In review`, record the start/end dates, and announce it in the repository's normal public channel. The comment period is at least 14 calendar days.
4. Address each substantive concern in the RFC or resolution summary. A material rewrite receives at least seven additional calendar days.
5. Non-conflicted maintainers resolve it as `Accepted`, `Rejected`, or `Withdrawn` under [GOVERNANCE.md](../../GOVERNANCE.md). Record rationale and dissent, not just a vote count.
6. An accepted RFC links its implementation issues and any required ADR. Completion evidence is separate; acceptance does not mean rollout passed.

Numbers are never reused, and resolved RFC files are immutable except for links, errata, and supersession notices. Security-sensitive details may be reviewed privately until disclosure is safe, but any lasting public-contract change receives a public record afterward.
