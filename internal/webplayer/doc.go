// Package webplayer implements the optional Spotify Web Playback SDK
// backend: a lazy, checksum-verified Chromium runtime with Widevine, a
// loopback-only local HTTP server, a loopback WebSocket control socket and a
// process supervisor.
//
// The package is deliberately self-contained. Nothing here is linked into the
// default Cadence playback path until main selects the "web" backend, and the
// runtime is never bundled with the application: it is downloaded on demand
// under the user's cache directory and can be removed with a single delete.
//
// The Chromium runtime is castlabs Electron for Content Security. The helper
// application (main process, preload and player page) is embedded in the Go
// binary and materialized next to the downloaded runtime, so the only network
// dependency is the pinned runtime archive.
package webplayer
