# Changelog

All notable changes to `synocli` are documented in this file.

The format is based on Keep a Changelog and uses Semantic Versioning.

## [Unreleased]

### Agent Notes
```yaml
breaking_changes: []
commands_added: []
commands_changed: []
flags_added: []
flags_changed: []
behavior_changes: []
skill_update_action: "No skill update required until this section is released."
```

## [0.4.10] - 2026-05-22

### Fixed
- `ds get <unknown-id>` on DS2-capable NAS firmware now exits 3 (`synology_error`) instead of 1. Previously only DS v1's code 404 mapped to exit 3; DS2 returns code 501 for the same "task not found" condition and is now mapped equivalently. The stderr message is also `invalid task id` instead of `unmapped`.

### Agent Notes
```yaml
breaking_changes: []
commands_added: []
commands_changed: []
flags_added: []
flags_changed: []
behavior_changes:
  - "ds get <unknown-id> on DS2 firmware now exits 3 (was 1); DS v1 behavior unchanged. DS error code 501 is mapped to message 'invalid task id' (was 'unmapped')."
skill_update_action: "No skill update required (exit-code mapping only; no command or flag surface changed)."
```

## [0.4.9] - 2026-05-22

### Added
- `synocli info disks` — show per-bay drive state in a single table: BAY, NAME, MODEL, SIZE, TYPE (HDD / SSD with remaining-life % when known), TEMP, AGE (power-on hours rendered as `5y 6mo`), HEALTH verdict, and USED BY (pool id). The HEALTH cell combines DSM's curated flags (`smart`, `exceed_bad_sector_thr`, `below_remain_life_thr`, `read_only`) with raw SMART id 5 (Reallocated_Sector_Ct) and id 197 (Current_Pending_Sector) so sub-threshold issues surface early; affected rows get a `warn` badge and a one-line footer summarises healthy / warning counts. Backed by `SYNO.Storage.CGI.Storage` `load_info` plus one `SYNO.Storage.CGI.Smart` `get_health_info` call per disk (best-effort: a per-disk SMART failure renders that row's AGE/HEALTH as `error WARN` rather than aborting the command). `--json` returns `{ "storage": <load_info>, "health": { "<disk_id>": { "overview": …, "reallocated": N, "pending": N, "error": "…" } } }`.
- `synocli info disks smart` — show detailed SMART data for each drive (or one drive when `--disk <id>` is passed): a per-disk KV header (model, serial, firmware, type, size, used-by, SMART status + test, power-on hours, IDNF/Retry/UNC counters, remaining-life, next scheduled tests) followed by the canonical SMART attribute table (ID, NAME, CURRENT, WORST, THRESHOLD, RAW, STATUS). Backed by the same two endpoints. `--json` returns `{ "health": { "<disk_id>": { "overview": …, "smart_info": [...], "reallocated": N, "pending": N, "error": "…" } } }`. Unknown `--disk <id>` exits 1 with `validation_error`.

### Agent Notes
```yaml
breaking_changes: []
commands_added:
  - "info disks: per-bay drive state table (BAY/NAME/MODEL/SIZE/TYPE/TEMP/AGE/HEALTH/USED BY). HEALTH combines DSM flags with raw SMART id 5/197 for early warning. JSON keys: storage, health[<id>].overview, health[<id>].reallocated, health[<id>].pending, health[<id>].error (best-effort per-disk SMART failures)"
  - "info disks smart: detailed SMART overview + canonical attribute table per drive. JSON keys: health[<id>].overview, health[<id>].smart_info, health[<id>].reallocated, health[<id>].pending, health[<id>].error"
commands_changed: []
flags_added:
  - "info disks smart --disk <id>: limit to a single disk by id (e.g. sda); unknown id exits 1 with validation_error"
flags_changed: []
behavior_changes: []
skill_update_action: "Document `synocli info disks` and `synocli info disks smart`, the --disk flag, the HEALTH verdict semantics (DSM flags + raw SMART id 5/197), and the JSON schemas under `health[<disk_id>]`."
```

## [0.4.8] - 2026-05-22

### Added
- `synocli info` — show NAS model, serial, DSM version, uptime, CPU/RAM/temperature, per-volume storage usage (path, RAID type, filesystem, used/total/%), and reboot-required flag. Backed by `SYNO.DSM.Info` (v2 `getinfo`), `SYNO.Core.System` (v3 `info`), `SYNO.Core.Hardware.NeedReboot` (v1 `get`), and `SYNO.Storage.CGI.Storage` (v1 `load_info`). Storage and reboot calls are best-effort: their failures degrade the output rather than failing the command, and JSON output surfaces them as `storage_error` / `reboot_error`. `--json` returns the combined raw payload under `dsm`, `system`, `reboot`, and `storage` keys.
- `synocli info utilization` — show live CPU/RAM/disk/network load plus diagnostic sensors: per-core/system/user/other CPU load percentages, 1m/5m/15m load averages, real and swap memory usage, system temperature (+ warning flag), per-disk model/temperature/SMART/utilization/read+write throughput, per-interface RX/TX throughput, and DSM `is_system_crashed` / `upgrade_ready` flags. Backed by `SYNO.Core.System.Utilization` (v1 `get`), `SYNO.DSM.Info` (for the system temperature), `SYNO.Core.System.Status` (v1 `get`), and `SYNO.Storage.CGI.Storage` (v1 `load_info`).

### Agent Notes
```yaml
breaking_changes: []
commands_added:
  - "info: show DSM model, serial, version, uptime, CPU/RAM/temperature, per-volume storage usage and reboot-required flag; JSON keys: dsm, system, reboot, storage (plus *_error for best-effort failures)"
  - "info utilization: show live CPU/RAM/disk/network load plus system temperature, per-disk temp + SMART status, and is_system_crashed / upgrade_ready flags; JSON keys: utilization, dsm, status, storage (plus *_error)"
commands_changed: []
flags_added: []
flags_changed: []
behavior_changes: []
skill_update_action: "Document `synocli info` and `synocli info utilization`, including their --json payload schemas and the best-effort *_error fields."
```

## [0.4.7] - 2026-05-17

### Added
- `ds max-tasks get` — display the current maximum number of simultaneously active download tasks plus DSM's upper bound (`Limit`). JSON envelope exposes `max_tasks` and `max_tasks_limit`.
- `ds max-tasks set <n>` — set the maximum number of simultaneously active download tasks. Value must be a positive integer within DSM's `max_tasks_limit`. The upper bound is enforced client-side (DSM silently accepts values above the limit even though the Download Station UI treats them as invalid). Backed by `SYNO.DownloadStation2.Settings.Scheduler` `get` / `set` (DSM2 entry.cgi).

### Agent Notes
```yaml
breaking_changes: []
commands_added:
  - "ds max-tasks get: show current maximum active download tasks (max_tasks) and DSM upper bound (max_tasks_limit)"
  - "ds max-tasks set <n>: set the maximum number of simultaneously active download tasks (positive integer; client-side capped at max_tasks_limit because DSM silently accepts higher values)"
commands_changed: []
flags_added: []
flags_changed: []
behavior_changes: []
skill_update_action: "Update CLI skill to document `ds max-tasks get` and `ds max-tasks set <n>` subcommands."
```

## [0.4.6] - 2026-05-16

### Added
- `ds bandwidth get` — display current BT bandwidth limits (download / upload, KB/s). `0` is rendered as `unlimited`.
- `ds bandwidth set` — update BT bandwidth limits via `--bt-max-download` and/or `--bt-max-upload` (KB/s; `0` = unlimited). At least one of the two flags is required; only the flags explicitly provided are sent to DSM, so the other cap is preserved. Backed by `SYNO.DownloadStation.Info` `getconfig` / `setserverconfig`.

### Agent Notes
```yaml
breaking_changes: []
commands_added:
  - "ds bandwidth get: show current BT bandwidth caps (bt_max_download, bt_max_upload in KB/s; 0 means unlimited)"
  - "ds bandwidth set: update BT bandwidth caps via --bt-max-download / --bt-max-upload (KB/s; 0 = unlimited; at least one flag required; only provided fields are sent)"
commands_changed: []
flags_added:
  - "ds bandwidth set --bt-max-download (int, KB/s, 0 = unlimited)"
  - "ds bandwidth set --bt-max-upload (int, KB/s, 0 = unlimited)"
flags_changed: []
behavior_changes: []
skill_update_action: "Update CLI skill to document `ds bandwidth get` and `ds bandwidth set` subcommands."
```

## [0.4.5] - 2026-04-06

### Fixed
- `fs list`, `fs mkdir`, and `fs search` now strip trailing slashes from the folder path argument before sending to the API. Synology FileStation returns error 418 for paths like `/foo/bar/`; the path is normalized with `path.Clean` before the request.
- Map FileStation API error code 418 to `"invalid path format"` instead of `"unmapped"`.
- Fix unchecked `fmt.Fprint` return values in `cli-update` download progress bar (lint: `errcheck`).

### Agent Notes
```yaml
breaking_changes: []
commands_added: []
commands_changed: []
flags_added: []
flags_changed: []
behavior_changes:
  - "fs list/mkdir/search: trailing slashes on folder path arguments are silently stripped before the API call"
skill_update_action: "No skill update required."
```

## [0.4.4] - 2026-03-29

### Added
- `cli-update` now shows a live download progress bar on stderr when connected to a terminal: `[##########----------] 7.3 MB / 14.5 MB (50.3%)  2.1 MB/s`. Progress is suppressed in `--json` mode and when stdout/stderr is piped.

### Agent Notes
```yaml
breaking_changes: []
commands_added: []
commands_changed:
  - "cli-update: shows download progress bar on TTY (stderr); suppressed in --json mode or when piped"
flags_added: []
flags_changed: []
behavior_changes:
  - "cli-update prints a live ASCII progress bar to stderr during the binary archive download when output is a terminal"
skill_update_action: "No skill update required."
```

## [0.4.3] - 2026-03-28

### Fixed
- Fix recursive upload double-rename bug: when upload fails but fallback rename succeeds, the second rename no longer re-runs on an already-renamed file; `uploaded_files` counter now reflects actual successes.
- `fs download` now validates HTTP status code before writing to output file, preventing silent save of HTML error pages from reverse proxies or 5xx responses.
- `outputError` in JSON mode now falls back to stderr when JSON write fails (broken pipe, full disk) instead of producing zero output.
- `--user` is now validated before login even when `--password` is pre-set, surfacing a clear `validation_error` instead of a confusing `auth_failed`.
- `fs list` and `fs tasks` now reject negative `--offset` and `--limit` values with a validation error.

### Changed
- Enabled `errcheck` and `staticcheck` linters in `.golangci.yml` for stronger static analysis.

### Agent Notes
```yaml
breaking_changes: []
commands_added: []
commands_changed:
  - "fs download: now returns error on non-200 HTTP status instead of silently writing error body to file"
  - "fs list: rejects negative --offset and --limit"
  - "fs tasks: rejects negative --offset and --limit"
flags_added: []
flags_changed: []
behavior_changes:
  - "JSON error output falls back to stderr when stdout write fails"
  - "Recursive upload counter now accurate; double-rename on error path eliminated"
  - "Missing --user with --password now returns validation_error instead of auth_failed"
skill_update_action: "Refresh validation rules for fs download, fs list, fs tasks."
```

## [0.4.2] - 2026-03-28

### Fixed
- `fs search clear` now attempts all task IDs instead of stopping on first failure, reports partial failures.
- `fs extract` and `fs compress` validate `--to` flag before login, avoiding unnecessary session creation on bad input.
- Upload goroutine in File Station client now explicitly cancels pipe on HTTP failure, preventing potential goroutine leak.
- Fixed import ordering in `filestation/ops.go` and trailing newline in `ds_test.go` (gofmt).

### Changed
- `ds cleanup` help text now documents that `--json` mode skips the confirmation prompt.
- Linter config adds `govet`, `ineffassign`, and `misspell` linters.
- Makefile lint target updated to `golangci-lint:v2.11.4` to match CI.

### Agent Notes
```yaml
breaking_changes: []
commands_added: []
commands_changed: []
flags_added: []
flags_changed: []
behavior_changes:
  - "fs search clear now clears all task IDs and reports partial failures instead of aborting on first error."
  - "fs extract/compress validate --to before session login."
skill_update_action: "No skill update required."
```

## [0.4.1] - 2026-03-27

### Fixed
- `ds pause`, `ds resume`, `ds delete` now encode task IDs as JSON arrays for DownloadStation2 API (DSM 7+), matching the format used by `ds list` and `ds get`.
- `ds cleanup` correctly parses failed task IDs returned as JSON arrays or comma-separated strings by DownloadStation2.
- Fixed indentation in `ds cleanup` command handler.

### Changed
- `ds cleanup` now prints "Nothing to cleanup" instead of a full summary block when no tasks match.

### Agent Notes
```yaml
breaking_changes: []
commands_added: []
commands_changed: []
flags_added: []
flags_changed: []
behavior_changes:
  - "ds pause/resume/delete now send JSON array IDs for DownloadStation2 API (DSM 7+); v1 API unchanged."
  - "ds cleanup empty-result output changed from summary block to single-line message."
skill_update_action: "No skill update required."
```

## [0.4.0] - 2026-03-27

### Added
- New `ds cleanup` command to delete finished Download Station task records while keeping downloaded data intact.
- New `-s, --include-seeding` and `-y, --yes` flags for `ds cleanup`.

### Agent Notes
```yaml
breaking_changes: []
commands_added:
  - "ds cleanup"
commands_changed: []
flags_added:
  - "ds cleanup --include-seeding (-s)"
  - "ds cleanup --yes (-y)"
flags_changed: []
behavior_changes:
  - "ds cleanup prompts for confirmation in CLI mode unless --yes is set; JSON mode never prompts."
  - "ds cleanup confirmation shows the same task table style as ds list."
  - "ds cleanup default scope is finished tasks only; use --include-seeding to include seeding tasks."
  - "ds cleanup removes task records only and preserves downloaded data."
skill_update_action: "Update synocli command references for ds cleanup, -s/--include-seeding, and -y/--yes confirmation bypass."
```

## [0.3.4] - 2026-03-27

### Added
- Download Station task output now includes computed ETA for downloads:
  - Human output (`ds list`, `ds get`) shows readable duration (for example, `5 days 2 hours`, `1 minute 30 seconds`).
  - JSON output includes `eta_seconds` on each task object.

### Changed
- `eta_seconds` is computed client-side from task size, downloaded bytes, and current download speed because DSM does not provide ETA directly.
- When ETA cannot be estimated (for example zero speed while download is incomplete), JSON uses `eta_seconds: -1`; completed tasks report `0`.

### Agent Notes
```yaml
breaking_changes: []
commands_added: []
commands_changed:
  - "ds list: adds ETA column in human table and eta_seconds in JSON task objects"
  - "ds get: adds ETA field in human detail and eta_seconds in JSON task object"
flags_added: []
flags_changed: []
behavior_changes:
  - "Download ETA is now computed locally (size/downloaded/speed) and exposed as human-readable ETA plus eta_seconds in JSON."
skill_update_action: "Update synocli output schema references for ds list/get to include eta_seconds semantics (-1 unknown, 0 completed)."
```

## [0.3.3] - 2026-03-27

### Added
- New `cli-update` command to check GitHub Releases and auto-install a newer synocli binary.
- New installer script:
  - `curl -fsSL https://raw.githubusercontent.com/barbashov/synocli/main/install.sh | bash`
  - Supports linux/darwin on amd64/arm64 (WSL2 follows linux flow).

### Changed
- Non-JSON commands now perform periodic background update checks and suggest `synocli cli-update` when a newer release exists.
- Added global flag `--no-update-check` to skip the background update check for a single invocation.

### Agent Notes
```yaml
breaking_changes: []
commands_added:
  - "cli-update"
commands_changed:
  - "root command: periodic background update checks for non-JSON commands"
flags_added:
  - "--no-update-check (global)"
flags_changed: []
behavior_changes:
  - "Non-JSON command runs now periodically check for a newer GitHub release and print an update hint."
skill_update_action: "Update synocli usage skills to include install script, cli-update command, and --no-update-check flag."
```

## [0.3.2] - 2026-03-27

### Added
- `make build-release VERSION=vX.Y.Z` now produces cross-platform release archives for linux/darwin/windows on amd64/arm64 and writes `dist/SHA256SUMS`.
- `make build-platform VERSION=vX.Y.Z GOOS=<os> GOARCH=<arch>` and `make checksums` targets for release artifact workflows.

### Changed
- CI now builds release archives on `v*` tag pushes and publishes a GitHub Release with all platform binaries and checksums.

### Agent Notes
```yaml
breaking_changes: []
commands_added: []
commands_changed: []
flags_added: []
flags_changed: []
behavior_changes:
  - "Tag pushes now publish GitHub Release binary assets for linux/darwin/windows amd64+arm64 with SHA256SUMS."
skill_update_action: "Update release automation docs in skills that mention distribution channels."
```

## [0.3.1] - 2026-03-27

### Changed
- CLI layer moved from `cmd/synocli/` (`package main`) into `internal/cli/` and `internal/cmdutil/` packages. Pure structural refactor; no behaviour change.

### Fixed
- E2e test: magnet task was not deleted before `assert_task_deleted`, causing a spurious failure when the torrent task was chosen for the pause/resume cycle instead.

### Agent Notes
```yaml
breaking_changes: []
commands_added: []
commands_changed: []
flags_added: []
flags_changed: []
behavior_changes: []
skill_update_action: "No skill update required."
```

## [0.3.0] - 2026-03-27

### Added
- `reuse_session=true` config directive: session SID is cached in `~/.synocli/session` (chmod 600) and reused across consecutive calls, skipping the login/logout round-trips. On session expiry or an invalid cached SID the session is transparently renewed without user interaction.
- `cli-config --help` now lists all supported config file directives with their types and defaults.

### Fixed
- Error code 119 (SID not found) is now treated as a session expiry and triggers a transparent re-login when `reuse_session` is enabled, instead of surfacing an unmapped error.

### Agent Notes
```yaml
breaking_changes: []
commands_added: []
commands_changed:
  - "cli-config: added long help listing all config directives"
flags_added: []
flags_changed: []
behavior_changes:
  - "reuse_session=true in config caches the session SID in ~/.synocli/session; subsequent calls skip login/logout"
  - "error 119 (SID not found) now triggers session refresh instead of an unmapped error"
skill_update_action: "Update skill: document reuse_session directive and session file location."
```

## [0.2.2] - 2026-03-27

### Fixed
- CI docker job no longer pushes to GHCR on `main` branch pushes; images are published only on `vX.Y.Z` tag events, preventing duplicate packages when a commit and its release tag are pushed together.

### Agent Notes
```yaml
breaking_changes: []
commands_added: []
commands_changed: []
flags_added: []
flags_changed: []
behavior_changes: []
skill_update_action: "No skill update required."
```

## [0.2.1] - 2026-03-27

### Added
- CI docker job now pushes to GitHub Container Registry (`ghcr.io`) on `main` branch and version tag pushes; PRs continue to build-only.

### Changed
- README Docker quick-start now references the pre-built `ghcr.io/ivbarbashov/synocli:latest` image; local `docker build` is kept as a secondary option.

### Agent Notes
```yaml
breaking_changes: []
commands_added: []
commands_changed: []
flags_added: []
flags_changed: []
behavior_changes: []
skill_update_action: "No skill update required."
```

## [0.2.0] - 2026-03-27

### Added
- `--watch` flag on `ds list`, `fs list`, and `fs tasks` for continuous polling mode; replaces the removed `ds watch`, `fs watch folder`, `fs watch tasks` subcommands.
- `--interval` flag on `ds list`, `fs list`, `fs tasks` (default 2s).
- `--id` and `--status` filter flags on `ds list`.
- `-r` shorthand for `--recursive` on `fs list`, `fs delete`, `fs search`.
- `rm` alias for `fs delete` and `ds delete`.
- `ls` alias for `ds list`.

### Changed
- **`ds add --destination`** renamed to **`--to`** for consistency with `fs copy`/`fs move`.
- **`fs list --filetype`** renamed to **`--file-type`**.
- **`fs delete -r`** short flag restored as `-r` (was removed in a prior refactor, now re-added alongside `--recursive`).
- **`tasks-clear`**: `--task-id` flag replaced with optional positional args (`fs tasks-clear [<task-id>...]`).
- `fs --help` now organises subcommands into four groups: File Operations, Archive, Utilities, Background Tasks.
- Async task helpers moved from top-level `fs` subcommands to subcommands of their parent:
  - `fs dir-size-status` / `fs dir-size-stop` → `fs dir-size status` / `fs dir-size stop`
  - `fs md5-status` / `fs md5-stop` → `fs md5 status` / `fs md5 stop`
  - `fs compress-status` / `fs compress-stop` → `fs compress status` / `fs compress stop`
  - `fs extract-status` / `fs extract-stop` → `fs extract status` / `fs extract stop`
  - `fs search-results` / `fs search-stop` / `fs search-clear` → `fs search results` / `fs search stop` / `fs search clear`
- DSM session name in audit log changed from `DownloadStation`/`FileStation` to `synocli`.
- `auth` JSON envelope now includes `task` and `fs_*` keys in `meta.api_version`.

### Removed
- `ds watch` subcommand (use `ds list --watch`).
- `fs watch folder` and `fs watch tasks` subcommands (use `fs list --watch` and `fs tasks --watch`).
- `fs info` command.
- Flat async helper subcommands at top-level `fs`: `dir-size-status`, `dir-size-stop`, `md5-status`, `md5-stop`, `compress-status`, `compress-stop`, `extract-status`, `extract-stop`, `search-results`, `search-stop`, `search-clear`.

### Agent Notes
```yaml
breaking_changes:
  - "ds watch removed; use ds list --watch"
  - "fs watch folder removed; use fs list --watch"
  - "fs watch tasks removed; use fs tasks --watch"
  - "fs info removed"
  - "ds add --destination renamed to --to"
  - "fs list --filetype renamed to --file-type"
  - "tasks-clear --task-id flag removed; use positional args"
  - "fs dir-size-status/stop moved to fs dir-size status/stop"
  - "fs md5-status/stop moved to fs md5 status/stop"
  - "fs compress-status/stop moved to fs compress status/stop"
  - "fs extract-status/stop moved to fs extract status/stop"
  - "fs search-results/stop/clear moved to fs search results/stop/clear"
commands_added:
  - "fs dir-size status"
  - "fs dir-size stop"
  - "fs md5 status"
  - "fs md5 stop"
  - "fs compress status"
  - "fs compress stop"
  - "fs extract status"
  - "fs extract stop"
  - "fs search results"
  - "fs search stop"
  - "fs search clear"
commands_changed:
  - "ds list: added --watch, --interval, --id, --status flags; added ls alias"
  - "fs list: added --watch, --interval flags; added -r shorthand for --recursive"
  - "fs tasks: added --watch, --interval flags"
  - "fs delete: added rm alias, -r shorthand for --recursive"
  - "fs tasks-clear: positional args instead of --task-id flag"
  - "ds delete: added rm alias"
flags_added:
  - "ds list --watch"
  - "ds list --interval"
  - "ds list --id"
  - "ds list --status"
  - "fs list --watch"
  - "fs list --interval"
  - "fs tasks --watch"
  - "fs tasks --interval"
  - "fs list -r (shorthand for --recursive)"
  - "fs delete -r (shorthand for --recursive)"
  - "fs search -r (shorthand for --recursive)"
flags_changed:
  - "ds add --destination renamed to --to"
  - "fs list --filetype renamed to --file-type"
  - "tasks-clear --task-id removed; use positional args"
behavior_changes:
  - "DSM session name in audit log is now 'synocli' (was 'DownloadStation' or 'FileStation')"
  - "fs --help output is grouped into sections"
  - "auth JSON envelope meta.api_version now includes task and fs_* keys"
skill_update_action: "Full refresh required: watch flags, renamed flags, removed commands, nested async helpers."
```

## [0.1.0] - 2026-03-27

### Added
- Initial public CLI workflows for Download Station (`ds`) and File Station (`fs`).
- `auth` connectivity and API discovery commands.
- Config management (`cli-config`) and JSON envelope output mode.
- Docker image build flow and CI checks for lint/test/docker-build.

### Changed
- Introduced semantic versioning and release process with git tags (`vX.Y.Z`).
- Added `synocli --version` and `synocli version` with build metadata output.

### Agent Notes
```yaml
breaking_changes: []
commands_added:
  - "version"
commands_changed: []
flags_added:
  - "--version"
flags_changed: []
behavior_changes:
  - "Top-level --version prints semver only."
  - "version command exposes version, commit, and build_date (JSON envelope when --json is set)."
skill_update_action: "Add version discovery guidance to synocli skills and treat v0.1.0 as baseline release."
```
