package mixed

import (
	"os"
	"strings"
	"testing"
)

func TestInternal(t *testing.T) {
	contents, err := os.ReadFile("testdata/message.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got, suffix := strings.TrimSpace(string(contents)), " from a declared resource"; !strings.HasSuffix(got, suffix) {
		t.Fatalf("resource = %q, want suffix %q", got, suffix)
	}
}
