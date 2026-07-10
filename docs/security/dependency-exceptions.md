# Dependency policy exceptions

An entry here is required before a dependency may bypass an automated source, ban, license, or advisory rule. Empty means no approved exceptions.

| Package and version/source | Rule | Runtime/build/dev scope | Rationale and obligations | Compensating controls | Approvers | Approved | Expires/review | Replacement issue |
|---|---|---|---|---|---|---|---|---|
| `borrow-or-share` 0.2.4 / crates.io | MIT-0 is outside the global license allowlist | Dev-only transitive edge: `jsonschema` -> `referencing` -> `fluent-uri`; excluded from release binary graph | MIT-0 is OSI-approved and has no attribution condition; keep the exception package/version exact | `cargo deny` exact exception; locked graph; non-dev attribution generator excludes it; review on every jsonschema update | [`@patbrac`](https://github.com/patbrac) as bootstrap maintainer, security responder, and release manager | 2026-07-10 | 2026-10-10 or next jsonschema update, whichever is first | [`P1-DEP-001`](#p1-dep-001-remove-the-borrow-or-share-exception) |

Expired entries fail the dependency audit. Removing an exception is a normal pull request; extending one requires fresh evidence and the same approval level as the original.

## P1-DEP-001: remove the borrow-or-share exception

**Owner:** `@patbrac`
**Trigger:** every automated or manual update to jsonschema/referencing/fluent-uri, and no later than 2026-10-10.
**Done when:** the dev dependency edge disappears or its license is covered by a newly approved general policy; the exact `deny.toml` exception and this row are removed in the same reviewed change.
