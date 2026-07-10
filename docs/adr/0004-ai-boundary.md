# ADR 0004: AI authority and data boundary

## Status

Accepted

## Date

2026-07-10

## Context

Models may make evidence easier to understand or help propose remediation, but model output is non-deterministic, provider-dependent, and capable of unsupported claims. Making it authoritative would violate reproducibility, offline value, privacy, and policy safety. Detecting whether code is AI-generated is also unreliable and unrelated to the maintenance consequence the project measures.

## Decision

AI is an optional, provider-neutral adapter for explanation, querying, and non-destructive remediation suggestions only. Deterministic observations, findings, metrics, severity, confidence, fingerprints, and policy decisions remain authoritative.

The project will not detect or label AI-generated code. AI cannot create, remove, escalate, or make a finding block. Generated fields are structurally separate, visibly labeled, independently deletable, and record prompt/model/provider/version provenance. Source sharing requires explicit configuration, a preview of the exact payload, and consent. A local/private inference path is required before AI assistance is considered complete. Customer source, prompts, or findings cannot be used for model training without explicit separate consent.

Optional AI work begins only after deterministic 1.0 contracts are stable.

## Alternatives considered

- **Let a model create scores, findings, severity, confidence, or blocking decisions:** Rejected because output would be non-reproducible, difficult to test, and structurally unsafe for policy.
- **Require a hosted model for the first useful scan:** Rejected because it violates offline value, provider neutrality, source-control expectations, and no-account onboarding.
- **Detect or rank “AI-generated” code:** Rejected as unreliable, easily gamed, surveillance-prone, and unrelated to the maintenance consequence being measured.
- **Store generated text inside canonical findings:** Rejected because regeneration or provider changes would contaminate stable finding identity and retention/deletion boundaries.
- **Prohibit AI forever:** Deferred rather than adopted. Optional explanation or remediation can be evaluated after deterministic 1.0 if it remains non-authoritative and separately consented.

## Consequences

- Every useful core workflow must operate without a model, account, network, or API key.
- Generated explanations may be stale, speculative, or regenerated without changing finding identity.
- Provider changes affect assistance fields only.
- Suggested patches are never auto-applied and must include verification guidance.
- Product differentiation cannot depend on prompts or exclusive access to one provider.

## Verification

- Run the complete corpus with AI disabled and enabled; canonical findings, metrics, fingerprints, and policy decisions must be identical.
- Prove the report/schema type system cannot map generated content into a policy input.
- Deny networking and verify all deterministic workflows remain complete.
- Verify no source leaves the machine without explicit configuration, payload preview, and recorded consent.
- Swap providers and confirm only generated-assistance fields change.
- Delete generated content/provenance without deleting deterministic findings.
- Evaluate unsupported claims, hallucinations, unsafe remediation, and comprehension before release.

## Supersession

Any expansion of AI authority or source-sharing behavior requires a public RFC and a new ADR that explicitly supersedes ADR 0004 and the corresponding product-contract language. The proposal must preserve deterministic no-provider parity or clearly version the affected public contract; prove through schema/type tests that generated data cannot silently enter policy; document provider, consent, retention, deletion, and training terms; and include privacy/security review plus human evaluation. AI cannot become blocking merely through a minor implementation or prompt change.
