# Changelog

All notable changes to `synocli` are documented in this file.

The format is based on Keep a Changelog and uses Semantic Versioning.

## [Unreleased]

### Added
- Placeholder for upcoming unreleased changes.

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
