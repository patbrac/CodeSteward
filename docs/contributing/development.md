# Development guide

Start with the repository-root [contribution guide](../../CONTRIBUTING.md). This page explains project-specific safety and portability review.

## Bounded local loop

Run the same workspace checks natively on your platform:

```text
cargo fmt --all -- --check
cargo clippy --workspace --all-targets --all-features -- -D warnings
cargo build --workspace --locked
cargo test --workspace --locked
cargo doc --workspace --no-deps
```

These commands have equivalent syntax in POSIX shells and PowerShell. Do not wrap them in a shell-specific script as the only documented path. When debugging a single test, use Cargo package/test filters and finish by running the bounded workspace suite.

## Cross-platform review

Normalize stable identities to repository-relative `/`-separated paths, but use native path APIs for filesystem access. Do not parse Windows paths with POSIX assumptions or convert user paths by string replacement. Add fixtures for drive/UNC/device paths, reserved names, case collisions, Unicode NFC/NFD, CRLF/LF, long paths, symlinks, junctions, reparse points, native terminal redirection, child cleanup, and SQLite locking when affected.

Machine roots, username, locale, wall-clock time, filesystem enumeration order, and randomized map order must not affect semantic reports. If a platform difference is intentionally non-semantic, encode and test that boundary.

## Hostile-input review

Ask for each new reader/parser/renderer:

1. What bytes, metadata, names, paths, config, or messages can an attacker control?
2. Can they trigger code execution, network, root escape, source/secret disclosure, terminal control, or unbounded resources?
3. Which hard limit applies and how is incompleteness reported?
4. Does failure leave transactions, temporary files, subprocesses, caches, or partial output?
5. Are tests synthetic, deterministic, and native where OS behavior matters?

Never invoke a shell with repository data. New plugin, execution, network, or source-transfer capability needs threat-model review and an ADR; product-contract changes also need an RFC.

## Fixtures and diagnostics

Use the smallest synthetic fixture that demonstrates the invariant. Negative fixtures should prove safe rejection, not merely a crash-free path. Diagnostics go to stderr, remain bounded, sanitize control characters/secrets/machine paths, and say when limits or missing history make analysis incomplete.

Do not update a golden output until the behavior change is explained and reviewed. Compare semantic fields separately from approved run metadata.
