---
name: framework-generate
description: Generate Velocity framework code - providers, handlers, middleware, models, and more
---

# Framework Generate

Generate idiomatic Velocity framework code following the project's patterns.

## When to use

- Creating new service providers
- Adding HTTP handlers/controllers
- Writing middleware
- Defining models and migrations
- Setting up event listeners
- Creating queue jobs

## Key patterns

- Use the provider lifecycle: Register → Boot → Shutdown
- Handlers take `*router.Context` and return `error`
- Models use generics: `orm.Model[T]`, `orm.UUIDModel[T]`
- Middleware wraps `HandlerFunc`: `func(next HandlerFunc) HandlerFunc`
