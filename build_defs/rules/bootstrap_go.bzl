load(
    "@prelude//go:package_builder.bzl",
    "BuildPackageGoList",
    "BuildPackageParams",
    "build_package",
)
load(
    "@prelude//go:packages.bzl",
    "GoPkg",
    "GoStdlib",
    "GoStdlibDynamicValue",
    "make_link_importcfg",
    "merge_pkgs",
)
load(
    "@prelude//go:toolchain.bzl",
    "GoToolchainInfo",
    "get_toolchain_env_vars",
)

BootstrapGoPackageInfo = provider(fields = {
    "own": provider_field(typing.Any),
    "pkgs": provider_field(typing.Any),
})

def _disable_cgo_impl(*, platform: PlatformInfo, refs: struct, attrs: struct) -> PlatformInfo:
    configuration = platform.configuration
    configuration.insert(refs.cgo_disabled[ConstraintValueInfo])
    return PlatformInfo(
        label = platform.label,
        configuration = configuration,
    )

_disable_cgo = transition(
    impl = _disable_cgo_impl,
    refs = {
        "cgo_disabled": "prelude//go/constraints:cgo_enabled[false]",
    },
    attrs = [],
)

def _generated_go_source_impl(ctx):
    out = ctx.actions.write(
        ctx.attrs.out,
        ctx.attrs.content,
        has_content_based_path = True,
    )
    return [DefaultInfo(default_output = out)]

generated_go_source = rule(
    impl = _generated_go_source_impl,
    attrs = {
        "content": attrs.string(),
        "out": attrs.string(),
    },
)

def _bootstrap_package_action_impl(
        actions: AnalysisActions,
        target_label: Label,
        go_toolchain: GoToolchainInfo,
        stage0_stdlib_value: ResolvedDynamicValue,
        go_list: BuildPackageGoList,
        pkg_import_path: str,
        package_root: str,
        main: bool,
        embed_srcs: dict[str, Artifact],
        compiler_flags: list[str],
        assembler_flags: list[str],
        deps_pkgs: dict[str, GoPkg],
        import_map: dict[str, str],
        out_a: OutputArtifact,
        out_x: OutputArtifact,
        out_a_shared: OutputArtifact,
        out_x_shared: OutputArtifact):
    stage0_stdlib = stage0_stdlib_value.providers[GoStdlibDynamicValue]
    result = build_package(
        actions = actions,
        target_label = target_label,
        go_toolchain = go_toolchain,
        cgo_build_context = None,
        go_list = go_list,
        params = BuildPackageParams(
            main = main,
            standard = True,
            pkg_import_path = pkg_import_path,
            package_root = package_root,
            embed_srcs = embed_srcs,
            compiler_flags = compiler_flags,
            assembler_flags = assembler_flags,
            coverage_enabled = False,
            coverage_mode = None,
            deps = merge_pkgs([stage0_stdlib.pkgs, deps_pkgs]),
            import_map = import_map,
        ),
    )
    actions.copy_file(out_a, result.a_file)
    actions.copy_file(out_x, result.x_file)
    actions.copy_file(out_a_shared, result.a_file_shared)
    actions.copy_file(out_x_shared, result.x_file_shared)
    return []

_bootstrap_package_action = dynamic_actions(
    impl = _bootstrap_package_action_impl,
    attrs = {
        "assembler_flags": dynattrs.value(list[str]),
        "compiler_flags": dynattrs.value(list[str]),
        "deps_pkgs": dynattrs.value(dict[str, GoPkg]),
        "embed_srcs": dynattrs.value(dict[str, Artifact]),
        "go_list": dynattrs.value(BuildPackageGoList),
        "go_toolchain": dynattrs.value(GoToolchainInfo),
        "import_map": dynattrs.value(dict[str, str]),
        "main": dynattrs.value(bool),
        "out_a": dynattrs.output(),
        "out_a_shared": dynattrs.output(),
        "out_x": dynattrs.output(),
        "out_x_shared": dynattrs.output(),
        "package_root": dynattrs.value(str),
        "pkg_import_path": dynattrs.value(str),
        "stage0_stdlib_value": dynattrs.dynamic_value(),
        "target_label": dynattrs.value(Label),
    },
)

def _bootstrap_go_package_impl(ctx):
    deps_pkgs = merge_pkgs([dep[BootstrapGoPackageInfo].pkgs for dep in ctx.attrs.deps])

    out_a = ctx.actions.declare_output(ctx.label.name + ".a", has_content_based_path = True)
    out_x = ctx.actions.declare_output(ctx.label.name + ".x", has_content_based_path = True)
    out_a_shared = ctx.actions.declare_output(ctx.label.name + "_shared.a", has_content_based_path = True)
    out_x_shared = ctx.actions.declare_output(ctx.label.name + "_shared.x", has_content_based_path = True)

    own = GoPkg(
        archive_file = out_a,
        archive_file_shared = out_a_shared,
        export_file = out_x,
        export_file_shared = out_x_shared,
        coverage_instrumented = False,
    )

    ctx.actions.dynamic_output_new(
        _bootstrap_package_action(
            target_label = ctx.label,
            go_toolchain = ctx.attrs.compiler_toolchain[GoToolchainInfo],
            stage0_stdlib_value = ctx.attrs.stdlib[GoStdlib].dynamic_value,
            go_list = BuildPackageGoList(
                pkg_name = ctx.attrs.package_name,
                go_files = ctx.attrs.go_srcs,
                cgo_files = [],
                s_files = ctx.attrs.asm_srcs,
                h_files = ctx.attrs.headers,
                c_cxx_files = [],
                syso_files = ctx.attrs.syso_srcs,
                imports = set(ctx.attrs.imports),
                embed_patterns = ctx.attrs.embed_patterns,
                cgo_cflags = [],
                cgo_cppflags = [],
            ),
            pkg_import_path = ctx.attrs.pkg_import_path,
            package_root = ctx.attrs.package_root,
            main = ctx.attrs.main,
            embed_srcs = ctx.attrs.embed_srcs,
            compiler_flags = ctx.attrs.compiler_flags,
            assembler_flags = ctx.attrs.assembler_flags,
            deps_pkgs = deps_pkgs,
            import_map = ctx.attrs.import_map,
            out_a = out_a.as_output(),
            out_x = out_x.as_output(),
            out_a_shared = out_a_shared.as_output(),
            out_x_shared = out_x_shared.as_output(),
        )
    )

    return [
        DefaultInfo(default_output = out_a, other_outputs = [out_x, out_a_shared, out_x_shared]),
        BootstrapGoPackageInfo(
            own = own,
            pkgs = merge_pkgs([deps_pkgs, {ctx.attrs.pkg_import_path: own}]),
        ),
    ]

bootstrap_go_package = rule(
    impl = _bootstrap_go_package_impl,
    cfg = _disable_cgo,
    attrs = {
        "asm_srcs": attrs.list(attrs.source(), default = []),
        "assembler_flags": attrs.list(attrs.string(), default = []),
        "compiler_flags": attrs.list(attrs.string(), default = []),
        "deps": attrs.list(attrs.dep(providers = [BootstrapGoPackageInfo]), default = []),
        "embed_patterns": attrs.list(attrs.string(), default = []),
        "embed_srcs": attrs.dict(attrs.string(), attrs.source(), default = {}),
        "go_srcs": attrs.list(attrs.source(), default = []),
        "headers": attrs.list(attrs.source(), default = []),
        "import_map": attrs.dict(attrs.string(), attrs.string(), default = {}),
        "imports": attrs.list(attrs.string(), default = []),
        "main": attrs.bool(default = False),
        "package_name": attrs.string(),
        "package_root": attrs.string(),
        "pkg_import_path": attrs.string(),
        "syso_srcs": attrs.list(attrs.source(), default = []),
        "stdlib": attrs.dep(
            default = "prelude//go/tools:stdlib",
            providers = [GoStdlib],
        ),
        "compiler_toolchain": attrs.toolchain_dep(
            default = "toolchains//:go",
            providers = [GoToolchainInfo],
        ),
    },
)

def _bootstrap_link_action_impl(
        actions: AnalysisActions,
        go_toolchain: GoToolchainInfo,
        stage0_stdlib_value: ResolvedDynamicValue,
        main_pkg: GoPkg,
        deps_pkgs: dict[str, GoPkg],
        identifier: str,
        out: OutputArtifact):
    stage0_stdlib = stage0_stdlib_value.providers[GoStdlibDynamicValue]
    all_pkgs = merge_pkgs([stage0_stdlib.pkgs, deps_pkgs])
    importcfg = make_link_importcfg(actions, all_pkgs, False)
    cmd = cmd_args(
        go_toolchain.go_wrapper,
        ["--go", go_toolchain.linker],
        "--",
        go_toolchain.linker_flags,
        "-buildmode=exe",
        "-buildid=",
        "-linkmode=internal",
        ["-importcfg", importcfg],
        ["-o", out],
        main_pkg.archive_file,
    )
    actions.run(
        cmd,
        env = get_toolchain_env_vars(go_toolchain),
        category = "bootstrap_go_link",
        identifier = identifier,
        allow_cache_upload = go_toolchain.allow_cache_upload,
    )
    return []

_bootstrap_link_action = dynamic_actions(
    impl = _bootstrap_link_action_impl,
    attrs = {
        "deps_pkgs": dynattrs.value(dict[str, GoPkg]),
        "go_toolchain": dynattrs.value(GoToolchainInfo),
        "identifier": dynattrs.value(str),
        "main_pkg": dynattrs.value(GoPkg),
        "out": dynattrs.output(),
        "stage0_stdlib_value": dynattrs.dynamic_value(),
    },
)

def _bootstrap_go_binary_impl(ctx):
    go_toolchain = ctx.attrs.compiler_toolchain[GoToolchainInfo]
    main = ctx.attrs.main[BootstrapGoPackageInfo]
    suffix = ".exe" if go_toolchain.env_go_os == "windows" else ""
    out = ctx.actions.declare_output(ctx.label.name + suffix, has_content_based_path = True)
    ctx.actions.dynamic_output_new(
        _bootstrap_link_action(
            go_toolchain = go_toolchain,
            stage0_stdlib_value = ctx.attrs.stdlib[GoStdlib].dynamic_value,
            main_pkg = main.own,
            deps_pkgs = main.pkgs,
            identifier = ctx.label.name,
            out = out.as_output(),
        )
    )
    return [DefaultInfo(default_output = out), RunInfo(args = cmd_args(out))]

bootstrap_go_binary = rule(
    impl = _bootstrap_go_binary_impl,
    cfg = _disable_cgo,
    attrs = {
        "main": attrs.dep(providers = [BootstrapGoPackageInfo]),
        "stdlib": attrs.dep(
            default = "prelude//go/tools:stdlib",
            providers = [GoStdlib],
        ),
        "compiler_toolchain": attrs.toolchain_dep(
            default = "toolchains//:go",
            providers = [GoToolchainInfo],
        ),
    },
)
