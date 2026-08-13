---
title: Auth vocabulary is Scheme, UserStore and Access; there is no Guard, Gate or UserProvider
package: auth
tags: [auth, authentication, authorization, scheme, userstore, access, policy, jwt, session, AUTH_SCHEME]
use_instead: auth.Manager.RegisterScheme / SetUserStore / Access, schemes.NewSessionScheme, schemes.NewJWTScheme, ormauth.Store[T]
deprecated: false
---
The authentication surface names three things: a `Scheme` (how a request is
authenticated), a `UserStore` (where users are read from), and `Access` (the
authorizer behind abilities and policies). Guard, Gate and UserProvider are not
part of the API.

    m.RegisterScheme("web", webScheme)   // auth.Manager
    m.SetDefaultScheme("web")
    m.RegisterUserStore("users", store)
    m.Access().Define("edit-post", fn)

Key points:

- Concrete schemes live in `auth/drivers/schemes`:
  `schemes.NewSessionScheme`, `schemes.NewJWTScheme`.
- The ORM-backed user store is `ormauth.Store[T]` in `auth/stores/ormauth`,
  re-exported at the root as `velocity.ORMUserStore[T]`.
- The env var that names the default scheme is `AUTH_SCHEME`, not AUTH_GUARD.
  When it is empty, `ConfigFromEnv` builds no scheme configs at all, so auth is
  unconfigured and the auth model helpers return
  `velocity.ErrAuthNotConfigured`.
- Authorization from a handler goes through the context: `ctx.Can(...)`,
  `ctx.Cannot(...)`, `ctx.Authorize(...)`. They delegate to `Access` via the
  `contract.AuthManager` interface and degrade safely when auth is not
  configured.
