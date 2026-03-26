# File Station Manual Test Plan

## Prerequisites

- Built binary: `./bin/synocli`
- Synology endpoint reachable over HTTPS
- Credentials file available (`user=...`, `password=...`)
- Test share and fixtures:
  - Remote root: `/volume1/synocli-e2e`
  - Local fixture directory: `./tmp/fs-fixtures`
  - One text file, one nested directory, one zip archive, one large file (50MB+)

Example env:

```bash
ENDPOINT="https://<nas>:5001"
CREDS="/path/to/creds.env"
BASE="/volume1/synocli-e2e"
```

Fixture generation commands:

```bash
mkdir -p ./tmp/fs-fixtures/tree/sub
printf 'hello synocli\n' > ./tmp/fs-fixtures/a.txt
printf 'nested file\n' > ./tmp/fs-fixtures/tree/sub/file.txt
dd if=/dev/zero of=./tmp/fs-fixtures/large.bin bs=1m count=50
cd ./tmp/fs-fixtures && zip -r ./archive.zip ./tree ./a.txt >/dev/null && cd -
```

Optional reset of remote fixture root:

```bash
synocli fs delete "$ENDPOINT" "$BASE" -r --credentials-file "$CREDS" --insecure-tls --json || true
synocli fs mkdir "$ENDPOINT" /volume1 synocli-e2e --parents --credentials-file "$CREDS" --insecure-tls
```

## 1. Auth and Discovery Smoke

1. `synocli auth ping "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls`
2. `synocli auth api-info "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls --prefix SYNO.FileStation`
3. Verify success and that File Station APIs are listed.

## 2. Core File Ops

1. Create test root: `synocli fs mkdir "$ENDPOINT" /volume1 synocli-e2e --parents --credentials-file "$CREDS" --insecure-tls`
2. List shares: `synocli fs shares "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls`
3. List root folder: `synocli fs list "$ENDPOINT" "$BASE" --credentials-file "$CREDS" --insecure-tls`
4. Upload file: `synocli fs upload "$ENDPOINT" ./tmp/fs-fixtures/a.txt "$BASE" --credentials-file "$CREDS" --insecure-tls`
5. Rename file: `synocli fs rename "$ENDPOINT" "$BASE/a.txt" a-renamed.txt --credentials-file "$CREDS" --insecure-tls`
6. Get file info: `synocli fs get "$ENDPOINT" "$BASE/a-renamed.txt" --credentials-file "$CREDS" --insecure-tls`
7. Copy and move file:
   - `synocli fs copy "$ENDPOINT" "$BASE/a-renamed.txt" --to "$BASE/copy" --credentials-file "$CREDS" --insecure-tls`
   - `synocli fs move "$ENDPOINT" "$BASE/copy/a-renamed.txt" --to "$BASE/moved" --credentials-file "$CREDS" --insecure-tls`
8. Delete file: `synocli fs delete "$ENDPOINT" "$BASE/moved/a-renamed.txt" --credentials-file "$CREDS" --insecure-tls`

Expected:
- Commands succeed and produce correct file state transitions.
- `fs delete` on directory without `-r` fails with validation error.

## 3. Recursive Upload (cp -r Semantics)

1. Prepare local tree `./tmp/fs-fixtures/tree/sub/file.txt`.
2. Case A (dest exists and is dir):
   - Ensure `$BASE/targetA` exists.
   - `synocli fs upload "$ENDPOINT" ./tmp/fs-fixtures/tree "$BASE/targetA" --credentials-file "$CREDS" --insecure-tls`
   - Verify remote path contains `$BASE/targetA/tree/...`.
3. Case B (dest does not exist):
   - `synocli fs upload "$ENDPOINT" ./tmp/fs-fixtures/tree "$BASE/targetB" --credentials-file "$CREDS" --insecure-tls`
   - Verify remote path contains `$BASE/targetB/...`.
4. Conflict policy checks:
   - Default should fail on existing file.
   - `--overwrite` replaces.
   - `--skip-existing` keeps existing.

## 4. Download

1. Single file download:
   - `synocli fs download "$ENDPOINT" "$BASE/a-renamed.txt" --output ./tmp/download-a.txt --credentials-file "$CREDS" --insecure-tls`
2. Directory/archive download:
   - `synocli fs download "$ENDPOINT" "$BASE" --output ./tmp/base.zip --credentials-file "$CREDS" --insecure-tls`
3. Validate output files exist and sizes are non-zero.

## 5. Search Workflow

1. Start+wait one-shot:
   - `synocli fs search "$ENDPOINT" "$BASE" --pattern file --credentials-file "$CREDS" --insecure-tls`
2. Async flow:
   - `synocli fs search "$ENDPOINT" "$BASE" --pattern file --async --credentials-file "$CREDS" --insecure-tls --json`
   - Capture `task_id`.
   - `synocli fs search-results "$ENDPOINT" <task_id> --credentials-file "$CREDS" --insecure-tls`
   - `synocli fs search-stop "$ENDPOINT" <task_id> --credentials-file "$CREDS" --insecure-tls`
3. Clear search cache:
   - `synocli fs search-clear "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls`

## 6. DirSize and MD5

1. Dir size sync:
   - `synocli fs dir-size "$ENDPOINT" "$BASE" --credentials-file "$CREDS" --insecure-tls`
2. Dir size async:
   - `synocli fs dir-size "$ENDPOINT" "$BASE" --async --json --credentials-file "$CREDS" --insecure-tls`
   - `synocli fs dir-size-status "$ENDPOINT" <task_id> --credentials-file "$CREDS" --insecure-tls`
3. MD5 sync:
   - `synocli fs md5 "$ENDPOINT" "$BASE/a-renamed.txt" --credentials-file "$CREDS" --insecure-tls`
4. MD5 async stop path:
   - start async and call `fs md5-stop`.

## 7. Extract and Compress

1. Compress sync:
   - `synocli fs compress "$ENDPOINT" "$BASE" --to "$BASE/archive.zip" --credentials-file "$CREDS" --insecure-tls`
2. Compress async status:
   - start with `--async`, poll with `fs compress-status`, stop with `fs compress-stop`.
3. Extract sync:
   - `synocli fs extract "$ENDPOINT" "$BASE/archive.zip" --to "$BASE/extracted" --credentials-file "$CREDS" --insecure-tls`
4. Extract async status/stop similarly.

## 8. Background Tasks and Watch

1. List background tasks:
   - `synocli fs tasks "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls`
2. Clear finished tasks:
   - `synocli fs tasks-clear "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls`
3. Human watch tasks:
   - `synocli fs watch tasks "$ENDPOINT" --interval 2s --credentials-file "$CREDS" --insecure-tls`
4. Human watch folder:
   - `synocli fs watch folder "$ENDPOINT" "$BASE" --interval 2s --recursive --credentials-file "$CREDS" --insecure-tls`

Expected:
- TTY refreshes in-place.
- Ctrl+C exits cleanly.

## 9. JSON Contract Validation (OpenClaw-friendly)

Run each key command with `--json` and assert:

1. Envelope contains `ok`, `command`, `meta.timestamp`, `meta.duration_ms`, `meta.endpoint`.
2. On failure, envelope contains structured `error.code`, `error.message`, and details when available.
3. Watch commands emit JSONL snapshots (multiple lines), each a valid envelope.

Suggested checks:

```bash
synocli fs list "$ENDPOINT" "$BASE" --json --credentials-file "$CREDS" --insecure-tls | jq .ok
synocli fs watch tasks "$ENDPOINT" --json --interval 1s --credentials-file "$CREDS" --insecure-tls | head -n 3 | jq .command
```

## 10. DS Regression

1. `synocli ds list "$ENDPOINT" --credentials-file "$CREDS" --insecure-tls`
2. `synocli ds watch "$ENDPOINT" --interval 2s --credentials-file "$CREDS" --insecure-tls --json | head -n 3`

Expected:
- Existing Download Station behavior is unchanged.

## Optional: Run All Steps Via Script

You can run a scripted subset/end-to-end flow:

```bash
ENDPOINT="https://<nas>:5001" \
CREDS="/path/to/creds.env" \
BASE="/volume1/synocli-e2e" \
RESET_REMOTE=1 \
./testplans/filestation.sh
```
