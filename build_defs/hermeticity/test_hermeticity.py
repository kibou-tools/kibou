"""Linux positive and negative controls for Buck2 local-action hermeticity.

Run this file directly with pytest.  It deliberately demonstrates dependencies
that Buck2's local executor does not include in an action key, and checks that
the accompanying syscall audit sees them. Ubuntu 24.04 requires ``strace`` and
``util-linux``, passwordless sudo for attaching to the forkserver, and these
ephemeral-runner settings before pytest starts::

    sudo sysctl -w kernel.unprivileged_userns_clone=1
    sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0
"""

from __future__ import annotations

import ast
import contextlib
import json
import os
import platform
import re
import shlex
import shutil
import socketserver
import subprocess
import sys
import threading
import time
from pathlib import Path

import pytest

pytestmark = pytest.mark.skipif(
    sys.platform != "linux",
    reason="Buck2's local network namespace and strace audit are Linux-specific",
)

PACKAGE = "//build_defs/hermeticity:"
REPO = Path(__file__).resolve().parents[2]


def _runtime_directory() -> Path:
    configured = os.environ.get("KIBOU_HERMETICITY_RUNTIME")
    if configured:
        return Path(configured).resolve()
    # A jj workspace for this prototype lives below PRIMARY/.cache/jj-workspaces.
    # Keep traces in PRIMARY/.cache, not in the nested Buck cell watched by buckd.
    if REPO.parent.name == "jj-workspaces" and REPO.parent.parent.name == ".cache":
        return REPO.parent.parent / "2026-08-09-hermeticity-audit"
    runner_temp = os.environ.get("RUNNER_TEMP")
    if runner_temp:
        return Path(runner_temp).resolve() / "kibou-hermeticity-audit"
    return Path.home() / ".cache" / "kibou-hermeticity-audit"


RUNTIME = _runtime_directory()
_BUCK_INSTANCES = []


def _buck2() -> str:
    for candidate in (
        os.environ.get("KIBOU_BUCK2"),
        os.environ.get("BUCK2"),
        shutil.which("buck2"),
    ):
        if candidate and Path(candidate).is_file():
            return str(Path(candidate).resolve())
    pytest.fail("set KIBOU_BUCK2 (or BUCK2) to the Buck2 binary under test")


def _generate_native_graph() -> None:
    machine = platform.machine().lower()
    goarch = {
        "amd64": "amd64",
        "x86_64": "amd64",
        "arm64": "arm64",
        "aarch64": "arm64",
    }.get(machine)
    if goarch is None:
        pytest.fail(f"unsupported Linux prototype architecture: {machine}")
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
            goarch,
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


class Buck:
    def __init__(self, isolation: str, *, secret: str | None = None) -> None:
        self.binary = _buck2()
        self.isolation = isolation
        self.environment = os.environ.copy()
        self.environment.pop("KIBOU_AUDIT_SECRET", None)
        self.environment["KIBOU_HERMETICITY_RUNTIME"] = str(RUNTIME)
        if secret is not None:
            self.environment["KIBOU_AUDIT_SECRET"] = secret
        _BUCK_INSTANCES.append(self)

    def run(self, *args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
        command = [self.binary, "--isolation-dir", self.isolation, *args]
        if args and args[0] in {"build", "test", "uquery"}:
            command[4:4] = ["-c", f"kibou.hermeticity_runtime={RUNTIME}"]
        result = subprocess.run(
            command,
            cwd=REPO,
            env=self.environment,
            text=True,
            capture_output=True,
            check=False,
        )
        if check and result.returncode:
            pytest.fail(
                f"buck2 {' '.join(args)} failed ({result.returncode})\n"
                f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
            )
        return result

    def build_outputs(self, *targets: str) -> dict[str, Path]:
        result = self.run(
            "build",
            "--local-only",
            "--no-remote-cache",
            "--show-full-output",
            *targets,
        )
        outputs = {}
        for line in result.stdout.splitlines():
            fields = line.split(maxsplit=1)
            if len(fields) != 2:
                continue
            for target in targets:
                if fields[0].endswith(target):
                    outputs[target] = Path(fields[1])
        assert outputs.keys() == set(targets), result.stdout
        return outputs

    def build_output(self, target: str) -> Path:
        return self.build_outputs(target)[target]

    def what_ran(self) -> list[dict[str, object]]:
        result = self.run("log", "what-ran", "--format", "json", "--skip-cache-hits")
        return [
            json.loads(line)
            for line in result.stdout.splitlines()
            if line.startswith("{")
        ]


def _successful_trace_line(trace: str, needle: str) -> str:
    for line in trace.splitlines():
        if needle in line and not re.search(r"= -1(?:\s|$)", line):
            return line
    pytest.fail(f"no successful syscall mentioning {needle!r} in trace:\n{trace}")


class _ReplyAlpha(socketserver.BaseRequestHandler):
    def handle(self) -> None:
        self.request.recv(4096)
        self.request.sendall(b"alpha\n")


@contextlib.contextmanager
def _alpha_server():
    with socketserver.TCPServer(("127.0.0.1", 0), _ReplyAlpha) as server:
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            yield server.server_address[1]
        finally:
            server.shutdown()
            thread.join(timeout=5)


@pytest.fixture(autouse=True)
def _stop_audit_daemons():
    del _BUCK_INSTANCES[:]
    yield
    seen = set()
    for buck in reversed(_BUCK_INSTANCES):
        if buck.isolation not in seen:
            buck.run("kill", check=False)
            seen.add(buck.isolation)
    del _BUCK_INSTANCES[:]


def _require_linux_audit_tools() -> None:
    if RUNTIME.is_relative_to(REPO):
        pytest.fail(
            f"KIBOU_HERMETICITY_RUNTIME must be outside the watched Buck cell: {RUNTIME}"
        )
    missing = [tool for tool in ("strace", "unshare") if shutil.which(tool) is None]
    if missing:
        pytest.fail("Linux hermeticity audit requires: " + ", ".join(missing))


def _sysctl(name: str) -> str:
    path = Path("/proc/sys") / name.replace(".", "/")
    try:
        return path.read_text().strip()
    except OSError:
        return "unavailable"


def _verify_staged_tools(buck: Buck, outputs: dict[str, Path]) -> None:
    for target, tool in outputs.items():
        assert tool.is_file(), f"{target} did not produce a file: {tool}"
        assert os.access(tool, os.X_OK), f"{target} is not executable: {tool}"
        version = subprocess.run(
            [tool, "-V=full"],
            cwd=REPO,
            text=True,
            capture_output=True,
            check=False,
        )
        output = version.stdout + version.stderr
        assert version.returncode == 0, output
        assert " version go" in output and "buildID=" in output, output

    staged_events = [
        event
        for event in buck.what_ran()
        if any(target in str(event.get("identity")) for target in outputs)
    ]
    assert staged_events, "the fresh isolation did not execute either staged target"
    assert all(event["reproducer"]["executor"] == "Local" for event in staged_events), (
        staged_events
    )


def test_local_executor_negative_controls() -> None:
    _require_linux_audit_tools()
    RUNTIME.mkdir(parents=True, exist_ok=True)
    (RUNTIME / "escape").unlink(missing_ok=True)

    # 1. A source-tree file omitted from attrs is readable, and the audit detects it.
    file_buck = Buck(f"hermetic-file-{os.getpid()}")
    file_out = file_buck.build_output(PACKAGE + "undeclared_file_read")
    assert (file_out / "value").read_text() == "undeclared-alpha\n"
    file_trace = (file_out / "trace").read_text(errors="replace")
    _successful_trace_line(file_trace, "build_defs/hermeticity/undeclared.txt")

    # 2. The action inherits buckd's environment, but a warm build does not notice
    # a changed client value.  A clean daemon/output state executes the same action
    # under the new value and produces different bytes.
    env_isolation = f"hermetic-env-{os.getpid()}"
    alpha_buck = Buck(env_isolation, secret="alpha")
    alpha_out = alpha_buck.build_output(PACKAGE + "inherited_environment")
    assert (alpha_out / "value").read_text() == "alpha\n"
    alpha_run = alpha_buck.what_ran()
    assert any(
        "hermeticity_probe environment" in str(item.get("identity"))
        for item in alpha_run
    )

    beta_client = Buck(env_isolation, secret="beta")
    warm_out = beta_client.build_output(PACKAGE + "inherited_environment")
    assert (warm_out / "value").read_text() == "alpha\n"
    assert not any(
        "hermeticity_probe environment" in str(item.get("identity"))
        for item in beta_client.what_ran()
    ), "the unchanged action should not have re-executed"

    beta_client.run("kill")
    beta_client.run("clean")
    beta_out = beta_client.build_output(PACKAGE + "inherited_environment")
    assert (beta_out / "value").read_text() == "beta\n"
    beta_run = beta_client.what_ran()
    assert any(
        "hermeticity_probe environment" in str(item.get("identity"))
        for item in beta_run
    )

    explicit_envs = []
    for event in alpha_run + beta_run:
        if "hermeticity_probe environment" in str(event.get("identity")):
            details = event["reproducer"]["details"]
            explicit_envs.append(details.get("env", {}))
    assert explicit_envs
    assert all("KIBOU_AUDIT_SECRET" not in env for env in explicit_envs)

    # 3. The explicit all-network executor reaches a host listener. network_access=none
    # enters a fresh Linux network namespace and the same connect is rejected.
    network_buck = Buck(f"hermetic-network-{os.getpid()}")
    preflight = subprocess.run(
        ["unshare", "--user", "--map-root-user", "--net", "true"],
        text=True,
        capture_output=True,
        check=False,
    )
    assert preflight.returncode == 0, (
        "unprivileged user/network namespaces are required by Buck2's local "
        "network policy. On Ubuntu 24.04, enable them for the ephemeral runner with:\n"
        "  sudo sysctl -w kernel.unprivileged_userns_clone=1\n"
        "  sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0\n"
        f"unprivileged_userns_clone={_sysctl('kernel.unprivileged_userns_clone')}\n"
        "apparmor_restrict_unprivileged_userns="
        f"{_sysctl('kernel.apparmor_restrict_unprivileged_userns')}\n"
        f"unshare stderr: {preflight.stderr}"
    )
    with _alpha_server() as port:
        (RUNTIME / "port").write_text(f"{port}\n")
        network_buck.run(
            "test",
            "--local-only",
            "--no-remote-cache",
            PACKAGE + "network_all",
        )
        unrestricted = (RUNTIME / "network-success.trace").read_text(errors="replace")
        assert re.search(r"connect\(.*AF_INET.*\)\s+=\s+0", unrestricted), unrestricted

        network_buck.run(
            "test",
            "--local-only",
            "--no-remote-cache",
            PACKAGE + "network_none",
        )
        restricted = (RUNTIME / "network-blocked.trace").read_text(errors="replace")
        assert re.search(
            r"connect\(.*AF_INET.*\)\s+=\s+-1\s+"
            r"(?:ENETUNREACH|EHOSTUNREACH|ENETDOWN|ECONNREFUSED|EACCES|EPERM)",
            restricted,
        ), restricted

    status = json.loads(network_buck.run("status").stdout)
    assert status.get("forkserver_pid") is not None, (
        "network policy is ignored without Buck2's Linux forkserver"
    )

    # 4. A local action can write beyond its declared output.  The file and trace
    # together prove the escape and give the detector a stable negative control.
    write_buck = Buck(f"hermetic-write-{os.getpid()}")
    write_out = write_buck.build_output(PACKAGE + "undeclared_write")
    escape = RUNTIME / "escape"
    assert escape.read_text() == "escaped\n"
    write_trace = (write_out / "trace").read_text(errors="replace")
    line = _successful_trace_line(write_trace, str(escape))
    assert "O_WRONLY" in line, line
    escape.unlink()


_QUOTED = re.compile(r'"((?:\\.|[^"\\])*)"')
_WRITE_FLAGS = ("O_WRONLY", "O_RDWR", "O_CREAT", "O_TRUNC", "O_APPEND")
_WRITE_SYSCALLS = {
    "chmod",
    "fchmodat",
    "link",
    "linkat",
    "mkdir",
    "mkdirat",
    "rename",
    "renameat",
    "renameat2",
    "rmdir",
    "symlink",
    "symlinkat",
    "truncate",
    "unlink",
    "unlinkat",
}


def _decode_quoted(blob: str) -> list[str]:
    decoded = []
    for value in _QUOTED.findall(blob):
        try:
            decoded.append(ast.literal_eval(f'"{value}"'))
        except (SyntaxError, ValueError):
            decoded.append(value)
    return decoded


def _execve_records(trace: str) -> list[tuple[list[str], dict[str, str]]]:
    records = []
    for line in trace.splitlines():
        if "execve(" not in line or not re.search(r"\)\s+=\s+0$", line):
            continue
        match = re.search(r"execve\([^,]+, \[(.*)\], \[(.*)\]\)\s+=\s+0$", line)
        if not match:
            continue
        argv = _decode_quoted(match.group(1))
        environment = {}
        for entry in _decode_quoted(match.group(2)):
            key, separator, value = entry.partition("=")
            if separator:
                environment[key] = value
        records.append((argv, environment))
    return records


def _lexical_path(value: str, *, base: Path = REPO) -> Path:
    path = Path(value)
    if not path.is_absolute():
        path = base / path
    return Path(os.path.normpath(path))


def _path_variants(value: str, *, base: Path = REPO) -> set[Path]:
    lexical = _lexical_path(value, base=base)
    return {lexical, lexical.resolve(strict=False)}


def _add_manifest_inputs(path: Path, reads: set[Path]) -> None:
    if not path.is_file():
        return
    try:
        tokens = shlex.split(path.read_text())
    except (OSError, UnicodeDecodeError, ValueError):
        return
    for token in tokens:
        candidate = token.split("=", 1)[-1] if "=" in token else token
        if "/" in candidate or candidate.startswith("."):
            reads.update(_path_variants(candidate))


def _declared_paths(
    argv: list[str], environment: dict[str, str]
) -> tuple[set[Path], set[Path], set[Path], set[Path]]:
    reads: set[Path] = set()
    read_directories: set[Path] = set()
    writes: set[Path] = set()
    write_directories: set[Path] = set()
    output_flags = {"-o", "-linkobj"}
    directory_flags = {"-I"}

    for index, token in enumerate(argv):
        value = token.removeprefix("@")
        if token.startswith("-") and not token.startswith("@"):
            continue
        if value in {"", "%cwd%"}:
            continue
        candidate = value.split("=", 1)[-1] if "=" in value else value
        if "/" not in candidate and not candidate.startswith("."):
            continue
        variants = _path_variants(candidate)
        reads.update(variants)
        if token.startswith("@") or candidate.endswith(".importcfg"):
            for path in variants:
                _add_manifest_inputs(path, reads)
        previous = argv[index - 1] if index else ""
        if previous in output_flags:
            writes.update(variants)
        elif previous in directory_flags:
            read_directories.update(variants)

    for variable in ("GOROOT",):
        if environment.get(variable):
            read_directories.update(_path_variants(environment[variable]))
    for variable in ("TMPDIR", "BUCK_SCRATCH_PATH"):
        if environment.get(variable):
            write_directories.update(_path_variants(environment[variable]))

    return reads, read_directories, writes, write_directories


def _trace_paths(line: str) -> list[Path]:
    syscall = re.match(r"([a-zA-Z0-9_]+)\(", line)
    if not syscall:
        return []
    quoted = _decode_quoted(line)
    if not quoted:
        return []
    if syscall.group(1) == "execve":
        quoted = quoted[:1]

    dirfd = re.match(r"[a-zA-Z0-9_]+\([^<]*<([^>]+)>,", line)
    base = Path(dirfd.group(1)) if dirfd else REPO
    paths = []
    for value in quoted:
        if value.startswith(("AF_", "PF_")) or "=" in value:
            continue
        paths.append(_lexical_path(value, base=base))
    return paths


def _within(path: Path, directory: Path) -> bool:
    return path == directory or directory in path.parents


def _is_ancestor(path: Path, candidates: set[Path]) -> bool:
    return any(
        path == candidate or path in candidate.parents for candidate in candidates
    )


def _audit_action_trace(path: Path) -> dict[str, object] | None:
    trace = path.read_text(errors="replace")
    records = _execve_records(trace)
    wrapper = next((record for record in records if "--go" in record[0]), None)
    if wrapper is None:
        return None

    argv, environment = wrapper
    reads, read_directories, writes, write_directories = _declared_paths(
        argv, environment
    )
    # Buck starts local actions without stdin. The Go runtime opens this stable
    # device to repair the missing standard descriptor before running the tool.
    # Keep this exception exact: mutable host configuration under /dev, /proc,
    # and /sys remains undeclared and therefore fails the audit.
    reads.add(Path("/dev/null"))
    # The wrapper execs the selected Go tool in-place. Include its argv as an
    # independent source of path-bearing flags in case the wrapper rewrote them.
    for child_argv, child_environment in records:
        child = _declared_paths(child_argv, child_environment)
        reads.update(child[0])
        read_directories.update(child[1])
        writes.update(child[2])
        write_directories.update(child[3])

    violations = []
    observed = []
    for line in trace.splitlines():
        if re.search(r"= -1(?:\s|$)", line):
            continue
        syscall = re.match(r"([a-zA-Z0-9_]+)\(", line)
        if not syscall:
            continue
        is_write = syscall.group(1) in _WRITE_SYSCALLS or any(
            flag in line for flag in _WRITE_FLAGS
        )
        for traced in _trace_paths(line):
            variants = {traced, traced.resolve(strict=False)}
            observed.append(str(traced))
            if is_write:
                allowed = (
                    bool(variants & writes)
                    or (
                        syscall.group(1) in {"mkdir", "mkdirat"}
                        and bool(variants & {output.parent for output in writes})
                    )
                    or any(
                        _within(item, directory)
                        for item in variants
                        for directory in write_directories
                    )
                )
            else:
                allowed = (
                    bool(variants & reads)
                    or any(
                        _within(item, directory)
                        for item in variants
                        for directory in read_directories
                    )
                    or any(
                        _is_ancestor(item, reads | read_directories)
                        for item in variants
                    )
                )
            if not allowed:
                violations.append(
                    {
                        "access": "write" if is_write else "read",
                        "path": str(traced),
                        "syscall": line,
                    }
                )

    return {
        "trace": path.name,
        "argv": argv,
        "observed_paths": sorted(set(observed)),
        "violations": violations,
    }


def test_staged_go_actions_are_hermetic_under_strace() -> None:
    """Audit actual staged Go actions against their command-level manifests."""
    _require_linux_audit_tools()
    _generate_native_graph()
    if shutil.which("sudo") is None:
        pytest.fail("attaching strace to Buck2's forkserver requires passwordless sudo")
    if subprocess.run(
        ["sudo", "-n", "true"],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        check=False,
    ).returncode:
        pytest.fail("attaching strace to Buck2's forkserver requires passwordless sudo")

    RUNTIME.mkdir(parents=True, exist_ok=True)
    buck = Buck(f"hermetic-staged-{os.getpid()}")
    buck.run("uquery", "//:stage1_goroot")
    status = json.loads(buck.run("status").stdout)
    forkserver_pid = status.get("forkserver_pid")
    assert forkserver_pid is not None

    trace_prefix = RUNTIME / "staged.trace"
    for old_trace in RUNTIME.glob("staged.trace*"):
        old_trace.unlink()
    tracer = subprocess.Popen(
        [
            "sudo",
            "-n",
            "strace",
            "-ff",
            "-qq",
            "-yy",
            "-v",
            "-s",
            "65535",
            "-e",
            "trace=%file,%network,execve",
            "-o",
            str(trace_prefix),
            "-p",
            str(forkserver_pid),
        ],
        cwd=REPO,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.PIPE,
        text=True,
        start_new_session=True,
    )
    try:
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline and not list(RUNTIME.glob("staged.trace*")):
            if tracer.poll() is not None:
                pytest.fail(
                    "strace attach failed: "
                    + (tracer.stderr.read() if tracer.stderr else "")
                )
            time.sleep(0.1)
        outputs = buck.build_outputs(
            "//go/src/cmd/compile:stage3",
            "//go/src/cmd/link:stage3",
        )
    finally:
        subprocess.run(
            ["sudo", "-n", "/bin/kill", "-INT", "--", f"-{tracer.pid}"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )
        try:
            tracer.wait(timeout=10)
        except subprocess.TimeoutExpired:
            subprocess.run(
                ["sudo", "-n", "/bin/kill", "-KILL", "--", f"-{tracer.pid}"],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                check=False,
            )
            tracer.wait(timeout=5)

    _verify_staged_tools(buck, outputs)

    traces = list(RUNTIME.glob("staged.trace*"))
    combined = "\n".join(path.read_text(errors="replace") for path in traces)
    assert not re.search(
        r"(?:connect|sendto|recvfrom)\(.*AF_(?:INET|INET6)", combined
    ), "a staged Go action attempted network I/O"

    audits = [audit for path in traces if (audit := _audit_action_trace(path))]
    (RUNTIME / "staged-audit-report.json").write_text(
        json.dumps(audits, indent=2, sort_keys=True) + "\n"
    )
    assert audits, "no staged Go wrapper process was captured"
    violations = [violation for audit in audits for violation in audit["violations"]]
    assert not violations, json.dumps(violations[:20], indent=2)
