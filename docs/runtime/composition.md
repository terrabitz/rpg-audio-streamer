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
[WebSocket hub](../playback/synchronization.md),
[YouTube service](../backend/youtube.md),
[music processing service](../data/track-processing.md), and the
[HTTP server](./http-serving.md). Startup applies pending migrations before
accepting requests. The CLI also exposes migration commands; `cmd/hashpass`
produces the Argon2id password hash used in configuration.

The CLI owns the signal-cancelled application context, recovers interrupted
processing before serving, and runs the processing service and WebSocket hub
with `Run(ctx)`. Shutdown cancels workers, drains accepted processing jobs into
a failed state, stops hub event admission, waits for in-flight dispatches,
closes socket clients, and waits for
both services before closing the datastore. The HTTP server receives its
lifetime context through `Start(ctx)`; neither the server nor the processing
service stores a context.

When [Discord streaming](../discord/voice-streaming.md) is configured, the
HTTP server owns and runs the [live mixer](../discord/live-mixer.md), with a
Discord-aware wrapper for session restore and teardown. It registers the
mixer's event adapter on the shared hub before accepting connections. The hub
owns event dispatch and subscriptions, not the mixer service.
Each accepted socket connection registers its own typed event listeners on
that same bus and releases them when the connection ends.
