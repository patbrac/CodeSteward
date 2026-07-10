# ADR 0002: Local graph and cache storage

## Status

Accepted

## Date

2026-07-10

## Context

The local engine needs an embedded store for repository/revision identities, canonical entities and observations, graph projections, analyzer results, incremental invalidation keys, suppressions, and migrations. The initial workload is a single local workspace; a separately operated graph database would add installation, network, security, and portability costs before measured need exists.

## Decision

Use SQLite for derived local graph projections and caches in the 0.x engine.

SQLite is an implementation detail. Documented versioned JSON schemas are the public interchange boundary. Cache migrations must be transactional and recoverable, and the CLI must retain a documented migration window. Persist hashes and structural metadata instead of source content whenever practical. Do not introduce a separate local graph database in 0.x; reconsider it only with measured query or scale evidence through a new ADR.

## Alternatives considered

- **Flat JSON or line-oriented cache files:** Rejected because multi-table graph projections, atomic updates, indexed queries, and recoverable migrations would have to be rebuilt poorly in application code. JSON remains the public export format.
- **Embedded key-value store:** Rejected for 0.x because the initial access patterns are relational and inspectability plus transactional schema migration matter more than raw key lookup throughput.
- **Separate local graph database:** Deferred until measured workloads exceed SQLite budgets. It would add a service lifecycle, network surface, installation burden, and cross-platform operational matrix before evidence justifies them.
- **Hosted-only persistence:** Rejected because offline/local use, user-owned results, and an open scanner that works without an account are product contracts.

## Consequences

- Local operation remains embedded, inspectable, backup-friendly, and offline-capable.
- Schema design, indexes, precomputed projections, and migration discipline become core engineering work.
- SQLite locking, filesystem atomicity, and path placement must be tested on every supported OS family.
- Public consumers cannot depend on database tables or copy a cache as if it were a stable export.
- Hosted services may use other storage internally but must consume and emit public logical contracts.

## Verification

- Interrupt every migration boundary and prove rollback preserves the previous usable cache.
- Verify full scans and cache-restored/incremental scans produce equivalent canonical reports.
- Validate the native export independently of SQLite and import it across supported OS families.
- Verify platform-appropriate cache location, documented deletion, and complete derived-data export/deletion.
- Exercise concurrent access, file locking, corruption diagnostics, size limits, and recovery on the eventual Tier 1 matrix.
- Do not claim cross-platform storage support until those tests have archived evidence.

## Supersession

Replacing SQLite or making a cache/export format authoritative requires a public RFC and a new ADR that explicitly supersedes ADR 0002. The change must include measured query/scale evidence, transactional migration and rollback from supported cache versions, export/import parity, corruption recovery, and native locking/concurrency results on every Tier 1 target. Public JSON contracts and local data export/deletion must remain compatible or receive a documented versioned migration and deprecation window.
