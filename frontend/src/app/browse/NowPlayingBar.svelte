<script lang="ts">
  import { Status } from "../../shared/api/types";
  import { coverURL } from "../../lib/artwork";
  import { formatTime } from "../../lib/format";
  import { player } from "../../lib/player.svelte";
  import { t } from "../../shared/i18n";
  import Button from "../../shared/ui/Button.svelte";
  import Icon from "./Icon.svelte";
  import { browse } from "./browse.svelte";

  let positionMs = $state(0);

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
  let heartLabel = $derived(browse.currentLiked ? t("browse.unlike") : t("browse.like"));

  $effect(() => {
    const isPlaying = player.state.status === Status.StatusPlaying;
    positionMs = player.positionMs;
    if (!isPlaying) return;
    const timer = setInterval(() => {
      positionMs = player.positionMs;
    }, 250);
    return () => clearInterval(timer);
  });

  function setVolume(event: Event): void {
    const input = event.currentTarget as HTMLInputElement;
    void player.setVolume(input.valueAsNumber);
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

  <div class="transport">
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
    <Icon name="volume" size={15} />
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

  .heart-button {
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
      background-color var(--motion-fast, 120ms) ease;
  }

  .heart-button:hover:not(:disabled) {
    background: var(--surface-2);
    color: var(--text);
  }

  .heart-button.is-liked {
    color: var(--accent);
  }

  .heart-button:disabled {
    opacity: 0.45;
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
