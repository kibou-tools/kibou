package externaltestmethodice_test

import (
	"testing"

	fixture "code.kibou.tools/build_defs/testdata/external_test_method_ice"
	"code.kibou.tools/build_defs/testdata/external_test_method_ice/helper"
)

func TestExternalMethod(t *testing.T) {
	value := helper.NewValue()
	if got, want := value.TestOnlyMethod(), "test-only"; got != want {
		t.Fatalf("TestOnlyMethod() = %q, want %q", got, want)
	}
	var _ *fixture.Value = value
}
