from __future__ import annotations

import pytest

from lib.cli import SynoCLI, envelope_data


pytestmark = pytest.mark.ds


def test_max_tasks_get(cli: SynoCLI) -> None:
    data = envelope_data(cli.run("ds", "max-tasks", "get"))
    assert isinstance(data.get("max_tasks"), int) and data["max_tasks"] > 0
    assert isinstance(data.get("max_tasks_limit"), int) and data["max_tasks_limit"] >= data["max_tasks"]


def test_max_tasks_set_then_restore(cli: SynoCLI, max_tasks_snapshot) -> None:
    current = max_tasks_snapshot.get("max_tasks")
    limit = max_tasks_snapshot.get("max_tasks_limit")
    assert isinstance(current, int) and isinstance(limit, int)

    # Pick a value that's definitely in-range and different from current.
    target = 1 if current != 1 else min(2, limit)
    if target > limit:
        pytest.skip(f"DSM limit {limit} too low to test setting max_tasks")

    set_data = envelope_data(cli.run("ds", "max-tasks", "set", str(target)))
    assert set_data.get("max_tasks") == target

    after = envelope_data(cli.run("ds", "max-tasks", "get"))
    assert after.get("max_tasks") == target


def test_max_tasks_set_above_limit_rejected(cli: SynoCLI) -> None:
    # Above-limit validation happens *inside* withSession (after fetching
    # cfg.MaxTasksLimit), so it goes through outputError and emits an envelope.
    data = envelope_data(cli.run("ds", "max-tasks", "get"))
    limit = data["max_tasks_limit"]
    bad = limit + 1
    result = cli.run_expect_failure(
        "ds", "max-tasks", "set", str(bad), exit_code=1, code="validation_error"
    )
    env = result.envelope_or_raise()
    assert env.get("ok") is False


def test_max_tasks_set_zero_rejected(cli: SynoCLI) -> None:
    # Fails before withSession — no envelope; assert on exit code + stderr.
    result = cli.run_expect_failure("ds", "max-tasks", "set", "0", exit_code=1)
    assert ">= 1" in result.stderr, f"unexpected stderr: {result.stderr!r}"


def test_max_tasks_set_non_integer_rejected(cli: SynoCLI) -> None:
    result = cli.run_expect_failure("ds", "max-tasks", "set", "abc", exit_code=1)
    assert "integer" in result.stderr.lower(), f"unexpected stderr: {result.stderr!r}"
