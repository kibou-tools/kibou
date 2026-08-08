package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInspectProjectPackageSplitsTestsAndResources(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "sample.go"), `package sample

import (
	_ "embed"
	"fmt"
)

var _ = fmt.Sprintf

//go:embed testdata/input.txt
var input string
`)
	writeTestFile(t, filepath.Join(dir, "sample_test.go"), `package sample

import "testing"

func TestInternal(t *testing.T) {}
func BenchmarkInternal(b *testing.B) {}
func FuzzInternal(f *testing.F) {}
`)
	writeTestFile(t, filepath.Join(dir, "sample_external_test.go"), `package sample_test

import "testing"

func TestExternal(t *testing.T) {}
`)
	writeTestFile(t, filepath.Join(dir, "testdata", "input.txt"), "declared input\n")

	pkg, ok, err := inspectProjectPackage(
		dir,
		"module/sample",
		"example.com/module/sample",
		[]string{"sample.go", "sample_test.go", "sample_external_test.go"},
		"linux",
		"amd64",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("package was not detected")
	}
	assertStrings(t, pkg.srcs, []string{"sample.go"})
	assertStrings(t, pkg.internalTests, []string{"sample_test.go"})
	assertStrings(t, pkg.externalTests, []string{"sample_external_test.go"})
	assertStrings(t, pkg.internalTestFns, []string{"TestInternal"})
	assertStrings(t, pkg.externalTestFns, []string{"TestExternal"})
	assertStrings(t, pkg.internalBenches, []string{"BenchmarkInternal"})
	assertStrings(t, pkg.internalFuzz, []string{"FuzzInternal"})
	assertStrings(t, pkg.resources, []string{"testdata/input.txt"})
	assertStrings(t, pkg.embedFiles, []string{"testdata/input.txt"})
}

func TestResolveProjectDependencyKeepsVendorGraphLocal(t *testing.T) {
	workspace := projectPackage{dir: "tools/example", importPath: "example.com/dep"}
	baseVendor := projectPackage{dir: "base/vendor/example.com/dep", moduleDir: "base", vendored: true, importPath: "example.com/dep"}
	delveVendor := projectPackage{dir: "delve/vendor/example.com/dep", moduleDir: "delve", vendored: true, importPath: "example.com/dep"}
	candidates := []projectPackage{baseVendor, delveVendor, workspace}

	got := resolveProjectDependency(projectPackage{moduleDir: "base", vendored: true}, candidates)
	if got.dir != baseVendor.dir {
		t.Fatalf("vendored dependency resolved to %q, want %q", got.dir, baseVendor.dir)
	}
	got = resolveProjectDependency(projectPackage{moduleDir: "misc"}, candidates)
	if got.dir != workspace.dir {
		t.Fatalf("workspace dependency resolved to %q, want %q", got.dir, workspace.dir)
	}
}

func TestWriteProjectBUCKEmitsRunnableBinary(t *testing.T) {
	root := t.TempDir()
	pkg := projectPackage{
		dir:        "cmd/tool",
		importPath: "example.com/project/cmd/tool",
		name:       "main",
		srcs:       []string{"main.go"},
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(pkg.dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := writeProjectBUCK(root, pkg, map[string][]projectPackage{pkg.importPath: {pkg}}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(pkg.dir), "BUCK"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, want := range []string{
		`load("//build_defs/rules:kibou_go.bzl", "kibou_go_binary", "kibou_go_library", "kibou_go_test")`,
		"kibou_go_binary(\n",
		"    name = \"bin\",\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated BUCK file does not contain %q:\n%s", want, text)
		}
	}
}

func TestWriteProjectBUCKOptsGeneratedTestsIntoNativeCache(t *testing.T) {
	root := t.TempDir()
	pkg := projectPackage{
		dir:             "library",
		importPath:      "example.com/project/library",
		name:            "library",
		srcs:            []string{"library.go"},
		externalTests:   []string{"library_test.go"},
		externalTestFns: []string{"TestLibrary"},
		resources:       []string{"testdata/input.txt"},
		testCacheable:   true,
	}
	if err := os.MkdirAll(filepath.Join(root, pkg.dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := writeProjectBUCK(root, pkg, map[string][]projectPackage{pkg.importPath: {pkg}}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(root, pkg.dir, "BUCK"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, want := range []string{
		"external_test_srcs = [\n        \"library_test.go\",\n    ],",
		"resources = {\n        \"testdata/input.txt\": \"testdata/input.txt\",\n    },",
		"supports_test_execution_caching = True,",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated BUCK file does not contain %q:\n%s", want, text)
		}
	}
}

func writeTestFile(t *testing.T, filename, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
