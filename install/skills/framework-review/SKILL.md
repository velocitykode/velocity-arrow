---
name: framework-review
description: Review Velocity framework code for correctness, performance, and best practices
---

# Framework Review

Review Velocity code for common mistakes, performance issues, and adherence to framework conventions.

## What to check

- Error handling - all errors returned and wrapped with context
- Context pooling - no `*router.Context` held beyond handler lifetime
- Route registration - all routes registered before `Serve()`
- Provider lifecycle - no cross-provider usage in `Register()`
- Import cycles - use `contract/` interfaces to break cycles
- Concurrency - no goroutines started in handlers without coordination
- SQL injection - parameterized queries only, validated column names
