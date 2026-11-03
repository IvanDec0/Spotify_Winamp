<script lang="ts">
  import Icon from "../../components/Icon.svelte";
  import { player } from "../../player.svelte";
  import { clamp } from "../../visualizer";

  interface Props {
    orientation?: "horizontal" | "vertical";
    showIcon?: boolean;
  }

  let { orientation = "horizontal", showIcon = true }: Props = $props();

  let local = $state(0);
  let previous = 65;

  $effect(() => {
    const next = clamp(player.state.volume, 0, 100);
    local = next;
    if (next > 0) previous = next;
  });

  function update(value: number): void {
    local = clamp(value, 0, 100);
    if (local > 0) previous = local;
    void player.setVolume(Math.round(local));
  }

  function toggleMute(): void {
    if (!player.state.canControl) return;
    update(local > 0 ? 0 : previous || 65);
  }
</script>

<div
  class="cad-volume"
  class:is-vertical={orientation === "vertical"}
  aria-label="Volume"
  data-component="volume"
>
  {#if showIcon}
    <button
      class="cad-volume-button"
      type="button"
      disabled={!player.state.canControl}
      aria-label={local > 0 ? "Mute" : "Unmute"}
      title={local > 0 ? "Mute (M)" : "Unmute (M)"}
      onclick={toggleMute}
    >
      <Icon name={local > 0 ? "volume" : "volumeMute"} size={12} />
    </button>
  {/if}
  <input
    type="range"
    min="0"
    max="100"
    step="1"
    value={local}
    disabled={!player.state.canControl}
    aria-label="Volume level"
    aria-valuetext={`${Math.round(local)} percent`}
    style={`--progress: ${local}%`}
    oninput={(event) => update((event.currentTarget as HTMLInputElement).valueAsNumber)}
  />
</div>
