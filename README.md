# synocli

`synocli` is a CLI for Synology DSM WebAPI.
It currently focuses on Download Station (`ds`) and File Station (`fs`) workflows.

## Quick Start

Build:

```bash
make build
./bin/synocli --help
```

Set reusable shell vars:

```bash
export ENDPOINT="https://192.168.0.1:5001"
export CREDS="./creds.env"
```

Create credentials file (do not commit it):

```env
user=admin
password=secret
```

First connectivity check:

```bash
./bin/synocli auth ping --endpoint "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls
```

## Authentication

Endpoint resolution order:

1. `--endpoint`
2. `~/.synocli/config` (`endpoint=...`)

Credentials:

- `--credentials-file <path>` (exclusive mode), or
- `--user` + (`--password` or `--password-stdin`)

Initialize local config quickly:

```bash
./bin/synocli cli-config init --endpoint "$ENDPOINT" --user admin --password secret --insecure-tls
./bin/synocli cli-config show
```

Config file permissions should be `0600`.

## Common Workflows

Download Station:

```bash
./bin/synocli ds list --endpoint "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls
./bin/synocli ds add "https://example.com/file.iso" --endpoint "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls
./bin/synocli ds delete dbid_1 --endpoint "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls
```

File Station:

```bash
./bin/synocli fs shares --endpoint "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls
./bin/synocli fs ls /volume1 --endpoint "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls
./bin/synocli fs cp /volume1/a.txt --to /volume1/archive --endpoint "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls
./bin/synocli fs delete /volume1/archive/old -r --endpoint "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls
```

## Command Discovery

Use CLI help for full command reference:

```bash
./bin/synocli --help
./bin/synocli ds --help
./bin/synocli fs --help
./bin/synocli fs watch --help
```

## Output Modes

Human mode (default):

- Tables and key-value summaries.
- TTY watch commands (`ds watch`, `fs watch ...`) refresh in place.

JSON mode (`--json`):

- Non-watch commands return one envelope with `ok`, `command`, `data`, `error`, `meta`.
- Watch commands stream JSONL snapshots (one envelope per tick).

## Testing

Unit tests:

```bash
make test
```

Manual E2E scripts:

- `tests_e2e/filestation.sh`
- `tests_e2e/downloadstation.sh`

> Caution
> E2E scripts operate on a real DSM target. They can create/delete Download Station tasks,
> create/delete real files/folders, and download external fixtures.
> Run them only against disposable test data or a dedicated test NAS.

## Notes

- `fs` and `ds` share the same auth/session orchestration.
- `fs delete` requires `--recursive` (`-r`) for directory deletion.
- Recursive upload follows Linux `cp -r` destination behavior.

## Architecture

See [docs/architecture.md](docs/architecture.md).
