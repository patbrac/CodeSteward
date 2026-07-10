# Phase 1 Cargo workspace boundaries

The reusable engine owns product behavior. The CLI is only an adapter, policy/report types sit below the engine, and the report model is the leaf. The exact approved direct internal graph is:

~~~text
steward-cli -> steward-engine
steward-engine -> steward-policy
steward-engine -> steward-report
steward-policy -> (no workspace crate)
steward-report -> (no workspace crate)
~~~

These are direct workspace edges, not a statement about crates.io dependencies. Every listed non-empty edge is required in Phase 1 and every other workspace edge is forbidden. Policy and report are sibling inward-facing leaf crates. In particular, engine/policy/report cannot depend on the CLI; report and policy cannot depend on the engine or on each other.

**scripts/ci/check_public_contracts.py** validates the exact package set and edge set from locked Cargo metadata. **scripts/ci/test_check_public_contracts.py** injects a sample inverted report-to-engine edge and proves it is rejected. Any new workspace crate or edge requires an architecture review, an updated boundary document, and an updated executable allowlist in the same pull request.
