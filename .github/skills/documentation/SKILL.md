---
name: documentation
description: Add, update, split, or reorganize RPG Audio Streamer documentation in docs/, the Open Knowledge Format (OKF v0.2) wiki of atomic, cross-linked concept files.
---

# Maintaining the docs wiki

`docs/` is an Open Knowledge Format (OKF v0.2) bundle: a wiki of small concept
files that describe the app's current structure and behavior. Procedures
(commands, workflows, checklists) belong in skills, not in `docs/`.

## Layout

- `docs/index.md` is the bundle root. Its frontmatter is only
  `okf_version: "0.2"`; its body lists the top-level areas.
- Each area directory (`runtime/`, `backend/`, `data/`, `frontend/`,
  `playback/`, `discord/`, `operations/`) has an `index.md` with no
  frontmatter: a heading and one `- [Title](./file.md) - Description` bullet
  per concept, using the concept's frontmatter `description`.
- Every other `.md` file is one concept: one component, data model, protocol,
  UI surface, workflow, or domain rule.

## Concept files

Start each concept with YAML frontmatter:

```yaml
---
type: UI Surface
title: Track editor
description: One sentence that also appears in the directory index.
resource: ui/src/components/TrackTimeline.vue
tags: [frontend, playback]
---
```

- `type` is required. Reuse the existing values: `Overview`, `Component`,
  `Component Map`, `Subsystem`, `Data Model`, `Protocol`, `Domain Concept`,
  `Security Model`, `UI Surface`, `Workflow`, `Tool`, `Pipeline`,
  `Quality Model`.
- `title` and `description` are expected. `description` is one sentence.
- `resource` is the repository-relative path of the primary code the concept
  describes; it must exist. Omit it for abstract concepts.
- `tags` start with the area name.
- Do not add `generated`, `verified`, `status`, or `stale_after`; docs are kept
  current through code review instead.

Write the body as an `#` title followed by short `##` sections. Keep each
concept focused; when a section grows into a separate idea, move it to a new
concept and link to it.

## Links

- Use relative markdown links (`../playback/seek-crossfades.md`) so they work
  in GitHub and editors.
- Weave links into prose where the relationship matters; do not add standalone
  "Related" sections.
- State each fact in one concept and link to it from others rather than
  repeating it.
- Refer to code paths with inline code, not markdown links.

## Changing the docs

1. Find affected concepts through `docs/index.md` and the area indexes, or by
   searching `docs/` for the changed package, component, or endpoint.
2. Update concepts to describe the current state only, without history or
   transition notes.
3. For a new concept, create the file, add it to its directory `index.md`, and
   link it from related concepts.
4. When renaming or removing a concept, update its index entry and every link to
   it.
5. Check conformance: every non-index file has frontmatter with a non-empty
   `type`, every relative link resolves, every `resource` path exists, and every
   concept is listed in its directory `index.md`.
