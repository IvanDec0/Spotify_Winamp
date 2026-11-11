<script lang="ts">
  import { onMount, untrack } from "svelte";
  import { t } from "../../shared/i18n";
  import { formatTime } from "../format";
  import { clamp, trackSeed } from "../visualizer";
  import Waveform from "./Waveform.svelte";

  interface Props {
    positionMs: number;
    durationMs: number;
    playing: boolean;
    canSeek: boolean;
    onSeek: (positionMs: number) => void;
    trackId?: string;
  }

  let { positionMs, durationMs, playing, canSeek, onSeek, trackId = "" }: Props = $props();

  let dragging = $state(false);
  let dragValue = $state(0);
  let displayMs = $state(0);
  let visible = $state(true);
  let hoverPercent = $state<number | null>(null);
  let hoverMs = $state(0);

  let anchorMs = 0;
  let anchorAt = 0;
  let lastDuration = 0;

  let seed = $derived(trackSeed(trackId));

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
  let bubbleVisible = $derived(hoverPercent !== null || dragging);
  let bubblePercent = $derived(dragging ? percent : (hoverPercent ?? 0));
  let bubbleLabel = $derived(formatTime((dragging ? shownMs : hoverMs) / 1000));

  function handleInput(event: Event): void {
    const input = event.currentTarget as HTMLInputElement;
    dragging = true;
    dragValue = clamp(input.valueAsNumber, 0, Math.max(0, durationMs));
    displayMs = dragValue;
  }

  function handleMove(event: PointerEvent): void {
    const target = event.currentTarget as HTMLElement;
    const rect = target.getBoundingClientRect();
    if (rect.width <= 0) return;
    const ratio = clamp((event.clientX - rect.left) / rect.width, 0, 1);
    hoverPercent = ratio * 100;
    hoverMs = ratio * Math.max(0, durationMs);
  }

  function handleLeave(): void {
    if (!dragging) hoverPercent = null;
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

<div
  class="progress"
  class:is-playing={playing}
  class:is-dragging={dragging}
  style={`--progress: ${percent}%`}
>
  <div
    class="progress-track"
    role="presentation"
    onpointermove={handleMove}
    onpointerleave={handleLeave}
  >
    <span class="progress-glass" aria-hidden="true"></span>
    <span class="progress-wave">
      <Waveform {percent} {seed} {playing} />
    </span>
    <span class="progress-playhead" aria-hidden="true"></span>
    {#if bubbleVisible}
      <span
        class="seek-bubble mono"
        style={`left: clamp(17px, ${bubblePercent}%, calc(100% - 17px))`}
        aria-hidden="true"
      >
        {bubbleLabel}
      </span>
    {/if}
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
    <span class="progress-elapsed">{formatTime(shownMs / 1000)}</span>
    <span class="progress-total">{formatTime(durationMs / 1000)}</span>
  </div>
</div>

<style>
  .progress {
    display: flex;
    flex-direction: column;
    gap: 3px;
    --progress: 0%;
  }

  .progress-track {
    position: relative;
    display: flex;
    height: 20px;
    align-items: center;
    isolation: isolate;
  }

  .progress-glass {
    position: absolute;
    inset: 1px 0;
    border: 1px solid transparent;
    border-radius: 9px;
    background:
      linear-gradient(180deg, rgb(255 255 255 / 4%), transparent 55%),
      color-mix(in srgb, var(--surface) 26%, transparent);
    box-shadow:
      0 1px 0 rgb(255 255 255 / 5%) inset,
      0 6px 16px rgb(0 0 0 / 18%) inset;
    opacity: 0.75;
    pointer-events: none;
    transition:
      border-color var(--motion-fast) ease,
      opacity var(--motion-fast) ease,
      background-color var(--motion-fast) ease;
  }

  .progress:hover .progress-glass,
  .progress:focus-within .progress-glass,
  .progress.is-dragging .progress-glass {
    border-color: color-mix(in srgb, var(--accent) 24%, transparent);
    background-color: color-mix(in srgb, var(--surface-2) 34%, transparent);
    opacity: 1;
  }

  .progress:focus-within .progress-track {
    outline: 1px solid color-mix(in srgb, var(--accent) 55%, transparent);
    outline-offset: 3px;
    border-radius: 9px;
  }

  .progress-wave {
    position: absolute;
    inset: 1px 6px;
    display: block;
    opacity: 0.9;
    transition: opacity var(--motion-fast) ease;
  }

  .progress:hover .progress-wave,
  .progress:focus-within .progress-wave,
  .progress.is-dragging .progress-wave {
    opacity: 1;
  }

  .progress-playhead {
    position: absolute;
    z-index: 2;
    top: 50%;
    left: clamp(5px, var(--progress), calc(100% - 5px));
    width: 9px;
    height: 9px;
    border: 1.5px solid color-mix(in srgb, white 55%, var(--accent));
    border-radius: 50%;
    background: var(--accent);
    box-shadow:
      0 0 0 3px color-mix(in srgb, var(--accent) 14%, transparent),
      0 0 14px color-mix(in srgb, var(--accent) 45%, transparent);
    transform: translate(-50%, -50%);
    pointer-events: none;
    transition:
      width var(--motion-fast) var(--ease-spring),
      height var(--motion-fast) var(--ease-spring),
      box-shadow var(--motion-fast) ease;
  }

  .progress.is-playing .progress-playhead::after {
    content: "";
    position: absolute;
    inset: -4px;
    border: 1px solid color-mix(in srgb, var(--accent) 35%, transparent);
    border-radius: 50%;
    animation: playhead-pulse 1.9s var(--ease-in-out) infinite;
  }

  .progress:hover .progress-playhead,
  .progress:focus-within .progress-playhead,
  .progress.is-dragging .progress-playhead {
    width: 12px;
    height: 12px;
    box-shadow:
      0 0 0 4px color-mix(in srgb, var(--accent) 16%, transparent),
      0 0 20px color-mix(in srgb, var(--accent) 55%, transparent);
  }

  .seek-bubble {
    position: absolute;
    z-index: 4;
    bottom: calc(100% + 3px);
    padding: 3px 6px;
    border: 1px solid color-mix(in srgb, var(--accent) 34%, var(--line));
    border-radius: 5px;
    background:
      linear-gradient(180deg, rgb(255 255 255 / 7%), transparent 58%),
      color-mix(in srgb, var(--surface) 92%, transparent);
    box-shadow:
      0 6px 16px rgb(0 0 0 / 42%),
      0 0 14px color-mix(in srgb, var(--accent) 12%, transparent);
    color: var(--text);
    font-size: 8px;
    letter-spacing: 0.04em;
    line-height: 1;
    transform: translateX(-50%);
    pointer-events: none;
    white-space: nowrap;
    animation: bubble-in 130ms var(--ease-out) both;
  }

  input[type="range"] {
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

  input[type="range"]:disabled {
    cursor: default;
  }

  .progress-labels {
    display: flex;
    align-items: center;
    justify-content: space-between;
    color: var(--faint);
    font-size: 8px;
    letter-spacing: 0.06em;
    line-height: 1;
    transition: color var(--motion-fast) ease;
  }

  .progress:hover .progress-labels,
  .progress:focus-within .progress-labels {
    color: var(--muted);
  }

  .progress-elapsed {
    color: color-mix(in srgb, var(--text) 72%, var(--faint));
  }

  .progress.is-playing .progress-elapsed {
    color: var(--accent);
    text-shadow: 0 0 8px color-mix(in srgb, var(--accent) 35%, transparent);
  }

  @keyframes playhead-pulse {
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

  @keyframes bubble-in {
    from {
      opacity: 0;
      transform: translateX(-50%) translateY(3px) scale(0.94);
    }
    to {
      opacity: 1;
      transform: translateX(-50%) translateY(0) scale(1);
    }
  }
</style>
