<script lang="ts">
  import { onMount, untrack } from "svelte";
  import { t } from "../../shared/i18n";
  import { formatTime } from "../format";
  import { clamp } from "../visualizer";

  interface Props {
    positionMs: number;
    durationMs: number;
    playing: boolean;
    canSeek: boolean;
    onSeek: (positionMs: number) => void;
  }

  let { positionMs, durationMs, playing, canSeek, onSeek }: Props = $props();

  let dragging = $state(false);
  let dragValue = $state(0);
  let displayMs = $state(0);
  let visible = $state(true);

  let anchorMs = 0;
  let anchorAt = 0;
  let lastDuration = 0;

  $effect(() => {
    const reported = clamp(positionMs, 0, Math.max(0, durationMs));
    const total = Math.max(0, durationMs);
    const isPlaying = playing;
    const now = performance.now();

    if (!isPlaying) {
      anchorMs = reported;
      anchorAt = now;
      lastDuration = total;
      displayMs = reported;
      return;
    }

    const projected = anchorMs + (now - anchorAt);
    if (total !== lastDuration || Math.abs(projected - reported) > 450) {
      anchorMs = reported;
      anchorAt = now;
      lastDuration = total;
      displayMs = reported;
    }
  });

  // The frame loop only runs while playing, visible and not dragging. Pausing
  // or hiding cancels it and freezes the display at the anchored position.
  $effect(() => {
    const animate = playing && visible && !dragging;
    const total = Math.max(0, durationMs);
    if (!animate) {
      const frozen = playing ? anchorMs + (performance.now() - anchorAt) : anchorMs;
      displayMs = clamp(frozen, 0, total);
      return;
    }

    let active = true;
    let frame = requestAnimationFrame(function tick(now) {
      if (!active) return;
      displayMs = clamp(anchorMs + (now - anchorAt), 0, total);
      frame = requestAnimationFrame(tick);
    });
    return () => {
      active = false;
      cancelAnimationFrame(frame);
    };
  });

  let shownMs = $derived(dragging ? dragValue : displayMs);
  let percent = $derived(durationMs > 0 ? clamp((shownMs / durationMs) * 100, 0, 100) : 0);

  function handleInput(event: Event): void {
    const input = event.currentTarget as HTMLInputElement;
    dragging = true;
    dragValue = clamp(input.valueAsNumber, 0, Math.max(0, durationMs));
    displayMs = dragValue;
  }

  function commit(): void {
    if (!canSeek) {
      dragging = false;
      return;
    }
    const value = clamp(dragValue, 0, Math.max(0, durationMs));
    anchorMs = value;
    anchorAt = performance.now();
    lastDuration = durationMs;
    displayMs = value;
    dragging = false;
    onSeek(Math.round(value));
  }

  onMount(() => {
    anchorMs = untrack(() => clamp(positionMs, 0, Math.max(0, durationMs)));
    anchorAt = performance.now();
    displayMs = anchorMs;

    const updateVisibility = (): void => {
      visible = !document.hidden;
    };
    updateVisibility();
    document.addEventListener("visibilitychange", updateVisibility);
    return () => document.removeEventListener("visibilitychange", updateVisibility);
  });
</script>

<div class="progress" class:is-playing={playing} style={`--progress: ${percent}%`}>
  <div class="progress-track">
    <span class="progress-rail" aria-hidden="true"></span>
    <span class="progress-fill" aria-hidden="true"></span>
    <span class="progress-shine" aria-hidden="true"></span>
    <input
      type="range"
      min="0"
      max={Math.max(1, durationMs)}
      step="100"
      value={shownMs}
      disabled={!canSeek}
      aria-label={t("progress.label")}
      aria-valuetext={t("progress.value", {
        position: formatTime(shownMs / 1000),
        duration: formatTime(durationMs / 1000),
      })}
      onpointerdown={() => {
        dragging = true;
        dragValue = shownMs;
      }}
      oninput={handleInput}
      onchange={commit}
    />
  </div>
  <div class="progress-labels mono" aria-hidden="true">
    <span>{formatTime(shownMs / 1000)}</span>
    <span>{formatTime(durationMs / 1000)}</span>
  </div>
</div>

<style>
  .progress {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .progress-track {
    position: relative;
    display: flex;
    height: 11px;
    align-items: center;
  }

  .progress-rail,
  .progress-fill,
  .progress-shine {
    position: absolute;
    top: 50%;
    left: 0;
    height: 2px;
    border-radius: 999px;
    transform: translateY(-50%);
    pointer-events: none;
    transition: height var(--motion-fast) var(--ease-out);
  }

  .progress:hover .progress-rail,
  .progress:hover .progress-fill,
  .progress:hover .progress-shine,
  .progress:focus-within .progress-rail,
  .progress:focus-within .progress-fill,
  .progress:focus-within .progress-shine {
    height: 3px;
  }

  .progress-rail {
    width: 100%;
    background: var(--line-strong);
  }

  .progress-fill,
  .progress-shine {
    width: var(--progress);
  }

  .progress-fill {
    background: linear-gradient(
      90deg,
      color-mix(in srgb, var(--accent) 52%, transparent),
      var(--accent)
    );
    box-shadow: 0 0 12px color-mix(in srgb, var(--accent) 32%, transparent);
  }

  .progress-shine {
    overflow: hidden;
  }

  .progress-shine::after {
    content: "";
    position: absolute;
    inset: 0;
    background: linear-gradient(
      90deg,
      transparent,
      color-mix(in srgb, white 34%, transparent),
      transparent
    );
    transform: translateX(-100%);
  }

  .progress.is-playing .progress-shine::after {
    animation: progress-sheen 2.8s ease-in-out infinite;
  }

  @keyframes progress-sheen {
    0% {
      transform: translateX(-100%);
    }
    45%,
    100% {
      transform: translateX(100%);
    }
  }

  input[type="range"] {
    position: relative;
    z-index: 1;
    width: 100%;
    height: 11px;
    margin: 0;
    padding: 0;
    appearance: none;
    background: transparent;
    cursor: pointer;
  }

  input[type="range"]:disabled {
    cursor: default;
    opacity: 0.55;
  }

  input[type="range"]::-webkit-slider-runnable-track {
    height: 2px;
    border-radius: 999px;
    background: transparent;
  }

  input[type="range"]::-moz-range-track {
    height: 2px;
    border-radius: 999px;
    background: transparent;
  }

  input[type="range"]::-webkit-slider-thumb {
    width: 7px;
    height: 7px;
    margin-top: -2.5px;
    appearance: none;
    border: 0;
    border-radius: 50%;
    background: var(--accent);
    box-shadow:
      0 0 0 3px color-mix(in srgb, var(--accent) 14%, transparent),
      0 0 10px var(--accent-glow);
    opacity: 0.7;
    transition:
      opacity var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-spring);
  }

  input[type="range"]::-moz-range-thumb {
    width: 7px;
    height: 7px;
    border: 0;
    border-radius: 50%;
    background: var(--accent);
    box-shadow: 0 0 10px var(--accent-glow);
    opacity: 0.7;
    transition:
      opacity var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-spring);
  }

  input[type="range"]:hover::-webkit-slider-thumb,
  input[type="range"]:focus-visible::-webkit-slider-thumb,
  input[type="range"]:active::-webkit-slider-thumb {
    opacity: 1;
    transform: scale(1.18);
  }

  input[type="range"]:hover::-moz-range-thumb,
  input[type="range"]:focus-visible::-moz-range-thumb,
  input[type="range"]:active::-moz-range-thumb {
    opacity: 1;
    transform: scale(1.18);
  }

  .progress-labels {
    display: flex;
    justify-content: space-between;
    color: var(--faint);
    font-size: 7.5px;
    letter-spacing: 0.06em;
    line-height: 1;
  }
</style>
