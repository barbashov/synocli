from __future__ import annotations

import os
from pathlib import Path

import pytest

from lib.cli import SynoCLI, assert_envelope_ok, envelope_data


def test_auth_ping(cli: SynoCLI) -> None:
    result = cli.run("auth", "ping")
    data = envelope_data(result)
    assert data.get("status") == "ok"


def test_auth_whoami(cli: SynoCLI) -> None:
    result = cli.run("auth", "whoami")
    data = envelope_data(result)
    assert data.get("authenticated") is True
    assert isinstance(data.get("user"), str) and data["user"]


def test_auth_api_info_filestation(cli: SynoCLI) -> None:
    result = cli.run("auth", "api-info", "--prefix", "SYNO.FileStation")
    data = envelope_data(result)
    apis = data.get("apis") or {}
    assert isinstance(apis, dict) and len(apis) > 0
    # Every key must start with the requested prefix.
    for name in apis.keys():
        assert name.startswith("SYNO.FileStation"), name


def test_bad_credentials_returns_exit_2(
    cli: SynoCLI, tmp_path: Path
) -> None:
    """A wrong password should map to apperr code 'auth_failed' with exit 2."""
    bad_creds = tmp_path / "bad-creds"
    bad_creds.write_text(
        "user=__synocli_e2e_no_such_user__\npassword=__definitely_wrong__\n",
        encoding="utf-8",
    )
    os.chmod(bad_creds, 0o600)
    bad_cli = cli.with_credentials(bad_creds)
    result = bad_cli.run_expect_failure(
        "auth", "ping", exit_code=2, code="auth_failed"
    )
    env = result.envelope_or_raise()
    assert env.get("ok") is False
