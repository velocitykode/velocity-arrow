# Velocity Cache

## Drivers

- `memory` - In-process, lost on restart (good for dev)
- `file` - Filesystem-backed, in `CACHE_PATH` (default `./storage/cache`)
- `redis` - Redis-backed, configured via `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`
- `database` - Database-backed

## Usage

```go
cache := services.Cache
cache.Put("key", value, 5*time.Minute)
val, err := cache.Get("key")
cache.Forget("key")
cache.Flush()
```

## Rules

- Always set a TTL - avoid unbounded cache entries
- Use cache for read-heavy, compute-expensive data
- Invalidate on write - don't rely on TTL alone for consistency
- Use `CACHE_PREFIX` to namespace cache keys across environments
