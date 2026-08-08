def _audit_action_impl(ctx):
    out = ctx.actions.declare_output(ctx.attrs.mode, dir = True)
    command = cmd_args(
        "/bin/sh",
        "-c",
        """
set -eu
out="$1"
probe="$2"
mode="$3"
mkdir -p "$out"
exec /usr/bin/strace -f -qq -s 4096 \
  -e trace=%file,%network,execve \
  -o "$out/trace" \
  /bin/bash "$probe" "$mode" "$out"
""",
        "kibou-hermeticity-audit",
        out.as_output(),
        ctx.attrs.probe,
        ctx.attrs.mode,
    )
    ctx.actions.run(
        command,
        category = "hermeticity_probe",
        identifier = ctx.attrs.mode,
        local_only = True,
        allow_cache_upload = False,
    )
    return [DefaultInfo(default_output = out)]

audit_action = rule(
    impl = _audit_action_impl,
    attrs = {
        "mode": attrs.enum(["file", "environment", "write"]),
        "probe": attrs.source(),
    },
)

def _network_audit_test_impl(ctx):
    runtime = read_root_config("kibou", "hermeticity_runtime", "")
    if not runtime:
        fail("set kibou.hermeticity_runtime to an absolute directory outside the Buck cell")
    executor = CommandExecutorConfig(
        local_enabled = True,
        remote_enabled = False,
        remote_cache_enabled = False,
        allow_cache_uploads = False,
        network_access = ctx.attrs.network_access,
    )
    return [
        DefaultInfo(),
        ExternalRunnerTestInfo(
            type = "hermeticity_audit",
            command = [ctx.attrs.test, ctx.attrs.expected, runtime],
            env = {},
            labels = [],
            contacts = [],
            default_executor = executor,
            executor_overrides = {},
            run_from_project_root = True,
            use_project_relative_paths = True,
            supports_test_execution_caching = False,
        ),
    ]

network_audit_test = rule(
    impl = _network_audit_test_impl,
    attrs = {
        "expected": attrs.enum(["success", "blocked"]),
        "network_access": attrs.enum(["all", "none"]),
        "test": attrs.source(),
    },
)
