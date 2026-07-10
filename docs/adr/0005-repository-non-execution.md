# ADR 0005: Repository non-execution and security boundary

## Status

Accepted

## Date

2026-07-10

## Context

Every scanned repository is untrusted input. Git configuration, hooks, filters, filenames, symlinks, parser inputs, oversized histories, package metadata, and checked-in executables can cause code execution, data exposure, terminal injection, or resource exhaustion. Requiring a project build also produces slow and unreliable onboarding and expands the trusted computing base.

## Decision

Default scans only read repository objects and explicitly allowed files. They never execute build scripts, package-manager lifecycle hooks, tests, repository binaries, Git hooks, external diff/text-conversion programs, checked-in plugins, submodule commands, or LFS fetchers.

Repository reads must bypass executable Git helpers and filters. Paths are canonicalized and contained within the repository root; external symlinks/reparse points are rejected; device files, sockets, and other special files are unsupported. File size, parse depth, history, CPU, memory, wall-clock, observation, finding, and protocol-message limits use safe defaults. Terminal text is sanitized. Temporary data uses private platform-appropriate directories and is removed on normal and abnormal exit where practical.

Plugins are never trusted because a repository names them. They require explicit installation and allowance, run out of process, are pinned by digest, declare capabilities, and cannot declare their own findings trusted for blocking.

A future semantic enrichment that requires a build must be a separate explicit command with a visible capability request and sandbox; it cannot change default-scan behavior.

## Alternatives considered

- **Shell out to the user's Git executable with ambient configuration:** Rejected for repository ingestion because helpers, filters, hooks, external diff drivers, and environment-specific configuration expand the execution and determinism boundary.
- **Run package installation, builds, or tests automatically for better semantic precision:** Rejected for default scans because repositories are untrusted and lifecycle hooks can execute arbitrary code. Explicit sandboxed enrichment remains a future option.
- **Load repository-declared plugins in process:** Rejected because a checked-in declaration is not user consent and an in-process failure could corrupt the engine or cache. Plugins require explicit installation, digest pinning, capabilities, and process isolation.
- **Follow all symlinks and special files for completeness:** Rejected because root escape and device/socket behavior create data-exposure and resource risks. Unsupported inputs degrade with diagnostics.
- **Avoid parsing untrusted content entirely:** Rejected because it would eliminate core value. Bounded parsers, fuzzing, containment, and failure isolation are the selected risk controls.

## Consequences

- Some exact semantic relationships remain unresolved unless separately imported.
- Scans degrade with explicit diagnostics instead of executing repository-specific tooling.
- Cross-platform path containment and process isolation require OS-specific implementations and fixtures.
- Optional analyzer failure or timeout makes required output incomplete and cannot produce a partial blocking decision.
- Security takes precedence over convenience or silent completeness.

## Verification

- Instrument malicious fixtures to prove no hook, filter, diff tool, package script, repository binary, submodule command, or checked-in plugin executes.
- Test path traversal, symlink and Windows reparse-point escape, special files, Unicode/control filenames, terminal injection, oversized input, and resource exhaustion.
- Verify containment using canonical paths on Linux, macOS, and Windows rather than separator-only string checks.
- Fuzz Git metadata, parsers, configuration, plugin messages, and terminal rendering.
- Crash, time out, and overrun analyzers/plugins; confirm the core report/cache remains consistent and policy cannot falsely pass.
- Run default commands with networking denied and archive evidence before declaring a release supported.

## Supersession

Weakening the default non-execution boundary or adding a new repository-controlled capability requires a public security RFC and a new ADR that explicitly supersedes ADR 0005. It must update the threat model, describe user consent and sandbox boundaries, preserve the behavior of existing default commands or introduce an explicit versioned command, and provide malicious-corpus, fuzz, resource-limit, no-network, and native Windows/macOS/Linux containment evidence. Security review and a rollback path are mandatory; a convenience change cannot silently broaden execution.
