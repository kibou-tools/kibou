import path from "node:path";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

function writablePath(filePath: string, cwd: string): boolean {
	const relative = path.relative(cwd, path.resolve(cwd, filePath)).split(path.sep).join("/");
	return relative.startsWith(".pi/extensions/") && relative.endsWith(".ts") && relative !== ".pi/extensions/tool-allowlist.ts";
}

export default function (pi: ExtensionAPI) {
	pi.on("tool_call", async (event, ctx) => {
		if (event.toolName !== "write" && event.toolName !== "edit") return undefined;

		const filePath = event.input.path;
		if (typeof filePath === "string" && writablePath(filePath, ctx.cwd)) return undefined;

		return {
			block: true,
			reason: "Writes are only allowed to .pi/extensions/*.ts, excluding tool-allowlist.ts",
		};
	});
}
