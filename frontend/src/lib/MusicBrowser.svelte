<script lang="ts">
  import Icon from "./components/Icon.svelte";
  import MediaCover from "./components/MediaCover.svelte";
  import { player } from "./player.svelte";
  import { music } from "./music.svelte";
  import { formatTime } from "./format";

  interface Props {
    onClose: () => void;
  }

  type Tab = "playlists" | "albums" | "artists" | "liked";

  let { onClose }: Props = $props();

  let tab = $state<Tab>("playlists");
  let contextTitle = $state("");
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
    await music.loadContext(uri);
  }

  function close(): void {
    music.clearContext();
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
          <Icon name="chevronDown" size={11} />
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
    {#if !audio.available}
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
            class="primary"
            type="button"
            disabled={music.busy}
            onclick={() => void music.playContext(context.uri || "")}
          >
            Play all
          </button>
          {#if !context.ready}
            <span class="context-status">Loading tracks… {context.cached}/{context.length}</span>
          {/if}
        </div>
        {#if context.items.length === 0}
          <p class="empty-copy">
            {context.ready ? "This context has no tracks." : "Waiting for track metadata."}
          </p>
        {:else}
          <ol class="track-list">
            {#each context.items as item, index (item.uri + ":" + index)}
              <li class="track-row" class:is-current={item.uri === currentTrackURI}>
                <span class="track-index">{String(index + 1).padStart(2, "0")}</span>
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
          </button>
        {/each}
      </nav>

      {#if tab === "playlists"}
        {#if music.playlists.length === 0}
          <p class="empty-copy">{music.busy ? "Loading playlists…" : "No playlists found."}</p>
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
                  <Icon name="chevronDown" size={10} />
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      {:else if tab === "albums"}
        {#if music.albums.length === 0}
          <p class="empty-copy">{music.busy ? "Loading albums…" : "No saved albums found."}</p>
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
                  <Icon name="chevronDown" size={10} />
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      {:else if tab === "artists"}
        {#if music.artists.length === 0}
          <p class="empty-copy">{music.busy ? "Loading artists…" : "No followed artists found."}</p>
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
                  <Icon name="chevronDown" size={10} />
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
          <Icon name="chevronDown" size={10} />
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
    background: var(--bg);
    font-family: "Inter", "Segoe UI", system-ui, sans-serif;
  }

  .library-head {
    display: flex;
    min-height: 34px;
    flex: 0 0 auto;
    align-items: center;
    justify-content: space-between;
    padding: 5px 10px 5px 13px;
    border-bottom: 1px solid var(--line);
    background: color-mix(in srgb, var(--bg) 82%, transparent);
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
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .head-button {
    height: 20px;
    padding: 0 5px;
    border: 1px solid var(--line);
    border-radius: 4px;
    color: var(--muted);
    font-size: 7.5px;
  }

  .head-button:hover {
    border-color: color-mix(in srgb, var(--accent) 35%, transparent);
    color: var(--accent);
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
    display: flex;
    gap: 4px;
    margin-bottom: 6px;
  }

  .tab {
    padding: 3px 6px;
    border: 1px solid transparent;
    border-radius: 3px;
    color: var(--faint);
    font-size: 7.5px;
  }

  .tab:hover {
    color: var(--text);
  }

  .tab.is-active {
    border-color: color-mix(in srgb, var(--accent) 30%, transparent);
    background: color-mix(in srgb, var(--accent) 9%, transparent);
    color: var(--accent);
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
  }

  .library-row:hover {
    background: color-mix(in srgb, var(--accent) 7%, transparent);
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
    transform: rotate(-90deg);
    color: var(--faint);
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
      border-color 120ms ease,
      background-color 120ms ease;
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
  }

  .track-row {
    display: flex;
    min-height: 34px;
    align-items: center;
    gap: 6px;
    border-radius: 4px;
    padding: 2px 4px;
  }

  .track-row:hover,
  .track-row.is-current {
    background: color-mix(in srgb, var(--accent) 8%, transparent);
  }

  .track-row.is-current .track-name {
    color: var(--accent);
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

  .empty-copy {
    margin: 8px 0;
    color: var(--faint);
    font-size: 8px;
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
