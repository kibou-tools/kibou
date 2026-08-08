package externaltestmethodice_test

import (
	"testing"

	left "code.kibou.tools/build_defs/testdata/external_test_method_ice/left/same"
	right "code.kibou.tools/build_defs/testdata/external_test_method_ice/right/same"
)

func TestExternalMethodThroughSameBasenames(t *testing.T) {
	for name, value := range map[string]interface{ TestOnlyMethod() string }{
		"left":  left.NewValue(),
		"right": right.NewValue(),
	} {
		if got, want := value.TestOnlyMethod(), "test-only"; got != want {
			t.Errorf("%s TestOnlyMethod() = %q, want %q", name, got, want)
		}
	}
}
