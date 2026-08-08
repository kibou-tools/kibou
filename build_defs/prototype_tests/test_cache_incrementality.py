"""Linux interoperability proof for Kibou's Buck2 cache and Go bootstrap graph.

This test intentionally mutates two tracked prototype inputs. It restores their
original bytes in ``finally`` and keeps NativeLink state, Buck command logs, and
the isolated Buck output tree outside the watched cell where possible.

Run with::

    KIBOU_BUCK2=/path/to/buck2 pytest -q \
        build_defs/prototype_tests/test_cache_incrementality.py
"""

from __future__ import annotations

import hashlib
import json
import os
import platform
import shutil
import socket
import subprocess
import sys
import tarfile
import time
import urllib.request
import uuid
from pathlib import Path
from typing import Any

import pytest

pytestmark = pytest.mark.skipif(
    sys.platform != "linux" or platform.machine() not in {"x86_64", "AMD64"},
    reason="the pinned NativeLink artifact is Linux x86-64",
)

NATIVELINK_VERSION = "1.6.4"
NATIVELINK_ARCHIVE = "nativelink-1.6.4-x86_64-unknown-linux-musl.tar.gz"
NATIVELINK_SHA256 = "81f0140f7d2f167c875e2ef1ce7825d92ac86c09b355441a9772b577a0c3f3bd"
NATIVELINK_URL = (
    "https://github.com/TraceMachina/nativelink/releases/download/"
    f"v{NATIVELINK_VERSION}/{NATIVELINK_ARCHIVE}"
)

REPO = Path(__file__).resolve().parents[2]
AUDITED_TEST = "//build_defs/testdata/mixed:mixed_test"
AUDITED_TEST_IDENTITY = AUDITED_TEST.rsplit(":", 1)[1]
PROJECT_PACKAGE_IDENTITY = "root//build_defs/testdata/mixed"
COMPILER_TARGETS = tuple(f"//go/src/cmd/compile:stage{stage}" for stage in (1, 2, 3))
COMPILER_IDENTITY = REPO / "build_defs/generated/BUCK"
COMPILER_SOURCE = REPO / "go/src/cmd/compile/main.go"
TEST_RESOURCE = REPO / "build_defs/testdata/mixed/testdata/message.txt"


def _runtime_base() -> Path:
    configured = os.environ.get("KIBOU_CACHE_PROTOTYPE_RUNTIME")
    if configured:
        return Path(configured).resolve()
    runner_temp = os.environ.get("RUNNER_TEMP")
    if runner_temp:
        return Path(runner_temp).resolve() / "kibou-cache-prototype"
    # The disposable jj workspace normally lives below PRIMARY/.cache.
    if REPO.parent.name == "jj-workspaces" and REPO.parent.parent.name == ".cache":
        return REPO.parent.parent / "kibou-cache-prototype"
    return Path.home() / ".cache/kibou-cache-prototype"


def _buck2() -> Path:
    for candidate in (
        os.environ.get("KIBOU_BUCK2"),
        os.environ.get("BUCK2"),
        shutil.which("buck2"),
    ):
        if candidate and Path(candidate).is_file():
            return Path(candidate).resolve()
    pytest.fail("set KIBOU_BUCK2 (or BUCK2) to the patched Buck2 binary")


def _generate_native_graph() -> None:
    for mode in ("bootstrap", "project"):
        command = [
            "go",
            "run",
            "./build_defs/generator/main.go",
            mode,
            "--root",
            ".",
            "--goos",
            "linux",
            "--goarch",
            "amd64",
        ]
        result = subprocess.run(
            command,
            cwd=REPO,
            text=True,
            capture_output=True,
            check=False,
        )
        if result.returncode:
            pytest.fail(
                f"{' '.join(command)} failed ({result.returncode})\n"
                f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
            )


class NativeLink:
    def __init__(
        self, binary: Path, config: Path, cache: Path, log: Path, port: int
    ) -> None:
        self._log = log.open("w")
        self._process = subprocess.Popen(
            [str(binary), str(config)],
            env={**os.environ, "NATIVELINK_CACHE_ROOT": str(cache)},
            stdout=self._log,
            stderr=subprocess.STDOUT,
        )
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            if self._process.poll() is not None:
                self.stop()
                pytest.fail(
                    f"NativeLink exited before becoming ready:\n{log.read_text()}"
                )
            try:
                with socket.create_connection(("127.0.0.1", port), timeout=0.1):
                    return
            except OSError:
                time.sleep(0.1)
        self.stop()
        pytest.fail(f"NativeLink did not become ready:\n{log.read_text()}")

    def stop(self) -> None:
        if self._process.poll() is None:
            self._process.terminate()
            try:
                self._process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self._process.kill()
                self._process.wait()
        if not self._log.closed:
            self._log.close()


class Buck:
    def __init__(self, binary: Path, isolation: str, port: int, logs: Path) -> None:
        self.binary = binary
        self.isolation = isolation
        self.logs = logs
        self.logs.mkdir(parents=True)
        self._command_number = 0
        self.environment = {
            **os.environ,
            "BUCK2_HARD_ERROR": "false",
            "BUCK2_TEST_BLOCK_ON_UPLOAD": "true",
            "BUCK2_TEST_DISABLE_DAEMON_CGROUP": "true",
            "BUCK2_TEST_DISABLE_LOG_UPLOAD": "true",
        }
        address = f"grpc://127.0.0.1:{port}"
        self.cache_config = (
            "-c",
            "kibou.allow_cache_uploads=true",
            "-c",
            "kibou.remote_cache_enabled=true",
            "-c",
            f"buck2_re_client.action_cache_address={address}",
            "-c",
            f"buck2_re_client.cas_address={address}",
            "-c",
            f"buck2_re_client.engine_address={address}",
            "-c",
            "buck2_re_client.capabilities=false",
            "-c",
            "buck2_re_client.digest_algorithms=SHA256",
            "-c",
            "buck2_re_client.instance_name=main",
            "-c",
            "buck2_re_client.tls=false",
        )

    def run(
        self, command: str, *args: str, cache: bool = False, check: bool = True
    ) -> subprocess.CompletedProcess[str]:
        argv = [str(self.binary), "--isolation-dir", self.isolation, command]
        if cache:
            argv.extend(self.cache_config)
        argv.extend(args)
        result = subprocess.run(
            argv,
            cwd=REPO,
            env=self.environment,
            text=True,
            capture_output=True,
            check=False,
        )
        self._command_number += 1
        stem = f"{self._command_number:02d}-{command}"
        self.logs.joinpath(stem + ".command.json").write_text(
            json.dumps({"argv": argv, "returncode": result.returncode}, indent=2) + "\n"
        )
        self.logs.joinpath(stem + ".stdout").write_text(result.stdout)
        self.logs.joinpath(stem + ".stderr").write_text(result.stderr)
        if check and result.returncode:
            pytest.fail(
                f"{' '.join(argv)} failed ({result.returncode})\n"
                f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
            )
        return result

    def test(self) -> None:
        self.run("test", AUDITED_TEST, "-v=0", cache=True)

    def compiler_outputs(self) -> dict[str, Path]:
        return self.build_outputs(*COMPILER_TARGETS)

    def test_output(self) -> Path:
        return self.build_outputs(AUDITED_TEST)[AUDITED_TEST]

    def build_outputs(self, *targets: str) -> dict[str, Path]:
        result = self.run("build", "--show-full-output", *targets, cache=True)
        outputs: dict[str, Path] = {}
        for line in result.stdout.splitlines():
            fields = line.split(maxsplit=1)
            if len(fields) != 2:
                continue
            for target in targets:
                if fields[0].endswith(target):
                    path = Path(fields[1])
                    outputs[target] = path if path.is_absolute() else REPO / path
        assert outputs.keys() == set(targets), result.stdout
        return outputs

    def entries(self, name: str) -> list[dict[str, Any]]:
        result = self.run("log", "what-ran", "--format", "json", "--emit-cache-queries")
        entries = [
            json.loads(line)
            for line in result.stdout.splitlines()
            if line.startswith("{")
        ]
        self.logs.joinpath(name + ".what-ran.json").write_text(
            json.dumps(entries, indent=2) + "\n"
        )
        events = self.run("log", "show").stdout
        self.logs.joinpath(name + ".events.jsonl").write_text(events)
        return entries

    def successful_uploads(self) -> list[dict[str, Any]]:
        uploads = []
        for line in self.run("log", "show").stdout.splitlines():
            event = json.loads(line)
            upload = (
                event.get("Event", {})
                .get("data", {})
                .get("SpanEnd", {})
                .get("data", {})
                .get("CacheUpload")
            )
            if upload is not None and upload.get("success"):
                uploads.append(upload)
        return uploads

    def reset(self) -> None:
        self.run("kill", check=False)
        output = REPO / "buck-out" / self.isolation
        assert output.parent == REPO / "buck-out"
        shutil.rmtree(output, ignore_errors=True)


def test_cache_and_compiler_incrementality() -> None:
    _generate_native_graph()
    run = _runtime_base() / (
        f"run-{time.strftime('%Y%m%d-%H%M%S')}-{os.getpid()}-{uuid.uuid4().hex[:8]}"
    )
    cache = run / "cache"
    logs = run / "logs"
    cache.mkdir(parents=True)

    port = _unused_port()
    config = run / "nativelink.json5"
    config.write_text(_nativelink_config(port))
    nativelink = _download_nativelink(run)
    isolation = f"kibou-cache-proof-{os.getpid()}-{port}"
    buck = Buck(_buck2(), isolation, port, logs)
    server = NativeLink(nativelink, config, cache, run / "nativelink.log", port)

    originals = {
        COMPILER_IDENTITY: COMPILER_IDENTITY.read_bytes(),
        COMPILER_SOURCE: COMPILER_SOURCE.read_bytes(),
        TEST_RESOURCE: TEST_RESOURCE.read_bytes(),
    }
    try:
        baseline_compilers = buck.compiler_outputs()
        baseline_compiler_hashes = _hash_outputs(baseline_compilers)

        buck.test()
        first_entries = buck.entries("first-local")
        assert _test_run(first_entries)["reproducer"]["executor"] == "Local"
        assert buck.successful_uploads(), "the successful native test was not uploaded"
        _assert_no_remote_execution(first_entries)
        baseline_test_hash = _sha256(buck.test_output())

        buck.reset()
        buck.test()
        restored_entries = buck.entries("after-output-reset")
        assert _test_run(restored_entries)["reproducer"]["executor"] == "Cache"
        _assert_no_remote_execution(restored_entries)
        assert _sha256(buck.test_output()) == baseline_test_hash

        before_touch = TEST_RESOURCE.stat()
        os.utime(
            TEST_RESOURCE,
            ns=(before_touch.st_atime_ns, before_touch.st_mtime_ns + 1_000_000_000),
        )
        assert TEST_RESOURCE.stat().st_mtime_ns != before_touch.st_mtime_ns
        buck.test()
        touched_entries = buck.entries("resource-mtime-only")
        assert _test_run(touched_entries)["reproducer"]["executor"] == "Cache"
        _assert_no_remote_execution(touched_entries)

        TEST_RESOURCE.write_text(
            f"pytest-{uuid.uuid4().hex} from a declared resource\n"
        )
        buck.test()
        changed_resource_entries = buck.entries("resource-content-change")
        assert _test_run(changed_resource_entries)["reproducer"]["executor"] == "Local"
        assert buck.successful_uploads(), "changed resource result was not uploaded"
        _assert_no_remote_execution(changed_resource_entries)

        _mutate_compiler_identity()
        buck.test()
        identity_entries = buck.entries("compiler-identity-change")
        assert _test_run(identity_entries)["reproducer"]["executor"] == "Cache"
        project_compiles = [
            entry
            for entry in _executed_builds(identity_entries)
            if PROJECT_PACKAGE_IDENTITY in entry.get("identity", "")
            and "(go_compile " in entry.get("identity", "")
        ]
        assert project_compiles, identity_entries
        assert all(
            entry["reproducer"]["executor"] == "Local" for entry in project_compiles
        ), project_compiles
        _assert_no_remote_execution(identity_entries)
        assert _sha256(buck.test_output()) == baseline_test_hash

        _mutate_compiler_comment()
        changed_compilers = buck.compiler_outputs()
        source_entries = buck.entries("compiler-source-comment-change")
        for stage in (1, 2, 3):
            needle = f"root//go/src/cmd/compile:stage{stage}_pkg"
            matches = [
                entry
                for entry in _executed_builds(source_entries)
                if needle in entry.get("identity", "")
                and "(go_compile bootstrap/cmd/compile " in entry.get("identity", "")
            ]
            assert matches, (needle, source_entries)
            assert any(
                entry["reproducer"]["executor"] == "Local" for entry in matches
            ), matches
        assert _hash_outputs(changed_compilers) == baseline_compiler_hashes
        _assert_no_remote_execution(source_entries)

        buck.test()
        downstream_entries = buck.entries("after-compiler-comment-change")
        assert _test_run(downstream_entries)["reproducer"]["executor"] == "Cache"
        assert not [
            entry
            for entry in _executed_builds(downstream_entries)
            if PROJECT_PACKAGE_IDENTITY in entry.get("identity", "")
            and entry["reproducer"]["executor"] == "Local"
        ], downstream_entries
        assert _sha256(buck.test_output()) == baseline_test_hash
        _assert_no_remote_execution(downstream_entries)

        buck.compiler_outputs()
        noop_compiler_entries = buck.entries("noop-compilers")
        assert not _executed_builds(noop_compiler_entries), noop_compiler_entries
        buck.test()
        noop_test_entries = buck.entries("noop-test")
        assert not _executed_builds(noop_test_entries), noop_test_entries
        assert _test_run(noop_test_entries)["reproducer"]["executor"] == "Cache"
        _assert_no_remote_execution(noop_test_entries)
    finally:
        cleanup_errors = []
        for path, content in originals.items():
            try:
                path.write_bytes(content)
            except OSError as error:
                cleanup_errors.append(error)
        try:
            buck.reset()
        except OSError as error:
            cleanup_errors.append(error)
        try:
            server.stop()
        except OSError as error:
            cleanup_errors.append(error)
        if cleanup_errors:
            raise cleanup_errors[0]


def _download_nativelink(root: Path) -> Path:
    archive = root / NATIVELINK_ARCHIVE
    urllib.request.urlretrieve(NATIVELINK_URL, archive)
    actual = hashlib.sha256(archive.read_bytes()).hexdigest()
    assert actual == NATIVELINK_SHA256
    with tarfile.open(archive, "r:gz") as tar:
        members = [member for member in tar.getmembers() if member.name == "nativelink"]
        assert len(members) == 1, [member.name for member in tar.getmembers()]
        source = tar.extractfile(members[0])
        assert source is not None
        binary = root / "nativelink"
        binary.write_bytes(source.read())
    binary.chmod(0o755)
    return binary


def _unused_port() -> int:
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return listener.getsockname()[1]


def _nativelink_config(port: int) -> str:
    return f"""{{
  stores: [
    {{
      name: "AC_STORE",
      filesystem: {{
        content_path: "${{NATIVELINK_CACHE_ROOT}}/ac/content",
        temp_path: "${{NATIVELINK_CACHE_ROOT}}/ac/temp",
        eviction_policy: {{ max_bytes: 1000000000 }},
      }},
    }},
    {{
      name: "CAS_STORE",
      filesystem: {{
        content_path: "${{NATIVELINK_CACHE_ROOT}}/cas/content",
        temp_path: "${{NATIVELINK_CACHE_ROOT}}/cas/temp",
        eviction_policy: {{ max_bytes: 6000000000 }},
      }},
    }},
  ],
  servers: [{{
    name: "cache",
    listener: {{ http: {{ socket_address: "127.0.0.1:{port}" }} }},
    services: {{
      cas: [{{ instance_name: "main", cas_store: "CAS_STORE" }}],
      ac: [{{ instance_name: "main", ac_store: "AC_STORE" }}],
      bytestream: [{{ instance_name: "main", cas_store: "CAS_STORE" }}],
    }},
  }}],
}}"""


def _mutate_compiler_identity() -> None:
    text = COMPILER_IDENTITY.read_text()
    marker = 'name = "compiler_identity"'
    start = text.index(marker)
    content = text.index('content = "', start) + len('content = "')
    end = text.index('"', content)
    replacement = f"pytest-identity-{uuid.uuid4().hex}\\n"
    COMPILER_IDENTITY.write_text(text[:content] + replacement + text[end:])


def _mutate_compiler_comment() -> None:
    original = COMPILER_SOURCE.read_text()
    before = "\t// disable timestamps for reproducible output"
    after = "\t// Disable timestamps for reproducible output"
    assert len(before) == len(after)
    assert original.count(before) == 1
    changed = original.replace(before, after)
    assert len(changed.splitlines()) == len(original.splitlines())
    COMPILER_SOURCE.write_text(changed)


def _test_run(entries: list[dict[str, Any]]) -> dict[str, Any]:
    matches = [
        entry
        for entry in entries
        if entry.get("reason") == "test.run"
        and entry.get("identity") == AUDITED_TEST_IDENTITY
        and entry.get("reproducer", {}).get("executor") != "CacheQuery"
    ]
    assert len(matches) == 1, matches
    return matches[0]


def _executed_builds(entries: list[dict[str, Any]]) -> list[dict[str, Any]]:
    return [
        entry
        for entry in entries
        if entry.get("reason") == "build"
        and entry.get("reproducer", {}).get("executor") != "CacheQuery"
    ]


def _assert_no_remote_execution(entries: list[dict[str, Any]]) -> None:
    assert not [
        entry
        for entry in entries
        if entry.get("reproducer", {}).get("executor") == "Re"
    ], entries


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        while chunk := source.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def _hash_outputs(outputs: dict[str, Path]) -> dict[str, str]:
    return {target: _sha256(path) for target, path in outputs.items()}
