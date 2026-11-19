---
type: Security Model
title: Authentication and roles
description: GM login, signed cookies, and GM-only operations.
resource: internal/auth
tags: [backend, security, auth]
---

# Authentication and roles

## Instance claiming

Without an admin account or bootstrap password, the instance opens
`/setup`. Startup prints a one-time `setup_code` to standard error. The server
keeps only its SHA-256 digest in memory; the code expires after 30 minutes.
Restarting an unclaimed instance issues a new code. `GET /api/v1/setup`
reports claim status without exposing the code. `POST` consumes the code only
after persisting the username and Argon2id password hash. There is no default
password or open registration. UI passwords require 12 characters and permit
at most 1024 UTF-8 bytes. The username and hash are saved in SQLite's `users`
table. [Deployment bootstrap](../runtime/composition.md) creates the same
account without a setup code.

## GM sessions

GM login reads and verifies the saved username and Argon2id password hash, then
sets an HTTP-only `auth_token` cookie containing a signed JWT. Cookies are
secure outside development mode. JWT validation does not read the account
from SQLite. Logout expires the cookie.

Settings uses `GET/PATCH /api/v1/profile` to change the password after
verifying the current password. The new password applies to future sign-ins;
existing cookies keep their original expiration, and open WebSockets stay
connected. There is no unauthenticated password reset.

Only GMs can create, rename, or delete tables, associate tracks, manage the
shared library, or change [table settings](../data/table-settings.md). The
auth-status endpoint reports GM authentication only.

Players do not authenticate; they reach a table through its
[public table link](./public-table-links.md). A valid GM cookie on the same
table URL selects the GM controls.
