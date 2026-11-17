---
type: UI Surface
title: Screen wake lock
description: Browser-local Keep screen awake option on mobile tables.
resource: ui/src/composables
tags: [frontend, mobile, preferences]
---

# Screen wake lock

On mobile layouts, the table options menu in the [app shell](./app-shell.md)
includes **Keep screen awake**. This browser-local preference uses the Screen
Wake Lock API while the app is visible on a mobile layout, releases the lock
when disabled or hidden, and reacquires it when the app returns to the
foreground. Unsupported browsers show a disabled option, and denied requests
produce an error notification.

It is independent of [table settings](../data/table-settings.md) and is not
sent to the server or other clients. `stores/clientPreferences.ts` persists it
(see [client state](./state-management.md)).
