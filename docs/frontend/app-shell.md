---
type: UI Surface
title: App shell
description: App bar, navigation sidebar, and table options menu.
resource: ui/src
tags: [frontend, navigation, layout]
---

# App shell

To preserve space during play, the table name appears only in the app bar.

## Navigation

Authenticated navigation lives in a left sidebar that is closed by default and
overlays the page when opened. Tables, Library, and Settings use music,
bookshelf, and cog icons; logout sits at the bottom. Settings contains the
password change form. Logged-out visitors
see a login button in the app bar; player views show neither navigation nor
login.

## GM table actions

On GM tables, the master-volume control appears in the desktop
[now-playing](./now-playing.md) sidebar and the mobile now-playing drawer.
**Add Track** (see [Add Track dialog](./add-track-dialog.md)) and
**Copy invite link** appear at the top-right of the app bar on desktop and in
its three-dot options menu on smaller screens.

**Rotate invite link** is always a red item in that menu. Its confirmation
dialog warns that active players will be disconnected and the GM must reshare
the new link; cancellation leaves the link unchanged. After rotation, the GM's
table URL updates to the new invite code (see
[public table links](../backend/public-table-links.md)).

On mobile layouts, the table options menu also includes
[Keep screen awake](./screen-wake-lock.md).

## Notifications

Operation failures use `ErrorToast`: a compact error snackbar with a Dismiss
action and a five-second timeout. Dismissing a notification does not discard
unsaved edits or failed settings patches; their retry controls remain available.
Form validation stays beside the relevant controls, and an unavailable or
deleted table displays a persistent page-status message rather than a banner.
