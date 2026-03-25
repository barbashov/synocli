# Architecture

`synocli` is organized as a reusable DSM CLI foundation.

## Layers

- `cmd/synocli`: Cobra command tree and UX.
- `internal/httpclient`: shared HTTP client, TLS setup, debug transport.
- `internal/synology/apiinfo`: endpoint and version discovery (`SYNO.API.Info`).
- `internal/synology/auth`: session login/logout (`SYNO.API.Auth`).
- `internal/synology/downloadstation`: typed Download Station task operations.
- `internal/output`: stable JSON envelope and JSONL helpers.

## How to add a new DSM module (example: File Station)

1. Add `internal/synology/filestation` with typed request/response models.
2. Reuse `apiinfo.Select` for API path/version discovery.
3. Add `cmd/synocli/fs.go` and wire into root command.
4. Reuse `withSession` in command handlers for auth/session lifecycle.
5. Return results through `internal/output.Envelope` for schema consistency.
