---
type: Security Model
title: Authentication and roles
description: GM login, signed cookies, and GM-only operations.
resource: internal/auth
tags: [backend, security, auth]
---

# Authentication and roles

GM login compares the configured username and Argon2id password hash (produced
by `cmd/hashpass`, see [process composition](../runtime/composition.md)), then
sets an HTTP-only `auth_token` cookie containing a signed JWT. Cookies are
secure outside development mode. Logout expires the cookie.

Only GMs can create, rename, or delete tables, associate tracks, manage the
shared library, or change [table settings](../data/table-settings.md). The
auth-status endpoint reports GM authentication only.

Players do not authenticate; they reach a table through its
[public table link](./public-table-links.md). A valid GM cookie on the same
table URL selects the GM controls.
