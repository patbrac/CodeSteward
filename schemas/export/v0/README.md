# Export schema placeholder v0

This Draft 2020-12 schema reserves deterministic, portable export metadata. It
is intentionally metadata-only and marked `contract_status: placeholder`;
Phase 1 does not claim that import/export is implemented.

The eventual export contract must define canonical payload serialization,
complete public data coverage, streaming/resource limits, deletion behavior,
and schema migration. Machine-local paths, required commercial account IDs, and
implicit source excerpts remain outside this placeholder.

Examples under `examples/valid` must validate; examples under
`examples/invalid` must fail. The `.invalid` `$id` is resolved locally only.
