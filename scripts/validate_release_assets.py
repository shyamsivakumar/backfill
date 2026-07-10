#!/usr/bin/env python3
"""Validate GitHub release metadata and checksums before PyPI publishing.

This module is deliberately network-free. The release workflow downloads the
GitHub API response and ``checksums.txt`` with its scoped token, then passes the
local files to this validator. Tests can exercise the same gate with fixtures.
"""

import argparse
import json
import re
import sys
from collections import Counter
from pathlib import Path
from typing import Any, Dict, List, Optional, Sequence


TAG_PATTERN = re.compile(r"^v[0-9]+\.[0-9]+\.[0-9]+$")
REQUIRED_ARCHIVES = (
    "backfill_darwin_amd64.tar.gz",
    "backfill_darwin_arm64.tar.gz",
    "backfill_linux_amd64.tar.gz",
    "backfill_linux_arm64.tar.gz",
)
CHECKSUMS_ASSET = "checksums.txt"
CHECKSUM_LINE_PATTERN = re.compile(
    r"^(?P<digest>[0-9a-fA-F]{64})\s+[*]?(?P<name>\S+)\s*$"
)


def validate_release(
    release: Dict[str, Any], checksums_text: str, expected_tag: str
) -> List[str]:
    """Return deterministic validation errors, or an empty list on success."""
    errors = []  # type: List[str]

    if not TAG_PATTERN.fullmatch(expected_tag):
        errors.append(
            "tag must use the release form vMAJOR.MINOR.PATCH: {!r}".format(
                expected_tag
            )
        )

    if release.get("tag_name") != expected_tag:
        errors.append(
            "release tag {!r} does not match workflow tag {!r}".format(
                release.get("tag_name"), expected_tag
            )
        )
    if release.get("draft") is not False:
        errors.append("release must be explicitly non-draft")
    if release.get("prerelease") is not False:
        errors.append("release must be explicitly non-prerelease")

    assets = release.get("assets")
    if not isinstance(assets, list):
        errors.append("release assets must be a list")
        return errors

    asset_names = []  # type: List[str]
    for asset in assets:
        if not isinstance(asset, dict) or not isinstance(asset.get("name"), str):
            errors.append("every release asset must have a string name")
            continue
        asset_names.append(asset["name"])

    asset_counts = Counter(asset_names)
    for name in sorted(name for name, count in asset_counts.items() if count > 1):
        errors.append("duplicate release asset: {}".format(name))

    required_assets = REQUIRED_ARCHIVES + (CHECKSUMS_ASSET,)
    for name in required_assets:
        if asset_counts[name] == 0:
            errors.append("missing release asset: {}".format(name))

    checksum_counts = Counter()  # type: Counter[str]
    for line_number, raw_line in enumerate(checksums_text.splitlines(), start=1):
        if not raw_line.strip():
            continue
        match = CHECKSUM_LINE_PATTERN.fullmatch(raw_line)
        if match is None:
            errors.append("malformed checksums.txt line {}".format(line_number))
            continue
        checksum_name = match.group("name").rsplit("/", 1)[-1]
        checksum_counts[checksum_name] += 1

    for name in sorted(name for name, count in checksum_counts.items() if count > 1):
        errors.append("duplicate checksum entry: {}".format(name))

    for name in REQUIRED_ARCHIVES:
        if checksum_counts[name] == 0:
            errors.append("checksums.txt does not mention: {}".format(name))

    return errors


def _load_release(path: Path) -> Dict[str, Any]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(payload, dict):
        raise ValueError("release JSON must contain an object")
    return payload


def main(argv: Optional[Sequence[str]] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--release-json", required=True, type=Path)
    parser.add_argument("--checksums", required=True, type=Path)
    parser.add_argument("--tag", required=True)
    args = parser.parse_args(argv)

    try:
        release = _load_release(args.release_json)
        checksums_text = args.checksums.read_text(encoding="utf-8")
    except (OSError, UnicodeError, ValueError) as exc:
        print("release asset gate could not read its inputs: {}".format(exc), file=sys.stderr)
        return 2

    errors = validate_release(release, checksums_text, args.tag)
    if errors:
        for error in errors:
            print("release asset gate: {}".format(error), file=sys.stderr)
        return 1

    print("release asset gate: matching release assets and checksums are present")
    return 0


if __name__ == "__main__":
    sys.exit(main())
