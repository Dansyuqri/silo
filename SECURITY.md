# Security Policy

Silo is an independent, community-maintained object-storage server derived from
the open-source MinIO server. The source repository retains the transitional
name `pgsty/minio` until the coordinated repository cutover. Upstream MinIO
security contacts do not handle Silo-specific fixes or release notes.

## Supported Versions

Security fixes are tracked on the active development branch and summarized in
[docs/security/advisories.md](docs/security/advisories.md). Only the current
Silo release line is supported unless an advisory says otherwise.

## Reporting a Vulnerability

For vulnerabilities in this fork:

1. Follow the fork-specific expectations in [VULNERABILITY_REPORT.md](VULNERABILITY_REPORT.md).
2. Prefer this repository's [private GitHub security advisory](https://github.com/pgsty/minio/security/advisories/new) workflow.
3. If private reporting is unavailable, contact the maintainers through the
   repository without publishing exploit details until a private channel is
   established.
4. If you confirm the issue also affects upstream `minio/minio`, report it upstream separately.

## Disclosure Process

Fork-specific fixes and user-visible upgrade notes are published in [docs/security/advisories.md](docs/security/advisories.md). The fork-specific triage and remediation process is described in [VULNERABILITY_REPORT.md](VULNERABILITY_REPORT.md).
