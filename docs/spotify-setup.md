# Spotify Developer Setup

Cadence uses the official Spotify Web API as a **controller**. It does not play
audio, does not use the Web Playback SDK and does not need a client secret.

## Requirements

- Spotify **Premium** account.
- Spotify Developer account.
- Development Mode is sufficient: the app owner must have Premium and at most
  five users can authorize the app.
- The official Spotify client must be available as a Spotify Connect device.

## 1. Create the Spotify app

1. Open the [Spotify Developer Dashboard](https://developer.spotify.com/dashboard).
2. Create an app.
3. Suggested name: `Cadence (personal)`.
4. Add this exact redirect URI:

   ```text
   http://127.0.0.1:43821/callback
   ```

5. Select the **Web API**.
6. Save the app and copy the **Client ID**.

Do **not** create or use a client secret. Cadence uses Authorization Code +
PKCE, which is designed for public clients.

## 2. Configure Cadence

The Client ID is not secret. Provide it in one of these ways:

```bash
export CADENCE_SPOTIFY_CLIENT_ID="your-client-id"
```

or click the **DEMO** / **CONNECT** badge in the titlebar, paste the Client ID in
the Spotify account panel and save it.

The redirect URI must match the Spotify Dashboard entry exactly. The fixed port
`43821` is reserved for this project. If the port is occupied at login time,
Cadence fails with a clear message instead of silently choosing another port.

The refresh token is stored in the OS keyring (Secret Service). Access tokens
remain in Go memory and are never sent to the frontend. Cadence never logs
tokens, authorization codes or PKCE verifiers.

The panel can switch between the real Spotify Connect session and the built-in
demo player without logging out. `cadence --demo` starts with the demo player
for the session.

## 3. Scopes

Cadence requests only the scopes needed for playback control and state:

```text
user-read-playback-state
user-modify-playback-state
user-read-currently-playing
```

No library, playlist, email, profile or offline-access scopes are requested.

## 4. OAuth flow

1. Cadence generates a PKCE verifier and a random `state`.
2. It opens the system browser at the Spotify authorization URL.
3. The user signs in and approves the requested scopes.
4. Spotify redirects to `http://127.0.0.1:43821/callback`.
5. Cadence validates `state`, exchanges the authorization code with the PKCE
   verifier and receives an access token and a refresh token.
6. The refresh token is stored in the OS keyring. The access token stays in Go
   memory.
7. The frontend receives only player state and controls; it never receives
   tokens.

## 5. Account panel

Click the provider badge in the titlebar (**DEMO** before connecting, **CONNECT**
afterward) to open the Spotify account panel. The panel shows:

- authorization state and the active playback source;
- the exact redirect URI to register, with a copy button;
- the Client ID field;
- reconnect, logout, and demo/Spotify mode controls.

If the active Spotify account has no Connect device, Cadence shows a **No active
device** state and keeps the device picker available. Start playback in the
official Spotify client, then transfer to that device from Cadence.

## 6. Development Mode notes

- The app owner must have Spotify Premium.
- A maximum of five users can be authorized while the app is in Development
  Mode.
- A shared public Client ID is not viable for a distributed app; each user
  should create their own Spotify app or the project must qualify for extended
  quota.
- Extended quota access is not assumed for Cadence v1.

## 7. API constraints to respect

Cadence must not depend on Spotify endpoints that were removed or restricted in
the February 2026 API changes, including new releases, categories, artist top
tracks, batch fetch and other users’ playlists. v1 only needs playback state,
devices, queue and player control endpoints.

Queue behavior is limited by the Web API. v1 displays the queue and does not
promise arbitrary queue reordering.

## 8. Troubleshooting

| Symptom | Likely cause |
|---|---|
| `INVALID_CLIENT` | Client ID is wrong or the app was deleted |
| `INVALID_REDIRECT_URI` | The dashboard URI does not exactly match `http://127.0.0.1:43821/callback` |
| `403` on playback | Account is not Premium or the app lacks a required scope |
| `404` on player endpoints | No active Spotify Connect device |
| `429` | Rate limit; honor `Retry-After` and back off |
| Login window does not return | Port 43821 is occupied or blocked by a local firewall |
| `OS keyring is unavailable` | Start a Secret Service provider such as GNOME Keyring and unlock it |
| Connect button is disabled | Save a non-empty Client ID and make sure the OS keyring is available |
| Session expired after logout or token rotation | Click **Reconnect Spotify**; Cadence clears the revoked local refresh token |
| `No active device` | Open the official Spotify client, start playback, then select its device in Cadence |

## 9. Security rules

- Never commit the Client ID inside a `.env` file or source file.
- Never add a client secret.
- Never send tokens to the frontend.
- Never log authorization codes, PKCE verifiers, access tokens or refresh
  tokens.
- Store the refresh token only in the OS keyring.
- Use the system browser, not an embedded WebView, for authorization.
