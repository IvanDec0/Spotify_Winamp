<script lang="ts">
  import { RepeatMode } from "../../shared/api/types";
  import { t } from "../../shared/i18n";
  import Icon from "./Icon.svelte";

  interface Props {
    playing: boolean;
    shuffle: boolean;
    repeat: RepeatMode;
    canControl: boolean;
    onToggle: () => void;
    onPrevious: () => void;
    onNext: () => void;
    onShuffle: () => void;
    onRepeat: () => void;
  }

  let {
    playing,
    shuffle,
    repeat,
    canControl,
    onToggle,
    onPrevious,
    onNext,
    onShuffle,
    onRepeat,
  }: Props = $props();

  let repeatLabel = $derived(
    repeat === RepeatMode.RepeatTrack
      ? t("transport.repeatTrack")
      : repeat === RepeatMode.RepeatContext
        ? t("transport.repeatContext")
        : t("transport.repeatOff"),
  );
</script>

<div class="transport" aria-label={t("transport.controls")}>
  <button
    class="control"
    class:is-active={shuffle}
    type="button"
    disabled={!canControl}
    aria-label={shuffle ? t("transport.shuffleOff") : t("transport.shuffleOn")}
    aria-pressed={shuffle}
    title={t("transport.shuffleShortcut")}
    onclick={onShuffle}
  >
    <Icon name="shuffle" size={13} />
  </button>

  <button
    class="control"
    type="button"
    disabled={!canControl}
    aria-label={t("transport.previousTrack")}
    title={t("transport.previousShortcut")}
    onclick={onPrevious}
  >
    <Icon name="previous" size={14} />
  </button>

  <button
    class="play"
    class:is-playing={playing}
    type="button"
    disabled={!canControl}
    aria-label={playing ? t("transport.pause") : t("transport.play")}
    title={playing ? t("transport.pauseShortcut") : t("transport.playShortcut")}
    onclick={onToggle}
  >
    <Icon name={playing ? "pause" : "play"} size={14} />
  </button>

  <button
    class="control"
    type="button"
    disabled={!canControl}
    aria-label={t("transport.nextTrack")}
    title={t("transport.nextShortcut")}
    onclick={onNext}
  >
    <Icon name="next" size={14} />
  </button>

  <button
    class="control"
    class:is-active={repeat !== RepeatMode.RepeatOff}
    type="button"
    disabled={!canControl}
    aria-label={repeatLabel}
    aria-pressed={repeat !== RepeatMode.RepeatOff}
    title={t("transport.repeatShortcut", { label: repeatLabel })}
    onclick={onRepeat}
  >
    <Icon name="repeat" size={13} />
    {#if repeat === RepeatMode.RepeatTrack}
      <span class="repeat-one" aria-hidden="true">1</span>
    {/if}
  </button>
</div>

<style>
  .transport {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 1px;
  }

  .control {
    position: relative;
    display: grid;
    width: 25px;
    height: 25px;
    place-items: center;
    border: 1px solid transparent;
    border-radius: 50%;
    color: var(--muted);
    transition:
      color var(--motion-fast) ease,
      background-color var(--motion-fast) ease,
      border-color var(--motion-fast) ease,
      transform var(--motion-fast) var(--ease-spring);
  }

  .control:hover:not(:disabled) {
    border-color: var(--line);
    background: var(--surface-2);
    color: var(--text);
    transform: translateY(-1px) scale(1.06);
  }

  .control:active:not(:disabled) {
    transform: translateY(0) scale(0.92);
  }

  .control.is-active {
    color: var(--accent);
    text-shadow: 0 0 8px var(--accent-glow);
  }

  .control.is-active::after {
    content: "";
    position: absolute;
    bottom: 1px;
    width: 2px;
    height: 2px;
    border-radius: 50%;
    background: currentColor;
    box-shadow: 0 0 4px currentColor;
  }

  .control:disabled,
  .play:disabled {
    cursor: default;
    opacity: 0.4;
  }

  .play {
    position: relative;
    display: grid;
    width: 36px;
    height: 36px;
    margin: 0 4px;
    place-items: center;
    border: 1px solid color-mix(in srgb, var(--accent) 38%, var(--line-strong));
    border-radius: 50%;
    background:
      radial-gradient(
        70% 70% at 32% 24%,
        color-mix(in srgb, var(--accent) 24%, transparent),
        transparent 70%
      ),
      color-mix(in srgb, var(--accent) 9%, var(--surface-2));
    color: var(--accent);
    box-shadow:
      0 0 0 3px color-mix(in srgb, var(--accent) 5%, transparent),
      0 0 16px color-mix(in srgb, var(--accent) 14%, transparent);
    transition:
      transform var(--motion-base) var(--ease-spring),
      border-color var(--motion-fast) ease,
      background-color var(--motion-fast) ease,
      box-shadow var(--motion-base) ease;
  }

  .play:hover:not(:disabled) {
    border-color: color-mix(in srgb, var(--accent) 62%, var(--line-strong));
    transform: scale(1.06);
    box-shadow:
      0 0 0 4px color-mix(in srgb, var(--accent) 7%, transparent),
      0 0 22px color-mix(in srgb, var(--accent) 22%, transparent);
  }

  .play:active:not(:disabled) {
    transform: scale(0.95);
  }

  .play.is-playing::before {
    content: "";
    position: absolute;
    inset: -5px;
    border: 1px solid color-mix(in srgb, var(--accent) 24%, transparent);
    border-radius: 50%;
    animation: breathe 2.8s ease-in-out infinite;
  }

  .repeat-one {
    position: absolute;
    right: 2px;
    bottom: 1px;
    font-family: "JetBrains Mono", monospace;
    font-size: 6px;
    font-weight: 700;
    line-height: 1;
  }

  @keyframes breathe {
    0%,
    100% {
      opacity: 0.28;
      transform: scale(1);
    }
    50% {
      opacity: 0.7;
      transform: scale(1.05);
    }
  }
</style>
