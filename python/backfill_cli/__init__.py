import hashlib
import os
import platform
import re
import shlex
import shutil
import subprocess
import sys
import sysconfig
import tarfile
import tempfile
import urllib.error
import urllib.request
from pathlib import Path

_CHECKSUM_TIMEOUT = 30
_DOWNLOAD_TIMEOUT = 60


def _fail(message):
    print(message, file=sys.stderr)
    raise SystemExit(1)


def _target():
    if sys.platform == "darwin":
        os_name = "darwin"
    elif sys.platform.startswith("linux"):
        os_name = "linux"
    else:
        _fail(f"backfill does not support {sys.platform} yet")

    machine = platform.machine().lower()
    if machine in ("x86_64", "amd64"):
        arch = "amd64"
    elif machine in ("arm64", "aarch64"):
        arch = "arm64"
    else:
        _fail(f"backfill does not support architecture {machine} yet")

    return os_name, arch


def _read_expected_sha256(archive_url, archive_name):
    # goreleaser publishes one checksums.txt next to the archives, with lines of
    # "<sha256>  <filename>". Fetch it and require an exact name match. Fail
    # closed on any problem — we will not exec an unverified binary.
    checksum_url = archive_url.rsplit("/", 1)[0] + "/checksums.txt"

    try:
        with urllib.request.urlopen(
            checksum_url, timeout=_CHECKSUM_TIMEOUT
        ) as response:
            data = response.read(1024 * 1024).decode("utf-8", "replace")
    except (urllib.error.URLError, TimeoutError, OSError) as exc:
        _fail(
            "backfill: failed to download bf checksums: "
            f"{exc}\nTry the curl installer instead: curl -fsSL https://backfill.sh/install.sh | sh"
        )

    matching = []
    for line in data.splitlines():
        parts = line.split()
        if len(parts) < 2:
            continue
        digest = parts[0].lower()
        if not re.fullmatch(r"[0-9a-f]{64}", digest):
            continue
        names = [part.lstrip("*") for part in parts[1:]]
        if archive_name in names or any(
            name.endswith("/" + archive_name) for name in names
        ):
            matching.append(digest)

    unique = sorted(set(matching))
    if not unique:
        _fail(f"backfill: checksums.txt has no SHA-256 entry for {archive_name}")
    if len(unique) != 1:
        _fail(
            f"backfill: checksums.txt has conflicting SHA-256 entries for {archive_name}"
        )
    return unique[0]


def _resign_macos(binary):
    # macOS 15.4+/26's Code Signing Monitor kills Go binaries that arrive carrying
    # the build machine's linker-signed ad-hoc signature (SIGKILL, crash bug_type
    # 309). Re-signing ad-hoc on this machine regenerates a signature the monitor
    # accepts. Best effort: if codesign is missing or fails, leave the binary as-is.
    if sys.platform != "darwin":
        return
    codesign = shutil.which("codesign")
    if not codesign:
        return
    try:
        subprocess.run(
            [codesign, "--force", "--sign", "-", str(binary)],
            check=False,
            capture_output=True,
            timeout=30,
        )
    except Exception:
        pass


def _prune_old_binaries(base, keep):
    # Remove stale bf-<version> binaries from prior upgrades; keep the current one.
    try:
        for entry in base.iterdir():
            if entry.name != keep and (
                entry.name == "bf" or entry.name.startswith("bf-")
            ):
                try:
                    entry.unlink()
                except OSError:
                    pass
    except OSError:
        pass


def _wheel_version():
    # The published wheel's version equals the release tag (publish-pypi syncs it),
    # so we can fetch the exact matching binary and key the cache to it. Returns
    # None when running from source, where there is no installed distribution.
    try:
        from importlib.metadata import version, PackageNotFoundError

        try:
            return version("backfill-cli")
        except PackageNotFoundError:
            return None
    except Exception:
        return None


def _download(binary, release_version=None):
    os_name, arch = _target()
    archive_name = f"backfill_{os_name}_{arch}.tar.gz"
    if release_version:
        url = f"https://github.com/shyamsivakumar/backfill/releases/download/v{release_version}/{archive_name}"
    else:
        url = f"https://github.com/shyamsivakumar/backfill/releases/latest/download/{archive_name}"
    expected_sha256 = _read_expected_sha256(url, archive_name)
    binary.parent.mkdir(parents=True, exist_ok=True)

    archive_tmp_name = None
    binary_tmp_name = None
    try:
        hasher = hashlib.sha256()
        with urllib.request.urlopen(url, timeout=_DOWNLOAD_TIMEOUT) as response:
            with tempfile.NamedTemporaryFile(
                dir=str(binary.parent), delete=False
            ) as archive_tmp:
                archive_tmp_name = archive_tmp.name
                while True:
                    chunk = response.read(1024 * 1024)
                    if not chunk:
                        break
                    hasher.update(chunk)
                    archive_tmp.write(chunk)

        actual_sha256 = hasher.hexdigest().lower()
        if actual_sha256 != expected_sha256.lower():
            _fail(
                "backfill: checksum verification failed for the downloaded bf archive "
                f"(expected {expected_sha256}, got {actual_sha256}). Refusing to run it."
            )

        with tarfile.open(archive_tmp_name, mode="r:gz") as tar:
            member = None
            for item in tar:
                if item.name == "bf":
                    member = item
                    break

            if member is None or not member.isfile():
                _fail("backfill: release archive did not contain a regular-file bf")

            source = tar.extractfile(member)
            if source is None:
                _fail("backfill: could not read bf from release archive")

            with tempfile.NamedTemporaryFile(
                dir=str(binary.parent), delete=False
            ) as tmp:
                binary_tmp_name = tmp.name
                shutil.copyfileobj(source, tmp)

        os.chmod(binary_tmp_name, 0o755)
        os.replace(binary_tmp_name, binary)
        binary_tmp_name = None
        _resign_macos(binary)
        print(f"backfill: downloaded and verified bf {os_name}/{arch}", file=sys.stderr)
    except (urllib.error.URLError, TimeoutError, OSError, tarfile.TarError) as exc:
        _fail(
            "backfill: failed to download bf: "
            f"{exc}\nTry the curl installer instead: curl -fsSL https://backfill.sh/install.sh | sh"
        )
    finally:
        for tmp_name in (binary_tmp_name, archive_tmp_name):
            if tmp_name is not None:
                try:
                    os.unlink(tmp_name)
                except OSError:
                    pass


def _shell_export_path_line(bindir):
    # shlex.quote makes the path injection-proof inside the rc file. The single
    # quotes it adds and the double-quoted $PATH concatenate correctly in sh/bash/zsh.
    try:
        bindir = str(bindir)
        if "\x00" in bindir or "\n" in bindir or "\r" in bindir:
            return None
        return f'export PATH={shlex.quote(bindir)}:"$PATH"'
    except Exception:
        return None


def _ensure_on_path():
    # 'pip install --user' drops the bf console script into a user scripts dir
    # (e.g. /workspace/.local/bin) that is frequently not on PATH, so plain `bf`
    # is not found. Detect that dir and add it to PATH for this process and for
    # future shells. Never raise — the launcher must still exec bf regardless.
    try:
        candidates = []

        def add_candidate(bindir):
            try:
                if not bindir:
                    return
                bindir = os.path.abspath(os.path.expanduser(str(bindir)))
                if (
                    os.path.isfile(os.path.join(bindir, "bf"))
                    and bindir not in candidates
                ):
                    candidates.append(bindir)
            except Exception:
                pass

        schemes = ["posix_user"]
        try:
            schemes.append(sysconfig.get_default_scheme())
        except Exception:
            schemes.append("posix_prefix")
        for scheme in schemes:
            try:
                add_candidate(sysconfig.get_path("scripts", scheme=scheme))
            except Exception:
                pass

        try:
            argv0 = sys.argv[0] if sys.argv else ""
            if argv0 and os.path.basename(argv0) == "bf":
                add_candidate(os.path.dirname(os.path.realpath(argv0)))
        except Exception:
            pass

        path_entries = os.environ.get("PATH", "").split(os.pathsep)
        for bindir in candidates:
            try:
                if bindir in path_entries:
                    continue

                current_path = os.environ.get("PATH", "")
                os.environ["PATH"] = (
                    bindir if not current_path else bindir + os.pathsep + current_path
                )
                path_entries = os.environ.get("PATH", "").split(os.pathsep)

                export_line = _shell_export_path_line(bindir)
                if export_line is None:
                    continue

                modified = []
                try:
                    home = os.path.expanduser("~")
                    home_writable = bool(
                        home and os.path.isdir(home) and os.access(home, os.W_OK)
                    )
                    rc_paths = []

                    for name in (".bashrc", ".zshrc"):
                        rc = os.path.join(home, name)
                        if os.path.exists(rc) or home_writable:
                            rc_paths.append(rc)

                    marker_start = "# >>> backfill path >>>"
                    marker_end = "# <<< backfill path <<<"

                    for rc in rc_paths:
                        try:
                            existing = ""
                            if os.path.exists(rc):
                                try:
                                    with open(rc, "r", encoding="utf-8") as f:
                                        existing = f.read()
                                except Exception:
                                    continue

                            already_present = False
                            search_from = 0
                            while True:
                                start = existing.find(marker_start, search_from)
                                if start == -1:
                                    break
                                end = existing.find(
                                    marker_end, start + len(marker_start)
                                )
                                if end == -1:
                                    break
                                if (
                                    export_line
                                    in existing[start : end + len(marker_end)]
                                ):
                                    already_present = True
                                    break
                                search_from = end + len(marker_end)

                            if already_present:
                                continue

                            prefix = (
                                "\n"
                                if (existing and not existing.endswith("\n"))
                                else ""
                            )
                            with open(rc, "a", encoding="utf-8") as f:
                                f.write(
                                    f"{prefix}{marker_start}\n{export_line}\n{marker_end}\n"
                                )
                            modified.append(rc)
                        except Exception:
                            continue
                except Exception:
                    modified = []

                if modified:
                    try:
                        home = os.path.expanduser("~")
                        pretty = [
                            "~/" + os.path.relpath(rc, home)
                            if home and rc.startswith(home + os.sep)
                            else rc
                            for rc in modified
                        ]
                        source_target = (
                            "~/.zshrc" if "~/.zshrc" in pretty else pretty[0]
                        )
                        print(
                            f"backfill: added {bindir} to PATH in {', '.join(pretty)}. "
                            f"Restart your shell or run: source {source_target}",
                            file=sys.stderr,
                        )
                    except Exception:
                        pass
            except Exception:
                continue
    except Exception:
        pass


def main():
    if sys.platform == "win32":
        _fail("backfill does not support Windows yet")

    # Key the cached binary to the wheel version so `pip install -U backfill-cli`
    # fetches the matching new binary instead of reusing a stale one forever. From
    # source (no installed version) fall back to a single "bf" + the latest release.
    base = Path.home() / ".local" / "share" / "backfill"
    release_version = _wheel_version()
    binary = base / (f"bf-{release_version}" if release_version else "bf")

    if not (binary.exists() and os.access(str(binary), os.X_OK)):
        _download(binary, release_version=release_version)
        _prune_old_binaries(base, keep=binary.name)

    _ensure_on_path()

    os.execv(str(binary), [str(binary)] + sys.argv[1:])
