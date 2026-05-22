from __future__ import annotations

import filecmp
from pathlib import Path

import pytest

from lib.cli import SynoCLI, envelope_data


pytestmark = pytest.mark.fs


class TestFSLifecycle:
    """Single ordered scenario covering the most common FS file operations.

    Each step uses pytest-subtests so a failure surfaces with a clear
    sub-context but does not abort the remaining steps. The remote tree is
    swept by the session-scoped sandbox_root teardown.
    """

    def test_lifecycle(
        self,
        cli: SynoCLI,
        sandbox_root: str,
        local_fs_fixture: dict,
        tmp_path: Path,
        subtests,
    ) -> None:
        base = f"{sandbox_root}/lifecycle"
        cli.run("fs", "mkdir", sandbox_root, "lifecycle", "--parents")

        a_local: Path = local_fs_fixture["a_txt"]

        with subtests.test(msg="shares"):
            data = envelope_data(cli.run("fs", "shares"))
            assert isinstance(data.get("shares") or data.get("files"), list)

        with subtests.test(msg="upload"):
            data = envelope_data(cli.run("fs", "upload", str(a_local), base))
            # UploadOne returns the synology File Station upload payload — at
            # minimum it must envelope ok.
            assert data, "empty upload data"

        with subtests.test(msg="rename"):
            envelope_data(cli.run("fs", "rename", f"{base}/a.txt", "a-renamed.txt"))

        with subtests.test(msg="get"):
            data = envelope_data(cli.run("fs", "get", f"{base}/a-renamed.txt"))
            files = data.get("files") or []
            assert files and files[0].get("name") == "a-renamed.txt"

        with subtests.test(msg="download-and-compare"):
            local_dl = tmp_path / "a-downloaded.txt"
            envelope_data(
                cli.run(
                    "fs",
                    "download",
                    f"{base}/a-renamed.txt",
                    "--output",
                    str(local_dl),
                )
            )
            assert filecmp.cmp(a_local, local_dl, shallow=False), (
                "downloaded file content does not match uploaded original"
            )

        with subtests.test(msg="cp"):
            envelope_data(
                cli.run(
                    "fs",
                    "cp",
                    f"{base}/a-renamed.txt",
                    "--to",
                    f"{base}/copy",
                )
            )

        with subtests.test(msg="mv"):
            envelope_data(
                cli.run(
                    "fs",
                    "mv",
                    f"{base}/copy/a-renamed.txt",
                    "--to",
                    f"{base}/moved",
                )
            )

        with subtests.test(msg="list-contains-moved"):
            data = envelope_data(cli.run("fs", "list", f"{base}/moved"))
            files = data.get("files") or []
            assert any(f.get("name") == "a-renamed.txt" for f in files), files

        with subtests.test(msg="md5-sync"):
            data = envelope_data(
                cli.run("fs", "md5", f"{base}/moved/a-renamed.txt")
            )
            status = data.get("status") or {}
            md5 = status.get("md5") or data.get("md5")
            assert md5, f"md5 not present in envelope.data: {data!r}"

        with subtests.test(msg="delete-leaf"):
            envelope_data(cli.run("fs", "delete", f"{base}/moved/a-renamed.txt"))
