<script lang="ts">
  import { Status, type PlayerState } from "../../shared/api/types";
  import { t } from "../../shared/i18n";
  import { playerErrorCopy } from "../player-error";
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
  let errorCopy = $derived(playerErrorCopy(state.error));
  let statusLabel = $derived(
    state.status === Status.StatusPlaying
      ? t("browse.nowPlaying")
      : state.status === Status.StatusPaused
        ? t("player.paused")
        : t("player.stopped"),
  );
  let statusText = $derived(errorCopy.statusKey ? t(errorCopy.statusKey) : statusLabel);
  let statusDetail = $derived(errorCopy.detail || statusText);
  let artists = $derived(
    state.track.artists?.join(", ") ||
      (errorCopy.hintKey ? t(errorCopy.hintKey) : t("player.unknownArtist")),
  );
  let displayTitle = $derived(
    state.track.title || (errorCopy.titleKey ? t(errorCopy.titleKey) : t("browse.nothingPlaying")),
  );
</script>

<section class="player" class:is-playing={playing} aria-label={t("player.label")}>
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
      <div class="cover-stage" class:is-playing={playing}>
        <span class="cover-halo" aria-hidden="true"></span>
        <CoverArt cover={state.track.cover} title={state.track.title} size={88} />
      </div>

      {#key state.track.id}
        <div class="track-copy">
          <div class="status-line">
            <span class="status-group">
              <span
                class="status-dot"
                class:is-playing={playing}
                class:is-error={Boolean(state.error)}
                aria-hidden="true"
              ></span>
              <span class="eyebrow" class:is-error={Boolean(state.error)} title={statusDetail}>
                {statusText}
              </span>
              {#if state.shuffle}
                <span class="mode-tag">{t("player.shuffleShort")}</span>
              {/if}
            </span>

            <span class="level" class:is-playing={playing} aria-hidden="true">
              <span></span><span></span><span></span><span></span>
            </span>
          </div>

          <h1 class="track-title" title={displayTitle}>{displayTitle}</h1>
          <p class="track-artists" title={artists}>{artists}</p>
          {#if state.track.album}
            <p class="track-album" title={state.track.album}>{state.track.album}</p>
          {/if}
        </div>
      {/key}
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
          aria-label={t("player.visualizerControl", { label: visualizerLabel })}
          title={t("player.changeVisualizer")}
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
    width: 100%;
    height: 100%;
    min-width: 0;
    flex: 1 1 auto;
    overflow: hidden;
    background:
      radial-gradient(
        80% 90% at 84% 8%,
        color-mix(in srgb, var(--accent) 8%, transparent),
        transparent 55%
      ),
      linear-gradient(180deg, transparent, color-mix(in srgb, var(--bg) 32%, transparent));
  }

  .player::before {
    content: "";
    position: absolute;
    z-index: 0;
    top: -34%;
    left: 32%;
    width: 58%;
    height: 90%;
    border-radius: 50%;
    background: radial-gradient(
      closest-side,
      color-mix(in srgb, var(--accent) 12%, transparent),
      transparent 72%
    );
    filter: blur(18px);
    opacity: 0.7;
    pointer-events: none;
    animation: player-bloom 12s var(--ease-in-out) infinite alternate;
  }

  @keyframes player-bloom {
    from {
      transform: translate3d(-2%, 0, 0) scale(0.98);
      opacity: 0.5;
    }
    to {
      transform: translate3d(3%, 4%, 0) scale(1.06);
      opacity: 0.82;
    }
  }

  .player-content {
    position: relative;
    z-index: 1;
    display: flex;
    height: 100%;
    flex-direction: column;
    padding: 11px 14px 9px;
    animation: content-in 460ms var(--ease-out) both;
  }

  @keyframes content-in {
    from {
      opacity: 0;
      transform: translateY(4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .track-row {
    display: grid;
    grid-template-columns: 88px minmax(0, 1fr);
    gap: 13px;
    align-items: start;
  }

  .cover-stage {
    position: relative;
    width: 88px;
    height: 88px;
  }

  .cover-stage.is-playing {
    animation: cover-float 6.5s ease-in-out infinite;
  }

  .cover-halo {
    position: absolute;
    z-index: -1;
    inset: -14%;
    border-radius: 24%;
    background: radial-gradient(
      closest-side,
      color-mix(in srgb, var(--accent) 28%, transparent),
      transparent 74%
    );
    filter: blur(13px);
    opacity: 0.55;
    pointer-events: none;
    transition: opacity var(--motion-slow) ease;
  }

  .cover-stage.is-playing .cover-halo {
    animation: cover-glow 4.8s ease-in-out infinite;
  }

  @keyframes cover-float {
    0%,
    100% {
      transform: translateY(0);
    }
    50% {
      transform: translateY(-2px);
    }
  }

  @keyframes cover-glow {
    0%,
    100% {
      opacity: 0.4;
      transform: scale(0.98);
    }
    50% {
      opacity: 0.72;
      transform: scale(1.05);
    }
  }

  .track-copy {
    display: flex;
    min-width: 0;
    flex-direction: column;
    padding-top: 2px;
    animation: track-in 380ms var(--ease-out) both;
  }

  @keyframes track-in {
    from {
      opacity: 0;
      transform: translateY(6px);
      filter: blur(1.5px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
      filter: blur(0);
    }
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
    overflow: hidden;
    color: var(--faint);
    text-overflow: ellipsis;
    font-family: "JetBrains Mono", monospace;
    font-size: 7.5px;
    font-weight: 500;
    letter-spacing: 0.17em;
    line-height: 1;
    text-transform: uppercase;
    white-space: nowrap;
  }

  .eyebrow.is-error {
    max-width: 190px;
    color: var(--danger);
    letter-spacing: 0.06em;
    text-transform: none;
  }

  .status-dot.is-error {
    background: var(--danger);
    box-shadow: 0 0 6px var(--danger);
    animation: none;
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
    transition: text-shadow var(--motion-slow) ease;
  }

  .player.is-playing .track-title {
    text-shadow: 0 0 24px color-mix(in srgb, var(--accent) 23%, transparent);
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
    transform: translateY(-1px);
  }

  .mode-chip {
    transition:
      color var(--motion-fast) ease,
      border-color var(--motion-fast) ease,
      background-color var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-spring);
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
