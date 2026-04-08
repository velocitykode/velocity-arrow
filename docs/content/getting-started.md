# Getting Started with Velocity

Velocity is a Go web framework designed for building modern web applications.

## Installation

```bash
go install github.com/velocitykode/velocity-cli@latest
vel new my-app
cd my-app
go run .
```

## Project Structure

A typical Velocity application:

```
my-app/
├── app/
│   ├── provider.go       # Service providers
│   ├── routes.go         # Route definitions
│   └── controllers/      # HTTP handlers
├── config/               # Configuration files
├── storage/
│   ├── logs/            # Application logs
│   └── cache/           # File cache
├── .env                  # Environment configuration
├── main.go              # Application entry point
└── go.mod
```

## Configuration

Configuration is loaded from `.env` files and environment variables using `velocity.ConfigFromEnv()`.

## Providers

Service providers are the central place to configure your application. They implement:

- `Register(s *Services) error` - bind services
- `Boot(s *Services) error` - wire dependencies
- `Shutdown(ctx context.Context) error` - cleanup
