# Contributing

## Setup

```sh
go mod download
go test ./...
go vet ./...
```

## Common Tasks

```sh
make build
make test
make lint
make cover
make bench
```

Golden fixtures should be regenerated only with an explicit `UPDATE_GOLDEN=1` workflow when golden tests are expanded.

## Release

Releases are built by `.github/workflows/release.yml`, which creates platform archives, checksums, SBOM placeholders, and the Docker image.
