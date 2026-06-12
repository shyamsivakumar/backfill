import os
import platform
import shutil
import sys
import tarfile
import tempfile
import urllib.error
import urllib.request
from pathlib import Path


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


def _download(binary):
    os_name, arch = _target()
    url = f"https://github.com/shyamsivakumar/backfill/releases/latest/download/backfill_{os_name}_{arch}.tar.gz"
    binary.parent.mkdir(parents=True, exist_ok=True)

    tmp_name = None
    try:
        with urllib.request.urlopen(url) as response:
            with tarfile.open(fileobj=response, mode="r|gz") as tar:
                member = None
                for item in tar:
                    if item.name == "bf":
                        member = item
                        break

                if member is None or not member.isfile():
                    _fail("backfill: release archive did not contain bf")

                source = tar.extractfile(member)
                if source is None:
                    _fail("backfill: could not read bf from release archive")

                with tempfile.NamedTemporaryFile(dir=str(binary.parent), delete=False) as tmp:
                    tmp_name = tmp.name
                    shutil.copyfileobj(source, tmp)

        os.chmod(tmp_name, 0o755)
        os.replace(tmp_name, binary)
        tmp_name = None
        print(f"backfill: downloaded bf {os_name}/{arch}", file=sys.stderr)
    except (urllib.error.URLError, TimeoutError, OSError) as exc:
        _fail(
            "backfill: failed to download bf: "
            f"{exc}\nTry the curl installer instead: curl -fsSL https://backfill.sh/install.sh | sh"
        )
    finally:
        if tmp_name is not None:
            try:
                os.unlink(tmp_name)
            except OSError:
                pass


def main():
    if sys.platform == "win32":
        _fail("backfill does not support Windows yet")

    binary = Path.home() / ".local" / "share" / "backfill" / "bf"

    if not (binary.exists() and os.access(str(binary), os.X_OK)):
        _download(binary)

    os.execv(str(binary), [str(binary)] + sys.argv[1:])
