load("@prelude//go:compile.bzl", "GoPkgCompileInfo", "get_inherited_compile_pkgs")
load("@prelude//go:link.bzl", "GoBuildMode", "GoPkgLinkInfo", "get_inherited_link_pkgs", "link")
load(
    "@prelude//go:package_builder.bzl",
    "GoBuildConfig",
    "GoSourceInputs",
    "declare_package_build",
)
load("@prelude//go:packages.bzl", "GoPkg", "GoStdlib", "merge_pkgs")
load("@prelude//go:toolchain.bzl", "GoToolchainInfo")
load("@prelude//linking:link_info.bzl", "LinkStyle")

KibouGoLibraryNode = record(
    package_name = field(str),
    sources = field(GoSourceInputs),
    direct_deps = field(list[str]),
    inherited_pkgs = field(typing.Any),  # dict[str, GoPkg]
)

KibouGoLibraryInfo = provider(
    fields = {
        "package_name": provider_field(str),
        # Topologically ordered, de-duplicated package metadata. Tests use this
        # to reproduce cmd/go's test-copy closure without collapsing packages
        # into a single compile action.
        "nodes": provider_field(typing.Any),  # list[KibouGoLibraryNode]
    },
)

_TestCopyOutput = provider(
    fields = {
        "pkg": provider_field(typing.Any),  # GoPkg
    },
)

def _deserialize_go_pkgs(values):
    return {
        name: GoPkg(
            archive_file = value[0],
            archive_file_shared = value[1],
            export_file = value[2],
            export_file_shared = value[3],
            coverage_instrumented = value[4],
        )
        for name, value in values.items()
    }

def _test_copy_impl(ctx):
    pkg, _, _ = declare_package_build(
        ctx = ctx,
        pkg_import_path = ctx.attrs.package_name,
        main = False,
        sources = GoSourceInputs(
            srcs = ctx.attrs.srcs,
            embed_srcs = ctx.attrs.embed_srcs,
            package_root = ctx.attrs.package_root,
        ),
        cgo_build_context = None,
        config = GoBuildConfig(cgo_enabled = False),
        pkgs = _deserialize_go_pkgs(ctx.attrs.pkgs),
    )
    return [DefaultInfo(), _TestCopyOutput(pkg = pkg)]

_test_copy_rule = anon_rule(
    impl = _test_copy_impl,
    attrs = {
        "embed_srcs": attrs.dict(attrs.string(), attrs.source()),
        "package_name": attrs.string(),
        "package_root": attrs.option(attrs.string(), default = None),
        "pkgs": attrs.dict(
            attrs.string(),
            attrs.tuple(
                attrs.source(),
                attrs.source(),
                attrs.source(),
                attrs.source(),
                attrs.bool(),
            ),
        ),
        "srcs": attrs.list(attrs.source()),
        "_go_stdlib": attrs.dep(providers = [GoStdlib]),
        "_go_toolchain": attrs.dep(providers = [GoToolchainInfo]),
    },
    artifact_promise_mappings = {
        "archive_file": lambda providers: providers[_TestCopyOutput].pkg.archive_file,
        "archive_file_shared": lambda providers: providers[_TestCopyOutput].pkg.archive_file_shared,
        "export_file": lambda providers: providers[_TestCopyOutput].pkg.export_file,
        "export_file_shared": lambda providers: providers[_TestCopyOutput].pkg.export_file_shared,
    },
)

def _go_sources(ctx, srcs):
    return GoSourceInputs(
        srcs = srcs,
        embed_srcs = ctx.attrs.embed_srcs,
        package_root = ctx.attrs.package_root,
    )

def _kibou_go_library_impl(ctx):
    sources = _go_sources(ctx, ctx.attrs.srcs)
    inherited_compile = get_inherited_compile_pkgs(ctx.attrs.deps)
    pkg, pkg_info, _ = declare_package_build(
        ctx = ctx,
        pkg_import_path = ctx.attrs.package_name,
        main = False,
        sources = sources,
        cgo_build_context = None,
        config = GoBuildConfig(cgo_enabled = False),
        deps = ctx.attrs.deps,
    )
    own = {ctx.attrs.package_name: pkg}
    nodes = []
    seen_nodes = set()
    direct_deps = []
    for dep in ctx.attrs.deps:
        if KibouGoLibraryInfo not in dep:
            continue
        info = dep[KibouGoLibraryInfo]
        direct_deps.append(info.package_name)
        for node in info.nodes:
            if node.package_name not in seen_nodes:
                seen_nodes.add(node.package_name)
                nodes.append(node)
    nodes.append(KibouGoLibraryNode(
        package_name = ctx.attrs.package_name,
        sources = sources,
        direct_deps = direct_deps,
        inherited_pkgs = inherited_compile,
    ))
    return [
        DefaultInfo(default_output = pkg.archive_file, other_outputs = [pkg.export_file]),
        GoPkgCompileInfo(pkgs = own),
        GoPkgLinkInfo(pkgs = merge_pkgs([own, get_inherited_link_pkgs(ctx.attrs.deps)])),
        pkg_info,
        KibouGoLibraryInfo(
            package_name = ctx.attrs.package_name,
            nodes = nodes,
        ),
    ]

def _build_go_binary(ctx, linker_flags):
    main, _, _ = declare_package_build(
        ctx = ctx,
        pkg_import_path = ctx.attrs.package_name,
        main = True,
        sources = _go_sources(ctx, ctx.attrs.srcs),
        cgo_build_context = None,
        config = GoBuildConfig(cgo_enabled = False),
        deps = ctx.attrs.deps,
    )
    inherited_link = get_inherited_link_pkgs(ctx.attrs.deps)
    pkgs = merge_pkgs([inherited_link, {ctx.attrs.package_name: main}])
    binary, runtime_files, external_debug_info = link(
        ctx = ctx,
        main = main,
        cgo_enabled = False,
        pkgs = pkgs,
        deps = [],
        link_style = LinkStyle("static"),
        build_mode = GoBuildMode("exe"),
        linker_flags = linker_flags,
    )
    run_cmd = cmd_args(binary, hidden = runtime_files + external_debug_info)
    return [
        DefaultInfo(
            default_output = binary,
            other_outputs = runtime_files + external_debug_info,
        ),
        RunInfo(args = run_cmd),
    ]

def _kibou_go_binary_impl(ctx):
    return _build_go_binary(ctx, [])

def _release_version():
    version = read_config("kibou", "release_version", "")
    if not version:
        fail("release target requires -c kibou.release_version=<version>")
    if not version.startswith("go") or "\n" in version or "\r" in version or "\0" in version:
        fail("invalid Kibou Go release version: {}".format(version))
    return version

def _kibou_go_release_binary_impl(ctx):
    return _build_go_binary(ctx, [
        "-X",
        "runtime.buildVersion=" + _release_version(),
    ])

def _named_entries(package, names):
    return [
        '    {{"{}", {}.{}}},'.format(name, package, name)
        for name in names
    ]

def _go_quote(value):
    return '"{}"'.format(value.replace("\\", "\\\\").replace('"', '\\"').replace("\n", "\\n").replace("\r", "\\r").replace("\t", "\\t"))

def _example_entries(package, examples, unordered):
    return [
        "    {{{}, {}.{}, {}, {}}},".format(
            _go_quote(name),
            package,
            name,
            _go_quote(examples[name]),
            "true" if name in unordered else "false",
        )
        for name in sorted(examples)
    ]

def _test_main(ctx):
    import_path = ctx.attrs.package_name
    internal_needed = bool(
        ctx.attrs.internal_tests or
        ctx.attrs.internal_benchmarks or
        ctx.attrs.internal_fuzz_targets or
        ctx.attrs.internal_examples or
        ctx.attrs.test_main == "internal"
    )
    external_needed = bool(
        ctx.attrs.external_tests or
        ctx.attrs.external_benchmarks or
        ctx.attrs.external_fuzz_targets or
        ctx.attrs.external_examples or
        ctx.attrs.test_main == "external"
    )
    imports = [
        '    "fmt"',
        '    "os"',
        '    "path/filepath"',
        '    "testing"',
        '    "testing/internal/testdeps"',
    ]
    if ctx.attrs.test_main != "none":
        imports.append('    "reflect"')
    if internal_needed:
        imports.append('    _test "{}"'.format(import_path))
    if external_needed:
        imports.append('    _xtest "{}_test"'.format(import_path))

    tests = _named_entries("_test", ctx.attrs.internal_tests) + _named_entries("_xtest", ctx.attrs.external_tests)
    benchmarks = _named_entries("_test", ctx.attrs.internal_benchmarks) + _named_entries("_xtest", ctx.attrs.external_benchmarks)
    fuzz_targets = _named_entries("_test", ctx.attrs.internal_fuzz_targets) + _named_entries("_xtest", ctx.attrs.external_fuzz_targets)
    examples = _example_entries("_test", ctx.attrs.internal_examples, ctx.attrs.internal_unordered_examples) + _example_entries("_xtest", ctx.attrs.external_examples, ctx.attrs.external_unordered_examples)
    main_body = [
        "    executable, err := os.Executable()",
        "    if err != nil {",
        '        fmt.Fprintf(os.Stderr, "resolve test executable: %v\\n", err)',
        "        os.Exit(1)",
        "    }",
        '    tempDir, err := os.MkdirTemp("", "kibou-go-test-")',
        "    if err != nil {",
        '        fmt.Fprintf(os.Stderr, "create test temporary directory: %v\\n", err)',
        "        os.Exit(1)",
        "    }",
        "    tempDir, err = filepath.Abs(tempDir)",
        "    if err != nil {",
        '        fmt.Fprintf(os.Stderr, "resolve test temporary directory: %v\\n", err)',
        "        os.Exit(1)",
        "    }",
        "    defer os.RemoveAll(tempDir)",
        '    for _, key := range []string{"HOME", "TEMP", "TMP", "TMPDIR", "USERPROFILE", "XDG_RUNTIME_DIR"} {',
        "        if err := os.Setenv(key, tempDir); err != nil {",
        '            fmt.Fprintf(os.Stderr, "set %s: %v\\n", key, err)',
        "            os.Exit(1)",
        "        }",
        "    }",
        "    if err := os.Chdir(filepath.Dir(executable)); err != nil {",
        '        fmt.Fprintf(os.Stderr, "enter test directory: %v\\n", err)',
        "        os.Exit(1)",
        "    }",
        '    testdeps.ImportPath = "{}"'.format(import_path),
        "    m := testing.MainStart(testdeps.TestDeps{}, tests, benchmarks, fuzzTargets, examples)",
    ]
    if ctx.attrs.test_main == "none":
        main_body.append("    os.Exit(m.Run())")
    else:
        package = "_test" if ctx.attrs.test_main == "internal" else "_xtest"
        main_body.extend([
            "    {}.TestMain(m)".format(package),
            '    os.Exit(int(reflect.ValueOf(m).Elem().FieldByName("exitCode").Int()))',
        ])
    return "\n".join([
        "// Code generated by kibou_go_test; DO NOT EDIT.",
        "",
        "package main",
        "",
        "import (",
    ] + imports + [
        ")",
        "",
        "var tests = []testing.InternalTest{",
    ] + tests + [
        "}",
        "",
        "var benchmarks = []testing.InternalBenchmark{",
    ] + benchmarks + [
        "}",
        "",
        "var fuzzTargets = []testing.InternalFuzzTarget{",
    ] + fuzz_targets + [
        "}",
        "",
        "var examples = []testing.InternalExample{",
    ] + examples + [
        "}",
        "",
        "func main() {",
    ] + main_body + [
        "}",
        "",
    ])

def _stable_test_env():
    # Buck2's OSS local test executor inherits these host variables without adding
    # their values to the test action key. Cacheable tests must shadow them.
    keys = [
        "ALLUSERSPROFILE",
        "APPDATA",
        "COMPUTERNAME",
        "COMSPEC",
        "CommonProgramFiles",
        "CommonProgramFiles(x86)",
        "HOME",
        "HOMEDRIVE",
        "HOMEPATH",
        "LOCALAPPDATA",
        "LOGNAME",
        "NUMBER_OF_PROCESSORS",
        "OS",
        "PATH",
        "PATHEXT",
        "PROCESSOR_ARCHITECTURE",
        "PROCESSOR_ARCHITEW6432",
        "PROCESSOR_IDENTIFIER",
        "PROCESSOR_LEVEL",
        "PROCESSOR_REVISION",
        "PSModulePath",
        "ProgramData",
        "ProgramFiles",
        "ProgramFiles(x86)",
        "ProgramW6432",
        "Public",
        "SYSTEMDRIVE",
        "SYSTEMROOT",
        "TEMP",
        "TMP",
        "TMPDIR",
        "USER",
        "USERDOMAIN",
        "USERNAME",
        "USERPROFILE",
        "UserDnsDomain",
        "WINDIR",
        "XDG_RUNTIME_DIR",
    ]
    env = {key: "" for key in keys}
    env.update({
        "LOGNAME": "kibou",
        "USER": "kibou",
        "USERNAME": "kibou",
    })
    return env

def _serialize_go_pkgs(pkgs):
    return {
        name: (
            pkg.archive_file,
            pkg.archive_file_shared,
            pkg.export_file,
            pkg.export_file_shared,
            pkg.coverage_instrumented,
        )
        for name, pkg in pkgs.items()
    }

def _declare_test_copy(ctx, node, pkgs):
    target = ctx.actions.anon_target(
        _test_copy_rule,
        {
            "_go_stdlib": ctx.attrs._go_stdlib,
            "_go_toolchain": ctx.attrs._go_toolchain,
            "embed_srcs": node.sources.embed_srcs,
            "package_name": node.package_name,
            "package_root": node.sources.package_root,
            "pkgs": _serialize_go_pkgs(pkgs),
            "srcs": node.sources.srcs,
        },
    )
    return GoPkg(
        archive_file = ctx.actions.assert_has_content_based_path(target.artifact("archive_file")),
        archive_file_shared = ctx.actions.assert_has_content_based_path(target.artifact("archive_file_shared")),
        export_file = ctx.actions.assert_has_content_based_path(target.artifact("export_file")),
        export_file_shared = ctx.actions.assert_has_content_based_path(target.artifact("export_file_shared")),
        coverage_instrumented = False,
    )

def _declare_test_copy_closure(ctx, internal_pkg, inherited_compile):
    """Recompile packages which transitively depend on the package under test.

    Go's test build replaces the ordinary package P with P compiled together
    with its internal tests. Every package which depends on P must therefore be
    recompiled against that replacement before compiling the external test
    package. See cmd/go's recompileForTest.
    """
    nodes = []
    seen_nodes = set()
    for dep in ctx.attrs.deps:
        if KibouGoLibraryInfo not in dep:
            continue
        for node in dep[KibouGoLibraryInfo].nodes:
            if node.package_name not in seen_nodes:
                seen_nodes.add(node.package_name)
                nodes.append(node)

    test_copies = {ctx.attrs.package_name: internal_pkg}
    for node in nodes:
        if node.package_name == ctx.attrs.package_name:
            continue
        if not any([name in test_copies for name in node.direct_deps]):
            continue

        copy_pkgs = dict(node.inherited_pkgs)
        for name, pkg in test_copies.items():
            if name in copy_pkgs:
                copy_pkgs[name] = pkg
        test_copies[node.package_name] = _declare_test_copy(ctx, node, copy_pkgs)

    result = dict(inherited_compile)
    result.update(test_copies)
    return result, test_copies

def _kibou_go_test_impl(ctx):
    inherited_compile = {
        name: pkg
        for name, pkg in get_inherited_compile_pkgs(ctx.attrs.deps).items()
        if name != ctx.attrs.package_name
    }
    internal_pkg, _, _ = declare_package_build(
        ctx = ctx,
        pkg_import_path = ctx.attrs.package_name,
        main = False,
        sources = _go_sources(ctx, ctx.attrs.srcs + ctx.attrs.internal_test_srcs),
        cgo_build_context = None,
        config = GoBuildConfig(cgo_enabled = False, with_tests = True),
        pkgs = inherited_compile,
    )

    pkgs, test_copies = _declare_test_copy_closure(ctx, internal_pkg, inherited_compile)
    external_outputs = []
    if ctx.attrs.external_test_srcs:
        for index, src in enumerate(ctx.attrs.external_test_srcs):
            external_outputs.append(ctx.actions.copy_file(
                "__kibou_xtest_srcs__/xtest_{}.go".format(index),
                src,
                has_content_based_path = True,
            ))
        external_pkg, _, _ = declare_package_build(
            ctx = ctx,
            pkg_import_path = ctx.attrs.package_name + "_test",
            main = False,
            sources = GoSourceInputs(
                srcs = external_outputs,
                embed_srcs = ctx.attrs.embed_srcs,
            ),
            cgo_build_context = None,
            config = GoBuildConfig(cgo_enabled = False),
            pkgs = pkgs,
            cgo_gen_dir_name = "cgo_gen_external_test",
        )
        pkgs[ctx.attrs.package_name + "_test"] = external_pkg

    gen_main = ctx.actions.write(
        "testmain.go",
        _test_main(ctx),
        has_content_based_path = True,
    )
    main, _, _ = declare_package_build(
        ctx = ctx,
        pkg_import_path = ctx.attrs.package_name + ".test",
        main = True,
        sources = GoSourceInputs(srcs = [gen_main], package_root = ""),
        cgo_build_context = None,
        config = GoBuildConfig(cgo_enabled = False),
        pkgs = pkgs,
        cgo_gen_dir_name = "cgo_gen_test_main",
    )
    inherited_link = {
        name: pkg
        for name, pkg in get_inherited_link_pkgs(ctx.attrs.deps).items()
        if name not in test_copies
    }
    link_pkgs = merge_pkgs([inherited_link, pkgs])
    binary, runtime_files, external_debug_info = link(
        ctx = ctx,
        main = main,
        cgo_enabled = False,
        pkgs = link_pkgs,
        deps = [],
        link_style = LinkStyle("static"),
        build_mode = GoBuildMode("exe"),
    )

    copied_resources = []
    for destination, resource in ctx.attrs.resources.items():
        if destination.startswith("/") or destination == ".." or destination.startswith("../") or "/../" in destination:
            fail("test resource destination must be relative and cannot traverse upward: {}".format(destination))
        copied_resources.append(ctx.actions.copy_file(
            destination,
            resource,
            has_content_based_path = False,
        ))

    run_cmd = cmd_args(
        binary,
        hidden = copied_resources + runtime_files + external_debug_info,
    )
    env = _stable_test_env()
    env.update(ctx.attrs.env)
    return [
        DefaultInfo(
            default_output = binary,
            other_outputs = [gen_main] + external_outputs + copied_resources + runtime_files + external_debug_info,
        ),
        ExternalRunnerTestInfo(
            type = "go",
            command = [run_cmd],
            env = env,
            labels = ctx.attrs.labels,
            supports_test_execution_caching = ctx.attrs.supports_test_execution_caching,
            run_from_project_root = True,
            use_project_relative_paths = True,
        ),
        RunInfo(args = run_cmd),
    ]

_common_attrs = {
    "deps": attrs.list(attrs.dep(), default = []),
    "embed_srcs": attrs.dict(attrs.string(), attrs.source(), default = {}),
    "package_name": attrs.string(),
    "package_root": attrs.option(attrs.string(), default = None),
    "srcs": attrs.list(attrs.source()),
    "_go_stdlib": attrs.dep(
        default = "root//:stage2_stdlib",
        providers = [GoStdlib],
    ),
    "_go_toolchain": attrs.toolchain_dep(
        default = "toolchains//:final_go",
        providers = [GoToolchainInfo],
    ),
}

kibou_go_library = rule(
    impl = _kibou_go_library_impl,
    attrs = _common_attrs,
)

kibou_go_binary = rule(
    impl = _kibou_go_binary_impl,
    attrs = _common_attrs | {
        "_cxx_toolchain": attrs.toolchain_dep(default = "toolchains//:cxx"),
    },
)

kibou_go_release_binary = rule(
    impl = _kibou_go_release_binary_impl,
    attrs = _common_attrs | {
        "_cxx_toolchain": attrs.toolchain_dep(default = "toolchains//:cxx"),
    },
)

kibou_go_test = rule(
    impl = _kibou_go_test_impl,
    attrs = _common_attrs | {
        "env": attrs.dict(attrs.string(), attrs.arg(), default = {}),
        "external_benchmarks": attrs.list(attrs.string(), default = []),
        "external_examples": attrs.dict(attrs.string(), attrs.string(), default = {}),
        "external_fuzz_targets": attrs.list(attrs.string(), default = []),
        "external_test_srcs": attrs.list(attrs.source(), default = []),
        "external_tests": attrs.list(attrs.string(), default = []),
        "external_unordered_examples": attrs.list(attrs.string(), default = []),
        "internal_benchmarks": attrs.list(attrs.string(), default = []),
        "internal_examples": attrs.dict(attrs.string(), attrs.string(), default = {}),
        "internal_fuzz_targets": attrs.list(attrs.string(), default = []),
        "internal_test_srcs": attrs.list(attrs.source(), default = []),
        "internal_tests": attrs.list(attrs.string(), default = []),
        "internal_unordered_examples": attrs.list(attrs.string(), default = []),
        "labels": attrs.list(attrs.string(), default = []),
        "resources": attrs.dict(attrs.string(), attrs.source(), default = {}),
        "supports_test_execution_caching": attrs.bool(default = False),
        "test_main": attrs.enum(["none", "internal", "external"], default = "none"),
        "_cxx_toolchain": attrs.toolchain_dep(default = "toolchains//:cxx"),
    },
)
