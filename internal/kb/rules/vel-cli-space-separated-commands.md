---
title: The vel CLI uses space-separated subcommands, never colon-style tokens
package: console
tags: [cli, vel, console, commands, codegen, generators, migrate, routes, cache]
use_instead: space-separated tokens such as "vel gen model", "vel routes", "vel migrate fresh", "vel key generate", "vel cache clear"
deprecated: false
---
Command tokens are space-separated subcommands. Colon-style names such as
make:model, route:list, key:generate or cache:clear do not resolve.

Current grammar:

- Server: `serve`, `build`, `down`, `up`
- Database: `migrate`, `migrate fresh`, `migrate rollback`, `migrate status`,
  `db wipe`
- Queue and scheduler: `queue work`, `schedule work`
- Cache: `cache clear`
- Code generation: `gen handler`, `gen model`, `gen migration`,
  `gen middleware`, `gen event`, `gen listener`, `gen job`, `gen mail`,
  `gen notification`, `gen resource`, `gen policy`, `gen module`,
  `gen command`, `gen grpc service`, `gen grpc rpc`, `gen grpc gen`
- Custom: `run <name>` for commands registered via `App.Commands`
- Other: `routes`, `key generate`

Dispatch joins the leading argv tokens and matches longest-first, so a
subcommand wins over its bare parent (`migrate fresh` over `migrate`). Matching
never extends across a flag-like token, so `vel migrate --pretend` resolves to
`migrate` with the flag passed through.

The generators themselves live in the framework's `console` package (for
example `console.GenHandler`, `console.GenModule`), not in a separate CLI
module.
