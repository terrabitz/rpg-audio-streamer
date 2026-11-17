---
type: UI Surface
title: Table management
description: GM table list with create, rename, delete, copy, and rotation actions.
resource: ui/src
tags: [frontend, tables]
---

# Table management

The `/tables` [route](./routes.md) lets GMs create, rename, and delete tables.
GM login opens this page.

The list opens a table when its row is clicked. Each row's three-dot menu
copies the invite code and also contains rename, delete, and red invite-link
rotation actions. Rotation uses the same confirmation dialog as the
[app shell](./app-shell.md) (see [public table links](../backend/public-table-links.md)).

Deleting a table never deletes library audio (see
[table track associations](../data/table-track-associations.md)).
