package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var bootstrapDirs = []string{
	"cmp",
	"cmd/asm",
	"cmd/asm/internal/...",
	"cmd/cgo",
	"cmd/compile",
	"cmd/compile/internal/...",
	"cmd/internal/archive",
	"cmd/internal/bio",
	"cmd/internal/codesign",
	"cmd/internal/cov/covcmd",
	"cmd/internal/dwarf",
	"cmd/internal/edit",
	"cmd/internal/gcprog",
	"cmd/internal/goobj",
	"cmd/internal/hash",
	"cmd/internal/macho",
	"cmd/internal/obj/...",
	"cmd/internal/objabi",
	"cmd/internal/par",
	"cmd/internal/pgo",
	"cmd/internal/pkgpath",
	"cmd/internal/quoted",
	"cmd/internal/src",
	"cmd/internal/sys",
	"cmd/internal/telemetry",
	"cmd/internal/telemetry/counter",
	"cmd/link",
	"cmd/link/internal/...",
	"cmd/preprofile",
	"compress/flate",
	"compress/zlib",
	"container/heap",
	"debug/dwarf",
	"debug/elf",
	"debug/macho",
	"debug/pe",
	"go/build/constraint",
	"go/constant",
	"go/version",
	"internal/abi",
	"internal/bisect",
	"internal/buildcfg",
	"internal/coverage",
	"internal/exportdata",
	"internal/goarch",
	"internal/godebugs",
	"internal/goexperiment",
	"internal/goroot",
	"internal/gover",
	"internal/goversion",
	"internal/lazyregexp",
	"internal/pkgbits",
	"internal/platform",
	"internal/profile",
	"internal/race",
	"internal/runtime/gc",
	"internal/saferio",
	"internal/strconv",
	"internal/syscall/unix",
	"internal/types/errors",
	"internal/unsafeheader",
	"internal/xcoff",
	"internal/zstd",
	"math/bits",
	"sort",
}

var bootstrapCommands = map[string]bool{
	"cmd/asm":        true,
	"cmd/cgo":        true,
	"cmd/compile":    true,
	"cmd/link":       true,
	"cmd/preprofile": true,
}

type packageSpec struct {
	importPath    string
	name          string
	goFiles       []string
	asmFiles      []string
	headers       []string
	sysoFiles     []string
	imports       []string
	embedPatterns []string
	embedFiles    []string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./build_defs/generator/main.go <bootstrap|project> [flags]")
		os.Exit(2)
	}

	switch os.Args[1] {
	case "bootstrap":
		flags := flag.NewFlagSet("bootstrap", flag.ExitOnError)
		root := flags.String("root", ".", "repository root")
		goos := flags.String("goos", build.Default.GOOS, "target operating system")
		goarch := flags.String("goarch", build.Default.GOARCH, "target architecture")
		_ = flags.Parse(os.Args[2:])
		absRoot, err := filepath.Abs(*root)
		check(err)
		check(generateBootstrap(absRoot, *goos, *goarch))
	case "project":
		flags := flag.NewFlagSet("project", flag.ExitOnError)
		root := flags.String("root", ".", "repository root")
		modules := flags.String("modules", "base,delve,misc,third_party/build-tools,tools", "comma-separated workspace modules")
		goos := flags.String("goos", build.Default.GOOS, "target operating system")
		goarch := flags.String("goarch", build.Default.GOARCH, "target architecture")
		_ = flags.Parse(os.Args[2:])
		absRoot, err := filepath.Abs(*root)
		check(err)
		check(generateProject(absRoot, strings.Split(*modules, ","), *goos, *goarch))
	default:
		fmt.Fprintf(os.Stderr, "unknown generator command %q\n", os.Args[1])
		os.Exit(2)
	}
}

func generateBootstrap(root, goos, goarch string) error {
	goRoot := filepath.Join(root, "go")
	sourceRoot := filepath.Join(goRoot, "src")
	dirs, err := expandBootstrapDirs(sourceRoot)
	if err != nil {
		return err
	}

	ctx := build.Default
	ctx.GOROOT = goRoot
	ctx.GOPATH = ""
	ctx.GOOS = goos
	ctx.GOARCH = goarch
	ctx.CgoEnabled = false
	ctx.BuildTags = []string{"compiler_bootstrap", "math_big_pure_go", "purego"}
	ctx.ReleaseTags = append(ctx.ReleaseTags, "go1.27", "go1.28")

	packages := make(map[string]packageSpec)
	for _, importPath := range dirs {
		pkg, err := ctx.ImportDir(filepath.Join(sourceRoot, filepath.FromSlash(importPath)), 0)
		if _, ok := err.(*build.NoGoError); ok {
			continue
		}
		if err != nil {
			return fmt.Errorf("select %s: %w", importPath, err)
		}
		if len(pkg.CgoFiles) != 0 {
			return fmt.Errorf("bootstrap package %s selected cgo files: %v", importPath, pkg.CgoFiles)
		}

		spec := packageSpec{
			importPath:    importPath,
			name:          pkg.Name,
			goFiles:       filteredFiles(pkg.GoFiles),
			asmFiles:      filteredFiles(pkg.SFiles),
			headers:       filteredFiles(pkg.HFiles),
			sysoFiles:     filteredFiles(pkg.SysoFiles),
			imports:       sortedCopy(pkg.Imports),
			embedPatterns: sortedCopy(pkg.EmbedPatterns),
		}
		packages[importPath] = spec
	}

	paths := make([]string, 0, len(packages))
	for importPath := range packages {
		paths = append(paths, importPath)
	}
	sort.Strings(paths)
	for _, importPath := range paths {
		if err := writeBootstrapBUCK(root, packages[importPath], packages); err != nil {
			return err
		}
	}
	bundles, err := writeGoRootSourceBundles(root, packages)
	if err != nil {
		return err
	}
	bootstrapStdlibPackages, err := stdlibClosure(&ctx, sourceRoot, packages)
	if err != nil {
		return err
	}
	productionCtx := ctx
	productionCtx.BuildTags = nil
	productionStdlibPackages, err := stdlibClosure(&productionCtx, sourceRoot, packages)
	if err != nil {
		return err
	}
	stdlibPackages := sortedUnique(append(bootstrapStdlibPackages, productionStdlibPackages...))
	if err := writeRootBUCK(root, bundles, stdlibPackages); err != nil {
		return err
	}

	fmt.Printf("generated %d stage-1 package targets for %s/%s\n", len(paths), goos, goarch)
	return nil
}

func expandBootstrapDirs(sourceRoot string) ([]string, error) {
	set := make(map[string]bool)
	for _, pattern := range bootstrapDirs {
		recursive := strings.HasSuffix(pattern, "/...")
		base := strings.TrimSuffix(pattern, "/...")
		set[base] = true
		if !recursive {
			continue
		}
		basePath := filepath.Join(sourceRoot, filepath.FromSlash(base))
		err := filepath.WalkDir(basePath, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				return nil
			}
			if path != basePath && (entry.Name() == "testdata" || strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_")) {
				return filepath.SkipDir
			}
			rel, err := filepath.Rel(sourceRoot, path)
			if err != nil {
				return err
			}
			set[filepath.ToSlash(rel)] = true
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	result := make([]string, 0, len(set))
	for path := range set {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}

func writeBootstrapBUCK(root string, spec packageSpec, all map[string]packageSpec) error {
	var out bytes.Buffer
	out.WriteString("# Generated by build_defs/generator/main.go; changes will be overwritten.\n")
	out.WriteString("load(\"//build_defs/rules:bootstrap_go.bzl\", \"bootstrap_go_binary\", \"bootstrap_go_package\")\n\n")

	goFiles := append([]string(nil), spec.goFiles...)
	switch spec.importPath {
	case "internal/buildcfg":
		goFiles = append(goFiles, "//build_defs/generated:buildcfg_zbootstrap")
	case "cmd/internal/objabi":
		goFiles = append(goFiles, "//build_defs/generated:objabi_zbootstrap")
	case "cmd/cgo":
		goFiles = append(goFiles, "//build_defs/generated:cgo_zdefaultcc")
	}
	imports := make([]string, 0, len(spec.imports)+1)
	importMap := make(map[string]string)
	packageDeps := make([]string, 0)
	for _, imported := range spec.imports {
		resolved := imported
		if _, ok := all[imported]; ok {
			resolved = "bootstrap/" + imported
			importMap[imported] = resolved
			packageDeps = append(packageDeps, imported)
		}
		imports = append(imports, resolved)
	}
	if spec.importPath == "internal/buildcfg" && !contains(imports, "runtime") {
		imports = append(imports, "runtime")
	}
	sort.Strings(imports)
	sort.Strings(packageDeps)

	for stage := 1; stage <= 3; stage++ {
		if stage != 1 {
			out.WriteString("\n")
		}
		out.WriteString("bootstrap_go_package(\n")
		out.WriteString("    name = " + strconv.Quote(fmt.Sprintf("stage%d_pkg", stage)) + ",\n")
		out.WriteString("    package_name = " + strconv.Quote(spec.name) + ",\n")
		out.WriteString("    pkg_import_path = " + strconv.Quote("bootstrap/"+spec.importPath) + ",\n")
		out.WriteString("    package_root = " + strconv.Quote("go/src/"+spec.importPath) + ",\n")
		if stage > 1 {
			out.WriteString("    compiler_toolchain = " + strconv.Quote(fmt.Sprintf("toolchains//:stage%d_go", stage-1)) + ",\n")
			out.WriteString("    stdlib = " + strconv.Quote(fmt.Sprintf("//:stage%d_stdlib", stage-1)) + ",\n")
		}
		if bootstrapCommands[spec.importPath] {
			out.WriteString("    main = True,\n")
		}
		writeList(&out, "go_srcs", goFiles)
		writeList(&out, "asm_srcs", spec.asmFiles)
		writeList(&out, "headers", spec.headers)
		writeList(&out, "syso_srcs", spec.sysoFiles)
		writeList(&out, "embed_patterns", spec.embedPatterns)
		writeDictIdentity(&out, "embed_srcs", spec.embedFiles)
		writeList(&out, "imports", imports)
		writeDict(&out, "import_map", importMap)

		deps := make([]string, 0, len(packageDeps))
		for _, imported := range packageDeps {
			deps = append(deps, fmt.Sprintf("//go/src/%s:stage%d_pkg", imported, stage))
		}
		writeList(&out, "deps", deps)
		out.WriteString("    visibility = [\"PUBLIC\"],\n")
		out.WriteString(")\n")

		if bootstrapCommands[spec.importPath] {
			out.WriteString("\nbootstrap_go_binary(\n")
			out.WriteString("    name = " + strconv.Quote(fmt.Sprintf("stage%d", stage)) + ",\n")
			out.WriteString("    main = " + strconv.Quote(fmt.Sprintf(":stage%d_pkg", stage)) + ",\n")
			if stage > 1 {
				out.WriteString("    compiler_toolchain = " + strconv.Quote(fmt.Sprintf("toolchains//:stage%d_go", stage-1)) + ",\n")
				out.WriteString("    stdlib = " + strconv.Quote(fmt.Sprintf("//:stage%d_stdlib", stage-1)) + ",\n")
			}
			out.WriteString("    visibility = [\"PUBLIC\"],\n")
			out.WriteString(")\n")
		}
	}

	filename := filepath.Join(root, "go", "src", filepath.FromSlash(spec.importPath), "BUCK")
	return os.WriteFile(filename, out.Bytes(), 0o644)
}

func writeGoRootSourceBundles(root string, packages map[string]packageSpec) ([]string, error) {
	sourceRoot := filepath.Join(root, "go", "src")
	filesByDir := make(map[string][]string)
	err := filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != sourceRoot && (entry.Name() == "testdata" || strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if name == "BUCK" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || strings.HasPrefix(name, "#") ||
			strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_test.s") ||
			strings.HasSuffix(name, ".pgo") || strings.HasSuffix(name, "~") {
			return nil
		}
		relDir, err := filepath.Rel(sourceRoot, filepath.Dir(path))
		if err != nil {
			return err
		}
		filesByDir[filepath.ToSlash(relDir)] = append(filesByDir[filepath.ToSlash(relDir)], name)
		return nil
	})
	if err != nil {
		return nil, err
	}

	dirs := make([]string, 0, len(filesByDir))
	for dir := range filesByDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	labels := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		files := filesByDir[dir]
		sort.Strings(files)
		filename := filepath.Join(sourceRoot, filepath.FromSlash(dir), "BUCK")
		var out bytes.Buffer
		if _, ok := packages[dir]; ok {
			data, err := os.ReadFile(filename)
			if err != nil {
				return nil, err
			}
			out.Write(data)
			out.WriteString("\nload(\"//build_defs/rules:staged_go_toolchain.bzl\", \"go_root_sources\")\n")
		} else {
			out.WriteString("# Generated by build_defs/generator/main.go; changes will be overwritten.\n")
			out.WriteString("load(\"//build_defs/rules:staged_go_toolchain.bzl\", \"go_root_sources\")\n")
		}
		out.WriteString("\ngo_root_sources(\n")
		out.WriteString("    name = \"goroot_sources\",\n")
		writeList(&out, "srcs", files)
		out.WriteString("    visibility = [\"PUBLIC\"],\n")
		out.WriteString(")\n")
		if err := os.WriteFile(filename, out.Bytes(), 0o644); err != nil {
			return nil, err
		}
		labelDir := "go/src"
		if dir != "." {
			labelDir += "/" + dir
		}
		labels = append(labels, "//"+labelDir+":goroot_sources")
	}
	return labels, nil
}

func stdlibClosure(ctx *build.Context, sourceRoot string, packages map[string]packageSpec) ([]string, error) {
	needed := make(map[string]bool)
	queue := make([]string, 0)
	// Project generation records its direct standard-library imports. Seed
	// those plus the native test runtime, then close over imports below.
	stdlib, err := projectStdlibRoots(filepath.Dir(filepath.Dir(sourceRoot)))
	if err != nil {
		return nil, err
	}
	stdlib = append(stdlib, "testing", "testing/internal/testdeps")
	for _, importPath := range stdlib {
		needed[importPath] = true
		queue = append(queue, importPath)
	}
	for _, spec := range packages {
		for _, imported := range spec.imports {
			resolved := imported
			if _, err := os.Stat(filepath.Join(sourceRoot, "vendor", filepath.FromSlash(imported))); err == nil {
				resolved = "vendor/" + imported
			}
			if _, current := packages[imported]; !current && resolved != "unsafe" && resolved != "builtin" && resolved != "C" {
				if !needed[resolved] {
					needed[resolved] = true
					queue = append(queue, resolved)
				}
			}
		}
	}
	for index := 0; index < len(queue); index++ {
		importPath := queue[index]
		packageDir := filepath.Join(sourceRoot, filepath.FromSlash(importPath))
		if _, err := os.Stat(packageDir); os.IsNotExist(err) {
			packageDir = filepath.Join(sourceRoot, "vendor", filepath.FromSlash(importPath))
		}
		pkg, err := ctx.ImportDir(packageDir, 0)
		if err != nil {
			return nil, fmt.Errorf("select stdlib package %s: %w", importPath, err)
		}
		for _, imported := range pkg.Imports {
			resolved := imported
			if _, err := os.Stat(filepath.Join(sourceRoot, "vendor", filepath.FromSlash(imported))); err == nil {
				resolved = "vendor/" + imported
			}
			if resolved == "unsafe" || resolved == "builtin" || resolved == "C" || needed[resolved] {
				continue
			}
			needed[resolved] = true
			queue = append(queue, resolved)
		}
	}
	sort.Strings(queue)
	return queue, nil
}

func projectStdlibRoots(root string) ([]string, error) {
	filename := filepath.Join(root, "build_defs", "generated", "project_stdlib_roots.txt")
	contents, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	roots := make([]string, 0)
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			roots = append(roots, line)
		}
	}
	return sortedUnique(roots), nil
}

func allStdlibPackages(ctx *build.Context, sourceRoot string) ([]string, error) {
	packages := make([]string, 0)
	err := filepath.WalkDir(sourceRoot, func(filename string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if filename != sourceRoot {
			name := entry.Name()
			if name == "cmd" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				return filepath.SkipDir
			}
		}
		rel, err := filepath.Rel(sourceRoot, filename)
		if err != nil || rel == "." {
			return err
		}
		importPath := filepath.ToSlash(rel)
		pkg, err := ctx.ImportDir(filename, 0)
		if _, ok := err.(*build.NoGoError); ok {
			return nil
		}
		if err != nil {
			return fmt.Errorf("select standard package %s: %w", importPath, err)
		}
		if pkg.IsCommand() || importPath == "builtin" || importPath == "unsafe" {
			return nil
		}
		packages = append(packages, importPath)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(packages)
	return packages, nil
}

func writeRootBUCK(root string, bundles, stdlibPackages []string) error {
	var out bytes.Buffer
	out.WriteString("# Generated by build_defs/generator/main.go; changes will be overwritten.\n")
	out.WriteString("load(\"//build_defs/rules:staged_go_stdlib.bzl\", \"staged_go_stdlib\")\n")
	out.WriteString("load(\"//build_defs/rules:staged_go_toolchain.bzl\", \"staged_go_root\")\n\n")
	out.WriteString("_GO_ARCH = select({\n")
	out.WriteString("    \"config//cpu:arm64\": \"arm64\",\n")
	out.WriteString("    \"config//cpu:x86_64\": \"amd64\",\n")
	out.WriteString("})\n\n")
	out.WriteString("_GO_OS = select({\n")
	out.WriteString("    \"config//os:linux\": \"linux\",\n")
	out.WriteString("    \"config//os:macos\": \"darwin\",\n")
	out.WriteString("    \"config//os:windows\": \"windows\",\n")
	out.WriteString("})\n")
	for stage := 1; stage <= 2; stage++ {
		out.WriteString("\nstaged_go_root(\n")
		out.WriteString("    name = " + strconv.Quote(fmt.Sprintf("stage%d_goroot", stage)) + ",\n")
		out.WriteString("    assembler = " + strconv.Quote(fmt.Sprintf("//go/src/cmd/asm:stage%d", stage)) + ",\n")
		out.WriteString("    cgo = " + strconv.Quote(fmt.Sprintf("//go/src/cmd/cgo:stage%d", stage)) + ",\n")
		out.WriteString("    compiler = " + strconv.Quote(fmt.Sprintf("//go/src/cmd/compile:stage%d", stage)) + ",\n")
		out.WriteString("    linker = " + strconv.Quote(fmt.Sprintf("//go/src/cmd/link:stage%d", stage)) + ",\n")
		out.WriteString("    preprofile = " + strconv.Quote(fmt.Sprintf("//go/src/cmd/preprofile:stage%d", stage)) + ",\n")
		out.WriteString("    go_arch = _GO_ARCH,\n")
		out.WriteString("    go_os = _GO_OS,\n")
		out.WriteString("    generated = {\n")
		out.WriteString("        \"VERSION\": \"//build_defs/generated:goroot_version\",\n")
		out.WriteString("        \"src/cmd/cgo/zdefaultcc.go\": \"//build_defs/generated:cgo_zdefaultcc\",\n")
		out.WriteString("        \"src/cmd/internal/objabi/zbootstrap.go\": \"//build_defs/generated:objabi_zbootstrap\",\n")
		out.WriteString("        \"src/internal/buildcfg/zbootstrap.go\": \"//build_defs/generated:buildcfg_zbootstrap\",\n")
		out.WriteString("    },\n")
		writeList(&out, "srcs", bundles)
		out.WriteString("    visibility = [\"PUBLIC\"],\n")
		out.WriteString(")\n")

		out.WriteString("\nstaged_go_stdlib(\n")
		out.WriteString("    name = " + strconv.Quote(fmt.Sprintf("stage%d_stdlib", stage)) + ",\n")
		stdlibToolchain := fmt.Sprintf("toolchains//:stage%d_go", stage)
		if stage == 2 {
			stdlibToolchain = "toolchains//:stage2_stdlib_go"
		}
		out.WriteString("    compiler_toolchain = " + strconv.Quote(stdlibToolchain) + ",\n")
		writeList(&out, "roots", stdlibPackages)
		out.WriteString("    visibility = [\"PUBLIC\"],\n")
		out.WriteString(")\n")
	}
	return os.WriteFile(filepath.Join(root, "BUCK"), out.Bytes(), 0o644)
}

type projectFile struct {
	name      string
	pkgName   string
	imports   []string
	hasEmbed  bool
	test      bool
	tests     []string
	benches   []string
	fuzz      []string
	testMain  bool
	examples  map[string]string
	unordered []string
}

type projectPackage struct {
	dir               string
	moduleDir         string
	vendored          bool
	importPath        string
	name              string
	srcs              []string
	internalTests     []string
	externalTests     []string
	prodImports       []string
	testImports       []string
	internalTestFns   []string
	externalTestFns   []string
	internalBenches   []string
	externalBenches   []string
	internalFuzz      []string
	externalFuzz      []string
	internalExamples  map[string]string
	externalExamples  map[string]string
	internalUnordered []string
	externalUnordered []string
	testMain          string
	resources         []string
	embedFiles        []string
	testCacheable     bool
}

func generateProject(root string, moduleDirs []string, goos, goarch string) error {
	packages := make([]projectPackage, 0)
	for _, moduleDir := range moduleDirs {
		moduleDir = strings.TrimSpace(moduleDir)
		if moduleDir == "" {
			continue
		}
		moduleRoot := filepath.Join(root, filepath.FromSlash(moduleDir))
		modulePath, err := readModulePath(filepath.Join(moduleRoot, "go.mod"))
		if err != nil {
			return err
		}
		found, err := scanProjectModule(root, moduleRoot, modulePath, goos, goarch)
		if err != nil {
			return err
		}
		for index := range found {
			found[index].moduleDir = moduleDir
		}
		packages = append(packages, found...)
		vendorRoot := filepath.Join(moduleRoot, "vendor")
		if info, err := os.Stat(vendorRoot); err == nil && info.IsDir() {
			vendored, err := scanProjectModule(root, vendorRoot, "", goos, goarch)
			if err != nil {
				return err
			}
			for index := range vendored {
				vendored[index].moduleDir = moduleDir
				vendored[index].vendored = true
			}
			packages = append(packages, vendored...)
		}
	}
	cacheableTests, err := readLineSet(filepath.Join(root, "build_defs", "cacheable_tests.txt"))
	if err != nil {
		return err
	}
	for index := range packages {
		packages[index].testCacheable = cacheableTests[packages[index].importPath]
	}

	byImport := make(map[string][]projectPackage, len(packages))
	stdlibRoots := make([]string, 0)
	for _, pkg := range packages {
		byImport[pkg.importPath] = append(byImport[pkg.importPath], pkg)
		for _, imported := range append(append([]string(nil), pkg.prodImports...), pkg.testImports...) {
			if isStandardImport(imported) && imported != "C" {
				stdlibRoots = append(stdlibRoots, imported)
			}
		}
	}
	stdlibRoots = sortedUnique(stdlibRoots)
	stdlibFile := filepath.Join(root, "build_defs", "generated", "project_stdlib_roots.txt")
	if err := os.WriteFile(stdlibFile, []byte(strings.Join(stdlibRoots, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].dir < packages[j].dir })

	testCount := 0
	externalTestCount := 0
	unresolvedCount := 0
	for _, pkg := range packages {
		unresolved, err := writeProjectBUCK(root, pkg, byImport)
		if err != nil {
			return err
		}
		unresolvedCount += unresolved
		if len(pkg.internalTests)+len(pkg.externalTests) > 0 {
			testCount++
		}
		if len(pkg.externalTests) > 0 {
			externalTestCount++
		}
	}
	fmt.Printf("generated %d project packages (%d tests, %d with external tests, %d unresolved import edges)\n", len(packages), testCount, externalTestCount, unresolvedCount)
	return nil
}

func readLineSet(filename string) (map[string]bool, error) {
	contents, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool)
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		result[line] = true
	}
	return result, nil
}

func readModulePath(filename string) (string, error) {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", filename, err)
	}
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("no module directive in %s", filename)
}

func scanProjectModule(root, moduleRoot, modulePath, goos, goarch string) ([]projectPackage, error) {
	filesByDir := make(map[string][]string)
	err := filepath.WalkDir(moduleRoot, func(filename string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if filename != moduleRoot {
				name := entry.Name()
				if name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
					return filepath.SkipDir
				}
				if _, err := os.Stat(filepath.Join(filename, "go.mod")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".go" && ext != ".s" && ext != ".S" && ext != ".h" && ext != ".syso" {
			return nil
		}
		filesByDir[filepath.Dir(filename)] = append(filesByDir[filepath.Dir(filename)], entry.Name())
		return nil
	})
	if err != nil {
		return nil, err
	}

	dirs := make([]string, 0, len(filesByDir))
	for dir := range filesByDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	packages := make([]projectPackage, 0, len(dirs))
	for _, dir := range dirs {
		relModule, err := filepath.Rel(moduleRoot, dir)
		if err != nil {
			return nil, err
		}
		relRoot, err := filepath.Rel(root, dir)
		if err != nil {
			return nil, err
		}
		importPath := modulePath
		if relModule != "." {
			importPath = path.Join(modulePath, filepath.ToSlash(relModule))
		}
		pkg, ok, err := inspectProjectPackage(dir, filepath.ToSlash(relRoot), importPath, filesByDir[dir], goos, goarch)
		if err != nil {
			return nil, err
		}
		if ok {
			packages = append(packages, pkg)
		}
	}
	return packages, nil
}

func inspectProjectPackage(dir, relRoot, importPath string, filenames []string, goos, goarch string) (projectPackage, bool, error) {
	sort.Strings(filenames)
	parsed := make([]projectFile, 0)
	prodName := ""
	prodSources := make([]string, 0)
	for _, name := range filenames {
		matches, err := matchesProjectPlatform(dir, name, goos, goarch)
		if err != nil {
			return projectPackage{}, false, err
		}
		if !matches {
			continue
		}
		if filepath.Ext(name) != ".go" {
			prodSources = append(prodSources, name)
			continue
		}
		file, err := inspectProjectFile(filepath.Join(dir, name), name)
		if err != nil {
			return projectPackage{}, false, err
		}
		parsed = append(parsed, file)
		if !file.test {
			prodSources = append(prodSources, name)
			if prodName == "" {
				prodName = file.pkgName
			} else if prodName != file.pkgName {
				return projectPackage{}, false, fmt.Errorf("%s mixes production packages %s and %s", dir, prodName, file.pkgName)
			}
		}
	}
	if prodName == "" {
		return projectPackage{}, false, nil
	}

	pkg := projectPackage{
		dir: relRoot, importPath: importPath, name: prodName, srcs: sortedUnique(prodSources),
		internalExamples: make(map[string]string), externalExamples: make(map[string]string),
	}
	prodImports := make([]string, 0)
	testImports := make([]string, 0)
	hasEmbed := false
	for _, file := range parsed {
		hasEmbed = hasEmbed || file.hasEmbed
		if !file.test {
			prodImports = append(prodImports, file.imports...)
			continue
		}
		testImports = append(testImports, file.imports...)
		external := file.pkgName == prodName+"_test"
		if !external && file.pkgName != prodName {
			return projectPackage{}, false, fmt.Errorf("%s/%s has test package %s, want %s or %s_test", relRoot, file.name, file.pkgName, prodName, prodName)
		}
		if external {
			pkg.externalTests = append(pkg.externalTests, file.name)
			pkg.externalTestFns = append(pkg.externalTestFns, file.tests...)
			pkg.externalBenches = append(pkg.externalBenches, file.benches...)
			pkg.externalFuzz = append(pkg.externalFuzz, file.fuzz...)
			mergeExamples(pkg.externalExamples, file.examples)
			pkg.externalUnordered = append(pkg.externalUnordered, file.unordered...)
			if file.testMain {
				if pkg.testMain != "" {
					return projectPackage{}, false, fmt.Errorf("%s has multiple TestMain functions", relRoot)
				}
				pkg.testMain = "external"
			}
		} else {
			pkg.internalTests = append(pkg.internalTests, file.name)
			pkg.internalTestFns = append(pkg.internalTestFns, file.tests...)
			pkg.internalBenches = append(pkg.internalBenches, file.benches...)
			pkg.internalFuzz = append(pkg.internalFuzz, file.fuzz...)
			mergeExamples(pkg.internalExamples, file.examples)
			pkg.internalUnordered = append(pkg.internalUnordered, file.unordered...)
			if file.testMain {
				if pkg.testMain != "" {
					return projectPackage{}, false, fmt.Errorf("%s has multiple TestMain functions", relRoot)
				}
				pkg.testMain = "internal"
			}
		}
	}
	pkg.prodImports = sortedUnique(prodImports)
	pkg.testImports = sortedUnique(testImports)
	pkg.internalTests = sortedUnique(pkg.internalTests)
	pkg.externalTests = sortedUnique(pkg.externalTests)
	pkg.internalTestFns = sortedUnique(pkg.internalTestFns)
	pkg.externalTestFns = sortedUnique(pkg.externalTestFns)
	pkg.internalBenches = sortedUnique(pkg.internalBenches)
	pkg.externalBenches = sortedUnique(pkg.externalBenches)
	pkg.internalFuzz = sortedUnique(pkg.internalFuzz)
	pkg.externalFuzz = sortedUnique(pkg.externalFuzz)
	pkg.internalUnordered = sortedUnique(pkg.internalUnordered)
	pkg.externalUnordered = sortedUnique(pkg.externalUnordered)
	resources, err := projectResources(dir)
	if err != nil {
		return projectPackage{}, false, err
	}
	pkg.resources = resources
	if hasEmbed {
		embedPatterns, err := projectEmbedPatterns(dir, goos, goarch)
		if err != nil {
			return projectPackage{}, false, err
		}
		embedFiles, err := projectEmbedFiles(dir, embedPatterns)
		if err != nil {
			return projectPackage{}, false, err
		}
		pkg.embedFiles = embedFiles
	}
	return pkg, true, nil
}

func matchesProjectPlatform(dir, name, goos, goarch string) (bool, error) {
	ctx := build.Default
	ctx.GOOS = goos
	ctx.GOARCH = goarch
	ctx.CgoEnabled = false
	matched, err := ctx.MatchFile(dir, name)
	if err != nil {
		return false, fmt.Errorf("match build constraints for %s/%s: %w", dir, name, err)
	}
	return matched, nil
}

func inspectProjectFile(filename, name string) (projectFile, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filename, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return projectFile{}, fmt.Errorf("parse %s: %w", filename, err)
	}
	file := projectFile{name: name, pkgName: parsed.Name.Name, test: strings.HasSuffix(name, "_test.go"), examples: make(map[string]string)}
	for _, imported := range parsed.Imports {
		value, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return projectFile{}, fmt.Errorf("parse import in %s: %w", filename, err)
		}
		file.imports = append(file.imports, value)
	}
	for _, group := range parsed.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//go:embed ") {
				file.hasEmbed = true
			}
		}
	}
	if !file.test {
		file.imports = sortedUnique(file.imports)
		return file, nil
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		name := fn.Name.Name
		switch {
		case name == "TestMain":
			file.testMain = true
		case isGoTestName(name, "Test"):
			file.tests = append(file.tests, name)
		case isGoTestName(name, "Benchmark"):
			file.benches = append(file.benches, name)
		case isGoTestName(name, "Fuzz"):
			file.fuzz = append(file.fuzz, name)
		}
	}
	for _, example := range doc.Examples(parsed) {
		if example.Output == "" && !example.EmptyOutput {
			continue
		}
		name := "Example" + example.Name
		file.examples[name] = example.Output
		if example.Unordered {
			file.unordered = append(file.unordered, name)
		}
	}
	file.imports = sortedUnique(file.imports)
	file.tests = sortedUnique(file.tests)
	file.benches = sortedUnique(file.benches)
	file.fuzz = sortedUnique(file.fuzz)
	file.unordered = sortedUnique(file.unordered)
	return file, nil
}

func projectEmbedPatterns(dir, goos, goarch string) ([]string, error) {
	ctx := build.Default
	ctx.GOOS = goos
	ctx.GOARCH = goarch
	ctx.CgoEnabled = false
	pkg, err := ctx.ImportDir(dir, 0)
	if _, ok := err.(*build.NoGoError); ok {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read embed patterns for %s on %s/%s: %w", dir, goos, goarch, err)
	}
	patterns := append([]string(nil), pkg.EmbedPatterns...)
	patterns = append(patterns, pkg.TestEmbedPatterns...)
	patterns = append(patterns, pkg.XTestEmbedPatterns...)
	return sortedUnique(patterns), nil
}

func projectEmbedFiles(dir string, patterns []string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(dir, func(filename string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == "BUCK" {
			return nil
		}
		rel, err := filepath.Rel(dir, filename)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for _, pattern := range patterns {
			pattern = strings.TrimPrefix(pattern, "all:")
			matched, err := path.Match(pattern, rel)
			if err != nil {
				return fmt.Errorf("match embed pattern %q in %s: %w", pattern, dir, err)
			}
			if !matched {
				for ancestor := path.Dir(rel); ancestor != "."; ancestor = path.Dir(ancestor) {
					matched, err = path.Match(pattern, ancestor)
					if err != nil {
						return fmt.Errorf("match embed pattern %q in %s: %w", pattern, dir, err)
					}
					if matched {
						break
					}
				}
			}
			if matched {
				files = append(files, rel)
				break
			}
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func isGoTestName(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	if len(name) == len(prefix) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(name[len(prefix):])
	return !unicode.IsLower(r)
}

func projectResources(dir string) ([]string, error) {
	root := filepath.Join(dir, "testdata")
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	}
	resources := make([]string, 0)
	err := filepath.WalkDir(root, func(filename string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() == "BUCK" {
			return nil
		}
		rel, err := filepath.Rel(dir, filename)
		if err != nil {
			return err
		}
		resources = append(resources, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(resources)
	return resources, err
}

func writeProjectBUCK(root string, pkg projectPackage, all map[string][]projectPackage) (int, error) {
	var out bytes.Buffer
	out.WriteString("# Generated by build_defs/generator/main.go; changes will be overwritten.\n")
	out.WriteString("load(\"//build_defs/rules:kibou_go.bzl\", \"kibou_go_binary\", \"kibou_go_library\", \"kibou_go_test\")\n\n")

	prodDeps, prodUnresolved := projectDeps(pkg, pkg.prodImports, all)
	testDeps, testUnresolved := projectDeps(pkg, append(append([]string(nil), pkg.prodImports...), pkg.testImports...), all)
	unresolved := sortedUnique(append(prodUnresolved, testUnresolved...))
	if len(unresolved) > 0 {
		out.WriteString("# Unresolved external imports (targets are generated, but these packages need vendoring):\n")
		for _, imported := range unresolved {
			out.WriteString("#   " + imported + "\n")
		}
		out.WriteString("\n")
	}

	ruleName := "kibou_go_library"
	targetName := "lib"
	if pkg.name == "main" {
		ruleName = "kibou_go_binary"
		targetName = "bin"
	}
	out.WriteString(ruleName + "(\n")
	out.WriteString("    name = " + strconv.Quote(targetName) + ",\n")
	out.WriteString("    package_name = " + strconv.Quote(pkg.importPath) + ",\n")
	writeList(&out, "srcs", pkg.srcs)
	writeDictIdentity(&out, "embed_srcs", pkg.embedFiles)
	writeList(&out, "deps", prodDeps)
	out.WriteString("    visibility = [\"PUBLIC\"],\n")
	out.WriteString(")\n")

	if len(pkg.internalTests)+len(pkg.externalTests) > 0 {
		out.WriteString("\nkibou_go_test(\n")
		out.WriteString("    name = \"test\",\n")
		out.WriteString("    package_name = " + strconv.Quote(pkg.importPath) + ",\n")
		writeList(&out, "srcs", pkg.srcs)
		writeDictIdentity(&out, "embed_srcs", pkg.embedFiles)
		writeList(&out, "internal_test_srcs", pkg.internalTests)
		writeList(&out, "external_test_srcs", pkg.externalTests)
		writeList(&out, "internal_tests", pkg.internalTestFns)
		writeList(&out, "external_tests", pkg.externalTestFns)
		writeList(&out, "internal_benchmarks", pkg.internalBenches)
		writeList(&out, "external_benchmarks", pkg.externalBenches)
		writeList(&out, "internal_fuzz_targets", pkg.internalFuzz)
		writeList(&out, "external_fuzz_targets", pkg.externalFuzz)
		writeDict(&out, "internal_examples", pkg.internalExamples)
		writeDict(&out, "external_examples", pkg.externalExamples)
		writeList(&out, "internal_unordered_examples", pkg.internalUnordered)
		writeList(&out, "external_unordered_examples", pkg.externalUnordered)
		writeDictIdentity(&out, "resources", pkg.resources)
		writeList(&out, "deps", testDeps)
		if pkg.testCacheable {
			out.WriteString("    supports_test_execution_caching = True,\n")
		}
		if pkg.testMain != "" {
			out.WriteString("    test_main = " + strconv.Quote(pkg.testMain) + ",\n")
		}
		labels := make([]string, 0, 2)
		if len(unresolved) > 0 {
			labels = append(labels, "kibou_unresolved_deps")
		}
		if !pkg.testCacheable {
			labels = append(labels, "kibou_test_cache_not_audited")
		}
		writeList(&out, "labels", labels)
		out.WriteString("    visibility = [\"PUBLIC\"],\n")
		out.WriteString(")\n")
	}

	filename := filepath.Join(root, filepath.FromSlash(pkg.dir), "BUCK")
	return len(unresolved), os.WriteFile(filename, out.Bytes(), 0o644)
}

func projectDeps(self projectPackage, imports []string, all map[string][]projectPackage) ([]string, []string) {
	deps := make([]string, 0)
	unresolved := make([]string, 0)
	for _, imported := range sortedUnique(imports) {
		if imported == self.importPath || imported == "C" || isStandardImport(imported) {
			continue
		}
		if candidates, ok := all[imported]; ok {
			pkg := resolveProjectDependency(self, candidates)
			deps = append(deps, "//"+pkg.dir+":lib")
		} else {
			unresolved = append(unresolved, imported)
		}
	}
	return sortedUnique(deps), sortedUnique(unresolved)
}

func resolveProjectDependency(self projectPackage, candidates []projectPackage) projectPackage {
	if self.vendored {
		for _, candidate := range candidates {
			if candidate.vendored && candidate.moduleDir == self.moduleDir {
				return candidate
			}
		}
	}
	for _, candidate := range candidates {
		if !candidate.vendored {
			return candidate
		}
	}
	for _, candidate := range candidates {
		if candidate.moduleDir == self.moduleDir {
			return candidate
		}
	}
	return candidates[0]
}

func isStandardImport(importPath string) bool {
	first := strings.SplitN(importPath, "/", 2)[0]
	return !strings.Contains(first, ".")
}

func mergeExamples(dst, src map[string]string) {
	for name, output := range src {
		dst[name] = output
	}
}

func sortedUnique(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func writeList(out *bytes.Buffer, name string, values []string) {
	if len(values) == 0 {
		return
	}
	out.WriteString("    " + name + " = [\n")
	for _, value := range values {
		out.WriteString("        " + strconv.Quote(value) + ",\n")
	}
	out.WriteString("    ],\n")
}

func writeDictIdentity(out *bytes.Buffer, name string, values []string) {
	if len(values) == 0 {
		return
	}
	items := make(map[string]string, len(values))
	for _, value := range values {
		items[value] = value
	}
	writeDict(out, name, items)
}

func writeDict(out *bytes.Buffer, name string, values map[string]string) {
	if len(values) == 0 {
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out.WriteString("    " + name + " = {\n")
	for _, key := range keys {
		out.WriteString("        " + strconv.Quote(key) + ": " + strconv.Quote(values[key]) + ",\n")
	}
	out.WriteString("    },\n")
}

func filteredFiles(files []string) []string {
	result := make([]string, 0, len(files))
	for _, file := range files {
		if strings.HasPrefix(file, ".") || strings.HasPrefix(file, "_") || strings.HasPrefix(file, "#") ||
			strings.HasSuffix(file, "_test.go") || strings.HasSuffix(file, "_test.s") ||
			strings.HasSuffix(file, ".pgo") || strings.HasSuffix(file, "~") {
			continue
		}
		result = append(result, file)
	}
	sort.Strings(result)
	return result
}

func sortedCopy(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
