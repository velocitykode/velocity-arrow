# Velocity Queue

## Drivers

- `memory` - In-process (dev only, lost on restart)
- `redis` - Redis-backed, production-ready
- `database` - Database-backed

## Jobs

Jobs implement the `Job` interface:

```go
type SendEmail struct {
    To      string
    Subject string
    Body    string
}

func (j *SendEmail) Handle() error {
    // send the email
    return nil
}

func (j *SendEmail) Failed(err error) {
    // handle failure
}
```

Optional interfaces: `MaxAttempter`, `OnQueuer`, `Backoffer`, `RetryDecider`

## Dispatching

```go
services.Queue.Push(&SendEmail{To: "user@example.com", Subject: "Welcome"})
services.Queue.PushDelayed(&SendEmail{...}, 5*time.Minute)
```

## Rules

- Jobs must be serializable - no channels, funcs, or unexported fields
- Handle failures gracefully via `Failed(error)`
- Use `MaxAttempts()` and `Backoff()` for retry control
- Keep jobs small and focused - one responsibility per job
