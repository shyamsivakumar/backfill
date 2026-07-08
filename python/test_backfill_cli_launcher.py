import contextlib
import hashlib
import io
import os
import stat
import sys
import tarfile
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import backfill_cli as launcher


class FakeResponse(io.BytesIO):
    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        self.close()
        return False


def tar_bytes(entries):
    data = io.BytesIO()
    with tarfile.open(fileobj=data, mode="w:gz") as tar:
        for name, body, mode in entries:
            info = tarfile.TarInfo(name)
            if body is None:
                info.type = tarfile.DIRTYPE
                info.mode = mode
                tar.addfile(info)
                continue
            payload = body.encode("utf-8")
            info.size = len(payload)
            info.mode = mode
            tar.addfile(info, io.BytesIO(payload))
    return data.getvalue()


@contextlib.contextmanager
def patched_urlopen(payload):
    calls = []

    def fake_urlopen(url, timeout):
        calls.append((url, timeout))
        return FakeResponse(payload)

    with mock.patch.object(launcher.urllib.request, "urlopen", side_effect=fake_urlopen):
        yield calls


class LauncherChecksumTests(unittest.TestCase):
    def test_read_expected_sha256_matches_prefixed_and_star_archive_rows(self):
        archive = "backfill_linux_amd64.tar.gz"
        expected = "a" * 64
        checksums = "\n".join(
            [
                f"{'b' * 64}  backfill_darwin_amd64.tar.gz",
                f"{expected}  */tmp/releases/{archive}",
                f"{expected}  dist/{archive}",
                "not-a-digest  " + archive,
            ]
        ).encode("utf-8")

        with patched_urlopen(checksums) as calls:
            actual = launcher._read_expected_sha256(
                f"https://example.test/releases/v1/{archive}", archive
            )

        self.assertEqual(expected, actual)
        self.assertEqual(
            [("https://example.test/releases/v1/checksums.txt", launcher._CHECKSUM_TIMEOUT)],
            calls,
        )

    def test_read_expected_sha256_rejects_conflicting_duplicate_matches(self):
        archive = "backfill_linux_amd64.tar.gz"
        checksums = (
            f"{'a' * 64}  {archive}\n"
            f"{'b' * 64}  ./dist/{archive}\n"
            f"{'a' * 64}  *{archive}\n"
        ).encode("utf-8")

        with patched_urlopen(checksums), self.assertRaises(SystemExit):
            launcher._read_expected_sha256("https://example.test/releases/latest/x", archive)

    def test_read_expected_sha256_ignores_malformed_rows_with_extra_fields(self):
        archive = "backfill_linux_amd64.tar.gz"
        checksums = (
            f"{'a' * 64}  decoy {archive}\n"
            f"{'b' * 64}  {archive} trailing\n"
        ).encode("utf-8")

        with patched_urlopen(checksums), self.assertRaises(SystemExit):
            launcher._read_expected_sha256("https://example.test/releases/latest/x", archive)


class LauncherDownloadTests(unittest.TestCase):
    def test_download_verifies_archive_before_installing_binary(self):
        archive = tar_bytes([("bf", "verified-binary", 0o755)])
        digest = hashlib.sha256(archive).hexdigest()

        with tempfile.TemporaryDirectory() as tmp:
            binary = Path(tmp) / "bf-1.2.3"
            with (
                mock.patch.object(launcher, "_target", return_value=("linux", "amd64")),
                mock.patch.object(launcher, "_read_expected_sha256", return_value=digest),
                patched_urlopen(archive),
                mock.patch.object(launcher, "_resign_macos") as resign,
            ):
                launcher._download(binary, release_version="1.2.3")

            self.assertEqual("verified-binary", binary.read_text())
            self.assertTrue(os.access(binary, os.X_OK))
            resign.assert_called_once_with(binary)

    def test_download_refuses_mutated_archive_and_leaves_no_binary(self):
        original = tar_bytes([("bf", "original", 0o755)])
        mutated = tar_bytes([("bf", "mutated", 0o755)])
        expected_digest = hashlib.sha256(original).hexdigest()

        with tempfile.TemporaryDirectory() as tmp:
            binary = Path(tmp) / "bf"
            with (
                mock.patch.object(launcher, "_target", return_value=("linux", "amd64")),
                mock.patch.object(
                    launcher, "_read_expected_sha256", return_value=expected_digest
                ),
                patched_urlopen(mutated),
                self.assertRaises(SystemExit),
            ):
                launcher._download(binary)

            self.assertFalse(binary.exists())
            self.assertEqual([], [p.name for p in Path(tmp).iterdir()])

    def test_download_refuses_archive_without_regular_file_bf(self):
        archive = tar_bytes([("bf", None, 0o755), ("not-bf", "payload", 0o755)])
        digest = hashlib.sha256(archive).hexdigest()

        with tempfile.TemporaryDirectory() as tmp:
            binary = Path(tmp) / "bf"
            with (
                mock.patch.object(launcher, "_target", return_value=("linux", "amd64")),
                mock.patch.object(launcher, "_read_expected_sha256", return_value=digest),
                patched_urlopen(archive),
                self.assertRaises(SystemExit),
            ):
                launcher._download(binary)

            self.assertFalse(binary.exists())


class LauncherCacheAndPathTests(unittest.TestCase):
    def test_prune_old_binaries_keeps_current_version_and_unrelated_files(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            for name in ("bf", "bf-0.1.0", "bf-1.2.3", "notes.txt", "bf-helper"):
                (base / name).write_text(name)

            launcher._prune_old_binaries(base, keep="bf-1.2.3")

            self.assertEqual(
                ["bf-1.2.3", "notes.txt"],
                sorted(path.name for path in base.iterdir()),
            )

    def test_shell_export_path_line_quotes_shell_significant_paths(self):
        line = launcher._shell_export_path_line("/tmp/back fill/$bin;name")

        self.assertEqual("export PATH='/tmp/back fill/$bin;name':\"$PATH\"", line)
        self.assertIsNone(launcher._shell_export_path_line("/tmp/backfill\nbin"))

    def test_ensure_on_path_writes_only_temp_rc_files_for_candidate_with_spaces(self):
        with tempfile.TemporaryDirectory() as tmp:
            home = Path(tmp) / "home"
            bindir = Path(tmp) / "bin dir" / "$bf;tools"
            home.mkdir()
            bindir.mkdir(parents=True)
            fake_bf = bindir / "bf"
            fake_bf.write_text("#!/bin/sh\n")
            fake_bf.chmod(stat.S_IRUSR | stat.S_IWUSR | stat.S_IXUSR)

            def expanduser(value):
                return str(home) if value == "~" else os.path.expandvars(value)

            with (
                mock.patch.dict(os.environ, {"PATH": "/usr/bin"}, clear=True),
                mock.patch.object(
                    launcher.sysconfig, "get_path", return_value=str(bindir)
                ),
                mock.patch.object(launcher.os.path, "expanduser", side_effect=expanduser),
                mock.patch.object(launcher.os, "access", return_value=True),
            ):
                launcher._ensure_on_path()

                self.assertEqual(
                    str(bindir), os.environ["PATH"].split(os.pathsep)[0]
                )

            expected_line = launcher._shell_export_path_line(str(bindir))
            for rc_name in (".bashrc", ".zshrc"):
                rc = home / rc_name
                self.assertTrue(rc.exists())
                text = rc.read_text()
                self.assertIn("# >>> backfill path >>>", text)
                self.assertIn(expected_line, text)
                self.assertIn("# <<< backfill path <<<", text)

    def test_main_uses_version_keyed_cache_and_prunes_after_download(self):
        with tempfile.TemporaryDirectory() as tmp:
            home = Path(tmp)
            argv = ["bf", "status"]

            with (
                mock.patch.object(Path, "home", return_value=home),
                mock.patch.object(launcher, "_wheel_version", return_value="1.2.3"),
                mock.patch.object(launcher, "_download") as download,
                mock.patch.object(launcher, "_prune_old_binaries") as prune,
                mock.patch.object(launcher, "_ensure_on_path") as ensure_path,
                mock.patch.object(launcher.os, "execv") as execv,
                mock.patch.object(sys, "argv", argv),
            ):
                launcher.main()

            binary = home / ".local" / "share" / "backfill" / "bf-1.2.3"
            download.assert_called_once_with(binary, release_version="1.2.3")
            prune.assert_called_once_with(binary.parent, keep="bf-1.2.3")
            ensure_path.assert_called_once_with()
            execv.assert_called_once_with(str(binary), [str(binary), "status"])

    def test_main_does_not_exec_when_download_verification_fails(self):
        with tempfile.TemporaryDirectory() as tmp:
            home = Path(tmp)

            with (
                mock.patch.object(Path, "home", return_value=home),
                mock.patch.object(launcher, "_wheel_version", return_value=None),
                mock.patch.object(launcher, "_download", side_effect=SystemExit(1)),
                mock.patch.object(launcher, "_ensure_on_path") as ensure_path,
                mock.patch.object(launcher.os, "execv") as execv,
                self.assertRaises(SystemExit),
            ):
                launcher.main()

            ensure_path.assert_not_called()
            execv.assert_not_called()


if __name__ == "__main__":
    unittest.main()
