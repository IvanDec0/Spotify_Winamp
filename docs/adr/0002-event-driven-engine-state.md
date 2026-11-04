# ADR 0002 — Event-driven engine state

- **Status:** Proposed
- **Date:** 2026-11-04
- **Phase:** 4 (planned)
- **Source:** [`../../plan/cadence-v2.md`](../../plan/cadence-v2.md) §4.3 and §5

## Context

Cadence currently polls the engine about three times per second and re-sends the
full player state, queue included, on every 500 ms tick. go-librespot already
exposes a WebSocket stream at `GET /events` that the app never opens. The
polling design is the main source of idle traffic, CPU wakeups and large IPC
payloads.

## Decision

Make the engine event stream the primary state source:

- `GET /events` drives playback, queue and engine state.
- A 15-second `GET /status` reconciliation exists only to repair events missed
  after a reconnect.
- `GET /context/tracks` is fetched on demand when an entity opens; it is never
  fetched on a timer.
- Bursts are coalesced with a 50 ms debounce and a snapshot is emitted only when
  it changes.
- Snapshots are revisioned (`Revision`, `QueueRev`); the frontend drops stale
  payloads.
- Position travels as `position_ms` plus `captured_at`; the frontend interpolates
  locally at 10 Hz instead of asking the engine.
- Queue state is fetched when the queue panel opens and kept current by queue
  events.

## Consequences

- Phase 4 removes the 500 ms player ticker and the 2 s frontend poll; idle
  traffic and event payload size drop substantially.
- The app needs reconnect/backoff handling, a snapshot state machine and a
  revision-aware frontend event handler.
- There is no event while paused and unchanged, by design.
- The Wails event contract changes; generated bindings and the frontend stores
  must be updated in the same slice.
- The 15 s reconciliation is the safety net for missed events, not a second
  polling loop.
