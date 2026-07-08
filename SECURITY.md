# Security Policy

We take the security of CodeSteward seriously. Thank you for helping keep the
project and its users safe.

## Supported versions

Security fixes are provided for the versions below. CodeSteward is pre-1.0, so
only the latest v0.1.x line receives fixes.

| Version | Supported |
|---|---|
| 0.1.x | :white_check_mark: |
| < 0.1 | :x: |

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues,
pull requests, or discussions.**

Instead, report privately using one of these channels:

- **GitHub private vulnerability reporting** — open the repository's
  **Security** tab and choose **Report a vulnerability** (Privately report a
  vulnerability). This keeps the report confidential until a fix is ready.
- **Email** — send details to **[INSERT SECURITY CONTACT EMAIL]**.

Please include as much of the following as you can:

- A description of the vulnerability and its impact.
- Steps to reproduce, or a proof-of-concept.
- Affected version(s) and platform.
- Any suggested remediation.

## What to expect

- **Acknowledgement** within 3 business days.
- An initial **assessment** and severity triage shortly after.
- Coordinated disclosure: we will work with you on a fix and a disclosure
  timeline, and credit you in the release notes unless you prefer to remain
  anonymous.

Please give us a reasonable opportunity to remediate before any public
disclosure.

## Scope

CodeSteward is a deterministic CLI that reads your git history, CODEOWNERS, and
configuration, and posts a comment via the GitHub or GitLab API. Relevant
security concerns include, for example:

- Handling of tokens and credentials (`GITHUB_TOKEN`,
  `CODESTEWARD_GITLAB_TOKEN`, `CI_JOB_TOKEN`).
- Parsing of untrusted repository content (config, CODEOWNERS, diffs, PR/MR
  descriptions).
- Any behavior that could exfiltrate data or execute unintended commands.

CodeSteward uses no AI/LLM services and only the Go standard library plus
`gopkg.in/yaml.v3`, which keeps its dependency and network surface small.

Thank you for helping keep CodeSteward and its users safe.
