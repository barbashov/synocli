from __future__ import annotations

from typing import Optional

import pytest

from lib.cli import SynoCLI, envelope_data
from lib.waits import wait_for


pytestmark = pytest.mark.ds


def _add_torrent(cli: SynoCLI, source: str, destination: Optional[str]) -> str:
    args = ["ds", "add", source]
    if destination:
        args.extend(["--to", destination])
    data = envelope_data(cli.run(*args))
    ids = data.get("task_ids") or []
    assert ids and ids[0], f"empty task_ids in ds add response: {data!r}"
    return ids[0]


def _list_files(cli: SynoCLI, task_id: str) -> list[dict]:
    return envelope_data(cli.run("ds", "files", "list", task_id)).get("files", [])


def _try_list_files(cli: SynoCLI, task_id: str) -> Optional[list[dict]]:
    """Tolerant list for polling: a just-added torrent returns synology code
    1913 ("BT file list not ready") until its metadata is parsed. Treat any
    failure as "not ready yet" so wait_for keeps polling."""
    res = cli.run("ds", "files", "list", task_id, check=False)
    if res.exit_code != 0:
        return None
    data = (res.envelope or {}).get("data") or {}
    return data.get("files") or None


def test_ds_files_select_and_priority(
    cli: SynoCLI,
    cli_options,
    torrent_fixture,
    ds_task_tracker,
) -> None:
    """List a multi-file BT task's files, skip one, set priority, validate errors."""
    tid = _add_torrent(cli, str(torrent_fixture.torrent_path), cli_options.ds_destination)
    ds_task_tracker.register(tid)

    # Torrent metadata may take a moment to parse into the per-file list; until
    # then `ds files list` fails with synology code 1913.
    files = wait_for(
        lambda: _try_list_files(cli, tid),
        timeout=90.0,
        interval=3.0,
        description="BT file list to populate",
    )

    # Big Buck Bunny is a multi-file torrent (mp4 + subtitle + poster).
    assert len(files) >= 2, f"expected a multi-file torrent, got {files!r}"
    for f in files:
        for key in ("index", "name", "size", "wanted", "priority"):
            assert key in f, f"file entry missing {key!r}: {f!r}"
    assert all(f["wanted"] for f in files), f"new task should have all files wanted: {files!r}"

    # Skip the smallest file (cheap to flip; never the main payload).
    target = min(files, key=lambda f: f["size"])
    idx = target["index"]
    set_data = envelope_data(cli.run("ds", "files", "set", tid, "--skip", str(idx)))
    assert idx in set_data.get("skipped", []), f"skip not reflected: {set_data!r}"

    after = {f["index"]: f for f in _list_files(cli, tid)}
    assert after[idx]["wanted"] is False, f"index {idx} should be unwanted: {after[idx]!r}"
    for other_idx, f in after.items():
        if other_idx != idx:
            assert f["wanted"] is True, f"index {other_idx} should be unchanged: {f!r}"

    # Re-include everything.
    envelope_data(cli.run("ds", "files", "set", tid, "--all"))
    assert all(f["wanted"] for f in _list_files(cli, tid)), "all files should be wanted after --all"

    # Priority round-trip.
    envelope_data(cli.run("ds", "files", "priority", tid, "high", "--index", str(idx)))
    after_prio = {f["index"]: f for f in _list_files(cli, tid)}
    assert after_prio[idx]["priority"] == "high", f"priority not set to high: {after_prio[idx]!r}"

    # Out-of-range index is validated client-side -> exit 1, validation_error.
    cli.run_expect_failure(
        "ds", "files", "set", tid, "--skip", "99999",
        exit_code=1,
        code="validation_error",
    )

    # Conflicting selection flags are rejected before any network call.
    cli.run_expect_failure("ds", "files", "set", tid, "--all", "--none", exit_code=1)

    # Cleanup (tracker also sweeps on teardown as a safety net).
    envelope_data(cli.run("ds", "delete", tid))
    ds_task_tracker.mark_deleted(tid)


def test_ds_files_list_nonexistent_exit_3(cli: SynoCLI) -> None:
    """`ds files list` against an unknown task id exits 3 (task-not-found)."""
    result = cli.run_expect_failure(
        "ds", "files", "list", "dbid_99999999_nonexistent", code="synology_error"
    )
    assert result.exit_code == 3, f"unexpected exit code: {result.exit_code}"
    assert result.synology_code() in {404, 501}, (
        f"expected task-not-found synology code, got {result.synology_code()!r}"
    )
