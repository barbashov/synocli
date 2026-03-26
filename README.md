# synocli

`synocli` is a CLI for Synology DSM WebAPI with Download Station support.
It is designed for DSM 6.2+ compatibility and uses API discovery (`SYNO.API.Info`) to select the best available endpoints.

## Build

```bash
make build
```

Binary path:

```bash
./bin/synocli
```

## Authentication Modes

Every command requires an endpoint positional argument:

```bash
synocli <group> <command> <endpoint> [args]
```

Example endpoint: `https://192.168.0.1:5001`

### Mode A: Explicit flags

- `--user <username>`
- one of:
  - `--password <password>`
  - `--password-stdin`

### Mode B: Credentials file

- `--credentials-file <path>`
- **must not** be combined with `--user`, `--password`, or `--password-stdin`

Credentials file format is ENV-style:

```env
# comments and unknown keys are ignored
user=admin
password=secret
```

Keys are case-insensitive.

## Global Flags

- `--credentials-file string`
- `--user string`
- `--password string`
- `--password-stdin`
- `--insecure-tls`
- `--timeout duration` (default `30s`)
- `--json`
- `--debug`

## Commands

### auth

- `synocli auth ping <endpoint>`
- `synocli auth whoami <endpoint>`
- `synocli auth api-info <endpoint> [--prefix <api-prefix>]`

### ds

- `synocli ds list <endpoint>`
- `synocli ds get <endpoint> <task-id>`
- `synocli ds add <endpoint> <input> [--destination <folder>]`
- `synocli ds pause <endpoint> <task-id> [<task-id>...]`
- `synocli ds resume <endpoint> <task-id> [<task-id>...]`
- `synocli ds delete <endpoint> <task-id> [<task-id>...] [--with-data]`
- `synocli ds wait <endpoint> <task-id> [--interval <duration>] [--max-wait <duration>]`
- `synocli ds watch <endpoint> [--interval <duration>] [--id <task-id>]... [--status <normalized>]...`

## Examples

```bash
# auth checks
synocli auth ping https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls
synocli auth whoami https://192.168.0.1:5001 --user admin --password-stdin
synocli auth api-info https://192.168.0.1:5001 --credentials-file ./creds.env --prefix SYNO.DownloadStation

# add/list/get
synocli ds add https://192.168.0.1:5001 ./ubuntu.torrent --credentials-file ./creds.env --insecure-tls
synocli ds add https://192.168.0.1:5001 magnet:?xt=urn:btih:... --credentials-file ./creds.env --insecure-tls
synocli ds add https://192.168.0.1:5001 https://example.com/file.iso --credentials-file ./creds.env --insecure-tls
synocli ds list https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls
synocli ds get https://192.168.0.1:5001 dbid_123 --credentials-file ./creds.env --insecure-tls

# control
synocli ds pause https://192.168.0.1:5001 dbid_1 dbid_2 --credentials-file ./creds.env --insecure-tls
synocli ds resume https://192.168.0.1:5001 dbid_1 --credentials-file ./creds.env --insecure-tls
synocli ds delete https://192.168.0.1:5001 dbid_1 --with-data --credentials-file ./creds.env --insecure-tls

# watch/wait
synocli ds wait https://192.168.0.1:5001 dbid_1 --max-wait 10m --credentials-file ./creds.env --insecure-tls
synocli ds watch https://192.168.0.1:5001 --interval 2s --status downloading --credentials-file ./creds.env --insecure-tls
```

## Output

### Human output

- Human output uses styled tables and compact key-value blocks.
- Styling is enabled on TTY and disabled automatically for redirected/piped output and `NO_COLOR`.
- `auth api-info` is rendered as a sorted table with match summary.
- `ds watch` uses in-place refresh on TTY; JSON mode remains append-only snapshots.
- `ds add` auto-detects input type in this order: magnet URI, existing local file (torrent), URL with scheme.
- For DS2 numeric statuses, status is rendered with enum and code, e.g. `paused (3)`.

### JSON envelope (`--json`)

All commands return an envelope:

```json
{
  "ok": true,
  "command": "ds list",
  "data": {},
  "error": null,
  "meta": {
    "timestamp": "2026-01-01T00:00:00Z",
    "duration_ms": 12,
    "endpoint": "https://192.168.0.1:5001",
    "api_version": {
      "auth": 6,
      "task": 2
    }
  }
}
```

Task objects include status-related fields:

- `normalized_status` (`waiting|downloading|paused|finishing|finished|seeding|error|unknown`)
- `raw_status` (raw API value as received)
- `status_enum` (enum name when mapped)
- `status_display` (human display form, e.g. `paused (3)`)
- `status_code` (present for numeric statuses)

## DSM / Download Station Notes

- API discovery is used on each invocation.
- DS2 task APIs are used when available; DSM6-compatible behavior is preserved where needed.
- Session is login/logout per invocation (no session persistence).

## Debugging

Use `--debug` to print request/response flow with redaction.

## Testing

```bash
make test
```

## Architecture

See [docs/architecture.md](docs/architecture.md).
