
<h1 align="center">Silo</h1>

<p align="center">
  <strong>A conservatively maintained MinIO fork</strong><br>
  Security maintenance, versioned release artifacts, and operational continuity for existing deployments.
</p>

<p align="center">
  <a href="README_ZH.md">中文</a> ·
  <a href="https://silo.pigsty.io">Documentation</a> ·
  <a href="https://github.com/pgsty/minio/releases">Releases</a> ·
  <a href="https://hub.docker.com/r/pgsty/minio">Container Images</a> ·
  <a href="SECURITY.md">Security</a>
</p>

<p align="center">
  <a href="https://github.com/pgsty/minio/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/pgsty/minio?include_prereleases&label=release&logo=github"></a>
  <a href="https://hub.docker.com/r/pgsty/minio"><img alt="Docker Pulls" src="https://img.shields.io/docker/pulls/pgsty/minio?logo=docker"></a>
  <a href="go.mod"><img alt="Go Version" src="https://img.shields.io/github/go-mod/go-version/pgsty/minio?logo=go"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPLv3-blue"></a>
</p>

> [!IMPORTANT]
> Silo is an independent, community-maintained fork of the open-source MinIO server, published by [Pigsty](https://pigsty.io) from [`pgsty/minio`](https://github.com/pgsty/minio). It is not affiliated with, endorsed by, or sponsored by MinIO, Inc. “MinIO” is used only to identify the upstream project and compatibility lineage.

## Overview

Silo maintains one downstream release line based on MinIO [`RELEASE.2025-12-03T12-00-00Z`](https://github.com/minio/minio/releases/tag/RELEASE.2025-12-03T12-00-00Z). It provides maintained builds and release artifacts for existing MinIO-compatible deployments after upstream community distribution ended.

Pigsty uses this fork for object storage, including PostgreSQL backups.

## Maintenance Policy

The active release line covers:

- build and dependency maintenance;
- applicable security fixes and advisories;
- focused fixes for reproducible defects;
- versioned binaries, packages, checksums, and multi-architecture images;
- the web console, client, documentation, and Pigsty integration.

Changes are kept narrow and tested where practical. Maintenance is best effort; no response, remediation, or release schedule is guaranteed.

### Out of scope

- a separate product roadmap, new storage engine, or speculative S3 features;
- broad rewrites or changes that materially expand the downstream delta;
- historical releases or multiple support branches;
- commercial support, SLAs, 24×7 coverage, or SUBNET access;
- deployment design, access control, monitoring, backup, or recovery.

## Compatibility

Silo aims to preserve:

- the `minio` executable and `github.com/minio/minio` module path;
- MinIO-compatible S3 APIs, configuration, environment variables, and CLI conventions;
- `RELEASE.YYYY-MM-DDTHH-MM-SSZ` tags, container entrypoints, and common deployment workflows.

Compatibility is a goal, not a guarantee. Security fixes may change behavior. Treat each release as a downstream upgrade: pin versions, review [release notes](https://github.com/pgsty/minio/releases) and [security advisories](docs/security/advisories.md), keep a rollback path, and test before production use.

## Release Artifacts

| Artifact | Location |
| :-- | :-- |
| Source | [`github.com/pgsty/minio`](https://github.com/pgsty/minio) |
| Container image | [`pgsty/minio`](https://hub.docker.com/r/pgsty/minio), multi-arch for `linux/amd64` and `linux/arm64` |
| Server binaries | [GitHub Releases](https://github.com/pgsty/minio/releases) for Linux, macOS, and Windows on `amd64` and `arm64` |
| Linux packages | RPM, DEB, and APK artifacts, also distributed through the [Pigsty repository](https://pigsty.io/docs/repo/) |
| Client | [`pgsty/mc`](https://github.com/pgsty/mc), bundled in the container as `mcli` with an `mc` compatibility alias |
| Console | Maintained [`georgmangold/console`](https://github.com/georgmangold/console) fork, embedded in the server build |
| Documentation | [English](https://silo.pigsty.io), [Chinese](https://silo.pigsty.cc), and [`pgsty/minio-docs`](https://github.com/pgsty/minio-docs) |
| Security record | [`SECURITY.md`](SECURITY.md), [`VULNERABILITY_REPORT.md`](VULNERABILITY_REPORT.md), and [fork advisories](docs/security/advisories.md) |

## Quick Start

For local evaluation:

```bash
mkdir -p data

export MINIO_ROOT_USER=minioadmin
export MINIO_ROOT_PASSWORD=change-me-long-password

docker run -d --name silo \
  -p 9000:9000 \
  -p 9001:9001 \
  -e MINIO_ROOT_USER \
  -e MINIO_ROOT_PASSWORD \
  -v "$PWD/data:/data" \
  pgsty/minio:latest server /data --console-address ":9001"
```

Open the console at <http://localhost:9001>; the S3 API listens on <http://localhost:9000>.

The image includes the compatible client as `mcli`:

```bash
docker exec silo mcli alias set local http://127.0.0.1:9000 \
  "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
docker exec silo mcli mb local/demo
docker exec silo mcli ls local
```

> [!WARNING]
> For production, pin a release, use unique credentials and TLS, monitor the service, keep independent backups, and test recovery.

Build the server from source:

```bash
go build -o minio .
./minio --version
```

Deployment: [Silo documentation](https://silo.pigsty.io) · [Pigsty MinIO module](https://pigsty.io/docs/minio/)

## Security

Security fixes target the active `master` branch and are recorded in the [advisory log](docs/security/advisories.md). Report vulnerabilities privately as described in [`SECURITY.md`](SECURITY.md) and [`VULNERABILITY_REPORT.md`](VULNERABILITY_REPORT.md). Report issues that also affect upstream MinIO there as well.

## Contributing

Useful contributions include security and dependency updates, reproducible bug fixes, tests, release automation, packaging, and documentation.

Issues and pull requests should include the affected version, reproduction steps, impact, expected behavior, tests, and compatibility notes. Discuss large changes in an issue first.

## Background

This project was created in response to changes in the upstream community distribution and maintenance model. The maintainer’s analysis, alternatives considered, and early maintenance record are documented below:

| Essay | Subject |
| :-- | :-- |
| [MinIO Is Dead](https://blog.vonng.com/en/db/minio-is-dead/) | Changes to the upstream project and distribution model |
| [MinIO Is Dead, Long Live MinIO](https://blog.vonng.com/en/db/minio-resurrect/) | Establishing the fork and its release pipeline |
| [Two months into maintaining a MinIO fork](https://blog.vonng.com/en/db/minio-promise-kept/) | Initial security and maintenance work |

## License and Trademark

The server remains licensed under the [GNU Affero General Public License v3.0](LICENSE). See [`CREDITS`](CREDITS) for upstream authorship and attribution. MinIO is a trademark of MinIO, Inc. Silo and `pgsty/minio` are independent community efforts and are not affiliated with or endorsed by MinIO, Inc.
