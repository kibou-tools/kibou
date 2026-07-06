import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

const allowed = ["read", "grep", "find", "ls", "edit", "write", "jj", "git", "gh", "bash"];

export default function (pi: ExtensionAPI) {
	pi.on("session_start", () => {
		pi.setActiveTools(allowed);
	});
}
