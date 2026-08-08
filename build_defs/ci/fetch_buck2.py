"""Download and verify one native binary from the isolated Buck2 release."""

from __future__ import annotations

import argparse
import hashlib
import os
import platform
import stat
import sys
import urllib.request
from pathlib import Path

import zstandard

REPOSITORY = "kibou-tools/buck2"


def _asset_name() -> str:
    machine = platform.machine().lower()
    host = (sys.platform, machine)
    assets = {
        ("linux", "x86_64"): "buck2-x86_64-unknown-linux-gnu.zst",
        ("darwin", "arm64"): "buck2-aarch64-apple-darwin.zst",
        ("darwin", "aarch64"): "buck2-aarch64-apple-darwin.zst",
        ("win32", "amd64"): "buck2-x86_64-pc-windows-msvc.exe.zst",
        ("win32", "x86_64"): "buck2-x86_64-pc-windows-msvc.exe.zst",
    }
    try:
        return assets[host]
    except KeyError:
        raise SystemExit(f"unsupported Buck2 prototype host: {host!r}") from None


def _download(url: str, destination: Path) -> None:
    print(f"download {url}", flush=True)
    with urllib.request.urlopen(url) as response, destination.open("wb") as output:
        while chunk := response.read(1024 * 1024):
            output.write(chunk)


def _expected_checksum(sums: str, asset: str) -> str:
    matches = []
    for line in sums.splitlines():
        fields = line.split()
        if len(fields) == 2 and fields[1].lstrip("*") == asset:
            matches.append(fields[0])
    if len(matches) != 1 or len(matches[0]) != 64:
        raise SystemExit(f"SHA256SUMS has no unique checksum for {asset!r}")
    return matches[0].lower()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--tag", required=True)
    parser.add_argument("--output", required=True, type=Path)
    arguments = parser.parse_args()

    asset = _asset_name()
    arguments.output.parent.mkdir(parents=True, exist_ok=True)
    work = arguments.output.parent
    archive = work / asset
    sums = work / "SHA256SUMS"
    base = f"https://github.com/{REPOSITORY}/releases/download/{arguments.tag}"
    _download(f"{base}/SHA256SUMS", sums)
    _download(f"{base}/{asset}", archive)

    expected = _expected_checksum(sums.read_text(), asset)
    actual = hashlib.sha256(archive.read_bytes()).hexdigest()
    if actual != expected:
        raise SystemExit(
            f"checksum mismatch for {asset}: expected {expected}, got {actual}"
        )

    with archive.open("rb") as source, arguments.output.open("wb") as output:
        zstandard.ZstdDecompressor().copy_stream(source, output)
    if os.name != "nt":
        arguments.output.chmod(
            arguments.output.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH
        )
    print(arguments.output.resolve())


if __name__ == "__main__":
    main()
