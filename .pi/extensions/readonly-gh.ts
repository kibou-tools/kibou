import { spawn } from "node:child_process";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { Type } from "@sinclair/typebox";

const MAX_OUTPUT_BYTES = 128 * 1024;
const TIMEOUT_MS = 30_000;

const commands = [
	"issue-list",
	"issue-view",
	"issue-status",
	"pr-list",
	"pr-view",
	"pr-diff",
	"pr-checks",
	"pr-status",
	"search-issues",
	"search-prs",
	"search-repos",
	"run-list",
	"run-view",
	"workflow-list",
	"workflow-view",
	"repo-view",
	"release-list",
	"release-view",
	"status",
] as const;
const formats = ["text", "json"] as const;

type Command = (typeof commands)[number];
type Format = (typeof formats)[number];

const stringEnum = <T extends readonly string[]>(values: T) => Type.Union(values.map((value) => Type.Literal(value)) as never);

interface GHParams {
	command: Command;
	repo?: string;
	number?: number;
	query?: string;
	runId?: string;
	jobId?: string;
	workflow?: string;
	release?: string;
	limit?: number;
	format?: Format;
	log?: boolean;
	logFailed?: boolean;
	comments?: boolean;
}

function addRepo(args: string[], repo: string | undefined) {
	if (repo) args.push("--repo", repo);
}

function addLimit(args: string[], limit: number | undefined, max = 100) {
	if (limit === undefined) return;
	args.push("--limit", String(Math.max(1, Math.min(max, Math.floor(limit)))));
}

function addJson(args: string[], format: Format | undefined, fields: string) {
	if (format === "json") args.push("--json", fields);
}

function requireNumber(number: number | undefined, name = "number"): string {
	if (number === undefined || !Number.isFinite(number) || number < 1) throw new Error(`${name} is required`);
	return String(Math.floor(number));
}

function buildArgs(params: GHParams): string[] {
	const args: string[] = ["--no-pager"];
	const query = params.query;
	const runId = params.runId;
	const jobId = params.jobId;
	const workflow = params.workflow;
	const release = params.release;

	switch (params.command) {
		case "issue-list":
			args.push("issue", "list");
			addRepo(args, params.repo);
			addLimit(args, params.limit);
			addJson(args, params.format, "number,title,state,author,labels,assignees,createdAt,updatedAt,url");
			break;
		case "issue-view":
			args.push("issue", "view", requireNumber(params.number));
			addRepo(args, params.repo);
			if (params.comments) args.push("--comments");
			addJson(args, params.format, "number,title,state,author,labels,assignees,body,comments,createdAt,updatedAt,url");
			break;
		case "issue-status":
			args.push("issue", "status");
			addRepo(args, params.repo);
			break;
		case "pr-list":
			args.push("pr", "list");
			addRepo(args, params.repo);
			addLimit(args, params.limit);
			addJson(args, params.format, "number,title,state,author,headRefName,baseRefName,isDraft,mergeable,reviewDecision,statusCheckRollup,createdAt,updatedAt,url");
			break;
		case "pr-view":
			args.push("pr", "view", requireNumber(params.number));
			addRepo(args, params.repo);
			if (params.comments) args.push("--comments");
			addJson(args, params.format, "number,title,state,author,body,comments,headRefName,baseRefName,isDraft,mergeable,reviewDecision,statusCheckRollup,commits,files,createdAt,updatedAt,url");
			break;
		case "pr-diff":
			args.push("pr", "diff", requireNumber(params.number));
			addRepo(args, params.repo);
			break;
		case "pr-checks":
			args.push("pr", "checks", requireNumber(params.number));
			addRepo(args, params.repo);
			break;
		case "pr-status":
			args.push("pr", "status");
			addRepo(args, params.repo);
			break;
		case "search-issues":
			args.push("search", "issues");
			if (query) args.push(query);
			addRepo(args, params.repo);
			addLimit(args, params.limit);
			addJson(args, params.format, "number,title,state,author,labels,assignees,createdAt,updatedAt,url,repository");
			break;
		case "search-prs":
			args.push("search", "prs");
			if (query) args.push(query);
			addRepo(args, params.repo);
			addLimit(args, params.limit);
			addJson(args, params.format, "number,title,state,author,createdAt,updatedAt,url,repository");
			break;
		case "search-repos":
			args.push("search", "repos");
			if (query) args.push(query);
			addLimit(args, params.limit);
			addJson(args, params.format, "fullName,description,stars,forks,updatedAt,url");
			break;
		case "run-list":
			args.push("run", "list");
			addRepo(args, params.repo);
			if (workflow) args.push("--workflow", workflow);
			addLimit(args, params.limit, 200);
			addJson(args, params.format, "databaseId,name,displayTitle,status,conclusion,workflowName,headBranch,event,createdAt,updatedAt,url");
			break;
		case "run-view":
			args.push("run", "view");
			if (runId) args.push(runId);
			addRepo(args, params.repo);
			if (jobId) args.push("--job", jobId);
			if (params.logFailed) args.push("--log-failed");
			else if (params.log) args.push("--log");
			else addJson(args, params.format, "databaseId,name,displayTitle,status,conclusion,workflowName,jobs,headBranch,event,createdAt,updatedAt,url");
			break;
		case "workflow-list":
			args.push("workflow", "list");
			addRepo(args, params.repo);
			addLimit(args, params.limit);
			break;
		case "workflow-view":
			if (!workflow) throw new Error("workflow is required");
			args.push("workflow", "view", workflow);
			addRepo(args, params.repo);
			break;
		case "repo-view":
			args.push("repo", "view");
			addRepo(args, params.repo);
			addJson(args, params.format, "nameWithOwner,description,isPrivate,isFork,defaultBranchRef,stargazerCount,forkCount,issues,pullRequests,url");
			break;
		case "release-list":
			args.push("release", "list");
			addRepo(args, params.repo);
			addLimit(args, params.limit);
			break;
		case "release-view":
			args.push("release", "view");
			if (release) args.push(release);
			addRepo(args, params.repo);
			addJson(args, params.format, "name,tagName,isDraft,isPrerelease,publishedAt,author,body,url");
			break;
		case "status":
			args.push("status");
			break;
		default:
			throw new Error(`Unsupported gh command: ${params.command}`);
	}

	return args;
}

function runGH(args: string[], cwd: string, signal: AbortSignal | undefined): Promise<{ code: number | null; output: string; truncated: boolean }> {
	return new Promise((resolve, reject) => {
		const child = spawn("gh", args, { cwd, signal, env: { ...process.env, GH_PROMPT_DISABLED: "1", NO_COLOR: "1" } });
		let output = "";
		let truncated = false;

		const collect = (chunk: Buffer) => {
			if (Buffer.byteLength(output) >= MAX_OUTPUT_BYTES) {
				truncated = true;
				return;
			}
			output += chunk.toString("utf8");
			if (Buffer.byteLength(output) > MAX_OUTPUT_BYTES) {
				output = output.slice(0, MAX_OUTPUT_BYTES);
				truncated = true;
			}
		};

		const timer = setTimeout(() => {
			truncated = true;
			child.kill("SIGTERM");
		}, TIMEOUT_MS);

		child.stdout.on("data", collect);
		child.stderr.on("data", collect);
		child.on("error", reject);
		child.on("close", (code) => {
			clearTimeout(timer);
			resolve({ code, output, truncated });
		});
	});
}

export default function readonlyGHExtension(pi: ExtensionAPI) {
	pi.registerTool({
		name: "gh",
		label: "GitHub Readonly",
		description: "Run a constrained set of read-only GitHub CLI commands for issues, PRs, Actions runs/logs, workflows, repos, releases, and search.",
		promptSnippet: "Inspect GitHub issues, PRs, checks, Actions runs/logs, workflows, repos, releases, and search results using constrained gh commands.",
		promptGuidelines: [
			"Use gh to inspect GitHub issues, pull requests, CI checks, workflow runs, and logs instead of bash.",
			"The gh tool is read-only and only exposes list/view/search/status/diff/checks style commands.",
		],
		parameters: Type.Object({
			command: stringEnum(commands),
			repo: Type.Optional(Type.String({ description: "Repository for commands supporting --repo. Defaults to current repo where gh supports it." })),
			number: Type.Optional(Type.Number({ description: "Issue or PR number." })),
			query: Type.Optional(Type.String({ description: "Search query for gh search commands." })),
			runId: Type.Optional(Type.String({ description: "Workflow run ID for run-view." })),
			jobId: Type.Optional(Type.String({ description: "Job ID for run-view logs." })),
			workflow: Type.Optional(Type.String({ description: "Workflow name, ID, or file name." })),
			release: Type.Optional(Type.String({ description: "Release tag/name for release-view." })),
			limit: Type.Optional(Type.Number({ description: "List/search limit." })),
			format: Type.Optional(stringEnum(formats)),
			log: Type.Optional(Type.Boolean({ description: "For run-view, include full logs." })),
			logFailed: Type.Optional(Type.Boolean({ description: "For run-view, include failed job logs." })),
			comments: Type.Optional(Type.Boolean({ description: "For issue-view/pr-view, include comments." })),
		}),
		async execute(_toolCallId, params: GHParams, signal, _onUpdate, ctx) {
			try {
				const args = buildArgs(params);
				const result = await runGH(args, ctx.cwd, signal);
				const header = `$ gh ${args.join(" ")}`;
				const suffix = result.truncated ? "\n\n[output truncated or command timed out]" : "";
				const exit = result.code === 0 ? "" : `\n\n[exit code: ${result.code}]`;
				return {
					content: [{ type: "text", text: `${header}\n${result.output}${exit}${suffix}` }],
					details: { args, code: result.code, truncated: result.truncated },
				};
			} catch (error) {
				return {
					content: [{ type: "text", text: error instanceof Error ? error.message : String(error) }],
					details: { error: true },
				};
			}
		},
	});
}
