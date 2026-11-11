<script lang="ts">
  import { clamp, seededUnit } from "../visualizer";

  interface Props {
    percent: number;
    seed: number;
    count?: number;
    playing?: boolean;
    class?: string;
  }

  let { percent, seed, count = 64, playing = false, class: className = "" }: Props = $props();

  let progress = $derived(clamp(percent, 0, 100));
  let bars = $derived.by(() => {
    const total = Math.max(8, Math.round(count));
    return Array.from({ length: total }, (_, index) => {
      const x = index / Math.max(1, total - 1);
      const envelope = 0.42 + 0.58 * Math.sin(Math.PI * x);
      const detail = 0.45 + 0.55 * seededUnit(seed * 1000, index * 7);
      const group = 0.68 + 0.32 * Math.sin(index * 0.52 + seed * 17);
      const quiet = seededUnit(seed * 500, index * 13) > 0.92 ? 0.35 : 1;
      return 3 + Math.pow(Math.max(0, envelope * detail * group * quiet), 1.18) * 15;
    });
  });
</script>

<div
  class={`waveform ${className}`.trim()}
  class:is-playing={playing}
  style={`--progress: ${progress}%`}
  aria-hidden="true"
>
  <div class="waveform-layer base">
    {#each bars as height, index (index)}
      <span style={`--bar-height: ${height.toFixed(1)}px`}></span>
    {/each}
  </div>
  <div class="waveform-layer fill">
    {#each bars as height, index (index)}
      <span style={`--bar-height: ${height.toFixed(1)}px`}></span>
    {/each}
    <span class="waveform-shimmer"></span>
  </div>
</div>

<style>
  .waveform {
    position: relative;
    width: 100%;
    height: 100%;
    overflow: hidden;
    pointer-events: none;
  }

  .waveform-layer {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1px;
  }

  .waveform-layer span {
    display: block;
    width: 2px;
    max-width: 3px;
    height: var(--bar-height);
    flex: 1 1 0;
    border-radius: 999px;
    background: color-mix(in srgb, var(--text) 17%, transparent);
    transition: background-color var(--motion-fast) ease;
  }

  .waveform-layer.fill {
    clip-path: inset(0 calc(100% - var(--progress, 0%)) 0 0);
    filter: drop-shadow(0 0 4px color-mix(in srgb, var(--accent) 42%, transparent));
  }

  .waveform-layer.fill span {
    background: linear-gradient(
      180deg,
      color-mix(in srgb, white 42%, var(--accent)),
      var(--accent) 62%,
      color-mix(in srgb, var(--accent) 70%, var(--accent-2))
    );
  }

  .waveform-shimmer {
    position: absolute;
    top: 0;
    bottom: 0;
    left: -35%;
    width: 35%;
    background: linear-gradient(
      90deg,
      transparent,
      color-mix(in srgb, white 55%, transparent),
      transparent
    );
    mix-blend-mode: screen;
    opacity: 0;
  }

  .waveform.is-playing .waveform-layer.fill {
    animation: waveform-breathe 1.9s var(--ease-in-out, ease-in-out) infinite;
    transform-origin: center;
  }

  .waveform.is-playing .waveform-shimmer {
    opacity: 0.75;
    animation: waveform-shimmer 2.4s var(--ease-in-out, ease-in-out) infinite;
  }

  @keyframes waveform-breathe {
    0%,
    100% {
      transform: scaleY(0.94);
    }
    50% {
      transform: scaleY(1.05);
    }
  }

  @keyframes waveform-shimmer {
    from {
      transform: translateX(0);
    }
    to {
      transform: translateX(390%);
    }
  }
</style>
