# Support

Code Steward is pre-alpha open-source software. Community support is best effort; no production response or compatibility warranty is implied until a release and target are explicitly listed as supported.

## Community support covers

- installation and documented CLI behavior on published native targets;
- reproducible defects in the open scanner, built-in analyzers, schemas, outputs, local policy, import/export, deletion, and official open integrations;
- deterministic-result differences across Windows, macOS, and Linux targets;
- documentation and contribution questions;
- security fixes for the open scanner.

Use a GitHub issue for reproducible, non-sensitive bugs and scoped feature proposals. Search existing issues first. Include the Code Steward version/commit, operating-system target, sanitized command, expected/actual behavior, and the smallest synthetic reproduction. Remove source, repository URLs, identities, tokens, and absolute machine paths unless they are essential and safe to share.

## Use a private channel for

- suspected vulnerabilities or ways a crafted repository can cross a security boundary: follow [SECURITY.md](SECURITY.md);
- Code of Conduct reports with sensitive details: follow [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md#reporting).

Do not post secrets, private repositories, customer data, or embargoed vulnerabilities in public issues.

## Outside community support

The community project does not promise repository-specific architecture consulting, analyzer customization, organization identity mapping, hosted operations, SLA-backed incident response, legal/compliance certification, or support for unlisted operating-system targets and downstream forks. Maintainers may still help when capacity allows.

Commercial products may offer managed operation, enterprise integration, contractual assurance, and paid support. Payment does not unlock a more accurate deterministic analyzer, private-repository scanning, security fixes, evidence, local policy, export, or deletion.

## Version and platform scope

“Builds on my machine” is not the same as supported. The [release policy](docs/governance/release-and-deprecation.md) names supported release lines, while the supported-platform ADR names native target triples and tiers. Containers and WSL do not substitute for native Windows qualification.

Maintainers may close reports that cannot be reproduced, concern an unsupported snapshot/target, contain no actionable information, or belong upstream. A closure should explain the reason and, when known, point to the appropriate project or diagnostic step.
