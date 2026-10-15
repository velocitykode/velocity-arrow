# Velocity Arrow

**Arrow gives your AI coding agent the context of a Velocity app.** It is one
MCP server with two things agents do not have on their own:

- **The knowledge base.** The exact framework API, guard rules and docs for the
  Velocity version the app pins.
- **App context.** Tools that read the app in front of the agent: the real
  routes, the resolved config, the database schema and the latest logs.

Instead of guessing from stale training data, the agent works from your app as
it is and from Velocity as it is at your version.

Arrow is only that: no rules, no skills, no hooks, nothing injected into the
agent's prompt. It informs; it does not enforce. Database access is read-only.

## Install

Two steps: the `arrow` binary, then register the MCP server with your agent.

```bash
go install github.com/velocitykode/velocity-arrow/cmd/arrow@latest
```

This puts `arrow` in `$(go env GOPATH)/bin`; make sure that directory is on
your `PATH`. Run the same command again to update. Nothing is added to the app
itself: no import, no module to register.

For Claude Code, Codex and Cursor, `plugin/` in this repository is a thin
bundle that does one thing: register `arrow mcp`. Any other MCP client
registers it by hand.

### Claude Code

```bash
claude plugin marketplace add velocitykode/velocity-arrow
claude plugin install arrow@velocitykode
```

### Codex

```bash
codex plugin marketplace add velocitykode/velocity-arrow
codex plugin add arrow@velocitykode
```

Turn the plugin off for one project in `.codex/config.toml`:

```toml
[plugins."arrow@velocitykode"]
enabled = false
```

### Cursor

Open **Customize**, choose **Import from Repo** and give it
`https://github.com/velocitykode/velocity-arrow`.

### opencode

In `opencode.json` at the project root:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "velocity-arrow": {
      "type": "local",
      "command": ["arrow", "mcp"]
    }
  }
}
```

### Any other agent (pi included)

The tools are served over [MCP](https://modelcontextprotocol.io), so any MCP
client can use them. Register `arrow mcp` with the project as its working
directory:

```json
{
  "mcpServers": {
    "velocity-arrow": {
      "command": "arrow",
      "args": ["mcp"],
      "cwd": "/path/to/myapp"
    }
  }
}
```

## Tools

| Tool | Reads | From |
|------|-------|------|
| `velocity_app_info` | module, Go version, Velocity version, dependencies | `go.mod` |
| `velocity_routes` | every registered route and its handler | the `vel` CLI, falling back to static analysis |
| `velocity_db_schema` | tables and columns | the configured database |
| `velocity_db_query` | read-only ad-hoc queries | the configured database |
| `velocity_config` | resolved config plus raw `.env`, credentials redacted | `velocity.ConfigFromEnv()` and `.env` |
| `velocity_log_entries` / `velocity_last_error` | the most recent entries and errors | `storage/logs` |
| `velocity_search_docs` | the Velocity documentation | the knowledge base |
| `velocity_kb_symbol` / `velocity_kb_search` / `velocity_kb_guard` | exact signatures, intent lookups, and guard rules | the knowledge base |

The knowledge base describes the velocity version the current app pins
(`go.mod`), not the latest release. A miss means "not in this version", not
"not in Velocity".

## How it reads your app

Arrow runs in the project directory. That working directory is how it finds
`go.mod`, `.env`, the database and the logs, so start your agent from the
project root. It reads config through `velocity.ConfigFromEnv()` and connects
with Velocity's ORM, so the tools work against postgres, mysql or sqlite.

Two things follow: run `vel migrate` before asking for the schema, since the
tools read what exists rather than what the migrations intend, and expect
`velocity_log_entries` to find nothing while `LOG_DRIVER=console`, because it
reads log files and the console driver writes none.

## The knowledge base

At startup arrow reads the velocity version the app pins (`go list -m
github.com/velocitykode/velocity` in the working directory; the latest release
when there is no module) and serves a knowledge base for exactly that version,
resolved in this order:

1. the local cache, `~/Library/Caches/arrow/kb/<version>.db` on macOS
   (`$ARROW_CACHE_DIR` overrides the directory);
2. the published snapshot for that version,
   `https://github.com/velocitykode/velocity/releases/download/<version>/velocity-kb.db`
   (`$ARROW_KB_BASE_URL` overrides the base, `-` disables downloads);
3. a local build from the module cache, which takes well under a second and
   carries symbols and guard rules but no documentation pages.

Bump velocity in the app and the next server start serves the new version.
Arrow also compares the pin with the latest release and prefixes knowledge
base answers with one line when they differ: a patch, new symbols, or removed
symbols with their names. `kb://manifest` reports the pin, its origin, the
snapshot source and the gap.

Guard rules are curated markdown in `internal/kb/rules/*.md`, embedded in the
binary and ingested into every snapshot. A rename that needs new guidance
needs a rule file written by hand.

To build a snapshot by hand, for example while iterating on the ingester or
the rules:

```bash
VEL=$(go list -m -f '{{.Dir}}' github.com/velocitykode/velocity)
VER=$(go list -m -f '{{.Version}}' github.com/velocitykode/velocity)
go run ./cmd/ingest -velocity "$VEL" -version "$VER" -docs ~/code/velocity-docs/content/docs -out "$VER.db"
```

Drop it into the cache directory under `<version>.db` and the next start
serves it.

## Documentation

[vel.build/docs/ai/arrow](https://vel.build/docs/ai/arrow)

## License

MIT
