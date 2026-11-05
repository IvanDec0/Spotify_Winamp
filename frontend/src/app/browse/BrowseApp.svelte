<script lang="ts">
  import { onMount } from "svelte";
  import { Status } from "../../shared/api/types";
  import { t } from "../../shared/i18n";
  import Button from "../../shared/ui/Button.svelte";
  import { coverURL } from "../../lib/artwork";
  import { formatTime } from "../../lib/format";
  import { player } from "../../lib/player.svelte";
  import { isThemeId, preferences } from "../../lib/preferences.svelte";
  import { skins } from "../../lib/skins.svelte";
  import { applyTokens, readSkinTokens, syncBrowseStylesheet } from "./theme.svelte";

  type Section = "search" | "library" | "podcasts" | "settings";

  const ICONS = {
    search: "M11 18a7 7 0 1 1 0-14 7 7 0 0 1 0 14Zm5-2 5 5",
    library: "M4 5h3v14H4zM9 5h3v14H9zM15 6l4 12",
    podcasts:
      "M12 4a3 3 0 0 1 3 3v5a3 3 0 0 1-6 0V7a3 3 0 0 1 3-3ZM5.5 10.5a6.5 6.5 0 0 0 13 0M12 17v3",
    settings:
      "M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Zm7.4-3a7.4 7.4 0 0 0-.1-1.1l2-1.6-2-3.4-2.4 1a7.6 7.6 0 0 0-1.9-1.1L14.6 3h-4l-.4 2.8a7.6 7.6 0 0 0-1.9 1.1l-2.4-1-2 3.4 2 1.6A7.4 7.4 0 0 0 5.8 12c0 .4 0 .7.1 1.1l-2 1.6 2 3.4 2.4-1a7.6 7.6 0 0 0 1.9 1.1l.4 2.8h4l.4-2.8a7.6 7.6 0 0 0 1.9-1.1l2.4 1 2-3.4-2-1.6c.1-.4.1-.7.1-1.1Z",
    play: "M8 5.5 18 12 8 18.5Z",
    pause: "M9 5v14M15 5v14",
    previous: "M18 5.5 9 12l9 6.5ZM6 5v14",
    next: "M6 5.5 15 12l-9 6.5ZM18 5v14",
    volume: "M4 9.5h3L11 6v12l-4-3.5H4ZM15 9a4 4 0 0 1 0 6M17.5 6.5a7 7 0 0 1 0 11",
    note: "M9 18V5l10-2v13M9 18a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm10-2a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z",
  } as const;

  const SECTIONS = {
    search: {
      nav: "browse.search",
      title: "browse.searchTitle",
      body: "browse.searchBody",
      glyph: ICONS.search,
    },
    library: {
      nav: "browse.library",
      title: "browse.libraryTitle",
      body: "browse.libraryBody",
      glyph: ICONS.library,
    },
    podcasts: {
      nav: "browse.podcasts",
      title: "browse.podcastsTitle",
      body: "browse.podcastsBody",
      glyph: ICONS.podcasts,
    },
    settings: {
      nav: "browse.settings",
      title: "browse.settingsTitle",
      body: "browse.settingsBody",
      glyph: ICONS.settings,
    },
  } as const;

  let section = $state<Section>("search");
  let root: HTMLDivElement;
  let themeRequest = 0;
  let positionMs = $state(0);

  let navItems = $derived(
    (["search", "library", "podcasts", "settings"] as const).map((id) => ({
      id,
      label: t(SECTIONS[id].nav),
      glyph: SECTIONS[id].glyph,
    })),
  );
  let activeSection = $derived(SECTIONS[section]);
  let skinLabel = $derived.by(() => {
    const runtime = skins.runtime;
    if (!runtime || runtime.builtin) {
      return t("browse.skinTheme", { skin: "Clarity", theme: preferences.theme });
    }
    return t("browse.skinTheme", { skin: runtime.name, theme: skins.theme });
  });
  let track = $derived(player.state.track);
  let durationMs = $derived(player.state.durationMs);
  let playing = $derived(player.state.status === Status.StatusPlaying);
  let coverSrc = $derived(track.cover.startsWith("/art/") ? coverURL(track.cover, 48) : "");
  let artistLabel = $derived(
    (track.artists ?? []).join(", ") || track.album || t("browse.nothingPlaying"),
  );
  let progressPercent = $derived(
    durationMs > 0 ? Math.min(100, Math.max(0, (positionMs / durationMs) * 100)) : 0,
  );

  $effect(() => {
    const runtime = skins.runtime;
    const theme = runtime && !runtime.builtin ? skins.theme : preferences.theme;
    const target = root;
    if (!target) return;

    const request = ++themeRequest;
    let active = true;
    void readSkinTokens(runtime, theme).then((tokens) => {
      if (!active || request !== themeRequest) return;
      applyTokens(target, tokens);
    });
    return () => {
      active = false;
    };
  });

  $effect(() => {
    const runtime = skins.runtime;
    const version = runtime ? `${runtime.id}@${runtime.version}` : "";
    syncBrowseStylesheet(
      runtime && !runtime.builtin ? (runtime.browseStylesheetUrl ?? "") : "",
      version,
    );
  });

  $effect(() => {
    const isPlaying = player.state.status === Status.StatusPlaying;
    positionMs = player.positionMs;
    if (!isPlaying) return;
    const timer = setInterval(() => {
      positionMs = player.positionMs;
    }, 250);
    return () => clearInterval(timer);
  });

  onMount(() => {
    const onStorage = (event: StorageEvent): void => {
      if (event.key !== "cadence.theme") return;
      if (isThemeId(event.newValue)) preferences.syncTheme(event.newValue);
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  });

  function setVolume(event: Event): void {
    const input = event.currentTarget as HTMLInputElement;
    void player.setVolume(input.valueAsNumber);
  }
</script>

<div class="cadence-browse" bind:this={root}>
  <aside class="sidebar">
    <div class="sidebar-head">
      <span class="eyebrow">{t("browse.open")}</span>
      <span class="sidebar-caption">{t("browse.navLabel")}</span>
    </div>
    <nav class="nav" aria-label={t("browse.navLabel")}>
      {#each navItems as item (item.id)}
        <button
          class="nav-item"
          class:is-active={section === item.id}
          type="button"
          aria-current={section === item.id ? "page" : undefined}
          onclick={() => {
            section = item.id;
          }}
        >
          <svg
            class="nav-icon"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.6"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <path d={item.glyph} />
          </svg>
          <span>{item.label}</span>
        </button>
      {/each}
    </nav>
    <div class="sidebar-foot">
      <span class="status-dot" aria-hidden="true"></span>
      <span>{skins.loading ? t("ui.loading") : t("browse.catalogStatus")}</span>
    </div>
  </aside>

  <header class="topbar">
    <div class="wordmark">
      <strong>Cadence</strong>
      <span>Browse</span>
    </div>
    <span class="theme-chip" title={skinLabel}>{skinLabel}</span>
  </header>

  <main class="content">
    <section class="welcome" aria-labelledby="browse-section-title">
      <p class="eyebrow">{t(activeSection.nav)}</p>
      <h1 id="browse-section-title">{t(activeSection.title)}</h1>
      <p class="lede">{t(activeSection.body)}</p>

      <div class="cards">
        <article class="card">
          <h2>{t("browse.search")}</h2>
          <p>{t("browse.searchBody")}</p>
        </article>
        <article class="card">
          <h2>{t("browse.library")}</h2>
          <p>{t("browse.libraryBody")}</p>
        </article>
        <article class="card">
          <h2>{t("browse.podcasts")}</h2>
          <p>{t("browse.podcastsBody")}</p>
        </article>
      </div>

      <p class="placeholder-note">{t("browse.welcomeBody")}</p>
    </section>
  </main>

  <footer class="now-playing" aria-label={t("browse.nowPlaying")}>
    <div class="track">
      <div class="cover" aria-hidden="true">
        {#if coverSrc}
          <img src={coverSrc} alt="" width="48" height="48" loading="eager" decoding="async" />
        {:else}
          <svg
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.4"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d={ICONS.note} />
          </svg>
        {/if}
      </div>
      <div class="track-copy">
        <span class="track-title">{track.title || t("browse.nothingPlaying")}</span>
        <span class="track-meta">{artistLabel}</span>
      </div>
    </div>

    <div class="transport">
      <Button
        variant="ghost"
        size="sm"
        ariaLabel={t("browse.previous")}
        disabled={!player.state.canControl}
        onclick={() => void player.previous()}
      >
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d={ICONS.previous} />
        </svg>
      </Button>
      <Button
        variant="primary"
        size="lg"
        ariaLabel={playing ? t("browse.pause") : t("browse.play")}
        disabled={!player.state.canControl}
        onclick={() => void player.toggle()}
      >
        <svg
          viewBox="0 0 24 24"
          width="16"
          height="16"
          fill={playing ? "none" : "currentColor"}
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
          aria-hidden="true"
        >
          <path d={playing ? ICONS.pause : ICONS.play} />
        </svg>
      </Button>
      <Button
        variant="ghost"
        size="sm"
        ariaLabel={t("browse.next")}
        disabled={!player.state.canControl}
        onclick={() => void player.next()}
      >
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d={ICONS.next} />
        </svg>
      </Button>
    </div>

    <div class="progress" role="group" aria-label={t("browse.progress")}>
      <span class="time">{formatTime(positionMs / 1000)}</span>
      <span
        class="progress-rail"
        role="progressbar"
        aria-valuemin="0"
        aria-valuemax={Math.max(0, Math.round(durationMs))}
        aria-valuenow={Math.max(0, Math.round(positionMs))}
      >
        <span class="progress-fill" style={`width: ${progressPercent}%`}></span>
      </span>
      <span class="time">{formatTime(durationMs / 1000)}</span>
    </div>

    <div class="volume">
      <svg
        class="volume-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.6"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d={ICONS.volume} />
      </svg>
      <input
        type="range"
        min="0"
        max="100"
        step="1"
        value={player.state.volume}
        disabled={!player.state.canControl}
        aria-label={t("browse.volume")}
        style={`--volume: ${player.state.volume}%`}
        oninput={setVolume}
      />
    </div>
  </footer>
</div>

<style>
  .cadence-browse {
    display: grid;
    height: 100%;
    min-height: 0;
    grid-template-columns: 200px minmax(0, 1fr);
    grid-template-rows: auto minmax(0, 1fr) auto;
    overflow: hidden;
    background:
      radial-gradient(
        80% 60% at 0% 0%,
        color-mix(in srgb, var(--accent) 7%, transparent),
        transparent 55%
      ),
      var(--bg);
    color: var(--text);
    font-family: var(--font-ui);
  }

  .sidebar {
    display: flex;
    min-height: 0;
    flex-direction: column;
    grid-row: 1 / 4;
    gap: 18px;
    padding: 20px 14px 16px;
    border-right: 1px solid var(--line);
    background: color-mix(in srgb, var(--surface) 70%, transparent);
  }

  .sidebar-head {
    display: flex;
    flex-direction: column;
    gap: 3px;
    padding: 0 8px;
  }

  .eyebrow {
    margin: 0;
    color: var(--accent);
    font-family: var(--font-mono);
    font-size: 10px;
    letter-spacing: 0.18em;
    text-transform: uppercase;
  }

  .sidebar-caption {
    color: var(--faint);
    font-size: 10px;
  }

  .nav {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .nav-item {
    display: flex;
    width: 100%;
    align-items: center;
    gap: 10px;
    padding: 9px 10px;
    border: 1px solid transparent;
    border-radius: 8px;
    color: var(--muted);
    font-size: 12px;
    text-align: left;
    transition:
      background-color var(--motion-fast, 120ms) ease,
      border-color var(--motion-fast, 120ms) ease,
      color var(--motion-fast, 120ms) ease;
  }

  .nav-item:hover {
    background: color-mix(in srgb, var(--text) 5%, transparent);
    color: var(--text);
  }

  .nav-item.is-active {
    border-color: color-mix(in srgb, var(--accent) 30%, transparent);
    background: color-mix(in srgb, var(--accent) 12%, var(--surface-2));
    color: var(--text);
  }

  .nav-icon {
    width: 16px;
    height: 16px;
    flex: 0 0 16px;
  }

  .sidebar-foot {
    display: flex;
    align-items: center;
    gap: 7px;
    margin-top: auto;
    padding: 8px;
    color: var(--faint);
    font-size: 10px;
  }

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent);
    box-shadow: 0 0 6px color-mix(in srgb, var(--accent) 45%, transparent);
  }

  .topbar {
    display: flex;
    min-width: 0;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    padding: 14px 24px;
    border-bottom: 1px solid var(--line);
    background: color-mix(in srgb, var(--panel) 70%, transparent);
  }

  .wordmark {
    display: flex;
    min-width: 0;
    align-items: baseline;
    gap: 8px;
    font-size: 18px;
    letter-spacing: -0.03em;
  }

  .wordmark strong {
    font-weight: 650;
  }

  .wordmark span {
    color: var(--muted);
    font-weight: 450;
  }

  .theme-chip {
    overflow: hidden;
    flex: 0 0 auto;
    max-width: 45%;
    padding: 4px 9px;
    border: 1px solid var(--line);
    border-radius: 999px;
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: 10px;
    letter-spacing: 0.03em;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .content {
    min-height: 0;
    overflow: auto;
    padding: 40px 44px;
    scrollbar-width: thin;
  }

  .welcome {
    max-width: 760px;
  }

  h1 {
    margin: 8px 0 12px;
    font-size: clamp(26px, 3.6vw, 38px);
    font-weight: 620;
    letter-spacing: -0.045em;
    line-height: 1.05;
  }

  .lede {
    max-width: 580px;
    margin: 0;
    color: var(--muted);
    font-size: 14px;
    line-height: 1.55;
  }

  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
    gap: 12px;
    margin-top: 30px;
  }

  .card {
    padding: 16px;
    border: 1px solid var(--line);
    border-radius: 12px;
    background: color-mix(in srgb, var(--surface) 60%, transparent);
  }

  .card h2 {
    margin: 0 0 6px;
    font-size: 12px;
    font-weight: 600;
  }

  .card p {
    margin: 0;
    color: var(--faint);
    font-size: 11px;
    line-height: 1.5;
  }

  .placeholder-note {
    margin: 26px 0 0;
    color: var(--faint);
    font-size: 11px;
    line-height: 1.5;
  }

  .now-playing {
    display: grid;
    grid-template-columns: minmax(170px, 1fr) auto minmax(150px, 0.9fr) auto;
    align-items: center;
    gap: 22px;
    padding: 14px 24px;
    border-top: 1px solid var(--line);
    background: color-mix(in srgb, var(--panel) 88%, var(--bg));
  }

  .track {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 11px;
  }

  .cover {
    display: grid;
    width: 48px;
    height: 48px;
    flex: 0 0 48px;
    place-items: center;
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: 7px;
    background:
      radial-gradient(
        80% 70% at 70% 20%,
        color-mix(in srgb, var(--accent) 26%, transparent),
        transparent 60%
      ),
      var(--surface-2);
    color: var(--muted);
  }

  .cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .cover svg {
    width: 18px;
    height: 18px;
  }

  .track-copy {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 3px;
  }

  .track-title,
  .track-meta {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .track-title {
    color: var(--text);
    font-size: 12px;
    font-weight: 600;
  }

  .track-meta {
    color: var(--faint);
    font-size: 10px;
  }

  .transport {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .progress {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 9px;
  }

  .time {
    color: var(--faint);
    font-family: var(--font-mono);
    font-size: 10px;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .progress-rail {
    position: relative;
    display: block;
    height: 4px;
    flex: 1 1 auto;
    overflow: hidden;
    border-radius: 999px;
    background: var(--line-strong);
  }

  .progress-fill {
    position: absolute;
    inset: 0 auto 0 0;
    border-radius: 999px;
    background: linear-gradient(
      90deg,
      color-mix(in srgb, var(--accent) 58%, transparent),
      var(--accent)
    );
  }

  .volume {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--muted);
  }

  .volume-icon {
    width: 15px;
    height: 15px;
    flex: 0 0 15px;
  }

  input[type="range"] {
    width: 92px;
    height: 10px;
    margin: 0;
    padding: 0;
    appearance: none;
    background: transparent;
    cursor: pointer;
  }

  input[type="range"]:disabled {
    cursor: default;
    opacity: 0.5;
  }

  input[type="range"]::-webkit-slider-runnable-track {
    height: 3px;
    border-radius: 999px;
    background: linear-gradient(
      90deg,
      color-mix(in srgb, var(--accent) 70%, var(--muted)) var(--volume),
      var(--line-strong) var(--volume)
    );
  }

  input[type="range"]::-moz-range-track {
    height: 3px;
    border-radius: 999px;
    background: linear-gradient(
      90deg,
      color-mix(in srgb, var(--accent) 70%, var(--muted)) var(--volume),
      var(--line-strong) var(--volume)
    );
  }

  input[type="range"]::-webkit-slider-thumb {
    width: 11px;
    height: 11px;
    margin-top: -4px;
    appearance: none;
    border: 0;
    border-radius: 50%;
    background: var(--accent);
    transition: transform var(--motion-fast, 120ms) var(--ease-spring);
  }

  input[type="range"]::-moz-range-thumb {
    width: 11px;
    height: 11px;
    border: 0;
    border-radius: 50%;
    background: var(--accent);
  }

  input[type="range"]:hover::-webkit-slider-thumb {
    transform: scale(1.2);
  }

  @media (max-width: 820px) {
    .cadence-browse {
      grid-template-columns: 164px minmax(0, 1fr);
    }

    .sidebar {
      padding-inline: 11px;
    }

    .content {
      padding: 26px 22px;
    }

    .now-playing {
      grid-template-columns: minmax(0, 1fr) auto;
      gap: 10px 14px;
      padding: 12px 18px;
    }

    .progress {
      grid-column: 1 / -1;
    }

    .volume {
      grid-column: 1 / -1;
      justify-content: flex-end;
    }
  }
</style>
