from __future__ import annotations

import json
import subprocess
import time
from pathlib import Path
from typing import Optional

import pytest

from lib.cli import SynoCLI, assert_envelope_ok, envelope_data
from lib.waits import wait_for


pytestmark = pytest.mark.ds


HTTPS_FIXTURE = "https://raw.githubusercontent.com/barbashov/synocli/main/README.md"
PAUSE_CANDIDATE_STATUSES = {"waiting", "downloading", "seeding", "finishing"}
# Synology codes meaning "task is in a terminal/missing state" after delete.
DELETED_OK_CODES = {401, 402, 404}


def _add_one(cli: SynoCLI, source: str, destination: Optional[str]) -> str:
    args = ["ds", "add", source]
    if destination:
        args.extend(["--destination", destination])
    data = envelope_data(cli.run(*args))
    ids = data.get("task_ids") or []
    assert ids and ids[0], f"empty task_ids in ds add response: {data!r}"
    return ids[0]


def test_ds_list_envelope(cli: SynoCLI) -> None:
    data = envelope_data(cli.run("ds", "list"))
    assert "tasks" in data and isinstance(data["tasks"], list)


def test_full_lifecycle(
    cli: SynoCLI,
    cli_options,
    torrent_fixture,
    ds_task_tracker,
) -> None:
    """Add via URL / magnet / torrent file, list, get, pause, wait, resume, watch, delete."""
    dest = cli_options.ds_destination

    url_tid = _add_one(cli, HTTPS_FIXTURE, dest)
    ds_task_tracker.register(url_tid)
    magnet_tid = _add_one(cli, torrent_fixture.magnet_uri, dest)
    ds_task_tracker.register(magnet_tid)
    torrent_tid = _add_one(cli, str(torrent_fixture.torrent_path), dest)
    ds_task_tracker.register(torrent_tid)

    # ds get should return the task envelope for each id.
    for tid in (url_tid, magnet_tid, torrent_tid):
        envelope_data(cli.run("ds", "get", tid))

    # ds list should include all three.
    listing = envelope_data(cli.run("ds", "list"))
    ids_in_list = {t.get("task_id") for t in listing.get("tasks", []) if isinstance(t, dict)}
    assert {url_tid, magnet_tid, torrent_tid}.issubset(ids_in_list), (
        f"created task IDs missing from list: have {ids_in_list!r}"
    )

    paused_tid = _pause_any_candidate(
        cli, [magnet_tid, torrent_tid, url_tid], attempts=3
    )

    # ds wait on a paused task should hit max-wait timeout (exit 5).
    timeout_result = cli.run_expect_failure(
        "ds",
        "wait",
        paused_tid,
        "--interval",
        "1s",
        "--max-wait",
        "3s",
        exit_code=5,
        code="timeout",
    )
    assert timeout_result.envelope is not None
    assert timeout_result.envelope.get("error", {}).get("code") in {"timeout", "task_failed"}

    # DSM can transition a paused task (e.g. a torrent with no peers) to
    # 'error' during the wait window, which would make `ds resume` return
    # synology code 405. Re-establish the paused state before resuming so
    # the assertion is not racing the task lifecycle.
    _ensure_paused(cli, paused_tid)

    # Resume should succeed.
    envelope_data(cli.run("ds", "resume", paused_tid))

    # ds list --watch produces one JSON snapshot line per tick. Capture the
    # first line and assert it's a snapshot envelope.
    snapshot = _capture_first_watch_snapshot(cli, paused_tid)
    data = snapshot.get("data") or {}
    assert data.get("event") == "snapshot"
    assert isinstance(data.get("tasks"), list)

    # Delete each task (idempotent: only delete each id once).
    for tid in {url_tid, magnet_tid, torrent_tid}:
        envelope_data(cli.run("ds", "delete", tid))
        ds_task_tracker.mark_deleted(tid)

    # After delete, ds get should fail with a synology error code matching the
    # known "deleted/missing" codes.
    for tid in {url_tid, magnet_tid, torrent_tid}:
        _assert_task_deleted(cli, tid)


def test_get_nonexistent_returns_exit_3(cli: SynoCLI) -> None:
    """`ds get` against an unknown id must exit 3 with a synology_error envelope.

    DS v1 returns synology code 404 and DS2 returns 501 for the same
    "task not found" condition; both are now mapped to exit code 3.
    """
    result = cli.run_expect_failure(
        "ds", "get", "dbid_99999999_nonexistent", code="synology_error"
    )
    code = result.synology_code()
    assert code in {404, 501}, (
        f"expected task-not-found synology code (404 v1, 501 v2), got {code!r}"
    )
    assert result.exit_code == 3, f"unexpected exit code: {result.exit_code}"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _pause_any_candidate(cli: SynoCLI, candidates: list[str], attempts: int) -> str:
    for _attempt in range(attempts):
        for tid in candidates:
            get_res = cli.run("ds", "get", tid, check=False)
            if get_res.exit_code != 0:
                continue
            data = (get_res.envelope or {}).get("data") or {}
            if data.get("normalized_status") not in PAUSE_CANDIDATE_STATUSES:
                continue
            pause = cli.run("ds", "pause", tid, check=False)
            if pause.exit_code != 0:
                # synology code 405 = action invalid in current state — try the
                # next candidate / attempt; anything else is a real error.
                if pause.synology_code() != 405:
                    raise AssertionError(
                        f"ds pause {tid} failed unexpectedly: {pause.envelope!r}"
                    )
                continue
            # DSM occasionally accepts a pause API call but the task settles
            # into a non-paused terminal state (e.g. 'error' for a torrent
            # with no peers) instead of 'paused'. Confirm the post-pause
            # state before returning; otherwise the subsequent `ds resume`
            # would race with the task lifecycle and hit synology code 405.
            if _wait_for_status(cli, tid, "paused", timeout=5.0):
                return tid
        time.sleep(2)
    raise AssertionError(
        f"could not pause any of {candidates!r} after {attempts} attempts"
    )


def _current_status(cli: SynoCLI, tid: str) -> Optional[str]:
    res = cli.run("ds", "get", tid, check=False)
    if res.exit_code != 0:
        return None
    data = (res.envelope or {}).get("data") or {}
    status = data.get("normalized_status")
    return status if isinstance(status, str) else None


def _wait_for_status(
    cli: SynoCLI, tid: str, want: str, timeout: float, interval: float = 0.5
) -> bool:
    deadline = time.monotonic() + timeout
    while True:
        if _current_status(cli, tid) == want:
            return True
        if time.monotonic() >= deadline:
            return False
        time.sleep(interval)


def _ensure_paused(cli: SynoCLI, tid: str) -> None:
    """Make best-effort attempts to leave the task in 'paused' state.

    If the task drifted out of 'paused' during a prior wait, retry pause
    once. Raises if we cannot get it back into 'paused' state, because the
    test specifically wants to assert that `ds resume` works on a paused
    task.
    """
    if _wait_for_status(cli, tid, "paused", timeout=1.0):
        return
    cli.run("ds", "pause", tid, check=False)
    if _wait_for_status(cli, tid, "paused", timeout=5.0):
        return
    raise AssertionError(
        f"task {tid} could not be returned to 'paused' (current={_current_status(cli, tid)!r})"
    )


def _capture_first_watch_snapshot(cli: SynoCLI, task_id: str, timeout: float = 20.0) -> dict:
    """Run `ds list --watch` and capture the first JSON line, then terminate."""
    cmd = cli.build_cmd(
        "ds", "list", "--watch", "--json", "--interval", "1s", "--id", task_id
    )
    proc = subprocess.Popen(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    try:
        deadline = time.monotonic() + timeout
        line = ""
        assert proc.stdout is not None
        while time.monotonic() < deadline:
            line = proc.stdout.readline()
            if line:
                break
        if not line:
            stderr = proc.stderr.read() if proc.stderr else ""
            raise AssertionError(
                f"ds watch produced no snapshot line within {timeout}s; stderr={stderr!r}"
            )
        return json.loads(line)
    finally:
        if proc.poll() is None:
            proc.terminate()
            try:
                proc.wait(timeout=3)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=3)


def _assert_task_deleted(cli: SynoCLI, task_id: str, attempts: int = 10) -> None:
    for _ in range(attempts):
        result = cli.run("ds", "get", task_id, check=False)
        if result.exit_code != 0:
            env = result.envelope or {}
            err = env.get("error") or {}
            if err.get("code") == "synology_error":
                code = (err.get("details") or {}).get("synology_code")
                if code in DELETED_OK_CODES:
                    return
            # Any other failure mode is a regression.
            raise AssertionError(
                f"unexpected error for deleted task {task_id}: {env!r}"
            )
        time.sleep(1)
    raise AssertionError(f"task {task_id} still exists after delete polling")
