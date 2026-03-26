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
- must not be combined with `--user`, `--password`, or `--password-stdin`

Credentials file format is ENV-style:

```env
user=admin
password=secret
```

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

### fs (alias: filestation)

Core:
- `synocli fs info <endpoint>`
- `synocli fs shares <endpoint>`
- `synocli fs list <endpoint> <folder-path> [...]` (alias: `fs ls`)
- `synocli fs get <endpoint> <path> [<path>...]`
- `synocli fs mkdir <endpoint> <parent-path> <name> [<name>...] [--parents]`
- `synocli fs rename <endpoint> <path> <new-name>`
- `synocli fs copy <endpoint> <path> [<path>...] --to <destination> [--overwrite|--skip-existing] [--async]` (alias: `fs cp`)
- `synocli fs move <endpoint> <path> [<path>...] --to <destination> [--overwrite|--skip-existing] [--async]` (alias: `fs mv`)
- `synocli fs delete <endpoint> <path> [<path>...] -r|--recursive [--async]`

Transfer:
- `synocli fs upload <endpoint> <local-path> <remote-path> [--parents] [--overwrite|--skip-existing]`
- `synocli fs download <endpoint> <remote-path> [<remote-path>...] --output <local-file> [--mode download|open]`

Search:
- `synocli fs search <endpoint> <folder-path> --pattern <pattern> [--recursive] [--async]`
- `synocli fs search-results <endpoint> <task-id>`
- `synocli fs search-stop <endpoint> <task-id>`
- `synocli fs search-clear <endpoint>`

Task APIs:
- `synocli fs dir-size <endpoint> <path> [<path>...] [--async]`
- `synocli fs dir-size-status <endpoint> <task-id>`
- `synocli fs dir-size-stop <endpoint> <task-id>`
- `synocli fs md5 <endpoint> <file-path> [--async]`
- `synocli fs md5-status <endpoint> <task-id>`
- `synocli fs md5-stop <endpoint> <task-id>`
- `synocli fs extract <endpoint> <archive-path> --to <dest-folder> [--async]`
- `synocli fs extract-status <endpoint> <task-id>`
- `synocli fs extract-stop <endpoint> <task-id>`
- `synocli fs compress <endpoint> <path> [<path>...] --to <dest-archive> [--async]`
- `synocli fs compress-status <endpoint> <task-id>`
- `synocli fs compress-stop <endpoint> <task-id>`

Background tasks and watch:
- `synocli fs tasks <endpoint>`
- `synocli fs tasks-clear <endpoint> [--task-id <id>]...`
- `synocli fs watch tasks <endpoint> [--interval <duration>]`
- `synocli fs watch folder <endpoint> <folder-path> [--interval <duration>] [--recursive]`

## Examples

```bash
# auth checks
synocli auth ping https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls

# download station
synocli ds list https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls

# file station core
synocli fs shares https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls
synocli fs ls https://192.168.0.1:5001 /volume1 --credentials-file ./creds.env --insecure-tls
synocli fs cp https://192.168.0.1:5001 /volume1/a.txt --to /volume1/archive --credentials-file ./creds.env --insecure-tls
synocli fs mv https://192.168.0.1:5001 /volume1/archive/a.txt --to /volume1/final --credentials-file ./creds.env --insecure-tls
synocli fs delete https://192.168.0.1:5001 /volume1/archive/old -r --credentials-file ./creds.env --insecure-tls

# file station transfer
synocli fs upload https://192.168.0.1:5001 ./build.tar.gz /volume1/uploads --credentials-file ./creds.env --insecure-tls
synocli fs download https://192.168.0.1:5001 /volume1/uploads/build.tar.gz --output ./build.tar.gz --credentials-file ./creds.env --insecure-tls

# search and tasks
synocli fs search https://192.168.0.1:5001 /volume1 --pattern report --credentials-file ./creds.env --insecure-tls
synocli fs tasks https://192.168.0.1:5001 --credentials-file ./creds.env --insecure-tls

# watch
synocli fs watch tasks https://192.168.0.1:5001 --interval 2s --credentials-file ./creds.env --insecure-tls
synocli fs watch folder https://192.168.0.1:5001 /volume1 --interval 2s --recursive --credentials-file ./creds.env --insecure-tls
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

- `testplans/filestation.md`
- `testplans/filestation.sh` (runnable script derived from the plan)

## Architecture

See [docs/architecture.md](docs/architecture.md).
