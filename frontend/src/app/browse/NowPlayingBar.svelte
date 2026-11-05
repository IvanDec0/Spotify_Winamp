<script lang="ts">
  import { RepeatMode, Status } from "../../shared/api/types";
  import { coverURL } from "../../lib/artwork";
  import { formatTime } from "../../lib/format";
  import { player } from "../../lib/player.svelte";
  import { clamp } from "../../lib/visualizer";
  import { t } from "../../shared/i18n";
  import Button from "../../shared/ui/Button.svelte";
  import BrowseVisualizer from "./BrowseVisualizer.svelte";
  import Icon from "./Icon.svelte";
  import { browse } from "./browse.svelte";

  let positionMs = $state(0);
  let dragging = $state(false);
  let dragValue = $state(0);
  let lastVolume = $state(player.state.volume > 0 ? player.state.volume : 65);

  let track = $derived(player.state.track);
  let durationMs = $derived(player.state.durationMs);
  let playing = $derived(player.state.status === Status.StatusPlaying);
  let shuffle = $derived(player.state.shuffle);
  let repeat = $derived(player.state.repeat);
  let muted = $derived(player.state.volume <= 0);
  let coverSrc = $derived(track.cover.startsWith("/art/") ? coverURL(track.cover, 48) : "");
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
</script>

<footer class="now-playing" aria-label={t("browse.nowPlaying")}>
  <div class="track">
    <div class="cover" aria-hidden="true">
      {#if coverSrc}
        <img src={coverSrc} alt="" width="48" height="48" loading="eager" decoding="async" />
      {:else}
        <Icon name="note" size={18} />
      {/if}
    </div>
    <div class="track-copy">
      <span class="track-title">{track.title || t("browse.nothingPlaying")}</span>
      <span class="track-meta">{artistLabel}</span>
    </div>
    <div class="spectrum-preview" aria-hidden="true">
      <BrowseVisualizer />
    </div>
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

  <div class="progress" role="group" aria-label={seekLabel}>
    <span class="time">{formatTime(shownMs / 1000)}</span>
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
      style={`--seek: ${progressPercent}%`}
      onpointerdown={beginSeek}
      oninput={updateSeek}
      onchange={commitSeek}
    />
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
</footer>

<style>
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

  .spectrum-preview {
    width: 72px;
    height: 28px;
    flex: 0 0 72px;
    opacity: 0.85;
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

  .seek {
    width: 100%;
    height: 16px;
    margin: 0;
    padding: 0;
    appearance: none;
    background: transparent;
    cursor: pointer;
  }

  .seek:disabled {
    cursor: default;
    opacity: 0.5;
  }

  .seek::-webkit-slider-runnable-track {
    height: 3px;
    border-radius: 999px;
    background: linear-gradient(
      90deg,
      color-mix(in srgb, var(--accent) 70%, var(--muted)) var(--seek),
      var(--line-strong) var(--seek)
    );
  }

  .seek::-moz-range-track {
    height: 3px;
    border-radius: 999px;
    background: linear-gradient(
      90deg,
      color-mix(in srgb, var(--accent) 70%, var(--muted)) var(--seek),
      var(--line-strong) var(--seek)
    );
  }

  .seek::-webkit-slider-thumb {
    width: 11px;
    height: 11px;
    margin-top: -4px;
    appearance: none;
    border: 0;
    border-radius: 50%;
    background: var(--accent);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 12%, transparent);
    transition: transform var(--motion-fast, 120ms) var(--ease-spring);
  }

  .seek::-moz-range-thumb {
    width: 11px;
    height: 11px;
    border: 0;
    border-radius: 50%;
    background: var(--accent);
  }

  .seek:hover::-webkit-slider-thumb {
    transform: scale(1.2);
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
    .now-playing {
      grid-template-columns: minmax(0, 1fr) auto;
      gap: 10px 14px;
      padding: 12px 18px;
    }

    .spectrum-preview {
      display: none;
    }

    .progress {
      grid-column: 1 / -1;
    }

    .volume {
      grid-column: 1 / -1;
      justify-content: flex-end;
    }
  }

  @media (max-width: 560px) {
    .now-playing {
      padding: 10px 12px;
    }

    .transport {
      gap: 0;
    }
  }
</style>
