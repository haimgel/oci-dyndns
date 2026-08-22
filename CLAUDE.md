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

The project uses Go 1.25 (as specified in go.mod and .tool-versions).

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