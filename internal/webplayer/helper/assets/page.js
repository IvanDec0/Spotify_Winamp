"use strict";

// Cadence web player glue: runs in the sandboxed renderer served by the
// loopback HTTP server. It never receives Node integration; the only bridge
// is the preload's ready/error reporter.

(() => {
  const nonce = new URLSearchParams(location.search).get("nonce") || "";

  function report(code, message) {
    try {
      window.cadence?.error(code, message);
    } catch {
      // The preload bridge is optional while running the page outside the
      // helper (for example, during local validation).
    }
  }

  function fetchToken() {
    return fetch("/token?nonce=" + encodeURIComponent(nonce)).then((response) => {
      if (!response.ok) {
        throw new Error("token HTTP " + response.status);
      }
      return response.json();
    });
  }

  window.onSpotifyWebPlaybackSDKReady = () => {
    const player = new Spotify.Player({
      name: "Cadence Web",
      volume: 0.5,
      getOAuthToken: (callback) => {
        fetchToken()
          .then((data) => callback(data.access_token))
          .catch((error) => report("token_error", error.message));
      },
    });

    player.addListener("ready", ({ device_id }) => {
      try {
        window.cadence?.ready(device_id);
      } catch {
        // Nothing to report: without the bridge there is no consumer.
      }
    });
    player.addListener("not_ready", () => {});
    player.addListener("initialization_error", ({ message }) => report("initialization_error", message));
    player.addListener("authentication_error", ({ message }) => report("authentication_error", message));
    player.addListener("account_error", ({ message }) => report("account_error", message));
    player.addListener("playback_error", ({ message }) => report("playback_error", message));
    player.addListener("player_state_changed", () => {});

    player.connect().then(
      (connected) => {
        if (!connected) {
          report("connect_failed", "the Spotify player did not connect");
        }
      },
      (error) => report("connect_failed", error.message),
    );
  };
})();
