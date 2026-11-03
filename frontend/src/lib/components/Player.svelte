<script lang="ts">
  import { Status, type PlayerState } from "../../../bindings/cadence/internal/player/models";
  import type { VisualizerMode } from "../preferences.svelte";
  import CoverArt from "./CoverArt.svelte";
  import Icon from "./Icon.svelte";
  import Progress from "./Progress.svelte";
  import Transport from "./Transport.svelte";
  import Visualizer from "./Visualizer.svelte";
  import Volume from "./Volume.svelte";

  interface Props {
    state: PlayerState;
    visualizerMode: VisualizerMode;
    visualizerLabel: string;
    accent: string;
    secondary: string;
    reducedMotion: boolean;
    onToggle: () => void;
    onPrevious: () => void;
    onNext: () => void;
    onSeek: (positionMs: number) => void;
    onVolume: (volume: number) => void;
    onShuffle: () => void;
    onRepeat: () => void;
    onCycleVisualizer: () => void;
  }

  let {
    state,
    visualizerMode,
    visualizerLabel,
    accent,
    secondary,
    reducedMotion,
    onToggle,
    onPrevious,
    onNext,
    onSeek,
    onVolume,
    onShuffle,
    onRepeat,
    onCycleVisualizer,
  }: Props = $props();

  let playing = $derived(state.status === Status.StatusPlaying);
  let artists = $derived(state.track.artists?.join(", ") || "Unknown artist");
  let statusLabel = $derived(
    state.status === Status.StatusPlaying
      ? "Now playing"
      : state.status === Status.StatusPaused
        ? "Paused"
        : "Stopped",
  );
</script>

<section class="player" aria-label="Player">
  <Visualizer
    mode={visualizerMode}
    trackId={state.track.id}
    {playing}
    {reducedMotion}
    {accent}
    {secondary}
  />

  <div class="player-content">
    <div class="track-row">
      <CoverArt cover={state.track.cover} title={state.track.title} size={88} />

      <div class="track-copy">
        <div class="status-line">
          <span class="status-group">
            <span class="status-dot" class:is-playing={playing} aria-hidden="true"></span>
            <span class="eyebrow">{statusLabel}</span>
            {#if state.shuffle}
              <span class="mode-tag">SHUF</span>
            {/if}
          </span>

          <span class="level" class:is-playing={playing} aria-hidden="true">
            <span></span><span></span><span></span><span></span>
          </span>
        </div>

        <h1 class="track-title" title={state.track.title}>{state.track.title}</h1>
        <p class="track-artists" title={artists}>{artists}</p>
        {#if state.track.album}
          <p class="track-album" title={state.track.album}>{state.track.album}</p>
        {/if}
      </div>
    </div>

    <div class="lower">
      <Progress
        positionMs={state.progressMs}
        durationMs={state.durationMs}
        {playing}
        canSeek={state.canSeek}
        {onSeek}
      />

      <div class="control-row">
        <button
          class="mode-chip"
          type="button"
          aria-label={`Visualizer: ${visualizerLabel}. Change mode`}
          title="Change visualizer (V)"
          onclick={onCycleVisualizer}
        >
          <Icon name="visualizer" size={10} />
          <span>{visualizerLabel}</span>
        </button>

        <Transport
          {playing}
          shuffle={state.shuffle}
          repeat={state.repeat}
          canControl={state.canControl}
          {onToggle}
          {onPrevious}
          {onNext}
          {onShuffle}
          {onRepeat}
        />

        <Volume volume={state.volume} canControl={state.canControl} {onVolume} />
      </div>
    </div>
  </div>

  {#key state.track.id}
    <span class="track-sweep" aria-hidden="true"></span>
  {/key}
</section>

<style>
  .player {
    position: relative;
    height: 100%;
    overflow: hidden;
    background:
      radial-gradient(
        80% 90% at 84% 8%,
        color-mix(in srgb, var(--accent) 7%, transparent),
        transparent 55%
      ),
      linear-gradient(180deg, transparent, color-mix(in srgb, var(--bg) 32%, transparent));
  }

  .player-content {
    position: relative;
    z-index: 1;
    display: flex;
    height: 100%;
    flex-direction: column;
    padding: 11px 14px 9px;
  }

  .track-row {
    display: grid;
    grid-template-columns: 88px minmax(0, 1fr);
    gap: 13px;
    align-items: start;
  }

  .track-copy {
    display: flex;
    min-width: 0;
    flex-direction: column;
    padding-top: 2px;
  }

  .status-line {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    margin-bottom: 9px;
  }

  .status-group {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 5px;
  }

  .status-dot {
    width: 4px;
    height: 4px;
    flex: 0 0 4px;
    border-radius: 50%;
    background: var(--faint);
    transition:
      background-color 180ms ease,
      box-shadow 180ms ease;
  }

  .status-dot.is-playing {
    background: var(--accent);
    box-shadow: 0 0 7px var(--accent);
    animation: status-pulse 2.4s ease-in-out infinite;
  }

  .eyebrow {
    color: var(--faint);
    font-family: "JetBrains Mono", monospace;
    font-size: 7.5px;
    font-weight: 500;
    letter-spacing: 0.17em;
    line-height: 1;
    text-transform: uppercase;
    white-space: nowrap;
  }

  .mode-tag {
    padding: 1px 3px;
    border: 1px solid var(--line);
    border-radius: 2px;
    color: var(--faint);
    font-family: "JetBrains Mono", monospace;
    font-size: 6px;
    letter-spacing: 0.08em;
    line-height: 1.2;
  }

  .level {
    display: flex;
    height: 12px;
    align-items: flex-end;
    gap: 2px;
    opacity: 0.45;
    transition: opacity 180ms ease;
  }

  .level span {
    width: 1.5px;
    height: 3px;
    border-radius: 1px;
    background: var(--accent);
    transform-origin: bottom;
  }

  .level span:nth-child(1) {
    height: 4px;
  }

  .level span:nth-child(2) {
    height: 8px;
  }

  .level span:nth-child(3) {
    height: 12px;
  }

  .level span:nth-child(4) {
    height: 6px;
  }

  .level.is-playing {
    opacity: 0.8;
  }

  .level.is-playing span {
    animation: level-bounce 1.1s ease-in-out infinite;
  }

  .level.is-playing span:nth-child(2) {
    animation-delay: -0.36s;
  }

  .level.is-playing span:nth-child(3) {
    animation-delay: -0.72s;
  }

  .level.is-playing span:nth-child(4) {
    animation-delay: -0.18s;
  }

  .track-title {
    overflow: hidden;
    margin: 0;
    color: var(--text);
    font-size: 17.5px;
    font-weight: 620;
    letter-spacing: -0.025em;
    line-height: 1.16;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .track-artists {
    overflow: hidden;
    margin: 4px 0 0;
    color: color-mix(in srgb, var(--text) 68%, var(--muted));
    font-size: 11px;
    font-weight: 460;
    letter-spacing: 0.005em;
    line-height: 1.3;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .track-album {
    overflow: hidden;
    margin: 3px 0 0;
    color: var(--faint);
    font-size: 9.5px;
    font-weight: 420;
    letter-spacing: 0.008em;
    line-height: 1.3;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .lower {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: auto;
    padding-top: 7px;
  }

  .control-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
    align-items: center;
    gap: 6px;
    min-height: 36px;
  }

  .mode-chip {
    display: inline-flex;
    width: fit-content;
    max-width: 76px;
    height: 20px;
    align-items: center;
    gap: 4px;
    padding: 0 5px;
    border: 1px solid transparent;
    border-radius: 4px;
    color: var(--muted);
    font-family: "JetBrains Mono", monospace;
    font-size: 6.5px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    transition:
      color 130ms ease,
      border-color 130ms ease,
      background-color 130ms ease;
  }

  .mode-chip span {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .mode-chip:hover {
    border-color: var(--line);
    background: var(--surface-2);
    color: var(--accent);
  }

  .track-sweep {
    position: absolute;
    top: 0;
    bottom: 0;
    left: 0;
    width: 38%;
    background: linear-gradient(
      90deg,
      transparent,
      color-mix(in srgb, var(--accent) 9%, transparent),
      transparent
    );
    opacity: 0;
    pointer-events: none;
    animation: sweep 720ms ease-out;
  }

  @keyframes status-pulse {
    0%,
    100% {
      opacity: 0.55;
    }
    50% {
      opacity: 1;
    }
  }

  @keyframes level-bounce {
    0%,
    100% {
      transform: scaleY(0.45);
    }
    50% {
      transform: scaleY(1);
    }
  }

  @keyframes sweep {
    0% {
      opacity: 0;
      transform: translateX(-110%);
    }
    28% {
      opacity: 1;
    }
    100% {
      opacity: 0;
      transform: translateX(320%);
    }
  }
</style>
