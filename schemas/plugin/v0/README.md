# Plugin schema placeholder v0

This Draft 2020-12 schema reserves the first manifest vocabulary without
enabling plugins. Phase 1 has no plugin loader, executable path, network
capability, or repository-controlled activation field. A checked-in manifest is
therefore inert.

The product's plugin phase must replace this `contract_status: placeholder`
shape with an explicitly versioned out-of-process protocol, digest pinning,
installation trust, resource limits, and capability enforcement. It must not
silently reinterpret this placeholder as authorization to execute code.

Examples under `examples/valid` must validate; examples under
`examples/invalid` must fail. The `.invalid` `$id` is resolved locally only.
