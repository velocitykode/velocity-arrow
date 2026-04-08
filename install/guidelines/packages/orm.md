# Velocity ORM

## Models

Use generic model types for type-safe database operations:

```go
type User struct {
    orm.Model[User]
    Name  string `db:"name"`
    Email string `db:"email"`
}
```

Model variants: `Model[T]`, `UUIDModel[T]`, `SoftDeleteModel[T]`, `SoftDeleteUUIDModel[T]`

## Querying

```go
user, _ := orm.Model[User]{}.Find(id)
user, _ := orm.Model[User]{}.FindBy("email", "user@example.com")
users, _ := orm.Model[User]{}.Where("active", true).Get()
user, _ := orm.Model[User]{}.Create(data)
```

## Rules

- Always handle the error return from query methods
- Use parameterized queries - never concatenate user input into SQL
- Column/table names are validated against `^[a-zA-Z_][a-zA-Z0-9_.]*$`
- Use transactions for multi-step operations
- Configure connection pool via `DB_MAX_IDLE_CONNS`, `DB_MAX_OPEN_CONNS`, `DB_CONN_MAX_LIFETIME`
