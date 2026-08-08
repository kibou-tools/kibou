package externaltestmethodice_test

import (
	"testing"

	fixture "code.kibou.tools/build_defs/testdata/external_test_method_ice"
)

func TestExternalMethodDirect(t *testing.T) {
	value := new(fixture.Value)
	if got, want := value.TestOnlyMethod(), "test-only"; got != want {
		t.Fatalf("TestOnlyMethod() = %q, want %q", got, want)
	}
}
