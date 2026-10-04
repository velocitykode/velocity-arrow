#!/bin/sh
# SessionStart hook (Claude Code, Codex): inject the Velocity rules as additional context.
#
# Reads rules/velocity.mdc from the plugin root (never from outside the plugin),
# drops its frontmatter (Cursor reads that file directly as a rule) and emits the
# body as hookSpecificOutput.additionalContext. Falls back to plain stdout,
# which Claude Code also accepts as context for SessionStart, when no JSON
# encoder is available.
set -eu

# ${CLAUDE_PLUGIN_ROOT} is set by Claude Code and by Codex; the fallback keeps the script
# runnable by hand for testing.
if [ -n "${CLAUDE_PLUGIN_ROOT:-}" ]; then
  root=$CLAUDE_PLUGIN_ROOT
else
  root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
fi

rules="$root/rules/velocity.mdc"
[ -r "$rules" ] || exit 0

# Body only: everything after the closing --- of the frontmatter.
persona=$(mktemp)
trap 'rm -f "$persona"' EXIT
awk 'f>=2 {print; next} /^---$/ {f++; next} f==0 {print}' "$rules" >"$persona"

# The MCP server is the arrow binary. Without it the tools the rules name do
# not exist, so say so first, where the agent reads it before anything else.
if ! command -v arrow >/dev/null 2>&1; then
  body=$(cat "$persona")
  {
    printf '%s\n\n' 'NOTICE: the arrow binary is not on PATH, so the velocity_* tools are unavailable in this session. Tell the user at the start of your first reply to run `go install github.com/velocitykode/velocity-arrow/cmd/arrow@latest` and restart the session.'
    printf '%s\n' "$body"
  } >"$persona"
fi

if command -v jq >/dev/null 2>&1; then
  jq -Rs '{hookSpecificOutput: {hookEventName: "SessionStart", additionalContext: .}}' <"$persona"
elif command -v node >/dev/null 2>&1; then
  ARROW_PERSONA_FILE=$persona node -e '
    const fs = require("fs");
    const text = fs.readFileSync(process.env.ARROW_PERSONA_FILE, "utf8");
    process.stdout.write(JSON.stringify({
      hookSpecificOutput: {
        hookEventName: "SessionStart",
        additionalContext: text,
      },
    }));
  '
else
  cat "$persona"
fi
