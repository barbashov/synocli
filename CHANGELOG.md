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
