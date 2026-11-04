<script lang="ts">
  import Icon from "./components/Icon.svelte";
  import LibrarySkeleton from "./components/LibrarySkeleton.svelte";
  import MediaCover from "./components/MediaCover.svelte";
  import SearchRow from "./components/SearchRow.svelte";
  import type { SearchHit } from "../shared/api/types";
  import { player } from "./player.svelte";
  import { music } from "./music.svelte";
  import { formatTime } from "./format";

  interface Props {
    onClose: () => void;
  }

  type Tab = "playlists" | "albums" | "artists" | "liked";
  type SearchKind = "track" | "album" | "artist" | "playlist";

  let { onClose }: Props = $props();

  let tab = $state<Tab>("playlists");
  let contextTitle = $state("");
  let searchQuery = $state("");
  let loaded = $state<Record<Tab, boolean>>({
    playlists: false,
    albums: false,
    artists: false,
    liked: false,
  });
  let wasAuthenticated = $state(false);
  let copied = $state(false);
  let resetArmed = $state(false);

  let audio = $derived(music.audio);
  let context = $derived(music.context);
  let currentTrackURI = $derived(player.state.track.uri);
  let searchResults = $derived(music.searchResults);
  let searchActive = $derived(
    music.searchQuery !== "" || music.searching || music.searchError !== null,
  );
  let searchTab = $state<SearchKind>("track");
  let searchCounts = $derived({
    track: searchResults?.tracks.length ?? 0,
    album: searchResults?.albums.length ?? 0,
    artist: searchResults?.artists.length ?? 0,
    playlist: searchResults?.playlists.length ?? 0,
  });

  $effect(() => {
    if (!searchResults) return;
    if (searchCounts[searchTab] > 0) return;
    const first = (["track", "album", "artist", "playlist"] as const).find(
      (kind) => searchCounts[kind] > 0,
    );
    if (first) searchTab = first;
  });

  $effect(() => {
    const authenticated = music.authenticated;
    if (authenticated === wasAuthenticated) return;
    wasAuthenticated = authenticated;
    loaded = { playlists: false, albums: false, artists: false, liked: false };
    if (!authenticated) {
      music.clearLibrary();
      return;
    }
    music.dismissNotice();
  });

  $effect(() => {
    if (!music.authenticated) return;
    if (tab === "playlists" && !loaded.playlists) {
      loaded = { ...loaded, playlists: true };
      void music.loadPlaylists();
    } else if (tab === "albums" && !loaded.albums) {
      loaded = { ...loaded, albums: true };
      void music.loadAlbums();
    } else if (tab === "artists" && !loaded.artists) {
      loaded = { ...loaded, artists: true };
      void music.loadArtists();
    } else if (tab === "liked" && !loaded.liked) {
      loaded = { ...loaded, liked: true };
    }
  });

  async function openContext(uri: string, title: string): Promise<void> {
    contextTitle = title;
    music.clearSearch();
    searchQuery = "";
    await music.loadContext(uri);
  }

  async function runSearch(): Promise<void> {
    if (!searchQuery.trim()) {
      clearSearch();
      return;
    }
    await music.search(searchQuery);
  }

  function clearSearch(): void {
    searchQuery = "";
    music.clearSearch();
  }

  function openSearchHit(hit: SearchHit, kind: SearchKind): void {
    if (kind === "track") {
      music.clearSearch();
      searchQuery = "";
      void music.playTrack(hit.uri);
      return;
    }
    void openContext(hit.uri, hit.name);
  }

  function startRadio(uri: string): void {
    void music.startRadio(uri);
  }

  function close(): void {
    music.clearContext();
    music.clearSearch();
    onClose();
  }

  async function copyPairingCode(): Promise<void> {
    const code = audio.pairing?.code;
    if (!code) return;
    try {
      await navigator.clipboard.writeText(code);
      copied = true;
      setTimeout(() => {
        copied = false;
      }, 1600);
    } catch {
      copied = false;
    }
  }

  async function resetPairing(): Promise<void> {
    if (!resetArmed) {
      resetArmed = true;
      return;
    }
    resetArmed = false;
    await music.resetSession();
  }
</script>

<div class="library" role="dialog" aria-modal="true" aria-label="Music library">
  <header class="library-head">
    <div class="library-title">
      <span class="eyebrow">Cadence</span>
      <h2>{context ? contextTitle || "Playlist" : "Library"}</h2>
      {#if audio.authenticated}
        <span class="engine-chip">LOCAL</span>
      {/if}
    </div>
    <div class="head-actions">
      {#if context}
        <button class="head-button" type="button" onclick={() => music.clearContext()}>
          <span class="head-button-icon" aria-hidden="true">
            <Icon name="chevronDown" size={11} />
          </span>
          Library
        </button>
      {:else if audio.authenticated}
        <button
          class="head-button"
          type="button"
          disabled={music.busy}
          onclick={() => void music.restart()}
        >
          Restart
        </button>
      {/if}
      <button class="close" type="button" aria-label="Close music library" onclick={close}>
        <Icon name="close" size={12} />
      </button>
    </div>
  </header>

  <form
    class="searchbar"
    role="search"
    onsubmit={(event) => {
      event.preventDefault();
      void runSearch();
    }}
  >
    <span class="searchbar-icon" aria-hidden="true"><Icon name="search" size={12} /></span>
    <input
      type="search"
      bind:value={searchQuery}
      placeholder="Search songs, albums, artists, playlists"
      aria-label="Search Spotify"
      autocomplete="off"
      spellcheck="false"
    />
    {#if searchActive}
      <button class="searchbar-clear" type="button" aria-label="Clear search" onclick={clearSearch}>
        <Icon name="close" size={10} />
      </button>
    {/if}
    <button class="searchbar-submit" type="submit" disabled={music.searching}>
      {music.searching ? "Searching…" : "Search"}
    </button>
  </form>

  {#if music.error}
    <div class="banner error banner-row">
      <span>{music.error}</span>
      <button type="button" onclick={() => music.dismissError()}>Dismiss</button>
    </div>
  {/if}
  {#if music.notice}
    <div class="banner info banner-row">
      <span>{music.notice}</span>
      <button type="button" onclick={() => music.dismissNotice()}>Dismiss</button>
    </div>
  {/if}

  <div class="library-body">
    {#if searchActive}
      <section class="search-panel" aria-label="Search results">
        <div class="search-summary">
          <span class="eyebrow">Search</span>
          <strong>{music.searchQuery || "Spotify"}</strong>
          <button class="head-button" type="button" onclick={clearSearch}>Clear</button>
        </div>
        {#if music.searchError}
          <p class="banner error">{music.searchError}</p>
        {/if}
        {#if music.searching && !searchResults}
          <LibrarySkeleton rows={8} />
        {:else if searchResults}
          <div class="search-tabs" role="group" aria-label="Search result types">
            {#each [{ id: "track", label: "Tracks" }, { id: "album", label: "Albums" }, { id: "artist", label: "Artists" }, { id: "playlist", label: "Playlists" }] as option (option.id)}
              <button
                class="search-tab"
                class:is-active={searchTab === option.id}
                type="button"
                aria-pressed={searchTab === option.id}
                onclick={() => {
                  searchTab = option.id as SearchKind;
                }}
              >
                {option.label}
                <span>{searchCounts[option.id as SearchKind]}</span>
              </button>
            {/each}
          </div>

          {#if searchTab === "track" && searchResults.tracks.length > 0}
            <ul class="search-list">
              {#each searchResults.tracks as hit (hit.uri)}
                <SearchRow
                  {hit}
                  kind="track"
                  meta={`${hit.artists?.map((artist) => artist.name).join(", ") || "Unknown artist"}${hit.album ? ` · ${hit.album.name}` : ""}${hit.duration > 0 ? ` · ${formatTime(hit.duration / 1000)}` : ""}`}
                  onOpen={() => openSearchHit(hit, "track")}
                  onQueue={() => void music.addToQueue(hit.uri)}
                  onRadio={() => startRadio(hit.uri)}
                />
              {/each}
            </ul>
          {:else if searchTab === "album" && searchResults.albums.length > 0}
            <ul class="search-list">
              {#each searchResults.albums as hit (hit.uri)}
                <SearchRow
                  {hit}
                  kind="album"
                  meta={hit.artists?.map((artist) => artist.name).join(", ") || "Album"}
                  onOpen={() => openSearchHit(hit, "album")}
                  onRadio={() => startRadio(hit.uri)}
                />
              {/each}
            </ul>
          {:else if searchTab === "artist" && searchResults.artists.length > 0}
            <ul class="search-list">
              {#each searchResults.artists as hit (hit.uri)}
                <SearchRow
                  {hit}
                  kind="artist"
                  meta="Artist"
                  onOpen={() => openSearchHit(hit, "artist")}
                  onRadio={() => startRadio(hit.uri)}
                />
              {/each}
            </ul>
          {:else if searchTab === "playlist" && searchResults.playlists.length > 0}
            <ul class="search-list">
              {#each searchResults.playlists as hit (hit.uri)}
                <SearchRow
                  {hit}
                  kind="playlist"
                  meta={`${hit.author || "Playlist"}${hit.followersCount > 0 ? ` · ${hit.followersCount.toLocaleString()} followers` : ""}`}
                  onOpen={() => openSearchHit(hit, "playlist")}
                  onRadio={() => startRadio(hit.uri)}
                />
              {/each}
            </ul>
          {:else}
            <p class="empty-copy">No results in this category.</p>
          {/if}
        {:else if !music.searching}
          <p class="empty-copy">Type a search to explore the Spotify catalog.</p>
        {/if}
      </section>
    {:else if !audio.available}
      <section class="empty-state">
        <span class="empty-glyph" aria-hidden="true"><Icon name="visualizer" size={20} /></span>
        <h3>Local audio engine unavailable</h3>
        <p>
          Cadence plays Spotify through <strong>go-librespot</strong>. Build or install the engine,
          then restart Cadence.
        </p>
        <code>task audio:build</code>
        {#if audio.binaryPath}<p class="path">{audio.binaryPath}</p>{/if}
        <button
          class="primary"
          type="button"
          disabled={music.busy}
          onclick={() => void music.restart()}
        >
          Retry engine
        </button>
      </section>
    {:else if !audio.authenticated}
      <section class="pairing-state">
        <span class="eyebrow">Local Spotify device</span>
        <h3>{audio.pairing ? "Pair Cadence with Spotify" : "Starting the audio engine…"}</h3>
        {#if audio.pairing}
          <p class="pair-code">{audio.pairing.code}</p>
          <p class="hint">
            Open the Spotify pairing page and approve Cadence as a device. This window updates
            automatically.
          </p>
          <div class="pairing-actions">
            <button class="primary" type="button" onclick={() => void music.openPairing()}>
              Open pairing page
            </button>
            <button class="secondary" type="button" onclick={() => void copyPairingCode()}>
              {copied ? "Code copied" : "Copy code"}
            </button>
          </div>
          <p class="path">{audio.pairing.url}</p>
        {:else}
          <p class="hint">
            The engine is running but has not produced a pairing code yet.
            {#if audio.error}{audio.error}{/if}
          </p>
        {/if}
        <div class="pairing-actions">
          <button
            class="secondary"
            type="button"
            disabled={music.busy}
            onclick={() => void music.refresh()}
          >
            Check again
          </button>
          <button
            class="secondary"
            type="button"
            disabled={music.busy}
            onclick={() => void music.restart()}
          >
            Restart engine
          </button>
          <button
            class="secondary danger"
            class:is-armed={resetArmed}
            type="button"
            disabled={music.busy}
            onclick={() => void resetPairing()}
          >
            {resetArmed ? "Confirm reset" : "Reset pairing"}
          </button>
        </div>
      </section>
    {:else if context}
      <section class="context-panel">
        <div class="context-actions">
          <button
            class="primary play-all"
            type="button"
            disabled={music.busy}
            onclick={() => void music.playContext(context.uri || "")}
          >
            <Icon name="play" size={10} />
            Play all
          </button>
          <button
            class="secondary radio-button"
            type="button"
            disabled={music.busy}
            onclick={() => startRadio(context.uri || "")}
          >
            <Icon name="radio" size={10} />
            Radio
          </button>
          <span class="context-chip mono">
            {context.length || context.items.length} tracks
          </span>
          {#if !context.ready}
            <span class="context-status">Loading tracks… {context.cached}/{context.length}</span>
          {/if}
        </div>
        {#if context.items.length === 0}
          {#if context.ready}
            <p class="empty-copy">This context has no tracks.</p>
          {:else}
            <LibrarySkeleton rows={6} compact />
          {/if}
        {:else}
          <ol class="track-list">
            {#each context.items as item, index (item.uri + ":" + index)}
              <li class="track-row" class:is-current={item.uri === currentTrackURI}>
                {#if item.uri === currentTrackURI}
                  <span class="playing-bars" aria-label="Currently playing">
                    <i></i><i></i><i></i>
                  </span>
                {:else}
                  <span class="track-index">{String(index + 1).padStart(2, "0")}</span>
                {/if}
                <MediaCover src={item.cover} label={item.title || item.uri} size={28} />
                <button
                  class="track-copy"
                  type="button"
                  disabled={!item.title || music.busy}
                  onclick={() => void music.playContext(context.uri, item.uri)}
                >
                  <span class="track-name">{item.title || "Loading track…"}</span>
                  <span class="track-meta">
                    {item.artists?.join(", ") || "Unknown artist"}
                    {#if item.album}
                      · {item.album}{/if}
                  </span>
                </button>
                <span class="track-time">{formatTime(item.durationMs / 1000)}</span>
                <button
                  class="queue-add"
                  type="button"
                  title="Add to play queue"
                  aria-label={`Add ${item.title || "track"} to queue`}
                  disabled={!item.uri || music.busy}
                  onclick={() => void music.addToQueue(item.uri)}
                >
                  +
                </button>
                <button
                  class="queue-add radio-add"
                  type="button"
                  title="Start radio from this track"
                  aria-label={`Start radio from ${item.title || "this track"}`}
                  disabled={!item.uri || music.busy}
                  onclick={() => startRadio(item.uri)}
                >
                  <Icon name="radio" size={9} />
                </button>
              </li>
            {/each}
          </ol>
        {/if}
      </section>
    {:else}
      <nav class="tabs" aria-label="Library sections">
        {#each [{ id: "playlists", label: "Playlists" }, { id: "albums", label: "Albums" }, { id: "artists", label: "Artists" }, { id: "liked", label: "Liked" }] as option (option.id)}
          <button
            class="tab"
            class:is-active={tab === option.id}
            type="button"
            aria-pressed={tab === option.id}
            onclick={() => {
              tab = option.id as Tab;
            }}
          >
            {option.label}
            {#if option.id === "playlists" && loaded.playlists && music.playlists.length > 0}
              <span class="tab-count">{music.playlists.length}</span>
            {:else if option.id === "albums" && loaded.albums && music.albums.length > 0}
              <span class="tab-count">{music.albums.length}</span>
            {:else if option.id === "artists" && loaded.artists && music.artists.length > 0}
              <span class="tab-count">{music.artists.length}</span>
            {/if}
          </button>
        {/each}
      </nav>

      {#if tab === "playlists"}
        {#if music.playlists.length === 0}
          <p class="empty-copy">{music.busy ? "Loading playlists…" : "No playlists found."}</p>
          {#if music.busy}<LibrarySkeleton rows={7} />{/if}
        {:else}
          <ul class="library-list">
            {#each music.playlists as playlist (playlist.uri)}
              <li>
                <button
                  class="library-row"
                  type="button"
                  onclick={() => void openContext(playlist.uri, playlist.name)}
                >
                  <MediaCover src={playlist.imageUrl} label={playlist.name} size={34} />
                  <span class="row-copy">
                    <span class="row-name">{playlist.name}</span>
                    <span class="row-meta">
                      {playlist.owner || "Playlist"} · {playlist.length} tracks
                    </span>
                  </span>
                  <span class="row-arrow" aria-hidden="true">
                    <Icon name="chevronDown" size={10} />
                  </span>
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      {:else if tab === "albums"}
        {#if music.albums.length === 0}
          <p class="empty-copy">{music.busy ? "Loading albums…" : "No saved albums found."}</p>
          {#if music.busy}<LibrarySkeleton rows={7} />{/if}
        {:else}
          <ul class="library-list">
            {#each music.albums as album (album.uri)}
              <li>
                <button
                  class="library-row"
                  type="button"
                  onclick={() => void openContext(album.uri, album.name)}
                >
                  <MediaCover src={album.imageUrl} label={album.name} size={34} round={false} />
                  <span class="row-copy">
                    <span class="row-name">{album.name}</span>
                    <span class="row-meta">
                      {album.artistNames?.join(", ") || "Unknown artist"}
                      {#if album.year}
                        · {album.year}{/if}
                    </span>
                  </span>
                  <span class="row-arrow" aria-hidden="true">
                    <Icon name="chevronDown" size={10} />
                  </span>
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      {:else if tab === "artists"}
        {#if music.artists.length === 0}
          <p class="empty-copy">{music.busy ? "Loading artists…" : "No followed artists found."}</p>
          {#if music.busy}<LibrarySkeleton rows={7} />{/if}
        {:else}
          <ul class="library-list">
            {#each music.artists as artist (artist.uri)}
              <li>
                <button
                  class="library-row"
                  type="button"
                  onclick={() => void openContext(artist.uri, artist.name)}
                >
                  <MediaCover src={artist.imageUrl} label={artist.name} size={34} round={true} />
                  <span class="row-copy">
                    <span class="row-name">{artist.name}</span>
                    <span class="row-meta">Artist</span>
                  </span>
                  <span class="row-arrow" aria-hidden="true">
                    <Icon name="chevronDown" size={10} />
                  </span>
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      {:else}
        <button
          class="library-row liked-row"
          type="button"
          onclick={() => void openContext("spotify:collection:tracks", "Liked Songs")}
        >
          <span class="liked-mark" aria-hidden="true">♥</span>
          <span class="row-copy">
            <span class="row-name">Liked Songs</span>
            <span class="row-meta">Your saved tracks</span>
          </span>
          <span class="row-arrow" aria-hidden="true">
            <Icon name="chevronDown" size={10} />
          </span>
        </button>
      {/if}
    {/if}
  </div>

  <footer class="library-foot">
    <span>
      {#if audio.authenticated}
        {audio.deviceName} · local playback · {audio.audioBackend}
      {:else}
        go-librespot · local Spotify engine
      {/if}
    </span>
    {#if audio.authenticated}
      <button
        class="foot-button"
        type="button"
        disabled={music.busy}
        class:is-armed={resetArmed}
        onclick={() => void resetPairing()}
      >
        {resetArmed ? "Confirm reset" : "Reset pairing"}
      </button>
    {/if}
  </footer>
</div>

<style>
  .library {
    display: flex;
    height: 100%;
    flex-direction: column;
    color: var(--text);
    background:
      radial-gradient(
        70% 40% at 50% -10%,
        color-mix(in srgb, var(--accent) 8%, transparent),
        transparent 70%
      ),
      var(--bg);
    font-family: "Inter", "Segoe UI", system-ui, sans-serif;
  }

  .library-head {
    --wails-draggable: drag;
    position: relative;
    display: flex;
    min-height: 34px;
    flex: 0 0 auto;
    align-items: center;
    justify-content: space-between;
    padding: 5px 10px 5px 13px;
    border-bottom: 1px solid var(--line);
    background:
      linear-gradient(180deg, color-mix(in srgb, var(--surface-2) 26%, transparent), transparent),
      color-mix(in srgb, var(--bg) 82%, transparent);
    cursor: grab;
  }

  .library-head::after {
    content: "";
    position: absolute;
    right: 0;
    bottom: -1px;
    left: 0;
    height: 1px;
    background: linear-gradient(
      90deg,
      transparent,
      color-mix(in srgb, var(--accent) 22%, transparent),
      transparent
    );
    opacity: 0.45;
    pointer-events: none;
  }

  .library-title {
    display: flex;
    min-width: 0;
    align-items: baseline;
    gap: 7px;
  }

  h2 {
    overflow: hidden;
    margin: 0;
    font-size: 14px;
    font-weight: 620;
    letter-spacing: -0.02em;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .eyebrow {
    color: var(--accent);
    font-family: "JetBrains Mono", monospace;
    font-size: 6.5px;
    letter-spacing: 0.2em;
    text-transform: uppercase;
  }

  .engine-chip {
    padding: 1px 4px;
    border: 1px solid color-mix(in srgb, var(--accent) 28%, transparent);
    border-radius: 3px;
    color: var(--accent);
    font-family: "JetBrains Mono", monospace;
    font-size: 5.5px;
    letter-spacing: 0.08em;
  }

  .head-actions {
    --wails-draggable: no-drag;
    display: flex;
    align-items: center;
    gap: 4px;
    cursor: default;
  }

  .head-button,
  .close {
    --wails-draggable: no-drag;
    cursor: pointer;
  }

  .searchbar {
    display: flex;
    min-height: 30px;
    flex: 0 0 auto;
    align-items: center;
    gap: 6px;
    margin: 6px 12px 0;
    padding: 0 6px 0 9px;
    border: 1px solid var(--line-strong);
    border-radius: 6px;
    background:
      linear-gradient(180deg, rgb(255 255 255 / 3%), transparent),
      color-mix(in srgb, var(--surface-2) 45%, transparent);
    transition:
      border-color var(--motion-fast) ease,
      box-shadow var(--motion-fast) ease;
  }

  .searchbar:focus-within {
    border-color: color-mix(in srgb, var(--accent) 45%, transparent);
    box-shadow:
      0 0 0 3px color-mix(in srgb, var(--accent) 7%, transparent),
      0 0 16px color-mix(in srgb, var(--accent) 7%, transparent);
  }

  .searchbar-icon {
    display: grid;
    flex: 0 0 auto;
    place-items: center;
    color: var(--faint);
  }

  .searchbar input {
    min-width: 0;
    flex: 1 1 auto;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--text);
    font-family: inherit;
    font-size: 8.5px;
  }

  .searchbar input::placeholder {
    color: var(--faint);
  }

  .searchbar input::-webkit-search-cancel-button {
    display: none;
  }

  .searchbar-clear {
    display: grid;
    width: 18px;
    height: 18px;
    flex: 0 0 18px;
    place-items: center;
    border-radius: 3px;
    color: var(--faint);
  }

  .searchbar-clear:hover {
    background: var(--surface-2);
    color: var(--text);
  }

  .searchbar-submit {
    height: 20px;
    flex: 0 0 auto;
    padding: 0 6px;
    border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
    border-radius: 4px;
    background: color-mix(in srgb, var(--accent) 9%, transparent);
    color: var(--accent);
    font-family: "JetBrains Mono", monospace;
    font-size: 6.5px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    transition:
      background-color var(--motion-fast) ease,
      border-color var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-spring);
  }

  .searchbar-submit:hover:not(:disabled) {
    border-color: color-mix(in srgb, var(--accent) 55%, transparent);
    background: color-mix(in srgb, var(--accent) 16%, transparent);
    transform: translateY(-1px);
  }

  .search-summary {
    display: flex;
    align-items: baseline;
    gap: 6px;
    margin-bottom: 6px;
  }

  .search-tabs {
    display: flex;
    gap: 3px;
    margin-bottom: 5px;
  }

  .search-tab {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 3px 6px;
    border: 1px solid transparent;
    border-radius: 3px;
    color: var(--faint);
    font-size: 7.5px;
    transition:
      color var(--motion-fast) ease,
      border-color var(--motion-fast) ease,
      background-color var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-spring);
  }

  .search-tab span {
    color: var(--faint);
    font-family: "JetBrains Mono", monospace;
    font-size: 5.5px;
  }

  .search-tab:hover {
    color: var(--text);
    transform: translateY(-1px);
  }

  .search-tab.is-active {
    border-color: color-mix(in srgb, var(--accent) 30%, transparent);
    background: color-mix(in srgb, var(--accent) 9%, transparent);
    color: var(--accent);
    box-shadow: 0 0 14px color-mix(in srgb, var(--accent) 7%, transparent);
  }

  .search-tab.is-active span {
    color: color-mix(in srgb, var(--accent) 70%, var(--faint));
  }

  .search-summary strong {
    overflow: hidden;
    color: var(--text);
    font-size: 10px;
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .search-summary .head-button {
    margin-left: auto;
  }

  .search-list {
    display: flex;
    flex-direction: column;
    gap: 1px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .radio-button {
    display: inline-flex;
    align-items: center;
    gap: 3px;
  }

  .head-button {
    height: 20px;
    padding: 0 5px;
    border: 1px solid var(--line);
    border-radius: 4px;
    color: var(--muted);
    font-size: 7.5px;
    transition:
      border-color var(--motion-fast) ease,
      background-color var(--motion-fast) ease,
      color var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-spring);
  }

  .head-button:hover {
    border-color: color-mix(in srgb, var(--accent) 35%, transparent);
    background: color-mix(in srgb, var(--accent) 6%, transparent);
    color: var(--accent);
    transform: translateY(-1px);
  }

  .head-button-icon {
    display: inline-block;
    margin-right: 2px;
    transform: rotate(90deg);
  }

  .close {
    display: grid;
    width: 22px;
    height: 22px;
    place-items: center;
    border-radius: 4px;
    color: var(--faint);
  }

  .close:hover {
    background: var(--surface-2);
    color: var(--text);
  }

  .banner {
    margin: 5px 12px 0;
    padding: 5px 7px;
    border: 1px solid var(--line);
    border-radius: 4px;
    font-size: 8px;
    line-height: 1.35;
  }

  .banner.error {
    border-color: color-mix(in srgb, var(--danger) 35%, transparent);
    background: color-mix(in srgb, var(--danger) 7%, transparent);
    color: color-mix(in srgb, var(--danger) 80%, var(--text));
  }

  .banner.info {
    border-color: color-mix(in srgb, var(--accent) 26%, transparent);
    background: color-mix(in srgb, var(--accent) 6%, transparent);
    color: var(--text);
  }

  .banner-row {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 8px;
  }

  .banner-row button {
    flex: 0 0 auto;
    color: var(--muted);
    font-size: 7.5px;
    text-decoration: underline;
  }

  .library-body {
    overflow: auto;
    flex: 1 1 auto;
    min-height: 0;
    padding: 7px 12px 8px;
    scrollbar-width: thin;
  }

  .tabs {
    position: sticky;
    top: -7px;
    z-index: 2;
    display: flex;
    gap: 4px;
    margin: -7px -12px 6px;
    padding: 7px 12px 5px;
    background: color-mix(in srgb, var(--bg) 88%, transparent);
    backdrop-filter: blur(10px);
  }

  .tab {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 3px 6px;
    border: 1px solid transparent;
    border-radius: 3px;
    color: var(--faint);
    font-size: 7.5px;
    transition:
      color var(--motion-fast) ease,
      border-color var(--motion-fast) ease,
      background-color var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-spring);
  }

  .tab:hover {
    color: var(--text);
    transform: translateY(-1px);
  }

  .tab.is-active {
    border-color: color-mix(in srgb, var(--accent) 30%, transparent);
    background: color-mix(in srgb, var(--accent) 9%, transparent);
    color: var(--accent);
    box-shadow: 0 0 14px color-mix(in srgb, var(--accent) 7%, transparent);
  }

  .tab-count {
    padding: 0 3px;
    border-radius: 2px;
    background: color-mix(in srgb, var(--text) 6%, transparent);
    color: var(--faint);
    font-family: "JetBrains Mono", monospace;
    font-size: 5.5px;
    line-height: 1.5;
  }

  .library-list,
  .track-list {
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .library-row {
    display: flex;
    width: 100%;
    min-height: 40px;
    align-items: center;
    gap: 8px;
    padding: 3px 4px;
    border-radius: 4px;
    text-align: left;
    transition:
      background-color var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-out);
  }

  .library-row:hover {
    background: color-mix(in srgb, var(--accent) 7%, transparent);
    transform: translateX(1px);
  }

  .library-row:hover .row-arrow {
    color: var(--accent);
    transform: translateX(1px) rotate(-90deg);
  }

  .liked-row {
    border: 1px solid var(--line);
    background: color-mix(in srgb, var(--accent) 4%, transparent);
  }

  .liked-mark {
    display: grid;
    width: 34px;
    height: 34px;
    flex: 0 0 34px;
    place-items: center;
    border-radius: 5px;
    background: color-mix(in srgb, var(--accent) 13%, var(--surface-2));
    color: var(--accent);
    font-size: 14px;
  }

  .library-row > :last-child {
    margin-left: auto;
  }

  .row-arrow {
    margin-left: auto;
    color: var(--faint);
    transform: rotate(-90deg);
    transition:
      color var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-spring);
  }

  .row-copy {
    display: flex;
    min-width: 0;
    flex: 1 1 auto;
    flex-direction: column;
    gap: 2px;
  }

  .row-name {
    overflow: hidden;
    color: var(--text);
    font-size: 9.5px;
    font-weight: 550;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .row-meta {
    overflow: hidden;
    color: var(--faint);
    font-size: 7.5px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .context-actions {
    display: flex;
    align-items: center;
    gap: 7px;
    margin-bottom: 6px;
  }

  .play-all {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }

  .play-all:hover:not(:disabled) {
    background: color-mix(in srgb, var(--accent) 20%, var(--surface-2));
    box-shadow: 0 0 16px color-mix(in srgb, var(--accent) 12%, transparent);
    transform: translateY(-1px);
  }

  .context-chip {
    padding: 1px 4px;
    border: 1px solid var(--line);
    border-radius: 3px;
    color: var(--faint);
    font-size: 6px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .context-status {
    color: var(--faint);
    font-family: "JetBrains Mono", monospace;
    font-size: 6.5px;
  }

  .primary,
  .secondary {
    border: 1px solid var(--line-strong);
    border-radius: 4px;
    font-size: 8px;
    transition:
      border-color var(--motion-fast) ease,
      background-color var(--motion-fast) ease,
      color var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-spring),
      box-shadow var(--motion-fast) ease;
  }

  .primary {
    padding: 5px 8px;
    border-color: color-mix(in srgb, var(--accent) 35%, transparent);
    background: color-mix(in srgb, var(--accent) 13%, var(--surface-2));
    color: var(--text);
    font-weight: 550;
  }

  .secondary {
    padding: 4px 7px;
    background: var(--surface-2);
    color: var(--muted);
  }

  .secondary.danger,
  .foot-button.is-armed {
    color: color-mix(in srgb, var(--danger) 78%, var(--text));
  }

  .secondary.is-armed,
  .foot-button.is-armed {
    border-color: color-mix(in srgb, var(--danger) 50%, transparent);
    background: color-mix(in srgb, var(--danger) 12%, transparent);
    color: var(--danger);
  }

  .primary:hover:not(:disabled),
  .secondary:hover:not(:disabled),
  .foot-button:hover:not(:disabled) {
    border-color: color-mix(in srgb, var(--accent) 45%, transparent);
    color: var(--accent);
    transform: translateY(-1px);
  }

  .track-row {
    display: flex;
    min-height: 34px;
    align-items: center;
    gap: 6px;
    border-radius: 4px;
    padding: 2px 4px;
    transition:
      background-color var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-out);
  }

  .track-row:hover {
    background: color-mix(in srgb, var(--accent) 6%, transparent);
    transform: translateX(1px);
  }

  .track-row.is-current {
    background:
      linear-gradient(90deg, color-mix(in srgb, var(--accent) 10%, transparent), transparent 82%),
      color-mix(in srgb, var(--surface-2) 32%, transparent);
    box-shadow: 0 0 18px color-mix(in srgb, var(--accent) 5%, transparent) inset;
  }

  .track-row.is-current .track-name {
    color: var(--accent);
  }

  .playing-bars {
    display: flex;
    width: 16px;
    height: 10px;
    flex: 0 0 16px;
    align-items: flex-end;
    justify-content: flex-end;
    gap: 1.5px;
  }

  .playing-bars i {
    display: block;
    width: 1.5px;
    height: 5px;
    border-radius: 1px;
    background: var(--accent);
    box-shadow: 0 0 4px var(--accent-glow);
    animation: library-bounce 900ms ease-in-out infinite;
  }

  .playing-bars i:nth-child(2) {
    height: 9px;
    animation-delay: -0.3s;
  }

  .playing-bars i:nth-child(3) {
    height: 7px;
    animation-delay: -0.6s;
  }

  @keyframes library-bounce {
    0%,
    100% {
      transform: scaleY(0.4);
    }
    50% {
      transform: scaleY(1);
    }
  }

  .track-index {
    width: 16px;
    flex: 0 0 16px;
    color: var(--faint);
    font-family: "JetBrains Mono", monospace;
    font-size: 6.5px;
    text-align: right;
  }

  .track-copy {
    display: flex;
    min-width: 0;
    flex: 1 1 auto;
    flex-direction: column;
    gap: 1px;
    text-align: left;
  }

  .track-name {
    overflow: hidden;
    color: var(--text);
    font-size: 9px;
    font-weight: 520;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .track-meta {
    overflow: hidden;
    color: var(--faint);
    font-size: 7px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .track-time {
    flex: 0 0 auto;
    color: var(--faint);
    font-family: "JetBrains Mono", monospace;
    font-size: 6.5px;
  }

  .queue-add {
    display: grid;
    width: 17px;
    height: 17px;
    flex: 0 0 17px;
    place-items: center;
    border: 1px solid var(--line);
    border-radius: 3px;
    color: var(--faint);
    font-size: 11px;
    line-height: 1;
  }

  .queue-add:hover:not(:disabled) {
    border-color: color-mix(in srgb, var(--accent) 35%, transparent);
    color: var(--accent);
  }

  .radio-add {
    color: color-mix(in srgb, var(--accent) 70%, var(--faint));
  }

  .radio-button {
    border-color: color-mix(in srgb, var(--accent) 26%, var(--line-strong));
    color: color-mix(in srgb, var(--accent) 78%, var(--muted));
  }

  .empty-copy {
    margin: 8px 0;
    padding: 14px;
    border: 1px dashed var(--line);
    border-radius: 5px;
    color: var(--faint);
    font-size: 8px;
    text-align: center;
  }

  .empty-state,
  .pairing-state {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
    max-width: 330px;
    margin: 10px auto;
  }

  .empty-state h3,
  .pairing-state h3 {
    margin: 0;
    font-size: 13px;
    font-weight: 600;
  }

  .empty-state p,
  .pairing-state p {
    margin: 0;
    color: var(--muted);
    font-size: 8.5px;
    line-height: 1.45;
  }

  .empty-glyph {
    display: grid;
    width: 34px;
    height: 34px;
    place-items: center;
    border: 1px solid var(--line);
    border-radius: 7px;
    color: var(--accent);
  }

  .pair-code {
    color: var(--accent) !important;
    font-family: "JetBrains Mono", monospace;
    font-size: 20px !important;
    font-weight: 600;
    letter-spacing: 0.16em;
  }

  .pairing-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 5px;
  }

  .path {
    max-width: 100%;
    overflow: hidden;
    color: var(--faint);
    font-family: "JetBrains Mono", monospace;
    font-size: 6.5px;
    text-overflow: ellipsis;
    white-space: nowrap;
    -webkit-user-select: text;
    user-select: text;
  }

  code {
    padding: 2px 4px;
    border: 1px solid var(--line);
    border-radius: 3px;
    color: var(--muted);
    font-family: "JetBrains Mono", monospace;
    font-size: 7px;
    -webkit-user-select: text;
    user-select: text;
  }

  .library-foot {
    display: flex;
    min-height: 23px;
    flex: 0 0 auto;
    align-items: center;
    justify-content: space-between;
    gap: 7px;
    padding: 3px 12px;
    border-top: 1px solid var(--line);
    color: var(--faint);
    font-size: 6.5px;
  }

  .foot-button {
    flex: 0 0 auto;
    padding: 2px 5px;
    border: 1px solid var(--line);
    border-radius: 3px;
    color: var(--muted);
    font-size: 6.5px;
  }
</style>
