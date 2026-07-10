# Phase 1 CLI contract

`steward` is the native, offline-first command-line adapter over the reusable
Rust engine. The Phase 1 commands read only their explicit configuration input.
They do not initialize a network client, invoke Git, spawn a child process, load
a plugin, interpolate environment variables, or execute repository code.

## Commands

```text
steward version
steward doctor [--format human|json] [--config PATH]
steward config validate [PATH] [--allow-unknown]
```

`version` writes exactly `steward <semantic-version>` plus a newline to stdout.

`doctor` writes installation diagnostics to stdout. Human output has stable check
IDs and no terminal control sequences. JSON output follows the experimental
`0.1.0` doctor-report shape. Without `--config`, doctor validates
`./steward.yaml` when present and otherwise reports that built-in defaults are
available. An explicit missing or unreadable `--config` is an error.

`config validate` reads `./steward.yaml` by default. Paths are passed through the
native operating-system path API, so spaces and Unicode do not require a
project-specific encoding convention. File contents must be valid UTF-8, may use
LF or CRLF, must contain exactly one YAML document, and may not exceed 1 MiB.
JSON configuration is accepted as a YAML subset. Before YAML deserialization,
the validator enforces a maximum structural nesting depth of 64 and a maximum
of 65,536 structural tokens so a byte-bounded document cannot amplify parser
work without bound.

The mandatory version is the quoted string `"0.1"`. Strict validation rejects
unknown keys recursively. `--allow-unknown` is an explicit forward-compatible
inspection mode: ignored paths are printed to stdout, while known values and the
configuration version remain strict. It does not enable a feature represented by
an ignored key. JSON numbers with a mathematically integral value, such as `1.0`
or `1e3`, are accepted for integer fields; fractional and quoted numbers are not.

The checked-in Draft 2020-12 contract is
`schemas/config/v0/steward-config.schema.json`.

## Stream and exit-code contract

Successful command results go to stdout. Configuration and I/O diagnostics go
to stderr, except `doctor`, whose requested report stays on stdout even when a
check is unhealthy. User-controlled control characters are escaped before text
is written to a terminal; JSON uses JSON string escaping.
Diagnostics do not echo invalid known scalar values, and each sanitized
terminal-controlled value is bounded to 8 KiB.

| Code | Meaning |
|---:|---|
| `0` | Command completed successfully. Warnings alone do not fail. |
| `2` | Invalid CLI syntax or usage. |
| `3` | Configuration syntax, shape, version, encoding, size, or known value is invalid. |
| `4` | A requested file or output stream could not be opened, read, or written. |
| `70` | An internal invariant or serialization operation failed unexpectedly. |

These codes are platform-independent. Adding a new failure category or changing
an existing meaning requires a documented compatibility decision and integration
tests on native Windows, macOS, and Linux.

## Minimal configuration

```yaml
version: "0.1"
```

Configuration is declarative. The v0 schema intentionally has no command,
environment interpolation, network, source-upload, or plugin-activation field.
