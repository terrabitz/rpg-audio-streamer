# Application architecture

RPG Audio Streamer (also labeled Skald Bot in the UI) is a multi-table audio
library and synchronized listening app for tabletop games. One configured Game
Master account manages named tables and a shared track library. Each table has
its own track associations and public table URL. Players see their table's
name, opt in to that table's playback, and adjust their local volume mix.

## Runtime and boundaries

```mermaid
flowchart LR
  GM[GM browser] --> UI[Vue UI]
  Player[Player browser] --> UI
  UI -->|HTTP via generated SDK| API[Go HTTP server]
  UI <-->|WebSocket sync| Hub[Go WebSocket hub]
  UI -->|Table-scoped HLS requests| API
  API --> Auth[Authentication service]
  API --> Store[SQLite datastore adapter]
  Store --> Queries[Generated sqlc queries]
  Queries --> DB[(SQLite metadata)]
  API --> FFmpeg[FFmpeg conversion]
  FFmpeg --> Uploads[HLS files on disk]
  API --> Uploads
```

`main.go` composes configuration, logging, embedded migrations, the SQLite
adapter, authentication service, hub, and HTTP server. Startup applies pending
migrations before accepting requests. The CLI also exposes migration commands;
`cmd/hashpass` produces the Argon2id password hash used in configuration.

The Go server serves `/api/v1` routes; it does not serve the Vue application.
The Vue app is built and hosted separately. Vite development/preview uses an
API and WebSocket proxy (by default to `http://localhost:8080`); the proxy target
can be supplied through `API_PROXY_TARGET`. The browser SDK base URL comes from
`VITE_API_BASE_URL`, normally `/`.

The optional API `--host` / `HOST` setting controls the listener interface;
its empty default preserves listening on all interfaces. The seeded local
development launcher (`scripts/dev.mjs`, exposed as `make dev`) binds both
the API and Vite to loopback on automatically selected ports. Each invocation
builds the real API into a unique temporary directory and creates a fresh
SQLite database, upload directory, and Vite cache. It uses public fixture GM
credentials, freshly salted native Argon2id password hashes, and per-instance
random signing keys. Nine generated audio fixtures (three per default
track type) use ordinary display names and are uploaded into the default table
through authenticated HTTP. Music fixtures use short original note sequences;
seeded ambiance noise gently swells over time so loop crossfades are audible.
Repeating fixtures remain 12 seconds long; one-shot tones retain their short
durations. This exercises normal validation, conversion, and persistence rather than writing
database fixtures directly. Vite uses the isolated API proxy and ignores
developer dotenv files; the API runs from the temporary directory with
explicit configuration. The launcher owns shutdown and removes its temporary
state on normal termination or startup failure. Uncatchable termination
cannot guarantee cleanup. Procedures live in the local-dev skill.

## Backend

| Package | Responsibility |
| --- | --- |
| `internal/auth` | Argon2id credential checks, signed JWTs, GM/player roles |
| `internal/server` | HTTP handlers, cookies/middleware, public table access, track/table contracts, datastore and hub interfaces |
| `internal/middlewares` | API request logging and CORS |
| `internal/sqlitedatastore` | Domain-to-SQLite mapping, migrations, context-aware datastore implementation |
| `internal/sqlitedatastore/sqlitedb` | Generated sqlc models and query implementations |
| `internal/websocket` | Authoritative live table playback, connected clients, message dispatch, recipient filters, read/write pumps |

There is not a separate Go domain package today. Authentication rules live
in `auth`; synchronization authorization/routing lives in `websocket`; the
track/table models and store interfaces live in `server`. The datastore implements
those interfaces, which keeps HTTP handlers independent of SQLite details.
`Server.Handler()` assembles the production HTTP pipeline independently of
binding a network listener. WebSocket upgrades bypass the API logging wrapper.

### Authentication and roles

GM login compares the configured username and Argon2id password hash, then
sets an HTTP-only `auth_token` cookie containing a signed JWT. Cookies are
secure outside development mode. Logout expires the cookie.

Each table has a database UUID and a separate, randomly generated Base64URL
external ID. Anyone with `/tables/<external-id>` can read that table's name
and associated tracks, stream those tracks, and connect to its WebSocket
without signing in or exchanging credentials. API stream URLs and WebSocket
connections carry the external ID; the server resolves it to the internal UUID
for table-scoped playback and track access. There is no player session, so each
tab can open a different table independently. A valid GM cookie on the same
URL selects the GM controls; it does not change the URL. GMs can rotate the
external ID; this invalidates the old URL and disconnects connected players.

Only GMs can create, rename, or delete tables, associate tracks, manage the
shared library, or change table settings. The auth-status endpoint reports GM
authentication only.

### Library and persistence

SQLite stores metadata, not media bytes:

- `tracks`: UUID ID, creation timestamp, display name, disk path, track-type ID,
  and shared playback-region points (beginning, optional end, optional loop-back).
- `track_types`: UUID ID, name, color, repeat policy, simultaneous-play policy,
  and creation timestamp.
- `game_tables`: UUID ID, unique random Base64URL external ID, name, and JSON GM
  mixing/ordering settings. The external ID is the public table identifier
  used in player URLs; the UUID remains the internal database identity.
- `table_tracks`: many-to-many table/track associations.

UUIDs are stored as BLOBs. Track creation timestamps are serialized as RFC3339
text, with second precision. Partial updates preserve omitted name/type fields.
The initial migration seeds Ambiance (repeating, simultaneous), Music
(repeating, exclusive), and One-Shot (non-repeating, simultaneous).
Empty metadata collections are returned as JSON arrays, not `null`.

Each table stores the GM's master volume, per-type volume map (keyed by type
name), per-track volume map (keyed by track UUID), and ordered track UUIDs.
The GM-only settings endpoint reads these preferences and accepts partial
patches: volume maps merge by key, the order is replaced, and omitted fields
are preserved atomically. Master volume accepts 0–200 percent; type and track
volumes accept 0–100 percent, including mute. Tables default to 100 percent
volume and creation-time track order. Playback position, play/pause state,
duration, and fades are not part of stored settings.

The table migration seeds "My Table" and associates all pre-existing tracks
with it. New tables start empty. Removing a track association or deleting a
table never deletes library audio. Deleting
a library track removes its associations everywhere. Cleanup triggers enforce
association removal even on SQLite connections without foreign keys enabled.

An upload is multipart audio plus a display name, type UUID, and optional table
UUID. The server validates the type, assigns a UUIDv7, stages the input, and runs FFmpeg to
produce AAC HLS playlists and `.ts` segments under `<upload-dir>/<track-id>/`.
It then saves metadata; table uploads save the library entry and association
in one database transaction. Streaming reads files from that upload directory.
After conversion the server also computes a 400-bucket peak waveform with
FFmpeg and caches it as `waveform.json` beside the playlist; a failure there only
logs a warning. The GM-only `GET /api/v1/files/{trackID}/waveform` endpoint
serves that cache, generating it on first request for older tracks.
Deletion removes metadata and the associated HLS directory. These filesystem
and database operations are not a single atomic transaction.

`sql/migrations` defines schema evolution; `sql/queries` is the authoritative
query source. `openapi.yaml` defines the HTTP client contract. Generated Go
queries and the TypeScript SDK are adapters, not handwritten business logic.

## Frontend

The Vue 3 application uses Vue Router, Vuetify, and Pinia. Routes are `/`
(landing), `/login` (GM credentials), `/join` (player invite-URL guidance),
`/tables` (GM table creation, renaming, and deletion), `/library` (shared
uploads and metadata management), and `/tables/:externalID` (the shared
GM/player table URL). GM login opens table management. Join as Player opens a
small information page explaining that players need the full
`/tables/:externalID` URL from their GM; it creates no player session. Table
views show associated tracks only; GMs can add an existing library track or
upload a new one directly into the table.
To preserve space during play, the table name appears only in the app bar.
Authenticated navigation lives in a left sidebar that is closed by default and
overlays the page when opened, with logout at the bottom. Logged-out visitors
see a login button in the app bar; player views show neither navigation nor login.
On GM tables, the master-volume control appears in the desktop now-playing
sidebar and the mobile now-playing drawer. **Add Track** and **Copy invite link**
appear at the top-right of the app bar on desktop and in its three-dot options
menu on smaller screens. **Rotate invite link** is always a red item in that
menu. Its confirmation dialog warns that active players will be disconnected
and the GM must reshare the new link; cancellation leaves the link unchanged.
After rotation, the GM's table URL updates to the new invite code.
**Add Track** opens a dialog with library selection and new-upload tabs;
neither workflow adds a permanent control row to the track grid.
Uploads require a file, a nonblank name, and a valid library track type before
the Upload action is enabled.
Upload results appear as compact, auto-dismissing bottom-center toasts attached
to the document body, independent of the uploader's app-bar or library placement;
upload failures remain inline while the dialog is open.
The table-management list opens a table when its row is clicked. Each row's
three-dot menu copies the invite code and also contains rename, delete, and
red invite-link rotation actions. Rotation uses the same confirmation dialog
as the app bar.

The shared library uses a sortable, paginated metadata table showing each
track's name, type, and creation time. Row clicks (or keyboard
activation) toggle a local preview, with at most one preview playing at once.
Resuming a paused preview continues from its current position.
Library previews reuse the HLS player without opening a WebSocket or sending
synchronization messages. Preview state is cleared on leaving the library;
it cannot become a table's playback state. Track editing is shared with the
table tiles through the audio-controls dialog's activator slot. Before deleting
a library track, the GM-only usage endpoint counts its current table
associations. When that count is nonzero, a separate follow-up dialog states
how many tables lose the track and must be confirmed; unused tracks delete
immediately. Deletion is blocked if the usage lookup fails. The deletion compares the confirmed count
atomically with the current associations; if it changed, the GM must check
usage again before deleting.
The audio-controls dialog combines seeking and playback-region editing on one
full-duration timeline, with a play/pause button to its left that uses the same
playback path as the track tile. When the dialog opens it fetches the track's
waveform once and draws it as the timeline bar: the played portion is bright, the
unplayed region dimmer, and trimmed areas grey. If the waveform cannot be loaded
the plain bar is shown instead. Beginning and end handles sit above the bar; repeating
types also have a loop-back handle below it. The draft type selection controls
its visibility without discarding saved loop-back points. Areas outside the
selected region are dimmed and cannot be
selected for seeking. Handles support mouse/touch dragging and keyboard adjustment.
The handles display their timestamps directly. Moving a boundary keeps the
loop-back point inside the region. The end defaults to the track duration;
moving the loop-back handle to the beginning reanchors it so it follows later
beginning adjustments. Moving the beginning to zero and the end to the full
duration restores the full track without separate reset controls. Point edits
immediately update live playback (including listening players at the current
table), but shared library metadata is only persisted on Save. Closing without
saving restores the saved region in playback; invalid drafts are not applied.
Region edits preserve each listener's live playhead without seeking to a reported
position; only a playhead outside the edited region is moved to its nearest boundary.
An in-flight loop crossfade keeps its incoming source playing through marker edits,
preserving its position when valid and moving it into the edited region otherwise.
Shortening the end can complete that handoff early; extending it applies to the next
loop. A source that reaches its end without a prepared crossfade starts the next
loop from the configured loop-back point.
These settings are stored with the shared library track and apply in every table.
Opening library controls loads audio metadata without starting playback or
sending synchronization messages.

The table route checks for an optional GM cookie and loads by public external ID.
It mounts the GM controls for a logged-in GM and the player view otherwise.
Players need no session, so switching links or opening multiple table tabs
requires no authentication exchange.

| Surface | Responsibility |
| --- | --- |
| `stores/auth.ts` | GM session role, credentials, and logout |
| `stores/files.ts`, `stores/trackTypes.ts`, `stores/tables.ts` | Library, track-type, table, and association metadata through the generated SDK |
| `stores/audio.ts` | Per-file playback state, synchronization reconciliation, fades, master/type volumes |
| `stores/websocket.ts` | Socket connection/reconnection, message handlers and history |
| `stores/debug.ts` | Development-only debug panel state |
| `composables/` | Shared app-bar state, browser origin, immutable partial-object patches |
| `FileList`, `AudioControls`, `PlayingTrackList`, `TrackPlaybackControls`, `TrackTimeline`, `AudioUploader` | Shared-library metadata management and uploads; table associations, responsive now-playing surfaces, and shared playback controls |
| `AudioPlayer`, `AudioTrackPlayer` | Per-track HLS.js media elements and playback synchronization |
| `VolumeMixer`, `VolumeSlider` | Active master and per-type mixing; player preferences stay local |

Library and track-type metadata and player mixer preferences use the Pinia
persistence plugin. GM mixes and track order load from the server before table
controls become available. They do not inherit browser-local mixer preferences.
Mix edits are debounced and flushed when leaving the table; saves are serialized
with ordering edits, and failures show an explicit retry action. Table metadata,
GM settings, and playback state are not persisted in browser storage.
Browser playback is keyed by file UUID within the active table and
cleared on table changes, so even the same library track cannot inherit another
table's playback. GM controls wait for the server's playback snapshot before
becoming available. Local browser state is not authoritative for stored metadata
or table playback.
Players must explicitly connect audio; full-state synchronization is ignored
while listening is disabled.
Associated entries initialize GM playback state as they load. Removing a table
association stops that track for the table; library deletion stops it in every
connected table. The SDK's shared runtime configuration throws on HTTP failures
so store and upload error handlers cannot report false success.

GM track tiles show their track type as a colored border instead of a visible
badge, with a screen-reader label retaining the type name. Cards stay compact
on hover, focus, and touch. Playing tiles have a saturated background mixed from
their track-type color; paused tiles retain the neutral surface. Tile names stay
on one line with ellipsis overflow and a full-name hover title.
A currently-playing sidebar lists active table tracks
with volume, playback-position, elapsed/total time, pause, and restart controls,
and provides master volume even when no tracks are playing.
The sidebar timer and seek bar's accessible value text measure elapsed time from
the configured beginning and total time to the configured end. Seeking still
uses absolute positions within the source audio.
Compact cards use a single-line name, tightly spaced controls, and a 260 px
desktop sidebar to preserve grid space. `PlayingTrackList` renders the same
cards in the sidebar and mobile drawer. The GM table fills the viewport width
without outer gutters or a centered maximum width. The desktop sidebar sits
flush against the right edge below the app bar, with a distinct surface,
left divider, full available viewport height, and independent scrolling.
The track grid uses the remaining width with 10 px of inner padding and 3 px
between cards; the sidebar remains flush against the right edge.
Truncated names retain their full
accessible label and hover title. On smaller screens, a fixed bottom bar shows
the playing-track count and opens a scrollable bottom drawer rather than taking
space above the grid. The drawer is capped at 70% of the viewport height and
supports closing with its button, Escape, or the backdrop; closing it does not
affect playback. The grid reserves space for the bar and device safe area.
Vuetify's `smAndDown` state controls both the sidebar/drawer selection and the
grid's mobile layout so they switch at the same breakpoint.
Paused, completed, removed, and type-exclusive displaced tracks leave the sidebar;
starting a track adds it automatically. Each card's always-visible three-dot
button opens a track-settings dialog with a volume slider aligned to the
timeline and a play/pause button, but no restart button.
`TrackPlaybackControls` supplies the compact sidebar controls; `TrackTimeline`
supplies the editor. Both use the same state and synchronization handlers.
Seeking preserves the playing/paused state; playing tracks synchronize volume
and seek changes to listening players. Sidebar restart seeks to the beginning.
Repeating tracks with a loop-back point distinct from the beginning show a small
orange tick on the compact seek bar. Anchored loop-back points have no tick.
Sidebar volume and seek bars use 6 px tracks and 18 px thumbs, matching the editor.
The editor's volume and seek bars share the same width, thickness, and thumb size, with
elapsed/total time centered vertically to the right of the seek bar. The metadata
dialog retains the track-type selector.
A small top-left drag handle enables mouse or touch dragging via `@dnd-kit/vue`
after a 4 px movement. `SortableTrack` registers each grid cell and its handle.
The lifted card follows the pointer while a dashed insertion placeholder moves
between the other cards, which animate aside. Only the handle initiates dragging;
other card areas retain normal touch scrolling. Card clicks or Enter/Space toggle
playback, while the inline controls do not trigger card clicks. Focused cards or
handles can be reordered with Alt + Left/Right, with screen-reader announcements.
Cancelled drags restore the original layout without changing the saved order.
The table store loads track order from server settings, scoped by table ID,
preserves it across metadata refreshes, appends newly discovered tracks, and
drops deleted tracks from the displayed order. Reordering saves to the server
so another GM browser restores the same layout. Ordering does not change
playback state or player mixes.

### Playback synchronization

The WebSocket hub owns authoritative live playback state in memory, keyed by
table and track UUID. Its datastore boundary reads track membership, library
metadata, track-type policies, and saved mixing settings through the existing
store interfaces.
Each connection has a server-validated table scope: the GM selects an existing
table by its external ID, while players supply that public external ID from the
URL. All routing filters,
including targeted full syncs and sync requests, also require a matching table.
Every GM and listening player requests a snapshot with `syncRequest` on
connection and reconnection. The server replies directly with `syncAll`,
including paused tracks and their positions; it does not need a connected GM.
GMs submit partial `syncTrack` commands for play/pause, seek, volume, duration,
and live playback-region edits. The server validates table membership and
regions, applies commands serially, and broadcasts canonical snapshots to both
roles, including the commanding GM. Omitted fields retain their current server
values rather than being replaced by a stale browser snapshot. Players cannot
change playback, and client-originated `syncAll` messages are not accepted.

Leaving a GM table disconnects that browser without changing table playback.
Deleting a table notifies and disconnects its active clients. Server-originated
`removeTrack` messages stop removed/deleted tracks for both roles. Socket
reconnects preserve table scope; leaving or switching tables cancels old retries
and ignores stale socket callbacks.

The server enforces track-type exclusivity atomically: starting Music stops other
Music tracks, while Ambiance and One-Shot can overlap. Snapshot positions advance
from server timestamps, accounting for repeating-region crossfade overlap and
one-shot completion without browser time-update messages. Browsers supply media
duration as metadata becomes available; the server does not decode audio.
Active tables receive a snapshot each second, keeping control-only browsers
and completion state current even when local audio cannot autoplay.
Explicit seek and play/pause commands advance a position version. Running media
uses that version to distinguish commanded position changes from clock snapshots,
preserving local loop handoffs during live marker edits.
GM snapshots carry unscaled track volume and saved table settings. Successful
settings patches update connected GM mixes and listener playback without
echoing remote changes back into another settings save. Player snapshots apply
the table master volume once; each listener then applies their own master and
per-type multipliers. Local autoplay failures do not pause the shared table.

HLS.js binds authenticated playlists to hidden media elements. Track state
controls play/pause, seeking, looping, and volume. Playback starts at the
configured beginning and stops or loops at the configured end. Repeating tracks
use a second HLS media element to crossfade over a global two-second interval
(shortened for shorter loop regions), starting the incoming source at the
loop-back point or configured beginning. Significant seek differences are
reconciled. Track, master, and type-volume changes ramp the mixed gain over two
seconds independently of play/pause fades and loop crossfades. A new adjustment
starts from the current interpolated gain, including mute and non-repeating
tracks. Removing a media component destroys its HLS instances and clears
playback timers; queued media events cannot recreate deleted track state.
Live playback survives every GM disconnect, table navigation, and browser
refresh for the lifetime of the server process. Reopening a table restores its
saved mix and layout together with the server's current playing/paused state
and positions. A server restart clears live playback; SQLite persists library
regions and table mixing/ordering settings, not the live playback clock.

## Release pipeline

Pushes to `main` that match the release workflow's path filters run
semantic-release in dry-run mode to calculate a version and release notes from
conventional commits and existing tags. Only the commit analyzer and notes
generator are enabled; the workflow does not commit a changelog or push to
`main`. `gh release create` creates the version tag at the triggering commit
and publishes the generated notes on GitHub, respecting branch protection.
Release notes live in GitHub releases.

Release runs are serialized to avoid competing version calculations. When no
release is needed, publication and image building are skipped. After successful
publication, the shared `docker-build` workflow builds the Linux amd64 and
arm64 images natively on matching GitHub-hosted runners, pushes each by digest
to GHCR, and combines them into a multi-arch manifest tagged `latest`,
full-version, major.minor, and major. Each architecture keeps its own Buildx
GitHub Actions cache scope.

Pull requests that touch any image input run the same per-architecture builds
without pushing or publishing a manifest, and check that the Dockerfile's Go
and Node base images match `go.mod` and `ui/.nvmrc`, since dependency bumps do
not update the Dockerfile.

All pull requests run the `pr-checks` workflow. `dorny/paths-filter` selects Go
tests for backend inputs, frontend lint/build/domain checks for UI, development
scripts, or OpenAPI inputs, full-app browser and seeded development instance
tests for either, and Docker checks for image inputs.
Changes to the workflow select every check; manual runs also execute every
check. The final `PR Checks` job depends on path detection and all check jobs,
accepts successful or skipped jobs, and fails on failed or cancelled jobs.
It provides a single required-check target, including for documentation-only
pull requests.

## Architectural assurance

Domain tests exercise authentication, routing policies, and frontend state
transitions. SQLite integration tests use the real schema and adapter against
isolated file-backed databases. Browser journeys run the built Vue app with
the real Go server, database, FFmpeg, WebSockets, and HLS media playback.
These layers cover different boundaries rather than substituting mocks for
end-to-end behavior. Test execution and maintenance procedures live in the
repository testing skill.
