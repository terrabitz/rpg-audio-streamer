---
type: Component
title: Process composition
description: How main.go assembles configuration, migrations, services, and the server.
resource: main.go
tags: [runtime, backend]
---

# Process composition

`main.go` composes configuration, logging, embedded migrations, the
[SQLite adapter](../data/sqlite-schema.md), the
[authentication service](../backend/authentication.md), the
[WebSocket hub](../playback/synchronization.md), and the
[HTTP server](./http-serving.md). Startup applies pending migrations before
accepting requests. The CLI also exposes migration commands; `cmd/hashpass`
produces the Argon2id password hash used in configuration.

When [Discord streaming](../discord/voice-streaming.md) is configured, the
process also runs the [live mixer](../discord/live-mixer.md).
