load("@prelude//go:toolchain.bzl", "GoToolchainInfo", "GoVersion")

GoRootSourcesInfo = provider(fields = {
    "files": provider_field(dict[str, Artifact]),
})

def _go_root_sources_impl(ctx):
    package = ctx.label.package.removeprefix("go/")
    return [
        DefaultInfo(),
        GoRootSourcesInfo(files = {
            package + "/" + src.short_path: src
            for src in ctx.attrs.srcs
        }),
    ]

go_root_sources = rule(
    impl = _go_root_sources_impl,
    attrs = {
        "srcs": attrs.list(attrs.source()),
    },
)

def _staged_go_root_impl(ctx):
    files = {}
    for sources in ctx.attrs.srcs:
        files.update(sources[GoRootSourcesInfo].files)
    files.update(ctx.attrs.generated)
    for header in ["asm_amd64.h", "asm_ppc64x.h", "asm_riscv64.h", "funcdata.h", "textflag.h"]:
        files["pkg/include/" + header] = files["src/runtime/" + header]

    suffix = ".exe" if ctx.attrs.go_os == "windows" else ""
    tool_dir = "pkg/tool/{}_{}".format(ctx.attrs.go_os, ctx.attrs.go_arch)
    files[tool_dir + "/asm" + suffix] = ctx.attrs.assembler[DefaultInfo].default_outputs[0]
    files[tool_dir + "/cgo" + suffix] = ctx.attrs.cgo[DefaultInfo].default_outputs[0]
    files[tool_dir + "/compile" + suffix] = ctx.attrs.compiler[DefaultInfo].default_outputs[0]
    files[tool_dir + "/link" + suffix] = ctx.attrs.linker[DefaultInfo].default_outputs[0]
    files[tool_dir + "/preprofile" + suffix] = ctx.attrs.preprofile[DefaultInfo].default_outputs[0]

    out = ctx.actions.declare_output("goroot", dir = True, has_content_based_path = True)
    ctx.actions.copied_dir(out, files)
    return [DefaultInfo(default_output = out)]

staged_go_root = rule(
    impl = _staged_go_root_impl,
    attrs = {
        "assembler": attrs.exec_dep(providers = [RunInfo]),
        "cgo": attrs.exec_dep(providers = [RunInfo]),
        "compiler": attrs.exec_dep(providers = [RunInfo]),
        "generated": attrs.dict(attrs.string(), attrs.source(), default = {}),
        "go_arch": attrs.string(),
        "go_os": attrs.string(),
        "linker": attrs.exec_dep(providers = [RunInfo]),
        "preprofile": attrs.exec_dep(providers = [RunInfo]),
        "srcs": attrs.list(attrs.dep(providers = [GoRootSourcesInfo])),
    },
)

def _staged_go_toolchain_impl(ctx):
    base = ctx.attrs.base[GoToolchainInfo]
    build_tags = base.build_tags if ctx.attrs.build_tags == None else ctx.attrs.build_tags
    compiler = ctx.attrs.compiler[RunInfo]
    if ctx.attrs.compiler_identity_inputs:
        # Identity inputs deliberately participate in compile action keys without
        # becoming compiler arguments. This models a changed compiler producer
        # whose emitted compiler bytes remain identical.
        compiler = RunInfo(args = cmd_args(
            compiler,
            hidden = ctx.attrs.compiler_identity_inputs,
        ))
    return [
        DefaultInfo(),
        GoToolchainInfo(
            allow_cache_upload = base.allow_cache_upload,
            assembler = ctx.attrs.assembler[RunInfo],
            assembler_flags = base.assembler_flags,
            cxx_compiler_flags = base.cxx_compiler_flags,
            cgo = ctx.attrs.cgo[RunInfo],
            compiler = compiler,
            compiler_flags = base.compiler_flags,
            pkg_analyzer = base.pkg_analyzer,
            gen_embedcfg = base.gen_embedcfg,
            external_linker_flags = base.external_linker_flags,
            go_wrapper = base.go_wrapper,
            cover = base.cover,
            go = base.go,
            env_go_arch = base.env_go_arch,
            env_go_os = base.env_go_os,
            env_go_arm = base.env_go_arm,
            env_go_root = ctx.attrs.go_root,
            # Current Go otherwise derives scheduler limits and transparent-huge-
            # page behavior from host /proc and /sys state. Make those choices
            # explicit, stable, and part of every staged action key.
            env_go_debug = base.env_go_debug | {
                "containermaxprocs": "0",
                "disablethp": "1",
            },
            env_go_experiment = base.env_go_experiment,
            linker = ctx.attrs.linker[RunInfo],
            linker_flags = base.linker_flags,
            packer = base.packer,
            build_tags = build_tags,
            asan = base.asan,
            race = base.race,
            fuzz = base.fuzz,
            version = GoVersion(minor = 28, patch = 0),
        ),
    ]

staged_go_toolchain = rule(
    impl = _staged_go_toolchain_impl,
    is_toolchain_rule = True,
    attrs = {
        "assembler": attrs.exec_dep(providers = [RunInfo]),
        "base": attrs.toolchain_dep(providers = [GoToolchainInfo]),
        "build_tags": attrs.option(attrs.list(attrs.string()), default = None),
        "cgo": attrs.exec_dep(providers = [RunInfo]),
        "compiler": attrs.exec_dep(providers = [RunInfo]),
        "compiler_identity_inputs": attrs.list(attrs.source(), default = []),
        "go_root": attrs.source(allow_directory = True),
        "linker": attrs.exec_dep(providers = [RunInfo]),
    },
)
