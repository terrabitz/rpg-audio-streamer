---
type: UI Surface
title: Track grid
description: GM track tiles, layout, and rearrange mode for reordering tracks.
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
independently of master volume. Tile names
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
WebSocket. Touch scrolling does not open the menu, and the menu is unavailable
while rearranging.

## Layout

The GM table fills the viewport width without outer gutters or a centered
maximum width. The track grid uses the remaining width beside the
[now-playing sidebar](./now-playing.md) with 10 px of inner padding and 3 px
between cards. On smaller screens the grid reserves space for the now-playing
bottom bar and device safe area. Vuetify's `smAndDown` state controls both the
sidebar/drawer selection and the grid's mobile layout so they switch at the
same breakpoint.

## Ordering

Tracks are reordered in rearrange mode, entered through **Rearrange tracks** in
the table options menu when the table has at least two tracks. Outside this
mode cards have no drag affordance. In rearrange mode a sticky bar shows the
selection count and a **Finished** button that returns the grid to normal
playback. The grid becomes a multi-select listbox, card clicks select instead
of playing, and the three-dot and failure buttons are hidden.

Ctrl-click (Cmd-click on macOS), a touch tap, or Enter/Space toggles a track's
selection, and a plain click clears it. Shift-click, Shift + Space, or a touch
long-press adds the range between the last selected track and the target.
Selected cards show a primary outline and tint. Selection starts empty and
clears when leaving the mode.

Mouse dragging starts on any card after a 4 px movement; touch dragging starts
only on selected cards, so swiping unselected cards keeps scrolling the page.
`SortableTrack` registers each grid cell with `@dnd-kit/vue`. The lifted card
follows the pointer while a dashed insertion placeholder moves between the
other cards, which animate aside. Dragging a selected card moves the whole
selection: whichever card is grabbed, the lifted card renders as a stack topped
by the group's first track with a count badge, the other selected cards fade in
place, and on drop the group gathers contiguously, in its existing display
order, at the drop position. Focused cards move one position with
Alt + Left/Right. Moves are announced to screen readers, and cancelled drags
restore the original layout without changing the saved order.

The drag library moves the lifted card in the DOM while sorting, so Vue's keyed
patch cannot rely on the rendered order after a drop. Once the store order
updates, the grid reorders its card elements to match it.

The table store loads track order from server
[table settings](../data/table-settings.md), scoped by table ID, preserves it
across metadata refreshes, appends newly discovered tracks, and drops deleted
tracks from the displayed order. Reordering saves to the server so another GM
browser restores the same layout. Ordering does not change playback state or
player mixes.
