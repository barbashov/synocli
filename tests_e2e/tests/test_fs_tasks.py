from __future__ import annotations

import pytest

from lib.cli import SynoCLI, envelope_data


pytestmark = pytest.mark.fs


def test_fs_tasks_list_envelope(cli: SynoCLI) -> None:
    data = envelope_data(cli.run("fs", "tasks"))
    # DSM's background-task list returns a `tasks` array (possibly empty).
    assert "tasks" in data and isinstance(data["tasks"], list)


def test_fs_tasks_clear(cli: SynoCLI) -> None:
    data = envelope_data(cli.run("fs", "tasks-clear"))
    assert data.get("cleared") is True
