# Security policy

Code Steward analyzes hostile repositories and security-sensitive development history. Please disclose vulnerabilities privately so users can be protected before technical details are public.

## Supported versions

There is not yet a stable release. During pre-alpha, security fixes are made on the active `main` development line; old commits and unpublished development artifacts are not supported. Each public release will list its support status. The release policy defines the bounded 0.x support window and the 1.0 commitment.

| Version | Security fixes |
|---|---|
| `main` pre-alpha development | Best effort; not a production-support claim |
| Unreleased snapshots and older commits | No |

## Report a vulnerability

The intended route is [GitHub private vulnerability reporting](https://github.com/patbrac/CodeSteward/security/advisories/new). Use it only when the page visibly offers **Report a vulnerability**, then submit the private advisory form. Repository administrators enable the feature and complete a synthetic end-to-end test after public publication; this policy does not claim that a form is operational merely because the link exists.

If the form is unavailable, do not open a public issue or send vulnerability details through a conduct report. Use [GitHub Support](https://support.github.com/contact) to request a private route to the `patbrac/CodeSteward` repository owner without including exploit details in the initial request.

Include, when safe:

- affected version, commit, platform, and installation method;
- impact and the security boundary that failed;
- minimal reproduction using synthetic data;
- whether exploitation requires a crafted repository, path, Git setting, terminal, plugin, artifact, or network interaction;
- logs with tokens, usernames, machine paths, private URLs, and source removed;
- your disclosure timeline and whether a CVE has been requested.

Do not upload private source, repository history, credentials, access tokens, personal data, or an active exploit against another party. We will arrange a narrower exchange if more information is necessary.

## Response targets

Response times start when the private report is visible to a security responder. Targets are:

| Stage | Target |
|---|---|
| Human acknowledgement | Within 2 business days |
| Initial severity and scope assessment | Within 5 business days |
| Progress update while unresolved | At least every 7 calendar days |
| Critical issue mitigation or actionable workaround | Target 7 calendar days |
| High-severity issue mitigation or actionable workaround | Target 14 calendar days |
| Coordinated disclosure | Normally within 90 days, adjusted for user safety and release availability |

These are public response targets, not a paid support warranty. If a target cannot be met, the responder will explain the constraint and provide a revised date through the private advisory.

## Handling and disclosure

Only designated security responders receive private reports. They reproduce with synthetic data when possible, minimize retained material, record access and decisions in the private advisory, and recuse for conflicts of interest. Reporter identity and exploit details remain private until coordinated disclosure unless law or immediate user safety requires otherwise.

The project will assign severity, identify affected versions, prepare tests and fixes on a restricted branch, audit adjacent code, and plan release/notification. Credit is offered unless the reporter declines or attribution would create risk. The project will not request silence beyond the coordinated period or condition a fix on a waiver of rights.

Public advisories describe impact, affected/fixed versions, mitigations, credit, and enough evidence to verify the boundary without exposing private repository content.

## Security boundaries

The authoritative security design is the [threat model](docs/security/threat-model.md). Its key rule is that a default scan reads repository data but does not execute repository-controlled code. Network access, source transmission, build execution, and plugins are distinct capabilities that require explicit user action and separate controls.

Dependency vulnerability, update, license, lockfile, and exception handling are defined in the [dependency policy](docs/security/dependency-policy.md). Security fixes for the open scanner are never withheld for a commercial tier.

## Out of scope

Without prior authorization, do not test infrastructure you do not own, perform denial of service, access another user's data, use social engineering, or publish an unpatched exploit. Purely theoretical reports without a violated security boundary may be handled as normal hardening proposals, but we still prefer that uncertain cases begin privately.
