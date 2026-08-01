from __future__ import annotations

import pytest

from lib.cli import SynoCLI, envelope_data


pytestmark = pytest.mark.ds


def test_bandwidth_get(cli: SynoCLI) -> None:
    data = envelope_data(cli.run("ds", "bandwidth", "get"))
    assert "bt_max_download" in data and isinstance(data["bt_max_download"], int)
    assert "bt_max_upload" in data and isinstance(data["bt_max_upload"], int)


def test_bandwidth_set_then_restore(cli: SynoCLI, bandwidth_snapshot) -> None:
    # Pick a known non-zero value distinct from current to confirm a real round trip.
    new_down = (bandwidth_snapshot.get("bt_max_download") or 0) + 256
    new_up = (bandwidth_snapshot.get("bt_max_upload") or 0) + 128

    set_data = envelope_data(
        cli.run(
            "ds",
            "bandwidth",
            "set",
            "--bt-max-download",
            str(new_down),
            "--bt-max-upload",
            str(new_up),
        )
    )
    assert set_data.get("bt_max_download") == new_down
    assert set_data.get("bt_max_upload") == new_up

    after = envelope_data(cli.run("ds", "bandwidth", "get"))
    assert after.get("bt_max_download") == new_down
    assert after.get("bt_max_upload") == new_up
    # Restore happens in the bandwidth_snapshot fixture teardown.


def test_bandwidth_set_rejects_no_flag(cli: SynoCLI) -> None:
    # Early validation (before withSession) still emits the JSON envelope.
    result = cli.run_expect_failure(
        "ds", "bandwidth", "set", exit_code=1, code="validation_error"
    )
    env = result.envelope_or_raise()
    message = env["error"]["message"]
    assert "bt-max-download" in message or "bt-max-upload" in message, (
        f"unexpected error for missing-flag case: {env['error']!r}"
    )


def test_bandwidth_set_rejects_negative(cli: SynoCLI) -> None:
    result = cli.run_expect_failure(
        "ds", "bandwidth", "set", "--bt-max-download", "-1",
        exit_code=1, code="validation_error",
    )
    env = result.envelope_or_raise()
    assert ">= 0" in env["error"]["message"], (
        f"unexpected error for negative-value case: {env['error']!r}"
    )
