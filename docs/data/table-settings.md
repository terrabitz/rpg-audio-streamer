---
type: Data Model
title: Table settings
description: Per-table GM master and type mixing and track-order preferences.
resource: internal/server
tags: [data, mixing, tables]
---

# Table settings

Each table stores the GM's master volume, per-type volume map (keyed by type
name), and ordered track UUIDs in the `game_tables` [schema](./sqlite-schema.md).
Track volume is shared across tables and belongs to the library [track](./sqlite-schema.md).

The GM-only settings endpoint reads these preferences and accepts partial
patches: volume maps merge by key, the order is replaced, and omitted fields
are preserved atomically. Master volume accepts 0–200 percent and type volume
accepts 0–100 percent, including mute. Track volume is shared and accepts
0–100 percent, including mute. Tables default to 100 percent master volume and
creation-time track order.

Playback position, play/pause state, duration, and fades are not part of stored
settings; they belong to the in-memory
[playback synchronization](../playback/synchronization.md) state.

The frontend loads these settings before enabling GM controls and saves master,
type, and [track-order](../frontend/track-grid.md) edits back to them (see
[client state](../frontend/state-management.md)).
