// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package main

import (
	"bytes"
	"os"
	"testing"

	"code.kibou.tools/base/check"
	"code.kibou.tools/base/core/pathx"
	"code.kibou.tools/base/envx"
	"code.kibou.tools/base/fsx/fsx_testkit"
	"code.kibou.tools/base/logx"
	"code.kibou.tools/base/syscaps"
	"code.kibou.tools/misc/internal/config"
)

func TestList(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	repoFS := fsx_testkit.TempDirFS(h)
	fsx_testkit.WriteTree(h, repoFS, map[string]string{
		"alpha/go.mod": "module alpha\n",
		"beta/go.mod":  "module beta\n",
		"gamma/go.mod": "module gamma\n",
		"delta/":       "",
		"file.txt":     "not a dir\n",
	})

	ws := Workspace{
		FS:     repoFS,
		Runner: syscaps.CmdRunner{Env: envx.Empty()},
		Config: config.WorkspaceConfig{
			ForkedFolders: map[pathx.RelPath]config.ForkedFolder{
				pathx.MustParseRelPath("beta"): {Folder: "beta", GitHubRepo: "example/beta", AutoSync: true},
			},
			BranchMappings: config.BranchMappings{ByLocalBranch: nil},
		},
	}

	tests := []struct {
		name       string
		provenance ListProvenance
		want       string
	}{
		{"All", ListProvenance_All, "alpha\nbeta\ngamma\n"},
		{"FirstParty", ListProvenance_FirstParty, "alpha\ngamma\n"},
		{"Forked", ListProvenance_Forked, "beta\n"},
	}
	for _, tt := range tests {
		h.Run(tt.name, func(h check.Harness) {
			h.Parallel()
			var buf bytes.Buffer
			logger := logx.NewLogger(os.Stderr, logx.ColorSupport_Disable)
			err := ws.List(logger, &buf, ListOptions{Type: ListType_GoModules, Provenance: tt.provenance})
			h.NoErrorf(err, "List(%v)", tt.provenance)
			h.Assertf(buf.String() == tt.want, "got %q, want %q", buf.String(), tt.want)
		})
	}
}
