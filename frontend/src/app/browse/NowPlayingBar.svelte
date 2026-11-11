<script lang="ts">
  import { RepeatMode, Status } from "../../shared/api/types";
  import { coverURL } from "../../lib/artwork";
  import { formatTime } from "../../lib/format";
  import { player } from "../../lib/player.svelte";
  import { clamp, trackSeed } from "../../lib/visualizer";
  import { t } from "../../shared/i18n";
  import Button from "../../shared/ui/Button.svelte";
  import Waveform from "../../lib/components/Waveform.svelte";
  import BrowseVisualizer from "./BrowseVisualizer.svelte";
  import Icon from "./Icon.svelte";
  import { browse } from "./browse.svelte";

  let positionMs = $state(0);
  let dragging = $state(false);
  let dragValue = $state(0);
  let hoverPercent = $state<number | null>(null);
  let hoverMs = $state(0);
  let lastVolume = $state(player.state.volume > 0 ? player.state.volume : 65);

  let track = $derived(player.state.track);
  let durationMs = $derived(player.state.durationMs);
  let playing = $derived(player.state.status === Status.StatusPlaying);
  let shuffle = $derived(player.state.shuffle);
  let repeat = $derived(player.state.repeat);
  let muted = $derived(player.state.volume <= 0);
  let coverSrc = $derived(track.cover.startsWith("/art/") ? coverURL(track.cover, 48) : "");
  let ambienceSrc = $derived(track.cover.startsWith("/art/") ? coverURL(track.cover, 240) : "");
  let artistLabel = $derived(
    (track.artists ?? []).join(", ") || track.album || t("browse.nothingPlaying"),
  );
  let repeatLabel = $derived(
    repeat === RepeatMode.RepeatTrack
      ? t("transport.repeatTrack")
      : repeat === RepeatMode.RepeatContext
        ? t("transport.repeatContext")
        : t("transport.repeatOff"),
  );
  let shownMs = $derived(dragging ? dragValue : positionMs);
  let progressPercent = $derived(durationMs > 0 ? clamp((shownMs / durationMs) * 100, 0, 100) : 0);
  let waveSeed = $derived(trackSeed(track.id));
  let bubbleVisible = $derived(hoverPercent !== null || dragging);
  let bubblePercent = $derived(dragging ? progressPercent : (hoverPercent ?? 0));
  let bubbleLabel = $derived(formatTime((dragging ? shownMs : hoverMs) / 1000));
  let heartLabel = $derived(browse.currentLiked ? t("browse.unlike") : t("browse.like"));
  let seekLabel = $derived(t("progress.label"));
  let seekValueText = $derived(
    t("progress.value", {
      position: formatTime(shownMs / 1000),
      duration: formatTime(durationMs / 1000),
    }),
  );

  $effect(() => {
    const isPlaying = player.state.status === Status.StatusPlaying;
    positionMs = player.positionMs;
    if (!isPlaying) return;
    const timer = setInterval(() => {
      positionMs = player.positionMs;
    }, 250);
    return () => clearInterval(timer);
  });

  $effect(() => {
    if (player.state.volume > 0) lastVolume = player.state.volume;
  });

  function setVolume(event: Event): void {
    const input = event.currentTarget as HTMLInputElement;
    void player.setVolume(input.valueAsNumber);
  }

  function toggleMute(): void {
    if (!player.state.canControl) return;
    if (player.state.volume > 0) {
      lastVolume = player.state.volume;
      void player.setVolume(0);
    } else {
      void player.setVolume(lastVolume || 65);
    }
  }

  function beginSeek(): void {
    if (!player.state.canSeek) return;
    dragging = true;
    dragValue = positionMs;
  }

  function updateSeek(event: Event): void {
    if (!player.state.canSeek) return;
    const input = event.currentTarget as HTMLInputElement;
    dragging = true;
    dragValue = clamp(input.valueAsNumber, 0, Math.max(0, durationMs));
  }

  function commitSeek(): void {
    if (!player.state.canSeek) {
      dragging = false;
      return;
    }
    const value = Math.round(clamp(dragValue, 0, Math.max(0, durationMs)));
    dragging = false;
    positionMs = value;
    void player.seek(value);
  }

  function handleSeekMove(event: PointerEvent): void {
    const target = event.currentTarget as HTMLElement;
    const rect = target.getBoundingClientRect();
    if (rect.width <= 0) return;
    const ratio = clamp((event.clientX - rect.left) / rect.width, 0, 1);
    hoverPercent = ratio * 100;
    hoverMs = ratio * Math.max(0, durationMs);
  }

  function handleSeekLeave(): void {
    if (!dragging) hoverPercent = null;
  }
</script>

<footer class="now-playing" class:is-playing={playing} aria-label={t("browse.nowPlaying")}>
  {#if ambienceSrc}
    <div
      class="bar-ambience"
      style={`background-image: url("${ambienceSrc}")`}
      aria-hidden="true"
    ></div>
  {/if}
  <div class="visualizer-strip" aria-hidden="true">
    <BrowseVisualizer />
  </div>
  <div class="bar-row">
    <div class="track">
      <button
        class="track-open"
        type="button"
        title={t("player.openNowPlaying")}
        aria-label={t("player.openNowPlaying")}
        onclick={() => browse.setSection("nowplaying")}
      >
        <div class="cover" aria-hidden="true">
          {#if coverSrc}
            <img src={coverSrc} alt="" width="48" height="48" loading="eager" decoding="async" />
          {:else}
            <Icon name="note" size={18} />
          {/if}
          {#if playing}
            <span class="cover-eq" aria-hidden="true"><i></i><i></i><i></i></span>
          {/if}
        </div>
        <div class="track-copy">
          <span class="track-title">{track.title || t("browse.nothingPlaying")}</span>
          <span class="track-meta">{artistLabel}</span>
        </div>
      </button>
      <button
        class="heart-button"
        class:is-liked={browse.currentLiked}
        type="button"
        disabled={!track.uri || browse.likedLoading}
        title={heartLabel}
        aria-label={heartLabel}
        aria-pressed={browse.currentLiked}
        onclick={() => void browse.toggleCurrentLike()}
      >
        <Icon name="heart" size={15} filled={browse.currentLiked} />
      </button>
    </div>

    <div class="transport" role="group" aria-label={t("transport.controls")}>
      <button
        class="bar-button"
        class:is-active={shuffle}
        type="button"
        disabled={!player.state.canControl}
        aria-label={shuffle ? t("transport.shuffleOff") : t("transport.shuffleOn")}
        aria-pressed={shuffle}
        title={t("transport.shuffleShortcut")}
        onclick={() => void player.toggleShuffle()}
      >
        <Icon name="shuffle" size={14} />
      </button>
      <Button
        variant="ghost"
        size="sm"
        ariaLabel={t("browse.previous")}
        title={t("browse.previous")}
        disabled={!player.state.canControl}
        onclick={() => void player.previous()}
      >
        <Icon name="previous" size={14} />
      </Button>
      <Button
        variant="primary"
        size="lg"
        ariaLabel={playing ? t("browse.pause") : t("browse.play")}
        title={playing ? t("browse.pause") : t("browse.play")}
        disabled={!player.state.canControl}
        onclick={() => void player.toggle()}
      >
        <Icon name={playing ? "pause" : "play"} size={16} filled={!playing} />
      </Button>
      <Button
        variant="ghost"
        size="sm"
        ariaLabel={t("browse.next")}
        title={t("browse.next")}
        disabled={!player.state.canControl}
        onclick={() => void player.next()}
      >
        <Icon name="next" size={14} />
      </Button>
      <button
        class="bar-button"
        class:is-active={repeat !== RepeatMode.RepeatOff}
        type="button"
        disabled={!player.state.canControl}
        aria-label={repeatLabel}
        aria-pressed={repeat !== RepeatMode.RepeatOff}
        title={t("transport.repeatShortcut", { label: repeatLabel })}
        onclick={() => void player.cycleRepeat()}
      >
        <Icon name={repeat === RepeatMode.RepeatTrack ? "repeatOne" : "repeat"} size={14} />
      </button>
    </div>

    <div
      class="progress"
      role="group"
      aria-label={seekLabel}
      style={`--progress: ${progressPercent}%`}
    >
      <span class="time">{formatTime(shownMs / 1000)}</span>
      <div
        class="seek-shell"
        role="presentation"
        onpointermove={handleSeekMove}
        onpointerleave={handleSeekLeave}
      >
        <span class="seek-wave">
          <Waveform percent={progressPercent} seed={waveSeed} {playing} count={72} />
        </span>
        <span class="seek-playhead" aria-hidden="true"></span>
        {#if bubbleVisible}
          <span
            class="seek-bubble mono"
            style={`left: clamp(18px, ${bubblePercent}%, calc(100% - 18px))`}
            aria-hidden="true"
          >
            {bubbleLabel}
          </span>
        {/if}
        <input
          class="seek"
          type="range"
          min="0"
          max={Math.max(1, durationMs)}
          step="100"
          value={shownMs}
          disabled={!player.state.canSeek}
          aria-label={seekLabel}
          aria-valuetext={seekValueText}
          onpointerdown={beginSeek}
          oninput={updateSeek}
          onchange={commitSeek}
        />
      </div>
      <span class="time">{formatTime(durationMs / 1000)}</span>
    </div>

    <div class="volume" role="group" aria-label={t("volume.label")}>
      <button
        class="bar-button"
        type="button"
        disabled={!player.state.canControl}
        aria-label={muted ? t("volume.unmute") : t("volume.mute")}
        title={muted ? t("volume.unmuteShortcut") : t("volume.muteShortcut")}
        onclick={toggleMute}
      >
        <Icon name={muted ? "volumeMute" : "volume"} size={15} />
      </button>
      <input
        class="volume-slider"
        type="range"
        min="0"
        max="100"
        step="1"
        value={player.state.volume}
        disabled={!player.state.canControl}
        aria-label={t("volume.level")}
        aria-valuetext={t("volume.percent", { value: Math.round(player.state.volume) })}
        style={`--volume: ${player.state.volume}%`}
        oninput={setVolume}
      />
    </div>
  </div>
</footer>

<style>
  .now-playing {
    position: relative;
    isolation: isolate;
    display: flex;
    overflow: hidden;
    flex-direction: column;
    border-top: 1px solid var(--line);
    background:
      linear-gradient(180deg, rgb(255 255 255 / 3%), transparent 26%),
      color-mix(in srgb, var(--panel) 90%, var(--bg));
    backdrop-filter: blur(22px) saturate(118%);
  }

  .bar-ambience {
    position: absolute;
    z-index: 0;
    inset: -38%;
    background-position: center;
    background-size: cover;
    filter: blur(38px) saturate(1.35) brightness(0.9);
    opacity: 0.24;
    pointer-events: none;
    transform: scale(1.12);
  }

  .now-playing::before {
    content: "";
    position: absolute;
    z-index: 2;
    top: -1px;
    right: 0;
    left: 0;
    height: 1px;
    background: linear-gradient(
      90deg,
      transparent,
      color-mix(in srgb, var(--accent) 42%, transparent),
      color-mix(in srgb, var(--accent-2) 32%, transparent),
      transparent
    );
    background-size: 220% 100%;
    opacity: 0.7;
    pointer-events: none;
  }

  .now-playing.is-playing::before {
    animation: bar-flow 6s linear infinite;
  }

  @keyframes bar-flow {
    from {
      background-position: 0% 0;
    }
    to {
      background-position: 220% 0;
    }
  }

  .visualizer-strip {
    position: relative;
    z-index: 1;
    width: 100%;
    height: 18px;
    flex: 0 0 18px;
    opacity: 0.55;
    pointer-events: none;
  }

  .bar-row {
    position: relative;
    z-index: 1;
    display: grid;
    grid-template-columns: minmax(220px, 1.3fr) auto minmax(180px, 1fr) auto;
    align-items: center;
    gap: 18px;
    padding: 12px 24px;
  }

  .track {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 11px;
  }

  .track-open {
    display: flex;
    min-width: 0;
    flex: 1 1 auto;
    align-items: center;
    gap: 11px;
    padding: 0;
    border: 0;
    border-radius: 9px;
    background: none;
    text-align: left;
    cursor: pointer;
    transition: transform var(--motion-fast, 140ms) var(--ease-spring, ease);
  }

  .track-open:hover {
    transform: translateY(-1px);
  }

  .track-open:focus-visible {
    outline: 1px solid var(--accent);
    outline-offset: 3px;
  }

  .cover {
    position: relative;
    display: grid;
    width: 48px;
    height: 48px;
    flex: 0 0 48px;
    place-items: center;
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: 8px;
    background:
      radial-gradient(
        80% 70% at 70% 20%,
        color-mix(in srgb, var(--accent) 26%, transparent),
        transparent 60%
      ),
      var(--surface-2);
    color: var(--muted);
    box-shadow: 0 6px 16px rgb(0 0 0 / 26%);
    transition:
      box-shadow var(--motion-base) ease,
      transform var(--motion-base) var(--ease-spring);
  }

  .now-playing.is-playing .cover {
    box-shadow:
      0 8px 20px rgb(0 0 0 / 32%),
      0 0 18px color-mix(in srgb, var(--accent) 16%, transparent);
  }

  .cover:hover {
    transform: translateY(-1px) scale(1.03);
  }

  .cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .cover-eq {
    position: absolute;
    right: 3px;
    bottom: 3px;
    display: flex;
    height: 11px;
    align-items: flex-end;
    gap: 1.5px;
    padding: 2px 3px;
    border-radius: 4px;
    background: rgb(0 0 0 / 46%);
    backdrop-filter: blur(4px);
  }

  .cover-eq i {
    display: block;
    width: 1.5px;
    height: 5px;
    border-radius: 1px;
    background: var(--accent);
    box-shadow: 0 0 4px var(--accent-glow);
    animation: cover-eq-bounce 900ms ease-in-out infinite;
  }

  .cover-eq i:nth-child(2) {
    height: 9px;
    animation-delay: -0.3s;
  }

  .cover-eq i:nth-child(3) {
    height: 7px;
    animation-delay: -0.6s;
  }

  @keyframes cover-eq-bounce {
    0%,
    100% {
      transform: scaleY(0.4);
    }
    50% {
      transform: scaleY(1);
    }
  }

  .track-copy {
    display: flex;
    min-width: 0;
    flex: 1 1 auto;
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

  .heart-button,
  .bar-button {
    display: grid;
    width: 28px;
    height: 28px;
    flex: 0 0 28px;
    place-items: center;
    border: 1px solid transparent;
    border-radius: 7px;
    color: var(--faint);
    transition:
      color var(--motion-fast, 120ms) ease,
      border-color var(--motion-fast, 120ms) ease,
      background-color var(--motion-fast, 120ms) ease,
      transform var(--motion-fast, 120ms) var(--ease-spring, ease);
  }

  .heart-button:hover:not(:disabled),
  .bar-button:hover:not(:disabled) {
    border-color: var(--line);
    background: var(--surface-2);
    color: var(--text);
    transform: translateY(-1px);
  }

  .heart-button.is-liked,
  .bar-button.is-active {
    border-color: color-mix(in srgb, var(--accent) 28%, transparent);
    background: color-mix(in srgb, var(--accent) 9%, transparent);
    color: var(--accent);
  }

  .heart-button.is-liked {
    animation: heart-pop 340ms var(--ease-spring);
  }

  @keyframes heart-pop {
    0% {
      transform: scale(0.85);
    }
    55% {
      transform: scale(1.14);
    }
    100% {
      transform: scale(1);
    }
  }

  .heart-button:disabled,
  .bar-button:disabled {
    opacity: 0.45;
  }

  .transport {
    display: flex;
    align-items: center;
    gap: 2px;
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

  .seek-shell {
    position: relative;
    display: flex;
    min-width: 0;
    flex: 1 1 auto;
    height: 22px;
    align-items: center;
  }

  .seek-shell::before {
    content: "";
    position: absolute;
    inset: 1px 0;
    border: 1px solid transparent;
    border-radius: 9px;
    background:
      linear-gradient(180deg, rgb(255 255 255 / 4%), transparent 55%),
      color-mix(in srgb, var(--surface) 24%, transparent);
    box-shadow: 0 1px 0 rgb(255 255 255 / 4%) inset;
    opacity: 0.8;
    pointer-events: none;
    transition:
      border-color var(--motion-fast) ease,
      opacity var(--motion-fast) ease;
  }

  .seek-shell:hover::before,
  .seek-shell:focus-within::before {
    border-color: color-mix(in srgb, var(--accent) 24%, transparent);
    opacity: 1;
  }

  .seek-wave {
    position: absolute;
    inset: 1px 6px;
    display: block;
  }

  .seek-playhead {
    position: absolute;
    z-index: 2;
    top: 50%;
    left: clamp(5px, var(--progress, 0%), calc(100% - 5px));
    width: 10px;
    height: 10px;
    border: 1.5px solid color-mix(in srgb, white 55%, var(--accent));
    border-radius: 50%;
    background: var(--accent);
    box-shadow:
      0 0 0 3px color-mix(in srgb, var(--accent) 14%, transparent),
      0 0 14px color-mix(in srgb, var(--accent) 40%, transparent);
    transform: translate(-50%, -50%);
    pointer-events: none;
    transition:
      width var(--motion-fast) var(--ease-spring),
      height var(--motion-fast) var(--ease-spring);
  }

  .seek-bubble {
    position: absolute;
    z-index: 4;
    bottom: calc(100% + 4px);
    padding: 3px 7px;
    border: 1px solid color-mix(in srgb, var(--accent) 34%, var(--line));
    border-radius: 5px;
    background:
      linear-gradient(180deg, rgb(255 255 255 / 7%), transparent 58%),
      color-mix(in srgb, var(--surface) 92%, transparent);
    box-shadow:
      0 6px 16px rgb(0 0 0 / 42%),
      0 0 14px color-mix(in srgb, var(--accent) 12%, transparent);
    color: var(--text);
    font-size: 9px;
    letter-spacing: 0.04em;
    line-height: 1;
    transform: translateX(-50%);
    pointer-events: none;
    white-space: nowrap;
    animation: seek-bubble-in 130ms var(--ease-out) both;
  }

  @keyframes seek-bubble-in {
    from {
      opacity: 0;
      transform: translateX(-50%) translateY(3px) scale(0.94);
    }
    to {
      opacity: 1;
      transform: translateX(-50%) translateY(0) scale(1);
    }
  }

  .now-playing.is-playing .seek-playhead::after {
    content: "";
    position: absolute;
    inset: -4px;
    border: 1px solid color-mix(in srgb, var(--accent) 35%, transparent);
    border-radius: 50%;
    animation: seek-pulse 1.9s var(--ease-in-out) infinite;
  }

  .seek-shell:hover .seek-playhead,
  .seek-shell:focus-within .seek-playhead {
    width: 13px;
    height: 13px;
  }

  .seek {
    position: absolute;
    z-index: 3;
    inset: -5px 0;
    width: 100%;
    height: calc(100% + 10px);
    margin: 0;
    padding: 0;
    appearance: none;
    background: transparent;
    cursor: pointer;
    opacity: 0;
  }

  .seek:disabled {
    cursor: default;
  }

  @keyframes seek-pulse {
    0% {
      opacity: 0.75;
      transform: scale(0.8);
    }
    70%,
    100% {
      opacity: 0;
      transform: scale(1.55);
    }
  }

  .volume {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--muted);
  }

  .volume-slider {
    width: 92px;
    height: 10px;
    margin: 0;
    padding: 0;
    appearance: none;
    background: transparent;
    cursor: pointer;
  }

  .volume-slider:disabled {
    cursor: default;
    opacity: 0.5;
  }

  .volume-slider::-webkit-slider-runnable-track {
    height: 3px;
    border-radius: 999px;
    background: linear-gradient(
      90deg,
      color-mix(in srgb, var(--accent) 70%, var(--muted)) var(--volume),
      var(--line-strong) var(--volume)
    );
  }

  .volume-slider::-moz-range-track {
    height: 3px;
    border-radius: 999px;
    background: linear-gradient(
      90deg,
      color-mix(in srgb, var(--accent) 70%, var(--muted)) var(--volume),
      var(--line-strong) var(--volume)
    );
  }

  .volume-slider::-webkit-slider-thumb {
    width: 11px;
    height: 11px;
    margin-top: -4px;
    appearance: none;
    border: 0;
    border-radius: 50%;
    background: var(--accent);
    transition: transform var(--motion-fast, 120ms) var(--ease-spring);
  }

  .volume-slider::-moz-range-thumb {
    width: 11px;
    height: 11px;
    border: 0;
    border-radius: 50%;
    background: var(--accent);
  }

  .volume-slider:hover::-webkit-slider-thumb {
    transform: scale(1.2);
  }

  @media (max-width: 900px) {
    .bar-row {
      grid-template-columns: minmax(190px, 1fr) auto;
      grid-template-areas:
        "track transport"
        "progress volume";
      gap: 8px 14px;
      padding: 10px 16px;
    }

    .track {
      grid-area: track;
    }

    .transport {
      grid-area: transport;
      justify-self: end;
    }

    .progress {
      grid-area: progress;
    }

    .volume {
      grid-area: volume;
      justify-self: end;
    }
  }

  @media (max-width: 560px) {
    .bar-row {
      padding: 10px 12px;
    }

    .transport {
      gap: 0;
    }
  }
</style>
