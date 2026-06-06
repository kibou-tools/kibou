// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package main

import (
	"fmt"

	"code.kibou.tools/base/cmdx"
	. "code.kibou.tools/base/core"
	"code.kibou.tools/base/fsx"
	"code.kibou.tools/base/logx"
)

func (ws Workspace) runUpdate(ctx logx.LogCtx, dir AbsPath, localBranch string, projects []fsx.Name) error {
	for _, project := range projects {
		upstream, err := ws.Config.UpstreamForProject(localBranch, project)
		if err != nil {
			return err
		}
		upstreamURL := fmt.Sprintf("https://github.com/%s.git", upstream.GitHubRepo)
		ctx.Info("running subtree pull", "project", project, "upstream", upstreamURL, "upstream_branch", upstream.Branch)
		subtreePullCmd := cmdx.New(
			"git", "subtree", "pull", "--prefix", project.String(), upstreamURL, upstream.Branch,
		).In(dir)
		output, err := ws.Runner.Run(ctx, subtreePullCmd, cmdx.RunOptionsDefault().WithCaptureStdout())
		if err != nil {
			if output.Stdout != "" {
				ctx.Info("subtree pull stdout", "output", output.Stdout)
			}
			return err
		}
	}
	return nil
}
