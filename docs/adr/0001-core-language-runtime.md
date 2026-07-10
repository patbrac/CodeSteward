# ADR 0001: Core language and runtime

## Status

Accepted

## Date

2026-07-10

## Context

The engine must inspect large, untrusted repositories with predictable resource use; produce deterministic output; support incremental parsing and history analysis; and ship as native software on Windows, macOS, and Linux. Requiring a language runtime or project build would add installation friction and expand the execution boundary. TypeScript would accelerate early development, but it is a weaker fit for single-binary distribution, parser isolation, memory control, and long-running multi-language workloads.

## Decision

Implement the CLI, reusable engine, repository adapter, canonical observation layer, local store, policy evaluator, report model, and built-in analyzer runtime as a Rust Cargo workspace.

The CLI remains a thin adapter over reusable engine APIs so CI integrations, the local dashboard backend, and future workers cannot acquire different scanner semantics. Language adapters sit behind versioned language-neutral observation interfaces. Third-party plugins use a language-neutral out-of-process protocol; contributors do not need Rust to add fixtures, schemas, importers, or external analyzers.

The UI may use a web ecosystem, but UI or integration packages cannot become dependencies of the engine.

## Alternatives considered

- **TypeScript/Node.js core:** Rejected for the core because the runtime dependency, memory-control model, parser isolation, and large-history workload conflict with the native single-binary and predictable-resource goals. TypeScript remains appropriate for UI work and a plugin SDK.
- **Python core:** Rejected for the core because packaging a consistent native CLI and bounding CPU/memory behavior across three OS families would add runtime and distribution complexity. Python remains a supported plugin/importer language.
- **Split implementation with a scripting-language CLI owning business logic:** Rejected because local, CI, dashboard, and hosted callers could acquire different semantics. The CLI must remain a thin Rust adapter over one reusable engine.
- **Build/compiler-required semantic engine:** Deferred to an explicit sandboxed enrichment path. It cannot be the default engine because it expands execution risk and weakens clone-only onboarding.

## Consequences

- Native distribution, memory safety, Tree-sitter integration, and controlled concurrency are favored.
- Rust's contributor learning curve and cross-compilation/release complexity become project costs.
- The project must provide strong fixture documentation, generated protocol types where useful, and narrowly scoped contribution paths.
- Core logic cannot drift into a GitHub Action, renderer, dashboard, or commercial service.
- A native binary does not remove the obligation to test OS-specific filesystem and process behavior.

## Verification

- Build and test the Cargo workspace on every published Tier 1 target once the exact support matrix is approved.
- Prove the CLI and at least one non-CLI caller use the same engine API and produce canonical semantic parity.
- Enforce dependency-direction checks so the engine has no renderer, GitHub, cloud-account, or commercial dependency.
- Run parser, repository, fuzz, resource-limit, and no-execution suites against the Rust implementation.
- Archive binary installation and semantic-parity evidence before calling a platform supported; this ADR records the choice, not current CI evidence.

## Supersession

Superseding Rust as the core, moving scanner semantics into another runtime, or making builds mandatory requires a public RFC and a new ADR that explicitly supersedes ADR 0001. The proposal must document public API, schema, plugin, cache, packaging, and migration compatibility; compare deterministic output and resource/security behavior on the published corpus; and include native Windows, macOS, and Linux build/install evidence. A rollout and deprecation window is required before removing a supported interface.
