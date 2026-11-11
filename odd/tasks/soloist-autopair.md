# Soloist automatic account pairing (no Spotify app)

## Objective

Sign Soloist in with the same Spotify account that Cadence's go-librespot
engine already uses, without asking the user to open the Spotify app or select
the `Cadence` device on another phone/computer.

## Problem

The API key authenticates Soloist against Spotify's audio service; it does not
sign a Spotify account in. Soloist is a Spotify Connect device, and the
documented setup asks the user to pick the device in the Spotify app. That is
both a second app and a second device, which is unacceptable for a desktop
player.

The previous fallback (open Spotify and detect the pick) is a one-tap flow but
still requires Spotify to be installed somewhere. This feature removes that
requirement when Cadence's library engine is already paired.

## Evidence

- The go-librespot engine persists `credentials.username` and
  `credentials.data` in `<config>/audio/state.json` (`0600`).
- `session.StoredCredentials()` returns
  `Accesspoint.StoredCredentials()` = `APWelcome.ReusableAuthCredentials`,
  which is exactly the payload accepted by the
  `AUTHENTICATION_STORED_SPOTIFY_CREDENTIALS` auth type.
- Soloist exposes a Spotify Connect ZeroConf endpoint. Its log reports
  `Started ZeroConf service on port <port> path /zc`.
- The ZeroConf `addUser` request carries a form with `userName`, `blob`,
  `clientKey` and `deviceName`. The blob is an outer DH/AES layer around a
  base64 inner blob; the inner blob is AES-ECB encrypted with a key derived
  from `sha1(deviceID)`, the username and PBKDF2-SHA1(256 iterations), then
  tail-scrambled. `ap.ConnectBlob` is the reference parser.
- A standalone spike reimplemented the client side and round-tripped a
  260-byte reusable credential through the server parser:
  `authType=1 dataLen=260 equal=true`.
- The reference implementation's comment says "over ZeroConf that did not
  work"; this feature is a second, tested attempt with the actual reusable
  credentials and the exact server parser.

## Scope

In:

- `engine.Supervisor.StoredCredentials()` reads the paired library account's
  username and base64 credentials from `state.json`.
- `internal/soloist/pair.go`: a ZeroConf client that builds the inner and
  outer blobs and calls `getInfo` + `addUser` on `127.0.0.1`.
- `soloist.Supervisor`: after starting, if Soloist is not logged in, discover
  the ZeroConf port/path, send the library credentials once, and wait for
  `auth_state.logged_in=true`.
- Fallback to the existing one-tap pairing state when any step fails: no
  library credentials, no ZeroConf port, rejected credentials, or a protocol
  mismatch.
- Tests for the crypto round trip, a fake ZeroConf HTTP server, the engine
  state reader, and the supervisor auto-pair path.
- Docs: automatic pairing when the library is paired; one-tap fallback only.

Out:

- Reading Spotify passwords or asking the user for them.
- Scraping the dashboard or the Spotify app.
- Sharing credentials with third parties or logging them.
- Using a different account than the library engine's account.

## Decisions

- **Reuse the library account.** The engine already authenticated that account;
  Soloist should use the same one. The API key must be generated with that
  account. This is the normal Cadence setup.
- **Loopback only.** The ZeroConf request goes to `127.0.0.1:<port>`; the
  credentials never leave the machine and are encrypted with Soloist's
  ephemeral public key.
- **Best effort with fallback.** Soloist's ZeroConf behavior is not documented.
  If the port, path, auth type or response differs, the app falls back to the
  pairing state and the one-tap flow. No boot loop, no dead player.
- **No new dependencies.** The DH group and PBKDF2 are small; use
  `golang.org/x/crypto/pbkdf2` only if it is already in the module graph, or
  implement PBKDF2-HMAC-SHA1 locally (it is ~30 lines).
- **Port discovery:** prefer the Soloist log line
  `Started ZeroConf service on port <port> path <path>`; fall back to
  `ss -ltnp` filtered by the child PID and probe both `/zc` and `/` with
  `getInfo`.

## Tasks

### Phase A — Credentials and pairing client

- [ ] A1 `engine.Supervisor.StoredCredentials()`: read
      `<ConfigDir>/state.json`, parse `credentials.username` and
      `credentials.data`, base64-decode the data, and return a clear error when
      the file/account is missing. Never log the data.
- [ ] A2 `internal/soloist/pair.go`:
  - `PairWithCredentials(ctx, port int, path, deviceName, username string, credentials []byte) error`.
  - `getInfo` (device ID + public key), build the inner blob
    (`authType=1`, username, device ID), outer DH/AES blob, `addUser`.
  - Check the HTTP status and the JSON `status == 101`.
  - A `probeZeroConf(ctx, port int) (path string, err error)` helper that tries
    `/zc` then `/`.
- [ ] A3 Tests: crypto round trip against a copied server parser; fake
      `httptest` ZeroConf server for `getInfo`/`addUser`; invalid public key,
      checksum mismatch, non-101 response; credentials never appear in errors.

### Phase B — Supervisor auto-pair and wiring

- [ ] B1 `soloist.SupervisorOptions.EngineCredentials func() (username string, data []byte, err error)`.
- [ ] B2 After `Start` sees `!loggedIn`, before returning
      `ErrPairingRequired`:
      - discover the ZeroConf port/path from the log (poll up to ~5s) or `ss`;
      - call `PairWithCredentials` with the engine credentials and
        `s.deviceName`;
      - wait for the next `auth_state.logged_in=true` (bounded, ~5s);
      - on success set `loggedIn`, clear the pairing error and continue;
      - on failure keep the pairing state.
- [ ] B3 `main.go` passes `audioRuntime.StoredCredentials` into
      `soloist.Options.EngineCredentials`.
- [ ] B4 Tests: fake credentials + fake ZeroConf server + fake WS `auth_state`
      flip; success returns nil and `State().LoggedIn`; failure returns
      `ErrPairingRequired`; no library credentials falls back.

### Phase C — Docs and verification

- [ ] C1 `docs/spotify-setup.md`: automatic pairing when the library engine is
      already paired; no Spotify app or second device needed; the one-tap flow
      remains only as a fallback.
- [ ] C2 `odd/tasks/soloist-onboarding.md`: update the pairing notes.
- [ ] C3 Full checks and a manual E2E on the failing PC:
      - fresh config, library paired, key saved → Soloist logs in
        automatically and plays without opening Spotify;
      - unpaired library → fallback one-tap;
      - wrong account/key → fallback, no crash.

## File map

New:

- `internal/soloist/pair.go`
- `internal/soloist/pair_test.go`
- `odd/tasks/soloist-autopair.md`

Modified:

- `internal/engine/supervisor.go`, `supervisor_test.go`
- `internal/soloist/supervisor.go`, `supervisor_test.go`
- `internal/soloist/service.go` (pass the callback)
- `main.go`
- `docs/spotify-setup.md`
- `odd/tasks/soloist-onboarding.md`

## Acceptance criteria

- With the library engine paired and a valid API key, starting Soloist signs it
  in automatically; the setup dialog never reaches the Spotify-app pairing
  step.
- The credentials are read from the engine's `0600` state file, sent only to
  `127.0.0.1`, and never logged.
- If any auto-pair step fails, the existing one-tap fallback still works.
- `task verify` passes; no new runtime dependency or bundled browser.

## Risks

- **Undocumented protocol.** Soloist's ZeroConf path, port log line or auth
  type may differ. The fallback keeps the app usable; Phase 0/E2E confirms the
  real build.
- **Same account.** The library account and the API key must belong to the same
  Spotify account. A mismatch falls back.
- **Credential sensitivity.** The reusable credentials are equivalent to a
  session; they must never be logged, shown in the UI, or sent anywhere except
  loopback.
- **Log parsing.** If Soloist changes the log line, the `ss` fallback covers it;
  if both fail, the one-tap flow remains.

## Outcome (2026-11-06) — rejected, code removed

The headless ZeroConf pairing was tested against the real Soloist build and
did not complete the account login. It is not part of the shipped code.

- The official `BlobBuilder` from `SpotifyWebApiPython` (`I/P/Q` markers,
  AES-ECB with `PBKDF2-SHA1(sha1(deviceID), username, 256, 20)`, outer
  DH/AES-CTR/HMAC) was used with the library's AP reusable credentials.
- `tokenType=""`, `default` and `accesstoken` returned HTTP 200 / `status 101`
  but `auth_state.logged_in` stayed `false`.
- `tokenType="auth_token"` reached `loginAuthToken` but failed with
  `session_invalid_connect_auth_token` (HTTP 500).
- The real `getInfo` returns `tokenType: "default"`, `modelDisplayName:
  "Soloist"`, `clientID: 334d00fed24f4e7b9abbe02ee28e0b5f`, and an `authInfo`
  base64 protobuf: a UUID, the client ID, and a nested 32-byte token/secret
  pair. That is the Soloist/Core Playback SDK client auth material; no public
  client uses it to create the initial session.
- `SpotifyWebApiPython` has an explicit Soloist branch stating that the initial
  session has not been figured out and that an authorized Spotify App must
  connect first. It records the same `auth_token`/`authorization_code`
  failures.
- Conclusion: the API key and librespot credentials are not enough. Soloist's
  initial account session is an undocumented, proprietary handshake. The
  auto-pair implementation (`pair.go`, `tryAutoPair`, `EngineCredentials`,
  `RetryPairing`, `OpenSpotify`) was removed in `1ea7a45`; the proven one-time
  Spotify app pairing is the only supported path.

### OAuth token matrix

A Spotify OAuth device-flow token (the same account, 446 characters) was also
tested against the real Soloist:

| Payload | `tokenType` | Result |
|---|---|---|
| raw OAuth token | `accesstoken` | HTTP 400, invalid arguments |
| raw OAuth token | `auth_token` | HTTP 500, invalid token |
| raw OAuth token | `default` | HTTP 400, invalid arguments |
| `BlobBuilder`, auth type 3 (OAuth token as password) | `accesstoken` | HTTP 200 / `101`, `logged_in=false` |
| `BlobBuilder`, auth type 3 | `auth_token` | HTTP 500, `session_invalid_connect_auth_token` |
| `BlobBuilder`, auth type 3 | `default` | HTTP 200 / `101`, `logged_in=false` |

The `auth_token` attempts are the only ones that reach `loginAuthToken`; they
fail with `session_invalid_connect_auth_token` and log
`login failed: make sure --api-key is valid`. The API key may also be invalid;
either way, no presented combination creates the initial Soloist session. No
playback was achieved.
