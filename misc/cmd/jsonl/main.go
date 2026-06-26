// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package main

import (
	"context" //nolint:depguard // cli main is the program boundary.
	"io"
	"os"

	"github.com/urfave/cli/v3"
	"golang.org/x/term"

	"code.kibou.tools/base/assert"
	. "code.kibou.tools/base/core"
	"code.kibou.tools/base/core/pathx"
	"code.kibou.tools/base/core/result"
	"code.kibou.tools/base/errorx"
	"code.kibou.tools/base/fsx"
	"code.kibou.tools/base/fsx/fsx_temp"
	"code.kibou.tools/base/logx"
	"code.kibou.tools/base/syscaps"
	"code.kibou.tools/misc/internal/go_test"
)

func main() {
	logger := logx.NewLogger(os.Stderr, logx.ColorSupport_AutoDetect)
	app := &cli.Command{
		Name:  "jsonl",
		Usage: "Operate on JSON Lines streams",
		Description: `The jsonl tool is used to operate on a closed set
of JSONL formats, to massage them into a uniform shape.

The problem with consuming 'go test -json' and
'go tool dist test -json' output directly is that these streams
are not always strictly JSONL, and can have other output in-between.
This makes it trickier to ingest the data elsewhere when edge cases
come up.

Additionally, there doesn't seem to be any existing tool
which converts these JSON streams to the default 'go test'
human-readable output (no -v). The jsonl tool can write both
normalized JSONL and human-readable output.

In the future, we may extend this to JSONL output from
other tools that we rely on.

This tool is only meant for use within the Kibou monorepo.
`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:      "format",
				Usage:     "input format to render: go-test",
				TakesFile: true,
				OnlyOnce:  true,
				Value:     "go-test",
			},
			&cli.StringFlag{
				Name:      "input",
				Usage:     "input path, or :stdin for standard input",
				OnlyOnce:  true,
				TakesFile: true,
				Value:     ":stdin",
			},
			&cli.StringFlag{
				Name:  "color",
				Usage: "color output: auto, always, never",
				Value: string(ColorMode_Auto),
			},
			&cli.StringFlag{
				Name:      "pretty-output",
				Usage:     "human-readable render output path, or :stdout/:discard",
				OnlyOnce:  true,
				TakesFile: true,
				Value:     ":discard",
			},
			&cli.StringFlag{
				Name:      "jsonl-output",
				Usage:     "normalized JSONL output path, or :stdout/:discard",
				OnlyOnce:  true,
				TakesFile: true,
				Value:     ":stdout",
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) (retErr error) {
			if cmd.String("format") != "go-test" {
				return errorx.Newf("nostack", "unsupported --format %q, want go-test", cmd.String("format"))
			}

			mode, err := parseColorMode(cmd.String("color"))
			if err != nil {
				return err
			}

			inputPath, err := parseInputPathOrSpecial(cmd.String("input"))
			if err != nil {
				return err
			}
			prettyPath, err := parseOutputPathOrSpecial(cmd.String("pretty-output"))
			if err != nil {
				return err
			}
			jsonlPath, err := parseOutputPathOrSpecial(cmd.String("jsonl-output"))
			if err != nil {
				return err
			}
			if err := validateOutputPaths(prettyPath, jsonlPath); err != nil {
				return err
			}

			input, err := openInput(inputPath)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := input.Close(); closeErr != nil {
					logger.Warn("close --input", "path", cmd.String("input"), "err", closeErr.Error())
				}
			}()

			prettyOutput, err := openOutput(prettyPath, "pretty-output")
			if err != nil {
				return err
			}
			defer func() {
				status := result.NewStatusFromError(retErr)
				if closeErr := prettyOutput.Close(status); closeErr != nil {
					retErr = errorx.Join(retErr, closeErr)
				}
			}()
			jsonlOutput, err := openOutput(jsonlPath, "jsonl-output")
			if err != nil {
				return err
			}
			defer func() {
				status := result.NewStatusFromError(retErr)
				if closeErr := jsonlOutput.Close(status); closeErr != nil {
					retErr = errorx.Join(retErr, closeErr)
				}
			}()

			return renderGoTest(logger, colorizerForMode(mode, prettyPath), input, prettyOutput.writer, jsonlOutput.writer)
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

type ColorMode string

const (
	ColorMode_Auto   ColorMode = "auto"
	ColorMode_Always ColorMode = "always"
	ColorMode_Never  ColorMode = "never"
)

func parseColorMode(value string) (ColorMode, error) {
	mode := ColorMode(value)
	switch mode {
	case ColorMode_Auto, ColorMode_Always, ColorMode_Never:
		return mode, nil
	default:
		return "", errorx.Newf("nostack", "unsupported --color %q, want auto, always, never", value)
	}
}

func colorizerForMode(mode ColorMode, out OutputPath) go_test.Colorizer {
	return go_test.NewColorizer(mode == ColorMode_Always ||
		(mode == ColorMode_Auto && out.kind == OutputPath_Stdout && term.IsTerminal(int(os.Stdout.Fd()))))
}

func validateOutputPaths(pretty OutputPath, jsonl OutputPath) error {
	if pretty.kind == OutputPath_Stdout && jsonl.kind == OutputPath_Stdout {
		return errorx.Newf("nostack", "--pretty-output and --jsonl-output cannot both be :stdout")
	}
	return nil
}

func openInput(input InputPath) (io.ReadCloser, error) {
	switch input.kind {
	case InputPath_Stdin:
		return io.NopCloser(os.Stdin), nil
	case InputPath_FilePath:
		fs, rel, err := fsForPath(input.path)
		if err != nil {
			return nil, err
		}
		f, err := fsx.OpenReadOnly(fs, rel)
		if err != nil {
			return nil, errorx.Wrapf("nostack", err, "open --input %q", input.path)
		}
		return f, nil
	default:
		return nil, assert.PanicUnknownCase[error](input.kind)
	}
}

// outputTarget is an opened output destination.
type outputTarget struct {
	// writer is always non-nil.
	writer io.Writer
	// fs is set only for filesystem path outputs.
	fs fsx.FS
	// file is set only for filesystem path outputs.
	file *OutputFile
}

func openOutput(path OutputPath, flag string) (outputTarget, error) {
	switch path.kind {
	case OutputPath_Stdout:
		return outputTarget{writer: os.Stdout, fs: nil, file: nil}, nil
	case OutputPath_Discard:
		return outputTarget{writer: io.Discard, fs: nil, file: nil}, nil
	case OutputPath_FilePath:
	default:
		return outputTarget{}, assert.PanicUnknownCase[error](path.kind)
	}

	fs, rel, err := fsForPath(path.path)
	if err != nil {
		return outputTarget{}, errorx.Wrapf("nostack", err, "open --%s %q", flag, path.path)
	}

	info, err := fs.Stat(rel, fsx.StatOptions{FollowFinalSymlink: true, OnErrorTraverseParents: false})
	if err == nil {
		if info.IsDir() {
			return outputTarget{}, errorx.Newf("nostack", "open --%s %q: is a directory", flag, path.path)
		}
		base := rel.BaseName()
		f, createErr := fsx_temp.CreateFile(fs, pathx.Dot(),
			fsx_temp.Names([]byte("."+base.String()+".tmp."), syscaps.TempFileFragments(), nil),
			fsx.NewOpenOptions(fsx.OpenRW_WriteOnly))
		if createErr != nil {
			return outputTarget{}, errorx.Wrapf("nostack", createErr, "open --%s %q: create temporary output file", flag, path.path)
		}
		out := &OutputFile{file: f, target: rel, temp: Some(pathx.Dot().JoinOne(f.Name()))}
		return outputTarget{writer: out, fs: fs, file: out}, nil
	}
	if !errorx.GetRootCauseAsValue(err, fsx.ErrNotExist) {
		return outputTarget{}, errorx.Wrapf("nostack", err, "open --%s %q: stat output path", flag, path.path)
	}

	f, err := fs.OpenFile(rel, fsx.NewOpenOptions(fsx.OpenRW_WriteOnly).
		WithMode(fsx.OpenMode_CreateNew).
		MustBuild())
	if err != nil {
		return outputTarget{}, errorx.Wrapf("nostack", err, "open --%s %q: create output file", flag, path.path)
	}
	out := &OutputFile{file: f, target: rel, temp: None[pathx.RelPath]()}
	return outputTarget{writer: out, fs: fs, file: out}, nil
}

func (out outputTarget) Close(status result.Status) error {
	if out.file == nil {
		return nil
	}
	return out.file.Close(out.fs, status)
}

// OutputFile is a file being written, optionally via a temporary that replaces
// an existing target atomically on success. See [OutputFile.Close].
type OutputFile struct {
	// The open file where writes are directed.
	//
	// This is a temporary file when temp is Some, otherwise,
	// this corresponds to target.
	file fsx.File
	// target is the final destination path, relative to the output FS root.
	target pathx.RelPath
	// temp, when set, is the temporary file to rename onto target at a
	// successful Close. It is None when file already is the target.
	temp Option[pathx.RelPath]
}

func (o *OutputFile) Write(p []byte) (int, error) {
	return o.file.Write(p)
}

// Close closes the file and finalizes it based on status.
//
// If status is result.Status_Success, then the temporary output file
// is moved to the destination. Otherwise, the temporary file is
// deleted.
func (o *OutputFile) Close(fs fsx.FS, status result.Status) error {
	closeErr := o.file.Close()
	switch status {
	case result.Status_Success:
		if closeErr != nil {
			return errorx.Join(closeErr, o.delete(fs))
		}
		if temp, ok := o.temp.Get(); ok {
			if err := fs.Rename(temp, o.target); err != nil {
				return errorx.Join(errorx.Wrapf("nostack", err, "replace output file"), o.delete(fs))
			}
		}
		return nil
	case result.Status_Failure:
		return errorx.Join(closeErr, o.delete(fs))
	default:
		return assert.PanicUnknownCase[error](status)
	}
}

func (o *OutputFile) delete(fs fsx.FS) error {
	return fs.RemoveAll(o.temp.ValueOr(o.target))
}

// fsForPath creates a scoped fsx.FS for accessing this file,
// and creates a basename-only RelPath for the file.
func fsForPath(path string) (fsx.FS, pathx.RelPath, error) {
	abs, err := syscaps.ResolvePath(path)
	if err != nil {
		return nil, pathx.RelPath{}, err
	}

	dir, name := abs.Split()
	fs, err := syscaps.FS(dir)
	if err != nil {
		return nil, pathx.RelPath{}, err
	}
	return fs, pathx.NewRelPathFromName(name), nil
}
