# Arrow

A Velocity-focused MCP server that accelerates AI-assisted development. Gives AI agents the context they need to write high-quality Velocity-specific code.

## Features

- **MCP Server** - Tools for app info, database schema, route listing, doc search, log reading, and config inspection
- **AI Guidelines** - Auto-generated CLAUDE.md with Velocity conventions based on your project's go.mod
- **Skills** - Auto-installed SKILL.md files matched to your project's dependencies
- **Multi-agent support** - Works with Claude Code, Cursor, Codex, and more

## Install

```bash
go install github.com/velocitykode/velocity-arrow@latest
```

## Usage

```bash
# Install into a Velocity project
arrow install

# Run MCP server (stdio transport)
arrow mcp
```

## License

MIT
