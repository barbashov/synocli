# Repository Guidelines

## Project Overview
CLI client for Synology DSM APIs (Download Station, File Station). Built with Go and Cobra.

## Project Structure
- `cmd/synocli/` — CLI entrypoint (`main.go` only).
- `internal/cli/` — Cobra commands (`auth`, `ds`, `fs`, `cli-config`, `cli-update`, `version`), session management, human-readable output formatting.
- `internal/synology/` — DSM API clients by module: `apiinfo` (discovery), `auth` (session login/logout), `downloadstation` (task CRUD, torrent handling), `filestation` (files, search, archive, background tasks).
- `internal/apperr/` — structured errors with exit codes.
- `internal/cmdutil/` — human output helpers (tables, key-value blocks, TTY detection, formatting).
- `internal/config/` — endpoint validation, config file parsing, credential resolution.
- `internal/httpclient/` — shared HTTP client with TLS, cookie jar, debug transport.
- `internal/output/` — JSON envelope for machine-readable output.
- `internal/redact/` — sensitive field redaction for debug logs.
- `internal/update/` — self-update from GitHub releases (version check, download, checksum verification).
- `tests_e2e/` — shell-based e2e tests requiring a real Synology NAS.

Keep CLI wiring in `cmd/` and protocol/domain logic in `internal/`.

## Build, Test, and Lint
```
make build     # CGO_ENABLED=0, outputs bin/synocli
make test      # go test ./...
make lint      # golangci-lint v2.11 via Docker
docker build . # multi-stage alpine image
```

Run `make test` and `make lint` before committing.

## E2E Tests
Require a live Synology NAS. Not part of CI. Always ask the user for endpoint, credentials file, and options before running.
```
tests_e2e/filestation.sh --endpoint <url> --credentials-file <path> [--base <path>] [--insecure-tls]
tests_e2e/downloadstation.sh --endpoint <url> --credentials-file <path> [--insecure-tls]
```

## Coding Conventions
- Go 1.26, `gofmt`-clean.
- Idiomatic names: exported `CamelCase`, unexported `lowerCamel`, lowercase package names.
- Small focused functions. Typed request/response models in `internal/synology/`.
- Tests: standard `testing` package, `TestXxx`, table-driven with `t.Run(...)`.
- Regression tests for bug fixes (status mapping, validation, encoding, auth).

## Commit Style
Short imperative subject lines: `Fix DS add URL handling`, `Add CI pipeline`, `Remove dead code`.

## Versioning and Releases
- SemVer is mandatory. Release tags must be annotated and formatted as `vX.Y.Z`.
- Current baseline release: `v0.1.0` (dated 2026-03-27).
- Every release must have a matching `CHANGELOG.md` section: `## [X.Y.Z] - YYYY-MM-DD`.
- Every release section must include `### Agent Notes` with YAML keys:
  - `breaking_changes`
  - `commands_added`
  - `commands_changed`
  - `flags_added`
  - `flags_changed`
  - `behavior_changes`
  - `skill_update_action`
- Keep `## [Unreleased]` updated whenever command behavior/flags/output changes, so AI agents can refresh CLI skills before release.
- Release flow:
  1. Update `CHANGELOG.md` (`Unreleased` -> new version section) and commit.
  2. Run `make release-check VERSION=vX.Y.Z`.
  3. Run `make release VERSION=vX.Y.Z` (requires clean working tree).
  4. Push: `git push origin main vX.Y.Z`.

## Security
- Never commit credentials. Use `--password-stdin` or `--credentials-file` (gitignored).
- `--debug` redacts sensitive fields via `internal/redact/`, but review logs before sharing.
