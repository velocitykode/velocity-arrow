---
name: vel-test
description: How Velocity apps are actually integration-tested - velocitytest.NewApp with in-memory cache/queue, the TestClient for endpoints, transaction-rollback isolation, model factories, seeders, fakes for events/mail/queue, DB assertions, and the always--race rule. Use when writing, scaffolding, or fixing any test that touches a handler, route, repository, job, listener, mailer, or the database in a Go module that requires github.com/velocitykode/velocity. Also use when asked to set up a test database, a TestMain, factories, or seeders.
---

# Velocity integration tests

Drive real handlers over the real router and the real ORM. Build data with
factories, isolate every test, assert through the framework's helpers. Never
hand-roll a harness when the framework ships one.

## Reach for this whenever

- A new or changed endpoint, handler, middleware, repository, job, listener,
  policy, or mailer needs a test.
- Someone says "integration test", "test the endpoint", "set up the test DB",
  "add a factory/seeder", or "why is this test flaky".

Two independent harness layers. Use whichever the test actually needs:

| Need | Use |
|---|---|
| DB only (repository, model, migration) | ORM manager from `TestMain` + `ormtesting` |
| HTTP surface (endpoint, middleware, auth) | above **plus** `velocitytest.NewApp` + `velhttp.TestClient` |
| Side effects (events, mail, queue) | above **plus** the `WithFake*` options |

## 1. The app harness: `velocitytest.NewApp`

`velocitytest.NewApp(opts ...velocity.Option) (*velocity.App, error)` boots an
app with test defaults already wired: **memory cache** (prefix `test_cache`),
**memory queue**, console logger, `log` mail driver, `Debug: true`, `Port: "0"`,
and `APP_ENV=testing` (which waives the `APP_KEY` requirement). It lives in its
own package so production binaries never link the in-memory defaults.

```go
import (
    "github.com/velocitykode/velocity"
    "github.com/velocitykode/velocity/chain"
    "github.com/velocitykode/velocity/router"
    "github.com/velocitykode/velocity/velocitytest"
    velhttp "github.com/velocitykode/velocity/testing/http"
)

app, err := velocitytest.NewApp()          // + any velocity.Option
if err != nil {
    t.Fatalf("velocitytest.NewApp: %v", err)
}
t.Cleanup(func() { _ = app.Shutdown(context.Background()) })

app.Routes(func(r *chain.Routing) {
    r.API("/api", func(rt router.Router) {
        rt.Post("/orders", h.Store)          // {param} syntax, never :param
    })
})
if err := app.Bootstrap(); err != nil {     // runs the declarative chain, no HTTP listener
    t.Fatalf("Bootstrap: %v", err)
}

client := velhttp.NewTestClient(t, app.Router)   // app.Router is an http.Handler
```

`Bootstrap()` is the seam that makes this an integration test without a socket:
it registers modules, middleware, routes, events, schedule and exceptions, then
stops. It is sticky - the first call does the work, later calls return the same
result. `Serve()` is never called in tests.

**Leave the DB out of the app config.** `velocitytest.NewApp` sets no
`DBConfig`, so `app.DB` is nil and the app does *not* touch the ORM default.
The test manager set up in `TestMain` (next section) stays the default and
repositories resolve it. If you hand the app a `DB.Connection`, `velocity.New`
calls `orm.SetDefault` with its own manager and `ResetDefault` on shutdown -
silently yanking the harness manager out from under the suite.

`velocity.WithConfig` replaces the **whole** `Config`, discarding the in-memory
defaults. To adjust one knob use a targeted option (`WithPort`, `WithoutEvents`,
`WithFake*`, `WithModules`), not `WithConfig`.

## 2. The DB harness: `TestMain` (once per test package)

Go gives one `TestMain` per package and no per-test hook, so setup is explicit.
Put the shared harness in an internal package and call it from each package's
`TestMain`; that is how real apps do it.

```go
// internal/testing/bootstrap.go  (a normal package - NOT _test.go)
package testing

import (
    "context"
    "os"
    "sync"

    "github.com/joho/godotenv"
    "github.com/velocitykode/velocity/app"
    "github.com/velocitykode/velocity/orm"

    // Register the same drivers main.go registers.
    _ "github.com/velocitykode/velocity/cache/standard"
    _ "github.com/velocitykode/velocity/orm/standard"
    _ "github.com/velocitykode/velocity/queue/standard"

    _ "myapp/database/migrations" // schema via init() side effect
)

var (
    initOnce    sync.Once
    testManager *orm.Manager
)

func Bootstrap() {
    initOnce.Do(func() {
        // Velocity never auto-loads .env.testing: ConfigFromEnv calls
        // godotenv.Load(), which reads ".env" only. Overload (not Load) so a
        // stray .env or shell var cannot leak into the suite.
        for _, p := range []string{".env.testing", "../.env.testing", "../../.env.testing"} {
            if err := godotenv.Overload(p); err == nil {
                break
            }
        }
        if !app.IsTesting() {
            panic("APP_ENV must be testing")
        }
        m, err := orm.NewManager(orm.ManagerConfig{
            Driver:   os.Getenv("DB_CONNECTION"),
            Host:     os.Getenv("DB_HOST"),
            Port:     os.Getenv("DB_PORT"),
            Database: os.Getenv("DB_DATABASE"), // must look like a test DB, e.g. myapp_test
            Username: os.Getenv("DB_USERNAME"),
            Password: os.Getenv("DB_PASSWORD"),
            SSLMode:  "disable",
        })
        if err != nil {
            panic("init ORM: " + err.Error())
        }
        testManager = m
        orm.SetDefault(m)
    })
}

func Cleanup() {
    if testManager != nil {
        _ = testManager.Shutdown(context.Background())
        orm.ResetDefault()
    }
}

func Manager() *orm.Manager { return testManager }
```

```go
// internal/orders/orders_test.go
func TestMain(m *testing.M) {
    inttesting.Bootstrap()
    defer inttesting.Cleanup()
    os.Exit(m.Run())
}
```

**Test on the engine production uses.** In-memory sqlite
(`Driver: "sqlite", Database: ":memory:", MaxOpenConns: 1`) is hermetic and fast
and fine for pure model tests, but it is a different dialect: it passes while
postgres breaks. If prod is postgres, the integration suite runs postgres.

## 3. Isolation: transaction rollback by default

`ormtesting.NewTestCase(t, manager)` gives three strategies. Pick per test.

```go
tc := ormtesting.NewTestCase(t, inttesting.Manager())

ctx := tc.BeginTransaction()   // FASTEST. Migrate once per binary, open a tx,
                               // t.Cleanup rolls it back. Nothing to truncate.
tc.LazyRefreshDatabase()       // Migrate once per binary, TRUNCATE all tables per test.
tc.RefreshDatabase()           // Drop + migrate per test. Slowest; for migration tests.
```

`BeginTransaction` returns a `context.Context` carrying the `*sql.Tx`. **Thread
that ctx into every ORM call and use the `*Ctx` assertion variants** - anything
reading without it hits the pool and cannot see the transaction's uncommitted
rows. Nested `Manager.Transaction` calls inside the handler become savepoints on
the test transaction, so handler code that manages its own transaction works
unchanged.

Use `LazyRefreshDatabase` when the code under test writes on
`context.Background()` and therefore cannot enroll in the test transaction -
notably **seeders** (`Seeder.Run` takes no ctx).

All three refuse to run outside a test environment: production-class `APP_ENV`
panics outright, and anything that is not `testing`/`test` requires a database
name that looks like a test fixture.

## 4. Factories

A factory describes how to build one model; seeders reuse the same definition.

```go
// models/order_factory.go
func (Order) Factory(m *orm.Manager) *factory.ModelFactory[Order] {
    return factory.NewModelFactory(m, func() *Order {
        f := factory.Faker()
        return &Order{Item: f.Word(), Qty: f.Number(1, 5), Status: "created"}
    }).DefineState("paid", func(o *Order) { o.Status = "paid" })
}
```

```go
order,  err := models.Order{}.Factory(db).State("paid").CreateOne(ctx, nil)
orders, err := models.Order{}.Factory(db).CreateMany(ctx, 5, nil)   // typed []*Order
draft      := models.Order{}.Factory(nil).MakeOne(nil)              // in-memory, no DB
```

Use `CreateOne` / `CreateMany` / `MakeOne`. Bare `Create` / `Make` return `any`.

Placement decides one thing only - whether gofakeit links into the production
binary. `orm/factory` imports gofakeit unconditionally, and `models` is in the
production import graph. Model-rooted (above) is ergonomic; a plain
`database/factories` package imported only by tests keeps gofakeit out of
shipped binaries. Prefer the latter for anything security-sensitive or
size-sensitive. Neither creates an import cycle.

Two factory behaviours bite:

- **Overrides merge non-zero fields only.** `CreateOne(ctx, &Order{Qty: 0})`
  does *not* set `Qty` to 0 - zero values are skipped. To force a zero, use a
  `DefineState` modifier.
- **`State(...)` and `Count(...)` are consumed by the next call** and reset
  afterwards. Set them immediately before the `Create`/`Make`, never once for a
  reused factory variable.

## 5. Seeders reuse the factory

```go
type OrderSeeder struct{}

func (OrderSeeder) Name() string { return "OrderSeeder" }
func (OrderSeeder) Run(m *orm.Manager) error {
    _, err := models.Order{}.Factory(m).CreateMany(context.Background(), 10, nil)
    return err
}
func init() { seed.Register(OrderSeeder{}) }
```

Run with `seed.Seed(manager)` (all) or `seed.SeedOne(manager, "OrderSeeder")`.
`Run` has no ctx, so it writes on `context.Background()` and autocommits - test
seeders with `LazyRefreshDatabase`, never with `BeginTransaction`.

## 6. Fakes for side effects

Pass a fake at construction; assert after acting. All three are plain
`velocity.Option`s.

```go
ev   := events.NewFakeDispatcher()
q    := queuetest.NewFakeQueue()
mail := mailtest.NewFakeMailer()

app, _ := velocitytest.NewApp(
    velocity.WithFakeEvents(ev),
    velocity.WithFakeQueue(q),
    velocity.WithFakeMail(mail),
)

// events: assertions RETURN AN ERROR - check it, they do not fail the test.
if err := ev.AssertDispatched(orders.Placed{}, nil); err != nil {
    t.Error(err)
}
// queue and mail: assertions take *testing.T and fail it directly.
q.AssertPushedOn(t, "emails", func(j contract.QueueJob) bool {
    job, ok := j.(*jobs.SendReceipt); return ok && job.OrderID == order.ID
})
mail.AssertSentTimes(t, 1, func(m *contract.Message) bool {
    return m.Subject == "Your receipt"
})
```

`WithFakeMail` also backs the notification manager, so mail-routed notifications
land in the same fake. `WithoutEvents()` disables the dispatcher entirely when
framework events are just noise.

## 7. Assertions

Database (`ormtesting`) - the `*Ctx` variants take `ctx` second and are
**mandatory** under `BeginTransaction`:

```go
ormtesting.AssertDatabaseCountCtx(t, ctx, db, "orders", 1)
ormtesting.AssertDatabaseHasCtx(t, ctx, db, "orders", map[string]any{"status": "paid"})
ormtesting.AssertDatabaseMissingCtx(t, ctx, db, "orders", map[string]any{"status": "void"})
ormtesting.AssertSoftDeletedCtx(t, ctx, db, "orders", map[string]any{"id": order.ID})
```

HTTP (`velhttp`, fluent and chainable):

```go
client.PostJSON("/api/orders", body).
    AssertCreated().
    AssertJSONPath("data.status", "created").
    AssertJSONStructure([]string{"data", "meta"})

client.PostJSON("/api/orders", bad).AssertUnprocessable().AssertInvalid("item", "qty")
client.WithToken(tok).Get("/api/orders/1").AssertOk()
client.ActingAs(scheme, user).Get("/api/orders").AssertOk()
```

Never assert literal auto-increment IDs - postgres `TRUNCATE ... RESTART
IDENTITY` resets them, sqlite `DELETE` does not, and rollback leaves sequences
advanced. Assert counts and content.

## 8. Table-driven cases

Default shape for anything with more than one input. One subtest per case, fresh
isolation inside the subtest so cases cannot leak into each other:

```go
func TestCreateOrder(t *testing.T) {
    tests := []struct {
        name       string
        body       map[string]any
        wantStatus int
        wantFields []string // validation fields expected to fail
    }{
        {"valid", map[string]any{"item": "widget", "qty": 2}, http.StatusCreated, nil},
        {"missing item", map[string]any{"qty": 2}, http.StatusUnprocessableEntity, []string{"item"}},
        {"qty below minimum", map[string]any{"item": "widget", "qty": 0}, http.StatusUnprocessableEntity, []string{"qty"}},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := ormtesting.NewTestCase(t, inttesting.Manager()).BeginTransaction()
            client := newTestClient(t)

            res := client.PostJSON("/api/orders", tt.body).AssertStatus(tt.wantStatus)
            if len(tt.wantFields) > 0 {
                res.AssertInvalid(tt.wantFields...)
                ormtesting.AssertDatabaseCountCtx(t, ctx, inttesting.Manager(), "orders", 0)
                return
            }
            ormtesting.AssertDatabaseHasCtx(t, ctx, inttesting.Manager(), "orders",
                map[string]any{"item": tt.body["item"]})
        })
    }
}
```

Do **not** add `t.Parallel()` to subtests that share the ORM default manager or
a single in-memory sqlite connection.

## 9. `-race`, always

Every `go test` invocation runs with `-race`. It is the CI default and the local
default; a change is not verified until `go test -race ./...` on the touched
packages passes.

```bash
go test -race ./internal/orders/...
```

Two rules that keep `-race` meaningful:

- **No unconditional `t.Skip`.** Gate every skip on a real condition
  (`os.Getenv`, `testing.Short()`) or delete the test.
- **No wait-and-hope `time.Sleep`.** For anything asynchronous use
  `testsync.Eventually(t, cond, timeout, msg)` or
  `testsync.EventuallyEqual(t, get, want, timeout, msg)`, or a channel /
  `sync.WaitGroup`. A sleep is only defensible when wall-clock time is the thing
  under test (TTL expiry, rate-limit window) or when it is fake work inside a
  closure the helper runs - and then it carries a comment saying which.

## 10. Full example: testing a new endpoint

```go
// internal/orders/orders_integration_test.go
package orders

import (
    "context"
    "os"
    "strconv"
    "testing"

    "github.com/velocitykode/velocity"
    "github.com/velocitykode/velocity/chain"
    "github.com/velocitykode/velocity/contract"
    "github.com/velocitykode/velocity/events"
    ormtesting "github.com/velocitykode/velocity/orm/testing"
    "github.com/velocitykode/velocity/queue/queuetest"
    "github.com/velocitykode/velocity/router"
    velhttp "github.com/velocitykode/velocity/testing/http"
    "github.com/velocitykode/velocity/velocitytest"

    inttesting "myapp/internal/testing"
    "myapp/models"
)

func TestMain(m *testing.M) {
    inttesting.Bootstrap()
    defer inttesting.Cleanup()
    os.Exit(m.Run())
}

// harness boots the app with the routes under test and the side-effect fakes,
// and returns a client pointed at its router.
func harness(t *testing.T) (*velhttp.TestClient, *events.FakeDispatcher, *queuetest.FakeQueue) {
    t.Helper()

    ev, q := events.NewFakeDispatcher(), queuetest.NewFakeQueue()

    app, err := velocitytest.NewApp(velocity.WithFakeEvents(ev), velocity.WithFakeQueue(q))
    if err != nil {
        t.Fatalf("velocitytest.NewApp: %v", err)
    }
    t.Cleanup(func() { _ = app.Shutdown(context.Background()) })

    h := NewHandler(NewService(NewRepository()))
    app.Routes(func(r *chain.Routing) {
        r.API("/api", func(rt router.Router) {
            rt.Post("/orders", h.Store)
            rt.Get("/orders/{orderID}", h.Show)
        })
    })
    if err := app.Bootstrap(); err != nil {
        t.Fatalf("Bootstrap: %v", err)
    }

    return velhttp.NewTestClient(t, app.Router), ev, q
}

func TestStoreOrder_Integration(t *testing.T) {
    ctx := ormtesting.NewTestCase(t, inttesting.Manager()).BeginTransaction()
    client, ev, q := harness(t)

    client.PostJSON("/api/orders", map[string]any{"item": "widget", "qty": 2}).
        AssertCreated().
        AssertJSONPath("data.item", "widget").
        AssertJSONPath("data.status", "created")

    ormtesting.AssertDatabaseCountCtx(t, ctx, inttesting.Manager(), "orders", 1)
    ormtesting.AssertDatabaseHasCtx(t, ctx, inttesting.Manager(), "orders", map[string]any{
        "item": "widget", "qty": 2, "status": "created",
    })

    if err := ev.AssertDispatched(OrderPlaced{}, nil); err != nil {
        t.Error(err)
    }
    q.AssertPushedOn(t, "emails", func(j contract.QueueJob) bool {
        _, ok := j.(*SendReceiptJob)
        return ok
    })
}

func TestShowOrder_Integration(t *testing.T) {
    ctx := ormtesting.NewTestCase(t, inttesting.Manager()).BeginTransaction()
    client, _, _ := harness(t)

    order, err := models.Order{}.Factory(inttesting.Manager()).State("paid").CreateOne(ctx, nil)
    if err != nil {
        t.Fatalf("factory CreateOne: %v", err)
    }

    client.Get("/api/orders/" + strconv.FormatInt(order.ID, 10)).
        AssertOk().
        AssertJSONPath("data.status", "paid")

    client.Get("/api/orders/999999").AssertNotFound()
}
```

Run it: `go test -race ./internal/orders/...`

## Gotchas worth remembering

- `context.Background()` carries no transaction - every write autocommits. Thread
  the ctx from `BeginTransaction` (or `c.Request.Context()` in a handler) into
  every ORM call, or the rollback isolates nothing.
- `factory.Faker()` is a process-global seeded from `crypto/rand`, so generated
  values differ every run. It *is* safe under `-race` (locked source), but never
  assert on a faked value - assert on what you passed in. For reproducible data,
  construct your own `gofakeit.New(seed)` in the app.
- Migrations must be registered by blank import (`_ "myapp/database/migrations"`)
  or the refresh helpers build an empty schema.
- `ormtesting` migrates once per **test binary** (a package-level `sync.Once`).
  Each Go test package is its own binary, so each needs its own `TestMain`.
- A `_test.go` suffix is mandatory on test files. Shared helpers that import
  `testing` must live in `setup_test.go` or a normal non-test package like
  `internal/testing` - a plain `setup.go` importing `testing` will not compile.
- Test files in `tests/` share one package: helpers are visible with no import,
  and there is exactly one `TestMain` among them.
