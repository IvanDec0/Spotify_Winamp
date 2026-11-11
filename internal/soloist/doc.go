// Package soloist integrates Spotify Soloist, Spotify's headless Spotify
// Connect receiver, as an optional audio backend.
//
// Soloist is never bundled or redistributed. The installer downloads builds
// only from Spotify's own servers, and the user supplies their own API key
// from the Spotify developer dashboard. This package owns the loopback
// WebSocket protocol, version and expiry parsing, the credential store, the
// installer, the child process supervisor and the service facade used by the
// application.
package soloist
