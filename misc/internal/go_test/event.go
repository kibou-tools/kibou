// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package go_test models and renders the JSON records emitted by
// `go test -json` and `go tool dist test -json`.
package go_test

// TestEvent is the union of JSON records emitted by go test -json and
// go tool dist test -json.
type TestEvent struct {
	// Time is the RFC3339 timestamp for the event. It is omitted on build-output
	// and build-fail events. Kept as encoded text because this package does not
	// do time arithmetic.
	Time string `json:"Time"`

	// Action identifies the event kind: lifecycle, output, result, or metadata.
	Action TestAction `json:"Action"`
	// Package is the import path for the package being tested.
	Package string `json:"Package"`
	// Test is the test, example, benchmark, fuzz target, or subtest name, if any.
	// Subtests use testing's slash-separated full name, such as "TestFoo/bar";
	// parent and child subtests each get their own lifecycle/result events.
	Test string `json:"Test"`
	// Elapsed is the result duration in seconds, set for pass/fail/skip/bench events.
	Elapsed *float64 `json:"Elapsed"`
	// Output is stdout/stderr text for output events.
	Output string `json:"Output"`
	// OutputType classifies output events as regular, frame, or error output.
	OutputType TestOutputType `json:"OutputType"`
	// FailedBuild is the import path whose build failure caused a fail event.
	FailedBuild string `json:"FailedBuild"`
	// Key is the attribute key for events emitted on calls to [testing.T.Attr].
	Key string `json:"Key"`
	// Value is the attribute value for events emitted on calls to [testing.T.Attr].
	Value string `json:"Value"`
	// Path is the artifact directory path for artifacts events. For go test
	// -artifacts, upstream testing emits an absolute directory under
	// -outputdir/_artifacts.
	Path string `json:"Path"`
	// ImportPath is the package being built, set on build-output and build-fail
	// events. Unlike Package, build events carry ImportPath rather than Package
	// (see `go doc cmd/go`); it matches the FailedBuild of the eventual fail.
	ImportPath string `json:"ImportPath"`
}

// Event is a shorthand alias for [TestEvent].
type Event = TestEvent

// TestAction is [TestEvent.Action].
//
// In addition to test lifecycle and output records, the supported producers
// emit attr, artifacts, build-output, and build-fail records.
type TestAction string

// Action is a shorthand alias for [TestAction].
type Action = TestAction

const (
	// Action_Start means the test binary is about to be executed.
	Action_Start Action = "start"
	// Action_Run means a test, example, benchmark, or subtest started.
	Action_Run Action = "run"
	// Action_Pause means a parallel test paused.
	Action_Pause Action = "pause"
	// Action_Cont means a paused parallel test continued running.
	Action_Cont Action = "cont"
	// Action_Pass means a test, benchmark, or package passed.
	Action_Pass Action = "pass"
	// Action_Bench means a benchmark printed log output but did not fail.
	Action_Bench Action = "bench"
	// Action_Fail means a test, benchmark, or package failed.
	Action_Fail Action = "fail"
	// Action_Output means Output contains test stdout/stderr text.
	Action_Output Action = "output"
	// Action_Skip means a test or package was skipped, or a package had no tests.
	Action_Skip Action = "skip"
	// Action_Attr records testing.T.Attr metadata; Key and Value are set.
	Action_Attr Action = "attr"
	// Action_Artifacts records testing.T.ArtifactDir metadata; Path is set.
	Action_Artifacts Action = "artifacts"
	// Action_BuildOutput carries a portion of the build's output (compiler or vet
	// diagnostics) on ImportPath when a package fails to build.
	Action_BuildOutput Action = "build-output"
	// Action_BuildFail marks that the build for ImportPath failed.
	Action_BuildFail Action = "build-fail"
)

func (a TestAction) Known() bool {
	switch a {
	case Action_Start, Action_Run, Action_Pause, Action_Cont,
		Action_Pass, Action_Bench, Action_Fail, Action_Output,
		Action_Skip, Action_Attr, Action_Artifacts,
		Action_BuildOutput, Action_BuildFail:
		return true
	default:
		return false
	}
}

// TestOutputType classifies Action=output events.
type TestOutputType string

// OutputType is a shorthand alias for [TestOutputType].
type OutputType = TestOutputType

const (
	// OutputType_Regular is ordinary test output.
	OutputType_Regular OutputType = ""
	// OutputType_Frame is output synthesized from test framing lines such as
	// "=== RUN" or "--- FAIL:".
	OutputType_Frame OutputType = "frame"
	// OutputType_Error is output produced by Error, Errorf, Fatal, or Fatalf.
	OutputType_Error OutputType = "error"
	// OutputType_ErrorContinue continues a multi-line error output event.
	OutputType_ErrorContinue OutputType = "error-continue"
)

func (typ TestOutputType) Known() bool {
	switch typ {
	case OutputType_Regular, OutputType_Frame, OutputType_Error, OutputType_ErrorContinue:
		return true
	default:
		return false
	}
}
