---
type: Workflow
title: Background track processing
description: Accepted audio jobs, worker lifetimes, terminal statuses, and restart recovery.
resource: internal/musicprocessing/service.go
tags: [data, media, processing]
---

# Background track processing

The music processing service accepts [uploads](./track-uploads.md) and
[YouTube imports](../backend/youtube.md), persists a `processing` track and
optional table association, and returns HTTP 202 before retrieval or conversion
finishes. Up to sixteen tasks can be accepted, with two processing concurrently.

## Lifetimes and cleanup

Jobs have a fifteen-minute deadline, including queue time. Closing a dialog,
navigating, or disconnecting does not cancel accepted tasks. Submission uses
the request context only for staging and metadata persistence; workers receive
the application's context through `Run(ctx)` as part of
[process composition](../runtime/composition.md).

Successful HLS conversion marks a track `ready`; waveform failure remains a
logged warning rather than failing the job. Failed processing records a
`failed` status and diagnostics and removes partial HLS output. Both outcomes
remove staging. Server startup marks interrupted tasks failed and removes
their partial files; tasks are not silently retried.

Playback commands, mixer sources, and HTTP streams reject tracks that are not
ready. The [processing UI](../frontend/track-processing.md) reflects these
states without preventing reordering.

## Completion notifications

After persisting a terminal status, the service notifies the hub through its
`Notifier` dependency. The hub pushes
[`trackStatus`](../playback/websocket-protocol.md) to every connected GM.
Diagnostics are retained in SQLite and omitted from public player track
responses. Browser handling of pushed updates, response races, and reconnects
belongs to [client state](../frontend/state-management.md).
