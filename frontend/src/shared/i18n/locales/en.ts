/**
 * Base catalog. Keys are the source of truth: `MessageKey` is derived from
 * this object, so a typo is a compile-time error.
 */
export const en = {
  "ui.close": "Close",
  "ui.cancel": "Cancel",
  "ui.dismiss": "Dismiss",
  "ui.retry": "Retry",
  "ui.search": "Search",
  "ui.loading": "Loading",
  "ui.notifications": "Notifications",
  "ui.empty": "Nothing here yet",
  "ui.items": "{count, plural, one {# item} other {# items}}",
  "browse.open": "Browse",
  "browse.navLabel": "Browse sections",
  "browse.search": "Search",
  "browse.library": "Library",
  "browse.podcasts": "Podcasts",
  "browse.settings": "Settings",
  "browse.searchTitle": "Search the catalog",
  "browse.searchBody":
    "Find tracks, albums, artists, playlists, shows and episodes. Search lands in the next update.",
  "browse.libraryTitle": "Your library",
  "browse.libraryBody":
    "Playlists, saved albums, followed artists, shows and Your Episodes will live here.",
  "browse.podcastsTitle": "Podcasts",
  "browse.podcastsBody": "Follow shows and catch up on Your Episodes from this panel.",
  "browse.settingsTitle": "Settings",
  "browse.settingsBody": "Playback, cache and appearance controls are coming to the Browse window.",
  "browse.welcomeBody":
    "This window keeps the catalog within reach while the compact player stays untouched.",
  "browse.catalogStatus": "Spotify catalog",
  "browse.skinTheme": "{skin} · {theme}",
  "browse.nowPlaying": "Now playing",
  "browse.nothingPlaying": "Nothing playing",
  "browse.play": "Play",
  "browse.pause": "Pause",
  "browse.previous": "Previous",
  "browse.next": "Next",
  "browse.volume": "Volume",
  "browse.progress": "Playback position",
} as const;

export type MessageKey = keyof typeof en;
