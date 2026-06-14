# Velocity Arrow

**Give your AI coding agent live, grounded context about your Velocity app.** Arrow is a first-party, ready-to-run [MCP](https://modelcontextprotocol.io) server that runs alongside your project and lets agents like Claude Code, Cursor, and Codex read what is actually there: the real routes, the resolved config, the current database schema, the latest logs, and the docs.

Instead of guessing from stale training data, the agent asks Arrow and writes code grounded in your app's reality.

```bash
go install github.com/velocitykode/velocity-arrow@latest
```

## Why Arrow

- **Live context, not guesswork.** Routes, schema, config values, and recent errors come from the running project, so generated code matches what exists right now.
- **Read-only and safe.** Database access is read-only and ad-hoc; Arrow inspects, it does not mutate.
- **Zero-config onboarding.** `arrow install` drops in AI guidelines and skills tuned to your project's actual dependencies.
- **Works with any MCP client.** Claude Code, Cursor, Codex, and others, over stdio.
- **Built on Velocity.** Reads config through `velocity.ConfigFromEnv()` and connects with Velocity's ORM, so its tools work against postgres, mysql, or sqlite.

## What the agent can see

- **App info** - module, version, and project layout
- **Database schema** - tables, columns, and read-only ad-hoc queries
- **Routes** - every registered route and its handler
- **Config** - resolved configuration values
- **Logs** - the most recent entries and errors
- **Docs** - search across the Velocity documentation

## Install into a project

```bash
arrow install   # adds AI guidelines + skills matched to your go.mod
arrow mcp       # run the MCP server (stdio transport)
```

Point your editor's MCP client at `arrow mcp` and the agent gains the tools above.

## Documentation

[vel.build/docs/ecosystem/velocity-arrow](https://vel.build/docs/ecosystem/velocity-arrow)

## License

MIT
