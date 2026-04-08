# Go Conventions for Velocity

## Error Handling

- Always check and return errors - never use `_` for error returns
- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`
- Use sentinel errors for expected conditions
- Log at the boundary, wrap in between

## Naming

- Use MixedCaps (not snake_case) for Go identifiers
- Package names are lowercase, single-word
- Interface names don't need `I` prefix - use `-er` suffix for single-method interfaces
- Unexported fields and methods are preferred unless needed externally

## Struct Design

- Keep structs focused - one responsibility
- Use constructor functions: `NewXxx(opts ...Option) *Xxx`
- Use functional options pattern for complex initialization
- Embed interfaces sparingly

## Concurrency

- Never start goroutines in handlers without coordination
- Use `context.Context` for cancellation propagation
- Protect shared state with `sync.Mutex` or channels
- Use `sync.Pool` for frequently allocated objects (Velocity already pools router.Context)

## Testing

- Table-driven tests with `t.Run()`
- Use `testing.TB` interface when helpers accept both `*testing.T` and `*testing.B`
- Test files go next to the code they test
- Use `testdata/` for test fixtures
- Prefer real implementations over mocks for integration tests

## Dependencies

- Minimize external dependencies
- Vendor critical dependencies
- Pin dependency versions in go.mod
