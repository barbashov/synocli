# Repository Guidelines

## Project Structure & Module Organization
- `cmd/synocli`: Cobra CLI entrypoint and command handlers (`auth`, `ds`, formatting/human output).
- `internal/`: reusable packages by concern: `synology/` (API discovery/auth/Download Station), `httpclient/`, `config/`, `output/`, `redact/`, `apperr/`.
- `docs/architecture.md`: high-level design and extension pattern for new DSM modules.
- `bin/`: local build artifacts (for example `bin/synocli`).

Keep CLI wiring in `cmd/` and protocol/domain logic in `internal/`.

## Build, Test, and Development Commands
- `make build`: builds `./cmd/synocli` with `CGO_ENABLED=0` into `bin/synocli`.
- `make test`: runs all unit tests (`go test ./...`).
- `make lint`: runs `golangci-lint` in Docker with repo-mounted source.
- `docker build .`: validates container build used in CI.

Run `make test` and `make lint` before opening a PR.

## Coding Style & Naming Conventions
- Target Go `1.26` and keep code `gofmt`-clean.
- Use idiomatic Go names: exported `CamelCase`, unexported `lowerCamel`, package names lowercase.
- Test files use `*_test.go`; test funcs follow `TestXxx` and table tests with `t.Run(...)` where helpful.
- Prefer small, focused functions and typed request/response models in `internal/synology/...`.

## Testing Guidelines
- Primary framework is the standard `testing` package.
- Cover command behavior in `cmd/synocli/*_test.go` and transport/API behavior in `internal/.../*_test.go`.
- Add regression tests for bug fixes (status mapping, input validation, request encoding, auth edge cases).

## Commit & Pull Request Guidelines
- Follow existing commit style: short imperative subject lines (for example, `Fix DS add URL handling`, `Add CI pipeline`).
- PRs should include:
  - what changed and why,
  - test evidence (`make test`, `make lint`),
  - sample CLI output when UX/formatting changes.

## Security & Configuration Tips
- Do not commit credentials. Prefer `--password-stdin` or `--credentials-file` in local, ignored files.
- Use `--debug` carefully; sensitive fields are redacted, but review logs before sharing.
