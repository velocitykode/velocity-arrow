# Velocity Project

This is a Velocity framework application written in Go.

- **Module**: {{.Module}}
- **Go version**: {{.GoVersion}}

## Project Conventions

- Use the Velocity framework patterns for all application logic
- Follow Go idioms and best practices
- Use the service provider pattern for dependency wiring
- Handle errors explicitly - do not ignore returned errors
- Use the `*router.Context` for HTTP handlers (do NOT hold references beyond the handler)
