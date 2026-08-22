# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a dynamic DNS updater for Oracle Cloud Infrastructure (OCI) DNS. It provides an HTTP API endpoint that receives client IP addresses and updates DNS A records in OCI DNS zones. The service is designed to run in Kubernetes (OKE) and be called via simple HTTP clients (e.g., `curl` from DHCP hooks or cron jobs).

## Architecture

The codebase follows a simple three-layer architecture:

1. **Entry Point** (`cmd/main.go`): Handles CLI flags, loads configuration, initializes logging, and starts the service
2. **HTTP Service** (`internal/service.go`): Implements the HTTP server with basic auth and request handling
3. **DNS Operations** (`internal/dns.go`): Handles OCI DNS client initialization and DNS record updates

**Key flow**: HTTP request → basic auth check → extract IP (from X-Forwarded-For or RemoteAddr) → read the current A records, skip the write if they already match → update DNS A records via OCI SDK → return a dyndns2 status line

**Configuration**: The app uses a JSON config file (default `config.json`) containing:
- DNS zone and host(s) to update
- HTTP basic auth credentials
- OCI credentials (tenancy, user, region, fingerprint, private key)

**Multi-host support**: The config supports either a single `host` field (legacy) or a `hosts` array for updating multiple DNS records with the same IP address (internal/config.go:44-53).

## Development Commands

### Build
```bash
go build -o oci-dyndns cmd/main.go
```

### Test
```bash
go test ./...
```

### Run locally
```bash
go run cmd/main.go -config config.json -listen :8080
```

### Build Docker image
```bash
docker build -t oci-dyndns .
```

### Run with Docker
```bash
docker run -v /path/to/config.json:/config.json -p 8080:8080 oci-dyndns
```

## Go Version

The toolchain is Go 1.26, set in `.tool-versions` and the `Dockerfile` — bump both together.

`go.mod` declares `go 1.25.0`. That is the *language* version and is deliberately lower than the toolchain;
raise it only to use newer language features, since it is the minimum for anyone building from source.

Go supports a release until two newer majors exist, so plan on a major bump roughly every six months. Check
the current support window at https://go.dev/doc/devel/release before picking a version.

## Container Image

`Dockerfile` builds a static binary on `golang:<version>-alpine` and ships it on a pinned `alpine:<version>`.

- **Name base images inline, never behind an `ARG`.** Dependabot cannot resolve a tag behind an `ARG`
  (dependabot-core#2057), so an `ARG`'d image is one nothing ever updates.
- **Keep the builder on the bare `-alpine` tag.** The suffixed `-alpine<version>` variants stop being rebuilt
  once the Go team's default Alpine moves on, which silently freezes the toolchain at an old patch release.
  Only the runtime base needs pinning.
- **Build locally when changing base images.** `package.yml` runs only on pushes to `main` and on tags, so a
  Dockerfile change is not validated before merge.

To check what a published image actually contains, extract the binary and inspect it directly — the toolchain
version is recorded in the binary, not in the image metadata:

```bash
go version -m ./oci-dyndns              # toolchain and module versions
govulncheck -mode binary ./oci-dyndns   # vulnerabilities reachable from this build
```

## Releases

Releases are git tags (`vX.Y.Z`); there is no version string in the source. Pushing a tag builds the image and
moves `:latest` onto it. Pushes to `main` publish `:main` only and leave `:latest` alone.

## Dependencies

Primary dependency: Oracle OCI Go SDK v65 (`github.com/oracle/oci-go-sdk/v65`)

## API

Single endpoint: `/nic/update` (`GET` or `POST`)
- Requires HTTP Basic Authentication
- Extracts caller's IP from the X-Forwarded-For header (first entry, if present) or RemoteAddr
- Reads the current A record(s) and updates them only if they differ from the caller's IP
- Speaks the dyndns2 protocol: a `text/plain` status line, `good <ip>` / `nochg <ip>` (both HTTP 200, since the
  distinction has no HTTP equivalent), `badauth` (401, with a `WWW-Authenticate` challenge) or `911` (500). The
  keyword is what dyndns2 clients parse; the HTTP status is kept honest for everything else
- The `hostname`/`myip` parameters sent by dyndns2 clients are ignored; hosts come from the config file
- With `Accept: application/json`, returns the legacy `{"message": ...}` body under the same HTTP status codes

Example usage:
```bash
curl 'https://username:password@my-domain/nic/update'
```

## Logging

Uses Go's `log/slog` package with JSON output to stdout at DEBUG level.