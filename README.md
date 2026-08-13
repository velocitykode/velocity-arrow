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
| `velocity_search_docs` | the Velocity documentation | bundled docs |
| `velocity_kb_symbol` / `velocity_kb_search` / `velocity_kb_guard` | exact signatures, intent lookups, and guard rules | the baked knowledge base |

The knowledge base is a version-stamped snapshot, not a live read of the
framework. A miss means "not in this snapshot", not "not in Velocity".

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

## Rebake the knowledge base

The knowledge base ships as a SQLite snapshot embedded in the binary
(`internal/kb/data/velocity-kb.db`), so it goes stale the moment Velocity
renames something. Rebuild it against a specific framework checkout:

```bash
go run ./cmd/ingest -velocity ~/code/velocity -version v0.73.0
```

`-velocity` is the source tree to parse and `-version` is the stamp written
into every entry and into `kb://manifest`. **Nothing checks that the two
agree** - bake from a clean checkout of the tag you are stamping (a
`git worktree` of it, not a dirty working tree) or the snapshot will lie about
its own version.

Symbols come from the source tree, but guard rules do not: they are curated
markdown in `internal/kb/rules/*.md`. A rename that needs new guidance needs a
rule file written by hand; rebaking alone will not produce one.

Then commit the regenerated `.db` and reinstall, because the snapshot is
compiled in - a running server keeps serving the old one until its binary is
replaced:

```bash
go install ./cmd/arrow   # then restart the MCP server in your client
```

Verify with `velocity_kb_symbol` on a symbol the rename touched, and check
that `kb://manifest` reports the version you stamped.

## Documentation

[vel.build/docs/ecosystem/velocity-arrow](https://vel.build/docs/ecosystem/velocity-arrow)

## License

MIT
