# synocli end-to-end tests

A pytest suite that exercises the `synocli` binary against a live Synology
NAS. Not part of CI — run locally before cutting a release or when reviewing
a change that touches CLI surface, command output, or error handling.

## Running via Docker (recommended)

No host Python or Go install required — just Docker. The image is multi-stage:
it builds `synocli` and bundles pytest with all dependencies.

```
make test-e2e \
  ENDPOINT=https://nas.local:5001 \
  CREDS=./.creds \
  INSECURE_TLS=1
```

Useful Makefile variables:

| Variable | Effect |
|---|---|
| `ENDPOINT` | (required) DSM endpoint, e.g. `https://nas.local:5001` |
| `CREDS` | (required) path to a credentials file (`user=...`, `password=...`); bind-mounted read-only, perms normalized inside the container |
| `INSECURE_TLS=1` | Pass `--insecure-tls` to synocli |
| `BASE=/volume1/synocli-e2e` | Remote sandbox root (default `/volume1/synocli-e2e`) |
| `DS_DESTINATION=/path` | Download Station destination folder |
| `REFRESH_TORRENT_CACHE=1` | Re-fetch the webtorrent.io fixture (else cached in `tests_e2e/.cache/webtorrent/`) |
| `PYTEST_ARGS="-k bandwidth -x"` | Forwarded to pytest (test selection, fail-fast, etc.) |
| `E2E_IMAGE=synocli-e2e:latest` | Override the image tag |

To rebuild the image explicitly (e.g., after editing the Dockerfile):

```
make test-e2e-build
```

The container is invoked with `--user $(id -u):$(id -g)` so the persisted
cache directory (`tests_e2e/.cache/`) stays owned by the host user.

## Running without Docker

If you'd rather skip the container, install the deps in a virtualenv and
run pytest directly:

```
make build
python3 -m venv .venv
source .venv/bin/activate
pip install "pytest>=8.0" "pytest-timeout>=2.3" "pytest-subtests>=0.13"
pytest tests_e2e/ -v \
  --endpoint https://nas.local:5001 \
  --credentials-file ./.creds \
  --insecure-tls
```

Useful pytest options (forward via `PYTEST_ARGS=...` when using Docker):

| Flag | Purpose |
|---|---|
| `--bin <path>` | Path to the `synocli` binary (default `./bin/synocli` — auto-set inside the Docker image) |
| `--base <path>` | Remote sandbox root (default `/volume1/synocli-e2e`); a per-run subdirectory is created and recursively deleted at session end |
| `--ds-destination <path>` | Download Station destination folder (DSM default when unset) |
| `--refresh-torrent-cache` | Force re-fetch of the webtorrent.io fixture (otherwise cached in `tests_e2e/.cache/webtorrent/`) |
| `--allow-skip-on-unreachable` | Skip rather than `pytest.exit` when the initial `auth ping` fails — useful during dev loops |
| `-k <expr>` | Standard pytest test selection |
| `-m fs` / `-m ds` | Run only the file-station or download-station suite |

## What lives where

- `lib/cli.py` — `SynoCLI` runner. On non-zero exit it auto-retries with
  `--debug` once and surfaces both stderr blobs.
- `lib/fs_helpers.py` — race-tolerant stop helper (`assert_stop_or_finished`),
  shared by every FS async test.
- `lib/webtorrent.py` — caches the upstream Big Buck Bunny `.torrent` + magnet
  in `tests_e2e/.cache/webtorrent/` keyed by SHA256 of the source URL.
- `conftest.py` — session fixtures: `cli`, `sandbox_root` (recursive sweep
  on teardown), `ds_task_tracker` (durable JSON log, pre-/post-sweep),
  `bandwidth_snapshot`, `max_tasks_snapshot`, `torrent_fixture`.
- `tests/` — one file per command group; see file names.

## State and cleanup

- **Remote FS state**: `sandbox_root` creates `<base>/run-<id>/` per session
  and recursively deletes it on teardown. Individual tests use the
  `fs_sandbox` fixture which carves out a per-test subdirectory.
- **DS tasks**: every task ID created via `ds add` is logged to
  `tests_e2e/.cache/ds-task-ids.jsonl`. On session start the tracker sweeps
  any IDs left behind by an interrupted prior run; on teardown it sweeps the
  current run.
- **DS bandwidth / max-tasks**: snapshot fixtures capture the values before
  each test and restore them in the `finally` half of the yield, so a
  crashed `set` test doesn't leave the NAS in an unexpected state.

## Auth, info, FS and DS coverage

| Area | File |
|---|---|
| `auth ping`, `auth whoami`, `auth api-info`, bad-credential exit-2 | `tests/test_auth.py` |
| `info`, `info utilization`, `info disks`, `info disks smart` (+ `--disk`) | `tests/test_info.py` |
| FS upload/rename/get/download/cp/mv/list/md5/delete (ordered subtests) | `tests/test_fs_lifecycle.py` |
| FS async search / dir-size / md5 with race-tolerant stop | `tests/test_fs_async.py` |
| FS compress/extract sync + async | `tests/test_fs_archive.py` |
| FS `tasks` list + `tasks-clear` | `tests/test_fs_tasks.py` |
| DS add (URL / magnet / torrent) / list / get / pause / wait timeout (exit 5) / resume / watch snapshot / delete | `tests/test_ds_lifecycle.py` |
| DS bandwidth get/set with restore + validation | `tests/test_ds_bandwidth.py` |
| DS max-tasks get/set with restore + range/parse validation | `tests/test_ds_max_tasks.py` |
