from __future__ import annotations

import pytest

from lib.cli import SynoCLI, envelope_data


def test_info_overview(cli: SynoCLI) -> None:
    result = cli.run("info")
    data = envelope_data(result)
    # DSM + system always present; storage may be missing for non-storage-mgr accounts.
    dsm = data.get("dsm")
    sys_block = data.get("system")
    assert isinstance(dsm, dict) and dsm, "info.dsm missing"
    assert isinstance(sys_block, dict) and sys_block, "info.system missing"
    assert dsm.get("model"), "info.dsm.model empty"
    # storage may be absent OR present alongside a storage_error per fs_info.go.
    if "storage" in data:
        assert isinstance(data["storage"], dict)


def test_info_utilization(cli: SynoCLI) -> None:
    result = cli.run("info", "utilization")
    data = envelope_data(result)
    util = data.get("utilization")
    assert isinstance(util, dict) and util, "info.utilization missing"
    cpu = util.get("cpu") or {}
    mem = util.get("memory") or {}
    disk = util.get("disk") or {}
    # CPU loads are integers (centi-load values).
    for key in ("user_load", "system_load", "other_load"):
        assert key in cpu, f"cpu.{key} missing: {cpu!r}"
        assert isinstance(cpu[key], (int, float))
    # Memory percentages are integers.
    assert "real_usage" in mem and isinstance(mem["real_usage"], (int, float))
    # Per utilization.go the inner field carrying the per-drive list is "disk",
    # not "disks" — see internal/synology/system/utilization.go DiskUtilizationSet.
    assert "disk" in disk and isinstance(disk["disk"], list)
    assert "network" in util and isinstance(util["network"], list)


def test_info_disks(cli: SynoCLI) -> None:
    result = cli.run("info", "disks")
    data = envelope_data(result)
    storage = data.get("storage") or {}
    disks = storage.get("disks") or []
    assert isinstance(disks, list) and len(disks) > 0, "no disks reported"
    # Each disk should at least carry an id/name + model.
    for d in disks:
        assert isinstance(d, dict)
        assert d.get("id"), f"disk missing id: {d!r}"
        # Model can be empty on virtualized DSM, but the field should be present.
        assert "model" in d
    # Per-disk health map keyed by disk id.
    health = data.get("health") or {}
    assert isinstance(health, dict)
    for d in disks:
        assert d["id"] in health, f"health missing for {d['id']!r}"


def test_info_disks_smart_default(cli: SynoCLI) -> None:
    result = cli.run("info", "disks", "smart")
    data = envelope_data(result)
    health = data.get("health") or {}
    assert isinstance(health, dict) and health, "no SMART entries"
    # Find at least one disk with a non-empty SMART attribute table; some
    # virtualized disks legitimately have none, so don't require every disk
    # to have one — just at least one.
    any_attrs = False
    for entry in health.values():
        attrs = (entry or {}).get("smart_info") or []
        if isinstance(attrs, list) and len(attrs) > 0:
            any_attrs = True
            break
    assert any_attrs, "no SMART attributes anywhere in info disks smart payload"


def test_info_disks_smart_filter(cli: SynoCLI) -> None:
    overview = envelope_data(cli.run("info", "disks"))
    storage = overview.get("storage") or {}
    disks = storage.get("disks") or []
    if not disks:
        pytest.skip("no disks reported by info disks")
    disk_id = disks[0]["id"]
    result = cli.run("info", "disks", "smart", "--disk", disk_id)
    data = envelope_data(result)
    health = data.get("health") or {}
    assert list(health.keys()) == [disk_id], f"filter leaked: {list(health.keys())!r}"
