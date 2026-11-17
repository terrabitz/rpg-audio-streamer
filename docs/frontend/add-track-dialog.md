---
type: UI Surface
title: Add Track dialog
description: Library selection and new-upload tabs for adding tracks to a table.
resource: ui/src/components/AudioUploader.vue
tags: [frontend, uploads, library]
---

# Add Track dialog

**Add Track** (in the [app shell](./app-shell.md)) opens a dialog with library
selection and new-upload tabs; neither workflow adds a permanent control row to
the [track grid](./track-grid.md).

Uploads require a file, a nonblank name, and a valid library track type before
the Upload action is enabled. The server side of uploads is described in
[track uploads](../data/track-uploads.md).

Upload results appear as compact, auto-dismissing bottom-center toasts attached
to the document body, independent of the uploader's app-bar or library
placement; upload failures remain inline while the dialog is open.
