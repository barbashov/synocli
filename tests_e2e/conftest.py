from __future__ import annotations

import json
import os
import re
import shutil
import sys
import time
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Iterator, Optional

import pytest

ROOT = Path(__file__).resolve().parent
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from lib.cli import SynoCLI, CLIError, assert_envelope_ok, envelope_data  # noqa: E402
from lib.webtorrent import TorrentFixture, ensure_fixture  # noqa: E402


DEFAULT_BIN = Path(__file__).resolve().parents[1] / "bin" / "synocli"
DEFAULT_BASE = "/volume1/synocli-e2e"
CACHE_DIR = ROOT / ".cache"
TASK_LOG_PATH = CACHE_DIR / "ds-task-ids.jsonl"
INVOCATION_LOG_PATH = CACHE_DIR / "invocations.log"


# ---------------------------------------------------------------------------
# Options
# ---------------------------------------------------------------------------


def pytest_addoption(parser: pytest.Parser) -> None:
    group = parser.getgroup("synocli-e2e")
    group.addoption("--endpoint", action="store", help="DSM endpoint, e.g. https://nas:5001")
    group.addoption(
        "--credentials-file",
        action="store",
        help="Credentials file with user=..., password=... entries",
    )
    group.addoption(
        "--bin",
        action="store",
        default=str(DEFAULT_BIN),
        help="Path to the synocli binary (default: ./bin/synocli)",
    )
    group.addoption(
        "--base",
        action="store",
        default=DEFAULT_BASE,
        help="Remote sandbox root (default: %(default)s)",
    )
    group.addoption("--insecure-tls", action="store_true", help="Pass --insecure-tls to synocli")
    group.addoption(
        "--refresh-torrent-cache",
        action="store_true",
        help="Re-fetch the webtorrent.io fixture even if cached",
    )
    group.addoption(
        "--allow-skip-on-unreachable",
        action="store_true",
        help="Skip rather than exit when the NAS is unreachable (dev loops)",
    )
    group.addoption(
        "--ds-destination",
        action="store",
        default=None,
        help="Optional Download Station destination folder (DSM default if unset)",
    )


# ---------------------------------------------------------------------------
# Session-scoped fixtures
# ---------------------------------------------------------------------------


@dataclass
class CLIOptions:
    bin_path: Path
    endpoint: str
    credentials_file: Optional[Path]
    insecure_tls: bool
    base: str
    refresh_torrent_cache: bool
    allow_skip_on_unreachable: bool
    ds_destination: Optional[str]


def _require_option(config: pytest.Config, name: str) -> str:
    value = config.getoption(name)
    if not value:
        pytest.exit(
            f"missing required option {name}; pass it via `pytest {name}=...` or the equivalent flag.",
            returncode=2,
        )
    return value


@pytest.fixture(scope="session")
def cli_options(pytestconfig: pytest.Config) -> CLIOptions:
    endpoint = _require_option(pytestconfig, "--endpoint")
    creds = _require_option(pytestconfig, "--credentials-file")
    bin_path = Path(pytestconfig.getoption("--bin")).expanduser().resolve()
    if not bin_path.exists():
        pytest.exit(
            f"synocli binary not found at {bin_path}. Run `make build` first.",
            returncode=2,
        )
    creds_path = Path(creds).expanduser()
    if not creds_path.exists():
        pytest.exit(f"credentials file not found: {creds_path}", returncode=2)
    return CLIOptions(
        bin_path=bin_path,
        endpoint=endpoint,
        credentials_file=creds_path,
        insecure_tls=bool(pytestconfig.getoption("--insecure-tls")),
        base=pytestconfig.getoption("--base"),
        refresh_torrent_cache=bool(pytestconfig.getoption("--refresh-torrent-cache")),
        allow_skip_on_unreachable=bool(pytestconfig.getoption("--allow-skip-on-unreachable")),
        ds_destination=pytestconfig.getoption("--ds-destination"),
    )


@pytest.fixture(scope="session")
def cli(cli_options: CLIOptions) -> SynoCLI:
    CACHE_DIR.mkdir(parents=True, exist_ok=True)
    return SynoCLI(
        bin_path=cli_options.bin_path,
        endpoint=cli_options.endpoint,
        credentials_file=cli_options.credentials_file,
        insecure_tls=cli_options.insecure_tls,
        invocation_log=INVOCATION_LOG_PATH,
    )


@pytest.fixture(scope="session", autouse=True)
def _nas_reachable(cli: SynoCLI, cli_options: CLIOptions) -> None:
    """Single auth ping at session start. Fails loudly so missing creds
    don't manifest as confusing per-test errors."""
    try:
        cli.run("auth", "ping", retry_with_debug=False, timeout=30)
    except CLIError as exc:
        msg = f"auth ping failed against {cli_options.endpoint}: {exc}"
        if cli_options.allow_skip_on_unreachable:
            pytest.skip(msg)
        else:
            pytest.exit(msg, returncode=2)


# ---------------------------------------------------------------------------
# Remote sandbox: one root per session, recursive delete on teardown.
# ---------------------------------------------------------------------------


def _sandbox_id() -> str:
    return f"{int(time.time())}-{uuid.uuid4().hex[:8]}"


@pytest.fixture(scope="session")
def sandbox_root(cli: SynoCLI, cli_options: CLIOptions) -> Iterator[str]:
    base = _resolve_sandbox_base(cli, cli_options.base)
    root = f"{base}/run-{_sandbox_id()}"
    leaf_name = root.rsplit("/", 1)[1]
    cli.run("fs", "mkdir", base, leaf_name, "--parents")
    yield root
    # Single recursive sweep handles every FS test artifact.
    try:
        cli.run("fs", "delete", root, "--recursive", retry_with_debug=False, check=False)
    except Exception:
        pass


def _resolve_sandbox_base(cli: SynoCLI, desired: str) -> str:
    """Pick a usable remote sandbox base.

    DSM File Station addresses files relative to share roots (e.g. `/Data`,
    `/docker`), not under `/volume1`. If `desired` is reachable we use it
    as-is; otherwise we fall back to creating `<first-share>/<leaf>` so the
    suite works on any NAS without per-host configuration.
    """
    desired = desired.rstrip("/")
    parent = desired.rsplit("/", 1)[0] or "/"
    leaf = desired.rsplit("/", 1)[1] if "/" in desired else desired or "synocli-e2e"

    mkdir = cli.run("fs", "mkdir", parent, leaf, "--parents", check=False)
    if mkdir.exit_code == 0:
        return desired

    shares_data = envelope_data(cli.run("fs", "shares"))
    shares = shares_data.get("shares") or []
    if not shares:
        raise AssertionError(
            f"cannot create {desired!r} and the NAS reports no shares "
            f"to fall back to. mkdir error: {mkdir.envelope!r}"
        )
    share_path = shares[0].get("path")
    assert share_path, f"share entry missing 'path' field: {shares[0]!r}"
    cli.run("fs", "mkdir", share_path, leaf, "--parents")
    return f"{share_path}/{leaf}"


def _sanitize_node_id(node_id: str) -> str:
    """Compact directory name derived from a pytest node id.

    Keep just the leaf test name (post-`::`) and strip module/file path
    prefixes — DSM has tight path length limits and the full node id can
    push the absolute path past them for deeply-nested archive operations.
    """
    leaf = node_id.split("::")[-1]
    return re.sub(r"[^A-Za-z0-9._-]+", "_", leaf)[:60] or "test"


@pytest.fixture
def fs_sandbox(cli: SynoCLI, sandbox_root: str, request: pytest.FixtureRequest) -> str:
    node = _sanitize_node_id(request.node.nodeid)
    path = f"{sandbox_root}/{node}"
    cli.run("fs", "mkdir", sandbox_root, node, "--parents")
    return path


# ---------------------------------------------------------------------------
# Local fixtures (small file tree for upload/compress/extract tests).
# ---------------------------------------------------------------------------


@pytest.fixture(scope="session")
def local_fs_fixture(tmp_path_factory: pytest.TempPathFactory) -> dict:
    root = tmp_path_factory.mktemp("synocli-fs-fixture")
    a = root / "a.txt"
    a.write_text("hello synocli\n", encoding="utf-8")
    nested = root / "tree" / "sub"
    nested.mkdir(parents=True)
    (nested / "file.txt").write_text("nested file\n", encoding="utf-8")
    return {"root": root, "a_txt": a, "nested_file": nested / "file.txt"}


# ---------------------------------------------------------------------------
# DS task tracking: durable JSON log so a Ctrl-C / kill -9 mid-run can be
# swept on the next invocation.
# ---------------------------------------------------------------------------


class DSTaskTracker:
    def __init__(self, log_path: Path, cli: SynoCLI) -> None:
        self.log_path = log_path
        self.cli = cli
        self.alive: set[str] = set()
        self.log_path.parent.mkdir(parents=True, exist_ok=True)

    def register(self, task_id: str) -> None:
        if not task_id:
            return
        self.alive.add(task_id)
        line = json.dumps({"task_id": task_id, "ts": time.time()})
        tmp = self.log_path.with_suffix(self.log_path.suffix + ".tmp")
        with self.log_path.open("a", encoding="utf-8") as fh:
            fh.write(line + "\n")
            fh.flush()
            os.fsync(fh.fileno())
        if tmp.exists():
            tmp.unlink()

    def mark_deleted(self, task_id: str) -> None:
        self.alive.discard(task_id)

    def _all_logged_ids(self) -> list[str]:
        if not self.log_path.exists():
            return []
        ids: list[str] = []
        with self.log_path.open("r", encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if not line:
                    continue
                try:
                    obj = json.loads(line)
                except json.JSONDecodeError:
                    continue
                tid = obj.get("task_id")
                if tid:
                    ids.append(tid)
        return ids

    def sweep(self) -> None:
        ids = list(set(self._all_logged_ids()) | self.alive)
        for tid in ids:
            try:
                self.cli.run("ds", "delete", tid, retry_with_debug=False, check=False)
            except Exception:
                pass
        if self.log_path.exists():
            try:
                self.log_path.unlink()
            except OSError:
                pass
        self.alive.clear()


@pytest.fixture(scope="session")
def ds_task_tracker(cli: SynoCLI) -> Iterator[DSTaskTracker]:
    tracker = DSTaskTracker(TASK_LOG_PATH, cli)
    # Pre-sweep stale IDs left by an interrupted prior run.
    tracker.sweep()
    yield tracker
    tracker.sweep()


# ---------------------------------------------------------------------------
# DS config snapshots — restore even if a test crashes mid-set.
# ---------------------------------------------------------------------------


@pytest.fixture
def bandwidth_snapshot(cli: SynoCLI) -> Iterator[dict]:
    result = cli.run("ds", "bandwidth", "get")
    snap = envelope_data(result)
    yield snap
    restore_args = ["ds", "bandwidth", "set"]
    if "bt_max_download" in snap:
        restore_args.extend(["--bt-max-download", str(snap["bt_max_download"])])
    if "bt_max_upload" in snap:
        restore_args.extend(["--bt-max-upload", str(snap["bt_max_upload"])])
    try:
        cli.run(*restore_args, retry_with_debug=False, check=False)
    except Exception:
        pass


@pytest.fixture
def max_tasks_snapshot(cli: SynoCLI) -> Iterator[dict]:
    result = cli.run("ds", "max-tasks", "get")
    snap = envelope_data(result)
    yield snap
    if "max_tasks" in snap:
        try:
            cli.run(
                "ds",
                "max-tasks",
                "set",
                str(snap["max_tasks"]),
                retry_with_debug=False,
                check=False,
            )
        except Exception:
            pass


# ---------------------------------------------------------------------------
# WebTorrent fixture cache.
# ---------------------------------------------------------------------------


@pytest.fixture(scope="session")
def torrent_fixture(cli_options: CLIOptions) -> TorrentFixture:
    cache = CACHE_DIR / "webtorrent"
    return ensure_fixture(cache, refresh=cli_options.refresh_torrent_cache)
