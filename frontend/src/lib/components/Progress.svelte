<script lang="ts">
  import { onMount, untrack } from "svelte";
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
    anchorMs = untrack(() => positionMs);
    anchorAt = performance.now();
    displayMs = anchorMs;

    let frame = 0;
    const tick = (): void => {
      if (!dragging && playing) {
        displayMs = clamp(anchorMs + (performance.now() - anchorAt), 0, durationMs);
      }
      frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  });
</script>

<div class="progress">
  <div class="progress-track">
    <input
      type="range"
      min="0"
      max={Math.max(1, durationMs)}
      step="100"
      value={shownMs}
      disabled={!canSeek}
      aria-label="Playback position"
      aria-valuetext={`${formatTime(shownMs / 1000)} of ${formatTime(durationMs / 1000)}`}
      style={`--progress: ${percent}%`}
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
    display: flex;
    height: 11px;
    align-items: center;
  }

  input[type="range"] {
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
    background: linear-gradient(
      90deg,
      var(--accent) 0%,
      var(--accent) var(--progress),
      var(--line-strong) var(--progress),
      var(--line-strong) 100%
    );
    box-shadow: 0 0 8px color-mix(in srgb, var(--accent) 18%, transparent);
  }

  input[type="range"]::-moz-range-track {
    height: 2px;
    border-radius: 999px;
    background: linear-gradient(
      90deg,
      var(--accent) 0%,
      var(--accent) var(--progress),
      var(--line-strong) var(--progress),
      var(--line-strong) 100%
    );
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
    opacity: 0;
    transition: opacity 120ms ease;
  }

  input[type="range"]::-moz-range-thumb {
    width: 7px;
    height: 7px;
    border: 0;
    border-radius: 50%;
    background: var(--accent);
    box-shadow: 0 0 10px var(--accent-glow);
    opacity: 0;
    transition: opacity 120ms ease;
  }

  input[type="range"]:hover::-webkit-slider-thumb,
  input[type="range"]:focus-visible::-webkit-slider-thumb,
  input[type="range"]:active::-webkit-slider-thumb {
    opacity: 1;
  }

  input[type="range"]:hover::-moz-range-thumb,
  input[type="range"]:focus-visible::-moz-range-thumb,
  input[type="range"]:active::-moz-range-thumb {
    opacity: 1;
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
