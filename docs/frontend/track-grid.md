---
type: UI Surface
title: Track grid
description: GM track tiles, layout, and drag-and-drop ordering.
resource: ui/src/components
tags: [frontend, layout, ordering]
---

# Track grid

## Tiles

GM track tiles show their track type as a colored border instead of a visible
badge, with a screen-reader label retaining the type name. Cards stay compact
on hover, focus, and touch. Playing tiles have a background mixed
from their track-type color, with a more saturated left portion showing elapsed
progress through the playback region. Paused tiles show the same position as a
subtle grayscale fill against the neutral surface. Non-playing tile contents,
including processing status icons, are monochrome; the type border stays colored.
A 2 px white bar along the bottom shows track volume from left to right,
independently of master volume. Each [tag](../data/track-tags.md) on the
track appears as a small color block in the bottom-right corner, with the tag
name in a hover tooltip and as its accessible label. Tile names
stay on one line with ellipsis overflow and a full-name hover title.

Card clicks or Enter/Space toggle playback for ready tracks, while the inline
controls do not trigger card clicks. Tiles open the
[track editor](./track-editor.md); tracks still
[processing or failed](./track-processing.md) use status indicators and failure
diagnostics instead of playback controls.

Right-clicking or long-pressing a card opens its context menu; keyboard users
can use Shift + F10 or the context-menu key. **Play from start** starts or
restarts the track at its configured playback-region beginning while retaining
its volume. The action is disabled for unavailable tracks or a disconnected
WebSocket. Touch scrolling and drag handles do not open the menu.

## Layout

The GM table fills the viewport width without outer gutters or a centered
maximum width. The track grid uses the remaining width beside the
[now-playing sidebar](./now-playing.md) with 10 px of inner padding and 3 px
between cards. On smaller screens the grid reserves space for the now-playing
bottom bar and device safe area. Vuetify's `smAndDown` state controls both the
sidebar/drawer selection and the grid's mobile layout so they switch at the
same breakpoint.

## Ordering

A small top-left drag handle enables mouse or touch dragging via
`@dnd-kit/vue` after a 4 px movement. `SortableTrack` registers each grid cell
and its handle. The lifted card follows the pointer while a dashed insertion
placeholder moves between the other cards, which animate aside. Only the handle
initiates dragging; other card areas keep normal touch scrolling. Focused cards
or handles can be reordered with Alt + Left/Right, with screen-reader
announcements. Cancelled drags restore the original layout without changing
the saved order.

The table store loads track order from server
[table settings](../data/table-settings.md), scoped by table ID, preserves it
across metadata refreshes, appends newly discovered tracks, and drops deleted
tracks from the displayed order. Reordering saves to the server so another GM
browser restores the same layout. Ordering does not change playback state or
player mixes.
