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
produces Argon2id hashes for password bootstrapping.

`internal/auth` uses the SQLite datastore for admin accounts. If no account exists,
`ROOT_PASSWORD` creates one with `ROOT_USERNAME` (default `admin`);
`ROOT_PASSWORD_HASH` accepts a precomputed Argon2id hash instead. Once an
account exists these inputs do not change it. With neither password input,
the server issues a [setup code](../backend/authentication.md). `TOKEN_SECRET`
configures JWT signing independently; if absent, the server uses a key
generated and persisted in SQLite.

The CLI owns the signal-cancelled application context, recovers interrupted
processing before serving, and runs the processing service and WebSocket hub
with `Run(ctx)`. Shutdown cancels workers, drains accepted processing jobs into
a failed state, stops hub event admission, waits for in-flight dispatches,
closes socket clients, and waits for
both services before closing the datastore. The HTTP server receives its
lifetime context through `Start(ctx)`; neither the server nor the processing
service stores a context.

The HTTP server owns the [live mixer](../discord/live-mixer.md) and registers
its event adapter on the shared hub before accepting connections. The adapter
announces mixer readiness and consumes audio commands only when
[Discord streaming](../discord/voice-streaming.md) is enabled. The hub owns
event dispatch and subscriptions, not the mixer service.
Each accepted socket connection registers its own typed event listeners on
that same bus and releases them when the connection ends.
