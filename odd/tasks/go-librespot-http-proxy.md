# go-librespot HTTP proxy for access point traffic

## Objective

Make Cadence's pinned go-librespot reach Spotify access points on networks
where only an HTTP CONNECT proxy is available.

## Problem

The current corporate network has no direct egress and no recursive DNS.
go-librespot dials access points through `golang.org/x/net/proxy`, which only
reads `ALL_PROXY` and only understands `socks5://`. The environment provides
`ALL_PROXY=http://...`, so the dialer silently falls back to a direct
connection and fails with `Temporary failure in name resolution`.

The proxy allows `CONNECT` to access point hosts on 443 and denies 4070/80.
The engine must therefore use 443 and speak HTTP CONNECT.

## Scope

- Add an HTTP CONNECT dialer for access point connections in the pinned
  go-librespot patch set.
- Prefer firewall-friendly access point ports (443 first) in the generated
  engine configuration.
- Keep existing SOCKS5 proxy behavior and direct connections when no proxy is
  configured.
- Do not change pairing, playback, or UI behavior.

## Constraints

- The corporate proxy is `http://10.100.254.240:3128`; only TCP 443 CONNECT to
  access point hosts is allowed.
- `*.spotify.com` must not be added to `NO_PROXY`.
- The patch must apply to `go-librespot v0.10.6-0.20261102145552-522adcae8d65`.
- Technical artifacts are in English.

## Tasks

- [x] T1 Add `build/patches/go-librespot-http-proxy.patch` with `ap/proxy.go`
      and the `ap/ap.go` dial change.
- [x] T2 Apply the new patch from `scripts/build-go-librespot.sh` and add the
      rebuild dependency in `Taskfile.yml`.
- [x] T3 Add `prefer_firewall_friendly_ports: true` to the generated
      configuration in `internal/golibrespot/runtime.go`.
- [x] T4 Add a unit test asserting the generated configuration prefers
      firewall-friendly ports.
- [x] T5 Build the engine and verify the access point handshake through the
      proxy.

## Authorized scope

User explicitly requested implementation on 2026-11-04.

## Acceptance criteria

- `task audio:build` produces `bin/go-librespot` from the patched source.
- The patched engine dials an access point on 443 through the HTTP proxy; a
  fake-token handshake returns `BadCredentials`, not a dial or DNS error.
- `go test ./...` and `go vet ./...` pass.
- The generated configuration contains `prefer_firewall_friendly_ports: true`.

## Checks

- TDD: off (no strict TDD configuration in the project).
- Test runner: `go test ./...`.
- Build: `task audio:build`.
- Vet: `go vet ./...`.
- Handshake: temporary test in `build/audio/go-librespot-src/ap` using
  `ConnectSpotifyToken` with a fake token.

## Route

- T1-T5: delegated writer (writer trigger: two or more non-trivial files).
- Verification: writer runs the commands; parent spot-checks one command.

## Delivery

- Strategy: `ask-on-risk` (default), forecast around 170 authored lines,
  under the 400-line heuristic. No pull request requested.

## Progress

- [x] Implementation committed as `9fe53e4ad7b76bfb8a0ab0440e2f656971b38b15` on
      `feat/go-librespot-http-proxy`.
- [x] Engine built to `bin/go-librespot` from the patched source.
- [x] Access point handshake through the corporate proxy reached
      authentication.

## Verification evidence

- `go test ./internal/golibrespot -run TestWriteConfigPrefersFirewallFriendlyPorts -v`: PASS.
- `./scripts/build-go-librespot.sh`: PASS; both patches applied and
  `bin/go-librespot` produced.
- `cd build/audio/go-librespot-src && go test ./ap -run TestAPHandshakeViaProxy -v`:
  PASS (5.92s), `BadCredentials` for `ap-guc3.spotify.com:443` through
  `10.100.254.240:3128`.
- `go test ./...` and `go vet ./...`: blocked only by the pre-existing missing
  `frontend/dist` embed in the root package; all library packages pass. The
  failure is not caused by this change.
- `task audio:build`: not runnable because the `task` binary is absent in the
  implementation environment; the script it invokes passed.

## Next step

- Rebuild or repackage Cadence so the app uses the patched
  `bin/go-librespot`, then pair on the corporate network.
- Re-run `go test ./...` after a frontend build to cover the root package.

## Engram mirror

- Pending: Engram has no registered context for `spotify_winamp`.
