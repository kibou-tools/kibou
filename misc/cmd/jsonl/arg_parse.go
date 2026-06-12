// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package main

import (
	"strings"

	"code.kibou.tools/base/errorx"
)

// InputPath is a parsed input path or input special form.
type InputPath struct {
	kind InputPathKind
	// path is set only when kind is InputPath_FilePath.
	path string
}

type InputPathKind uint8

const (
	InputPath_FilePath InputPathKind = iota + 1
	InputPath_Stdin
)

func parseInputPathOrSpecial(value string) (InputPath, error) {
	if value == "" {
		return InputPath{}, errorx.Newf("nostack", "input path must be non-empty")
	}
	if value == ":stdin" {
		return InputPath{kind: InputPath_Stdin, path: ""}, nil
	}
	if strings.HasPrefix(value, "::") {
		return InputPath{kind: InputPath_FilePath, path: value[1:]}, nil
	}
	if strings.HasPrefix(value, ":") {
		return InputPath{}, errorx.Newf("nostack", "unsupported input special path %q, want :stdin", value)
	}
	return InputPath{kind: InputPath_FilePath, path: value}, nil
}

// OutputPath is a parsed output path or output special form.
type OutputPath struct {
	kind OutputPathKind
	// path is set only when kind is OutputPath_FilePath.
	path string
}

type OutputPathKind uint8

const (
	OutputPath_FilePath OutputPathKind = iota + 1
	OutputPath_Stdout
	OutputPath_Discard
)

func parseOutputPathOrSpecial(value string) (OutputPath, error) {
	if value == "" {
		return OutputPath{}, errorx.Newf("nostack", "output path must be non-empty")
	}
	switch value {
	case ":stdout":
		return OutputPath{kind: OutputPath_Stdout, path: ""}, nil
	case ":discard":
		return OutputPath{kind: OutputPath_Discard, path: ""}, nil
	}
	if strings.HasPrefix(value, "::") {
		return OutputPath{kind: OutputPath_FilePath, path: value[1:]}, nil
	}
	if strings.HasPrefix(value, ":") {
		return OutputPath{}, errorx.Newf("nostack", "unsupported output special path %q, want :stdout or :discard", value)
	}
	return OutputPath{kind: OutputPath_FilePath, path: value}, nil
}
