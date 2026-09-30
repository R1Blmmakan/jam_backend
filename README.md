# BitChord: Listen Together

The party server behind Listen Together creates a six-character code so up to five devices can listen in sync. Any member can control playback by default.

The backend is built in Go with a WebSocket connection per device. It requires no database and stores all active sessions in memory.

```bash
go run .
go test -v ./...
```

## How devices stay in sync

The server synchronizes devices using time anchors rather than broadcasting immediate play commands. If a message arrives 40 ms late on one phone and 300 ms late on another, an immediate play command starts them out of sync.

Instead, the server tracks a track position alongside the server timestamp when that position was recorded:

```jsonc
{
  "positionMs": 42000,          // playback position in milliseconds
  "anchorMs": 1757630001234,    // server timestamp when positionMs was valid
  "isPlaying": true
}
```

Each device calculates its local offset from the server clock and determines its exact playhead locally:

```
serverNow   = deviceNow + clockOffset
playhead    = positionMs + max(0, serverNow - anchorMs)      // while playing
playhead    = positionMs                                      // while paused
```

When a network delay occurs, the anchor timestamp reflects that delay and places the client at the correct position. Four mechanisms maintain this accuracy:

1. **Measured clock offsets.** Each device sends `{"type":"ping","clientMs":...}`. The server echoes the client timestamp untouched alongside its own server timestamp. The offset is `serverMs - (t0 + t1) / 2`, bounding error to half the round-trip latency. Keeping the sample with the lowest round-trip time maintains accuracy over cellular networks. Clients can also query `GET /api/time` for an initial estimate before opening a WebSocket.
2. **Monotonic server time.** `clock/clock.go` reads the system wall clock at boot and advances monotonically. Host NTP adjustments cannot shift the anchor backward during active sessions.
3. **Scheduled playback lead.** Play and seek actions anchor `JAM_PLAY_LEAD_MS` (default 350 ms) into the future. Devices target a shared timestamp and buffer audio beforehand.
4. **Heartbeat state broadcasts.** The server broadcasts the current state every `JAM_STATE_HEARTBEAT_MS` (default 5 s) without waiting for client requests. This corrects dropped frames, device sleep wakeups, and wandering offsets.

The server state serves as the source of truth. Device reports (`{"type":"report"}`) are logged for diagnostic monitoring and do not slow down other members.

## Identity and access

Joining or hosting a session requires `userId`, `deviceId`, and `displayName` from the user profile.

The server does not authenticate Google credentials directly. Verifying each user against external authentication APIs on every join would introduce latency and invite rate limiting. Instead, the server issues a random, unguessable session token upon joining. Members must supply this token with subsequent requests.

## Capacity and lifecycle

- **Device limits.** Rooms permit five active devices (`JAM_MAX_MEMBERS`), tracked per device rather than per account.
- **Slot recovery.** Reinstalling the app or reconnecting after a network drop reclaims the existing device slot instead of consuming a new one.
- **Connection grace periods.** When a socket disconnects, the membership remains reserved for `JAM_DISCONNECT_GRACE_MS` (default 45 s) to handle screen locks or transient network switches.
- **Host permissions.** By default, any participant can control playback. When the host leaves, the host role passes to the next connected member. If the host enables `hostOnlyControl`, the server rejects playback and queue modifications from other participants.
- **Room cleanup.** Rooms are purged after remaining empty for `JAM_EMPTY_PARTY_TTL_MS`, and expire unconditionally after `JAM_PARTY_MAX_AGE_MS`.

## API

Payloads and WebSocket messages use camelCase to align with the Android client.

### REST

| Endpoint | Description |
|---|---|
| `GET /healthz` | Health check endpoint returning `{"ok": true, "serverMs": ...}`. |
| `GET /api/time` | Returns `{"serverMs": ...}` for initial clock synchronization before connecting to WebSockets. |
| `POST /api/parties` | Creates a party and assigns the creator as host. Request body requires `userId`, `deviceId`, and `displayName` (optional `avatarUrl`). Returns `201` with `code`, `token`, and party details. Rate-limited by IP. |
| `POST /api/parties/{code}/join` | Joins an existing party using the same body format. Returns `404` for unknown codes, `409` when full, and `422` for invalid payloads. Normalizes ambiguous characters (`O`/`0`, `I`/`1`, `L`/`1`). |
| `GET /api/parties/{code}/preview` | Returns member display names and room status without joining or requiring tokens. |
| `GET /api/parties/{code}` | Fetches full party snapshot. Requires `Authorization: Bearer <token>`. |
| `POST /api/parties/{code}/leave` | Forfeits the member slot. Requires `Authorization: Bearer <token>`. |
| `GET /invite/{code}` | Web landing page for shared links. Redirects mobile browsers into BitChord via intent URLs. |

### WebSocket: `/ws/parties/{code}`

Pass the session token in the handshake header: `Authorization: Bearer <token>`. Query parameter tokens are rejected to prevent token exposure in access logs.

Client to server:

```jsonc
{"type": "ping",    "clientMs": 1757630000000}
{"type": "sync"}                                   // requests current playback state
{"type": "syncQueue"}                              // requests current queue
{"type": "report",  "positionMs": 42210, "isPlaying": true}
{"type": "control", "action": "play",        "positionMs": 42000}
{"type": "control", "action": "pause",       "positionMs": 42000}   // positionMs optional
{"type": "control", "action": "seek",        "positionMs": 90000}   // positionMs required
{"type": "control", "action": "setTrack",    "track": {...}, "positionMs": 0, "isPlaying": true}
{"type": "control", "action": "setQueue",    "queue": [{...}], "queueIndex": 0}
{"type": "control", "action": "queueAdd",    "tracks": [{...}], "playNext": false}
{"type": "control", "action": "queueRemove", "videoId": "..."}
{"type": "control", "action": "queueClear"}
{"type": "control", "action": "queueMove",   "fromIndex": 2, "toIndex": 5, "videoId": "..."}
{"type": "control", "action": "next"}
{"type": "control", "action": "previous"}
{"type": "control", "action": "setHostOnlyControl", "enabled": true}   // host only
```

`setMaxMembers`, `kick`, and `setHostOnlyControl` require host privileges and return `403 host_only` to non-host callers. When `hostOnlyControl` is active, playback and queue actions are also restricted to the host.

A `bye` frame provides a `reason`: `left` when a member departs, or `kicked` when removed by the host.

A `track` payload contains `{videoId, title, artist, thumbnailUrl, durationMs, fromAutoplay}`. Stream URLs, audio decoders, and bitrate settings remain local to each device.

Server to client:

```jsonc
{"type": "welcome", "you": {...}, "party": {...}, "serverMs": ...}
{"type": "pong",    "clientMs": ..., "serverMs": ...}
{"type": "state",   "playback": {...}, "serverMs": ...}
{"type": "queue",   "queue": {"seq": 3, "index": 1, "items": [{...}]}, "serverMs": ...}
{"type": "members", "members": [{...}], "maxMembers": 5, "serverMs": ...}
{"type": "error",   "error": "rate_limited", "message": "..."}
{"type": "bye",     "reason": "left"}
```

State changes increment `playback.seq`. Clients ignore state updates where `seq` is less than or equal to their currently applied sequence number.

### Queue updates

The `state` frame excludes track lists to reduce payload size on cellular data. It includes only `queueSeq`, `queueIndex`, and `queueLength`.

The complete queue is transmitted:
- Within the initial snapshot (`welcome` frame and `GET /api/parties/{code}`).
- On queue modifications, where a `queue` frame precedes the `state` broadcast.
- On request, when a client receives a `queueSeq` mismatch and requests resynchronization via `syncQueue`.

Queue constraints:
- **Upcoming song limit.** Rooms hold up to 25 upcoming songs (`JAM_MAX_UPCOMING_QUEUE`). Additional songs are rejected with `queue_full`.
- **Deltas.** Clients transmit specific actions (`queueAdd`, `queueRemove`, `queueClear`, `queueMove`) rather than uploading entire playlists.
- **Identifier resolution.** `queueMove` accepts an optional `videoId` alongside numerical indices to prevent reordering conflicts under high latency.

## Deployment

### Local development

Run natively with Go 1.22+:

```bash
go run .
```

### Docker Compose (ZimaOS / Homelab / VPS)

The included multi-stage `Dockerfile` compiles an unprivileged Alpine container (~15 MB):

```bash
docker compose up -d --build
```

Resource limits in `docker-compose.yml` allocate up to 1.0 CPU and 256 MB RAM, with a built-in health check polling `/healthz`.

### Cloudflare Tunnel

When exposing the backend through Cloudflare Tunnel (`cloudflared`) to a custom domain:

1. Configure the tunnel ingress to point to the local HTTP service (`http://localhost:8080` or container hostname).
2. Set `TRUST_PROXY=true` in `.env` or `docker-compose.yml`. The server extracts the real client IP from `CF-Connecting-IP` for rate limiting.
3. In Cloudflare Tunnel settings, ensure HTTP/2 Origin is disabled so WebSocket upgrade handshakes proceed over HTTP/1.1 without buffering.

### Oracle Cloud (Always Free VM)

Ubuntu 24.04 setup using Caddy for HTTPS and WebSockets:

1. Launch an Always Free `VM.Standard.E2.1.Micro` instance on Ubuntu 24.04.
2. Permit ingress traffic on ports 80 and 443 in the Oracle VCN Security List.
3. Point your DNS A record to the instance public IP.
4. Execute the deployment script:

```sh
sudo DOMAIN=api.bitchord.example.com bash deploy/setup.sh
```

Inspect service output with `journalctl -u bitchord-jam` and `journalctl -u caddy`.

### Render

Deploy as a Web Service:

- **Runtime:** Go
- **Build Command:** `go build -o server .`
- **Start Command:** `./server`
- **Health Check Path:** `/healthz`
- **Instances:** 1 (in-memory state requires a single instance)

Point the Android client to your deployed instance:

```properties
LISTEN_TOGETHER_SERVER=https://jam.example.com
```

## Environment configuration

All settings are optional and provide defaults in `config/config.go`.

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Bind port for HTTP and WebSocket listeners. |
| `TRUST_PROXY` | `true` | Reads real client IP from `CF-Connecting-IP` or `X-Forwarded-For`. |
| `JAM_TCP_NODELAY` | `true` | Disables Nagle algorithm on socket connections to minimize latency. |
| `JAM_WS_READ_BUFFER_SIZE` | `2048` | Read buffer size in bytes for WebSocket connections. |
| `JAM_WS_WRITE_BUFFER_SIZE` | `2048` | Write buffer size in bytes for WebSocket connections. |
| `JAM_MAX_MEMBERS` | `5` | Maximum connected devices per party. |
| `JAM_STATE_HEARTBEAT_MS` | `5000` | State broadcast interval in milliseconds. |
| `JAM_PLAY_LEAD_MS` | `350` | Playhead lead buffer in milliseconds. |
| `JAM_MAX_UPCOMING_QUEUE` | `25` | Maximum upcoming queue capacity. |
| `JAM_MAX_QUEUE_LENGTH` | `26` | Total queue capacity limit (`1 + JAM_MAX_UPCOMING_QUEUE`). |
| `JAM_DISCONNECT_GRACE_MS` | `45000` | Retention window for disconnected members in milliseconds. |
| `JAM_EMPTY_PARTY_TTL_MS` | `120000` | Expiration window for empty rooms in milliseconds. |
| `JAM_PARTY_MAX_AGE_MS` | `43200000` | Maximum room lifetime in milliseconds (12 hours). |
| `JAM_CONTROL_RATE_PER_SECOND` | `25` | Per-member control action rate limit. |
| `JAM_FRAME_RATE_PER_SECOND` | `30` | Per-member WebSocket frame rate limit. |
| `JAM_MAX_PARTIES` | `100` | Maximum concurrent active rooms in memory. |
| `JAM_CREATE_RATE_PER_MINUTE` | `2` | Room creation rate limit per client IP. |
| `JAM_RATE_LIMIT_MAX_ENTRIES` | `10000` | Capacity limit for tracked IP rate limit entries. |
| `JAM_REQUEST_MAX_BYTES` | `16384` | Maximum REST payload size in bytes. |
| `JAM_WEBSOCKET_MAX_BYTES` | `16384` | Maximum incoming WebSocket message size in bytes. |
| `JAM_CONNECTION_IDLE_MS` | `900000` | Idle timeout for inactive WebSockets (15 minutes). |
| `JAM_ALLOWED_ORIGINS` | *(none)* | Comma-separated browser Origin allowlist. Mobile clients do not send Origin. |

## Project layout

```
clock/clock.go        Monotonic clock advancing steadily across NTP updates
codes/codes.go        Six-character room code generation and character normalization
config/config.go      Environment variables and network tuning defaults
deploy/               Caddyfile, systemd service units, and host setup scripts
hub/hub.go            Concurrent WebSocket connection hub and pre-serialized broadcaster
party/party.go        Room state, member allocations, queue mutations, and host privileges
protocol/protocol.go  Message schemas, action constants, and payload validators
main.go               Router, rate limiting, panic recovery, and server lifecycle
main_test.go          REST endpoint and WebSocket flow integration tests
party/party_test.go   Room capacity and queue synchronization unit tests
tools/jam_probe.py    Python integration probe simulating multiple listening devices
```
