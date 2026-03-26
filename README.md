# synocli

`synocli` is a CLI for Synology DSM WebAPI.
It currently focuses on Download Station (`ds`) and File Station (`fs`) workflows.

## Quick Start

Build natively:

```bash
make build
./bin/synocli --help
```

Or build and run via Docker:

```bash
docker build -t synocli .
docker run --rm synocli --help
```

Pass credentials and options as you would with the native binary:

```bash
docker run --rm \
  -v "$HOME/.synocli:/root/.synocli:ro" \
  synocli --endpoint https://192.168.0.1:5001 ds list
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
./bin/synocli auth ping
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
./bin/synocli ds list
./bin/synocli ds add "https://example.com/file.iso"
./bin/synocli ds delete dbid_1
```

File Station:

```bash
./bin/synocli fs shares
./bin/synocli fs ls /volume1
./bin/synocli fs cp /volume1/a.txt --to /volume1/archive
./bin/synocli fs delete /volume1/archive/old -r
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