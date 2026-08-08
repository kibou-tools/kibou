load("@prelude//go:go_list_stdlib.bzl", "go_list_stdlib", "parse_go_list_stdlib_out")
load(
    "@prelude//go:package_builder.bzl",
    "BuildPackageParams",
    "build_package",
    "go_list_for_build",
)
load(
    "@prelude//go:packages.bzl",
    "GoPkg",
    "GoStdlib",
    "GoStdlibDynamicValue",
    "implicit_imports",
)
load("@prelude//go:toolchain.bzl", "GoToolchainInfo")
load("@prelude//utils:graph_utils.bzl", "post_order_traversal")

def _staged_go_stdlib_impl(ctx):
    go_toolchain = ctx.attrs.compiler_toolchain[GoToolchainInfo]
    go_list_out = go_list_stdlib(ctx.actions, go_toolchain, False)
    pkgdir = ctx.actions.declare_output("pkgdir", dir = True, has_content_based_path = True)
    value = ctx.actions.dynamic_output_new(
        _build_staged_stdlib(
            go_list_out = go_list_out,
            go_root = go_toolchain.env_go_root,
            go_toolchain = go_toolchain,
            pkgdir = pkgdir.as_output(),
            roots = ctx.attrs.roots,
            target_label = ctx.label,
        )
    )
    return [
        DefaultInfo(default_output = pkgdir),
        GoStdlib(dynamic_value = value),
    ]

def _build_staged_stdlib_impl(
        actions: AnalysisActions,
        target_label: Label,
        go_toolchain: GoToolchainInfo,
        go_list_out: ArtifactValue,
        go_root: Artifact,
        roots: list[str],
        pkgdir: OutputArtifact):
    parsed = {}
    for item in go_list_out.read_json():
        package = parse_go_list_stdlib_out(go_root, item)
        if len(package.go_list.go_files) == 0 and len(package.go_list.cgo_files) == 0:
            continue
        if package.import_path in ["unsafe", "builtin"]:
            continue
        parsed[package.import_path] = package

    needed = set(roots)

    graph = {}
    for import_path in needed:
        if import_path not in parsed:
            continue
        package = parsed[import_path]
        graph[import_path] = [
            imported
            for imported in package.go_list.imports
            if imported in needed and imported in parsed
        ]

    packages = {}
    for import_path in post_order_traversal(graph):
        package = parsed[import_path]
        packages[import_path] = _declare_staged_stdlib_package(
            actions = actions,
            target_label = target_label,
            go_toolchain = go_toolchain,
            go_list = go_list_for_build(package.go_list, False),
            params = BuildPackageParams(
                main = False,
                standard = True,
                pkg_import_path = import_path,
                package_root = go_root.short_path + "/src/" + import_path,
                embed_srcs = {src.short_path: src for src in package.embed_files},
                compiler_flags = [],
                assembler_flags = [],
                coverage_enabled = False,
                coverage_mode = None,
                deps = packages,
                import_map = package.import_map,
            ),
        )

    actions.copied_dir(
        pkgdir,
        {path + ".a": package.archive_file for path, package in packages.items()} |
        {path + ".x": package.export_file for path, package in packages.items()},
    )
    return [GoStdlibDynamicValue(pkgs = packages)]

_build_staged_stdlib = dynamic_actions(
    impl = _build_staged_stdlib_impl,
    attrs = {
        "go_list_out": dynattrs.artifact_value(),
        "go_root": dynattrs.value(Artifact),
        "go_toolchain": dynattrs.value(GoToolchainInfo),
        "pkgdir": dynattrs.output(),
        "roots": dynattrs.value(list[str]),
        "target_label": dynattrs.value(Label),
    },
)

def _declare_staged_stdlib_package(actions, target_label, go_toolchain, go_list, params):
    out_a = actions.declare_output(params.pkg_import_path + "_non-shared.a", has_content_based_path = True)
    out_x = actions.declare_output(params.pkg_import_path + "_non-shared.x", has_content_based_path = True)
    out_a_shared = actions.declare_output(params.pkg_import_path + "_shared.a", has_content_based_path = True)
    out_x_shared = actions.declare_output(params.pkg_import_path + "_shared.x", has_content_based_path = True)
    actions.dynamic_output_new(
        _build_staged_stdlib_package(
            go_list = go_list,
            go_toolchain = go_toolchain,
            out_a = out_a.as_output(),
            out_a_shared = out_a_shared.as_output(),
            out_x = out_x.as_output(),
            out_x_shared = out_x_shared.as_output(),
            params = params,
            target_label = target_label,
        )
    )
    return GoPkg(
        archive_file = out_a,
        archive_file_shared = out_a_shared,
        export_file = out_x,
        export_file_shared = out_x_shared,
        coverage_instrumented = False,
    )

def _build_staged_stdlib_package_impl(
        actions: AnalysisActions,
        target_label: Label,
        go_toolchain: GoToolchainInfo,
        go_list,
        params: BuildPackageParams,
        out_a: OutputArtifact,
        out_x: OutputArtifact,
        out_a_shared: OutputArtifact,
        out_x_shared: OutputArtifact):
    result = build_package(
        actions = actions,
        target_label = target_label,
        go_toolchain = go_toolchain,
        cgo_build_context = None,
        go_list = go_list,
        params = params,
    )
    actions.copy_file(out_a, result.a_file)
    actions.copy_file(out_x, result.x_file)
    actions.copy_file(out_a_shared, result.a_file_shared)
    actions.copy_file(out_x_shared, result.x_file_shared)
    return []

_build_staged_stdlib_package = dynamic_actions(
    impl = _build_staged_stdlib_package_impl,
    attrs = {
        "go_list": dynattrs.value(typing.Any),
        "go_toolchain": dynattrs.value(GoToolchainInfo),
        "out_a": dynattrs.output(),
        "out_a_shared": dynattrs.output(),
        "out_x": dynattrs.output(),
        "out_x_shared": dynattrs.output(),
        "params": dynattrs.value(BuildPackageParams),
        "target_label": dynattrs.value(Label),
    },
)

staged_go_stdlib = rule(
    impl = _staged_go_stdlib_impl,
    attrs = {
        "compiler_toolchain": attrs.toolchain_dep(providers = [GoToolchainInfo]),
        "roots": attrs.list(attrs.string()),
    },
)
