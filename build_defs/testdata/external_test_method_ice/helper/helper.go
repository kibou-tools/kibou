package helper

import fixture "code.kibou.tools/build_defs/testdata/external_test_method_ice"

func NewValue() *fixture.Value { return new(fixture.Value) }

type Box struct {
	Value *fixture.Value
}

func NewBox() Box { return Box{Value: NewValue()} }
