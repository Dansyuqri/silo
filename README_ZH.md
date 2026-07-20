<p align="center">
  <img src=".github/logo.svg" alt="Silo" width="260">
</p>

<h1 align="center">Silo</h1>

<p align="center">
  <strong>审慎维护的 MinIO 社区分支</strong><br>
  为现有部署提供安全维护、带版本的发行产物与持续运维支持。
</p>

<p align="center">
  <a href="README.md">English</a> ·
  <a href="https://silo.pigsty.cc">文档</a> ·
  <a href="https://github.com/pgsty/minio/releases">版本发布</a> ·
  <a href="https://hub.docker.com/r/pgsty/minio">容器镜像</a> ·
  <a href="SECURITY.md">安全策略</a>
</p>

<p align="center">
  <a href="https://github.com/pgsty/minio/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/pgsty/minio?include_prereleases&label=release&logo=github"></a>
  <a href="https://hub.docker.com/r/pgsty/minio"><img alt="Docker Pulls" src="https://img.shields.io/docker/pulls/pgsty/minio?logo=docker"></a>
  <a href="go.mod"><img alt="Go Version" src="https://img.shields.io/github/go-mod/go-version/pgsty/minio?logo=go"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPLv3-blue"></a>
</p>

> [!IMPORTANT]
> Silo 是由 [Pigsty](https://pigsty.cc) 独立维护、从 [`pgsty/minio`](https://github.com/pgsty/minio) 发布的开源 MinIO 社区分支。本项目与 MinIO, Inc. 不存在隶属、背书或赞助关系；文中使用 “MinIO” 仅用于说明上游项目及兼容谱系。

## 概述

Silo 维护一条基于 MinIO [`RELEASE.2025-12-03T12-00-00Z`](https://github.com/minio/minio/releases/tag/RELEASE.2025-12-03T12-00-00Z) 的下游版本线，为上游停止社区发行后仍在运行 MinIO 兼容部署的用户提供持续构建与发行产物。

Pigsty 使用本分支提供对象存储，包括 PostgreSQL 备份存储。

## 维护政策

活跃版本线的维护范围包括：

- 构建与依赖项维护；
- 适用的安全修复与公告；
- 针对可复现缺陷的范围明确的修复；
- 带版本的二进制、软件包、校验和与多架构镜像；
- Web Console、客户端、文档与 Pigsty 集成。

改动保持克制，并在可行时提供测试。所有维护均为尽力而为，不承诺固定的响应、修复或发布时间。

### 范围之外

- 独立产品路线图、新存储引擎或假设性的 S3 新特性；
- 大规模重写或显著扩大下游差异的改动；
- 历史版本或多条支持分支；
- 商业支持、SLA、7×24 服务或 SUBNET 服务；
- 部署设计、访问控制、监控、备份与恢复。

## 兼容策略

Silo 尽量保留：

- `minio` 可执行文件与 `github.com/minio/minio` module path；
- MinIO 兼容的 S3 API、配置、环境变量与命令行约定；
- `RELEASE.YYYY-MM-DDTHH-MM-SSZ` 标签、容器入口与常见部署方式。

兼容是目标，而非保证。安全修复可能改变行为。每个版本都应视为下游升级：锁定版本，阅读[版本说明](https://github.com/pgsty/minio/releases)与[安全公告](docs/security/advisories.md)，保留回滚路径，并在生产使用前完成测试。

## 发行产物

| 产物 | 位置 |
| :-- | :-- |
| 源码 | [`github.com/pgsty/minio`](https://github.com/pgsty/minio) |
| 容器镜像 | [`pgsty/minio`](https://hub.docker.com/r/pgsty/minio)，支持 `linux/amd64` 与 `linux/arm64` 多架构清单 |
| 服务端二进制 | [GitHub Releases](https://github.com/pgsty/minio/releases)，覆盖 Linux、macOS、Windows 的 `amd64` 与 `arm64` |
| Linux 软件包 | RPM、DEB、APK，并通过 [Pigsty 软件仓库](https://pigsty.cc/docs/repo/) 分发 |
| 客户端 | [`pgsty/mc`](https://github.com/pgsty/mc)，容器内以 `mcli` 提供，并保留 `mc` 兼容别名 |
| 管理控制台 | 社区维护的 [`georgmangold/console`](https://github.com/georgmangold/console)，嵌入服务端构建 |
| 文档 | [中文](https://silo.pigsty.cc)、[英文](https://silo.pigsty.io)与 [`pgsty/minio-docs`](https://github.com/pgsty/minio-docs) |
| 安全记录 | [`SECURITY.md`](SECURITY.md)、[`VULNERABILITY_REPORT.md`](VULNERABILITY_REPORT.md) 与[本分支安全公告](docs/security/advisories.md) |

## 快速开始

本地体验：

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

管理控制台位于 <http://localhost:9001>，S3 API 位于 <http://localhost:9000>。

镜像内置兼容客户端 `mcli`：

```bash
docker exec silo mcli alias set local http://127.0.0.1:9000 \
  "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
docker exec silo mcli mb local/demo
docker exec silo mcli ls local
```

> [!WARNING]
> 生产环境应锁定版本，使用独立凭据与 TLS，配置监控，保留独立备份，并验证恢复流程。

从源码构建服务端：

```bash
go build -o minio .
./minio --version
```

部署：[Silo 中文文档](https://silo.pigsty.cc) · [Pigsty MinIO 模块](https://pigsty.cc/docs/minio/)

## 安全

安全修复面向活跃的 `master` 分支，并记录在[安全公告](docs/security/advisories.md)中。请按照 [`SECURITY.md`](SECURITY.md) 与 [`VULNERABILITY_REPORT.md`](VULNERABILITY_REPORT.md) 私密报告漏洞；同时影响上游 MinIO 的问题也应向上游报告。

## 参与贡献

欢迎安全与依赖项更新、可复现缺陷修复、测试、发布自动化、打包与文档改进。

Issue 与 Pull Request 应说明受影响版本、复现步骤、影响、预期行为、测试与兼容性说明。大型改动请先提交 Issue 讨论。

## 背景

本项目源于上游社区发行与维护模式的变化。维护者对相关变化的分析、替代方案评估与早期维护记录见以下文章：

| 文章 | 主题 |
| :-- | :-- |
| [MinIO已死](https://vonng.com/db/minio-is-dead/) | 上游项目与发行模式的变化 |
| [MinIO已死，谁能接盘？](https://vonng.com/db/minio-alternative/) | 可选替代方案评估 |
| [MinIO 已死，MinIO 复生](https://vonng.com/db/minio-resurrect/) | 建立分支及其发行流水线 |
| [续命 MinIO：承诺兑现](https://vonng.com/db/minio-promise-kept/) | 初期安全与维护工作 |

## 许可证与商标

服务端继续采用 [GNU Affero General Public License v3.0](LICENSE) 发布。上游作者与署名信息见 [`CREDITS`](CREDITS)。

MinIO 是 MinIO, Inc. 的商标。Silo、Pigsty 与 `pgsty/minio` 均为独立社区项目，与 MinIO, Inc. 不存在隶属或背书关系。
