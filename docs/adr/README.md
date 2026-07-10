# Architecture decision records

ADRs record decisions that constrain implementation, compatibility, security, or the open-core boundary. The cleared public project name is **Code Steward**. An ADR's `Accepted` status records a governing technical decision; it does not by itself prove that implementation or release verification has passed.

## Process

1. Copy [`template.md`](./template.md), including security, privacy, compatibility, evidence, rollout, and supersession sections.
2. Assign the next four-digit number; numbers are never reused.
3. Use `Proposed`, `Accepted`, `Superseded`, or `Rejected` status.
4. Link a superseding ADR in both records.
5. Use a public RFC before changing a product contract, license/governance promise, public schema semantics, or major roadmap commitment.
6. Archive verification evidence in tests, releases, research records, or the project dashboard; do not claim completion from an ADR alone.

## Decision records

- [ADR 0001 — Core language and runtime](./0001-core-language-runtime.md)
- [ADR 0002 — Local graph and cache storage](./0002-local-storage-sqlite.md)
- [ADR 0003 — Public-code license recommendation](./0003-public-code-license.md)
- [ADR 0004 — AI authority and data boundary](./0004-ai-boundary.md)
- [ADR 0005 — Repository non-execution and security boundary](./0005-repository-non-execution.md)
- [ADR 0006 — Supported-platform policy](./0006-supported-platform-policy.md)
- [ADR 0007 — Phase 1 native release qualification matrix](./0007-phase-1-native-release-matrix.md)
