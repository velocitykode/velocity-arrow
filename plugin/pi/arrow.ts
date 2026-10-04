// Velocity Arrow for pi.
//
// Does what the plugin's SessionStart hook and mcp.json do for the other
// agents: appends the Velocity rules to the system prompt and registers the
// arrow MCP server. The skills are loaded by pi from plugin/skills, named in
// the package manifest.
import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

const pluginRoot = join(dirname(fileURLToPath(import.meta.url)), "..");

// rules/velocity.mdc is the one copy of the rules. Its frontmatter is for
// Cursor; everything after the closing --- is the body.
function loadRules(): string {
	try {
		const text = readFileSync(join(pluginRoot, "rules", "velocity.mdc"), "utf8");
		const match = /^---\r?\n[\s\S]*?\r?\n---\r?\n/.exec(text);
		return (match ? text.slice(match[0].length) : text).trim();
	} catch {
		return "";
	}
}

export default function (pi: ExtensionAPI) {
	const rules = loadRules();

	// Same entry as plugin/mcp.json. A server of the same name in the user's
	// mcp.json takes precedence. pi releases without built-in MCP support have
	// no registerMcpServer; there the server is added by hand.
	const canRegister = typeof (pi as { registerMcpServer?: unknown }).registerMcpServer === "function";
	if (canRegister) {
		pi.registerMcpServer("velocity-arrow", {
			command: "arrow",
			args: ["mcp"],
			description: "Live context for this Velocity app: routes, config, database schema, logs, docs and the framework knowledge base.",
		});
	}

	pi.on("session_start", (_event, ctx) => {
		// The MCP server is the arrow binary; without it the tools do not exist.
		if (spawnSync("arrow", ["--help"], { stdio: "ignore" }).error) {
			ctx.ui.notify("arrow: binary not on PATH, run `go install github.com/velocitykode/velocity-arrow/cmd/arrow@latest` and restart pi", "warning");
			return;
		}
		if (!canRegister) {
			ctx.ui.notify("arrow: this pi has no built-in MCP support; update pi to use the arrow tools", "warning");
		}
	});

	pi.on("before_agent_start", (event) => {
		if (!rules) return;
		return { systemPrompt: `${event.systemPrompt}\n\n${rules}` };
	});
}
