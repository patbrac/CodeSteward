# Threat model

## Scope and objectives

This model covers the open Code Steward CLI, deterministic engine, Git/history readers, parsers, local SQLite state, renderers, schemas, release artifacts, and future out-of-process plugin protocol. Hosted control-plane infrastructure requires its own model and cannot weaken these local boundaries.

Security objectives are to preserve:

- **non-execution:** a normal scan never runs repository-controlled code;
- **containment:** repository input cannot read, overwrite, or identify data outside the authorized root;
- **confidentiality:** source, history, secrets, identities, and local paths are not transmitted or exposed by default;
- **integrity:** findings, evidence, policy results, updates, and artifacts cannot be silently substituted or made nondeterministic;
- **availability:** hostile input cannot consume unbounded time, memory, disk, processes, terminal output, or recursion;
- **user control:** network, source transmission, build execution, and plugins are separately named and explicitly enabled.

Code Steward does not claim to prove that a repository is safe or vulnerability-free. The operating system, explicitly selected Code Steward executable, Rust runtime, and local user account are trusted. Repository contents and configuration, Git metadata, filenames, source text, external evidence, plugin binaries/messages, environment-derived display text, and downloaded artifacts are untrusted.

## Data flow and trust boundaries

```text
untrusted repository/history/config
             |
             v
 bounded readers -> canonical observations -> deterministic analyzers
             |                 |                       |
             +------ diagnostics/evidence ------------+
                                      |
                       terminal / JSON / SARIF / HTML
                                      |
                         local cache and user export

explicit, separate boundary: user-approved plugin process
explicit, future boundary: user-approved network/source transmission
```

The default path ends on the user's machine. Merely opening a repository must not activate a plugin, network client, package manager, language runtime, compiler, or build system.

## Threats and required controls

### Hostile repositories and Git behavior

Attackers may craft objects, refs, commit metadata, encodings, very deep trees, corrupt pack data, ambiguous revisions, submodules, attributes, config, replace objects, mailmaps, and filenames to crash the scanner, escape its root, trigger Git helpers, or forge evidence.

Controls:

- Read repository objects through bounded library/data APIs. Do not invoke repository hooks, aliases, filters, clean/smudge processes, credential helpers, external diff/textconv, pagers, editors, submodule commands, LFS clients, or configured shell commands.
- Resolve revisions to immutable object identifiers and report ambiguity or corruption explicitly.
- Treat repository-local and worktree Git configuration as data, not instruction. Allow only reviewed semantic keys; ignore or reject executable keys.
- Never add the repository directory to executable/library/module search paths.
- Keep parser faults and malformed history as contained diagnostics, not panics or silent omission. Mark results incomplete when a safety limit affects coverage.
- Use synthetic malicious fixtures and fuzz parsers/readers; never use private source in public crash artifacts.

### Paths, symlinks, junctions, and Windows reparse points

Attackers may use `..`, absolute/drive-relative paths, UNC/device namespaces, alternate data streams, reserved names, mixed separators, case collisions, Unicode normalization, symlink loops, mount points, junctions, or other reparse points. A path can change between validation and open.

Controls:

- Convert observations to canonical repository-relative identities; never place a machine root in a stable ID or report.
- Reject NUL/control characters, root escapes, disallowed absolute/UNC/device paths, and ambiguous collisions with a deterministic diagnostic.
- Do not follow filesystem indirection by default. When a deliberate feature reads a working-tree target, open it with platform-safe no-follow semantics where available, inspect each component/handle, and prove containment after resolution.
- Treat every Windows reparse point as indirection until its tag and containment policy are explicitly handled; a junction is not assumed equivalent to a safe Unix symlink.
- Bound link depth, detect loops, and fail closed on races or unverifiable containment. Validate immediately around the actual open rather than trusting a prior string check.
- Test case-sensitive/insensitive filesystems, NFC/NFD, long paths, drive letters, UNC/device forms, reserved names, alternate streams, mixed separators, in-root/escaping/broken links, junctions, and reparse loops natively.

### Terminal, log, and report injection

Commit text, author names, paths, findings, plugin messages, and parser errors may contain ANSI/OSC escapes, bidi controls, newlines, hyperlinks, tabs, or delimiter-like text that changes the display, clipboard, window title, CI log, SARIF/HTML interpretation, or following record.

Controls:

- Escape or strip terminal control sequences from all untrusted fields; generate styling only from trusted renderer tokens. Never pass untrusted text through a shell.
- Preserve record boundaries and make replacement/escaping visible. Bound field and diagnostic length.
- Use structured encoders for JSON/SARIF and context-correct HTML escaping with a restrictive content-security policy. Do not concatenate markup.
- Keep stdout machine-readable when promised and diagnostics on stderr. Test TTY/redirection and native PowerShell/Windows Terminal behavior.
- Normalize only where the public schema says to; retain safe evidence that explains any lossy display sanitization.

### Resource exhaustion

Small repositories can contain huge blobs, deep syntax, compressed expansion, massive history, rename explosions, path fan-out, parser worst cases, plugin floods, SQLite growth, or endless diagnostics.

Controls:

- Define configurable hard limits for blob size, object/tree count, history range, parse depth/time, rename candidates, graph nodes/edges, diagnostics, memory, temporary/cache bytes, plugin frame size, and subprocess duration.
- Apply streaming and bounded queues; avoid loading whole histories or arbitrary blobs when a bounded read suffices.
- Cancel cooperatively, terminate child process trees on timeout/cancel, and remove partial temporary output. On Windows use a Job Object or equivalent process-tree control; on POSIX use a process group/equivalent.
- Make limit-triggered incompleteness explicit in diagnostics and manifests. Never present partial coverage as a clean result.
- Use transaction boundaries, disk-space checks, busy timeouts, and recoverable migrations for SQLite. Cache deletion must be safe and complete.

### Plugins and imported evidence

Plugins are untrusted executables even when they use a valid manifest. They may read source, access the network, hang, spawn descendants, return malformed/huge messages, impersonate built-ins, or produce unsupported evidence.

Controls:

- No implicit plugin discovery or execution during a default scan. The user selects a plugin and approves its declared capabilities.
- Run plugins out of process with a versioned, length-delimited protocol, schema validation, bounded frames, timeouts, cancellation, deterministic environment, and stdout/stderr separation.
- Assign a distinct namespace and provenance to plugin findings. Plugin output cannot claim built-in authority or bypass deterministic evidence/policy eligibility.
- Grant the minimum source/history paths required. A future sandbox must state its actual OS guarantees; absence of a network or filesystem sandbox must be disclosed, not implied.
- Pin/check plugin identity where configured, do not shell-expand arguments, and clean up complete process trees.

### Secrets, source, and private history

Repositories may contain credentials, personal data, regulated source, private URLs, and sensitive authorship/history. Evidence excerpts, debug logs, cache, crash reports, terminal output, HTML, CI artifacts, or hosted features could leak them.

Controls:

- Default to no telemetry, no network, and no source upload. Network/source transmission is a separate explicit operation with destination, data classes, and retention visible before consent.
- Persist hashes, bounded metadata, and structural facts instead of source excerpts unless the user explicitly enables a documented output. Redact tokens, URLs, environment values, absolute roots, and credentials from logs/errors.
- Put config/cache/temp files in documented per-user OS locations with restrictive permissions where supported. Avoid shared world-readable temp names and follow safe atomic-create semantics.
- Make report/export contents inspectable and provide complete local cache/export deletion. Never send crash dumps automatically.
- Treat CI artifacts as disclosure surfaces: use synthetic fixtures and avoid dumping untrusted source on failure.

### Supply chain and release substitution

Dependencies, build scripts, package registries, CI actions/runners, maintainer credentials, release archives, SBOMs, and update instructions can be compromised.

Controls:

- Commit and enforce `Cargo.lock`; allow reviewed registries/Git sources only; minimize dependencies/features and audit licenses/advisories under the [dependency policy](./dependency-policy.md).
- Pin third-party CI actions by immutable commit, use least-privilege workflow tokens, isolate untrusted pull requests from release credentials, and require reviewed protected release workflows.
- Produce per-artifact SHA-256 checksums, SBOMs, and GitHub build-provenance attestations bound to source revision/workflow. Publish independent verification instructions.
- Do not describe an artifact as Authenticode-signed or Apple-notarized until those platform-native checks exist. Never overwrite a published version.
- Require phishing-resistant MFA where available, least-privilege publisher roles, access review, credential rotation/revocation, and documented release rollback.

### Configuration, policy, and determinism confusion

Repository configuration could weaken safety limits, hide findings, select remote services, load plugins, or alter output. Nondeterminism can make an attacker-controlled result appear legitimate on only one platform.

Controls:

- Define configuration precedence and whether each file is trusted. Repository-owned config cannot enable execution, network, plugins, source transmission, or exceed safety ceilings without explicit trusted-user approval.
- Validate typed configuration before scanning; reject unknown security-sensitive fields and unsafe paths.
- Record effective configuration and analyzer versions without secrets or absolute machine roots.
- Canonically sort outputs and avoid locale, time, randomized iteration, platform separators, or machine state in semantic identities.

## Security verification baseline

Required suites include malformed Git/parser fuzzing; non-execution canaries; no-network default-command tests; path containment/link/reparse fixtures; control-sequence rendering; resource-limit/cancellation/process cleanup; config capability-escalation rejection; SQLite interruption/recovery; dependency/license/secret scanning; release checksum/SBOM/attestation verification; and canonical report comparison on every Tier 1 native target.

Tests must verify both success and safe failure. A document or ADR is not evidence that a control passed.

## Residual risks and future review

Native parsers and Git libraries may contain memory-safety or algorithmic flaws; filesystem race resistance varies by OS; an explicitly launched unsandboxed plugin has the user's privileges; terminal implementations differ; GitHub-hosted runners and registries remain external trust; and metadata alone may be sensitive. These risks require defense in depth, disclosure, updates, and bounded defaults—not claims of perfect isolation.

Review this model before enabling plugins, any network/source-transfer feature, repository build enrichment, a new parser/Git backend, automatic updates, platform-native signing, hosted scanning, or a changed OS support tier. Material boundary changes require an ADR and, when they alter the product contract, a public RFC.
