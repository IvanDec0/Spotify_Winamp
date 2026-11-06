// Package developer automates the Spotify Developer dashboard for Cadence.
//
// Spotify does not provide an API to create a developer app or a Soloist API
// key. The service runs a small Electron helper against the user's signed-in
// dashboard profile, performs the requested action headlessly and returns only
// the public Client ID or the Soloist API key. Credentials are never logged.
// The helper shows its window only when Spotify asks the user to sign in: a
// consent page is accepted in the background, and OAuth mode drives the same
// persistent profile to the web player authorization page.
package developer
