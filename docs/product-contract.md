# Code Steward product contract

**Status:** Approved product contract
**Date:** 2026-07-10
**Approval state:** Founder approval, public-name clearance, and Apache-2.0 adoption are treated as complete for Phase 1 implementation.
**Change control:** A public RFC and an accepted ADR are required to weaken or materially change a commitment in this document.

## Public problem statement

Software teams can produce changes faster than they can review, understand, operate, and retire them. The resulting maintenance burden appears over time as weak stewardship, concentrated knowledge, hidden change coupling, duplicated behavior, architectural drift, missing test evidence, and unnecessary dependency burden.

Existing linters, security scanners, code-ownership files, Git queries, diagrams, spreadsheets, and AI review tools each expose only part of that history. The project will provide a deterministic, inspectable maintenance record derived from code structure, repository history, ownership, configuration, and change behavior. It supplements engineering judgment; it does not replace it.

The initial wedge is a pull-request maintenance delta for TypeScript/JavaScript and Python repositories, backed by an inspectable historical graph.

## Product promise

A user should eventually be able to run the placeholder command:

```text
steward scan --base origin/main --head HEAD
```

and receive clear answers to four questions:

1. What maintenance risk did this change add or remove?
2. What evidence supports each conclusion?
3. Who or what area is likely to bear the future cost?
4. What is the smallest reasonable follow-up action?

The first useful result must require no hosted account, internet connection, source upload, or LLM API key.

## Initial scope

The initial product includes:

- TypeScript and JavaScript syntax analysis.
- Python syntax analysis.
- Language-neutral Git and history analysis for any text repository.
- Standard Git repositories, monorepos, and local multi-repository workspaces.
- Local clones in branch or detached-head state.
- Explicit degradation for shallow or incomplete history.
- Git submodules treated as opaque dependencies by default.
- Git LFS pointers inspected without automatically fetching large objects.
- Ownership, knowledge concentration, coupling, duplication, declared architecture, test-evidence, and direct-dependency context.
- Local policy, suppressions, native JSON, terminal, SARIF, and offline HTML output.
- Local and CI operation on private or public repositories.
- An open plugin protocol and local dashboard after their product phases are complete.

Repository builds and compiler-produced semantic indexes may later be explicit enrichments. They are not prerequisites for useful default analysis.

## Approved Phase 0 decisions

The following decisions are compatibility requirements in the approved product baseline:

1. The project does not detect or label whether code was AI-generated.
2. The project does not rank developers or turn maintenance evidence into individual performance surveillance.
3. Deterministic evidence and an explicit policy decision are required before a finding can block a change.
4. TypeScript/JavaScript and Python are the first syntax-aware language families.
5. Git history is a first-class input, not optional decoration around a snapshot scanner.
6. AI is optional assistance and is never the source of a metric, finding, severity, confidence, or policy decision.
7. Basic value never requires source-code upload or a hosted account.
8. Windows, macOS, and Linux require native support; a container image does not substitute for native support.

See [Non-goals](./non-goals.md) for the broader refusal boundary.

## Determinism contract

Given the same project version, configuration, repository object database, base and head revisions, installed analyzer set, and external metadata snapshot, the project must produce the same semantic findings, fingerprints, metrics, and policy decisions.

Execution timestamps, machine-local paths, filesystem enumeration order, host operating system, locale, time zone, terminal capabilities, and execution timing must not affect semantic output. Explicitly identified non-semantic run metadata may vary and must remain separate from canonical report content.

Full and incremental analysis must be semantically equivalent. A platform-specific implementation difference is a defect unless a documented capability degradation makes the scan explicitly incomplete; it may not silently change a pass/fail decision.

## Explainability contract

Every finding must contain:

- A stable rule identifier and rule version.
- A concise claim.
- Stable identities for affected entities.
- Base-state and head-state evidence.
- The calculation, relationship, or exact policy rule that triggered it.
- Severity and independently represented confidence.
- Limitations and degradation notices.
- A stable fingerprint that excludes volatile wording, line numbers, timestamps, and machine paths.
- A documented suppression mechanism.
- A non-destructive suggested next action.

A finding that cannot meet this contract cannot block a change. Line ranges are display hints, not finding identity.

## Privacy and security contract

The open CLI and engine:

- Make no network request unless the user explicitly invokes or enables a feature that requires one.
- Send no telemetry by default.
- Never upload source code implicitly.
- Remain useful when all upload and hosted features are disabled.
- Never execute repository build scripts, package-manager lifecycle hooks, tests, binaries, Git hooks, external diff/text-conversion programs, or checked-in plugins during a default scan.
- Treat every repository, filename, parser input, Git configuration, and plugin message as untrusted input.
- Keep source excerpts out of persisted reports unless explicitly enabled and bounded.
- Store caches in documented platform-appropriate locations that can be deleted safely.
- Support complete export and deletion of derived local data.

Any future build-requiring enrichment must be a separate, explicit operation with a visible capability request and an appropriate sandbox. It cannot weaken the default non-execution boundary.

## Portability contract

Users own their results. Native reports, graph exports, suppressions, configuration, policy, and plugin protocols are documented and independently versioned. SQLite is a local implementation detail, not the interchange format.

Hosted users can export the same logical data consumed and produced by the open scanner. Export and deletion must not be paywalled or deliberately degraded. Public schemas must not contain a required commercial account identifier.

## Open-core contract

The complete deterministic scanner remains open and able to run indefinitely in production on a user's own hardware. The open project includes:

- CLI, engine, Git/history ingestion, and local SQLite graph/cache.
- TypeScript/JavaScript and Python adapters.
- All core analyzers and their evidence.
- Local multi-repository workspaces.
- Native report schemas and terminal, JSON, SARIF, and HTML outputs.
- Typed local policy and suppressions.
- GitHub Action, plugin protocol/SDKs, local dashboard, import/export, and data deletion.
- Private-repository scanning, unlimited local history subject only to the user's hardware, and public security fixes.

Paid products may add managed operation, durable organization history, collaboration, identity/team mapping, centrally governed policy, cross-repository organization views, integrations, private-runner orchestration, enterprise access controls, data residency, support, and contractual assurance.

Paid products must not:

- Use a secret or more accurate analyzer unavailable to the open project.
- Produce a blocking finding that cannot be reproduced locally from the same inputs, scanner version, configuration, and policy.
- Hide evidence, basic suppression, export, deletion, security fixes, private-repository scanning, local execution, or core analyzer use behind payment.
- Charge by lines of code or create technical lock-in through withheld data.
- Patch or fork scanner behavior privately; generally useful scanner changes must be contributed through public contracts first.

The same rule must produce the same finding in local, CI, and hosted modes when its declared inputs are the same.

## Cross-platform promise

Windows, macOS, and Linux are first-class native operating-system families. By 1.0, at least one documented native release target in each family must meet the project's release-blocking support tier. Containers may be an additional distribution and isolation option, but they do not satisfy the native support promise.

An OS-family commitment is not an unsupported claim about every version or CPU architecture. [ADR 0007](./adr/0007-phase-1-native-release-matrix.md) selects Ubuntu 24.04 x86-64, macOS 15 arm64, and Windows Server 2025 x64 as the Phase 1 qualification targets, with `.tar.gz`, `.tar.gz`, and `.zip` archives respectively. These remain qualification targets—not supported-target claims—until installation, security, offline, and semantic-parity evidence passes for each row.

## Contract verification and change

Verification evidence belongs in tests, release artifacts, research records, or the project dashboard; prose alone does not satisfy these contracts. At minimum, releases must preserve deterministic fixture reports, no-network and non-execution checks, schema validation, export/deletion tests, and cross-platform semantic parity for the published support matrix.

This contract governs until a public RFC explains the user impact and an accepted ADR records the replacement decision. Verification still requires tests and release evidence; approval of this document alone is not proof that an implementation or platform gate passed.
