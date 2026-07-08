# CodeSteward v0 Non-Goals

CodeSteward v0 has a deliberately narrow scope. This document exists to prevent scope creep: the following are **out of scope for v0** and must not be built into it. Keeping this list explicit protects CodeSteward's core identity as a deterministic, maintainer-time protection tool.

## Not in v0

Do not include these in v0:

- **LLM-based code review** — CodeSteward does not use any language model to review code.
- **AI-generated code detection** — no attempt to guess whether code was written by AI.
- **Security vulnerability scanning** — CodeSteward is not a security scanner.
- **Deep semantic code analysis** — checks are path- and diff-based, not semantic.
- **Blocking checks** — CodeSteward never blocks a merge or fails a build on report content.
- **Auto-labeling** — no automatic PR/MR labels.
- **Auto-reviewer requests** — no automatic reviewer assignment or suggestions.
- **Contributor open-PR limits** — no cap on how many PRs a contributor may open.
- **Moderation rules** — no contributor moderation features.
- **SaaS dashboard** — no hosted web dashboard.
- **Full CI artifact report** — the output is a compact comment, not a full artifact report.
- **Enterprise governance features** — no org-level policy or governance layer.
- **Language-specific analyzers** — behavior stays language-agnostic.
- **Historical trend dashboard** — no review-burden trends over time.
- **SSO/SAML/SCIM** — no enterprise identity integration.

## Why these are excluded

CodeSteward's value in v0 comes from being **deterministic, explainable, and non-hostile**. Every excluded feature above either introduces non-determinism (AI review, AI detection), turns the tool into a gatekeeper (blocking, moderation, open-PR limits), or expands it into a platform (dashboards, governance, SSO) before the core comment has proven useful. The v0 product promise is one compact, helpful, reproducible readiness comment — nothing more.

Features deferred to later (v0.2 candidates and a future commercial layer) — such as opt-in auto-labeling, language presets, machine-readable exports, and hosted/private-repo reporting — are tracked in the phased build plan and must not be pulled forward into v0.1. The rule is simple: do not add later-phase features until v0.1 users confirm the core report is useful.
