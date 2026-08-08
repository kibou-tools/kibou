def _host_platform_identity():
    host = host_info()
    if host.os.is_macos:
        os_family = "macos"
    elif host.os.is_windows:
        os_family = "windows"
    else:
        os_family = "linux"

    if host.arch.is_aarch64:
        isa = "arm64"
    elif host.arch.is_x86_64:
        isa = "x86_64"
    else:
        fail("unsupported Buck2 prototype host architecture: {}".format(host.arch))
    return os_family, isa

def _execution_platforms_impl(ctx):
    os_family, isa = _host_platform_identity()
    platform = ExecutionPlatformInfo(
        label = ctx.label.raw_target(),
        configuration = ctx.attrs.platform[PlatformInfo].configuration,
        executor_config = CommandExecutorConfig(
            allow_cache_uploads = read_config("kibou", "allow_cache_uploads", "false") == "true",
            local_enabled = True,
            remote_cache_enabled = read_config("kibou", "remote_cache_enabled", "false") == "true",
            remote_enabled = False,
            remote_execution_action_key = "kibou-local-{}-{}-v1".format(os_family, isa),
            remote_execution_properties = {
                "ISA": isa,
                "OSFamily": os_family,
            },
        ),
    )

    return [
        DefaultInfo(),
        ExecutionPlatformRegistrationInfo(platforms = [platform]),
    ]

execution_platforms = rule(
    impl = _execution_platforms_impl,
    attrs = {
        "platform": attrs.dep(
            default = "prelude//platforms:default",
            providers = [PlatformInfo],
        ),
    },
)
