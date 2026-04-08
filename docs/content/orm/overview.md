# Velocity ORM

The Velocity ORM provides an expressive, fluent interface for interacting with databases.

## Supported Databases

- MySQL
- PostgreSQL
- SQLite

## Models

Models use Go generics for type-safe queries:

```go
type User struct {
    orm.Model[User]
    Name  string
    Email string
}
```

## Queries

```go
// Find by ID
user, err := orm.Model[User]{}.Find(1)

// Find by column
user, err := orm.Model[User]{}.FindBy("email", "user@example.com")

// Where clause
users, err := orm.Model[User]{}.Where("active", true).Get()

// Create
user, err := orm.Model[User]{}.Create(map[string]any{
    "name":  "John",
    "email": "john@example.com",
})
```

## Migrations

Migrations are managed through the Velocity CLI:

```bash
vel make:migration create_users_table
vel migrate
vel migrate:rollback
```
