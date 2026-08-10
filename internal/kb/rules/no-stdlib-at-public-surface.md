---
title: Never expose the standard library at a public API surface; wrap it
package: (general)
tags: [api-design, conventions, velocity-max, stdlib, standard-library, wrap]
use_instead: a velocity helper, or an internal/ wrapper when none exists
deprecated: false
---
Reaching for the standard library without first checking whether velocity covers
the need is a defect. Use velocity helpers for anything the framework handles
(router, validation, events, contract, async, pipeline, str, httpclient, cache,
log, crypto, config, exceptions).

When no velocity equivalent exists, build the capability in an internal/ package
with a doc.go and wrap it; never let a raw stdlib type appear at the public API
surface. Escape order when a helper does not fit: compose other helpers; if it is
buggy, stop and route the fix upstream; only then build internally.
