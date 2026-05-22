from __future__ import annotations

import pytest

from lib.cli import SynoCLI, envelope_data
from lib.fs_helpers import RACE_OK_CODES, assert_stop_or_finished, start_async


pytestmark = pytest.mark.fs


def test_search_async(cli: SynoCLI, fs_sandbox: str, local_fs_fixture: dict) -> None:
    cli.run("fs", "upload", str(local_fs_fixture["a_txt"]), fs_sandbox)
    tid = start_async(cli, "fs", "search", fs_sandbox, "--pattern", "a.txt")
    # search uses 'results' instead of 'status' and has its own 'stop' subcommand.
    results = envelope_data(cli.run("fs", "search", "results", tid))
    assert "files" in results
    stop = cli.run("fs", "search", "stop", tid, check=False)
    if stop.exit_code != 0 and stop.synology_code() not in RACE_OK_CODES:
        envelope_data(cli.run("fs", "search", "results", tid))
    envelope_data(cli.run("fs", "search", "clear", tid))


def test_dir_size_async(cli: SynoCLI, fs_sandbox: str, local_fs_fixture: dict) -> None:
    cli.run("fs", "upload", str(local_fs_fixture["a_txt"]), fs_sandbox)
    tid = start_async(cli, "fs", "dir-size", fs_sandbox)
    envelope_data(cli.run("fs", "dir-size", "status", tid))
    assert_stop_or_finished(cli, "dir-size", tid)


def test_md5_async(cli: SynoCLI, fs_sandbox: str, local_fs_fixture: dict) -> None:
    cli.run("fs", "upload", str(local_fs_fixture["a_txt"]), fs_sandbox)
    remote = f"{fs_sandbox}/a.txt"
    tid = start_async(cli, "fs", "md5", remote)
    envelope_data(cli.run("fs", "md5", "status", tid))
    assert_stop_or_finished(cli, "md5", tid)
