---
name: vel-gen
description: Create new Velocity artifacts with the console generator (`go run . gen model|migration|handler|job|module|policy|middleware|event|listener|resource|command|mail|notification|grpc service|grpc rpc|grpc gen`) instead of hand-authoring files. Use whenever asked to add, create, or scaffold anything in a Go app that imports github.com/velocitykode/velocity - a model, migration, handler/route, job, policy, module, middleware, event, listener, resource, command, mailable, notification, or gRPC service/rpc - and for the exact flags, output paths, naming rules, and the follow-up edits each generated file needs.
---

# Scaffold via generator

## Trigger

Any request to ADD a new artifact to a Velocity app (a Go module requiring
`github.com/velocitykode/velocity`): "add a User model", "new endpoint",
"create a job", "add a policy", "scaffold a gRPC service".

## Rule

1. Generator first. Never author a new artifact file from scratch when a
   generator covers it - skipping the generator check is a defect.
2. Confirm the surface before running: `go run . help` prints the registry.
3. Run the generator, read its `Created: <path>` line, then open that file and
   edit it. Generate -> edit. Not write-from-memory.
4. The generator scaffolds; the framework rules (persona) still govern the
   contents of what you then write into it.

## CLI shape

From the app root: `go run . gen <thing> <Name> [flags]` (or `./vel gen ...`
against a built binary).

- Space-separated group-then-action. There is **no `make:` prefix** and **no
  `list` command** - both are retired legacy. An app doc that still says
  `go run . make:model` or `go run . list` is stale; trust `go run . help`.
- Unknown flags and stray positionals are rejected, not ignored.
- Flags take `--flag value` or `--flag=value`.
- `--dir <path>` overrides the output directory on every generator.

## Command table

| Artifact | Command | Produces | Common follow-up edits |
|---|---|---|---|
| Model | `gen model User [--uuid] [--soft-deletes] [-m\|--migration]` | `internal/models/user.go` (embeds `orm.Model[User]`, `TableName()`, `AssignableFields()`); `-m` also runs the create-table migration | Fill `AssignableFields()` allowlist, add columns/relations. Never add `Fillable()` |
| Migration | `gen migration create_posts --create=posts` / `gen migration add_slug_to_posts --table=posts` | `database/migrations/<version>_<name>.go` with `migrate.Register` + Up/Down | Write the `TableBuilder` columns and a real `Down`; ensure `_ "<mod>/database/migrations"` is imported in `main.go`; then `go run . migrate` |
| Handler | `gen handler User [-r\|--resource] [--api]` | `internal/handlers/user.go`; `--resource` = Index/Create/Store/Show/Edit/Update/Destroy, `--api` = JSON bodies. Nested `Admin/Users` -> `internal/handlers/admin/users.go`, package `admin` | Real logic (vform validation, ctx-first ORM), then register the route in `routes/` using `{param}` syntax |
| Middleware | `gen middleware Auth` | `internal/middleware/auth.go` (`func(next router.HandlerFunc) router.HandlerFunc`) | Body, then add to the app's middleware stack |
| Event | `gen event OrderShipped` | `internal/events/order_shipped.go` (struct + `Name()`) | Add payload fields |
| Listener | `gen listener SendShipmentEmail` | `internal/listeners/send_shipment_email.go` | Handle body, then subscribe it in the app's events wiring (`d.Listen(...)`) |
| Job | `gen job ProcessPayment` | `internal/jobs/process_payment.go` (`Handle`, `Failed`, `MaxAttempts`) | Payload fields + `Handle`; dispatch via the queue; uncomment `OnQueue()` for a non-default queue |
| Mailable | `gen mail OrderShipped` | `internal/mail/order_shipped.go` (`Envelope`/`Content`/`Build`) | Recipients, subject, body |
| Notification | `gen notification InvoicePaid` | `internal/notifications/invoice_paid.go` (`Via`, `ToMail`) | Channels in `Via`; uncomment `ToDatabase` if stored |
| Resource | `gen resource User` | `internal/resources/user.go` (`ToResource() map[string]any`) | Map model fields to response keys |
| Policy | `gen policy Post` | `internal/policies/post.go` - a plain struct with View/Create/Update/Delete taking `*http.Request` | Real rules. The stub shape does **not** satisfy `auth.Policy` (`Authorize(user, action, resource) bool`): call its methods directly, or adapt it via `auth.PolicyFunc` and `access.RegisterPolicy("post", ...)`. Check how sibling policies in the app are wired |
| Module | `gen module Cache` | `internal/modules/cache.go` (`Init`/`Start`/`Shutdown`) | Bind services in `Init`, register the module in the app's bootstrap wiring. **This replaced service providers - there is no `gen provider`** |
| Command | `gen command SyncUsers` | `internal/commands/sync_users.go` (`Name`/`Description`/`Handle`) | Logic, then register in `internal/commands` kernel (`r.Add(&SyncUsersCommand{})`); run via `go run . run <name>` |
| gRPC service | `gen grpc service Foo [--package <leaf>] [--proto-package <pkg>] [--alias <ident>] [--proto-name <base>] [--impl-name <base>] [--no-module]` | `api/proto/<leaf>/v1/foo.proto`, impl in `internal/grpc/services/`, buf configs, and module wiring at the `// vel:grpc:imports` / `// vel:grpc:services` markers unless `--no-module` | Add rpcs/messages to the proto, run `gen grpc gen`, fill the impl. Use `--no-module` when the app registers services through its own wiring |
| gRPC rpc | `gen grpc rpc Foo Hello [--stream\|--client-stream\|--bidi]` | rpc + message stubs on the existing service | Then `gen grpc gen`, implement the method |
| Proto codegen | `gen grpc gen` (no args) | Runs `buf generate` in `api/proto` | Never invoke `buf` directly |

## Naming and re-runs

- Pass the bare PascalCase name: `User`, not `UserModel` / `UserHandler` /
  `FooService` - the redundant suffix is stripped, and a name that is *only*
  the suffix is an error.
- Migration names are snake_case verbs: `create_posts`, `add_slug_to_posts`.
- Generators refuse to overwrite: `<kind> already exists: <path>`. To change an
  existing artifact, edit the file - do not re-run and do not delete-and-regen.
- After generating, build (`go build ./...`) before reporting done.

## Never hand-write

A new file under `internal/models`, `internal/handlers`, `internal/middleware`,
`internal/events`, `internal/listeners`, `internal/jobs`, `internal/mail`,
`internal/notifications`, `internal/resources`, `internal/policies`,
`internal/modules`, `internal/commands`, `database/migrations`, or
`api/proto/**` - every one of those has a generator.

## Escape hatch

No generator fits (a plain `internal/` package, a test file, config, or a case
the generator cannot express - e.g. consolidated migration numbering): hand-write
it, mirroring the layout of a generated sibling in the same directory, and getting
it exactly right matters more than usual. Say in one line that no generator
covered it.
