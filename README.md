# synocli

`synocli` is a CLI for Synology DSM WebAPI with Download Station and File Station support.
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

Endpoint is resolved from:

1. `--endpoint`
2. `~/.synocli/config` (`endpoint=...`)

Credentials are resolved from either:

- `--credentials-file <path>` (exclusive mode), or
- `--user` + (`--password` or `--password-stdin`), with optional defaults from `~/.synocli/config`.

Use `cli-config` helpers:

```bash
synocli cli-config init --endpoint https://192.168.0.1:5001 --user admin --password secret --insecure-tls
synocli cli-config show
```

Config file permissions must be `0600`.

Credentials file format is ENV-style:

```env
user=admin
password=secret
```

## Global Flags

- `--endpoint string`
- `--config string` (default `~/.synocli/config`)
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

- `synocli auth ping`
- `synocli auth whoami`
- `synocli auth api-info [--prefix <api-prefix>]`

### ds

- `synocli ds list`
- `synocli ds get <task-id>`
- `synocli ds add <input> [--destination <folder>]`
- `synocli ds pause <task-id> [<task-id>...]`
- `synocli ds resume <task-id> [<task-id>...]`
- `synocli ds delete <task-id> [<task-id>...] [--with-data]`
- `synocli ds wait <task-id> [--interval <duration>] [--max-wait <duration>]`
- `synocli ds watch [--interval <duration>] [--id <task-id>]... [--status <normalized>]...`

### fs (alias: filestation)

Core:
- `synocli fs info`
- `synocli fs shares`
- `synocli fs list <folder-path> [...]` (alias: `fs ls`)
- `synocli fs get <path> [<path>...]`
- `synocli fs mkdir <parent-path> <name> [<name>...] [--parents]`
- `synocli fs rename <path> <new-name>`
- `synocli fs copy <path> [<path>...] --to <destination> [--overwrite|--skip-existing] [--async]` (alias: `fs cp`)
- `synocli fs move <path> [<path>...] --to <destination> [--overwrite|--skip-existing] [--async]` (alias: `fs mv`)
- `synocli fs delete <path> [<path>...] -r|--recursive [--async]`

Transfer:
- `synocli fs upload <local-path> <remote-path> [--parents] [--overwrite|--skip-existing]`
- `synocli fs download <remote-path> [<remote-path>...] --output <local-file> [--mode download|open]`

Search:
- `synocli fs search <folder-path> --pattern <pattern> [--recursive] [--async]`
- `synocli fs search-results <task-id>`
- `synocli fs search-stop <task-id>`
- `synocli fs search-clear`

Task APIs:
- `synocli fs dir-size <path> [<path>...] [--async]`
- `synocli fs dir-size-status <task-id>`
- `synocli fs dir-size-stop <task-id>`
- `synocli fs md5 <file-path> [--async]`
- `synocli fs md5-status <task-id>`
- `synocli fs md5-stop <task-id>`
- `synocli fs extract <archive-path> --to <dest-folder> [--async]`
- `synocli fs extract-status <task-id>`
- `synocli fs extract-stop <task-id>`
- `synocli fs compress <path> [<path>...] --to <dest-archive> [--async]`
- `synocli fs compress-status <task-id>`
- `synocli fs compress-stop <task-id>`

Background tasks and watch:
- `synocli fs tasks`
- `synocli fs tasks-clear [--task-id <id>]...`
- `synocli fs watch tasks [--interval <duration>]`
- `synocli fs watch folder <folder-path> [--interval <duration>] [--recursive]`

## Examples

```bash
# auth checks
synocli auth ping --endpoint https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls

# download station
synocli ds list --endpoint https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls

# file station core
synocli fs shares --endpoint https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls
synocli fs ls /volume1 --endpoint https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls
synocli fs cp /volume1/a.txt --to /volume1/archive --endpoint https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls
synocli fs mv /volume1/archive/a.txt --to /volume1/final --endpoint https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls
synocli fs delete /volume1/archive/old -r --endpoint https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls

# file station transfer
synocli fs upload ./build.tar.gz /volume1/uploads --endpoint https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls
synocli fs download /volume1/uploads/build.tar.gz --output ./build.tar.gz --endpoint https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls

# search and tasks
synocli fs search /volume1 --pattern report --endpoint https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls
synocli fs tasks --endpoint https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls

# watch
synocli fs watch tasks --endpoint https://192.168.0.1:5001 --interval 2s --credentials-file ./creds.env --insecure-tls
synocli fs watch folder /volume1 --endpoint https://192.168.0.1:5001 --interval 2s --recursive --credentials-file ./creds.env --insecure-tls
```

## Output

### Human output

- Styled tables and key-value blocks.
- TTY watch commands (`ds watch`, `fs watch *`) use in-place refresh.

### JSON output (`--json`)

- All non-watch commands return a single JSON envelope with `ok`, `command`, `data`, `error`, and `meta`.
- Watch commands return JSONL snapshots (`one envelope per tick`) for agent-friendly streaming consumption.

Example envelope:

```json
{
  "ok": true,
  "command": "fs list",
  "data": {},
  "meta": {
    "timestamp": "2026-01-01T00:00:00Z",
    "duration_ms": 12,
    "endpoint": "https://192.168.0.1:5001",
    "api_version": {
      "auth": 6,
      "fs_list": 2
    }
  }
}
```

## Notes

- `fs` and `ds` share the same auth/session orchestration in CLI runtime.
- `fs delete` requires `--recursive` (`-r`) for directory deletion.
- Recursive directory upload follows Linux `cp -r` style destination behavior.
- `fs list` human output shows `Name | Path | Size | MTime`; directories render `<DIR>` in the size column.

## Testing

```bash
make test
```

Manual File Station test plan:

- `testplans/filestation.sh`

## Architecture

See [docs/architecture.md](docs/architecture.md).
