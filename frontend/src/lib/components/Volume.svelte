<script lang="ts">
  import { clamp } from "../visualizer";
  import Icon from "./Icon.svelte";

  interface Props {
    volume: number;
    canControl: boolean;
    onVolume: (volume: number) => void;
  }

  let { volume, canControl, onVolume }: Props = $props();

  let local = $state(0);
  let previous = 65;

  $effect(() => {
    const next = clamp(volume, 0, 100);
    local = next;
    if (next > 0) previous = next;
  });

  function update(value: number): void {
    local = clamp(value, 0, 100);
    if (local > 0) previous = local;
    onVolume(Math.round(local));
  }

  function toggleMute(): void {
    if (!canControl) return;
    update(local > 0 ? 0 : previous || 65);
  }
</script>

<div class="volume" aria-label="Volume">
  <button
    class="volume-button"
    type="button"
    disabled={!canControl}
    aria-label={local > 0 ? "Mute" : "Unmute"}
    title={local > 0 ? "Mute (M)" : "Unmute (M)"}
    onclick={toggleMute}
  >
    <Icon name={local > 0 ? "volume" : "volumeMute"} size={13} />
  </button>
  <input
    type="range"
    min="0"
    max="100"
    step="1"
    value={local}
    disabled={!canControl}
    aria-label="Volume level"
    aria-valuetext={`${Math.round(local)} percent`}
    style={`--volume: ${local}%`}
    oninput={(event) => update((event.currentTarget as HTMLInputElement).valueAsNumber)}
  />
</div>

<style>
  .volume {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 4px;
    color: var(--muted);
  }

  .volume-button {
    display: grid;
    width: 20px;
    height: 20px;
    place-items: center;
    border-radius: 4px;
    color: var(--muted);
    transition: color 130ms ease;
  }

  .volume-button:hover:not(:disabled) {
    color: var(--text);
  }

  .volume-button:disabled {
    opacity: 0.4;
  }

  input[type="range"] {
    width: 48px;
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
    height: 2px;
    border-radius: 999px;
    background: linear-gradient(
      90deg,
      color-mix(in srgb, var(--accent) 70%, var(--muted)) var(--volume),
      var(--line-strong) var(--volume)
    );
  }

  input[type="range"]::-moz-range-track {
    height: 2px;
    border-radius: 999px;
    background: linear-gradient(
      90deg,
      color-mix(in srgb, var(--accent) 70%, var(--muted)) var(--volume),
      var(--line-strong) var(--volume)
    );
  }

  input[type="range"]::-webkit-slider-thumb {
    width: 6px;
    height: 6px;
    margin-top: -2px;
    appearance: none;
    border: 0;
    border-radius: 50%;
    background: var(--text);
    box-shadow: 0 0 6px var(--accent-glow);
  }

  input[type="range"]::-moz-range-thumb {
    width: 6px;
    height: 6px;
    border: 0;
    border-radius: 50%;
    background: var(--text);
    box-shadow: 0 0 6px var(--accent-glow);
  }
</style>
