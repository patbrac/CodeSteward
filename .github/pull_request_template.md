## Outcome

Describe the user-visible result and why this is the smallest coherent change.

## Evidence and tests

- [ ] `cargo fmt --all -- --check`
- [ ] `cargo clippy --workspace --all-targets --all-features -- -D warnings`
- [ ] `cargo build --workspace --locked`
- [ ] `cargo test --workspace --locked`
- [ ] Schema/golden/native-platform checks affected by this change
- [ ] Checks not run are named and explained below; no unrun check is marked passed

Evidence, fixtures, and checks not run:

## Trust and compatibility review

- [ ] Hostile inputs, resource bounds, terminal/log output, and failure cleanup were considered.
- [ ] The change does not add implicit repository execution, network access, source transmission, or plugin activation.
- [ ] Privacy/secrets and local data persistence/deletion were considered.
- [ ] Determinism and native Windows, macOS, and Linux behavior were considered.
- [ ] Public CLI/config/schema/plugin/output compatibility and migrations were considered.
- [ ] The open-core and dependency-license boundaries remain intact.

Explain any material impact or why it is absent:

## Decision records and release notes

- [ ] Required ADR/RFC, schema examples, migration/deprecation note, documentation, and changelog are included or not applicable.
- [ ] Every commit carries a DCO `Signed-off-by` line (`git commit --signoff`).
- [ ] The diff contains no secret, private source/history, personal data, accidental machine path, or unrelated generated file.

Related issue/ADR/RFC:
