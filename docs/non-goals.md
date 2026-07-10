# Code Steward non-goals

**Status:** Approved product boundary
**Date:** 2026-07-10

The project is intentionally not:

- A compiler, linter, SAST scanner, dependency vulnerability scanner, or replacement for those tools.
- A general-purpose AI code reviewer.
- A detector or classifier for “AI-generated” code.
- A developer ranking, employee evaluation, productivity-scoring, or performance-surveillance system.
- A definitive measure of an individual's knowledge, value, or employment risk.
- A product that reports commit counts or “debt introduced” as individual productivity.
- A universal maintainability score or opaque repository grade in its initial releases.
- An authority that declares one directory structure, layer count, test style, architecture, or complexity threshold universally correct.
- A compiler-build-dependent analysis system in its default path.
- An automatic refactoring or patch-application engine in its initial releases.
- An incident-prediction oracle.
- A source-upload-required service.
- A cloud-account-required scanner.
- A custom vulnerability database or duplicate software-composition-analysis backlog.
- A system that silently fetches history, submodules, LFS objects, dependencies, or external evidence.
- A platform whose core result depends on a particular model, provider, Git host, graph database, or commercial control plane.

These non-goals do not prevent interoperable imports, explicit sandboxed enrichment, optional AI explanations, or non-destructive suggested actions. Those features must preserve the product contract and expose their limitations.

Changing a non-goal requires a public RFC and an accepted ADR because it changes the project's trust boundary or category.
