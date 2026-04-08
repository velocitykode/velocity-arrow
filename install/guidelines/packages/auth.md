# Velocity Auth

## Guards

- `web` - Session/cookie-based authentication
- `jwt` - JWT token-based authentication

## Configuration

- `AUTH_GUARD` - Default guard (web or jwt)
- `SESSION_DRIVER` - cookie or file
- `JWT_ALGO` - HS256, RS256, HS512
- `JWT_TTL` - Token lifetime in minutes
- `JWT_REFRESH_TTL` - Refresh token lifetime in minutes

## Usage

```go
// In a handler
user, err := services.Auth.User(c)
if err != nil {
    return c.Unauthorized()
}

// Login
token, err := services.Auth.Attempt(credentials)

// Logout
services.Auth.Logout(c)
```

## Password Hashing

Uses bcrypt via `golang.org/x/crypto`:

```go
hash, _ := auth.HashPassword("plaintext")
ok := auth.CheckPassword("plaintext", hash)
```

## Rules

- Never store plaintext passwords
- Use `HASH_BCRYPT_COST` of at least 10
- Always validate JWT tokens on every request (middleware)
- Enable `JWT_BLACKLIST_ENABLED` for token revocation
- Use HTTPS in production for cookie/session security
