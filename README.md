# Velocity Arrow

**Arrow makes your AI coding agent a Velocity engineer.** It is the AI layer of
the Velocity framework: installed once, it gives agents like Claude Code,
Codex, Cursor and pi three things they do not have on their own.

- **Rules.** How Velocity code is written: introspect before guessing, use the
  framework's helpers, scaffold with the generator first.
- **Skills.** Step-by-step procedures the agent loads when the task calls for
  them: `vel-gen` and `vel-test`.
- **Live context.** Tools that read the app in front of it: the real routes,
  the resolved config, the database schema, the latest logs, the docs and the
  exact framework API for the version the app pins.

Instead of guessing from stale training data, the agent works from your app as
it is and from Velocity as it is at your version.

Arrow informs; it does not enforce. Nothing blocks an edit or a command. Database
access is read-only.

## Install

Two steps: the `arrow` binary, then the plugin for your agent.

```bash
go install github.com/velocitykode/velocity-arrow/cmd/arrow@latest
```

This puts `arrow` in `$(go env GOPATH)/bin`; make sure that directory is on
your `PATH`. Run the same command again to update. Nothing is added to the app
itself: no import, no module to register.

The plugin is one bundle (`plugin/` in this repository) for every agent. Without
`arrow` on `PATH` the rules and skills still load, the tools do not, and the
agent tells you to install it.

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

### pi

```bash
pi install git:github.com/velocitykode/velocity-arrow
```

The tools need pi 1.0 or later; on an older pi the rules and skills still load
and a notice says to update.

### opencode

opencode has no bundle to install, so wire the three parts by hand. The skills
and the rules are the same files the plugin ships.

**Tools**, in `opencode.json` at the project root:

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

**Skills**, into the shared `~/.agents/skills/` directory:

```bash
git clone https://github.com/velocitykode/velocity-arrow /tmp/velocity-arrow
mkdir -p ~/.agents/skills
cp -R /tmp/velocity-arrow/plugin/skills/* ~/.agents/skills/
```

**Rules**, appended to `AGENTS.md` in the project root without their
frontmatter:

```bash
awk 'f>=2; /^---$/ {f++}' /tmp/velocity-arrow/plugin/rules/velocity.mdc >> AGENTS.md
```

### Any other agent

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

This gives the tools only. Add the rules by copying `plugin/rules/velocity.mdc`
into the agent's instructions file.

## What the agent can do

### Rules

One file, `plugin/rules/velocity.mdc`, present in every session. It tells the
agent to check the app and the knowledge base before writing, to prefer
Velocity's helpers over the standard library, to scaffold with the generator,
to build and test after an edit, and to say so when it departs from what you
asked for.

### Skills

| Skill | Use |
|-------|-----|
| `vel-gen` | scaffold a model, migration, handler, job and the rest with the generator before writing a file by hand |
| `vel-test` | write integration tests the way Velocity apps are tested |

### Tools

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

[vel.build/docs/ecosystem/velocity-arrow](https://vel.build/docs/ecosystem/velocity-arrow)

## License

MIT
