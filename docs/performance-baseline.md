# Performance baseline

- **Status:** Procedure defined in Phase 0; reference-machine numbers pending.
- **Plan:** [`../plan/cadence-v2.md`](../plan/cadence-v2.md) §5 and §8.
- **Diagnostics flag:** `./bin/cadence --diagnostics`.

## Purpose

Cadence v2 changes how state reaches the UI and how artwork is fetched. Those
changes need a before/after comparison based on measurements, not impressions.
This document defines:

1. how to capture the runtime baseline with the built-in diagnostics endpoint;
2. the static baseline that can be derived from the code today;
3. the Phase 4 target gates that the measured baseline must satisfy.

The runtime numbers **must be captured on the reference machine with the
application running**. The table below is intentionally empty until that
capture happens; the static estimate is not a substitute for a measurement.

## Diagnostics endpoint

`--diagnostics` is off by default. When enabled, Cadence:

- keeps a `diagnostics.Recorder` for engine requests, artwork cache activity and
  player event payloads;
- wraps the engine HTTP client so every request is counted by `METHOD /path`
  (query strings are excluded to keep cardinality bounded);
- starts an ephemeral listener on `127.0.0.1:0` and logs its URL, for example
  `diagnostics available at http://127.0.0.1:43127/debug/stats (pprof at /debug/pprof/)`.

With the flag disabled no listener starts and the recorder stays `nil`. There is
no transport wrapper and no event JSON marshalling; the artwork hot path pays
one atomic load and returns.

### `GET /debug/stats`

Returns JSON with `Cache-Control: no-store`:

| Field | Meaning |
|---|---|
| `uptime_seconds` | Seconds since the recorder was created |
| `goroutines` | Current Go goroutine count |
| `engine` | Map of `METHOD /path` to `requests`, `errors`, `bytes_in`, `bytes_out`, `latency_ms_total` |
| `engine_total` | Aggregate of all engine routes |
| `artwork` | Map of `hit`, `disk_hit`, `miss`, `missing`, `error`, `served` to `count` and `bytes` |
| `events` | Map of event name to `count` and `bytes` |

`bytes_in` is the request body size; `bytes_out` is the response body bytes
actually read. `errors` counts transport failures (`status == 0`) and HTTP
status codes `>= 400`.

`artwork.missing` counts requests for an unknown or LRU-evicted key (the paths
that return 404). `artwork.served` counts only `GET` body writes that completed
without an error; aborted or partial writes are not counted, so delivered-image
stats are not inflated.

The pprof handlers are registered at `/debug/pprof/`, `/debug/pprof/cmdline`,
`/debug/pprof/profile`, `/debug/pprof/symbol` and `/debug/pprof/trace`.

## Procedure

### 1. Build and start

```bash
task build                 # builds frontend and bin/cadence
task audio:build           # once, if bin/go-librespot is missing
./bin/cadence --diagnostics
```

Copy the URL printed at startup. The examples below use
`http://127.0.0.1:PORT/debug/stats`.

### 2. Define the idle state

Use an authenticated account, pause playback, leave the window open and do not
open the queue, browse or scroll. Let the app settle for 60 seconds before
capturing. Record the window size and whether a custom skin is active.

### 3. Capture the request rate and event payload

Capture two snapshots 60 seconds apart:

```bash
curl -sS http://127.0.0.1:PORT/debug/stats | tee /tmp/cadence-stats-1.json
sleep 60
curl -sS http://127.0.0.1:PORT/debug/stats | tee /tmp/cadence-stats-2.json
```

Compute the idle request rate from the delta:

```bash
jq -n --slurpfile a /tmp/cadence-stats-1.json --slurpfile b /tmp/cadence-stats-2.json \
  '($b[0].engine_total.requests - $a[0].engine_total.requests) / 60'
```

The average `player:state` payload is:

```bash
jq '.events["player:state"] | .bytes / .count' /tmp/cadence-stats-2.json
```

Also record the per-route breakdown (`.engine`) so the source of idle traffic is
visible.

### 4. Capture idle CPU and RSS

```bash
pid=$(pgrep -n cadence)
pidstat -p "$pid" 1 60          # or: top -b -d 1 -n 60 -p "$pid"
ps -o rss= -p "$pid"             # sample every 30 s for 10 minutes
```

Record the mean CPU percentage over the 60-second sample and the RSS samples
over 10 minutes. RSS is considered stable when it has no sustained upward trend
after caches warm up.

### 5. Capture a CPU profile

```bash
curl -sS -o /tmp/cadence-cpu.pprof \
  "http://127.0.0.1:PORT/debug/pprof/profile?seconds=30"
go tool pprof -top /tmp/cadence-cpu.pprof
```

Capture one profile at idle and one while scrolling an 800-item list. Keep the
profiles alongside the completed baseline table.

### 6. Measure the 800-row scroll

Open a playlist or album with 800 rows, use the webview DevTools performance
panel (or a manual frame-timing pass on the reference machine) and record
whether scrolling is smooth and whether frames are dropped. This is the
qualitative gate from the plan.

## Static baseline derived from the code

These figures come from reading the current code at the Phase 0 commit. They
explain where idle traffic comes from; they do not replace runtime capture.

| Source | Code path | Static rate |
|---|---|---|
| Player ticker | `internal/wailsapi/player/service.go` emits every 500 ms; each `Provider.State()` calls `GET /status` | 2.0 req/s |
| Frontend audio poll | `frontend/src/lib/music.svelte.ts` calls `AudioService.GetState()` every 2 s | 0.5 req/s |
| Context fetch | `internal/playback/engine_provider.go` fetches `/context/tracks` only while the context is not `Ready`; once a successful response reports `Ready`, that URI is cached indefinitely. The 5 s window gates warmup and retries only. | 0.2 req/s while warming up or retrying; 0 in steady state |
| **Steady idle total** | | **≈2.5 req/s** |
| **Warmup total** | | **≈2.7 req/s** |

The plan estimated **≈3 req/s** of idle traffic. The code-derived steady-state
rate is ≈2.5 req/s (2.0 ticker + 0.5 frontend); context warmup or retry adds
≈0.2 req/s. The measured `/debug/stats` value remains authoritative.

Other static properties that affect the baseline:

- Every 500 ms tick emits the full `PlayerState`, including up to 50 queue items
  (`internal/wailsapi/player/service.go`, queue cap in `internal/playback/engine_provider.go`).
  Payload size grows with the queue; plan §5 estimates 15-30 KB per tick at 50
  items.
- The artwork cache is memory-only: 96 entries, 48 MiB total, 3 MiB per image,
  with no size variants, no single-flight and no disk cache
  (`internal/artwork/cache.go`).
- Context lists can reach 800 rows (`metadata.max_tracks: 800` in
  `internal/engine/supervisor.go`) and are rendered without virtualization
  (`frontend/src/lib/MusicBrowser.svelte`).

## Baseline table (reference machine)

Reference machine:

| Item | Value |
|---|---|
| Machine / CPU / RAM | pending |
| OS / kernel | pending |
| Cadence commit | `b5e326c` (Phase 3 close) |
| Engine pin | `v0.10.6-0.20261104085532-16b65bb4cd2b` |
| Build command and build tags | `task build` |
| Window size / active skin | pending |
| Captured at | 2026-11-04, idle with an authenticated session |

Measurements:

| Metric | How measured | Baseline | Phase 4 target |
|---|---|---|---|
| Idle engine request rate | `engine_total.requests` delta over 60 s | **2.5 req/s** (150 requests in 60 s) | ≤ 0.2 req/s |
| Engine response bytes | `engine_total.bytes_out` delta | 396 B/request (59,400 B in 60 s) | — |
| Engine latency | `engine_total.latency_ms_total` delta | 0.87 ms/request (130.5 ms in 60 s) | — |
| `player:state` payload | `events["player:state"].bytes / count` | pending (not in the captured sample) | ≤ 4 KB per change |
| Idle CPU | mean over a 60 s `pidstat`/`top` sample | pending | < 2% |
| RSS stability | RSS every 30 s for 10 minutes | pending | stable over 10 minutes |
| 800-row scroll | DevTools performance / manual pass | pending | smooth, no dropped frames |

Captured `engine_total` samples (60 s apart):

```json
{"requests":396,"errors":1,"bytes_in":0,"bytes_out":152881,"latency_ms_total":288.42264099999994}
{"requests":546,"errors":1,"bytes_in":0,"bytes_out":212281,"latency_ms_total":418.9707769999999}
```

The single error is unchanged between samples and is a startup error, not idle
traffic. The measured 2.5 req/s matches the static estimate exactly: 2.0 from
the 500 ms player ticker plus 0.5 from the 2 s frontend `GetState` poll. The
`player:state` payload, CPU, RSS and scroll rows still need capture before the
Phase 4 after-comparison is complete; the request-rate gate is the primary one.

## Phase 4 gates

The event-driven state and artwork work is accepted only when all of these hold
on the reference machine:

- Idle engine request rate **≤ 0.2 req/s**.
- Wails event payload **≤ 4 KB per change**, and no event while paused and
  unchanged.
- Idle CPU **< 2%**.
- RSS stable over a **10-minute** run.
- An **800-item list scrolls smoothly** without dropped frames.

## Limitations

- Counters are per process and reset on restart.
- Latency is recorded when the response body reaches EOF or is closed, so it
  includes body consumption time.
- `served` counts response bytes only for successful `GET` artwork writes;
  aborted or partial writes are not counted, and `HEAD` requests do not serve a
  body.
- `missing` counts requests for unknown or evicted artwork keys; it is separate
  from `error`, which counts remote fetch failures.
- The diagnostics listener is loopback-only and exists only while
  `--diagnostics` is set. It is a development tool, not a product surface.
