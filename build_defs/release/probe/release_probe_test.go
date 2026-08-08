package releaseprobe

import (
	"debug/buildinfo"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

type versionOutput struct {
	Runtime string `json:"runtime"`
}

func TestStampedVersionProbe(t *testing.T) {
	name := "version-probe"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join("testdata", name)
	output, err := exec.Command(path).Output()
	if err != nil {
		t.Fatalf("run stamped version probe: %v", err)
	}
	var version versionOutput
	if err := json.Unmarshal(output, &version); err != nil {
		t.Fatalf("decode stamped version probe output %q: %v", output, err)
	}
	want := os.Getenv("KIBOU_EXPECTED_GO_VERSION")
	if want == "" {
		t.Fatal("KIBOU_EXPECTED_GO_VERSION is empty")
	}
	if version.Runtime != want {
		t.Fatalf("runtime.Version() = %q, want %q", version.Runtime, want)
	}
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		t.Fatalf("read stamped version probe build info: %v", err)
	}
	if info.GoVersion != want {
		t.Fatalf("build info GoVersion = %q, want %q", info.GoVersion, want)
	}
}
