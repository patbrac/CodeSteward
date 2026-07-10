# Report schemas v0

> **Schema-host notice:** The `$id` host is deliberately in the reserved `.invalid` namespace. It is a non-fetching identifier until a durable public schema host and migration policy are approved.

**Draft:** 0.1.0
**JSON Schema dialect:** Draft 2020-12
**Stability:** Experimental Phase 0 contract; not a 1.0 compatibility promise

This directory defines the first machine-readable contracts for deterministic evidence, findings, the six-dimension maintenance delta, and a report envelope that separates semantic content from run-local metadata.

## Artifacts

- `evidence.schema.json` — provenance, revision context, calculation or exact-rule basis, confidence, and limitations.
- `finding.schema.json` — deterministic finding identity, closed fingerprint basis, embedded evidence, non-destructive action, and block eligibility.
- `maintenance-delta.schema.json` — explicit base/head observation sets, metrics, finding states, and all six initial dimensions.
- `report-envelope.schema.json` — scan inputs, findings, delta, policy decisions, diagnostics, completeness, and isolated non-semantic metadata.

Every schema has an absolute `$id`, a `schema_version`, reusable `$defs`, and closed contract objects where unknown properties would change meaning. The `.invalid` identifiers are intentionally non-fetching. A standards validator must preload the four local resources keyed by their `$id`; validation must not require network access.

Within v0, these `$id` values identify this contract family and must not be silently reused for incompatible content. Selecting a durable public schema host before v1 remains an open technical decision and will require an explicit migration/alias policy.

## Semantic boundary

Each artifact has a required `semantic` object and an optional `non_semantic_metadata` object.

- `semantic` contains reproducible inputs and results. It participates in canonical report comparison.
- `non_semantic_metadata` contains run IDs, rendering locations/order, host information, timing, timestamps, or optional machine paths. It never participates in a fingerprint, policy decision, cache identity, or semantic equality comparison and may be stripped safely.
- Writers should omit sensitive machine paths by default even though the report envelope can classify them as non-semantic diagnostics.
- Repository display paths must be repository-relative, use `/`, and are never fingerprint inputs.

The only finding fields allowed to feed a fingerprint are inside `semantic.fingerprint_basis`. That object is closed and contains only rule identity/major version, stable subject and relationship IDs, and normalized rule parameters. It has no line, column, path, timestamp, date, locale, time zone, host, OS, architecture, duration, run ID, or display-order field.

Schema shape alone cannot recognize every timestamp disguised as an arbitrary token. Implementations must additionally enforce these fingerprint canonicalization rules:

1. Use stable repository/entity/relationship IDs, never filesystem paths.
2. Treat `subject_ids`, `relationship_ids`, and parameters as sets and sort them using locale-independent UTF-8 byte ordering before hashing.
3. Ignore JSON object member order.
4. Serialize numbers without locale-specific separators or formatting.
5. Reject volatile parameter names and values rather than normalizing machine-local data into the hash.
6. Exclude all `non_semantic_metadata`, evidence display locations, wording, and execution diagnostics.

The exact canonical JSON and digest algorithm is still an open Phase 1 implementation decision. The v0 fingerprint string reserves the `mif:v0:` namespace followed by a lowercase 64-hex digest.

## Evidence-first policy boundary

Every finding requires at least one complete evidence object, confidence, limitations (which may be empty), an explanation, and a non-destructive suggested action.

`block_eligibility.status = eligible_with_policy` means only that deterministic evidence is complete enough for an external policy to consider. The schema condition also requires `evidence_completeness = complete` and the `deterministic_evidence_complete` reason code. Eligibility is not a block decision.

Only a typed `policy_decision` in the report envelope can record `fail`, and a failing decision must reference at least one finding fingerprint. Phase 1 semantic validation must also prove that every referenced finding exists, is eligible, remains unsuppressed, meets policy confidence requirements, and came from a trusted deterministic rule. JSON Schema cannot enforce those cross-instance references by itself.

AI-generated content has no field in these closed semantic contracts. If optional generated assistance is introduced after deterministic contracts stabilize, it requires a separate explicitly non-authoritative schema and cannot become a policy input.

## Examples and expected outcomes

The examples are intentionally small and hand-reviewable.

| File | Schema | Expected standards-validation outcome | Purpose |
|---|---|---|---|
| `examples/valid/evidence.json` | Evidence | Valid | Calculation evidence with provenance, confidence, a limitation, and repository-relative display location. |
| `examples/valid/finding.json` | Finding | Valid | Exact boundary finding eligible for explicit policy because evidence is complete. |
| `examples/valid/maintenance-delta.json` | Maintenance delta | Valid | Base/head snapshots and all six dimensions. |
| `examples/valid/report-envelope.json` | Report envelope | Valid | Failing typed policy decision plus Windows-only run metadata isolated outside semantics. |
| `examples/invalid/evidence-machine-path.json` | Evidence | Invalid | A Windows absolute path is used where only repository-relative `/` display paths are allowed. |
| `examples/invalid/finding-eligible-without-evidence.json` | Finding | Invalid | A block-eligible finding has no evidence. |
| `examples/invalid/maintenance-delta-missing-dimension.json` | Maintenance delta | Invalid | The required dependency-burden dimension is absent. |
| `examples/invalid/report-authoritative-ai.json` | Report envelope | Invalid | An undeclared AI policy field is inserted into closed semantic content. |

## Validation status

All checked-in `.json` files must parse as JSON, including intentionally schema-invalid fixtures. The primary repository check is shell-neutral and runs on Windows, macOS, and Linux from the repository root:

```text
python scripts/ci/check_schema_examples.py
```

Python is a CI and contributor-tooling dependency only. It is not a runtime dependency of the native CLI or engine.

PowerShell can optionally perform a schema-directory-only JSON syntax check:

```powershell
$files = Get-ChildItem schemas/report/v0 -Recurse -Filter *.json
$files | ForEach-Object { Get-Content -Raw -LiteralPath $_.FullName | ConvertFrom-Json | Out-Null }
```

The command validates every checked-in schema with a Draft 2020-12 implementation, registers local `$id` resources without network access, requires every positive fixture to pass, and requires every negative fixture to fail. CI runs the same command; its recorded result is the evidence for the schema-example gate.

## Compatibility and open questions

- Select a durable public `$id` namespace and define how v0 identifiers alias or migrate.
- Select the canonical JSON serialization and fingerprint hash specification.
- Finalize repository, entity, relationship, observation-set, and external-snapshot identity formats.
- Decide whether evidence should remain embedded in each finding or be deduplicated into an envelope evidence collection.
- Define schema-level rule/fingerprint version migration and suppression behavior.
- Add cross-instance checks for rule ID consistency, duplicate fingerprints, policy references, and suppressed findings.
- Decide which externally sourced timestamps are semantic snapshot attributes; they must never become fingerprint inputs.
- Decide whether report non-semantic machine paths should be prohibited entirely rather than opt-in and strip-safe.
- Publish compatibility rules before a v1 directory is created. No consumer should assume v0 forward compatibility.
