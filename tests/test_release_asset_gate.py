import copy
import contextlib
import io
import json
import re
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

from validate_release_assets import (  # noqa: E402
    CHECKSUMS_ASSET,
    REQUIRED_ARCHIVES,
    main,
    validate_release,
)


FIXTURE_PATH = ROOT / "tests" / "fixtures" / "release_asset_gate_cases.json"
WORKFLOW_PATH = ROOT / ".github" / "workflows" / "release.yml"
OLD_PUBLISH_WORKFLOW_PATH = ROOT / ".github" / "workflows" / "publish-pypi.yml"
GORELEASER_PATH = ROOT / ".goreleaser.yaml"


class ReleaseAssetFixtureTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.fixture = json.loads(FIXTURE_PATH.read_text(encoding="utf-8"))

    def _case_inputs(self, case):
        release = copy.deepcopy(self.fixture["base_release"])
        checksums = self.fixture["base_checksums"]
        tag = case.get("tag", release["tag_name"])

        release.update(case.get("release_overrides", {}))

        removed_assets = set(case.get("remove_assets", []))
        release["assets"] = [
            asset
            for asset in release["assets"]
            if asset["name"] not in removed_assets
        ]
        release["assets"].extend(
            {"name": name} for name in case.get("add_assets", [])
        )
        for name in case.get("duplicate_assets", []):
            release["assets"].append({"name": name})
        if case.get("reverse_assets"):
            release["assets"].reverse()

        removed_checksums = set(case.get("remove_checksum_entries", []))
        checksum_lines = [
            line
            for line in checksums.splitlines()
            if line.split()[-1] not in removed_checksums
        ]
        for name in case.get("duplicate_checksum_entries", []):
            checksum_lines.append(
                next(line for line in checksum_lines if line.split()[-1] == name)
            )
        checksum_lines.extend(case.get("append_checksum_lines", []))
        checksums = "\n".join(checksum_lines)
        if checksum_lines:
            checksums += "\n"

        return release, checksums, tag

    def test_offline_fixture_cases(self):
        for case in self.fixture["cases"]:
            with self.subTest(case=case["name"]):
                release, checksums, tag = self._case_inputs(case)
                errors = validate_release(release, checksums, tag)
                if case.get("valid"):
                    self.assertEqual([], errors)
                else:
                    for expected in case["errors"]:
                        self.assertTrue(
                            any(expected in error for error in errors),
                            "expected {!r} in {!r}".format(expected, errors),
                        )

    def test_cli_accepts_complete_fixture(self):
        release = copy.deepcopy(self.fixture["base_release"])
        with tempfile.TemporaryDirectory() as temp_dir:
            release_path = Path(temp_dir) / "release.json"
            checksums_path = Path(temp_dir) / CHECKSUMS_ASSET
            release_path.write_text(json.dumps(release), encoding="utf-8")
            checksums_path.write_text(
                self.fixture["base_checksums"], encoding="utf-8"
            )
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                exit_code = main(
                    [
                        "--release-json",
                        str(release_path),
                        "--checksums",
                        str(checksums_path),
                        "--tag",
                        release["tag_name"],
                    ]
                )
        self.assertEqual(0, exit_code)
        self.assertIn("matching release assets", output.getvalue())

    def test_cli_rejects_invalid_release(self):
        release = copy.deepcopy(self.fixture["base_release"])
        release["draft"] = True
        with tempfile.TemporaryDirectory() as temp_dir:
            release_path = Path(temp_dir) / "release.json"
            checksums_path = Path(temp_dir) / CHECKSUMS_ASSET
            release_path.write_text(json.dumps(release), encoding="utf-8")
            checksums_path.write_text(
                self.fixture["base_checksums"], encoding="utf-8"
            )
            error_output = io.StringIO()
            with contextlib.redirect_stderr(error_output):
                exit_code = main(
                    [
                        "--release-json",
                        str(release_path),
                        "--checksums",
                        str(checksums_path),
                        "--tag",
                        release["tag_name"],
                    ]
                )
        self.assertEqual(1, exit_code)
        self.assertIn("non-draft", error_output.getvalue())

    def test_cli_reports_unreadable_inputs(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            missing_release = Path(temp_dir) / "missing-release.json"
            missing_checksums = Path(temp_dir) / "missing-checksums.txt"
            error_output = io.StringIO()
            with contextlib.redirect_stderr(error_output):
                exit_code = main(
                    [
                        "--release-json",
                        str(missing_release),
                        "--checksums",
                        str(missing_checksums),
                        "--tag",
                        "v1.2.3",
                    ]
                )
        self.assertEqual(2, exit_code)
        self.assertIn("could not read its inputs", error_output.getvalue())


class ReleaseWorkflowStructureTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflow = WORKFLOW_PATH.read_text(encoding="utf-8")

    def _job_block(self, name):
        match = re.search(
            r"(?ms)^  {}:\n(?P<body>.*?)(?=^  [A-Za-z0-9_-]+:\n|\Z)".format(
                re.escape(name)
            ),
            self.workflow,
        )
        self.assertIsNotNone(match, "missing workflow job: {}".format(name))
        return match.group("body")

    def test_single_tag_triggered_release_workflow(self):
        self.assertIn('      - "v*"', self.workflow)
        self.assertFalse(OLD_PUBLISH_WORKFLOW_PATH.exists())

    def test_permissions_are_job_scoped(self):
        self.assertIsNone(re.search(r"(?m)^permissions:", self.workflow))
        release = self._job_block("release")
        pypi = self._job_block("pypi")
        self.assertIn("    permissions:\n      contents: write", release)
        self.assertIn("    permissions:\n      contents: read", pypi)
        self.assertIn("      id-token: write", pypi)
        self.assertNotIn("contents: write", pypi)

    def test_publish_job_depends_on_release_and_gate(self):
        pypi = self._job_block("pypi")
        self.assertIn("    needs: release", pypi)
        gate_index = pypi.index("scripts/validate_release_assets.py")
        publish_index = pypi.index("pypa/gh-action-pypi-publish@release/v1")
        self.assertLess(gate_index, publish_index)
        self.assertIn("gh api", pypi)
        self.assertIn("gh release download", pypi)

    def test_validator_matches_goreleaser_asset_matrix(self):
        goreleaser = GORELEASER_PATH.read_text(encoding="utf-8")
        self.assertIn("project_name: backfill", goreleaser)
        self.assertIn("{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}", goreleaser)
        self.assertIn("name_template: checksums.txt", goreleaser)
        self.assertEqual(
            {
                "backfill_darwin_amd64.tar.gz",
                "backfill_darwin_arm64.tar.gz",
                "backfill_linux_amd64.tar.gz",
                "backfill_linux_arm64.tar.gz",
            },
            set(REQUIRED_ARCHIVES),
        )


if __name__ == "__main__":
    unittest.main()
