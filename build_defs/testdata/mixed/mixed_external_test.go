package mixed_test

import (
	"testing"

	"code.kibou.tools/build_defs/testdata/mixed"
)

func TestExternal(t *testing.T) {
	if got, want := mixed.Greeting(), "hello"; got != want {
		t.Fatalf("Greeting() = %q, want %q", got, want)
	}
}
