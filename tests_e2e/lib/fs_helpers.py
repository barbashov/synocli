from __future__ import annotations

from .cli import SynoCLI, envelope_data


# Synology codes returned when a stop arrives after the task already terminated.
RACE_OK_CODES = {401, 599}


def start_async(cli: SynoCLI, *args: str) -> str:
    """Run `synocli ... --async`, assert envelope ok, return task_id."""
    data = envelope_data(cli.run(*args, "--async"))
    tid = data.get("task_id") or data.get("taskid")
    assert tid, f"expected task_id in async response, got {data!r}"
    return tid


def assert_stop_or_finished(cli: SynoCLI, parent_cmd: str, task_id: str) -> None:
    """Port of run_stop_with_race_fallback in tests_e2e/filestation.sh.

    A stop racing the task to completion is not a failure: if `stop` errors
    because the task already terminated (Synology codes 401/599), treat that
    as success. Otherwise the status payload must show finished=true.
    """
    stop = cli.run("fs", parent_cmd, "stop", task_id, check=False)
    if stop.exit_code == 0:
        return
    status = cli.run("fs", parent_cmd, "status", task_id, check=False)
    if status.exit_code == 0:
        data = (status.envelope or {}).get("data") or {}
        if data.get("finished") is True:
            return
    synology_code = status.synology_code()
    if synology_code in RACE_OK_CODES:
        return
    raise AssertionError(
        f"fs {parent_cmd} stop {task_id} failed non-race; "
        f"stop_env={stop.envelope!r} status_env={status.envelope!r}"
    )
