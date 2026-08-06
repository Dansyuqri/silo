# Contributing to Silo

Silo welcomes focused contributions that improve security, reliability,
compatibility, packaging, tests, or maintainability. This repository preserves
MinIO-compatible interfaces and storage formats, so changes must identify and
test any compatibility impact.

## Development Workflow

Fork the current Silo source repository, create a topic branch, and submit a
pull request. Discuss broad or compatibility-sensitive changes in an issue
before implementation.

### Set up a checkout

```sh
git clone https://github.com/pgsty/silo
cd silo
go build -o silo .
./silo --version
```

### Keep the lineage remote separate

```sh
git remote add lineage https://github.com/minio/minio
git fetch lineage
```

Do not merge an upstream branch into a pull request unless the maintainers have
agreed on the scope. Silo intentionally carries a small downstream delta.

### Create your feature branch

Create a separate branch before making code changes:

```
git checkout -b my-new-feature
```

### Test Silo server changes

Before opening a pull request:

- Add or update tests for changed behavior.
- Run `make verifiers`.
- Run the smallest relevant package tests, then `make test` when practical.
- Run `make build` and confirm the generated executable is `silo`.
- Explain any preserved `MINIO_*`, `minio_*`, `x-minio-*`, `/minio/*`,
  `.minio.sys`, ARN, module/import-path, or serialized compatibility name.

### Commit changes

After verification, commit your changes with a concise message:

```
git commit -am 'Fix object replication retry handling'
```

### Push to the branch

Push your locally committed changes to the remote origin (your fork)

```
git push origin my-new-feature
```

### Create a Pull Request

Pull requests should include motivation, reproduction steps where applicable,
test evidence, compatibility notes, and documentation impact. Public product
documentation is owned by the separate
[`pgsty/silo.pgsty.com`](https://github.com/pgsty/silo.pgsty.com) repository.

## FAQs

### How does Silo manage dependencies?

Silo uses Go modules. Preserve the compatibility module and import paths in
`go.mod`; downstream forks are selected with explicit `replace` directives.

- Run `go get foo/bar` in the source folder to add the dependency to `go.mod` file.

To remove a dependency

- Edit your code and remove the import reference.
- Run `go mod tidy` in the source folder to remove dependency from `go.mod` file.

### What are the coding guidelines?

Follow the existing Go style, run `gofmt` on changed Go files, and keep changes
compact. See the Go project's [code review comments](https://go.dev/wiki/CodeReviewComments).
