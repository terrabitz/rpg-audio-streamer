---
type: Component
title: HTTP serving
description: API, WebSocket, and static UI serving in production and Vite development.
resource: internal/server
tags: [runtime, backend, frontend, deployment]
---

# HTTP serving

The Go server serves API and WebSocket routes under `/api` and can serve the
built Vue application from `--frontend-dir` / `FRONTEND_DIR`. Non-API navigation
paths fall back to `index.html`; missing static files return 404.
`Server.Handler()` assembles the production HTTP pipeline independently of
binding a network listener. WebSocket upgrades bypass the API logging wrapper
(see [Go packages](../backend/packages.md)).

The optional API `--host` / `HOST` setting controls the listener interface;
its empty default listens on all interfaces.

On cancellation, the HTTP server uses Go's standard graceful shutdown with a
ten-second deadline. This allows unused browser speculative preconnects to age
out while active HTTP requests finish. The
[WebSocket hub](./composition.md) separately closes upgraded connections.

## Docker image

The Docker image serves both the API and the UI on HTTP port 80 by default, with
no built-in TLS; use a TLS-terminating reverse proxy for remote production
access so browsers can send the secure authentication cookie. Docker Compose
can load `../.env`, but does not require that file.
The [release pipeline](../operations/release-pipeline.md) builds and publishes
the image.

## Vite development

Vite development uses an API and WebSocket proxy (by default to
`http://localhost:8080`); the proxy target can be supplied through
`API_PROXY_TARGET`. The browser SDK base URL comes from `VITE_API_BASE_URL`,
normally `/`. The [seeded local development launcher](./local-dev-launcher.md)
configures both automatically.
