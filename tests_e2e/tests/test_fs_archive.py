from __future__ import annotations

import pytest

from lib.cli import SynoCLI, envelope_data
from lib.fs_helpers import assert_stop_or_finished, start_async


pytestmark = pytest.mark.fs


def _is_dir(file_entry: dict) -> bool:
    val = file_entry.get("isdir")
    if isinstance(val, bool):
        return val
    if isinstance(val, str):
        return val.lower() == "true"
    return False


def test_compress_extract_roundtrip_sync(
    cli: SynoCLI, fs_sandbox: str, local_fs_fixture: dict
) -> None:
    cli.run("fs", "upload", str(local_fs_fixture["a_txt"]), fs_sandbox)
    archive = f"{fs_sandbox}/archive.zip"
    # DSM's extract API requires the destination folder to exist — it does
    # not auto-create it the way mkdir --parents would.
    cli.run("fs", "mkdir", fs_sandbox, "extracted", "--parents")
    extracted = f"{fs_sandbox}/extracted"

    envelope_data(cli.run("fs", "compress", fs_sandbox, "--to", archive))
    data = envelope_data(cli.run("fs", "get", archive))
    files = data.get("files") or []
    assert files and not _is_dir(files[0]), f"archive not a file: {files!r}"

    envelope_data(cli.run("fs", "extract", archive, "--to", extracted))
    extracted_listing = envelope_data(cli.run("fs", "list", extracted))
    assert isinstance(extracted_listing.get("files"), list)


def test_compress_async(cli: SynoCLI, fs_sandbox: str, local_fs_fixture: dict) -> None:
    cli.run("fs", "upload", str(local_fs_fixture["a_txt"]), fs_sandbox)
    archive = f"{fs_sandbox}/archive-async.zip"
    tid = start_async(cli, "fs", "compress", fs_sandbox, "--to", archive)
    envelope_data(cli.run("fs", "compress", "status", tid))
    assert_stop_or_finished(cli, "compress", tid)


def test_extract_async(cli: SynoCLI, fs_sandbox: str, local_fs_fixture: dict) -> None:
    cli.run("fs", "upload", str(local_fs_fixture["a_txt"]), fs_sandbox)
    archive = f"{fs_sandbox}/archive.zip"
    cli.run("fs", "mkdir", fs_sandbox, "extracted-async", "--parents")
    extracted = f"{fs_sandbox}/extracted-async"
    envelope_data(cli.run("fs", "compress", fs_sandbox, "--to", archive))
    tid = start_async(cli, "fs", "extract", archive, "--to", extracted)
    envelope_data(cli.run("fs", "extract", "status", tid))
    assert_stop_or_finished(cli, "extract", tid)
