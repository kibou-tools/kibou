"""Run the disposable Kibou Buck2 prototype checks on a native CI host."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import platform
import subprocess
import sys
import tarfile
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]


def _run(
    command: list[str],
    *,
    cwd: Path = REPO,
    env: dict[str, str] | None = None,
    check: bool = True,
) -> subprocess.CompletedProcess[str]:
    print("+", subprocess.list2cmdline(command), flush=True)
    result = subprocess.run(command, cwd=cwd, env=env, text=True, check=False)
    if check and result.returncode:
        raise SystemExit(result.returncode)
    return result


def _capture(
    command: list[str],
    *,
    cwd: Path = REPO,
    env: dict[str, str] | None = None,
) -> str:
    print("+", subprocess.list2cmdline(command), flush=True)
    result = subprocess.run(
        command,
        cwd=cwd,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode:
        sys.stdout.write(result.stdout)
        sys.stderr.write(result.stderr)
        raise SystemExit(result.returncode)
    return result.stdout


def _host() -> tuple[str, str]:
    os_name = {
        "darwin": "darwin",
        "linux": "linux",
        "win32": "windows",
    }.get(sys.platform)
    if os_name is None:
        raise SystemExit(f"unsupported prototype host: {sys.platform}")
    machine = platform.machine().lower()
    arch = {
        "amd64": "amd64",
        "x86_64": "amd64",
        "arm64": "arm64",
        "aarch64": "arm64",
    }.get(machine)
    if arch is None:
        raise SystemExit(f"unsupported prototype architecture: {machine}")
    return os_name, arch


def _buck_environment(runtime: Path) -> dict[str, str]:
    runtime.mkdir(parents=True, exist_ok=True)
    home = runtime / "home"
    temp = runtime / "temp"
    home.mkdir(exist_ok=True)
    temp.mkdir(exist_ok=True)

    if os.name == "nt":
        names = (
            "COMSPEC",
            "PATHEXT",
            "SYSTEMDRIVE",
            "SYSTEMROOT",
            "WINDIR",
        )
        environment = {name: os.environ[name] for name in names if name in os.environ}
        environment.update(
            {
                "HOME": str(home),
                "PATH": os.pathsep.join(
                    filter(
                        None,
                        (
                            os.environ.get("SYSTEMROOT", "")
                            + r"\System32\WindowsPowerShell\v1.0",
                            os.environ.get("SYSTEMROOT", "") + r"\System32",
                            os.environ.get("SYSTEMROOT", ""),
                        ),
                    )
                ),
                "TEMP": str(temp),
                "TMP": str(temp),
                "USERPROFILE": str(home),
                "USERNAME": "kibou",
            }
        )
    else:
        environment = {
            "HOME": str(home),
            "LOGNAME": "kibou",
            "PATH": "/usr/bin:/bin",
            "TMPDIR": str(temp),
            "USER": "kibou",
        }
    environment.update(
        {
            "BUCK2_HARD_ERROR": "false",
            "BUCK2_TEST_BLOCK_ON_UPLOAD": "true",
            "BUCK2_TEST_DISABLE_DAEMON_CGROUP": "true",
            "BUCK2_TEST_DISABLE_LOG_UPLOAD": "true",
        }
    )
    return environment


class Buck:
    def __init__(self, binary: Path, environment: dict[str, str]) -> None:
        self.binary = str(binary.resolve())
        self.environment = environment
        self.base = [self.binary, "--isolation-dir", "kibou-prototype-ci"]

    def run(self, *arguments: str, check: bool = True) -> None:
        _run([*self.base, *arguments], env=self.environment, check=check)

    def capture(self, *arguments: str) -> str:
        return _capture([*self.base, *arguments], env=self.environment)

    def output(self, target: str, *, release_version: str | None = None) -> Path:
        command = [
            *self.base,
            "build",
            "--local-only",
            "--no-remote-cache",
            "--show-full-output",
        ]
        if release_version is not None:
            command.extend(["-c", f"kibou.release_version={release_version}"])
        command.append(target)
        stdout = _capture(command, env=self.environment)
        matches = []
        for line in stdout.splitlines():
            fields = line.split(maxsplit=1)
            if len(fields) == 2 and fields[0].endswith(target):
                matches.append(Path(fields[1]))
        if len(matches) != 1:
            raise SystemExit(f"cannot find output for {target!r} in:\n{stdout}")
        return matches[0]


def _generate(goos: str, goarch: str) -> None:
    generator = ["go", "run", "./build_defs/generator/main.go"]
    for mode in ("bootstrap", "project"):
        _run(
            [
                *generator,
                mode,
                "--root",
                ".",
                "--goos",
                goos,
                "--goarch",
                goarch,
            ]
        )


def _generator_tests() -> None:
    _run(
        [
            "go",
            "test",
            "-count=1",
            "-timeout=5m",
            "main.go",
            "main_test.go",
        ],
        cwd=REPO / "build_defs" / "generator",
    )


def _release_version() -> str:
    revision = os.environ.get("GITHUB_SHA", "local")[:12]
    return f"go1.28.0-kibou-prototype.{revision}"


def _build_and_test(buck: Buck, release_version: str) -> dict[str, Path]:
    targets = [line for line in buck.capture("uquery", "//...").splitlines() if line]
    if len(targets) < 2_000:
        raise SystemExit(f"generator exposed only {len(targets)} root targets")
    print(f"uquery loaded {len(targets)} root targets", flush=True)
    buck.run(
        "build",
        "--local-only",
        "--no-remote-cache",
        "//go/src/cmd/compile:stage3",
        "//go/src/cmd/link:stage3",
        "//base/...",
        "//delve/pkg/proc:test",
        "//third_party/build-tools/cmd/rtype:bin",
        "//tools/cmd/stringer:bin",
    )
    buck.run(
        "test",
        "--local-only",
        "--no-remote-cache",
        "//base/core/result:test",
        "//build_defs/testdata/mixed:mixed_test",
        "//build_defs/testdata/external_test_method_ice:direct_test",
        "//build_defs/testdata/external_test_method_ice:test",
        "//build_defs/testdata/external_test_method_ice:transitive_test",
        "//build_defs/testdata/external_test_method_ice:basename_collision_test",
        "//misc/internal/config:test",
        "//tools/txtar:test",
    )
    buck.run(
        "test",
        "--local-only",
        "--no-remote-cache",
        "-c",
        f"kibou.release_version={release_version}",
        "release//probe:release_stamp_test",
    )
    return {
        "compile": buck.output("//go/src/cmd/compile:stage3"),
        "link": buck.output("//go/src/cmd/link:stage3"),
        "stringer": buck.output("//tools/cmd/stringer:bin"),
        "stamped": buck.output(
            "release//probe:stamped", release_version=release_version
        ),
    }


def _bundle(
    outputs: dict[str, Path],
    destination: Path,
    *,
    buck2_version: str,
    goos: str,
    goarch: str,
    release_version: str,
) -> Path:
    destination.mkdir(parents=True, exist_ok=True)
    bundle = destination / f"kibou-buck-prototype-{goos}-{goarch}.tar.gz"
    manifest = {
        "buck2_version": buck2_version.strip(),
        "goos": goos,
        "goarch": goarch,
        "release_version": release_version,
        "outputs": {
            name: {
                "filename": path.name,
                "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
            }
            for name, path in outputs.items()
        },
    }
    manifest_path = destination / "manifest.json"
    manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")
    with tarfile.open(bundle, "w:gz", dereference=True) as archive:
        archive.add(manifest_path, arcname="manifest.json")
        for name, path in outputs.items():
            archive.add(path, arcname=f"bin/{name}{path.suffix}")
    manifest_path.unlink()
    return bundle


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--buck2", required=True, type=Path)
    parser.add_argument("--artifacts", required=True, type=Path)
    parser.add_argument("--runtime", required=True, type=Path)
    arguments = parser.parse_args()

    goos, goarch = _host()
    if (goos, goarch) not in {
        ("linux", "amd64"),
        ("darwin", "arm64"),
        ("windows", "amd64"),
    }:
        raise SystemExit(f"unsupported prototype lane: {goos}/{goarch}")
    _generator_tests()
    _generate(goos, goarch)

    environment = _buck_environment(arguments.runtime)
    buck = Buck(arguments.buck2, environment)
    buck2_version = _capture([str(arguments.buck2), "--version"], env=environment)
    release_version = _release_version()
    try:
        outputs = _build_and_test(buck, release_version)
        bundle = _bundle(
            outputs,
            arguments.artifacts,
            buck2_version=buck2_version,
            goos=goos,
            goarch=goarch,
            release_version=release_version,
        )
        print(bundle)
    finally:
        buck.run("kill", check=False)


if __name__ == "__main__":
    main()
