# Configuration schema v0

This directory publishes the experimental `0.1` configuration contract using
JSON Schema Draft 2020-12. `steward.yaml` is UTF-8 YAML, and JSON is accepted
because it is a YAML subset.

The runtime and schema agree on these rules:

- `version` is mandatory and must be the string `"0.1"`;
- objects are closed and unknown keys fail strict validation;
- `--allow-unknown` is an explicit inspection mode that reports ignored keys but
  never permits an unsupported version or invalid known value;
- one file is limited to 1 MiB and exactly one YAML document;
- structural input is limited to 64 collection/indentation levels and 65,536
  structural tokens before the YAML loader runs;
- JSON numbers with an integral value, including `1.0` and `1e3`, satisfy an
  integer field just as they do under JSON Schema; quoted numbers and fractional
  values remain invalid;
- no field represents a command, environment interpolation, network access, or a
  repository-selected plugin; and
- defaults are applied only to documented known fields.

The `$id` deliberately uses the reserved `.invalid` namespace until a durable
public schema host and migration policy are approved. Validators must use the
checked-in file and must not fetch its identifier over the network.

Examples under `examples/valid` must validate. Examples under
`examples/invalid` must fail.
