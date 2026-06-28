import { spawn } from "node:child_process";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { Type } from "@sinclair/typebox";

const MAX_OUTPUT_BYTES = 64 * 1024;
const TIMEOUT_MS = 15_000;

const jjCommands = ["status", "diff", "show", "log"] as const;
const gitCommands = ["status", "diff", "show", "log", "branch", "remote", "rev-parse", "ls-files"] as const;
const formats = ["default", "git", "summary", "stat", "name-only"] as const;

const stringEnum = <T extends readonly string[]>(values: T) => Type.Union(values.map((value) => Type.Literal(value)) as never);

type JJCommand = (typeof jjCommands)[number];
type GitCommand = (typeof gitCommands)[number];
type Format = (typeof formats)[number];

const jjParamsSchema = Type.Object({
	command: stringEnum(jjCommands),
	rev: Type.Optional(Type.String({ description: "Revset/revision for diff/show/log, e.g. @, @-, ::, main..@" })),
	from: Type.Optional(Type.String({ description: "Start revset for jj diff --from" })),
	to: Type.Optional(Type.String({ description: "End revset for jj diff --to" })),
	paths: Type.Optional(Type.Array(Type.String(), { description: "Optional path/fileset filters" })),
	format: Type.Optional(stringEnum(formats)),
	limit: Type.Optional(Type.Number({ description: "jj log limit" })),
	noGraph: Type.Optional(Type.Boolean({ description: "For jj log, use --no-graph" })),
});

const gitParamsSchema = Type.Object({
	command: stringEnum(gitCommands),
	rev: Type.Optional(Type.String({ description: "Revision/range for diff/show/log/rev-parse" })),
	from: Type.Optional(Type.String({ description: "Start revision for git diff" })),
	to: Type.Optional(Type.String({ description: "End revision for git diff" })),
	paths: Type.Optional(Type.Array(Type.String(), { description: "Optional path filters" })),
	format: Type.Optional(stringEnum(formats)),
	limit: Type.Optional(Type.Number({ description: "git log limit" })),
	all: Type.Optional(Type.Boolean({ description: "Include all refs for log/branch" })),
	oneLine: Type.Optional(Type.Boolean({ description: "For git log, use --oneline" })),
});

class JJParams {
	command!: JJCommand;
	rev?: string;
	from?: string;
	to?: string;
	paths?: string[];
	format?: Format;
	limit?: number;
	noGraph?: boolean;

	constructor(params: unknown) {
		Object.assign(this, params);
	}

	args(): string[] {
		const args = ["--no-pager", "--color=never", "--quiet", this.command];
		const paths = this.paths ?? [];

		if (this.command === "diff") {
			if (this.rev) args.push("--revisions", this.rev);
			if (this.from) args.push("--from", this.from);
			if (this.to) args.push("--to", this.to);
			this.addFormat(args);
		} else if (this.command === "show") {
			this.addFormat(args);
			if (this.rev) args.push(this.rev);
		} else if (this.command === "log") {
			if (this.rev) args.push("--revision", this.rev);
			if (this.limit !== undefined) args.push("--limit", String(this.limit));
			if (this.noGraph) args.push("--no-graph");
		}

		if (paths.length > 0) args.push("--", ...paths);
		return args;
	}

	private addFormat(args: string[]) {
		switch (this.format ?? "default") {
			case "default":
				return;
			case "git":
				args.push("--git");
				return;
			case "summary":
				args.push("--summary");
				return;
			case "stat":
				args.push("--stat");
				return;
			case "name-only":
				args.push("--name-only");
				return;
		}
	}
}

class GitParams {
	command!: GitCommand;
	rev?: string;
	from?: string;
	to?: string;
	paths?: string[];
	format?: Format;
	limit?: number;
	all?: boolean;
	oneLine?: boolean;

	constructor(params: unknown) {
		Object.assign(this, params);
	}

	args(): string[] {
		const args = ["--no-pager", this.command];
		const paths = this.paths ?? [];

		if (this.command === "status") {
			args.push("--short");
		} else if (this.command === "diff") {
			if (this.from && this.to) args.push(`${this.from}..${this.to}`);
			else if (this.rev) args.push(this.rev);
			if (this.format === "stat") args.push("--stat");
			if (this.format === "name-only") args.push("--name-only");
		} else if (this.command === "show") {
			if (this.format === "stat") args.push("--stat");
			if (this.format === "name-only") args.push("--name-only");
			if (this.rev) args.push(this.rev);
		} else if (this.command === "log") {
			if (this.all) args.push("--all");
			if (this.oneLine) args.push("--oneline");
			if (this.limit !== undefined) args.push("-n", String(this.limit));
			if (this.rev) args.push(this.rev);
		} else if (this.command === "branch") {
			args.push("--list");
			if (this.all) args.push("--all");
		} else if (this.command === "remote") {
			args.push("-v");
		} else if (this.command === "rev-parse") {
			if (this.rev) args.push(this.rev);
		}

		if (paths.length > 0) args.push("--", ...paths);
		return args;
	}
}

function run(command: string, args: string[], cwd: string, signal: AbortSignal | undefined): Promise<{ code: number | null; output: string; truncated: boolean }> {
	return new Promise((resolve, reject) => {
		const child = spawn(command, args, { cwd, signal });
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

export default function readonlyVcsExtension(pi: ExtensionAPI) {
	pi.registerTool({
		name: "jj",
		label: "Jujutsu Readonly",
		description: "Run read-only Jujutsu commands: status, diff, show, and log.",
		promptSnippet: "Inspect Jujutsu repository status, diffs, commits, and logs using status/diff/show/log.",
		promptGuidelines: [
			"Use jj to inspect repository history or current changes instead of bash.",
			"The jj tool only supports status, diff, show, and log.",
		],
		parameters: jjParamsSchema,
		async execute(_toolCallId, params, signal, _onUpdate, ctx) {
			const args = new JJParams(params).args();
			const result = await run("jj", args, ctx.cwd, signal);
			const header = `$ jj ${args.slice(3).join(" ")}`;
			const suffix = result.truncated ? "\n\n[output truncated or command timed out]" : "";
			const exit = result.code === 0 ? "" : `\n\n[exit code: ${result.code}]`;
			return {
				content: [{ type: "text", text: `${header}\n${result.output}${exit}${suffix}` }],
				details: { args, code: result.code, truncated: result.truncated },
			};
		},
	});

	pi.registerTool({
		name: "git",
		label: "Git Readonly",
		description: "Run read-only Git commands: status, diff, show, log, branch, remote, rev-parse, and ls-files.",
		promptSnippet: "Inspect Git repository status, diffs, commits, refs, remotes, and files using read-only git commands.",
		promptGuidelines: [
			"Use git for read-only repository inspection when jj is not appropriate.",
			"The git tool only supports status, diff, show, log, branch --list, remote -v, rev-parse, and ls-files.",
		],
		parameters: gitParamsSchema,
		async execute(_toolCallId, params, signal, _onUpdate, ctx) {
			const args = new GitParams(params).args();
			const result = await run("git", args, ctx.cwd, signal);
			const header = `$ git ${args.slice(1).join(" ")}`;
			const suffix = result.truncated ? "\n\n[output truncated or command timed out]" : "";
			const exit = result.code === 0 ? "" : `\n\n[exit code: ${result.code}]`;
			return {
				content: [{ type: "text", text: `${header}\n${result.output}${exit}${suffix}` }],
				details: { args, code: result.code, truncated: result.truncated },
			};
		},
	});
}
