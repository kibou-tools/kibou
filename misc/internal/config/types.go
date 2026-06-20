// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package config

import (
	"strings"

	"code.kibou.tools/base/core/option"
	"code.kibou.tools/base/errorx"
)

// WorkspaceConfigJSON is the top-level JSON input.
type WorkspaceConfigJSON struct {
	// ForkedFolders lists forked upstream folders. Always non-empty.
	ForkedFolders []ForkedFolderJSON `json:"forked_folders"`
	// BranchMappings lists local branch mappings and upstream branches. Always non-empty.
	BranchMappings []BranchMappingJSON `json:"branch_mappings"`
}

// ForkedFolderJSON describes one forked upstream folder.
type ForkedFolderJSON struct {
	// Folder is the forked folder path relative to the repo root (for example,
	// "go" or "third_party/build-tools"). Always non-empty.
	Folder string `json:"folder"`
	// GitHubProject is the upstream repository in "<owner>/<repo>" form (for example, "golang/go"). Always non-empty.
	GitHubProject string `json:"gh_project"`
	// AutoSync reports whether the upstream-sync automation should pull this
	// folder. Absent means true; set false to record a fork that is carried
	// but only updated manually.
	AutoSync option.Option[bool] `json:"auto_sync"`
}

// BranchMappingJSON describes one local branch and upstream project list in JSON input.
type BranchMappingJSON struct {
	// LocalBranch is the local target branch name (for example, "main"). Always non-empty.
	LocalBranch string `json:"branch"`
	// Upstream lists upstream projects to sync into LocalBranch. Always non-empty.
	Upstream []UpstreamRepoJSON `json:"upstream"`
}

// UpstreamRepoJSON describes one upstream project and branch in JSON input.
type UpstreamRepoJSON struct {
	// GitHubProject is the upstream repository in "<owner>/<repo>" form (for example, "golang/go"). Always non-empty.
	GitHubProject string `json:"gh_project"`
	// Branch is the upstream branch name (for example, "master"). Always non-empty.
	Branch string `json:"branch"`
}

// ForkedFolder describes one validated forked folder mapping.
type ForkedFolder struct {
	// Folder is the forked folder path relative to the repo root (for example,
	// "go" or "third_party/build-tools"). Always non-empty.
	Folder string
	// GitHubRepo is the upstream repository in "<owner>/<repo>" form. Always non-empty.
	GitHubRepo GitHubRepo
	// AutoSync controls whether automation should update this folder
	// based on changes to upstream.
	//
	// Defaults to true when unspecified in JSON.
	AutoSync bool
}

// BranchMappings is the validated in-memory branch mapping representation.
type BranchMappings struct {
	// ByLocalBranch maps branch name to mapping. Always non-empty.
	ByLocalBranch map[string]BranchMapping
}

// BranchMapping describes one validated local branch mapping.
type BranchMapping struct {
	// LocalBranch is the local target branch name (for example, "main"). Always non-empty.
	LocalBranch string
	// UpstreamMap contains upstream projects keyed by full GitHub repo. Always non-empty.
	UpstreamMap UpstreamMap
}

// UpstreamMap maps full GitHub repos to upstream project config.
type UpstreamMap struct {
	// ByGitHubRepo maps full upstream repo key (for example, "golang/go") to upstream config. Always non-empty.
	ByGitHubRepo map[GitHubRepo]UpstreamRepo
}

// UpstreamRepo describes one validated upstream project and branch.
type UpstreamRepo struct {
	// GitHubRepo is the upstream repository in "<owner>/<repo>" form (for example, "golang/go"). Always non-empty.
	GitHubRepo GitHubRepo
	// Branch is the upstream branch name (for example, "master"). Always non-empty.
	Branch string
}

// GitHubRepo is a validated "<owner>/<repo>" repository identifier.
type GitHubRepo string

func parseGitHubRepo(s string) (GitHubRepo, error) {
	owner, repo, ok := strings.Cut(s, "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return "", errorx.Newf("nostack", "invalid gh_project %q: want <owner>/<repo>", s)
	}
	return GitHubRepo(s), nil
}
