---
title: Validation rules are typed constructor values, not rule strings
package: validation
tags: [validation, rules, vform, forms, requests, unique, exists, custom, dbrules]
use_instead: validation.Rules built from constructors such as validation.Required(), validation.Email(), validation.Unique("users", "email")
deprecated: false
---
A rule set is `validation.Rules`, a map of field name to a slice of typed `Rule`
values built by constructors. Rule strings such as "required|email|max:255" are
not a supported form, and there is no rule-registration step: `RegisterRule` does
not exist.

    err := ctx.Validate(validation.Rules{
        "name":  {validation.Required(), validation.Max(255)},
        "email": {validation.Required(), validation.Email(), validation.Unique("users", "email")},
    })

Because parameters are carried pre-split on the rule value, a parameter may
itself contain "," or "|" with no escaping.

Key points:

- Custom rules carry their handler: `validation.Custom(name, handler)`. It runs
  on every path, including the per-request validator, so nothing is registered
  globally. A custom name that shadows a built-in rule is rejected.
- Database rules describe the check only; execution lives in
  `validation/dbrules`. Use `validation.Unique(table, column)` with `.Except(id)`
  / `.IDColumn(name)`, and `validation.Exists(table, column)`.
- Messages are keyed by field and rule: `validation.Messages{{Field: "email",
  Rule: "required"}: "..."}`, not by a "field.rule" string.
- The form-request path is `vform.Form[T](ctx)`, where T implements
  `Rules() validation.Rules`. There is no ValidateRequest helper.
- On failure the validator writes the redirect-back response with flashed errors
  and old input, then returns `router.ErrValidationAborted`. Return that error
  from the handler; do not write a response after it.
