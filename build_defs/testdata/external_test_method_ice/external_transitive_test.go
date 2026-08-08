package externaltestmethodice_test

import (
	"testing"

	"code.kibou.tools/build_defs/testdata/external_test_method_ice/wrapper"
)

func TestExternalMethodThroughTransitiveDependency(t *testing.T) {
	value := wrapper.NewBox().Value
	if got, want := value.TestOnlyMethod(), "test-only"; got != want {
		t.Fatalf("TestOnlyMethod() = %q, want %q", got, want)
	}
}
