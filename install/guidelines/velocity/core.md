# Velocity Framework Patterns

## Architecture

Velocity uses a **provider-based architecture**:

1. **Service Providers** - Register and boot services via `Register(s *Services)` and `Boot(s *Services)`
2. **Routing** - Declarative route registration with `routing.Web()` and `routing.API()`
3. **Middleware** - Global, web, and API middleware stacks
4. **Events** - Event dispatcher with listeners and subscribers
5. **Scheduling** - Cron-like job scheduling

## Key Patterns

### Service Provider Lifecycle
```
Register → Boot → (serve requests) → Shutdown
```

Providers implement optional interfaces for auto-wiring:
- `RouteProvider` - register routes
- `MiddlewareProvider` - configure middleware
- `EventProvider` - register event listeners
- `ScheduleProvider` - schedule jobs

### Route Registration
```go
routing.Web(func(r router.Router) {
    r.Get("/", homeHandler)
    r.Resource("/posts", &PostController{})
    r.Group("/admin", func(r router.Router) {
        r.Use(adminAuth)
        r.Get("/dashboard", dashboardHandler)
    })
})
```

### Handler Signature
```go
func handler(c *router.Context) error {
    return c.JSON(200, data)
}
```

### Middleware Pattern
```go
func myMiddleware(next router.HandlerFunc) router.HandlerFunc {
    return func(c *router.Context) error {
        // before
        err := next(c)
        // after
        return err
    }
}
```

### Bootstrap Chain
```go
v.Providers(registerProviders).
    Middleware(setupMiddleware).
    Routes(registerRoutes).
    Events(setupListeners).
    Schedule(setupJobs).
    Exceptions(setupErrorHandling).
    Serve()
```

## Important Rules

- Routes are **frozen** after `Serve()` - register all routes before serving
- `*router.Context` objects are **pooled** - never hold references beyond the handler
- Use the `contract/` package interfaces to avoid import cycles
- Event dispatchers use `SetEventDispatcher(fn)` to break circular dependencies
