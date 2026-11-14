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
  Hub -.->|Experimental GM commands| Mix[Live table mixer]
  Mix --> PCM[Standalone PCM core]
  Mix -->|Decode stored HLS| FFmpeg
  Mix -.->|Optional PCM subscription| Voice[Node Discord voice worker]
  Voice -->|DAVE-encrypted Opus| Discord[Table-selected Discord voice channel]
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
through authenticated HTTP. Music fixtures use short original note sequences;
seeded ambiance noise gently swells over time so loop crossfades are audible.
Repeating fixtures remain 12 seconds long; one-shot tones retain their short
durations. This exercises normal validation, conversion, and persistence rather than writing
database fixtures directly. Vite uses the isolated API proxy and ignores
developer dotenv files; the API runs from the temporary directory with
explicit configuration. The launcher owns shutdown and removes its temporary
state on normal termination or startup failure. Discord is enabled in a seeded
preview only when explicitly requested through `DEV_DISCORD=1`; an allowlisted
loader reads its configuration from the worktree environment or `.env`, and
the API automatically enables the live mixer. Ordinary previews and browser
tests strip ambient Discord credentials before starting the API.
Uncatchable termination
cannot guarantee cleanup. Procedures live in the local-dev skill.

## Backend

| Package | Responsibility |
| --- | --- |
| `internal/auth` | Argon2id credential checks, signed JWTs, GM/player roles |
| `internal/server` | HTTP handlers, cookies/middleware, table invite resolution, track/table contracts, datastore and hub interfaces |
| `internal/middlewares` | API request logging and CORS |
| `internal/mixer` | Standalone deterministic stereo PCM mixing, transport, loop overlap, and gain envelopes |
| `internal/livemixer` | Serialized live commands, table-scoped PCM engines, rendering lifecycle, and diagnostics |
| `internal/discord` | Bot installation/server discovery, table-bound voice connections, per-server reservations, bounded PCM transport, and worker lifecycle |
| `discord/` | DAVE-compatible Discord voice worker, PCM conversion, and Opus transport |
| `e2e/` | Opt-in live Discord suite: full-app harness, Go listener bot, Opus decoding, and tone analysis |
| `internal/sqlitedatastore` | Domain-to-SQLite mapping, migrations, context-aware datastore implementation |
| `internal/sqlitedatastore/sqlitedb` | Generated sqlc models and query implementations |
| `internal/websocket` | Connected clients, message dispatch, recipient filters, read/write pumps |

Authentication rules live in `auth`; synchronization authorization/routing lives in `websocket`; the
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

- `tracks`: UUID ID, creation timestamp, display name, disk path, track-type ID,
  and shared playback-region points (beginning, optional end, optional loop-back).
- `track_types`: UUID ID, name, color, repeat policy, simultaneous-play policy,
  and creation timestamp.
- `game_tables`: UUID ID, name, unique invite token, and JSON GM mixing/ordering settings.
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
with it, preserving the existing invite URL. New tables start empty. Removing
a track association or deleting a table never deletes library audio. Deleting
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
(landing), `/login` (GM credentials), `/join` (invite instructions), `/tables`
(GM table creation, renaming, and deletion), `/library` (shared uploads and
metadata management), `/tables/:tableID` (GM table), and `/table/:token?`
(invited table, or legacy default table for a GM without a token). Login opens
table management. Table views show associated tracks only; GMs can add an
existing library track or upload a new one directly into the table.
To preserve space during play, the table name appears only in the app bar.
Its **Add Track** action opens a dialog with library selection and new-upload
tabs; neither workflow adds a permanent control row to the track grid.
Uploads require a file, a nonblank name, and a valid library track type before
the Upload action is enabled.
Upload results appear as compact, auto-dismissing bottom-center toasts attached
to the document body, independent of the uploader's app-bar or library placement;
upload failures remain inline while the dialog is open.
The table-management list opens a table when its row is clicked; rename and
delete live in each row's three-dot menu.

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
These settings are stored with the shared library track and apply in every table.
Opening library controls loads audio metadata without starting playback or
sending synchronization messages.

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
| `FileList`, `AudioControls`, `PlayingTrackList`, `TrackPlaybackControls`, `TrackTimeline`, `AudioUploader` | Shared-library metadata management and uploads; table associations, responsive now-playing surfaces, and shared playback controls |
| `AudioPlayer`, `AudioTrackPlayer` | Per-track HLS.js media elements and playback synchronization |
| `VolumeMixer`, `VolumeSlider` | Active master and per-type mixing; player preferences stay local |

Library and track-type metadata and player mixer preferences use the Pinia
persistence plugin. GM mixes and track order load from the server before table
controls become available. They do not inherit browser-local mixer preferences.
Mix edits are debounced and flushed when leaving the table; saves are serialized
with ordering edits, and failures show an explicit retry action. Table metadata,
GM settings, and playback state are not persisted in browser storage.
Playback is keyed by file UUID within the active table and
cleared on table changes, so even the same library track cannot inherit another
table's playback. Local browser state is not authoritative for stored metadata.
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
with volume, playback-position, elapsed/total time, pause, and restart controls.
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
Playback is ephemeral browser state, not persisted in SQLite or recovered by
the server after every GM disconnect. Opening a table restores its saved mix
and layout with every track stopped at the beginning.

### Standalone PCM mixer

`internal/mixer` is an offline mixing core, not part of the HTTP, WebSocket,
or browser playback path. It accepts decoded, normalized stereo PCM frames at
one caller-selected sample rate. `Add` copies each clip and leaves it paused.
`Play`, `Pause`, `Seek`, `SetRegion`, `SetRepeating`, `SetGain`, and `Remove`
control tracks; `State` exposes their absolute frame position and requested
playing state. `Render` overwrites a caller-owned output buffer and advances
the mix by that many frames. No device, codec, network connection, clock,
timer, or goroutine is created. One owner must serialize renders and controls.

Regions use absolute frame offsets with an exclusive end; seeks may also land
exactly on the end, resolving completion or looping when playback renders.
A caller resolves an unset end to the decoded clip length
and an unset loop-back to the beginning before setting a region. Completion
stops non-repeating tracks and resets their position to the beginning.
Repeating tracks overlap the outgoing tail with an incoming cursor at the
loop-back point using complementary linear gains. The overlap length is
`min(2 * sampleRate, (end - loopBack) / 3, end - start)`, rounded down to
whole frames. The incoming cursor advances during the overlap and continues
from that position afterward, so an intro before the loop-back is not replayed.
Sub-frame overlaps become hard loops. A late seek shortens the overlap to
the remaining tail; seeking or changing a region cancels the existing overlap.

Repeating play/pause transitions use independent two-second linear envelopes.
Pause cancels loop overlap, lets the outgoing cursor advance while fading,
then freezes it; reaching the region end during fade-out stops and resets it
rather than starting another loop. Non-repeating play/pause is immediate.
`SetGain` takes an effective track/master/type product in `[0, 1]` and ramps it
over two seconds, including while paused. Interrupted ramps start at the
current interpolated gain. Duplicate play/pause, gain, repeat, and region
settings do not restart ramps or loops.
Tracks are summed in insertion order and only the final stereo output is
saturated to `[-1, 1]`; there is no normalization or peak limiter.

Tests use small known PCM arrays and exact sample positions to assert audible
output, completion, crossfade boundaries, interruption, and silence. Rendering
the same command timeline with single, irregular, or whole buffers produces
the same samples and track state, including empty renders.
The core stores complete decoded clips in memory (about 44 MiB per minute at
48 kHz with float64 stereo samples). File decoding/resampling, bounded-memory
source buffering, playback-state adapters, real-time scheduling, encoding,
and external audio delivery are outside this package.

### Experimental live server mixing

The `--experimental-mixer` / `EXPERIMENTAL_MIXER` server flag enables an
isolated `internal/livemixer` service inside the Go API process. It is disabled
by default. Browser HLS playback and player synchronization remain the normal
listening path; the experimental service does not replace them. Its rendered
audio is available to the optional Discord transport.

For an enabled service, the hub sends `mixerReady` to each authenticated GM
connection. The GM answers with `mixSync`: a full snapshot of playing tracks
and the current type-volume map. This happens on reconnect as well as initial
connection. Existing `syncTrack` messages feed partial play/pause, seek,
volume, repeating, and live playback-region changes into the mixer.
Their volume already includes the GM master multiplier; the service applies
the separately supplied type multiplier and caps effective gain at unity,
matching browser per-track gain clamping. The GM sends `mixVolumes` for live
type-volume changes. Master changes use the existing per-track volume updates.
Paused seeks and volumes are supplied by the full track state on resume.
Frontend partial updates coalesce by track ID, preserving independent fields
and updates to different tracks in the same debounce window. Immediate
exclusivity and region changes flush queued commands first.

The hub derives table identity and GM authorization from the authenticated
socket, never the command payload. Targeted player `syncAll` responses do not
mutate the mixer; untargeted full syncs replace it, including the empty snapshot
on leaving a table. Mixer-only commands are ignored when disabled and never
forwarded to players. Successful commands receive `mixerAck`; failures receive
`mixerError` and are logged with table and command context. The GM displays
errors without stopping browser playback. A successful reconnect snapshot
clears the error. Normal player broadcasts happen before mixer processing,
so warming a decoder does not delay that broadcast.

`internal/server` supplies the PCM source. It checks the track's association
with the socket's table, loads track/type metadata through the datastore, and
decodes the stored HLS playlist through FFmpeg to 48 kHz stereo float PCM.
Each track is decoded on its first command in a session and cached for
subsequent commands. Decoding has a ten-second deadline and a 64 MiB raw
output limit; errors and oversized clips fail explicitly. PCM samples are
clamped to unity to accommodate AAC decoding overshoot. Full clips and core
copies are retained in memory, so this is not a bounded-memory streaming
decoder.

Commands are serialized while a separate lock protects PCM rendering.
Validation and decoding precede mutation, so invalid patches and snapshots
do not partially alter a mix, and decoding does not hold the render lock.
The service renders 960-frame blocks every 20 ms with a cancellable loop.
The latest block's peak, cumulative frame count, command revision, and track
positions/target gains are available at the GM-only
`GET /api/v1/tables/{tableID}/mix` diagnostic endpoint. Disabled mixing or an
unknown table returns 404.

Rendered PCM blocks are published to internal subscriptions for Discord
streaming. Subscribing observes the existing timeline and never starts or
advances playback. Each subscriber has a two-block buffer and is closed when
it falls behind rather than delaying rendering. Closing the table session or
stopping the renderer closes its subscriptions. GM browser audio continues
through ordinary HLS playback; the server mix is audible through Discord.

Track disassociation removes the track from its table's mix; library deletion
removes it from all mixes. Table deletion and loss of its last GM connection
discard the session. Closing a session does not wait for decoding, and an
in-flight decode cannot recreate it. Multiple GMs share a table's mixer with
last-accepted-command semantics. Mixer state is ephemeral, not stored in SQLite.
Domain tests assert rendered samples after partial updates and snapshots,
table isolation, rejection without mutation, and nonblocking rendering/closure.
The opt-in full-app browser journey drives real GM controls and checks the
resulting rendered PCM telemetry, seeks, loop-back transitions, and cleanup.

### Optional Discord voice streaming

Configuring `DISCORD_BOT_TOKEN` enables Discord setup and the live mixer.
With no token, the UI hides all Discord controls.
The server uses its configured bot identity; the GM's existing app login
authorizes controls. This path does not link a Discord user account or use
OAuth client secrets. `DISCORD_WORKER` selects the Node worker path and defaults
to `discord/stream.mjs` relative to the server working directory. The Docker
image includes Node and the locked worker dependencies, including a
platform-matched native DAVE library.

Each table's Currently playing sidebar (or mobile drawer) has a Connect to
Discord button. Its dialog loads `GET /api/v1/discord/setup`, which obtains
the application ID and the bot's installed servers from Discord. An
installation link opens Discord's authorization page with only bot scope and
View Channel, Connect, and Speak permissions. The user chooses an eligible
server in Discord, approves installation, returns to the app, and refreshes
the server list. No callback URL or OAuth client secret is needed. The app's
single configured GM account is authorized to select any server that has
installed this configured bot.

The dialog prompts for a server and a regular voice channel. The GM-only
`GET /api/v1/discord/servers/{guildID}/channels` endpoint lists channels.
`POST /api/v1/tables/{tableID}/discord` accepts the selected server/channel
IDs, verifies their association through Discord, connects, and saves the
destination and display names in that table's SQLite `discord_destination`
column. A failed save cancels the connection. An omitted request body
reconnects the saved destination. `GET` returns state and the saved
destination; `DELETE` disconnects only this table without forgetting it.
The composable prevents duplicate commands, discards stale status/channel
responses, and displays failures.

`internal/discord` reserves one voice connection per Discord server. Tables
can stream concurrently in different servers, but another table in the same
server receives a conflict and cannot take over the connection. Each table
owns a separate Node worker. Voice startup does not hold the rendering lock;
the PCM subscription is created only once voice is ready, avoiding a full
subscriber queue during the handshake. PCM travels over a child-process pipe
as 48 kHz stereo float samples. A blocked write has a two-second deadline and
cancels the worker rather than stalling indefinitely.

The worker uses `discord.js` and `@discordjs/voice` with DAVE encryption
enabled and only Guilds/GuildVoiceStates intents. It validates the configured
channel's server, regular-voice type, and effective bot permissions before
joining. `PCMTransform` validates stereo frames across arbitrary pipe chunks
and converts float samples to signed 16-bit PCM. The voice library encodes
20 ms Opus frames and sends them over Discord's encrypted voice connection.
The stream bot joins self-deafened and does not receive other participants'
audio. Bot credentials reach only the worker environment, not process
arguments, browser payloads, or logs.

Disconnecting, closing the table mix, loss of its last GM, or renderer shutdown
ends the subscription and voice worker. Worker failures and slow PCM consumers
become observable error states. Startup has a
20-second deadline, and worker cancellation sends SIGTERM with a bounded
forced-exit fallback. The first successful mixer sync of a GM table session
attempts the saved destination once. Repeated syncs and status polling do not
reconnect a manually disconnected bot. Leaving a table or closing its last GM
tab cancels pending reconnection and disconnects the worker; returning creates
a new session and attempts reconnection. Multiple GM tabs keep the bot active
until the last one closes. A lost network connection is detected by the
WebSocket heartbeat. Reconnection conflicts and failures are visible and
require an explicit retry or destination change within that session.

The opt-in live Discord suite in `e2e/` is written in Go. It builds and starts
the real server, drives the GM API and mixer WebSocket, and joins a second bot
through `disgo` with pure-Go DAVE. The listener subscribes only to the
streamer, decrypts and decodes its received Opus through FFmpeg, and checks
both generated tones, live volume changes, pause/resume, and departure. This
verifies delivery through Discord rather than trusting sender-side telemetry.
The listener defers DAVE group commits until its session is ready, so the
streamer's DAVE implementation always admits it by Welcome; the two libraries
otherwise race to commit and reject each other's Welcome. Audio receiving is
supported by the selected library but is not a stable, documented Discord API
contract; live tests are therefore separate from deterministic transport
tests. Playwright journeys separately check installation-link presentation,
server/channel selection, disconnect, and automatic reconnect on return.

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
