# Roadmap

This roadmap describes intended outcomes, not release promises. Dates and scope change through the decision process in [GOVERNANCE.md](GOVERNANCE.md). A checked implementation milestone still needs its verification and release gates before it is called supported.

## Current: secure build foundation

Phase 1 establishes the public Rust workspace, cross-platform CI targets, Apache-2.0 and DCO governance, versioned public schemas, a minimal CLI (`version`, `doctor`, and `config validate`), supply-chain controls, security reporting, and development release provenance.

Success means a new contributor can perform a clean, documented build; the dependency and schema audits pass; a private security report can be tested; and a development artifact can be independently verified and installed natively on the published Windows, macOS, and Linux targets.

## Next: deterministic scanner

1. **Git ingestion and canonical repository model** — read untrusted Git history without executing repository code; normalize paths and identities; store transactional local cache state.
2. **TypeScript/JavaScript and Python structure** — parse tolerant, versioned structural entities while preserving evidence and uncertainty.
3. **Maintenance graph and incremental engine** — combine source structure and history into a reproducible graph with full/incremental equivalence.
4. **Core maintenance analyzers** — orphaned ownership, boundary drift, change coupling, coordination cost, hotspot concentration, interface risk, churn, and structural duplication.
5. **Evidence-first outputs and policy** — stable terminal, JSON, SARIF, and HTML reports; typed suppressions; policy gates based only on deterministic evidence.
6. **CI and pull-request workflow** — useful change-scoped reports, stable fingerprints, fork-safe execution, and bounded annotation volume.
7. **Multi-repository workspaces, local dashboard, and history** — local portfolio use without requiring a hosted account.
8. **Plugin protocol and external evidence import** — capability-declared, isolated extensions using documented public contracts.
9. **Beta hardening and 1.0** — stable schemas and CLI, migration and deprecation guarantees, signed reproducible artifacts, native platform parity, and complete operator documentation.

Optional AI explanation and remediation adapters come only after deterministic 1.0 contracts are stable. They may explain or draft, but never originate authoritative evidence or silently change policy outcomes.

## Open-core boundary

Every deterministic analyzer, core language adapter, local policy feature, public output, private-repository scan, security fix, and local import/export capability remains open. Commercial work may provide managed operations and organization-scale collaboration; it must reproduce the same scanner result from the same declared inputs. See the [product contract](docs/product-contract.md).

## Release and compatibility policy

Release stages, support windows, schema compatibility, and deprecation requirements are documented in the [release policy](docs/governance/release-and-deprecation.md). User-facing contract, license, governance, and major roadmap changes require a public RFC. Architecture and schema implementation changes require an ADR.
