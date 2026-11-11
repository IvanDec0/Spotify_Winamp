# Security policy

## Supported versions

Cadence 0.2.x receives security fixes. Older versions are not supported; update to the latest 0.2.x release.

## Reporting a vulnerability

Report vulnerabilities privately through GitHub Security Advisories:

https://github.com/IvanDec0/Spotify_Winamp/security/advisories/new

Do not open a public issue with exploit details, credentials or tokens. Include the Cadence version (`cadence --version`), the active playback backend (web player, Soloist or go-librespot), reproduction steps and the impact you observed. Remove Spotify credentials, API keys, tokens and personal library data from logs and screenshots.

Allow time for a fix and a coordinated release before disclosing the issue publicly.

## Scope

In scope:

- leakage of Spotify credentials, OAuth tokens or the Soloist API key to the frontend, logs or another local process;
- unintended exposure of the go-librespot loopback API or another local service beyond `127.0.0.1`;
- path traversal or unsafe file handling in skin packages, ZIP extraction or asset loading;
- unsafe handling of downloaded runtimes, engine binaries or updates;
- command injection through file names, URLs or engine arguments.

Out of scope:

- Spotify server-side behavior, account restrictions or bans;
- breakage caused by Spotify protocol changes or unofficial engine updates;
- vulnerabilities in Spotify Soloist, go-librespot, Chromium or system libraries that are not caused by Cadence's integration; report those upstream;
- issues that require an already-compromised local account or unrestricted physical access.

## Handling

Valid reports are investigated privately. Fixes are prepared on a non-public branch when needed, and credit is given in the advisory unless you prefer to remain anonymous.
