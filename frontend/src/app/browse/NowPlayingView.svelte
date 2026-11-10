<script lang="ts">
  import { Status } from "../../shared/api/types";
  import {
    lyrics as lyricsApi,
    toApiError,
    type LyricsResult,
    type LyricsTrack,
  } from "../../shared/api";
  import { t } from "../../shared/i18n";
  import { coverURL } from "../../lib/artwork";
  import CoverArt from "../../lib/components/CoverArt.svelte";
  import Progress from "../../lib/components/Progress.svelte";
  import Transport from "../../lib/components/Transport.svelte";
  import Volume from "../../lib/components/Volume.svelte";
  import { player } from "../../lib/player.svelte";
  import { preferences } from "../../lib/preferences.svelte";
  import { browse } from "./browse.svelte";
  import Icon from "./Icon.svelte";
  import { activeLyricIndex } from "./lyrics";

  interface Props {
    lyricsLoader?: (track: LyricsTrack) => Promise<LyricsResult>;
  }

  let { lyricsLoader }: Props = $props();

  let result = $state<LyricsResult | null>(null);
  let loading = $state(false);
  let error = $state<string | null>(null);
  let loadedKey = "";
  let requestId = 0;
  let lyricsEl = $state<HTMLDivElement | null>(null);
  let lastScrolled = -1;
  let positionTick = $state(0);

  let track = $derived(player.state.track);
  let playing = $derived(player.state.status === Status.StatusPlaying);
  let artists = $derived((track.artists ?? []).join(", ") || t("player.unknownArtist"));
  let lines = $derived(result?.lines ?? []);
  let hasSynced = $derived(Boolean(result?.synced && lines.some((line) => line.timeMs >= 0)));
  let coverSrc = $derived(track.cover.startsWith("/art/") ? coverURL(track.cover, 240) : "");
  let stageAmbience = $derived(
    coverSrc
      ? `background-image: url("${coverSrc}"); background-size: cover; background-position: center;`
      : "background: radial-gradient(70% 90% at 20% 12%, color-mix(in srgb, var(--accent) 34%, transparent), transparent 66%), radial-gradient(60% 80% at 86% 84%, color-mix(in srgb, var(--accent-2) 24%, transparent), transparent 68%);",
  );

  let activeIndex = $derived(hasSynced ? activeLyricIndex(lines, positionTick) : -1);

  $effect(() => {
    const current = player.state.track;
    const key = `${current.uri}|${current.title}|${current.album}|${current.durationMs}`;
    if (!current.title) {
      result = null;
      loading = false;
      error = null;
      loadedKey = "";
      return;
    }
    if (key === loadedKey) return;
    void loadLyrics(current, key);
  });

  $effect(() => {
    const isPlaying = player.state.status === Status.StatusPlaying;
    positionTick = player.positionMs;
    if (!isPlaying) return;
    const timer = setInterval(() => {
      positionTick = player.positionMs;
    }, 200);
    return () => clearInterval(timer);
  });

  $effect(() => {
    const index = activeIndex;
    if (index < 0 || index === lastScrolled || !lyricsEl) return;
    lastScrolled = index;
    const node = lyricsEl.querySelector<HTMLElement>(`[data-line="${index}"]`);
    if (!node) return;
    const top = node.offsetTop - lyricsEl.clientHeight / 2 + node.offsetHeight / 2;
    lyricsEl.scrollTo({
      top: Math.max(0, top),
      behavior: preferences.reducedMotion ? "auto" : "smooth",
    });
  });

  async function loadLyrics(current: typeof track, key: string): Promise<void> {
    const request = ++requestId;
    loadedKey = key;
    loading = true;
    error = null;
    result = null;
    lastScrolled = -1;
    lyricsEl?.scrollTo({ top: 0 });
    try {
      const load = lyricsLoader ?? lyricsApi.get;
      const next = await load({
        title: current.title,
        artists: current.artists ?? [],
        album: current.album,
        durationMs: current.durationMs,
        uri: current.uri,
      });
      if (request !== requestId) return;
      result = next;
    } catch (cause) {
      if (request !== requestId) return;
      error = toApiError(cause).message;
      loadedKey = "";
    } finally {
      if (request === requestId) loading = false;
    }
  }

  function retry(): void {
    if (!track.title) return;
    loadedKey = "";
    void loadLyrics(track, `${track.uri}|${track.title}|${track.album}|${track.durationMs}`);
  }

  function back(): void {
    browse.setSection("library");
  }

  function seekTo(timeMs: number): void {
    if (!player.state.canSeek || timeMs < 0) return;
    void player.seek(timeMs);
  }

  function lineTime(timeMs: number): string {
    const seconds = Math.max(0, Math.floor(timeMs / 1000));
    const minutes = Math.floor(seconds / 60);
    return `${minutes}:${String(seconds % 60).padStart(2, "0")}`;
  }
</script>

<section class="now-stage" aria-label={t("lyrics.title")}>
  <div class="stage-ambience" style={stageAmbience} aria-hidden="true"></div>
  <div class="stage-scrim" aria-hidden="true"></div>

  <div class="stage-content">
    <header class="stage-head">
      <button class="stage-back" type="button" onclick={back}>
        <Icon name="back" size={14} />
        <span>{t("lyrics.backToLibrary")}</span>
      </button>
      <span class="stage-badge">
        <span class="stage-badge-dot" aria-hidden="true"></span>
        {t("browse.nowPlaying")}
      </span>
    </header>

    <div class="stage-grid">
      <aside class="stage-track">
        <div class="stage-cover-wrap" class:is-playing={playing}>
          <span class="stage-cover-halo" aria-hidden="true"></span>
          <span class="stage-cover-ring" aria-hidden="true"></span>
          <CoverArt cover={track.cover} title={track.title} size={300} />
          <span class="stage-cover-floor" aria-hidden="true"></span>
        </div>

        <div class="stage-copy">
          <h1 class="stage-title" title={track.title || t("browse.nothingPlaying")}>
            {track.title || t("browse.nothingPlaying")}
          </h1>
          <p class="stage-artists" title={artists}>{artists}</p>
          {#if track.album}
            <p class="stage-album" title={track.album}>{track.album}</p>
          {/if}
        </div>

        <div class="stage-player">
          <Progress
            positionMs={player.state.progressMs}
            durationMs={player.state.durationMs}
            {playing}
            canSeek={player.state.canSeek}
            onSeek={(positionMs) => void player.seek(positionMs)}
            trackId={track.id}
          />
          <div class="stage-controls">
            <Transport
              {playing}
              shuffle={player.state.shuffle}
              repeat={player.state.repeat}
              canControl={player.state.canControl}
              onToggle={() => void player.toggle()}
              onPrevious={() => void player.previous()}
              onNext={() => void player.next()}
              onShuffle={() => void player.toggleShuffle()}
              onRepeat={() => void player.cycleRepeat()}
            />
            <Volume
              volume={player.state.volume}
              canControl={player.state.canControl}
              onVolume={(volume) => void player.setVolume(volume)}
            />
          </div>
        </div>
      </aside>

      <section class="lyrics-panel" aria-label={t("lyrics.title")}>
        <header class="lyrics-head">
          <span class="eyebrow">{t("lyrics.title")}</span>
          <span class="lyrics-source">{result?.available ? t("lyrics.source") : ""}</span>
        </header>

        {#if !track.title}
          <div class="lyrics-state">
            <span class="lyrics-state-icon" aria-hidden="true"><Icon name="note" size={22} /></span>
            <h2>{t("lyrics.empty")}</h2>
            <p>{t("lyrics.emptyHint")}</p>
          </div>
        {:else if loading}
          <div class="lyrics-loading" aria-live="polite">
            <span class="lyrics-state-icon is-loading" aria-hidden="true">
              <Icon name="note" size={22} />
            </span>
            <p>{t("lyrics.loading")}</p>
            <span class="lyrics-loading-bar" aria-hidden="true"></span>
          </div>
        {:else if error}
          <div class="lyrics-state">
            <span class="lyrics-state-icon is-error" aria-hidden="true">
              <Icon name="close" size={22} />
            </span>
            <h2>{t("lyrics.error")}</h2>
            <p>{error}</p>
            <button class="lyrics-retry" type="button" onclick={retry}>{t("lyrics.retry")}</button>
          </div>
        {:else if result?.instrumental}
          <div class="lyrics-state">
            <span class="lyrics-state-icon" aria-hidden="true"><Icon name="radio" size={22} /></span
            >
            <h2>{t("lyrics.instrumental")}</h2>
            <p>{t("lyrics.instrumentalHint")}</p>
          </div>
        {:else if !result?.available || lines.length === 0}
          <div class="lyrics-state">
            <span class="lyrics-state-icon" aria-hidden="true"><Icon name="note" size={22} /></span>
            <h2>{t("lyrics.unavailable")}</h2>
            <p>{t("lyrics.unavailableHint")}</p>
            <button class="lyrics-retry" type="button" onclick={retry}>{t("lyrics.retry")}</button>
          </div>
        {:else}
          {#if !hasSynced}
            <p class="lyrics-plain-note">{t("lyrics.unsynced")}</p>
          {/if}
          <div class="lyrics-scroll" bind:this={lyricsEl} class:is-synced={hasSynced}>
            {#each lines as line, index (index)}
              {#if line.timeMs >= 0}
                <button
                  class="lyric-line"
                  class:is-active={index === activeIndex}
                  class:is-past={index < activeIndex}
                  type="button"
                  data-line={index}
                  aria-label={t("lyrics.seekTo", { time: lineTime(line.timeMs) })}
                  onclick={() => seekTo(line.timeMs)}
                >
                  {line.text}
                </button>
              {:else}
                <p class="lyric-line is-plain" data-line={index}>{line.text}</p>
              {/if}
            {/each}
            <span class="lyrics-end" aria-hidden="true"></span>
          </div>
        {/if}
      </section>
    </div>
  </div>
</section>

<style>
  .now-stage {
    position: relative;
    display: flex;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    isolation: isolate;
    background: color-mix(in srgb, var(--bg) 78%, transparent);
    animation: stage-in 460ms var(--ease-out, cubic-bezier(0.16, 1, 0.3, 1)) both;
  }

  .stage-ambience {
    position: absolute;
    z-index: 0;
    inset: -22%;
    filter: blur(54px) saturate(1.35) brightness(0.82);
    opacity: 0.42;
    pointer-events: none;
    transform: scale(1.12);
    animation: stage-drift 20s var(--ease-in-out, ease-in-out) infinite alternate;
  }

  .stage-scrim {
    position: absolute;
    z-index: 0;
    inset: 0;
    background:
      linear-gradient(
        180deg,
        color-mix(in srgb, var(--bg) 40%, transparent),
        color-mix(in srgb, var(--bg) 12%, transparent) 38%,
        color-mix(in srgb, var(--bg) 70%, transparent) 100%
      ),
      linear-gradient(90deg, color-mix(in srgb, var(--bg) 55%, transparent), transparent 48%);
    pointer-events: none;
  }

  .stage-content {
    position: relative;
    z-index: 1;
    display: flex;
    width: 100%;
    min-height: 0;
    flex-direction: column;
    padding: 8px 26px 22px;
  }

  .stage-head {
    display: flex;
    flex: 0 0 auto;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 2px 0 12px;
  }

  .stage-back {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 5px 9px;
    border: 1px solid transparent;
    border-radius: 7px;
    color: var(--muted);
    font-size: 11px;
    transition:
      color var(--motion-fast, 140ms) ease,
      border-color var(--motion-fast, 140ms) ease,
      background-color var(--motion-fast, 140ms) ease,
      transform var(--motion-fast, 140ms) var(--ease-spring, ease);
  }

  .stage-back:hover {
    border-color: var(--line);
    background: color-mix(in srgb, var(--surface-2) 62%, transparent);
    color: var(--text);
    transform: translateX(-2px);
  }

  .stage-badge {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 8px;
    border: 1px solid color-mix(in srgb, var(--accent) 24%, var(--line));
    border-radius: 999px;
    background: color-mix(in srgb, var(--accent) 7%, transparent);
    color: var(--accent);
    font-family: var(--font-mono);
    font-size: 8px;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  .stage-badge-dot {
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: currentColor;
    box-shadow: 0 0 7px currentColor;
    animation: stage-pulse 2.2s ease-in-out infinite;
  }

  .stage-grid {
    display: grid;
    flex: 1 1 auto;
    min-height: 0;
    grid-template-columns: minmax(270px, 33%) minmax(0, 1fr);
    gap: clamp(24px, 4vw, 58px);
    align-items: center;
    padding: 0 clamp(0px, 2vw, 26px) 4px;
  }

  .stage-track {
    display: flex;
    min-height: 0;
    flex-direction: column;
    gap: 14px;
    padding: 8px 0;
  }

  .stage-cover-wrap {
    position: relative;
    width: min(100%, 300px);
    aspect-ratio: 1;
    align-self: center;
  }

  .stage-cover-wrap :global(.cover-art) {
    width: 100% !important;
    height: 100% !important;
    border-radius: 16px;
  }

  .stage-cover-halo {
    position: absolute;
    z-index: -1;
    inset: -18%;
    border-radius: 50%;
    background: radial-gradient(
      closest-side,
      color-mix(in srgb, var(--accent) 30%, transparent),
      transparent 74%
    );
    filter: blur(26px);
    opacity: 0.62;
    pointer-events: none;
    animation: stage-halo 5.6s ease-in-out infinite;
  }

  .stage-cover-ring {
    position: absolute;
    z-index: 1;
    inset: -7px;
    border: 1px solid color-mix(in srgb, var(--accent) 26%, transparent);
    border-radius: 20px;
    pointer-events: none;
  }

  .stage-cover-ring::after {
    content: "";
    position: absolute;
    inset: -1px;
    border-radius: inherit;
    background: conic-gradient(
      from 0deg,
      transparent 0deg,
      color-mix(in srgb, var(--accent) 62%, transparent) 58deg,
      transparent 128deg,
      transparent 248deg,
      color-mix(in srgb, var(--accent-2) 44%, transparent) 304deg,
      transparent 360deg
    );
    opacity: 0.7;
    filter: blur(2px);
    animation: stage-orbit 9s linear infinite;
  }

  .stage-cover-floor {
    position: absolute;
    z-index: -1;
    right: 8%;
    bottom: -22px;
    left: 8%;
    height: 18px;
    border-radius: 50%;
    background: radial-gradient(
      closest-side,
      color-mix(in srgb, var(--accent) 40%, transparent),
      transparent 74%
    );
    filter: blur(10px);
    opacity: 0.45;
    pointer-events: none;
  }

  .stage-copy {
    display: flex;
    min-width: 0;
    flex-direction: column;
    align-items: center;
    text-align: center;
    animation: stage-copy-in 420ms var(--ease-out, ease) both;
  }

  .stage-title {
    overflow: hidden;
    max-width: 100%;
    margin: 0;
    color: var(--text);
    font-size: clamp(21px, 2.6vw, 30px);
    font-weight: 680;
    letter-spacing: -0.045em;
    line-height: 1.05;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-shadow: 0 0 30px color-mix(in srgb, var(--accent) 18%, transparent);
  }

  .stage-artists {
    overflow: hidden;
    max-width: 100%;
    margin: 6px 0 0;
    color: color-mix(in srgb, var(--text) 70%, var(--muted));
    font-size: 13px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .stage-album {
    overflow: hidden;
    max-width: 100%;
    margin: 3px 0 0;
    color: var(--faint);
    font-size: 10px;
    letter-spacing: 0.04em;
    text-overflow: ellipsis;
    text-transform: uppercase;
    white-space: nowrap;
  }

  .stage-player {
    display: flex;
    width: 100%;
    flex-direction: column;
    gap: 5px;
    margin-top: 2px;
    padding: 10px 12px;
    border: 1px solid var(--line);
    border-radius: 13px;
    background:
      linear-gradient(180deg, rgb(255 255 255 / 5%), transparent 48%),
      color-mix(in srgb, var(--surface) 42%, transparent);
    box-shadow:
      0 1px 0 rgb(255 255 255 / 6%) inset,
      0 14px 34px rgb(0 0 0 / 26%);
    backdrop-filter: blur(16px);
  }

  .stage-controls {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: 10px;
    padding-top: 2px;
  }

  .stage-controls :global(.transport) {
    justify-self: center;
  }

  .lyrics-panel {
    display: flex;
    height: min(100%, 560px);
    min-height: 0;
    flex-direction: column;
    overflow: hidden;
    padding: 6px 8px 0;
    border-left: 1px solid color-mix(in srgb, var(--line) 72%, transparent);
    animation: stage-copy-in 480ms 60ms var(--ease-out, ease) both;
  }

  .lyrics-head {
    display: flex;
    flex: 0 0 auto;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 0 12px 12px;
    border-bottom: 1px solid var(--line);
  }

  .lyrics-head .eyebrow {
    font-size: 9px;
    letter-spacing: 0.22em;
  }

  .lyrics-source {
    color: var(--faint);
    font-family: var(--font-mono);
    font-size: 8px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .lyrics-scroll {
    position: relative;
    flex: 1 1 auto;
    min-height: 0;
    overflow-y: auto;
    padding: 20px 12px 38%;
    scrollbar-width: none;
    mask-image: linear-gradient(180deg, transparent 0, black 12%, black 86%, transparent 100%);
    -webkit-mask-image: linear-gradient(
      180deg,
      transparent 0,
      black 12%,
      black 86%,
      transparent 100%
    );
  }

  .lyrics-scroll::-webkit-scrollbar {
    display: none;
  }

  .lyrics-scroll.is-synced {
    padding-top: 28%;
    padding-bottom: 38%;
  }

  .lyric-line {
    display: block;
    width: 100%;
    margin: 0;
    padding: 8px 4px;
    border: 0;
    background: none;
    color: color-mix(in srgb, var(--text) 36%, var(--faint));
    font-family: var(--font-ui);
    font-size: clamp(15px, 1.55vw, 19px);
    font-weight: 560;
    letter-spacing: -0.02em;
    line-height: 1.35;
    text-align: left;
    text-wrap: balance;
    transition:
      color 260ms ease,
      opacity 260ms ease,
      transform 320ms var(--ease-spring, ease),
      filter 260ms ease;
  }

  button.lyric-line {
    cursor: pointer;
  }

  button.lyric-line:hover {
    color: color-mix(in srgb, var(--text) 68%, var(--muted));
    transform: translateX(3px);
  }

  .lyric-line.is-past {
    color: color-mix(in srgb, var(--text) 30%, var(--faint));
  }

  .lyric-line.is-active {
    color: var(--text);
    text-shadow:
      0 0 26px color-mix(in srgb, var(--accent) 34%, transparent),
      0 0 8px color-mix(in srgb, var(--accent) 18%, transparent);
    transform: translateX(6px) scale(1.015);
  }

  .lyric-line.is-plain {
    color: color-mix(in srgb, var(--text) 62%, var(--muted));
    font-weight: 480;
    line-height: 1.55;
  }

  .lyrics-plain-note {
    margin: 0;
    padding: 12px 12px 0;
    color: var(--faint);
    font-family: var(--font-mono);
    font-size: 8px;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  .lyrics-state {
    display: flex;
    flex: 1 1 auto;
    min-height: 0;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 9px;
    padding: 28px;
    text-align: center;
    animation: stage-copy-in 360ms var(--ease-out, ease) both;
  }

  .lyrics-state-icon {
    display: grid;
    width: 42px;
    height: 42px;
    place-items: center;
    border: 1px solid var(--line);
    border-radius: 12px;
    background: color-mix(in srgb, var(--surface-2) 52%, transparent);
    color: var(--accent);
    box-shadow:
      0 0 0 4px color-mix(in srgb, var(--accent) 4%, transparent),
      0 0 24px color-mix(in srgb, var(--accent) 10%, transparent);
  }

  .lyrics-state-icon.is-error {
    color: var(--danger);
    box-shadow:
      0 0 0 4px color-mix(in srgb, var(--danger) 4%, transparent),
      0 0 24px color-mix(in srgb, var(--danger) 10%, transparent);
  }

  .lyrics-state-icon.is-loading {
    animation: stage-loading 1.8s ease-in-out infinite;
  }

  .lyrics-state h2 {
    margin: 0;
    color: var(--text);
    font-size: 17px;
    font-weight: 620;
    letter-spacing: -0.02em;
  }

  .lyrics-state p {
    max-width: 360px;
    margin: 0;
    color: var(--faint);
    font-size: 12px;
    line-height: 1.55;
  }

  .lyrics-loading {
    display: flex;
    flex: 1 1 auto;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    color: var(--faint);
    font-size: 12px;
  }

  .lyrics-loading p {
    margin: 0;
  }

  .lyrics-loading-bar {
    width: 150px;
    height: 2px;
    overflow: hidden;
    border-radius: 999px;
    background: var(--line-strong);
  }

  .lyrics-loading-bar::after {
    content: "";
    display: block;
    width: 38%;
    height: 100%;
    border-radius: inherit;
    background: linear-gradient(90deg, transparent, var(--accent), transparent);
    animation: stage-loading-bar 1.4s ease-in-out infinite;
  }

  .lyrics-retry {
    margin-top: 4px;
    padding: 6px 12px;
    border: 1px solid color-mix(in srgb, var(--accent) 30%, var(--line));
    border-radius: 7px;
    background: color-mix(in srgb, var(--accent) 8%, transparent);
    color: var(--text);
    font-size: 11px;
    transition:
      border-color var(--motion-fast, 140ms) ease,
      background-color var(--motion-fast, 140ms) ease,
      transform var(--motion-fast, 140ms) var(--ease-spring, ease);
  }

  .lyrics-retry:hover {
    border-color: color-mix(in srgb, var(--accent) 52%, var(--line));
    background: color-mix(in srgb, var(--accent) 14%, transparent);
    transform: translateY(-1px);
  }

  .lyrics-end {
    display: block;
    height: 20px;
  }

  @keyframes stage-in {
    from {
      opacity: 0;
      transform: scale(0.992);
    }
    to {
      opacity: 1;
      transform: scale(1);
    }
  }

  @keyframes stage-drift {
    from {
      transform: scale(1.1) translate3d(-1%, -1%, 0);
    }
    to {
      transform: scale(1.16) translate3d(1.5%, 1.5%, 0);
    }
  }

  @keyframes stage-copy-in {
    from {
      opacity: 0;
      transform: translateY(8px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  @keyframes stage-halo {
    0%,
    100% {
      opacity: 0.45;
      transform: scale(0.97);
    }
    50% {
      opacity: 0.72;
      transform: scale(1.04);
    }
  }

  @keyframes stage-orbit {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }

  @keyframes stage-pulse {
    0%,
    100% {
      opacity: 0.5;
    }
    50% {
      opacity: 1;
    }
  }

  @keyframes stage-loading {
    0%,
    100% {
      opacity: 0.55;
      transform: scale(0.96);
    }
    50% {
      opacity: 1;
      transform: scale(1.03);
    }
  }

  @keyframes stage-loading-bar {
    from {
      transform: translateX(-120%);
    }
    to {
      transform: translateX(320%);
    }
  }

  @media (max-width: 900px) {
    .stage-content {
      padding-inline: 16px;
    }

    .stage-grid {
      grid-template-columns: minmax(230px, 38%) minmax(0, 1fr);
      gap: 22px;
      padding-inline: 0;
    }

    .stage-cover-wrap {
      width: min(100%, 230px);
    }

    .lyrics-panel {
      padding-inline: 2px;
    }
  }

  @media (max-width: 720px) {
    .stage-grid {
      display: flex;
      flex-direction: column;
      align-items: stretch;
      gap: 12px;
      overflow-y: auto;
    }

    .stage-track {
      flex: 0 0 auto;
    }

    .stage-cover-wrap {
      width: 170px;
    }

    .lyrics-panel {
      min-height: 260px;
      border-left: 0;
      border-top: 1px solid var(--line);
    }
  }
</style>
