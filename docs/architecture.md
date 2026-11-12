# Application architecture

RPG Audio Streamer (also labeled Skald Bot in the UI) is a multi-table audio
library and synchronized listening app for tabletop games. One configured Game
Master account manages named tables and a shared track library. Each table
has its own track associations and invite URL. Players see their table's name,
opt in to that table's playback, and adjust their local volume mix.

## Runtime and boundaries

```mermaid
flowchart LR
  GM[GM browser] --> UI[Vue UI]
  Player[Player browser] --> UI
  UI -->|HTTP via generated SDK| API[Go HTTP server]
  UI <-->|WebSocket sync| Hub[Go WebSocket hub]
  UI -->|Authenticated HLS requests| API
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
random signing/invite tokens. Nine generated audio fixtures (three per default
track type) use ordinary display names and are uploaded into the default table
through authenticated HTTP, exercising normal validation, conversion, and persistence rather than writing
database fixtures directly. Vite uses the isolated API proxy and ignores
developer dotenv files; the API runs from the temporary directory with
explicit configuration. The launcher owns shutdown and removes its temporary
state on normal termination or startup failure. Uncatchable termination
cannot guarantee cleanup. Procedures live in the local-dev skill.

## Backend

| Package | Responsibility |
| --- | --- |
| `internal/auth` | Argon2id credential checks, signed JWTs, GM/player roles |
| `internal/server` | HTTP handlers, cookies/middleware, table invite resolution, track/table contracts, datastore and hub interfaces |
| `internal/middlewares` | API request logging and CORS |
| `internal/sqlitedatastore` | Domain-to-SQLite mapping, migrations, context-aware datastore implementation |
| `internal/sqlitedatastore/sqlitedb` | Generated sqlc models and query implementations |
| `internal/websocket` | Connected clients, message dispatch, recipient filters, read/write pumps |

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

Players authenticate with a table-specific, randomly generated invite token,
not a GM cookie. The configured static invite token remains an alias for the
migrated default table; deleting that table invalidates the alias.
HTTP accepts that token as a bearer token; WebSockets use a `token` query
parameter because browser socket APIs cannot set Authorization headers.
Authentication resolves a cookie first, then the query token, then the bearer
token. Player authentication resolves the invite against SQLite on each request
and carries its table ID into streaming and WebSocket authorization. A GM
opening an invite URL retains their GM role but opens the invited table.
The auth-status endpoint returns the authenticated state and role.

Only GMs can manage tables, associate tracks, manage the shared library, or
retrieve invite tokens. Players can fetch their table's name, fetch track types,
stream HLS files associated with their table, and open authenticated WebSockets.
They cannot access other tables or list the library.

### Library and persistence

SQLite stores metadata, not media bytes:

- `tracks`: UUID ID, creation timestamp, display name, disk path, track-type ID.
- `track_types`: UUID ID, name, color, repeat policy, simultaneous-play policy,
  and creation timestamp.
- `game_tables`: UUID ID, name, and unique invite token.
- `table_tracks`: many-to-many table/track associations.

UUIDs are stored as BLOBs. Track creation timestamps are serialized as RFC3339
text, with second precision. Partial updates preserve omitted name/type fields.
The initial migration seeds Ambiance (repeating, simultaneous), Music
(repeating, exclusive), and One-Shot (non-repeating, simultaneous).
Empty metadata collections are returned as JSON arrays, not `null`.

The table migration seeds "My Table" and associates all pre-existing tracks
with it, preserving the existing invite URL. New tables start empty. Removing
a track association or deleting a table never deletes library audio. Deleting
a library track removes its associations everywhere. Cleanup triggers enforce
association removal even on SQLite connections without foreign keys enabled.

An upload is multipart audio plus a display name, type UUID, and optional table
UUID. The server validates the type, assigns a UUIDv7, stages the input, and runs FFmpeg to
produce AAC HLS playlists and `.ts` segments under `<upload-dir>/<track-id>/`.
It then saves metadata; table uploads save the library entry and association
in one database transaction. Streaming reads files from that upload directory.
Deletion removes metadata and the associated HLS directory. These filesystem
and database operations are not a single atomic transaction.

`sql/migrations` defines schema evolution; `sql/queries` is the authoritative
query source. `openapi.yaml` defines the HTTP client contract. Generated Go
queries and the TypeScript SDK are adapters, not handwritten business logic.

## Frontend

The Vue 3 application uses Vue Router, Vuetify, and Pinia. Routes are `/`
(landing), `/login` (GM credentials), `/join` (invite instructions), `/tables`
(GM table creation, renaming, and deletion), `/library` (shared uploads and
metadata management), `/tables/:tableID` (GM table), and `/table/:token?`
(invited table, or legacy default table for a GM without a token). Login opens
table management. Table views show associated tracks only; GMs can add an
existing library track or upload a new one directly into the table.
To preserve space during play, the table name appears only in the app bar.
Its **Add Track** action opens a dialog with library selection and new-upload
tabs; neither workflow adds a permanent control row to the track grid.
The table-management list opens a table when its row is clicked; rename and
delete live in each row's three-dot menu.

The shared library uses a sortable, paginated metadata table showing each
track's name, type, and creation time. Row clicks (or keyboard
activation) toggle a local preview, with at most one preview playing at once.
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

The table route resolves authentication before mounting a role-specific view.
The application shell checks authentication on non-table initial navigation;
table authentication is owned by the table view, not repeated in the player
view. This prevents competing status requests and logout-triggered player
reauthentication.

| Surface | Responsibility |
| --- | --- |
| `stores/auth.ts`, `stores/join.ts` | Session role, credentials, logout, invite retrieval |
| `stores/files.ts`, `stores/trackTypes.ts`, `stores/tables.ts` | Library, track-type, table, and association metadata through the generated SDK |
| `stores/audio.ts` | Per-file playback state, synchronization reconciliation, fades, master/type volumes |
| `stores/websocket.ts` | Socket connection/reconnection, message handlers and history |
| `stores/debug.ts` | Development-only debug panel state |
| `composables/` | Shared app-bar state, browser origin, immutable partial-object patches |
| `FileList`, `AudioControls`, `AudioUploader` | Shared-library metadata management and uploads; table associations and playback |
| `AudioPlayer`, `AudioTrackPlayer` | Per-track HLS.js media elements and playback synchronization |
| `VolumeMixer`, `VolumeSlider` | Local master and per-type mixing |

Library and track-type metadata and local mixer preferences use the Pinia
persistence plugin. Table metadata and playback state are not persisted in
browser storage. Playback is keyed by file UUID within the active table and
cleared on table changes, so even the same library track cannot inherit another
table's playback. Local browser state is not authoritative for stored metadata.
Players must explicitly connect audio; full-state synchronization is ignored
while listening is disabled.
Associated entries initialize GM playback state as they load. Removing a table
association stops that track for the table; library deletion stops it in every
connected table. The SDK's shared runtime configuration throws on HTTP failures
so store and upload error handlers cannot report false success.

### Playback synchronization

The server routes messages rather than storing authoritative playback state.
Each connection has a server-validated table scope: the GM selects an existing
table ID, while players inherit the table from their invite. All routing filters,
including targeted full syncs and sync requests, also require a matching table.
The GM browser owns current track state. On player connection, `syncRequest`
is forwarded to GM clients; a GM responds with `syncAll`, targeting the
requesting client. A full synchronization without a target goes to all players
at that table.
`syncTrack` carries incremental state from GMs to players. Players cannot issue
GM broadcasts, full-sync responses, or incremental sync commands.

Leaving a GM table sends an empty full sync to its players before disconnecting.
Deleting a table notifies and disconnects its active clients. Server-originated
`removeTrack` messages stop removed/deleted tracks for both roles. Socket
reconnects preserve table scope; leaving or switching tables cancels old retries
and ignores stale socket callbacks.

The GM enforces track-type exclusivity in the UI: starting Music stops other
Music tracks, while Ambiance and One-Shot can overlap. GM master volume is
applied to outgoing synchronization. Each listener applies its own master
and per-type multipliers to incoming volume.

HLS.js binds authenticated playlists to hidden video elements. Track state
controls play/pause, seeking, looping, and volume. Repeating tracks fade over
approximately two seconds; significant seek differences are reconciled.
Removing a media component destroys its HLS instance and clears playback
timers; queued media events cannot recreate deleted track state.
Playback is ephemeral browser state, not persisted in SQLite or recovered by
the server after every GM disconnect.

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
publication, the Docker job builds Linux amd64/arm64 images and pushes `latest`,
full-version, major, and major.minor tags to GHCR.

Pull requests that touch any image input build the same multi-arch image and
check that the Dockerfile's Go and Node base images match `go.mod` and
`ui/.nvmrc`, since dependency bumps do not update the Dockerfile.

## Architectural assurance

Domain tests exercise authentication, routing policies, and frontend state
transitions. SQLite integration tests use the real schema and adapter against
isolated file-backed databases. Browser journeys run the built Vue app with
the real Go server, database, FFmpeg, WebSockets, and HLS media playback.
These layers cover different boundaries rather than substituting mocks for
end-to-end behavior. Test execution and maintenance procedures live in the
repository testing skill.
