# Code Steward

Code Steward is a deterministic maintenance-intelligence system for software teams. It turns repository structure, change history, and declared policy into evidence-backed findings about maintenance risk. It does not rank developers, infer whether code was AI-generated, or execute the repository it examines.

The project is in pre-alpha development. Interfaces may change, and no released platform is yet designated as production-supported. See [the roadmap](ROADMAP.md) for the maturity plan.

## Trust contract

- Deterministic evidence, not model output, is the source of truth for findings and policy decisions.
- A normal scan is local-first, works without a service account, and makes no network request.
- Repositories are untrusted input. Code Steward does not run builds, hooks, filters, package managers, or project code while scanning.
- Windows, macOS, and Linux are first-class native targets. A target becomes supported only after its published native verification gates pass.
- Reports and public schemas are portable. Users can export and delete derived local data.

The complete deterministic scanner is Apache-2.0 software. Paid products may add managed operation, organization history, collaboration, identity mapping, centrally managed policy, enterprise integrations, access controls, support, and contractual assurance. They may not withhold a more accurate analyzer, private-repository scanning, security fixes, evidence, local policy, export, or deletion. The detailed boundary is in the [product contract](docs/product-contract.md).

## Prerequisites

Install the following natively on Windows, macOS, or Linux:

- [Git](https://git-scm.com/downloads).
- [Rust through `rustup`](https://rustup.rs/). The workspace's toolchain file is authoritative when present; `rustup` installs it on demand.
- A platform linker supported by Rust. On Windows, install Visual Studio Build Tools with **Desktop development with C++** when using the MSVC toolchain. On macOS, install Xcode Command Line Tools. Linux package names vary by distribution; a C toolchain and linker are normally required.

The first Cargo build may download Rust crates. Project commands must not execute the checked-out repository's hooks, build scripts, or other code beyond Code Steward's own trusted build dependencies.

## Clean-clone build

Replace `<repository-url>` with `https://github.com/patbrac/CodeSteward.git` or a reviewed fork URL.

POSIX shell (Linux or macOS):

```sh
git clone <repository-url> code-steward
cd code-steward
rustup show active-toolchain
cargo build --workspace --locked
cargo test --workspace --locked
cargo run --locked -p steward-cli -- version
```

PowerShell (native Windows):

```powershell
git clone <repository-url> code-steward
Set-Location code-steward
rustup show active-toolchain
cargo build --workspace --locked
cargo test --workspace --locked
cargo run --locked -p steward-cli -- version
```

These are bounded development commands: they build the finite workspace and run its checked-in test suites; they do not scan an arbitrary directory. `--locked` prevents an implicit lockfile update. If a dependency fetch is required, use Cargo's normal network access once, then repeat with `CARGO_NET_OFFLINE=true` (POSIX) or `$env:CARGO_NET_OFFLINE = "true"` (PowerShell) to check offline operation.

For the complete format, lint, test, and documentation checks, see [CONTRIBUTING.md](CONTRIBUTING.md). The current Phase 1 commands—`version`, `doctor`, and `config validate`—and their output and exit-code contracts are documented in the [CLI reference](docs/cli.md).

## Security and privacy

Do not open a public issue for a suspected vulnerability. Use [GitHub private vulnerability reporting](SECURITY.md#report-a-vulnerability). The [threat model](docs/security/threat-model.md) describes the non-execution boundary, path containment, resource controls, terminal safety, plugin isolation, source handling, and supply-chain assumptions.

By default, Code Steward keeps source and derived state on the user's machine. A future operation that transmits source or executes a build must be separately named, explicitly enabled, and outside the default scan boundary.

## Contributing and support

Contributions are accepted under the [Developer Certificate of Origin](CONTRIBUTING.md#developer-certificate-of-origin), without a broad CLA. Start with [the contribution guide](CONTRIBUTING.md), review [governance](GOVERNANCE.md), and follow the [Code of Conduct](CODE_OF_CONDUCT.md).

Use GitHub issues for reproducible bugs and scoped feature proposals. See [SUPPORT.md](SUPPORT.md) for what the community project supports and what belongs in a private security report.

## License and marks

Code is licensed under the [Apache License 2.0](LICENSE). The license does not grant rights to project names or logos; see [TRADEMARKS.md](TRADEMARKS.md) for permitted nominative and compatibility use.
