# Velocity Arrow

**Give your AI coding agent live, grounded context about your Velocity app.** Arrow is a first-party, ready-to-run [MCP](https://modelcontextprotocol.io) server that runs alongside your project and lets agents like Claude Code, Cursor, and Codex read what is actually there: the real routes, the resolved config, the current database schema, the latest logs, and the docs.

Instead of guessing from stale training data, the agent asks Arrow and writes code grounded in your app's reality.

```bash
go install github.com/velocitykode/velocity-arrow@latest
```

## Why Arrow

- **Live context, not guesswork.** Routes, schema, config values, and recent errors come from the running project, so generated code matches what exists right now.
- **Read-only and safe.** Database access is read-only and ad-hoc; Arrow inspects, it does not mutate.
- **Zero config.** Point the client at `arrow mcp` in the project directory; Arrow reads `go.mod` and `.env` from there.
- **Works with any MCP client.** Claude Code, Cursor, Codex, and others, over stdio.
- **Built on Velocity.** Reads config through `velocity.ConfigFromEnv()` and connects with Velocity's ORM, so its tools work against postgres, mysql, or sqlite.

## What the agent can see

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

## Use it in a project

Arrow runs *in* the project directory - that working directory is how it finds
`go.mod`, `.env`, the database, and the logs. Nothing is added to the app
itself; there is no import, no module to register.

```bash
velocity new myapp --stack react   # any Velocity project will do
cd myapp
arrow mcp                          # MCP server over stdio
```

Register it with your MCP client, pointing `cwd` at the project:

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

With Claude Code, `claude mcp add velocity-arrow -- arrow mcp` does the same
thing from inside the project directory.

Two things follow from the working-directory rule: run `./vel migrate` before
asking for the schema, since the tools read what exists rather than what the
migrations intend, and expect `velocity_log_entries` to find nothing while
`LOG_DRIVER=console` - it reads log files, and the console driver writes none.

## The knowledge base

Nothing is baked into the binary. At startup arrow reads the velocity version
the app pins (`go list -m github.com/velocitykode/velocity` in the working
directory; the latest release when there is no module) and serves a knowledge
base for exactly that version, resolved in this order:

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

[vel.build/docs/ecosystem/velocity-arrow](https://vel.build/docs/ecosystem/velocity-arrow)

## License

MIT
