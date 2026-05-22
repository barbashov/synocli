from __future__ import annotations

import hashlib
import html
import re
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Optional


WEBTORRENT_FREE_TORRENTS_URL = "https://webtorrent.io/free-torrents"
DEFAULT_FIXTURE_NAME = "Big Buck Bunny"


@dataclass
class TorrentFixture:
    torrent_path: Path
    magnet_uri: str
    source_url: str


class WebTorrentParseError(RuntimeError):
    pass


def ensure_fixture(
    cache_dir: Path,
    name: str = DEFAULT_FIXTURE_NAME,
    refresh: bool = False,
    timeout: float = 30.0,
) -> TorrentFixture:
    """Return a cached WebTorrent fixture, fetching from webtorrent.io if needed.

    Cache keying is by sha256(source_url) so renames on the upstream page
    don't silently return stale content for a different torrent.
    """
    cache_dir.mkdir(parents=True, exist_ok=True)
    key = hashlib.sha256(WEBTORRENT_FREE_TORRENTS_URL.encode()).hexdigest()
    torrent_path = cache_dir / f"{key}.torrent"
    magnet_path = cache_dir / f"{key}.magnet"
    source_path = cache_dir / f"{key}.source"

    if (
        not refresh
        and torrent_path.exists()
        and magnet_path.exists()
        and source_path.exists()
    ):
        return TorrentFixture(
            torrent_path=torrent_path,
            magnet_uri=magnet_path.read_text(encoding="utf-8").strip(),
            source_url=source_path.read_text(encoding="utf-8").strip(),
        )

    html_text = _fetch(WEBTORRENT_FREE_TORRENTS_URL, timeout=timeout)
    try:
        torrent_url, magnet_uri = _parse_links(html_text, name)
    except WebTorrentParseError as exc:
        debug_path = cache_dir / "last-fetch.html"
        debug_path.write_text(html_text, encoding="utf-8")
        raise WebTorrentParseError(
            f"{exc}; saved fetched HTML to {debug_path} for inspection"
        ) from None

    torrent_bytes = _fetch_bytes(torrent_url, timeout=timeout)
    torrent_path.write_bytes(torrent_bytes)
    magnet_path.write_text(magnet_uri, encoding="utf-8")
    source_path.write_text(torrent_url, encoding="utf-8")
    return TorrentFixture(
        torrent_path=torrent_path,
        magnet_uri=magnet_uri,
        source_url=torrent_url,
    )


def _fetch(url: str, timeout: float) -> str:
    return _fetch_bytes(url, timeout=timeout).decode("utf-8", errors="replace")


def _fetch_bytes(url: str, timeout: float) -> bytes:
    req = urllib.request.Request(url, headers={"User-Agent": "synocli-e2e/1.0"})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.read()
    except urllib.error.URLError as exc:
        raise RuntimeError(f"failed to fetch {url}: {exc}") from exc


def _parse_links(html_text: str, name: str) -> tuple[str, str]:
    pattern = re.compile(
        r"<li><p>"
        + re.escape(name)
        + r' <a href="([^"]+\.torrent)">\(torrent file\)</a> '
        + r'<a href="(magnet:[^"]+)">\(magnet link\)</a>',
        re.IGNORECASE,
    )
    m = pattern.search(html_text)
    if not m:
        raise WebTorrentParseError(
            f"could not find {name!r} torrent/magnet links in webtorrent.io page"
        )
    return html.unescape(m.group(1)), html.unescape(m.group(2))
