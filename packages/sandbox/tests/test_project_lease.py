import json

import pytest

from app.services.project_lease import StaleProjectEpoch, check_epoch, read_epoch


def test_stale_epoch_is_rejected(tmp_path):
    project = tmp_path / "p1"
    (project / ".vibebiz").mkdir(parents=True)
    (project / ".vibebiz" / "lease").write_text(json.dumps({"epoch": 5}), encoding="utf-8")
    with pytest.raises(StaleProjectEpoch):
        check_epoch(project, 4, "sandbox-a")


def test_new_epoch_updates_lease_file(tmp_path):
    project = tmp_path / "p1"
    check_epoch(project, 7, "sandbox-a")
    payload = json.loads((project / ".vibebiz" / "lease").read_text(encoding="utf-8"))
    assert payload["epoch"] == 7
    assert payload["sandbox"] == "sandbox-a"


def test_missing_epoch_is_tolerated(tmp_path):
    project = tmp_path / "p1"
    check_epoch(project, 0, "sandbox-a")
    assert read_epoch(project) == 0
    assert not (project / ".vibebiz" / "lease").exists()


def test_same_epoch_does_not_rewrite_lease(tmp_path):
    project = tmp_path / "p1"
    (project / ".vibebiz").mkdir(parents=True)
    path = project / ".vibebiz" / "lease"
    path.write_text(json.dumps({"epoch": 5, "sandbox": "sandbox-a"}), encoding="utf-8")
    check_epoch(project, 5, "sandbox-b")
    assert json.loads(path.read_text(encoding="utf-8"))["sandbox"] == "sandbox-a"


def test_corrupt_lease_reads_as_zero(tmp_path):
    project = tmp_path / "p1"
    (project / ".vibebiz").mkdir(parents=True)
    (project / ".vibebiz" / "lease").write_text("not-json", encoding="utf-8")
    assert read_epoch(project) == 0


@pytest.mark.parametrize("payload", ["null", "[]", '"epoch"', "12"])
def test_non_object_lease_reads_as_zero(tmp_path, payload):
    project = tmp_path / "p1"
    (project / ".vibebiz").mkdir(parents=True)
    (project / ".vibebiz" / "lease").write_text(payload, encoding="utf-8")
    assert read_epoch(project) == 0
